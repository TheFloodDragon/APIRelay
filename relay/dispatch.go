package relay

import (
	"math/rand/v2"

	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/circuitbreaker"
)

// SelectChannel 从候选渠道中选择一个用于本次（重试）请求。
//
// 调度策略（融合 new-api 思路）：
//  1. 拉取 group+model（含通配符 *）的全部可用渠道；
//  2. 过滤掉 excluded（本次已失败）与冷却中的渠道；
//  3. 在剩余渠道中取【最高优先级层】；
//  4. 该层内按 weight 加权随机。
//
// 这样 failover 排除高优先级渠道后，能自动降级到次高优先级层。
func SelectChannel(group, model_ string, excluded map[int]struct{}, nowMs int64) (*model.Channel, error) {
	candidates, err := model.GetChannelCandidates(group, model_)
	if err != nil {
		return nil, err
	}
	return SelectFromCandidates(candidates, excluded, nowMs), nil
}

// SelectFromCandidates 从已加载候选中选择渠道。
// 注意：过滤阶段只 Peek 熔断状态，避免 half-open 候选仅因被检查就占用探测名额；
// 真正选中后才调用 IsChannelAllowed 占用名额。如并发竞争导致占用失败，会临时跳过该渠道重选。
func SelectFromCandidates(candidates []model.ChannelCandidate, excluded map[int]struct{}, nowMs int64) *model.Channel {
	if len(candidates) == 0 {
		return nil
	}
	mgr := circuitbreaker.GetManager()
	skipped := map[int]struct{}{}

	for {
		avail := make([]model.ChannelCandidate, 0, len(candidates))
		for _, cand := range candidates {
			if cand.Channel == nil {
				continue
			}
			if excluded != nil {
				if _, skip := excluded[cand.Channel.Id]; skip {
					continue
				}
			}
			if _, skip := skipped[cand.Channel.Id]; skip {
				continue
			}
			if cand.Channel.CooldownUntil > nowMs {
				continue
			}
			if !mgr.PeekChannelAllowed(cand.Channel.Id) {
				continue
			}
			avail = append(avail, cand)
		}
		if len(avail) == 0 {
			return nil
		}

		// 取最高优先级层
		topPriority := avail[0].Priority
		for _, cand := range avail {
			if cand.Priority > topPriority {
				topPriority = cand.Priority
			}
		}
		tier := make([]model.ChannelCandidate, 0, len(avail))
		for _, cand := range avail {
			if cand.Priority == topPriority {
				tier = append(tier, cand)
			}
		}

		picked := weightedPick(tier)
		if picked == nil {
			return nil
		}
		if mgr.IsChannelAllowed(picked.Id) {
			return picked
		}
		skipped[picked.Id] = struct{}{}
	}
}

// weightedPick 按 weight 加权随机选择（weight+1 保证零权重渠道也有机会）。
func weightedPick(tier []model.ChannelCandidate) *model.Channel {
	if len(tier) == 1 {
		return tier[0].Channel
	}
	total := 0
	for _, cand := range tier {
		total += cand.Weight + 1
	}
	if total <= 0 {
		return tier[0].Channel
	}
	r := rand.IntN(total)
	for _, cand := range tier {
		r -= cand.Weight + 1
		if r < 0 {
			return cand.Channel
		}
	}
	return tier[len(tier)-1].Channel
}
