package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/apirelay/apirelay/relay/circuitbreaker"
	"github.com/apirelay/apirelay/relay/relaycommon"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Relayer 持有转发所需的依赖。
type Relayer struct {
	cfg *config.RelayConfig
}

// NewRelayer 创建转发器。
func NewRelayer(cfg *config.RelayConfig) *Relayer {
	if cfg != nil {
		relaycommon.SetRuntimeChannelMaxRetries(cfg.ChannelMaxRetries)
	}
	return &Relayer{cfg: cfg}
}

func (r *Relayer) channelMaxRetries() int {
	if r == nil || r.cfg == nil {
		return relaycommon.RuntimeChannelMaxRetries(defaultMaxSameChannelRetries)
	}
	return relaycommon.RuntimeChannelMaxRetries(r.cfg.ChannelMaxRetries)
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

// HandleListModels 处理 GET /v1/models - 返回全局可用模型列表。
func (r *Relayer) HandleListModels(c *gin.Context) {
	// 获取当前 token 的分组
	group := r.cfg.DefaultGroup
	if tok, ok := c.Get("token"); ok {
		if t, _ := tok.(*model.Token); t != nil && t.Group != "" {
			group = t.Group
		}
	}

	// 查询该分组下所有启用渠道的模型
	models, err := model.GetAvailableModels(group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 构造 OpenAI /v1/models 格式响应
	data := make([]gin.H, 0, len(models))
	now := time.Now().Unix()
	for _, m := range models {
		data = append(data, gin.H{
			"id":       m,
			"object":   "model",
			"created":  now,
			"owned_by": "apirelay",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// handle 是协议无关的入口：解析对外协议为 IR，再交给故障转移主循环。
func (r *Relayer) handle(c *gin.Context, ep constant.EndpointType, parse func([]byte) (*dto.UnifiedRequest, error)) {
	ctx := c.Request.Context()
	if r.cfg.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(r.cfg.RequestTimeout)*time.Second)
		defer cancel()
	}

	info := r.buildRelayInfo(c, ep)
	info.Context = ctx
	out := GetOutbound(ep)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			out.WriteError(c, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		category := classifyRelayError(ctx, err)
		if category == ErrorCategoryClientCanceled {
			r.logError(info, statusClientClosedRequest, "client canceled while reading request body")
			return
		}
		if category == ErrorCategoryRelayTimeout {
			out.WriteError(c, http.StatusGatewayTimeout, "request timeout")
			r.logError(info, http.StatusGatewayTimeout, "relay request timeout while reading request body")
			return
		}
		out.WriteError(c, http.StatusBadRequest, "read body failed")
		return
	}

	ir, err := parse(body)
	if err != nil {
		out.WriteError(c, http.StatusBadRequest, "请求体解析失败："+err.Error())
		return
	}
	info.OriginModel = ir.Model
	info.IsStream = ir.Stream

	if ir.Model == "" {
		out.WriteError(c, http.StatusBadRequest, "请求缺少 model 字段")
		return
	}

	// 模型白名单校验
	var billing *BillingSession
	if tok, ok := c.Get("token"); ok {
		if t, _ := tok.(*model.Token); t != nil {
			billing = NewBillingSession(t.Id)
			if !t.AllowModel(ir.Model) {
				msg := fmt.Sprintf("当前令牌无权使用模型 %q，请检查令牌的模型白名单设置", ir.Model)
				out.WriteError(c, http.StatusForbidden, msg)
				r.logError(info, http.StatusForbidden, "model not allowed: "+ir.Model)
				return
			}
		}
	}

	// 候选渠道预加载（本次请求复用），并用于预扣价格上界估算。
	candidates, err := model.GetChannelCandidates(info.Group, ir.Model)
	if err != nil {
		out.WriteError(c, http.StatusInternalServerError, "select channel failed")
		r.logError(info, http.StatusInternalServerError, err.Error())
		return
	}
	if len(candidates) == 0 {
		out.WriteError(c, http.StatusServiceUnavailable, noChannelError(info.Group, ir.Model))
		r.logError(info, http.StatusServiceUnavailable, "no available channel for model "+ir.Model+" in group "+info.Group)
		return
	}

	// 预扣额度：基于候选渠道价格上界 + 全局价格表估算，避免渠道级价格高于全局价时少预扣。
	if billing != nil {
		reserved := EstimateQuotaForCandidates(ir, candidates)
		if reserved > 0 {
			if err := billing.Reserve(reserved); err != nil {
				out.WriteError(c, http.StatusForbidden, "令牌额度不足，请充值或调整额度后重试")
				r.logError(info, http.StatusForbidden, "quota insufficient: need "+strconv.FormatInt(reserved, 10))
				return
			}
			info.ReservedQuota = billing.Reserved()
		}
	}
	defer func() {
		if billing != nil {
			billing.Refund()
		}
	}()

	r.relayWithFailover(c, info, ir, out, billing, candidates)
}

// relayWithFailover 执行带故障转移的转发主循环。
//
// 调度与故障转移策略（阶段4）：
//   - 优先级分层 + 加权随机选渠道（SelectChannel）；
//   - 临时性错误（429/503/504）在同渠道有限次重试（带退避）；
//   - 其它可重试错误冷却当前渠道并切换；高优先级耗尽后自动降级；
//   - 请求成功后清除该渠道冷却。
func (r *Relayer) relayWithFailover(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, out Outbound, billing *BillingSession, candidates []model.ChannelCandidate) {
	log := logger.FromContext(c.Request.Context())
	channelMaxRetries := r.channelMaxRetries()
	state := NewFailoverState(r.cfg.CooldownSeconds, channelMaxRetries)

	maxSwitches := r.cfg.MaxRetries
	if maxSwitches < 1 {
		maxSwitches = 1
	}
	switches := 0
	// hardCap 防止同渠道重试导致的无限循环（切换预算 + 每渠道重试预算）。
	maxRetries := channelMaxRetries
	if maxRetries < 0 {
		maxRetries = defaultMaxSameChannelRetries
	}
	hardCap := maxSwitches + (maxSwitches+1)*maxRetries + 2

	// 语义：MaxRetries=N 表示最多切换 N 次，即最多尝试 N+1 个渠道。
	// 因此循环守卫用 switches <= maxSwitches（此前用 < 会少尝试一个渠道，
	// 导致 MaxRetries=1 时故障转移完全不生效）。
	for iter := 0; iter < hardCap && switches <= maxSwitches; iter++ {
		if err := relayContext(info).Err(); err != nil {
			category := classifyRelayError(relayContext(info), err)
			if category == ErrorCategoryClientCanceled {
				r.logError(info, statusClientClosedRequest, "client canceled")
				return
			}
			if category == ErrorCategoryRelayTimeout {
				out.WriteError(c, http.StatusGatewayTimeout, "request timeout")
				r.logError(info, http.StatusGatewayTimeout, "relay request timeout")
				return
			}
		}
		nowMs := time.Now().UnixMilli()
		ch := SelectFromCandidates(candidates, state.Excluded(), nowMs)
		if ch == nil {
			if switches == 0 && iter == 0 {
				out.WriteError(c, http.StatusServiceUnavailable, noChannelError(info.Group, ir.Model))
				r.logError(info, http.StatusServiceUnavailable, "no available channel for model "+ir.Model+" in group "+info.Group)
				return
			}
			break // 所有渠道均已排除/冷却
		}

		info.Channel = ch
		info.ApiType = ResolveAPIType(ch, info.OriginModel)
		info.UpstreamModel = ch.MappedModel(info.OriginModel)

		ad := GetAdaptor(info.ApiType)
		if ad == nil {
			circuitbreaker.GetManager().ReleaseProbe(ch.Id)
			state.FailedChannels[ch.Id] = struct{}{}
			state.LastStatus, state.LastErr = http.StatusNotImplemented, "no adaptor for api type"
			state.RecordAttempt(FailoverAttempt{
				Iter:          iter,
				Switches:      switches,
				ChannelId:     ch.Id,
				ChannelName:   ch.Name,
				ApiType:       apiTypeLabel(info),
				OriginModel:   info.OriginModel,
				UpstreamModel: info.UpstreamModel,
				Status:        http.StatusNotImplemented,
				Retryable:     true,
				Decision:      "switch_channel",
				ErrorCategory: string(ErrorCategoryInternal),
				Error:         "no adaptor for api type",
			})
			switches++
			log.Warn("relay.no_adaptor", zap.Int("channel_id", ch.Id), zap.Int("api_type", int(info.ApiType)))
			continue
		}
		ad.Init(info)

		log.Info("relay.attempt",
			zap.String("request_id", info.RequestID),
			zap.Int("iter", iter),
			zap.Int("switches", switches),
			zap.Int("channel_id", ch.Id),
			zap.String("channel", ch.Name),
			zap.String("api_type", ad.ChannelTypeName()),
			zap.String("origin_model", info.OriginModel),
			zap.String("upstream_model", info.UpstreamModel),
			zap.Bool("stream", info.IsStream),
		)

		status, retryable, err := r.doOnce(c, info, ir, ad, out, billing)
		if err == nil {
			state.RecordAttempt(FailoverAttempt{
				Iter:          iter,
				Switches:      switches,
				ChannelId:     ch.Id,
				ChannelName:   ch.Name,
				ApiType:       ad.ChannelTypeName(),
				OriginModel:   info.OriginModel,
				UpstreamModel: info.UpstreamModel,
				Status:        http.StatusOK,
				Retryable:     false,
				Decision:      "success",
			})
			info.FailoverChain = state.ChainJSON()
			model.ClearChannelCooldown(ch.Id)                // 成功后清除冷却
			circuitbreaker.GetManager().RecordSuccess(ch.Id) // 记录熔断器成功
			return
		}

		category := classifyRelayError(relayContext(info), err)
		if category == ErrorCategoryClientCanceled {
			circuitbreaker.GetManager().ReleaseProbe(ch.Id)
			log.Warn("relay.client_canceled",
				zap.Int("channel_id", ch.Id),
				zap.String("channel", ch.Name),
				zap.Error(err),
			)
			r.logError(info, statusClientClosedRequest, "client canceled")
			return
		}
		if category == ErrorCategoryRelayTimeout {
			status = http.StatusGatewayTimeout
			retryable = false
			state.RecordAttempt(FailoverAttempt{
				Iter:          iter,
				Switches:      switches,
				ChannelId:     ch.Id,
				ChannelName:   ch.Name,
				ApiType:       ad.ChannelTypeName(),
				OriginModel:   info.OriginModel,
				UpstreamModel: info.UpstreamModel,
				Status:        status,
				Retryable:     retryable,
				Decision:      "fatal",
				ErrorCategory: string(category),
				Error:         err.Error(),
			})
			info.FailoverChain = state.ChainJSON()
			circuitbreaker.GetManager().RecordFailure(ch.Id, err.Error())
			if !c.Writer.Written() {
				out.WriteError(c, http.StatusGatewayTimeout, "request timeout")
			}
			r.logError(info, http.StatusGatewayTimeout, timeoutLogMessage(category))
			return
		}
		if isTimeoutCategory(category) {
			status = timeoutStatus(category)
			retryable = true
		}

		// 记录熔断器失败。客户端主动取消不计入渠道失败。
		circuitbreaker.GetManager().RecordFailure(ch.Id, err.Error())

		decision := state.OnFailure(ch.Id, status, retryable, err.Error())
		state.RecordAttempt(FailoverAttempt{
			Iter:          iter,
			Switches:      switches,
			ChannelId:     ch.Id,
			ChannelName:   ch.Name,
			ApiType:       ad.ChannelTypeName(),
			OriginModel:   info.OriginModel,
			UpstreamModel: info.UpstreamModel,
			Status:        status,
			Retryable:     retryable,
			Decision:      failoverDecisionLabel(decision),
			ErrorCategory: string(category),
			Error:         err.Error(),
		})
		info.FailoverChain = state.ChainJSON()
		log.Warn("relay.attempt_failed",
			zap.Int("channel_id", ch.Id),
			zap.String("channel", ch.Name),
			zap.String("api_type", ad.ChannelTypeName()),
			zap.String("error_category", string(category)),
			zap.Int("status", status),
			zap.Bool("retryable", retryable),
			zap.Int("decision", int(decision)),
			zap.Error(err),
		)

		// 切换渠道时记录该供应商的失败日志（便于在后台逐供应商排查）。
		// 同渠道重试不落库，避免噪声；致命错误由下方统一记录。
		if decision == DecisionSwitchChannel {
			r.logAttemptFailure(info, ch, status, err.Error())
		}

		switch decision {
		case DecisionFatal:
			// 致命错误：若尚未向客户端写出任何内容，返回友好错误响应。
			// （此前缺失，导致非流式致命错误时客户端收到空响应。）
			if !c.Writer.Written() {
				out.WriteError(c, statusOrDefault(status), friendlyUpstreamError(info, status, err))
			}
			r.logError(info, status, err.Error())
			return
		case DecisionRetrySameChannel:
			if !state.SameChannelDelay(relayContext(info)) {
				category := classifyRelayError(relayContext(info), relayContext(info).Err())
				if category == ErrorCategoryClientCanceled {
					r.logError(info, statusClientClosedRequest, "client canceled")
				} else if category == ErrorCategoryRelayTimeout {
					r.logError(info, http.StatusGatewayTimeout, "relay request timeout")
				}
				return // 客户端取消或请求超时
			}
			// 同渠道重试不消耗切换预算
		case DecisionSwitchChannel:
			switches++ // 已在 OnFailure 中冷却并排除
		}
	}

	// 重试耗尽
	info.FailoverChain = state.ChainJSON()
	if !c.Writer.Written() {
		out.WriteError(c, statusOrDefault(state.LastStatus),
			friendlyExhaustedError(info, state.LastStatus, state.LastErr))
	}
	r.logError(info, statusOrDefault(state.LastStatus), state.LastErr)
}

// doOnce 对单个渠道执行一次完整转发。
// 返回 (上游状态码, 是否可重试, error)；err==nil 表示成功。
func (r *Relayer) doOnce(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, adp adaptor.Adaptor, out Outbound, billing *BillingSession) (int, bool, error) {
	if err := relayContext(info).Err(); err != nil {
		category := classifyRelayError(relayContext(info), err)
		if category == ErrorCategoryClientCanceled {
			return statusClientClosedRequest, false, err
		}
		if category == ErrorCategoryRelayTimeout {
			return http.StatusGatewayTimeout, false, err
		}
	}

	upstreamReq, err := adp.ConvertRequest(info, ir)
	if err != nil {
		return http.StatusInternalServerError, false, fmt.Errorf("convert request: %w", err)
	}
	reqBody, err := json.Marshal(upstreamReq)
	if err != nil {
		return http.StatusInternalServerError, false, fmt.Errorf("marshal request: %w", err)
	}
	// B1 同协议零改写透传：对外协议与上游协议一致时，基于原始请求体仅改写顶层
	// model 字段整体透传，保留 IR 未建模字段（tool_choice/metadata/thinking 等）。
	// 任一步失败则回退上面的 IR 重建结果。
	if apicompat.SameProtocol(info.EndpointType, info.ApiType) && len(ir.Raw) > 0 {
		if passthrough, perr := apicompat.ReplaceTopLevelModel(ir.Raw, info.UpstreamModel); perr == nil {
			reqBody = passthrough
		}
	}

	resp, err := adp.DoRequest(info, bytes.NewReader(reqBody))
	if err != nil {
		switch category := classifyRelayError(relayContext(info), err); category {
		case ErrorCategoryClientCanceled:
			return statusClientClosedRequest, false, fmt.Errorf("do request: %w", err)
		case ErrorCategoryRelayTimeout:
			return http.StatusGatewayTimeout, false, fmt.Errorf("relay request timeout while contacting upstream: %w", err)
		case ErrorCategoryUpstreamTimeout, ErrorCategoryTimeout:
			return http.StatusGatewayTimeout, true, fmt.Errorf("upstream request timeout: %w", err)
		default:
			return http.StatusBadGateway, true, fmt.Errorf("do request: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		retryable := isRetryableStatus(resp.StatusCode)
		// 记录详细错误（含上游响应体）用于日志与故障转移决策；
		// 对客户端的友好提示在 friendlyUpstreamError 中生成。
		detail := extractUpstreamErrorMessage(respBody)
		return resp.StatusCode, retryable, fmt.Errorf("upstream status %d: %s", resp.StatusCode, detail)
	}

	if info.IsStream {
		return r.handleStream(c, info, ir, adp, resp, out, billing)
	}
	return r.handleNonStream(c, info, ir, adp, resp, out, billing)
}

func (r *Relayer) handleNonStream(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, adp adaptor.Adaptor, resp *http.Response, out Outbound, billing *BillingSession) (int, bool, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		switch category := classifyRelayError(relayContext(info), err); category {
		case ErrorCategoryClientCanceled:
			return statusClientClosedRequest, false, fmt.Errorf("read upstream body: %w", err)
		case ErrorCategoryRelayTimeout:
			return http.StatusGatewayTimeout, false, fmt.Errorf("relay request timeout while reading upstream body: %w", err)
		case ErrorCategoryUpstreamTimeout, ErrorCategoryTimeout:
			return http.StatusGatewayTimeout, true, fmt.Errorf("upstream read timeout: %w", err)
		default:
			return http.StatusBadGateway, true, fmt.Errorf("read upstream body: %w", err)
		}
	}
	if len(body) == 0 {
		return http.StatusBadGateway, true, errEmptyUpstreamResponse
	}
	uniResp, err := adp.ConvertResponse(info, body)
	if err != nil {
		return http.StatusBadGateway, false, fmt.Errorf("convert response: %w", err)
	}

	// 按对外协议序列化
	out.WriteResponse(c, uniResp, info.OriginModel)

	r.logConsume(info, ir, &uniResp.Usage, billing)
	return http.StatusOK, false, nil
}

func (r *Relayer) handleStream(c *gin.Context, info *RelayInfo, ir *dto.UnifiedRequest, adp adaptor.Adaptor, resp *http.Response, out Outbound, billing *BillingSession) (int, bool, error) {
	// 透传上游 request-id，便于排障（需在写出响应头之前设置）
	if info.UpstreamRequestId != "" {
		c.Writer.Header().Set("X-Request-Id", info.UpstreamRequestId)
	}
	var writer StreamWriter
	firstByte := false
	chunkCount := 0

	usage, err := adp.StreamHandler(info, resp, func(chunk *dto.UnifiedStreamChunk) error {
		if !firstByte {
			info.FirstByteMs = int(time.Now().UnixMilli() - info.StartAtMs)
			firstByte = true
		}
		if writer == nil {
			writer = out.NewStream(c, info.RequestID, info.OriginModel)
		}
		chunkCount++
		return writer.WriteChunk(c, chunk)
	})

	if err != nil {
		// 已经开始写流则尝试完成协议尾；首包前失败时不写头，允许故障转移。
		if writer != nil {
			_ = writer.Finish(c)
		}
		category := classifyRelayError(relayContext(info), err)
		if category == ErrorCategoryClientCanceled {
			return statusClientClosedRequest, false, fmt.Errorf("stream: %w", err)
		}
		if category == ErrorCategoryRelayTimeout {
			return http.StatusGatewayTimeout, false, fmt.Errorf("relay request timeout while streaming: %w", err)
		}
		if category == ErrorCategoryUpstreamTimeout || category == ErrorCategoryTimeout {
			if firstByte {
				return http.StatusGatewayTimeout, false, fmt.Errorf("upstream stream timeout after first byte: %w", err)
			}
			return http.StatusGatewayTimeout, true, fmt.Errorf("upstream stream timeout before first byte: %w", err)
		}
		// 非超时错误：若尚未写出任何字节（writer == nil），说明响应头未写，
		// 可安全故障转移到其它渠道；已开始写流则维持不可转移（协议尾已在上方 Finish）。
		if writer == nil {
			return http.StatusBadGateway, true, fmt.Errorf("stream: %w", err)
		}
		return http.StatusOK, false, fmt.Errorf("stream: %w", err)
	}

	// 检测空流（没有收到任何 chunk）。此时尚未写响应头，允许故障转移。
	if chunkCount == 0 {
		return http.StatusBadGateway, true, errEmptyUpstreamResponse
	}

	if writer != nil {
		_ = writer.Finish(c)
	}

	u := dto.Usage{}
	if usage != nil {
		u = *usage
	}
	r.logConsume(info, ir, &u, billing)
	return http.StatusOK, false, nil
}

// ---- 辅助 ----

func (r *Relayer) buildRelayInfo(c *gin.Context, ep constant.EndpointType) *RelayInfo {
	info := &RelayInfo{
		RequestID:    c.GetString(logger.RequestIDKey),
		ClientIP:     c.ClientIP(),
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

func (r *Relayer) logConsume(info *RelayInfo, ir *dto.UnifiedRequest, usage *dto.Usage, billing *BillingSession) {
	useTime := int(time.Now().UnixMilli() - info.StartAtMs)
	effectiveUsage := dto.Usage{}
	if usage != nil {
		effectiveUsage = *usage
	}
	// 部分上游流式响应不返回 usage；至少按请求 prompt 粗略估算，避免成功流式请求完全零计费。
	if info.IsStream && ir != nil && effectiveUsage.PromptTokens == 0 && effectiveUsage.CompletionTokens == 0 && effectiveUsage.TotalTokens == 0 {
		effectiveUsage.PromptTokens = EstimateTokens(ir)
		effectiveUsage.TotalTokens = effectiveUsage.PromptTokens
	}

	// 按实际渠道价格计算消耗额度（微美元）
	var quota int64
	if info.Channel != nil {
		in, out := ResolvePrice(info.Channel, info.OriginModel)
		quota = CalcQuota(effectiveUsage.PromptTokens, effectiveUsage.CompletionTokens, in, out)
	}

	l := &model.Log{
		RequestId:         info.RequestID,
		UpstreamRequestId: info.UpstreamRequestId,
		Type:              model.LogTypeConsume,
		UserId:            info.UserId,
		TokenId:           info.TokenId,
		TokenName:         info.TokenName,
		Group:             info.Group,
		EndpointType:      string(info.EndpointType),
		ApiType:           apiTypeLabel(info),
		SrcModel:          info.OriginModel,
		MappedModel:       info.UpstreamModel,
		IsStream:          info.IsStream,
		PromptTokens:      effectiveUsage.PromptTokens,
		CompletionTokens:  effectiveUsage.CompletionTokens,
		TotalTokens:       effectiveUsage.TotalTokens,
		Quota:             quota,
		UseTimeMs:         useTime,
		FirstByteMs:       info.FirstByteMs,
		Status:            http.StatusOK,
		Ip:                info.ClientIP,
		Content:           info.FailoverChain,
	}
	if info.Channel != nil {
		l.ChannelId = info.Channel.Id
		l.ChannelName = info.Channel.Name
	}

	// 结算额度（预扣 -> 实际）并异步落库，避免阻塞响应。
	info.Settled = true
	if billing != nil && billing.TokenID() > 0 {
		billing.AsyncLogAndSettle(l, quota)
	} else if info.TokenId > 0 {
		model.AsyncLogAndSettle(l, info.TokenId, info.ReservedQuota, quota)
	} else {
		model.AsyncLog(l)
	}
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
		Ip:           info.ClientIP,
		Error:        cleanErrorMessage(errMsg),
		Content:      info.FailoverChain,
	}
	// 记录实际尝试的上游协议（便于排查协议互转问题）
	if info.Channel != nil {
		l.ChannelId = info.Channel.Id
		l.ChannelName = info.Channel.Name
		l.ApiType = apiTypeLabel(info)
	}
	model.AsyncLog(l)
}

// logAttemptFailure 记录单个供应商的转发失败（切换渠道前）。
// 错误信息保持干净（不加供应商前缀，供应商已在 channel 列体现），便于查看完整原因。
func (r *Relayer) logAttemptFailure(info *RelayInfo, ch *model.Channel, status int, errMsg string) {
	l := &model.Log{
		RequestId:    info.RequestID,
		Type:         model.LogTypeError,
		UserId:       info.UserId,
		TokenId:      info.TokenId,
		TokenName:    info.TokenName,
		Group:        info.Group,
		EndpointType: string(info.EndpointType),
		ApiType:      apiTypeLabel(info),
		SrcModel:     info.OriginModel,
		MappedModel:  info.UpstreamModel,
		IsStream:     info.IsStream,
		UseTimeMs:    int(time.Now().UnixMilli() - info.StartAtMs),
		Status:       status,
		Ip:           info.ClientIP,
		Error:        cleanErrorMessage(errMsg),
		Content:      info.FailoverChain,
	}
	if ch != nil {
		l.ChannelId = ch.Id
		l.ChannelName = ch.Name
	}
	model.AsyncLog(l)
}

// apiTypeLabel 返回本次请求实际使用的上游协议可读名（基于解析后的 ApiType）。
func apiTypeLabel(info *RelayInfo) string {
	switch info.ApiType {
	case constant.APITypeAnthropic:
		return "Anthropic"
	case constant.APITypeResponses:
		return "OpenAI-Responses"
	default:
		return "OpenAI"
	}
}

func setSSEHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	if !c.Writer.Written() {
		c.Writer.WriteHeader(http.StatusOK)
	}
}

func statusOrDefault(s int) int {
	if s == 0 {
		return http.StatusServiceUnavailable
	}
	return s
}

func relayContext(info *RelayInfo) context.Context {
	if info != nil && info.Context != nil {
		return info.Context
	}
	return context.Background()
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, // 401 渠道级鉴权失败（key 失效）
		http.StatusForbidden,           // 403 渠道级无权限/资源不可用
		http.StatusNotFound,            // 404 渠道级模型/资源不存在
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}
