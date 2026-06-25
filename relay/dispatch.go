package relay

import (
	"math/rand"

	"github.com/apirelay/apirelay/model"
)

// SelectChannel 从候选渠道中按 weight 加权随机选择一个（已过滤冷却与排除）。
func SelectChannel(group, mappedModel string, excluded map[int]struct{}, nowMs int64) (*model.Channel, error) {
	channels, err := model.GetChannelsForModel(group, mappedModel, excluded)
	if err != nil {
		return nil, err
	}
	// 过滤冷却中的渠道
	avail := make([]*model.Channel, 0, len(channels))
	for _, c := range channels {
		if c.CooldownUntil > nowMs {
			continue
		}
		avail = append(avail, c)
	}
	if len(avail) == 0 {
		return nil, nil
	}
	return weightedPick(avail), nil
}

// weightedPick 按 weight 加权随机选择（weight+1 保证非零渠道也有机会）。
func weightedPick(channels []*model.Channel) *model.Channel {
	total := 0
	for _, c := range channels {
		total += c.Weight + 1
	}
	if total <= 0 {
		return channels[0]
	}
	r := rand.Intn(total)
	for _, c := range channels {
		r -= c.Weight + 1
		if r < 0 {
			return c
		}
	}
	return channels[len(channels)-1]
}
