package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	relayclient "github.com/TheFloodDragon/APIRelay/internal/relay/client"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestPrepareStreamBodyReplaysFirstByte(t *testing.T) {
	body := io.NopCloser(strings.NewReader("data: hello\n\n"))
	prepared, err := prepareStreamBody(context.Background(), body, time.Second)
	if err != nil {
		t.Fatalf("prepareStreamBody returned error: %v", err)
	}
	defer prepared.Close()

	got, err := io.ReadAll(prepared)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != "data: hello\n\n" {
		t.Fatalf("body = %q, want full stream body", string(got))
	}
}

func TestPrepareStreamBodyTimeoutClosesBody(t *testing.T) {
	body := newBlockingReadCloser()
	prepared, err := prepareStreamBody(context.Background(), body, 20*time.Millisecond)
	if err == nil {
		_ = prepared.Close()
		t.Fatal("prepareStreamBody returned nil error, want timeout")
	}
	if !body.closedWithin(time.Second) {
		t.Fatal("body was not closed after first byte timeout")
	}
}

func TestRelayStreamFallsBackWhenFirstCandidateHasNoFirstByte(t *testing.T) {
	gin.SetMode(gin.TestMode)

	blocked := make(chan struct{})
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-blocked
	}))
	defer first.Close()
	defer close(blocked)

	secondCalled := false
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: second\n\n"))
	}))
	defer second.Close()

	writer := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(writer)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)))

	reqCtx := &RequestContext{
		Gin:       ginCtx,
		RequestID: "stream-test",
		StartTime: time.Now(),
		App:       constant.RelayAppOpenAI,
		Mode:      constant.RelayModeChatCompletions,
		Format:    constant.RelayFormatOpenAI,
		Method:    http.MethodPost,
		Body:      []byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Meta:      relayRequestMeta{Model: "gpt-test", Stream: true},
		Candidates: []relayCandidate{
			{Channel: model.Channel{ID: 1, Type: "openai", BaseURL: first.URL, APIKey: "sk-first", Timeout: 20}, ResolvedModel: "gpt-test"},
			{Channel: model.Channel{ID: 2, Type: "openai", BaseURL: second.URL, APIKey: "sk-second", Timeout: 1000}, ResolvedModel: "gpt-test"},
		},
	}

	rc := &RelayController{httpClient: relayclient.NewHTTPClient()}
	rc.relayStream(reqCtx)

	if !secondCalled {
		t.Fatal("second upstream was not called after first byte timeout")
	}
	if writer.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", writer.Code, writer.Body.String())
	}
	if got := writer.Body.String(); got != "data: second\n\n" {
		t.Fatalf("body = %q, want second stream payload", got)
	}
}

func TestRelayStreamDoesNotFallbackAfterOutputStarts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: first\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer first.Close()

	secondCalled := false
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: second\n\n"))
	}))
	defer second.Close()

	writer := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(writer)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)))

	reqCtx := &RequestContext{
		Gin:       ginCtx,
		RequestID: "stream-test-output-started",
		StartTime: time.Now(),
		App:       constant.RelayAppOpenAI,
		Mode:      constant.RelayModeChatCompletions,
		Format:    constant.RelayFormatOpenAI,
		Method:    http.MethodPost,
		Body:      []byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Meta:      relayRequestMeta{Model: "gpt-test", Stream: true},
		Candidates: []relayCandidate{
			{Channel: model.Channel{ID: 1, Type: "openai", BaseURL: first.URL, APIKey: "sk-first", Timeout: 1000}, ResolvedModel: "gpt-test"},
			{Channel: model.Channel{ID: 2, Type: "openai", BaseURL: second.URL, APIKey: "sk-second", Timeout: 1000}, ResolvedModel: "gpt-test"},
		},
	}

	rc := &RelayController{httpClient: relayclient.NewHTTPClient()}
	rc.relayStream(reqCtx)

	if secondCalled {
		t.Fatal("second upstream was called after output already started")
	}
	if writer.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", writer.Code, writer.Body.String())
	}
	if got := writer.Body.String(); got != "data: first\n\n" {
		t.Fatalf("body = %q, want first stream payload", got)
	}
}

func TestRelayResponsesStreamFallsBackWhenFirstCandidateHasNoFirstByte(t *testing.T) {
	gin.SetMode(gin.TestMode)

	blocked := make(chan struct{})
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-blocked
	}))
	defer first.Close()
	defer close(blocked)

	secondCalled := false
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"second\"}}]}\n\n"))
	}))
	defer second.Close()

	writer := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(writer)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-test","stream":true,"input":"hi"}`)))

	requestBody := []byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	respCtx := &responsesRequestContext{
		RequestContext: &RequestContext{
			Gin:       ginCtx,
			RequestID: "responses-stream-test",
			StartTime: time.Now(),
			App:       constant.RelayAppOpenAI,
			Mode:      constant.RelayModeResponses,
			Format:    constant.RelayFormatOpenAIResponses,
			Method:    http.MethodPost,
			Body:      requestBody,
			Meta:      relayRequestMeta{Model: "gpt-test", Stream: true},
			Candidates: []relayCandidate{
				{Channel: model.Channel{ID: 1, Type: "openai", BaseURL: first.URL, APIKey: "sk-first", Timeout: 20}, ResolvedModel: "gpt-test"},
				{Channel: model.Channel{ID: 2, Type: "openai", BaseURL: second.URL, APIKey: "sk-second", Timeout: 1000}, ResolvedModel: "gpt-test"},
			},
		},
		ResponsesBody: []byte(`{"model":"gpt-test","stream":true,"input":"hi"}`),
		ChatBody:      requestBody,
	}

	rc := &RelayController{httpClient: relayclient.NewHTTPClient()}
	rc.relayResponsesStream(respCtx)

	if !secondCalled {
		t.Fatal("second upstream was not called after first byte timeout")
	}
	if writer.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", writer.Code, writer.Body.String())
	}
	if !strings.Contains(writer.Body.String(), "second") {
		t.Fatalf("responses stream body = %q, want converted second payload", writer.Body.String())
	}
}

type blockingReadCloser struct {
	closed chan struct{}
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (b *blockingReadCloser) Read(_ []byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingReadCloser) Close() error {
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	return nil
}

func (b *blockingReadCloser) closedWithin(timeout time.Duration) bool {
	select {
	case <-b.closed:
		return true
	case <-time.After(timeout):
		return false
	}
}
