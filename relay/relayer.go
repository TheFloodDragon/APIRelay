package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/apicompat"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Relayer 持有转发所需的依赖。
type Relayer struct {
	cfg *config.RelayConfig
}

// NewRelayer 创建转发器。
func NewRelayer(cfg *config.RelayConfig) *Relayer {
	return &Relayer{cfg: cfg}
}

// HandleOpenAIChat 处理对外的 OpenAI /v1/chat/completions 请求。
func (r *Relayer) HandleOpenAIChat(c *gin.Context) {
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)
}

// HandleAnthropicMessages 处理对外的 Anthropic /v1/messages 请求。
func (r *Relayer) HandleAnthropicMessages(c *gin.Context) {
	r.handle(c, constant.EndpointAnthropic, apicompat.ParseAnthropicRequest)
}

// HandleResponses 处理对外的 OpenAI /v1/responses 请求。
func (r *Relayer) HandleResponses(c *gin.Context) {
	r.handle(c, constant.EndpointResponses, apicompat.ParseResponsesRequest)
}

// handle 是协议无关的入口：解析对外协议为 IR，再交给故障转移主循环。
func (r *Relayer) handle(c *gin.Context, ep constant.EndpointType, parse func([]byte) (*dto.UnifiedRequest, error)) {
	info := r.buildRelayInfo(c, ep)
	out := GetOutbound(ep)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		out.WriteError(c, http.StatusBadRequest, "read body failed")
		return
	}

	ir, err := parse(body)
	if err != nil {
		out.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	info.OriginModel = ir.Model
	info.IsStream = ir.Stream

	// 模型白名单校验
	if tok, ok := c.Get("token"); ok {
		if t, _ := tok.(*model.Token); t != nil && !t.AllowModel(ir.Model) {
			out.WriteError(c, http.StatusForbidden, "model not allowed for this token: "+ir.Model)
			r.logError(info, http.StatusForbidden, "model not allowed")
			return
		}
	}

	r.relayWithFailover(c, info, ir, out)
}

// relayWithFailover 执行带故障转移的转发主循环。
func (r *Relayer) relayWithFailover(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, out Outbound) {
	log := logger.FromContext(c.Request.Context())
	excluded := make(map[int]struct{})
	maxRetries := r.cfg.MaxRetries
	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastStatus int
	var lastErr string

	for attempt := 0; attempt < maxRetries; attempt++ {
		nowMs := time.Now().UnixMilli()
		ch, err := SelectChannel(info.Group, ir.Model, excluded, nowMs)
		if err != nil {
			out.WriteError(c, http.StatusInternalServerError, "select channel failed")
			r.logError(info, http.StatusInternalServerError, err.Error())
			return
		}
		if ch == nil {
			if attempt == 0 {
				out.WriteError(c, http.StatusServiceUnavailable,
					fmt.Sprintf("no available channel for model %q in group %q", ir.Model, info.Group))
				r.logError(info, http.StatusServiceUnavailable, "no available channel")
				return
			}
			break
		}

		info.Channel = ch
		info.ApiType = ch.APIType()
		info.UpstreamModel = ch.MappedModel(ir.Model)

		ad := GetAdaptor(info.ApiType)
		if ad == nil {
			excluded[ch.Id] = struct{}{}
			lastStatus, lastErr = http.StatusNotImplemented, "no adaptor for api type"
			log.Warn("relay.no_adaptor", zap.Int("channel_id", ch.Id), zap.Int("api_type", int(info.ApiType)))
			continue
		}
		ad.Init(info)

		log.Info("relay.attempt",
			zap.String("request_id", info.RequestID),
			zap.Int("attempt", attempt),
			zap.Int("channel_id", ch.Id),
			zap.String("channel", ch.Name),
			zap.String("api_type", ad.ChannelTypeName()),
			zap.String("origin_model", info.OriginModel),
			zap.String("upstream_model", info.UpstreamModel),
			zap.Bool("stream", info.IsStream),
		)

		status, retryable, err := r.doOnce(c, info, ir, ad, out)
		if err == nil {
			return // 成功
		}

		lastStatus, lastErr = status, err.Error()
		log.Warn("relay.attempt_failed",
			zap.Int("channel_id", ch.Id),
			zap.Int("status", status),
			zap.Bool("retryable", retryable),
			zap.Error(err),
		)

		if !retryable {
			r.logError(info, status, err.Error())
			return
		}
		// 冷却该渠道并切换
		excluded[ch.Id] = struct{}{}
		model.SetChannelCooldown(ch.Id, time.Now().Add(time.Duration(r.cfg.CooldownSeconds)*time.Second).UnixMilli())
	}

	// 重试耗尽
	if !c.Writer.Written() {
		out.WriteError(c, statusOrDefault(lastStatus), "all channels failed: "+lastErr)
	}
	r.logError(info, statusOrDefault(lastStatus), "all channels failed: "+lastErr)
}

