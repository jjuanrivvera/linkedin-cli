package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPacer_WaitNoDelayFirstRequest(t *testing.T) {
	p := &Pacer{MinDelay: time.Hour, MaxDelay: time.Hour} // huge delay, but first Wait is free
	require.NoError(t, p.Wait(t.Context()))
}

func TestPacer_WaitDelaysSubsequent(t *testing.T) {
	slept := []time.Duration{}
	p := &Pacer{MinDelay: 3 * time.Second, MaxDelay: 3 * time.Second}
	p.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	require.NoError(t, p.Wait(t.Context())) // free
	require.NoError(t, p.Wait(t.Context())) // delayed
	require.Len(t, slept, 1)
	assert.Equal(t, 3*time.Second, slept[0])
}

func TestPacer_PerRunCap(t *testing.T) {
	p := &Pacer{PerRunCap: 2}
	p.sleep = func(context.Context, time.Duration) error { return nil }
	require.NoError(t, p.Wait(t.Context()))
	require.NoError(t, p.Wait(t.Context()))
	err := p.Wait(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "per-run request cap")
}

func TestPacer_WaitContextCancelled(t *testing.T) {
	p := &Pacer{MinDelay: time.Hour, MaxDelay: time.Hour}
	ctx, cancel := context.WithCancel(t.Context())
	require.NoError(t, p.Wait(ctx)) // first free
	cancel()
	err := p.Wait(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPacer_ChargeDaily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	p := &Pacer{DailyCap: 2, StatePath: path}
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	require.NoError(t, p.ChargeDaily(now))
	assert.Equal(t, 1, p.DailyRemaining(now))
	require.NoError(t, p.ChargeDaily(now))
	assert.Equal(t, 0, p.DailyRemaining(now))
	err := p.ChargeDaily(now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily job-detail cap")

	// A new day resets the counter.
	tomorrow := now.Add(24 * time.Hour)
	assert.Equal(t, 2, p.DailyRemaining(tomorrow))
	require.NoError(t, p.ChargeDaily(tomorrow))
}

func TestPacer_ChargeDaily_PersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	p1 := &Pacer{DailyCap: 5, StatePath: path}
	require.NoError(t, p1.ChargeDaily(now))
	p2 := &Pacer{DailyCap: 5, StatePath: path}
	assert.Equal(t, 4, p2.DailyRemaining(now))
}

func TestPacer_UnlimitedAndDisabled(t *testing.T) {
	p := &Pacer{DailyCap: 0}
	require.NoError(t, p.ChargeDaily(time.Now()))
	assert.Equal(t, -1, p.DailyRemaining(time.Now()))
}

func TestDefaultPacer(t *testing.T) {
	p := DefaultPacer("/x/state.json")
	assert.Equal(t, 30, p.DailyCap)
	assert.Equal(t, 3*time.Second, p.MinDelay)
	assert.Equal(t, 15*time.Second, p.MaxDelay)
}
