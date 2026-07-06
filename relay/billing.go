package relay

import (
	"math"
	"sync"

	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
)

// 计费：额度单位为微美元（micro-USD），1 USD = 1_000_000 微美元。
// 价格单位为 USD / 1M tokens。

const (
	// microUSDPerUSD 1 美元 = 1_000_000 微美元。
	microUSDPerUSD = 1_000_000.0
	// tokensPerPriceUnit 价格以每 100 万 token 计。
	tokensPerPriceUnit = 1_000_000.0
	// reserveSafetyMultiplier 预扣安全上浮，降低实际渠道价/输出偏差导致结算补扣失败的概率。
	reserveSafetyMultiplier = 1.2
	// estimatedImageTokens 粗略估算单张图片输入消耗。
	estimatedImageTokens = 1000
)

// BillingSession 管理一次请求的预扣、结算与失败退款，保证重复调用不会重复扣退。
type BillingSession struct {
	mu       sync.Mutex
	tokenID  int
	reserved int64
	settled  bool
	refunded bool
}

// NewBillingSession 创建一次请求的计费会话。
func NewBillingSession(tokenID int) *BillingSession {
	if tokenID <= 0 {
		return nil
	}
	return &BillingSession{tokenID: tokenID}
}

// TokenID 返回会话绑定的 token id。
func (b *BillingSession) TokenID() int {
	if b == nil {
		return 0
	}
	return b.tokenID
}

// Reserved 返回已预扣额度。
func (b *BillingSession) Reserved() int64 {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reserved
}

// Reserve 预扣额度。预扣成功后纳入会话管理，后续可被一次性结算或退款。
func (b *BillingSession) Reserve(amount int64) error {
	if b == nil || amount <= 0 {
		return nil
	}
	if err := model.PreConsumeQuota(b.tokenID, amount); err != nil {
		return err
	}
	b.mu.Lock()
	b.reserved += amount
	b.mu.Unlock()
	return nil
}

// Settle 同步结算额度，重复调用仅第一次生效。
func (b *BillingSession) Settle(actual int64) error {
	reserved, ok := b.markSettled()
	if !ok {
		return nil
	}
	return model.SettleQuota(b.tokenID, reserved, actual)
}

// AsyncLogAndSettle 异步写消费日志并结算额度，重复调用仅第一次触发结算。
func (b *BillingSession) AsyncLogAndSettle(l *model.Log, actual int64) {
	reserved, ok := b.markSettled()
	if !ok {
		model.AsyncLog(l)
		return
	}
	model.AsyncLogAndSettle(l, b.tokenID, reserved, actual)
}

// Refund 失败路径退款，重复调用仅第一次生效；已结算的会话不会退款。
func (b *BillingSession) Refund() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	if b.settled || b.refunded || b.reserved <= 0 {
		b.mu.Unlock()
		return false
	}
	b.refunded = true
	reserved := b.reserved
	tokenID := b.tokenID
	b.mu.Unlock()

	model.RefundQuota(tokenID, reserved)
	return true
}