// doOnce 对单个渠道执行一次完整转发。
// 返回 (上游状态码, 是否可重试, error)；err==nil 表示成功。
func (r *Relayer) doOnce(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, adp adaptor.Adaptor, out Outbound) (int, bool, error) {
	upstreamReq, err := adp.ConvertRequest(info, ir)
	if err != nil {
		return http.StatusInternalServerError, false, fmt.Errorf("convert request: %w", err)
	}
	reqBody, err := json.Marshal(upstreamReq)
	if err != nil {
		return http.StatusInternalServerError, false, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := adp.DoRequest(info, bytes.NewReader(reqBody))
	if err != nil {
		return http.StatusBadGateway, true, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		retryable := isRetryableStatus(resp.StatusCode)
		return resp.StatusCode, retryable, fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(respBody))
	}

	if info.IsStream {
		return r.handleStream(c, info, adp, resp, out)
	}
	return r.handleNonStream(c, info, adp, resp, out)
}

func (r *Relayer) handleNonStream(c *gin.Context, info *RelayInfo, adp adaptor.Adaptor, resp *http.Response, out Outbound) (int, bool, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusBadGateway, true, fmt.Errorf("read upstream body: %w", err)
	}
	uniResp, err := adp.ConvertResponse(info, body)
	if err != nil {
		return http.StatusBadGateway, false, fmt.Errorf("convert response: %w", err)
	}

	// 按对外协议序列化
	out.WriteResponse(c, uniResp, info.OriginModel)

	r.logConsume(info, &uniResp.Usage)
	return http.StatusOK, false, nil
}

func (r *Relayer) handleStream(c *gin.Context, info *RelayInfo, adp adaptor.Adaptor, resp *http.Response, out Outbound) (int, bool, error) {
	writer := out.NewStream(c, info.RequestID, info.OriginModel)
	firstByte := false

	usage, err := adp.StreamHandler(info, resp, func(chunk *dto.UnifiedStreamChunk) error {
		if !firstByte {
			info.FirstByteMs = int(time.Now().UnixMilli() - info.StartAtMs)
			firstByte = true
		}
		return writer.WriteChunk(c, chunk)
	})
	if err != nil {
		// 已经开始写流，无法切换渠道
		_ = writer.Finish(c)
		return http.StatusOK, false, fmt.Errorf("stream: %w", err)
	}

	_ = writer.Finish(c)

	u := dto.Usage{}
	if usage != nil {
		u = *usage
	}
	r.logConsume(info, &u)
	return http.StatusOK, false, nil
}

// ---- 辅助 ----

func (r *Relayer) buildRelayInfo(c *gin.Context, ep constant.EndpointType) *RelayInfo {
	info := &RelayInfo{
		RequestID:    c.GetString(logger.RequestIDKey),
		EndpointType: ep,
		Group:        r.cfg.DefaultGroup,
		StartAtMs:    time.Now().UnixMilli(),
	}
	if tok, ok := c.Get("token"); ok {
		if t, _ := tok.(*model.Token); t != nil {
			info.TokenId = t.Id
			info.TokenName = t.Name
			info.UserId = t.UserId
			if t.Group != "" {
				info.Group = t.Group
			}
		}
	}
	return info
}

func (r *Relayer) logConsume(info *RelayInfo, usage *dto.Usage) {
	useTime := int(time.Now().UnixMilli() - info.StartAtMs)
	l := &model.Log{
		RequestId:         info.RequestID,
		UpstreamRequestId: info.UpstreamRequestId,
		Type:              model.LogTypeConsume,
		UserId:            info.UserId,
		TokenId:           info.TokenId,
		TokenName:         info.TokenName,
		Group:             info.Group,
		EndpointType:      string(info.EndpointType),
		ApiType:           constant.ChannelTypeName(channelTypeOf(info)),
		SrcModel:          info.OriginModel,
		MappedModel:       info.UpstreamModel,
		IsStream:          info.IsStream,
		PromptTokens:      usage.PromptTokens,
		CompletionTokens:  usage.CompletionTokens,
		TotalTokens:       usage.TotalTokens,
		UseTimeMs:         useTime,
		FirstByteMs:       info.FirstByteMs,
		Status:            http.StatusOK,
	}
	if info.Channel != nil {
		l.ChannelId = info.Channel.Id
		l.ChannelName = info.Channel.Name
	}
	_ = model.CreateLog(l)
}

func (r *Relayer) logError(info *RelayInfo, status int, errMsg string) {
	l := &model.Log{
		RequestId:    info.RequestID,
		Type:         model.LogTypeError,
		UserId:       info.UserId,
		TokenId:      info.TokenId,
		TokenName:    info.TokenName,
		Group:        info.Group,
		EndpointType: string(info.EndpointType),
		SrcModel:     info.OriginModel,
		MappedModel:  info.UpstreamModel,
		IsStream:     info.IsStream,
		UseTimeMs:    int(time.Now().UnixMilli() - info.StartAtMs),
		Status:       status,
		Error:        errMsg,
	}
	if info.Channel != nil {
		l.ChannelId = info.Channel.Id
		l.ChannelName = info.Channel.Name
	}
	_ = model.CreateLog(l)
}

func channelTypeOf(info *RelayInfo) int {
	if info.Channel != nil {
		return info.Channel.Type
	}
	return 0
}

func setSSEHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

func statusOrDefault(s int) int {
	if s == 0 {
		return http.StatusServiceUnavailable
	}
	return s
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}

// 占位：保证 context 包被引用（流式取消等后续扩展）
var _ = context.Background
