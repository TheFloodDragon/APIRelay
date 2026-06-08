package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
	"github.com/gin-gonic/gin"
)

type relayRequestMeta struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type relayCandidate struct {
	Channel       model.Channel
	ResolvedModel string
}

func (rc *RelayController) handleRelay(c *gin.Context, app constant.RelayApp, mode constant.RelayMode, format constant.RelayFormat) {
	reqCtx, ok := rc.newRequestContext(c, app, mode, format)
	if !ok {
		return
	}
	if reqCtx.Meta.Stream {
		rc.relayStream(reqCtx)
		return
	}
	rc.relayJSON(reqCtx)
}

func parseRequestMeta(c *gin.Context, body []byte, mode constant.RelayMode, format constant.RelayFormat) (relayRequestMeta, error) {
	if format == constant.RelayFormatGemini {
		return parseGeminiRequestMeta(c, body)
	}

	var meta relayRequestMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func parseGeminiRequestMeta(c *gin.Context, body []byte) (relayRequestMeta, error) {
	meta := relayRequestMeta{}
	route, err := parseGeminiNativePath(c.Request.URL.Path, c.Request.URL.RawQuery)
	if err == nil {
		meta.Model = route.Model
		meta.Stream = route.Stream
		if route.Kind == geminiNativeRouteModels || route.Kind == geminiNativeRouteModel {
			return meta, fmt.Errorf("Gemini 路径缺少 generateContent、streamGenerateContent 或 countTokens 操作")
		}
	} else if c.Param("modelAction") != "" || c.Param("path") != "" {
		return meta, err
	}

	var payload relayRequestMeta
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return meta, err
		}
	}
	if meta.Model == "" {
		meta.Model = normalizeGeminiModelName(payload.Model)
	}
	if payload.Stream {
		meta.Stream = true
	}
	return meta, nil
}

func (rc *RelayController) resolveCandidates(requestedModel string) ([]relayCandidate, error) {
	resolvedModels, err := rc.modelRouter.ResolveModel(requestedModel)
	if err != nil {
		return nil, err
	}

	candidates := make([]relayCandidate, 0)
	for _, resolvedModel := range resolvedModels {
		channels, err := rc.scheduler.GetAllChannelsForModel(resolvedModel)
		if err != nil || len(channels) == 0 {
			continue
		}
		for _, channel := range channels {
			candidates = append(candidates, relayCandidate{Channel: channel, ResolvedModel: resolvedModel})
		}
	}
	return candidates, nil
}

func (rc *RelayController) filterCircuitOpenCandidates(app constant.RelayApp, candidates []relayCandidate) []relayCandidate {
	if rc == nil || rc.circuitBreaker == nil || len(candidates) == 0 {
		return candidates
	}

	filtered := make([]relayCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if rc.circuitBreaker.Allow(app, candidate.Channel.ID) {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		return candidates
	}
	return filtered
}

func (rc *RelayController) recordCircuitSuccess(info *relayinfo.RelayInfo) {
	if rc == nil || rc.circuitBreaker == nil || info == nil || info.Channel == nil {
		return
	}
	rc.circuitBreaker.RecordSuccess(info.RelayApp, info.Channel.ID)
}

func (rc *RelayController) recordCircuitFailure(info *relayinfo.RelayInfo, statusCode int, err error) {
	if rc == nil || rc.circuitBreaker == nil || info == nil || info.Channel == nil {
		return
	}
	if shouldRecordCircuitFailure(statusCode, err) {
		rc.circuitBreaker.RecordFailure(info.RelayApp, info.Channel.ID)
	}
}

func buildRelayInfo(c *gin.Context, requestID string, startTime time.Time, app constant.RelayApp, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, candidate relayCandidate, isStream bool) *relayinfo.RelayInfo {
	channel := candidate.Channel
	apiType := constant.APITypeFromChannelType(channel.Type)
	return &relayinfo.RelayInfo{
		RequestID:      requestID,
		StartTime:      startTime,
		RelayApp:       app,
		RelayMode:      mode,
		RelayFormat:    format,
		APIType:        apiType,
		Channel:        &channel,
		RequestedModel: meta.Model,
		ResolvedModel:  candidate.ResolvedModel,
		OriginalPath:   c.Request.URL.Path,
		Endpoint:       c.Request.URL.Path,
		Query:          c.Request.URL.RawQuery,
		IsStream:       isStream,
		ClientIP:       c.ClientIP(),
	}
}

func bodyWithResolvedModel(body []byte, resolvedModel string, format constant.RelayFormat) ([]byte, error) {
	if resolvedModel == "" || format == constant.RelayFormatGemini {
		return body, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["model"] = resolvedModel
	return json.Marshal(payload)
}

func convertRelayRequest(protocolAdaptor adaptor.Adaptor, requestBody []byte, info *relayinfo.RelayInfo) ([]byte, error) {
	meta := protocol.RequestMeta{Model: info.ResolvedModel, Stream: info.IsStream}
	if metaAware, ok := protocolAdaptor.(adaptor.RequestMetaAwareAdaptor); ok {
		return metaAware.ConvertRequestWithMeta(requestBody, info.RelayMode, info.RelayFormat, meta)
	}
	return protocolAdaptor.ConvertRequest(requestBody, info.RelayMode, info.RelayFormat)
}

func requestID(c *gin.Context) string {
	for _, header := range []string{"X-Request-ID", "X-Request-Id", "Request-ID"} {
		if value := c.GetHeader(header); value != "" {
			return value
		}
	}
	return fmt.Sprintf("relay-%s-%d", strconv.FormatInt(time.Now().UnixNano(), 36), time.Now().UnixNano())
}

func timeoutForChannel(channel *model.Channel) time.Duration {
	if channel == nil || channel.Timeout <= 0 {
		return 60 * time.Second
	}
	return time.Duration(channel.Timeout) * time.Millisecond
}
