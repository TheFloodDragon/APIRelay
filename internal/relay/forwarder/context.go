package forwarder

import (
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

type RelayKind string

const (
	RelayKindClaudeMessages RelayKind = "claude_messages"
	RelayKindOpenAIChat     RelayKind = "openai_chat"
	RelayKindCodexResponses RelayKind = "codex_responses"
	RelayKindGeminiNative   RelayKind = "gemini_native"
	RelayKindUnknown        RelayKind = "unknown"
)

type RequestContext struct {
	Gin          *gin.Context
	RequestID    string
	StartTime    time.Time
	Kind         RelayKind
	Method       string
	OriginalPath string
	Endpoint     string
	Query        string
	RawBody      []byte
	JSONBody     map[string]any
	Model        string
	Stream       bool
	Headers      http.Header

	Mode   constant.RelayMode
	Format constant.RelayFormat
}

func NewRequestContext(c *gin.Context) *RequestContext {
	return &RequestContext{
		Gin:          c,
		RequestID:    c.GetString("request_id"),
		StartTime:    time.Now(),
		Method:       c.Request.Method,
		OriginalPath: c.Request.URL.Path,
		Endpoint:     c.Request.URL.Path,
		Query:        c.Request.URL.RawQuery,
		Headers:      c.Request.Header.Clone(),
	}
}
