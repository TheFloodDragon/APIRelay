package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/forwarder"
	"github.com/gin-gonic/gin"
)

type RequestContext struct {
	Gin          *gin.Context
	RequestID    string
	StartTime    time.Time
	Mode         constant.RelayMode
	Format       constant.RelayFormat
	Method       string
	OriginalPath string
	Endpoint     string
	Query        string
	RawBody      []byte
	JSONBody     map[string]any
	Model        string
	Stream       bool
	Headers      http.Header
	// Body 是当前控制器路径使用的请求体。普通 relay 中等同 RawBody；Responses bridge 中为 Chat Completions 兼容体。
	Body       []byte
	Meta       relayRequestMeta
	Candidates []relayCandidate

	forwarderContext *forwarder.RequestContext
}

func (rc *RelayController) newRequestContext(
	c *gin.Context,
	mode constant.RelayMode,
	format constant.RelayFormat,
) (*RequestContext, bool) {
	startTime := time.Now()
	requestID := requestID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return nil, false
	}

	meta, err := parseRequestMeta(c, body, mode, format)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "请求格式错误", "invalid_request_error", err.Error())
		return nil, false
	}
	if meta.Model == "" {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, "缺少 model 参数")
		writeRelayError(c, http.StatusBadRequest, "缺少 model 参数", "invalid_request_error", "")
		return nil, false
	}

	headers := c.Request.Header.Clone()
	jsonBody := map[string]any(nil)
	if len(body) > 0 && format != constant.RelayFormatGemini {
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) == nil {
			jsonBody = parsed
		}
	}

	reqCtx := &RequestContext{
		Gin:          c,
		RequestID:    requestID,
		StartTime:    startTime,
		Mode:         mode,
		Format:       format,
		Method:       c.Request.Method,
		OriginalPath: c.Request.URL.Path,
		Endpoint:     c.Request.URL.Path,
		Query:        c.Request.URL.RawQuery,
		RawBody:      body,
		JSONBody:     jsonBody,
		Model:        meta.Model,
		Stream:       meta.Stream,
		Headers:      headers,
		Body:         body,
		Meta:         meta,
	}
	reqCtx.forwarderContext = reqCtx.toForwarderContext()

	if err := rc.attachCandidates(reqCtx); err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, meta.Model, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return nil, false
	}
	if len(reqCtx.Candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, mode, format, meta.Model, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return nil, false
	}
	return reqCtx, true
}

func (ctx *RequestContext) forwarderRequestContext() *forwarder.RequestContext {
	if ctx == nil {
		return nil
	}
	if ctx.forwarderContext == nil {
		ctx.forwarderContext = ctx.toForwarderContext()
	}
	return ctx.forwarderContext
}

func (ctx *RequestContext) toForwarderContext() *forwarder.RequestContext {
	if ctx == nil {
		return nil
	}
	return &forwarder.RequestContext{
		Gin:          ctx.Gin,
		RequestID:    ctx.RequestID,
		StartTime:    ctx.StartTime,
		Kind:         relayKindFor(ctx.Mode, ctx.Format),
		Method:       ctx.Method,
		OriginalPath: ctx.OriginalPath,
		Endpoint:     ctx.Endpoint,
		Query:        ctx.Query,
		RawBody:      ctx.RawBody,
		JSONBody:     ctx.JSONBody,
		Model:        ctx.Model,
		Stream:       ctx.Stream,
		Headers:      ctx.Headers,
		Mode:         ctx.Mode,
		Format:       ctx.Format,
	}
}

func (rc *RelayController) attachCandidates(reqCtx *RequestContext) error {
	candidates, err := rc.resolveCandidates(reqCtx.Model)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		reqCtx.Candidates = nil
		return nil
	}
	byChannelID := make(map[uint]relayCandidate, len(candidates))
	for _, candidate := range candidates {
		byChannelID[candidate.Channel.ID] = candidate
	}
	providers, err := rc.providerRouter.SelectProviders()
	if err != nil {
		return err
	}
	ordered := make([]relayCandidate, 0, len(candidates))
	seen := make(map[uint]struct{}, len(candidates))
	for _, provider := range providers {
		candidate, ok := byChannelID[provider.ID]
		if !ok {
			continue
		}
		candidate.Channel = provider
		ordered = append(ordered, candidate)
		seen[provider.ID] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, exists := seen[candidate.Channel.ID]; exists {
			continue
		}
		ordered = append(ordered, candidate)
	}
	reqCtx.Candidates = ordered
	return nil
}

func (ctx *RequestContext) candidateForProvider(channelID uint) (relayCandidate, bool) {
	if ctx == nil {
		return relayCandidate{}, false
	}
	for _, candidate := range ctx.Candidates {
		if candidate.Channel.ID == channelID {
			return candidate, true
		}
	}
	return relayCandidate{}, false
}

func relayKindFor(mode constant.RelayMode, format constant.RelayFormat) forwarder.RelayKind {
	switch {
	case format == constant.RelayFormatAnthropic:
		return forwarder.RelayKindClaudeMessages
	case mode == constant.RelayModeResponses:
		return forwarder.RelayKindCodexResponses
	case format == constant.RelayFormatGemini:
		return forwarder.RelayKindGeminiNative
	case mode == constant.RelayModeChatCompletions:
		return forwarder.RelayKindOpenAIChat
	default:
		return forwarder.RelayKindUnknown
	}
}
