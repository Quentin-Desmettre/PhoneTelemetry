package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"phonedashboard/internal/api"
	"phonedashboard/internal/db"
	"phonedashboard/internal/metrics"
	"phonedashboard/internal/web"
)

// randomSecretFn generates the persisted JWT signing secret on first launch.
func randomSecretFn() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	dbPath := envDefault("DB_PATH", "/data/dashboard.db")
	port := envDefault("PORT", "8080")

	store, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	secret, err := store.GetOrCreateSecret("jwt", randomSecretFn)
	if err != nil {
		log.Fatalf("init secret: %v", err)
	}

	pollSec := settingInt(store, "poll_interval_seconds", 5)
	collector := metrics.NewCollector(store, time.Duration(pollSec)*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go collector.Run(ctx)
	go runPurge(ctx, store)

	handler := api.New(store, collector, secret, web.FS())
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s (db=%s, poll=%ds)", port, dbPath, pollSec)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// runPurge periodically trims samples older than the retention window so the
// SQLite file stays small on a resource-constrained phone.
func runPurge(ctx context.Context, store *db.DB) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	purge(store)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge(store)
		}
	}
}

func purge(store *db.DB) {
	days := settingInt(store, "retention_days", 7)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	if err := store.Purge(cutoff); err != nil {
		log.Printf("purge: %v", err)
	}
}

func settingInt(store *db.DB, key string, def int) int {
	v, err := store.GetSetting(key, strconv.Itoa(def))
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
