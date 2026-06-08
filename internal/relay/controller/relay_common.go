package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	startTime := time.Now()
	requestID := requestID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return
	}

	meta, err := parseRequestMeta(c, body, mode, format)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "请求格式错误", "invalid_request_error", err.Error())
		return
	}
	if meta.Model == "" {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, "缺少 model 参数")
		writeRelayError(c, http.StatusBadRequest, "缺少 model 参数", "invalid_request_error", "")
		return
	}

	candidates, err := rc.resolveCandidates(meta.Model)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, meta.Model, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}
	if len(candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, meta.Model, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return
	}

	if meta.Stream {
		rc.relayStream(c, requestID, startTime, app, mode, format, meta, body, candidates)
		return
	}
	rc.relayJSON(c, requestID, startTime, app, mode, format, meta, body, candidates)
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
	modelAction := strings.TrimPrefix(c.Param("modelAction"), "/")
	if modelAction == "" {
		modelAction = strings.TrimPrefix(c.Param("path"), "/")
	}
	if modelAction != "" {
		if decoded, err := url.PathUnescape(modelAction); err == nil {
			modelAction = decoded
		}
		model, action, _ := strings.Cut(modelAction, ":")
		meta.Model = strings.TrimPrefix(model, "models/")
		switch action {
		case "generateContent":
			meta.Stream = false
		case "streamGenerateContent":
			meta.Stream = true
		case "":
			return meta, fmt.Errorf("Gemini 路径缺少 generateContent 或 streamGenerateContent 操作")
		default:
			return meta, fmt.Errorf("不支持的 Gemini 操作: %s", action)
		}
	}

	var payload relayRequestMeta
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return meta, err
		}
	}
	if meta.Model == "" {
		meta.Model = payload.Model
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