func (b *BillingSession) markSettled() (int64, bool) {
	if b == nil {
		return 0, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.settled || b.refunded {
		return 0, false
	}
	b.settled = true
	return b.reserved, true
}

// ResolvePrice 解析某渠道下指定显示模型的 input/output 价格（USD / 1M tokens）。
//
// 优先级（局部优先，与协议解析一致）：
//  1. 渠道模型显式价格（ChannelModel.Input/Output，任一 > 0 即视为已配置）
//  2. 全局价格表（按模型名精确匹配，回退 "default" 条目）
//  3. 未配置 -> 0（不计费）
func ResolvePrice(ch *model.Channel, displayModel string) (input, output float64) {
	if ch != nil {
		if m, ok := ch.ModelConfig(displayModel); ok && (m.Input > 0 || m.Output > 0) {
			return m.Input, m.Output
		}
	}
	if in, out, hit := model.LookupGlobalModelPrice(displayModel); hit {
		return in, out
	}
	return 0, 0
}

// CalcQuota 按 token 用量与价格计算消耗的微美元额度（向上取整到整数微美元）。
func CalcQuota(promptTokens, completionTokens int, inputPrice, outputPrice float64) int64 {
	if inputPrice <= 0 && outputPrice <= 0 {
		return 0
	}
	costUSD := (float64(promptTokens)*inputPrice + float64(completionTokens)*outputPrice) / tokensPerPriceUnit
	micro := costUSD * microUSDPerUSD
	if micro <= 0 {
		return 0
	}
	// 向上取整，避免长期累积少扣
	q := int64(micro)
	if float64(q) < micro {
		q++
	}
	return q
}

// EstimateTokens 粗略估算请求的 prompt token 数（用于预扣）。
// 经验公式：字符数 / 4 + 每条消息固定开销，附加 system 与工具定义长度。
func EstimateTokens(ir *dto.UnifiedRequest) int {
	if ir == nil {
		return 0
	}
	chars := len(ir.System)
	imageCount := 0
	for _, m := range ir.Messages {
		chars += len(m.Content)
		for _, p := range m.Parts {
			chars += len(p.Text)
			if p.Type == "image_url" && p.ImageURL != "" {
				imageCount++
				// URL 本身也有少量文本开销。
				chars += len(p.ImageURL)
			}
		}
		for _, tc := range m.ToolCalls {
			chars += len(tc.Arguments) + len(tc.Name)
		}
		chars += 12 // 每条消息角色/分隔开销
	}
	for _, t := range ir.Tools {
		chars += len(t.Name) + len(t.Description) + len(t.Parameters)
	}
	est := chars/4 + imageCount*estimatedImageTokens + 8
	if est < 1 {
		est = 1
	}
	return est
}

// estimateCompletionTokens 预扣时对补全 token 的保守估计。
// 优先用请求声明的 max_tokens，否则给一个默认上限。
func estimateCompletionTokens(ir *dto.UnifiedRequest) int {
	if ir != nil && ir.MaxTokens != nil && *ir.MaxTokens > 0 {
		return *ir.MaxTokens
	}
	return 1024
}

// EstimateQuota 预扣阶段估算需要冻结的额度（微美元）。
// 保留兼容旧调用方：仅基于全局价格表估算，并附加安全上浮。
func EstimateQuota(ir *dto.UnifiedRequest) int64 {
	if ir == nil {
		return 0
	}
	in, out, hit := model.LookupGlobalModelPrice(ir.Model)
	if !hit || (in <= 0 && out <= 0) {
		return 0
	}
	return applyReserveSafety(CalcQuota(EstimateTokens(ir), estimateCompletionTokens(ir), in, out))
}

// EstimateQuotaForCandidates 基于候选渠道价格上界估算预扣额度。
// 预扣发生在最终选定渠道之前，因此取所有候选渠道（含全局价格回退）的最大 input/output 价格，
// 避免渠道级价格高于全局价格时少预扣，导致成功请求在异步结算阶段补扣失败。
func EstimateQuotaForCandidates(ir *dto.UnifiedRequest, candidates []model.ChannelCandidate) int64 {
	if ir == nil {
		return 0
	}
	in, out := maxCandidatePrice(ir.Model, candidates)
	if in <= 0 && out <= 0 {
		return 0
	}
	return applyReserveSafety(CalcQuota(EstimateTokens(ir), estimateCompletionTokens(ir), in, out))
}

func maxCandidatePrice(displayModel string, candidates []model.ChannelCandidate) (input, output float64) {
	if in, out, hit := model.LookupGlobalModelPrice(displayModel); hit {
		input, output = in, out
	}
	for _, cand := range candidates {
		if cand.Channel == nil {
			continue
		}
		in, out := ResolvePrice(cand.Channel, displayModel)
		if in > input {
			input = in
		}
		if out > output {
			output = out
		}
	}
	return input, output
}

func applyReserveSafety(quota int64) int64 {
	if quota <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(quota) * reserveSafetyMultiplier))
}
