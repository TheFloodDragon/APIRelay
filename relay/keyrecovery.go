package relay

import (
	"context"

	"github.com/apirelay/apirelay/model"
)

// RecoverProbe 是注入给 keypool 恢复 worker 的探测实现（keypool.ProbeFunc）。
//
// 用选中 Key 的 overlay 视图对上游做一次轻量【模型列表】探测：能成功列出模型即视为
// 该 Key 恢复可用。模型列表探测不消耗对话额度，主要验证【连通性 + 鉴权】——这正是
// 绝大多数 Key 失效（auth_failed / 上游错误）的判定依据。
//
// 局限（已知且可接受）：额度耗尽（quota_exhausted）的 Key，其模型列表端点通常仍返回 200，
// 因此可能被误判为恢复；此时下一次真实请求会再次将其失效（有界抖动，每恢复间隔至多一次），
// 若额度确已恢复则正常回归。真正的额度探测需消耗 token，成本更高，故不在此默认路径中执行。
//
// 该函数放在 relay 主包（可访问 ProbeModelsContext），由 main.go 注入给 keypool 恢复 worker，
// 从而避免 keypool ← relay 的循环依赖。
func RecoverProbe(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error {
	overlay := model.BuildKeyOverlay(ch, key)
	_, err := ProbeModelsContext(ctx, overlay)
	return err
}
