package router

import (
	"errors"
	"testing"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
)

type mockChannelRepo struct {
	channels []model.Channel
	err      error
}

func (m *mockChannelRepo) GetEnabled() ([]model.Channel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.channels, nil
}

type mockProxyConfigRepo struct {
	config *model.ProxyConfig
	err    error
}

func (m *mockProxyConfigRepo) GetProxyConfig() (*model.ProxyConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.config == nil {
		return &model.ProxyConfig{Enabled: true, AutoFailoverEnabled: true}, nil
	}
	return m.config, nil
}

type mockFailoverQueueRepo struct {
	items []model.FailoverQueueItem
	err   error
}

func (m *mockFailoverQueueRepo) GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockProviderHealthRepo struct {
	health map[uint]*model.ProviderHealth
}

func (m *mockProviderHealthRepo) GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	if m.health == nil {
		m.health = make(map[uint]*model.ProviderHealth)
	}
	h, ok := m.health[channelID]
	if !ok {
		h = &model.ProviderHealth{ChannelID: channelID, IsHealthy: true}
		m.health[channelID] = h
	}
	return h, nil
}

func (m *mockProviderHealthRepo) UpdateProviderHealth(health *model.ProviderHealth) error {
	if m.health == nil {
		m.health = make(map[uint]*model.ProviderHealth)
	}
	m.health[health.ChannelID] = health
	return nil
}

func TestSelectProvidersUsesGlobalFailoverQueueOrder(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Name: "first", Priority: 10, Enabled: true},
		{ID: 2, Name: "second", Priority: 20, Enabled: true},
		{ID: 3, Name: "third", Priority: 5, Enabled: true},
	}
	queue := []model.FailoverQueueItem{
		{ChannelID: 3, Position: 1},
		{ChannelID: 1, Position: 2},
	}

	router := newMockRouter(channels, queue, nil)
	providers, err := router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error: %v", err)
	}
	assertChannelOrder(t, providers, []uint{3, 1, 2})
}

func TestSelectProvidersWithoutFailoverReturnsHighestPriority(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Name: "low", Priority: 1, Enabled: true},
		{ID: 2, Name: "high", Priority: 100, Enabled: true},
	}
	config := &model.ProxyConfig{Enabled: true, AutoFailoverEnabled: false}

	router := newMockRouterWithConfig(channels, nil, config, nil)
	providers, err := router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(providers))
	}
	if providers[0].ID != 2 {
		t.Fatalf("selected channel ID = %d, want 2 (highest priority)", providers[0].ID)
	}
}

func TestSelectProvidersSkipsOpenCircuit(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Name: "first", Priority: 10, Enabled: true},
		{ID: 2, Name: "second", Priority: 9, Enabled: true},
	}
	breaker := circuit.NewBreaker(1, 1, time.Minute)
	breaker.RecordFailure(1, false)

	router := newMockRouter(channels, nil, breaker)
	providers, err := router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error: %v", err)
	}
	assertChannelOrder(t, providers, []uint{2})
}

func TestResetCircuitReallowsProvider(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Name: "first", Priority: 10, Enabled: true},
	}
	breaker := circuit.NewBreaker(1, 1, time.Minute)
	router := newMockRouter(channels, nil, breaker)

	router.RecordFailure(1, "boom")
	providers, err := router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("providers len = %d, want 0 while circuit open", len(providers))
	}

	router.ResetCircuit(1)
	providers, err = router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error after reset: %v", err)
	}
	assertChannelOrder(t, providers, []uint{1})
}

func TestSelectProvidersReturnsEmptyWhenProxyDisabled(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Name: "channel", Priority: 10, Enabled: true},
	}
	config := &model.ProxyConfig{Enabled: false}

	router := newMockRouterWithConfig(channels, nil, config, nil)
	providers, err := router.SelectProviders()
	if err != nil {
		t.Fatalf("SelectProviders returned error: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("providers len = %d, want 0 when proxy disabled", len(providers))
	}
}

func TestSelectProvidersReturnsErrorWhenChannelRepoFails(t *testing.T) {
	router := &ProviderRouter{
		channels:    &mockChannelRepo{err: errors.New("db error")},
		proxyConfig: &mockProxyConfigRepo{},
	}
	_, err := router.SelectProviders()
	if err == nil {
		t.Fatal("SelectProviders should return error when channel repo fails")
	}
}

func newMockRouter(channels []model.Channel, queue []model.FailoverQueueItem, breaker *circuit.Breaker) *ProviderRouter {
	return newMockRouterWithConfig(channels, queue, nil, breaker)
}

func newMockRouterWithConfig(channels []model.Channel, queue []model.FailoverQueueItem, config *model.ProxyConfig, breaker *circuit.Breaker) *ProviderRouter {
	return &ProviderRouter{
		channels:       &mockChannelRepo{channels: channels},
		proxyConfig:    &mockProxyConfigRepo{config: config},
		failoverQueue:  &mockFailoverQueueRepo{items: queue},
		providerHealth: &mockProviderHealthRepo{},
		breaker:        breaker,
	}
}

func assertChannelOrder(t *testing.T, providers []model.Channel, want []uint) {
	t.Helper()
	if len(providers) != len(want) {
		t.Fatalf("providers len = %d, want %d", len(providers), len(want))
	}
	for index, channelID := range want {
		if providers[index].ID != channelID {
			t.Fatalf("providers[%d].ID = %d, want %d", index, providers[index].ID, channelID)
		}
	}
}
