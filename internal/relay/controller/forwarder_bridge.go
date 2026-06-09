package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/TheFloodDragon/APIRelay/internal/relay/forwarder"
	providerrouter "github.com/TheFloodDragon/APIRelay/internal/relay/router"
)

type nonRetryableBuildError struct {
	statusCode int
	message    string
}

func newNonRetryableBuildError(statusCode int, message string) error {
	return &nonRetryableBuildError{statusCode: statusCode, message: message}
}

func (e *nonRetryableBuildError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *nonRetryableBuildError) Retryable() bool {
	return false
}

type relayUpstreamError struct {
	statusCode int
	message    string
	retryable  bool
}

func (e *relayUpstreamError) Error() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	return fmt.Sprintf("upstream returned status %d", e.statusCode)
}

func (e *relayUpstreamError) Retryable() bool {
	if e == nil {
		return true
	}
	return e.retryable
}

func (rc *RelayController) forwardRelayAttempt(
	reqCtx *RequestContext,
	isStream bool,
	build func(model.Channel) (*RelayAttempt, error),
	preflight forwarder.ResponsePreflight,
) (*http.Response, *RelayAttempt, error) {
	if rc == nil {
		return nil, nil, errors.New("relay controller is not configured")
	}
	relayForwarder := rc.forwarderForRequest(reqCtx)
	if relayForwarder == nil {
		return nil, nil, errors.New("forwarder is not configured")
	}

	builder := func(_ *forwarder.RequestContext, provider model.Channel) (*forwarder.Attempt, error) {
		attempt, err := build(provider)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(reqCtx.Gin.Request.Context(), reqCtx.Method, attempt.URL, bytes.NewReader(attempt.ConvertedBody))
		if err != nil {
			return nil, err
		}
		for key, values := range attempt.Headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		return &forwarder.Attempt{
			Channel: attempt.Info.Channel,
			Request: req,
			Meta:    attempt,
		}, nil
	}

	resp, fwdAttempt, err := relayForwarder.ForwardWithBuilder(reqCtx.forwarderRequestContext(), builder, preflight)
	attempt := relayAttemptFromForwarderAttempt(fwdAttempt)
	if err != nil {
		var attemptErr *forwarder.AttemptError
		if errors.As(err, &attemptErr) && attempt == nil {
			attempt = relayAttemptFromForwarderAttempt(attemptErr.Attempt)
		}
	}
	_ = isStream
	return resp, attempt, err
}

func (rc *RelayController) forwarderForRequest(reqCtx *RequestContext) *forwarder.Forwarder {
	if rc == nil {
		return nil
	}
	if rc.forwarder != nil {
		return rc.forwarder
	}
	if reqCtx == nil || len(reqCtx.Candidates) == 0 {
		return nil
	}
	providers := make([]model.Channel, 0, len(reqCtx.Candidates))
	queue := make([]model.FailoverQueueItem, 0, len(reqCtx.Candidates))
	seen := make(map[uint]struct{}, len(reqCtx.Candidates))
	for index, candidate := range reqCtx.Candidates {
		if candidate.Channel.ID == 0 {
			continue
		}
		if _, exists := seen[candidate.Channel.ID]; exists {
			continue
		}
		seen[candidate.Channel.ID] = struct{}{}
		providers = append(providers, candidate.Channel)
		queue = append(queue, model.FailoverQueueItem{ChannelID: candidate.Channel.ID, Position: index + 1})
	}
	if len(providers) == 0 {
		return nil
	}
	providerRouter := providerrouter.NewProviderRouter(
		&staticChannelRepo{channels: providers},
		&staticProxyConfigRepo{maxRetries: len(providers) - 1},
		&staticFailoverQueueRepo{items: queue},
		&staticProviderHealthRepo{},
		circuit.NewBreaker(3, 1, 30*time.Second),
	)
	httpClient := http.DefaultClient
	if rc.httpClient != nil {
		httpClient = rc.httpClient.Client()
	}
	return forwarder.NewForwarderWithBuilder(providerRouter, httpClient, len(providers)-1, nil, nil)
}

func upstreamRequestSummary(attempt *RelayAttempt) string {
	if attempt == nil || len(attempt.ConvertedBody) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(attempt.ConvertedBody, &payload); err != nil {
		return ""
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	summary := map[string]interface{}{"fields": keys}
	for _, key := range []string{"model", "stream", "max_tokens", "max_completion_tokens"} {
		if value, ok := payload[key]; ok {
			summary[key] = value
		}
	}
	if messages, ok := payload["messages"].([]interface{}); ok {
		summary["messages"] = len(messages)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return ""
	}
	return string(data)
}

func relayAttemptFromForwarderAttempt(attempt *forwarder.Attempt) *RelayAttempt {
	if attempt == nil || attempt.Meta == nil {
		return nil
	}
	relayAttempt, _ := attempt.Meta.(*RelayAttempt)
	return relayAttempt
}

func relayJSONPreflight(resp *http.Response, attempt *forwarder.Attempt) error {
	if isSuccessfulStatus(resp.StatusCode) {
		return nil
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	relayAttempt := relayAttemptFromForwarderAttempt(attempt)
	message := string(body)
	if relayAttempt != nil && relayAttempt.ProtocolAdaptor != nil {
		if adaptorMessage := relayAttempt.ProtocolAdaptor.ErrorMessage(body); adaptorMessage != "" {
			message = adaptorMessage
		}
	}
	if summary := upstreamRequestSummary(relayAttempt); summary != "" {
		message += "; request=" + summary
	}
	return &relayUpstreamError{
		statusCode: resp.StatusCode,
		message:    message,
		retryable:  shouldTryNextCandidate(resp.StatusCode, nil),
	}
}

func relayStreamPreflight(resp *http.Response, attempt *forwarder.Attempt) error {
	if isSuccessfulStatus(resp.StatusCode) {
		relayAttempt := relayAttemptFromForwarderAttempt(attempt)
		timeout := time.Second
		if relayAttempt != nil && relayAttempt.Info != nil {
			timeout = timeoutForChannel(relayAttempt.Info.Channel)
		}
		preparedBody, err := prepareStreamBody(attempt.Request.Context(), resp.Body, timeout)
		if err != nil {
			return err
		}
		resp.Body = preparedBody
		return nil
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	relayAttempt := relayAttemptFromForwarderAttempt(attempt)
	message := string(body)
	if relayAttempt != nil && relayAttempt.ProtocolAdaptor != nil {
		if adaptorMessage := relayAttempt.ProtocolAdaptor.ErrorMessage(body); adaptorMessage != "" {
			message = adaptorMessage
		}
	}
	if summary := upstreamRequestSummary(relayAttempt); summary != "" {
		message += "; request=" + summary
	}
	return &relayUpstreamError{
		statusCode: resp.StatusCode,
		message:    message,
		retryable:  shouldTryNextCandidate(resp.StatusCode, nil),
	}
}
