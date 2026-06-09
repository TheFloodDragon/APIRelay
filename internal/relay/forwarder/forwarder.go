package forwarder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/TheFloodDragon/APIRelay/internal/relay/router"
)

type Attempt struct {
	Channel *model.Channel
	Request *http.Request
	Meta    any
}

type AttemptBuilder func(ctx *RequestContext, provider model.Channel) (*Attempt, error)

type ResponsePreflight func(resp *http.Response, attempt *Attempt) error

type RetryableError interface {
	Retryable() bool
}

type AttemptError struct {
	Attempt *Attempt
	Err     error
}

func (e *AttemptError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *AttemptError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Forwarder struct {
	router     *router.ProviderRouter
	httpClient *http.Client
	maxRetries int
	build      AttemptBuilder
	preflight  ResponsePreflight
}

func NewForwarder(providerRouter *router.ProviderRouter, httpClient *http.Client, maxRetries int) *Forwarder {
	return NewForwarderWithBuilder(providerRouter, httpClient, maxRetries, DefaultAttemptBuilder, nil)
}

func NewForwarderWithBuilder(providerRouter *router.ProviderRouter, httpClient *http.Client, maxRetries int, build AttemptBuilder, preflight ResponsePreflight) *Forwarder {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if build == nil {
		build = DefaultAttemptBuilder
	}
	return &Forwarder{
		router:     providerRouter,
		httpClient: httpClient,
		maxRetries: maxRetries,
		build:      build,
		preflight:  preflight,
	}
}

func (f *Forwarder) Forward(ctx *RequestContext) (*http.Response, *Attempt, error) {
	return f.ForwardWithBuilder(ctx, f.build, f.preflight)
}

func (f *Forwarder) ForwardWithBuilder(ctx *RequestContext, build AttemptBuilder, preflight ResponsePreflight) (*http.Response, *Attempt, error) {
	if f == nil || f.router == nil {
		return nil, nil, errors.New("provider router is not configured")
	}
	if ctx == nil {
		return nil, nil, errors.New("request context is nil")
	}
	if build == nil {
		build = f.build
	}
	if build == nil {
		build = DefaultAttemptBuilder
	}

	providers, err := f.router.SelectProviders()
	if err != nil {
		return nil, nil, fmt.Errorf("select providers: %w", err)
	}
	if len(providers) == 0 {
		return nil, nil, errors.New("no available providers")
	}

	maxAttempts := f.maxAttempts()
	actualAttempts := 0
	var lastAttempt *Attempt
	var lastErr error

	for actualAttempts < maxAttempts {
		progressed := false
		for _, provider := range providers {
			if actualAttempts >= maxAttempts {
				break
			}
			allow := f.router.Allow(provider.ID)
			if !allow.Allowed {
				continue
			}

			resp, attempt, err := f.tryProvider(ctx, provider, allow, build, preflight)
			lastAttempt = attempt
			if attempt != nil && attempt.Request != nil {
				actualAttempts++
				progressed = true
			}
			if err == nil {
				return resp, attempt, nil
			}

			if isRetryableError(err) {
				f.router.RecordFailureWithPermit(provider.ID, err.Error(), allow.UsedHalfOpenPermit)
			}
			lastErr = err
		}
		if !progressed {
			break
		}
	}

	if lastErr == nil {
		lastErr = errors.New("all providers failed without specific error")
	}
	return nil, lastAttempt, lastErr
}

func (f *Forwarder) maxAttempts() int {
	maxRetries := f.maxRetries
	if f != nil && f.router != nil {
		if config, err := f.router.GetProxyConfig(); err == nil && config != nil {
			maxRetries = config.MaxRetries
		}
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return maxRetries + 1
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.Retryable()
	}
	return true
}

func (f *Forwarder) tryProvider(ctx *RequestContext, provider model.Channel, allow circuit.AllowResult, build AttemptBuilder, preflight ResponsePreflight) (*http.Response, *Attempt, error) {
	attempt, err := build(ctx, provider)
	if err != nil {
		return nil, attempt, &AttemptError{Attempt: attempt, Err: fmt.Errorf("build request: %w", err)}
	}
	if attempt == nil || attempt.Request == nil {
		return nil, attempt, &AttemptError{Attempt: attempt, Err: errors.New("build request returned nil request")}
	}
	if attempt.Channel == nil {
		attempt.Channel = &provider
	}
	_ = allow

	resp, err := f.httpClient.Do(attempt.Request)
	if err != nil {
		return nil, attempt, &AttemptError{Attempt: attempt, Err: fmt.Errorf("http request: %w", err)}
	}

	if preflight != nil {
		if err := preflight(resp, attempt); err != nil {
			_ = resp.Body.Close()
			return resp, attempt, &AttemptError{Attempt: attempt, Err: err}
		}
		return resp, attempt, nil
	}
	if err := Preflight(resp); err != nil {
		_ = resp.Body.Close()
		return resp, attempt, &AttemptError{Attempt: attempt, Err: err}
	}

	return resp, attempt, nil
}

func DefaultAttemptBuilder(ctx *RequestContext, provider model.Channel) (*Attempt, error) {
	url := provider.BaseURL + ctx.Endpoint
	if ctx.Query != "" {
		url += "?" + ctx.Query
	}

	var body io.Reader
	if len(ctx.RawBody) > 0 {
		body = bytes.NewReader(ctx.RawBody)
	}

	req, err := http.NewRequestWithContext(ctx.Gin.Request.Context(), ctx.Method, url, body)
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, ctx.Headers)
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	providerCopy := provider
	return &Attempt{Channel: &providerCopy, Request: req}, nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
