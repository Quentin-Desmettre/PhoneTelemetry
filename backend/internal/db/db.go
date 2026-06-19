package db

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection and exposes the queries the app needs.
type DB struct {
	conn *sql.DB
}

// Sample is one point-in-time snapshot of the device metrics.
type Sample struct {
	TS              int64              `json:"ts"`
	CPUPct          float64            `json:"cpu_pct"`
	MemPct          float64            `json:"mem_pct"`
	MemUsed         uint64             `json:"mem_used"`
	MemTotal        uint64             `json:"mem_total"`
	DiskPct         float64            `json:"disk_pct"`
	DiskUsed        uint64             `json:"disk_used"`
	DiskTotal       uint64             `json:"disk_total"`
	BatteryPct      *float64           `json:"battery_pct"`
	BatteryCharging *bool              `json:"battery_charging"`
	Temps           map[string]float64 `json:"temps"`
}

// Open opens (and migrates) the SQLite database at path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	// SQLite handles concurrency best with a single writer connection.
	conn.SetMaxOpenConns(1)
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.conn.Close() }

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS samples (
			id INTEGER PRIMARY KEY,
			ts INTEGER NOT NULL,
			cpu_pct REAL NOT NULL,
			mem_pct REAL NOT NULL,
			mem_used INTEGER NOT NULL,
			mem_total INTEGER NOT NULL,
			disk_pct REAL NOT NULL,
			disk_used INTEGER NOT NULL,
			disk_total INTEGER NOT NULL,
			battery_pct REAL,
			battery_charging INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts)`,
		`CREATE TABLE IF NOT EXISTS temp_samples (
			ts INTEGER NOT NULL,
			sensor TEXT NOT NULL,
			value REAL NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_temp_samples_ts ON temp_samples(ts)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS admin (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			username TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS app_secret (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := d.conn.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// --- Samples ---------------------------------------------------------------

// InsertSample persists a sample and its temperature readings in one tx.
func (d *DB) InsertSample(s Sample) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var batPct interface{}
	if s.BatteryPct != nil {
		batPct = *s.BatteryPct
	}
	var batChg interface{}
	if s.BatteryCharging != nil {
		if *s.BatteryCharging {
			batChg = 1
		} else {
			batChg = 0
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO samples (ts, cpu_pct, mem_pct, mem_used, mem_total, disk_pct, disk_used, disk_total, battery_pct, battery_charging)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		s.TS, s.CPUPct, s.MemPct, s.MemUsed, s.MemTotal, s.DiskPct, s.DiskUsed, s.DiskTotal, batPct, batChg,
	); err != nil {
		return err
	}
	for sensor, v := range s.Temps {
		if _, err := tx.Exec(`INSERT INTO temp_samples (ts, sensor, value) VALUES (?,?,?)`, s.TS, sensor, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LatestSample returns the most recent stored sample (with its temps).
func (d *DB) LatestSample() (*Sample, error) {
	row := d.conn.QueryRow(
		`SELECT ts, cpu_pct, mem_pct, mem_used, mem_total, disk_pct, disk_used, disk_total, battery_pct, battery_charging
		 FROM samples ORDER BY ts DESC LIMIT 1`)
	s := Sample{Temps: map[string]float64{}}
	var batPct sql.NullFloat64
	var batChg sql.NullInt64
	err := row.Scan(&s.TS, &s.CPUPct, &s.MemPct, &s.MemUsed, &s.MemTotal, &s.DiskPct, &s.DiskUsed, &s.DiskTotal, &batPct, &batChg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if batPct.Valid {
		v := batPct.Float64
		s.BatteryPct = &v
	}
	if batChg.Valid {
		b := batChg.Int64 != 0
		s.BatteryCharging = &b
	}
	rows, err := d.conn.Query(`SELECT sensor, value FROM temp_samples WHERE ts = ?`, s.TS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var v float64
		if err := rows.Scan(&name, &v); err != nil {
			return nil, err
		}
		s.Temps[name] = v
	}
	return &s, rows.Err()
}

// Point is one [timestamp, value] pair in a downsampled series.
type Point struct {
	TS    int64   `json:"ts"`
	Value float64 `json:"value"`
}

// History is the bucketed time series returned for charts.
type History struct {
	Range   string             `json:"range"`
	Bucket  int64              `json:"bucket"`
	CPU     []Point            `json:"cpu"`
	Mem     []Point            `json:"mem"`
	Disk    []Point            `json:"disk"`
	Battery []Point            `json:"battery"`
	Temps   map[string][]Point `json:"temps"`
}

// QueryHistory returns metrics since `since` (unix seconds), averaged into
// time buckets of `bucket` seconds so charts stay light regardless of range.
func (d *DB) QueryHistory(rangeName string, since, bucket int64) (*History, error) {
	if bucket < 1 {
		bucket = 1
	}
	h := &History{Range: rangeName, Bucket: bucket, Temps: map[string][]Point{}}

	rows, err := d.conn.Query(
		`SELECT (ts/?)*? AS b, AVG(cpu_pct), AVG(mem_pct), AVG(disk_pct), AVG(battery_pct)
		 FROM samples WHERE ts >= ? GROUP BY b ORDER BY b`, bucket, bucket, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var cpu, mem, disk float64
		var bat sql.NullFloat64
		if err := rows.Scan(&b, &cpu, &mem, &disk, &bat); err != nil {
			return nil, err
		}
		h.CPU = append(h.CPU, Point{b, round1(cpu)})
		h.Mem = append(h.Mem, Point{b, round1(mem)})
		h.Disk = append(h.Disk, Point{b, round1(disk)})
		if bat.Valid {
			h.Battery = append(h.Battery, Point{b, round1(bat.Float64)})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	trows, err := d.conn.Query(
		`SELECT sensor, (ts/?)*? AS b, AVG(value)
		 FROM temp_samples WHERE ts >= ? GROUP BY sensor, b ORDER BY sensor, b`, bucket, bucket, since)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var sensor string
		var b int64
		var v float64
		if err := trows.Scan(&sensor, &b, &v); err != nil {
			return nil, err
		}
		h.Temps[sensor] = append(h.Temps[sensor], Point{b, round1(v)})
	}
	return h, trows.Err()
}

// Purge deletes samples older than the retention window (unix-seconds cutoff).
func (d *DB) Purge(before int64) error {
	if _, err := d.conn.Exec(`DELETE FROM samples WHERE ts < ?`, before); err != nil {
		return err
	}
	_, err := d.conn.Exec(`DELETE FROM temp_samples WHERE ts < ?`, before)
	return err
}

// --- Settings --------------------------------------------------------------

func (d *DB) GetSetting(key, def string) (string, error) {
	var v string
	err := d.conn.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	return v, nil
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.conn.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// --- Admin -----------------------------------------------------------------

func (d *DB) AdminExists() (bool, error) {
	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM admin`).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *DB) CreateAdmin(username, passwordHash string) error {
	_, err := d.conn.Exec(
		`INSERT INTO admin (id, username, password_hash, created_at) VALUES (1, ?, ?, ?)`,
		username, passwordHash, time.Now().Unix())
	return err
}

func (d *DB) GetAdmin() (username, passwordHash string, err error) {
	err = d.conn.QueryRow(`SELECT username, password_hash FROM admin WHERE id = 1`).
		Scan(&username, &passwordHash)
	return
}

// --- Secret ----------------------------------------------------------------

// GetOrCreateSecret returns the persisted value for key, generating it with
// `gen` on first use so JWT sessions survive restarts.
func (d *DB) GetOrCreateSecret(key string, gen func() string) (string, error) {
	var v string
	err := d.conn.QueryRow(`SELECT value FROM app_secret WHERE key = ?`, key).Scan(&v)
	if err == nil {
		return v, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	v = gen()
	if _, err := d.conn.Exec(`INSERT INTO app_secret (key, value) VALUES (?, ?)`, key, v); err != nil {
		return "", err
	}
	return v, nil
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
