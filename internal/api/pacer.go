package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Pacer enforces the ban-safety posture that is ON BY DEFAULT (not an option): one request in
// flight at a time, a jittered human-paced delay between requests, a per-run request cap, and a
// persisted per-DAY cap on job-detail fetches. These defaults exist because this CLI drives
// LinkedIn's unofficial Voyager API against a real personal account — the biggest risk isn't a
// bug, it's an account restriction from looking automated. See DECISIONS.md.
type Pacer struct {
	MinDelay  time.Duration // lower bound of the inter-request jitter
	MaxDelay  time.Duration // upper bound of the inter-request jitter
	PerRunCap int           // max live requests in one process run (0 = unlimited)
	DailyCap  int           // max job-detail fetches charged per calendar day (0 = unlimited)
	StatePath string        // JSON file persisting the daily counter (config/state dir)

	// seams for tests — never sleep for real, never touch a random source, in a unit test.
	sleep func(context.Context, time.Duration) error
	rand  func() float64

	mu       sync.Mutex
	runCount int
}

// DefaultPacer returns the shipped ban-safety defaults: a 3–15s jittered delay, no per-run cap,
// and ~30 job-detail fetches/day. statePath is where the daily counter persists.
func DefaultPacer(statePath string) *Pacer {
	return &Pacer{
		MinDelay:  3 * time.Second,
		MaxDelay:  15 * time.Second,
		PerRunCap: 0,
		DailyCap:  30,
		StatePath: statePath,
	}
}

func (p *Pacer) sleepFor(ctx context.Context, d time.Duration) error {
	if p.sleep != nil {
		return p.sleep(ctx, d)
	}
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (p *Pacer) randFloat() float64 {
	if p.rand != nil {
		return p.rand()
	}
	return rand.Float64() // #nosec G404 -- jitter timing, not a security boundary
}

// Wait blocks for a jittered delay before a live request and enforces the per-run cap. It is a
// no-op the FIRST time (no delay before the first request of a run) so a single command stays
// snappy; the pacing applies between successive requests. Returns an error if the per-run cap is
// exceeded, so a runaway loop stops rather than pounding LinkedIn.
func (p *Pacer) Wait(ctx context.Context) error {
	p.mu.Lock()
	n := p.runCount
	p.runCount++
	p.mu.Unlock()

	if p.PerRunCap > 0 && n >= p.PerRunCap {
		return fmt.Errorf("per-run request cap reached (%d) — the CLI paces requests to avoid looking "+
			"automated to LinkedIn; split the work across runs", p.PerRunCap)
	}
	if n == 0 {
		return ctx.Err() // no delay before the very first request
	}
	span := p.MaxDelay - p.MinDelay
	delay := p.MinDelay
	if span > 0 {
		delay += time.Duration(p.randFloat() * float64(span))
	}
	return p.sleepFor(ctx, delay)
}

// dailyState is the persisted per-day counter for job-detail fetches.
type dailyState struct {
	Date  string `json:"date"`  // YYYY-MM-DD (local)
	Count int    `json:"count"` // job-detail fetches charged today
}

// ChargeDaily records one job-detail fetch against today's cap, persisting the counter. It
// returns an error (and does NOT increment) when the cap is already reached, so `jobs get`
// refuses to exceed the daily budget rather than silently blowing past it. now is injected for
// deterministic tests.
func (p *Pacer) ChargeDaily(now time.Time) error {
	if p.DailyCap <= 0 || p.StatePath == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	today := now.Format("2006-01-02")
	st := p.loadState()
	if st.Date != today {
		st = dailyState{Date: today, Count: 0}
	}
	if st.Count >= p.DailyCap {
		return fmt.Errorf("daily job-detail cap reached (%d fetches today) — this ban-safety limit protects "+
			"your account; raise it with --daily-cap if you accept the risk, or resume tomorrow", p.DailyCap)
	}
	st.Count++
	p.saveState(st)
	return nil
}

// DailyRemaining reports how many job-detail fetches are left today (for doctor/status).
func (p *Pacer) DailyRemaining(now time.Time) int {
	if p.DailyCap <= 0 {
		return -1 // unlimited
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.loadState()
	if st.Date != now.Format("2006-01-02") {
		return p.DailyCap
	}
	if rem := p.DailyCap - st.Count; rem > 0 {
		return rem
	}
	return 0
}

func (p *Pacer) loadState() dailyState {
	var st dailyState
	b, err := os.ReadFile(p.StatePath) // #nosec G304 -- fixed path under the config/state dir
	if err != nil {
		return st
	}
	_ = json.Unmarshal(b, &st)
	return st
}

func (p *Pacer) saveState(st dailyState) {
	if err := os.MkdirAll(filepath.Dir(p.StatePath), 0o700); err != nil {
		return
	}
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.StatePath), ".state-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, p.StatePath)
}
