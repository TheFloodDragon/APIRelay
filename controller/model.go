package controller

import (
	"net/http"
	"sort"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay"

	"github.com/gin-gonic/gin"
)

// AggregatedModel 是按显示名聚合后的模型条目。
type AggregatedModel struct {
	Name       string                 `json:"name"`
	LastUsedAt int64                  `json:"last_used_at"`
	Providers  []AggregatedModelOwner `json:"providers"`
}

// AggregatedModelOwner 描述提供某模型的一个供应商。
type AggregatedModelOwner struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Group       string `json:"group"`
	Enabled     bool   `json:"enabled"`
	Protocol    string `json:"protocol"` // 解析后的协议名（继承时显示实际生效协议）
	Upstream    string `json:"upstream"`
}

// ListAggregatedModels GET /api/models
// 聚合所有渠道的模型，按显示名分组（允许跨供应商重复）。
func ListAggregatedModels(c *gin.Context) {
	channels, err := model.ListChannels()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	lastUsed, err := model.ListModelLastUsed()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	grouped := map[string]*AggregatedModel{}
	order := []string{}
	for _, ch := range channels {
		for _, m := range ch.ModelConfigList() {
			if m.Name == "" {
				continue
			}
			agg, ok := grouped[m.Name]
			if !ok {
				agg = &AggregatedModel{Name: m.Name, LastUsedAt: lastUsed[m.Name]}
				grouped[m.Name] = agg
				order = append(order, m.Name)
			}
			agg.Providers = append(agg.Providers, AggregatedModelOwner{
				ChannelId:   ch.Id,
				ChannelName: ch.Name,
				Group:       ch.Group,
				Enabled:     m.Enabled && ch.Status == model.ChannelStatusEnabled,
				Protocol:    resolveProtocolName(ch, m),
				Upstream:    m.Upstream,
			})
		}
	}

	out := make([]*AggregatedModel, 0, len(order))
	for _, name := range order {
		out = append(out, grouped[name])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastUsedAt == out[j].LastUsedAt {
			return out[i].Name < out[j].Name
		}
		return out[i].LastUsedAt > out[j].LastUsedAt
	})
	ok(c, out)
}

// resolveProtocolName 返回模型实际生效的协议名（用于聚合视图展示）。
func resolveProtocolName(ch *model.Channel, m model.ChannelModel) string {
	return constant.APITypeName(relay.ResolveAPIType(ch, m.Name))
}
