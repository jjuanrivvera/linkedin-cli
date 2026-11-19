package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChargeDailySend_CapAndRollover(t *testing.T) {
	p := &Pacer{DailySendCap: 2, StatePath: filepath.Join(t.TempDir(), "state.json")}
	day1 := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)

	require.NoError(t, p.ChargeDailySend(day1))
	require.NoError(t, p.ChargeDailySend(day1))
	err := p.ChargeDailySend(day1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daily message-send cap reached")
	assert.Equal(t, 0, p.DailySendRemaining(day1))

	// A new calendar day resets the budget.
	day2 := day1.Add(24 * time.Hour)
	assert.Equal(t, 2, p.DailySendRemaining(day2))
	require.NoError(t, p.ChargeDailySend(day2))
}

// TestDailyCounters_Independent proves the send budget and the job-detail budget share one
// state file without stepping on each other (the generic counters extension).
func TestDailyCounters_Independent(t *testing.T) {
	p := &Pacer{DailyCap: 1, DailySendCap: 1, StatePath: filepath.Join(t.TempDir(), "state.json")}
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)

	require.NoError(t, p.ChargeDaily(now))
	require.Error(t, p.ChargeDaily(now), "job-detail budget spent")
	require.NoError(t, p.ChargeDailySend(now), "send budget must be unaffected")
	require.Error(t, p.ChargeDailySend(now))
	assert.Equal(t, 0, p.DailyRemaining(now))
	assert.Equal(t, 0, p.DailySendRemaining(now))
}

// TestDailyState_LegacyFileStillCounts pins backward compatibility: a pre-messaging
// state.json ({date,count}) keeps constraining the job-detail budget after the upgrade.
func TestDailyState_LegacyFileStillCounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	require.NoError(t, os.WriteFile(path, []byte(`{"date":"2026-07-22","count":30}`), 0o600))

	p := &Pacer{DailyCap: 30, DailySendCap: 20, StatePath: path}
	require.Error(t, p.ChargeDaily(now), "legacy count field must still be honored")
	assert.Equal(t, 20, p.DailySendRemaining(now), "send budget starts fresh")
	require.NoError(t, p.ChargeDailySend(now))
	require.Error(t, p.ChargeDaily(now), "charging the send counter must not reset the legacy one")
}

func TestChargeDailySend_UnlimitedWhenZero(t *testing.T) {
	p := &Pacer{DailySendCap: 0, StatePath: filepath.Join(t.TempDir(), "state.json")}
	now := time.Now()
	require.NoError(t, p.ChargeDailySend(now))
	assert.Equal(t, -1, p.DailySendRemaining(now))
}
