package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
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

func (rc *RelayController) handleRelay(c *gin.Context, mode constant.RelayMode, format constant.RelayFormat) {
	startTime := time.Now()
	requestID := requestID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return
	}

	meta, err := parseRequestMeta(body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "请求格式错误", "invalid_request_error", err.Error())
		return
	}
	if meta.Model == "" {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, "缺少 model 参数")
		writeRelayError(c, http.StatusBadRequest, "缺少 model 参数", "invalid_request_error", "")
		return
	}

	candidates, err := rc.resolveCandidates(meta.Model)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, meta.Model, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}
	if len(candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, mode, format, meta.Model, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return
	}

	if meta.Stream {
		rc.relayStream(c, requestID, startTime, mode, format, meta, body, candidates)
		return
	}
	rc.relayJSON(c, requestID, startTime, mode, format, meta, body, candidates)
}

func parseRequestMeta(body []byte) (relayRequestMeta, error) {
	var meta relayRequestMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return meta, err
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

func buildRelayInfo(c *gin.Context, requestID string, startTime time.Time, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, candidate relayCandidate, isStream bool) *relayinfo.RelayInfo {
	channel := candidate.Channel
	apiType := constant.APITypeFromChannelType(channel.Type)
	return &relayinfo.RelayInfo{
		RequestID:      requestID,
		StartTime:      startTime,
		RelayMode:      mode,
		RelayFormat:    format,
		APIType:        apiType,
		Channel:        &channel,
		RequestedModel: meta.Model,
		ResolvedModel:  candidate.ResolvedModel,
		IsStream:       isStream,
		ClientIP:       c.ClientIP(),
	}
}

func bodyWithResolvedModel(body []byte, resolvedModel string) ([]byte, error) {
	if resolvedModel == "" {
		return body, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["model"] = resolvedModel
	return json.Marshal(payload)
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
