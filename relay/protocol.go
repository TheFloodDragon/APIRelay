package relay

import (
	"regexp"
	"sync"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
)

// ResolveAPIType 解析某渠道下指定显示模型应使用的上游协议。
//
// 优先级（越具体越优先 / 局部优先）：
//  1. 模型显式协议覆盖（ChannelModel.Protocol）
//  2. 供应商级正则规则命中
//  3. 全局正则规则命中
//  4. 供应商默认协议（Channel.Type）—— 最末兜底
//
// 说明：供应商默认协议恒有值，故置于最末，否则全局正则永不触发。
func ResolveAPIType(ch *model.Channel, displayModel string) constant.APIType {
	// 1. 模型显式协议
	if m, ok := ch.ModelConfig(displayModel); ok && m.Protocol != "" {
		if t, hit := constant.APITypeFromName(m.Protocol); hit {
			return t
		}
	}

	// 2. 供应商正则规则
	if t, ok := matchRules(ch.ProtocolRuleList(), displayModel); ok {
		return t
	}

	// 3. 全局正则规则
	if t, ok := matchRules(model.GetGlobalProtocolRules(), displayModel); ok {
		return t
	}

	// 4. 供应商默认协议
	return ch.APIType()
}

// matchRules 按顺序匹配规则，返回首个命中的协议。
func matchRules(rules []model.ProtocolRule, displayModel string) (constant.APIType, bool) {
	for _, r := range rules {
		if r.Pattern == "" || r.Protocol == "" {
			continue
		}
		re := compileRegexp(r.Pattern)
		if re == nil {
			continue
		}
		if re.MatchString(displayModel) {
			if t, hit := constant.APITypeFromName(r.Protocol); hit {
				return t, true
			}
		}
	}
	return constant.APITypeOpenAI, false
}

// ---- 正则编译缓存 ----

var (
	regexpMu    sync.RWMutex
	regexpCache = map[string]*regexp.Regexp{}
)

// compileRegexp 编译并缓存正则；无效正则返回 nil（并缓存 nil 避免重复编译）。
func compileRegexp(pattern string) *regexp.Regexp {
	regexpMu.RLock()
	re, ok := regexpCache[pattern]
	regexpMu.RUnlock()
	if ok {
		return re
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		compiled = nil
	}
	regexpMu.Lock()
	regexpCache[pattern] = compiled
	regexpMu.Unlock()
	return compiled
}
