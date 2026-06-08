package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
)

func TestHealthCheckerMarksUnhealthyAfterThreshold(t *testing.T) {
	failureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusInternalServerError)
	}))
	defer failureServer.Close()

	h := NewHealthChecker(nil, 60, 2)
	channel := &model.Channel{
		ID:           1,
		Name:         "failing-openai",
		Type:         "openai",
		APIKey:       "sk-test",
		BaseURL:      failureServer.URL,
		HealthStatus: "healthy",
	}

	if status := h.statusAfterCheck(channel); status != "healthy" {
		t.Fatalf("first failure should keep current status before threshold, got %q", status)
	}
	if failures := h.failureCounts[channel.ID]; failures != 1 {
		t.Fatalf("expected 1 recorded failure, got %d", failures)
	}

	if status := h.statusAfterCheck(channel); status != "unhealthy" {
		t.Fatalf("second consecutive failure should mark unhealthy, got %q", status)
	}
	if failures := h.failureCounts[channel.ID]; failures != 2 {
		t.Fatalf("expected 2 recorded failures, got %d", failures)
	}
}

func TestHealthCheckerResetsFailuresAfterSuccess(t *testing.T) {
	failureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusInternalServerError)
	}))
	defer failureServer.Close()

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer successServer.Close()

	h := NewHealthChecker(nil, 60, 2)
	channel := &model.Channel{
		ID:           1,
		Name:         "recovering-openai",
		Type:         "openai",
		APIKey:       "sk-test",
		BaseURL:      failureServer.URL,
		HealthStatus: "healthy",
	}

	_ = h.statusAfterCheck(channel)
	_ = h.statusAfterCheck(channel)
	if failures := h.failureCounts[channel.ID]; failures != 2 {
		t.Fatalf("expected failures before recovery, got %d", failures)
	}

	channel.BaseURL = successServer.URL
	channel.HealthStatus = "unhealthy"
	if status := h.statusAfterCheck(channel); status != "healthy" {
		t.Fatalf("successful check should mark healthy, got %q", status)
	}
	if _, ok := h.failureCounts[channel.ID]; ok {
		t.Fatal("successful check should clear failure count")
	}
}
