package model

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestResetChannelHealthClearsAllPersistentRuntimeState(t *testing.T) {
	setupTestDB(t)
	DB.Exec("DELETE FROM channel_health")
	DB.Exec("DELETE FROM channels")

	channel := &Channel{Name: "reset-test", Status: ChannelStatusEnabled, CooldownUntil: time.Now().Add(time.Hour).UnixMilli()}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	health := &ChannelHealth{
		ChannelId: channel.Id, ConsecutiveFailures: 4, ConsecutiveSuccesses: 2,
		TotalRequests: 12, FailedRequests: 7, LastSuccessAt: &now, LastFailureAt: &now,
		LastError: "upstream failed", CircuitState: CircuitOpen, CircuitOpenedAt: &now,
	}
	if err := DB.Create(health).Error; err != nil {
		t.Fatal(err)
	}

	if err := ResetChannelHealth(channel.Id, 8); err != nil {
		t.Fatal(err)
	}
	var gotChannel Channel
	if err := DB.First(&gotChannel, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	gotHealth, err := GetChannelHealth(channel.Id)
	if err != nil {
		t.Fatal(err)
	}
	if gotChannel.CooldownUntil != 0 {
		t.Fatalf("cooldown_until = %d", gotChannel.CooldownUntil)
	}
	if gotHealth.CircuitState != CircuitClosed || gotHealth.CircuitOpenedAt != nil || gotHealth.ConsecutiveFailures != 0 || gotHealth.ConsecutiveSuccesses != 0 || gotHealth.TotalRequests != 0 || gotHealth.FailedRequests != 0 || gotHealth.LastSuccessAt != nil || gotHealth.LastFailureAt != nil || gotHealth.LastError != "" || gotHealth.PersistVersion != 8 {
		t.Fatalf("health not fully reset: %+v", gotHealth)
	}
}

func TestResetChannelHealthRejectsMissingChannel(t *testing.T) {
	setupTestDB(t)
	DB.Exec("DELETE FROM channels")
	if err := ResetChannelHealth(987654, 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("error = %v, want record not found", err)
	}
}

func TestUpsertChannelHealthDropsOlderSnapshot(t *testing.T) {
	setupTestDB(t)
	DB.Exec("DELETE FROM channel_health")
	current := &ChannelHealth{ChannelId: 12345, CircuitState: CircuitClosed, PersistVersion: 2}
	if err := DB.Create(current).Error; err != nil {
		t.Fatal(err)
	}
	stale := &ChannelHealth{ChannelId: 12345, CircuitState: CircuitOpen, ConsecutiveFailures: 9, PersistVersion: 1}
	if err := UpsertChannelHealth(stale); err != nil {
		t.Fatal(err)
	}
	got, err := GetChannelHealth(12345)
	if err != nil {
		t.Fatal(err)
	}
	if got.CircuitState != CircuitClosed || got.ConsecutiveFailures != 0 || got.PersistVersion != 2 {
		t.Fatalf("stale snapshot overwrote reset: %+v", got)
	}
}
