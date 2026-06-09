package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/gin-gonic/gin"
)

type mockProxyConfigStore struct {
	config *model.ProxyConfig
	saved  *model.ProxyConfig
}

func (m *mockProxyConfigStore) GetProxyConfig() (*model.ProxyConfig, error) {
	if m.config == nil {
		return &model.ProxyConfig{Enabled: true, AutoFailoverEnabled: true, MaxRetries: 1}, nil
	}
	copy := *m.config
	return &copy, nil
}

func (m *mockProxyConfigStore) SaveProxyConfig(config *model.ProxyConfig) error {
	copy := *config
	m.saved = &copy
	m.config = &copy
	return nil
}

type mockFailoverQueueStore struct {
	items []model.FailoverQueueItem
	saved []uint
}

func (m *mockFailoverQueueStore) GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	return append([]model.FailoverQueueItem(nil), m.items...), nil
}

func (m *mockFailoverQueueStore) SaveFailoverQueue(channelIDs []uint) error {
	m.saved = append([]uint(nil), channelIDs...)
	m.items = make([]model.FailoverQueueItem, 0, len(channelIDs))
	for index, channelID := range channelIDs {
		m.items = append(m.items, model.FailoverQueueItem{ChannelID: channelID, Position: index + 1})
	}
	return nil
}

type mockProxyChannelStore struct {
	channels []model.Channel
}

func (m *mockProxyChannelStore) GetAll() ([]model.Channel, error) {
	return append([]model.Channel(nil), m.channels...), nil
}

type mockProxyHealthStore struct {
	health map[uint]*model.ProviderHealth
}

func (m *mockProxyHealthStore) GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	if m.health == nil {
		return &model.ProviderHealth{ChannelID: channelID, IsHealthy: true}, nil
	}
	if health, ok := m.health[channelID]; ok {
		copy := *health
		return &copy, nil
	}
	return &model.ProviderHealth{ChannelID: channelID, IsHealthy: true}, nil
}

type mockCircuitManager struct {
	snapshots     []circuit.Snapshot
	resetChannel  uint
	appliedConfig *model.ProxyConfig
}

func (m *mockCircuitManager) CircuitSnapshots(channelIDs []uint) []circuit.Snapshot {
	if len(m.snapshots) > 0 {
		return append([]circuit.Snapshot(nil), m.snapshots...)
	}
	result := make([]circuit.Snapshot, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		result = append(result, circuit.Snapshot{ChannelID: channelID, State: circuit.CircuitStateClosed})
	}
	return result
}

func (m *mockCircuitManager) ResetCircuit(channelID uint) {
	m.resetChannel = channelID
}

func (m *mockCircuitManager) ApplyProxyConfig(config *model.ProxyConfig) {
	copy := *config
	m.appliedConfig = &copy
}

func TestProxyHandlerUpdateConfigWritesGlobalConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configRepo := &mockProxyConfigStore{config: &model.ProxyConfig{Enabled: true, AutoFailoverEnabled: true, MaxRetries: 1, CircuitFailureThreshold: 3, CircuitSuccessThreshold: 1, CircuitOpenSeconds: 30}}
	circuits := &mockCircuitManager{}
	handler := NewProxyHandlerWithStores(configRepo, nil, nil, nil, circuits)
	r := gin.New()
	r.PUT("/api/proxy/config", handler.UpdateConfig)

	req := httptest.NewRequest(http.MethodPut, "/api/proxy/config", bytes.NewReader([]byte(`{"enabled":false,"max_retries":4,"circuit_failure_threshold":5}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if configRepo.saved == nil {
		t.Fatal("config was not saved")
	}
	if configRepo.saved.Enabled || configRepo.saved.MaxRetries != 4 || configRepo.saved.CircuitFailureThreshold != 5 {
		t.Fatalf("saved config = %#v", configRepo.saved)
	}
	if circuits.appliedConfig == nil || circuits.appliedConfig.CircuitFailureThreshold != 5 {
		t.Fatalf("runtime circuit config was not applied: %#v", circuits.appliedConfig)
	}
}

func TestProxyHandlerUpdateFailoverQueueWritesGlobalQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	queueRepo := &mockFailoverQueueStore{}
	handler := NewProxyHandlerWithStores(nil, queueRepo, nil, nil, nil)
	r := gin.New()
	r.PUT("/api/proxy/failover-queue", handler.UpdateFailoverQueue)

	req := httptest.NewRequest(http.MethodPut, "/api/proxy/failover-queue", bytes.NewReader([]byte(`{"channel_ids":[3,1,3,0,2]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	want := []uint{3, 1, 2}
	if len(queueRepo.saved) != len(want) {
		t.Fatalf("saved queue len = %d, want %d", len(queueRepo.saved), len(want))
	}
	for index, wantID := range want {
		if queueRepo.saved[index] != wantID {
			t.Fatalf("saved[%d] = %d, want %d", index, queueRepo.saved[index], wantID)
		}
	}
}

func TestProxyHandlerResetCircuitUsesChannelIDOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	circuits := &mockCircuitManager{}
	handler := NewProxyHandlerWithStores(nil, nil, nil, nil, circuits)
	r := gin.New()
	r.POST("/api/proxy/circuits/:channel_id/reset", handler.ResetCircuit)

	req := httptest.NewRequest(http.MethodPost, "/api/proxy/circuits/9/reset", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if circuits.resetChannel != 9 {
		t.Fatalf("reset channel = %d, want 9", circuits.resetChannel)
	}
}

func TestProxyHandlerGetCircuitsReturnsHealthAndMasksAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	channels := &mockProxyChannelStore{channels: []model.Channel{{ID: 7, Name: "primary", Type: "openai", APIKey: "secret", BaseURL: "https://example.test", Enabled: true}}}
	health := &mockProxyHealthStore{health: map[uint]*model.ProviderHealth{7: {ChannelID: 7, IsHealthy: false, ConsecutiveFailures: 2}}}
	circuits := &mockCircuitManager{snapshots: []circuit.Snapshot{{ChannelID: 7, State: circuit.CircuitStateOpen, ConsecutiveFailures: 3}}}
	handler := NewProxyHandlerWithStores(nil, nil, channels, health, circuits)
	r := gin.New()
	r.GET("/api/proxy/circuits", handler.GetCircuits)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy/circuits", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("secret")) {
		t.Fatalf("response leaked api key: %s", resp.Body.String())
	}
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			ChannelID uint `json:"channel_id"`
			Circuit   struct {
				State string `json:"state"`
			} `json:"circuit"`
			Health struct {
				IsHealthy bool `json:"is_healthy"`
			} `json:"health"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !payload.Success || len(payload.Data) != 1 || payload.Data[0].ChannelID != 7 || payload.Data[0].Circuit.State != string(circuit.CircuitStateOpen) || payload.Data[0].Health.IsHealthy {
		t.Fatalf("payload = %#v", payload)
	}
}
