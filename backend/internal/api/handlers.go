package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phonedashboard/internal/db"
	"phonedashboard/internal/metrics"
)

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	store     *db.DB
	collector *metrics.Collector
	secret    string
	static    fs.FS
}

const (
	keyPollInterval  = "poll_interval_seconds"
	keyRetentionDays = "retention_days"
	defPollInterval  = "5"
	defRetentionDays = "7"
)

// New wires up the server and returns an http.Handler ready to serve.
func New(store *db.DB, collector *metrics.Collector, secret string, static fs.FS) http.Handler {
	s := &Server{store: store, collector: collector, secret: secret, static: static}
	return s.routes()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)

	mux.HandleFunc("GET /api/metrics/current", s.requireAuth(s.handleCurrent))
	mux.HandleFunc("GET /api/metrics/history", s.requireAuth(s.handleHistory))
	mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleGetSettings))
	mux.HandleFunc("PUT /api/settings", s.requireAuth(s.handlePutSettings))

	// Everything else serves the embedded Vue SPA.
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

// --- Auth / setup ----------------------------------------------------------

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	exists, err := s.store.AdminExists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "status check failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_complete":  exists,
		"authenticated":   s.validSession(r),
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	exists, err := s.store.AdminExists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "setup failed")
		return
	}
	if exists {
		// The setup panel must never be usable again once an admin exists.
		writeError(w, http.StatusForbidden, "already configured")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if len(body.Username) < 3 || len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "username min 3 chars, password min 8 chars")
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	if err := s.store.CreateAdmin(body.Username, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create admin")
		return
	}
	if err := s.issueSession(w, r, body.Username); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	username, hash, err := s.store.GetAdmin()
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if strings.TrimSpace(body.Username) != username || !checkPassword(hash, body.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := s.issueSession(w, r, username); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Metrics ---------------------------------------------------------------

func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	sample, err := s.store.LatestSample()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read metrics")
		return
	}
	if sample == nil {
		// No sample collected yet (just started up).
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, sample)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	rangeName := r.URL.Query().Get("range")
	dur, ok := rangeDurations[rangeName]
	if !ok {
		rangeName = "24h"
		dur = rangeDurations[rangeName]
	}
	poll := s.pollInterval()
	bucket := int64(dur.Seconds()) / 400 // ~400 points max per series
	if bucket < int64(poll.Seconds()) {
		bucket = int64(poll.Seconds())
	}
	if bucket < 1 {
		bucket = 1
	}
	since := time.Now().Add(-dur).Unix()
	h, err := s.store.QueryHistory(rangeName, since, bucket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read history")
		return
	}
	writeJSON(w, http.StatusOK, h)
}

var rangeDurations = map[string]time.Duration{
	"live": 5 * time.Minute,
	"1h":   time.Hour,
	"24h":  24 * time.Hour,
	"7d":   7 * 24 * time.Hour,
}

// --- Settings --------------------------------------------------------------

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	poll, _ := s.store.GetSetting(keyPollInterval, defPollInterval)
	ret, _ := s.store.GetSetting(keyRetentionDays, defRetentionDays)
	writeJSON(w, http.StatusOK, map[string]any{
		"poll_interval_seconds": atoiDefault(poll, 5),
		"retention_days":        atoiDefault(ret, 7),
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PollIntervalSeconds int `json:"poll_interval_seconds"`
		RetentionDays       int `json:"retention_days"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.PollIntervalSeconds < 1 || body.PollIntervalSeconds > 3600 {
		writeError(w, http.StatusBadRequest, "poll_interval_seconds must be between 1 and 3600")
		return
	}
	if body.RetentionDays < 1 || body.RetentionDays > 365 {
		writeError(w, http.StatusBadRequest, "retention_days must be between 1 and 365")
		return
	}
	if err := s.store.SetSetting(keyPollInterval, strconv.Itoa(body.PollIntervalSeconds)); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save settings")
		return
	}
	if err := s.store.SetSetting(keyRetentionDays, strconv.Itoa(body.RetentionDays)); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save settings")
		return
	}
	// Apply the new cadence to the running collector immediately.
	s.collector.SetInterval(time.Duration(body.PollIntervalSeconds) * time.Second)

	writeJSON(w, http.StatusOK, map[string]any{
		"poll_interval_seconds": body.PollIntervalSeconds,
		"retention_days":        body.RetentionDays,
	})
}

func (s *Server) pollInterval() time.Duration {
	v, _ := s.store.GetSetting(keyPollInterval, defPollInterval)
	return time.Duration(atoiDefault(v, 5)) * time.Second
}

// --- Static SPA ------------------------------------------------------------

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	if f, err := s.static.Open(p); err == nil {
		f.Close()
		http.FileServer(http.FS(s.static)).ServeHTTP(w, r)
		return
	}
	// SPA fallback: unknown routes return index.html so the Vue router works.
	r.URL.Path = "/"
	http.FileServer(http.FS(s.static)).ServeHTTP(w, r)
}

// --- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}
