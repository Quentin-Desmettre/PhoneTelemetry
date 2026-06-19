package metrics

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"phonedashboard/internal/db"
)

// Collector samples the device metrics at a configurable interval and stores
// each snapshot in the database. The interval can be changed at runtime.
type Collector struct {
	store      *db.DB
	intervalNs atomic.Int64 // time.Duration as nanoseconds
	reset      chan struct{}
}

func NewCollector(store *db.DB, interval time.Duration) *Collector {
	c := &Collector{store: store, reset: make(chan struct{}, 1)}
	c.intervalNs.Store(int64(interval))
	return c
}

// SetInterval updates the sampling cadence and applies it immediately.
func (c *Collector) SetInterval(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	c.intervalNs.Store(int64(d))
	select {
	case c.reset <- struct{}{}:
	default:
	}
}

// Run blocks, collecting until ctx is cancelled. CPU% is a delta between ticks,
// so we prime the counter once before the first real sample.
func (c *Collector) Run(ctx context.Context) {
	PrimeCPU()
	for {
		d := time.Duration(c.intervalNs.Load())
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-c.reset:
			timer.Stop()
			continue
		case <-timer.C:
		}
		c.collectOnce()
	}
}

func (c *Collector) collectOnce() {
	s := db.Sample{TS: time.Now().Unix(), Temps: map[string]float64{}}

	if v, err := ReadCPU(); err == nil {
		s.CPUPct = v
	}
	if m, err := ReadMem(); err == nil {
		s.MemPct, s.MemUsed, s.MemTotal = m.Pct, m.Used, m.Total
	}
	if dk, err := ReadDisk(); err == nil {
		s.DiskPct, s.DiskUsed, s.DiskTotal = dk.Pct, dk.Used, dk.Total
	}
	if b := ReadBattery(); b.Present {
		pct := b.Pct
		chg := b.Charging
		s.BatteryPct = &pct
		s.BatteryCharging = &chg
	}
	s.Temps = ReadTemps()

	if err := c.store.InsertSample(s); err != nil {
		log.Printf("collector: insert sample failed: %v", err)
	}
}
