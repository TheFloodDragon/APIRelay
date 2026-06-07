package controller

import (
	"log"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) logRequest(c *gin.Context, info *relayinfo.RelayInfo, statusCode int, errMsg string) {
	if info == nil {
		return
	}

	latencyMS := int(time.Since(info.StartTime).Milliseconds())
	var channelID *uint
	channelType := "-"
	if info.Channel != nil {
		id := info.Channel.ID
		channelID = &id
		channelType = info.Channel.Type
		if channelType == "" {
			channelType = "openai_compatible"
		}
	}

	apiType := string(info.APIType)
	if apiType == "" {
		apiType = "-"
	}

	requestLog := &model.RequestLog{
		RequestID:     info.RequestID,
		ChannelID:     channelID,
		ChannelType:   channelType,
		APIType:       apiType,
		RelayMode:     string(info.RelayMode),
		RelayFormat:   string(info.RelayFormat),
		ResolvedModel: info.ResolvedModel,
		Model:         info.RequestedModel,
		Method:        c.Request.Method,
		Path:          c.Request.URL.Path,
		StatusCode:    statusCode,
		Latency:       latencyMS,
		Error:         errMsg,
		IP:            info.ClientIP,
		APIKeyID:      apiKeyIDFromContext(c),
	}

	if err := rc.logRepo.Create(requestLog); err != nil {
		log.Printf("[MODEL] request_id=%s model=%s resolved_model=%s channel_id=%v channel_type=%s api_type=%s relay_mode=%s relay_format=%s status=%d latency=%dms ip=%s error=%q log_error=%q",
			info.RequestID,
			info.RequestedModel,
			info.ResolvedModel,
			logChannelID(channelID),
			channelType,
			apiType,
			info.RelayMode,
			info.RelayFormat,
			statusCode,
			latencyMS,
			info.ClientIP,
			errMsg,
			err.Error(),
		)
		return
	}

	log.Printf("[MODEL] request_id=%s model=%s resolved_model=%s channel_id=%v channel_type=%s api_type=%s relay_mode=%s relay_format=%s status=%d latency=%dms ip=%s error=%q",
		info.RequestID,
		info.RequestedModel,
		info.ResolvedModel,
		logChannelID(channelID),
		channelType,
		apiType,
		info.RelayMode,
		info.RelayFormat,
		statusCode,
		latencyMS,
		info.ClientIP,
		errMsg,
	)
}

func (rc *RelayController) logNoChannel(c *gin.Context, requestID string, startTime time.Time, mode constant.RelayMode, format constant.RelayFormat, requestedModel string, statusCode int, errMsg string) {
	info := &relayinfo.RelayInfo{
		RequestID:      requestID,
		StartTime:      startTime,
		RelayMode:      mode,
		RelayFormat:    format,
		RequestedModel: requestedModel,
		ClientIP:       c.ClientIP(),
	}
	rc.logRequest(c, info, statusCode, errMsg)
}

func logChannelID(channelID *uint) interface{} {
	if channelID == nil {
		return "-"
	}
	return *channelID
}

func apiKeyIDFromContext(c *gin.Context) *uint {
	value, ok := c.Get("api_key_id")
	if !ok {
		return nil
	}

	switch id := value.(type) {
	case uint:
		return &id
	case int:
		if id < 0 {
			return nil
		}
		converted := uint(id)
		return &converted
	case uint64:
		converted := uint(id)
		return &converted
	default:
		return nil
	}
}
