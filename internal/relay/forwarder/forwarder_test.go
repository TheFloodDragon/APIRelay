package forwarder

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/TheFloodDragon/APIRelay/internal/relay/router"
	"github.com/gin-gonic/gin"
)

type staticChannelRepo struct {
	channels []model.Channel
}

func (r *staticChannelRepo) GetEnabled() ([]model.Channel, error) {
	return r.channels, nil
}

type staticProxyConfigRepo struct{}

func (r *staticProxyConfigRepo) GetProxyConfig() (*model.ProxyConfig, error) {
	return &model.ProxyConfig{Enabled: true, AutoFailoverEnabled: true, MaxRetries: 3, CircuitFailureThreshold: 3, CircuitSuccessThreshold: 1, CircuitOpenSeconds: 30}, nil
}

type staticQueueRepo struct{}

func (r *staticQueueRepo) GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	return []model.FailoverQueueItem{{ChannelID: 1, Position: 1}, {ChannelID: 2, Position: 2}}, nil
}

type staticHealthRepo struct{}

func (r *staticHealthRepo) GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	return &model.ProviderHealth{ChannelID: channelID, IsHealthy: true}, nil
}

func (r *staticHealthRepo) UpdateProviderHealth(health *model.ProviderHealth) error {
	return nil
}

type nonRetryableTestError struct{}

func (e nonRetryableTestError) Error() string { return "non retryable" }
func (e nonRetryableTestError) Retryable() bool { return false }

func TestForwardWithBuilderStopsOnNonRetryableError(t *testing.T) {
	providerRouter := router.NewProviderRouter(
		&staticChannelRepo{channels: []model.Channel{{ID: 1, Name: "first", Enabled: true}, {ID: 2, Name: "second", Enabled: true}}},
		&staticProxyConfigRepo{},
		&staticQueueRepo{},
		&staticHealthRepo{},
		circuit.NewBreaker(3, 1, 0),
	)
	forwarder := NewForwarderWithBuilder(providerRouter, http.DefaultClient, 3, nil, nil)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := &RequestContext{Gin: ginCtx}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	attempts := 0
	_, _, err := forwarder.ForwardWithBuilder(ctx, func(_ *RequestContext, provider model.Channel) (*Attempt, error) {
		attempts++
		req, err := http.NewRequest(http.MethodPost, server.URL, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		providerCopy := provider
		return &Attempt{Channel: &providerCopy, Request: req}, nil
	}, func(resp *http.Response, attempt *Attempt) error {
		return nonRetryableTestError{}
	})
	if err == nil {
		t.Fatal("ForwardWithBuilder returned nil error, want non-retryable error")
	}
	var nonRetryable nonRetryableTestError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("error = %T %v, want nonRetryableTestError", err, err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
