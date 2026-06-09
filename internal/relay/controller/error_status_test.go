package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestShouldTryNextCandidateTreatsBadRequestAsNonRetryable(t *testing.T) {
	if shouldTryNextCandidate(http.StatusBadRequest, nil) {
		t.Fatal("400 should not try next candidate")
	}
	if shouldTryNextCandidate(http.StatusUnprocessableEntity, nil) {
		t.Fatal("422 should not try next candidate")
	}
	if !shouldTryNextCandidate(http.StatusTooManyRequests, nil) {
		t.Fatal("429 should try next candidate")
	}
	if !shouldTryNextCandidate(http.StatusBadGateway, nil) {
		t.Fatal("502 should try next candidate")
	}
}

func TestWriteFinalRelayErrorPreservesUpstream4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)

	writeFinalRelayError(ctx, &relayUpstreamError{statusCode: http.StatusBadRequest, message: "unsupported parameter"}, "unsupported parameter", true)

	if writer.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", writer.Code, http.StatusBadRequest, writer.Body.String())
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(writer.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["error"]["type"] != "upstream_error" {
		t.Fatalf("error type = %v, want upstream_error", payload["error"]["type"])
	}
	if payload["error"]["details"] != "unsupported parameter" {
		t.Fatalf("details = %v, want unsupported parameter", payload["error"]["details"])
	}
}
