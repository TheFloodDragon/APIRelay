package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ChannelKey 表示渠道内的一个上游 API Key。
//
// 一个渠道可挂多个 Key，请求按 KeyIndex 升序「顺序优先」分配：取第一个可用 Key，
// 不可用（冷却中/已失效）才轮到下一个。某个 Key 鉴权失败/额度耗尽/限流/超时时，
// 会被冷却或失效并切换到下一个可用 Key（渠道内故障转移），渠道内 Key 全部耗尽后
// 才由上层切换到其它渠道。
//
// 每个 Key 维护独立的运行时健康与模型配置覆盖：切换 Key 不会复用其它 Key 的模型设置。
type ChannelKey struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	ChannelId int    `json:"channel_id" gorm:"index:idx_ck_channel,priority:1"`
	// KeyIndex 渠道内顺序（顺序优先依据）。越小越优先。
	KeyIndex int    `json:"key_index" gorm:"index:idx_ck_channel,priority:2"`
	Name     string `json:"name" gorm:"size:64"` // 备注，如 "key-1"

	// Key 上游凭据。json:"-" 确保它永不出现在任何 API 响应里；写入走 KeyInput，展示走 KeyMasked。
	Key string `json:"-" gorm:"type:text"`
	// KeyInput 仅接收管理端提交的新凭据，不落库。留空表示「沿用已保存的值」。
	KeyInput string `json:"key,omitempty" gorm:"-"`
	// KeyMasked 供管理端展示的只读掩码（如 sk-1…cdef）。
	KeyMasked string `json:"key_masked,omitempty" gorm:"-"`
	// HasKey 标记该 Key 是否已配置凭据。
	HasKey bool `json:"has_key" gorm:"-"`

	// Status 管理员意图：1 启用 2 禁用（与运行时健康分离）。
	Status int `json:"status" gorm:"default:1"`
	// ModelConfigs Key 级模型配置覆盖（JSON []ChannelModel）；空表示继承渠道级配置。
	// 仅覆盖同名模型的 Protocol/Upstream/Input/Output，不改变对外服务的模型集合。
	ModelConfigs string `json:"model_configs" gorm:"type:text"`

	// —— 运行时健康（持久化便于重启恢复与后台观测；keypool 内存为权威）——
	// Available 运行时是否可用；false 表示已失效，需恢复检测后才重新入轮询队列。
	// 不设 gorm default：否则创建失效 Key（Available=false）时会被 GORM 当作未设置而
	// 回退默认 true，导致失效状态无法持久化（与 Ability.Enabled / Token.Unlimited 同一陷阱）。
	// 所有创建路径（CreateChannelKey/BackfillChannelKeys/虚拟 Key）均显式置 true。
	Available bool `json:"available"`
	// CooldownUntil 冷却截止毫秒时间戳，0 表示未冷却。
	CooldownUntil       int64  `json:"cooldown_until" gorm:"default:0"`
	ConsecutiveFailures int    `json:"consecutive_failures" gorm:"default:0"`
	TotalRequests       int    `json:"total_requests" gorm:"default:0"`
	FailedRequests      int    `json:"failed_requests" gorm:"default:0"`
	// DisabledReason 失效原因：auth_failed|quota_exhausted|rate_limited|upstream_error|manual。
	DisabledReason string `json:"disabled_reason" gorm:"size:32"`
	LastError      string `json:"last_error" gorm:"type:text"`
	LastFailureAt  int64  `json:"last_failure_at" gorm:"default:0"`
	LastSuccessAt  int64  `json:"last_success_at" gorm:"default:0"`
	// PersistVersion 异步落库乱序保护（复用 ChannelHealth 模式）：版本较旧的快照被丢弃。
	PersistVersion uint64 `json:"-" gorm:"default:0"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func (ChannelKey) TableName() string {
	return "channel_keys"
}

const (
	ChannelKeyStatusEnabled  = 1
	ChannelKeyStatusDisabled = 2
)

// DisableReason 描述一个 Key 被冷却或失效的原因。
type DisableReason string

const (
	DisableReasonNone           DisableReason = ""
	DisableReasonAuthFailed     DisableReason = "auth_failed"     // 鉴权失败（401/403 非限流语义）
	DisableReasonQuotaExhausted DisableReason = "quota_exhausted" // 额度耗尽
	DisableReasonRateLimited    DisableReason = "rate_limited"    // 限流（429）
	DisableReasonUpstreamError  DisableReason = "upstream_error"  // 上游错误/超时累计
	DisableReasonManual         DisableReason = "manual"          // 管理员手动
)

// IsVirtual 判断是否为回退兼容层派生的虚拟单 Key（无正 Id，不落库、不主动探测）。
func (k *ChannelKey) IsVirtual() bool {
	return k == nil || k.Id <= 0
}

// PrepareForAPI 填充只读展示字段并清除明文凭据，供所有返回 ChannelKey 的响应统一调用。
func (k *ChannelKey) PrepareForAPI() {
	if k == nil {
		return
	}
	k.HasKey = strings.TrimSpace(k.Key) != ""
	k.KeyMasked = MaskChannelKey(k.Key)
	k.KeyInput = ""
}

// PrepareChannelKeysForAPI 批量填充展示字段。
func PrepareChannelKeysForAPI(list []*ChannelKey) {
	for _, k := range list {
		k.PrepareForAPI()
	}
}

// ApplySubmittedKey 应用管理端提交的凭据：留空表示沿用 existing 的已保存值。
// existing 为 nil（新建）时留空即视为未配置，由调用方校验必填。
func (k *ChannelKey) ApplySubmittedKey(existing *ChannelKey) {
	if k == nil {
		return
	}
	submitted := strings.TrimSpace(k.KeyInput)
	if submitted != "" {
		k.Key = submitted
	} else if existing != nil {
		k.Key = existing.Key
	} else {
		k.Key = ""
	}
	k.KeyInput = ""
}

// ModelConfigList 解析 Key 级模型配置覆盖；为空返回 nil（表示继承渠道级配置）。
func (k *ChannelKey) ModelConfigList() []ChannelModel {
	if k == nil || strings.TrimSpace(k.ModelConfigs) == "" {
		return nil
	}
	var list []ChannelModel
	if err := json.Unmarshal([]byte(k.ModelConfigs), &list); err != nil {
		return nil
	}
	return list
}

// ---- CRUD ----

// ListChannelKeys 返回某渠道全部 Key，按 KeyIndex 升序（顺序优先）。
func ListChannelKeys(channelId int) ([]*ChannelKey, error) {
	var list []*ChannelKey
	err := DB.Where("channel_id = ?", channelId).Order("key_index asc, id asc").Find(&list).Error
	return list, err
}

// GetChannelKeyByID 按 ID 查询 Key。
func GetChannelKeyByID(id int) (*ChannelKey, error) {
	var k ChannelKey
	if err := DB.First(&k, id).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

// ListRecoverableChannelKeys 返回「管理员启用但运行时失效」的 Key（供主动探测恢复 worker 扫描）。
// 仅返回 Status==Enabled 且 Available==false 的记录；管理员禁用的 Key 不参与恢复。
func ListRecoverableChannelKeys(limit int) ([]*ChannelKey, error) {
	if DB == nil {
		return nil, nil
	}
	q := DB.Where("status = ? AND available = ?", ChannelKeyStatusEnabled, false).
		Order("last_failure_at asc, id asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var list []*ChannelKey
	err := q.Find(&list).Error
	return list, err
}

// CountChannelKeys 统计某渠道的 Key 总数。
func CountChannelKeys(channelId int) (int64, error) {
	var n int64
	err := DB.Model(&ChannelKey{}).Where("channel_id = ?", channelId).Count(&n).Error
	return n, err
}

// nextChannelKeyIndex 返回某渠道下一个可用的 KeyIndex（现有最大值 +1）。
// 用 COALESCE 兜底空表的 MAX() NULL，Scan 到普通 int 避免 **int 的不确定行为。
func nextChannelKeyIndex(tx *gorm.DB, channelId int) (int, error) {
	var maxIdx int
	if err := tx.Model(&ChannelKey{}).Where("channel_id = ?", channelId).
		Select("COALESCE(MAX(key_index), -1)").Scan(&maxIdx).Error; err != nil {
		return 0, err
	}
	return maxIdx + 1, nil
}

// CreateChannelKey 创建一个渠道 Key。未指定 KeyIndex（<0）时自动追加到末尾。
func CreateChannelKey(k *ChannelKey) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if k.KeyIndex < 0 {
			idx, err := nextChannelKeyIndex(tx, k.ChannelId)
			if err != nil {
				return err
			}
			k.KeyIndex = idx
		}
		if k.Status == 0 {
			k.Status = ChannelKeyStatusEnabled
		}
		k.Available = true
		k.CooldownUntil = 0
		k.CreatedAt = nowMilli()
		k.UpdatedAt = k.CreatedAt
		return tx.Create(k).Error
	})
}

// UpdateChannelKey 更新 Key 的管理字段（凭据/备注/状态/模型配置/顺序）。
// 不覆盖运行时健康字段——它们由 keypool 异步落库维护。
func UpdateChannelKey(k *ChannelKey) error {
	k.UpdatedAt = nowMilli()
	return DB.Model(&ChannelKey{}).Where("id = ?", k.Id).Updates(map[string]any{
		"name":          k.Name,
		"key":           k.Key,
		"status":        k.Status,
		"model_configs": k.ModelConfigs,
		"key_index":     k.KeyIndex,
		"updated_at":    k.UpdatedAt,
	}).Error
}

// DeleteChannelKey 删除单个 Key。
func DeleteChannelKey(id int) error {
	return DB.Delete(&ChannelKey{}, id).Error
}

// DeleteChannelKeysByChannel 删除某渠道的全部 Key（渠道删除时级联调用）。
func DeleteChannelKeysByChannel(tx *gorm.DB, channelId int) error {
	return tx.Where("channel_id = ?", channelId).Delete(&ChannelKey{}).Error
}

// ReorderChannelKeys 按给定 ID 顺序重排 KeyIndex（首位最高优先，index=0）。
func ReorderChannelKeys(channelId int, orderedIDs []int) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			if err := tx.Model(&ChannelKey{}).Where("id = ? AND channel_id = ?", id, channelId).
				Updates(map[string]any{"key_index": i, "updated_at": nowMilli()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetChannelKeyCooldown 设置 Key 冷却截止时间（运行时软冷却）。
func SetChannelKeyCooldown(id int, until int64) {
	if DB == nil || id <= 0 {
		return
	}
	DB.Model(&ChannelKey{}).Where("id = ?", id).Update("cooldown_until", until)
}

// ClearChannelKeyCooldown 清除 Key 冷却（仅当当前确有冷却时更新）。
func ClearChannelKeyCooldown(id int) {
	if DB == nil || id <= 0 {
		return
	}
	DB.Model(&ChannelKey{}).Where("id = ? AND cooldown_until > 0", id).Update("cooldown_until", 0)
}

// UpsertChannelKeyHealth 落库 Key 的运行时健康快照；版本较旧的异步快照会被忽略。
// 仅更新运行时字段，不触碰管理字段（凭据/备注/状态/模型配置/顺序）。
func UpsertChannelKeyHealth(k *ChannelKey) error {
	if k == nil || k.Id <= 0 || DB == nil {
		return nil
	}
	k.UpdatedAt = nowMilli()
	values := map[string]interface{}{
		"available":            k.Available,
		"cooldown_until":       k.CooldownUntil,
		"consecutive_failures": k.ConsecutiveFailures,
		"total_requests":       k.TotalRequests,
		"failed_requests":      k.FailedRequests,
		"disabled_reason":      k.DisabledReason,
		"last_error":           k.LastError,
		"last_failure_at":      k.LastFailureAt,
		"last_success_at":      k.LastSuccessAt,
		"persist_version":      k.PersistVersion,
		"updated_at":           k.UpdatedAt,
	}
	return DB.Model(&ChannelKey{}).
		Where("id = ? AND persist_version <= ?", k.Id, k.PersistVersion).
		Updates(values).Error
}

// SetChannelKeyDisabled 手动/自动标记 Key 失效（落库）。运行时权威仍在 keypool，
// 此函数供管理 API 与迁移等非请求路径直接落库使用。
func SetChannelKeyDisabled(id int, reason DisableReason, lastErr string) error {
	if DB == nil || id <= 0 {
		return nil
	}
	return DB.Model(&ChannelKey{}).Where("id = ?", id).Updates(map[string]any{
		"available":       false,
		"disabled_reason": string(reason),
		"last_error":      truncateKeyError(lastErr),
		"last_failure_at": nowMilli(),
		"updated_at":      nowMilli(),
	}).Error
}

// ResetChannelKey 手动重置 Key 的运行时健康（清冷却/失效/失败计数），使其重新入轮询队列。
// 推进 persist_version，避免在途的旧快照回写覆盖本次重置。
func ResetChannelKey(id int) error {
	if DB == nil || id <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var k ChannelKey
		if err := tx.Select("id", "persist_version").First(&k, id).Error; err != nil {
			return err
		}
		return tx.Model(&ChannelKey{}).Where("id = ?", id).Updates(map[string]any{
			"available":            true,
			"cooldown_until":       0,
			"consecutive_failures": 0,
			"disabled_reason":      "",
			"last_error":           "",
			"persist_version":      k.PersistVersion + 1,
			"updated_at":           nowMilli(),
		}).Error
	})
}

// truncateKeyError 截断 Key 的错误信息，避免无界文本写入。
func truncateKeyError(s string) string {
	const max = 500
	r := []rune(strings.TrimSpace(s))
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return string(r)
}

// ---- 回退兼容层 ----

// LoadEnabledChannelKeys 返回某渠道全部「管理员启用」的 Key（Status==Enabled），按 KeyIndex 升序。
//
// 回退兼容：渠道在 channel_keys 中无任何记录时（旧渠道 / 测试直接构造的 Channel），
// 从 Channel.Key 派生一个虚拟单 Key（Id=0，不落库、不主动探测），保证旧转发路径与测试继续工作。
// 若渠道有子表记录但全部被管理员禁用，则返回空切片（无可用 Key）。
func LoadEnabledChannelKeys(ch *Channel) ([]*ChannelKey, error) {
	if ch == nil {
		return nil, nil
	}
	if DB != nil && ch.Id > 0 {
		total, err := CountChannelKeys(ch.Id)
		if err != nil {
			return nil, err
		}
		if total > 0 {
			var list []*ChannelKey
			if err := DB.Where("channel_id = ? AND status = ?", ch.Id, ChannelKeyStatusEnabled).
				Order("key_index asc, id asc").Find(&list).Error; err != nil {
				return nil, err
			}
			return list, nil
		}
	}
	// 回退：从渠道级单 Key 派生虚拟 Key。
	if strings.TrimSpace(ch.Key) == "" {
		return nil, nil
	}
	return []*ChannelKey{virtualKeyFromChannel(ch)}, nil
}

// virtualKeyFromChannel 由渠道级单 Key 构造虚拟 ChannelKey（Id=0）。
func virtualKeyFromChannel(ch *Channel) *ChannelKey {
	return &ChannelKey{
		Id:        0,
		ChannelId: ch.Id,
		KeyIndex:  0,
		Key:       ch.Key,
		Status:    ChannelKeyStatusEnabled,
		Available: true,
	}
}

// ---- 管理端单 key 表单与多 Key 子表的同步 ----

// EnsurePrimaryChannelKey 在渠道创建后，用渠道级单 key 建立首个 ChannelKey（幂等）。
// 使管理端的单 key 表单也纳入多 Key 轮询路径。已有任何 Key 记录时不重复创建。
// 失败仅记录不阻断：回退兼容层仍会由 Channel.Key 派生虚拟 Key，转发不受影响。
func EnsurePrimaryChannelKey(ch *Channel) {
	if ch == nil || ch.Id <= 0 || strings.TrimSpace(ch.Key) == "" || DB == nil {
		return
	}
	var count int64
	if err := DB.Model(&ChannelKey{}).Where("channel_id = ?", ch.Id).Count(&count).Error; err != nil || count > 0 {
		return
	}
	now := nowMilli()
	_ = DB.Create(&ChannelKey{
		ChannelId: ch.Id, KeyIndex: 0, Name: "key-1", Key: ch.Key,
		Status: ChannelKeyStatusEnabled, Available: true, CreatedAt: now, UpdatedAt: now,
	}).Error
}

// SyncPrimaryChannelKey 在管理端通过单 key 表单更新渠道凭据后，同步首个 ChannelKey 的凭据。
//
// 仅当渠道恰好只有一个 ChannelKey（即由单 key 表单派生、未通过多 Key 接口扩展）时才同步，
// 避免误改用户通过 /channels/:id/keys 独立管理的多 Key。渠道尚无任何 Key 时补建首个。
func SyncPrimaryChannelKey(ch *Channel) {
	if ch == nil || ch.Id <= 0 || strings.TrimSpace(ch.Key) == "" || DB == nil {
		return
	}
	var keys []*ChannelKey
	if err := DB.Where("channel_id = ?", ch.Id).Order("key_index asc, id asc").Find(&keys).Error; err != nil {
		return
	}
	if len(keys) == 0 {
		EnsurePrimaryChannelKey(ch)
		return
	}
	if len(keys) == 1 && keys[0].Key != ch.Key {
		DB.Model(&ChannelKey{}).Where("id = ?", keys[0].Id).
			Updates(map[string]any{"key": ch.Key, "updated_at": nowMilli()})
	}
}

// ---- 迁移回填 ----

// BackfillChannelKeys 幂等回填：为每个已配置 Key 且 channel_keys 尚无记录的渠道，
// 插入一条 KeyIndex=0 的 ChannelKey。启动迁移后调用；已迁移的渠道跳过。
func BackfillChannelKeys() error {
	if DB == nil {
		return nil
	}
	var channels []*Channel
	if err := DB.Find(&channels).Error; err != nil {
		return fmt.Errorf("load channels for key backfill: %w", err)
	}
	for _, ch := range channels {
		if strings.TrimSpace(ch.Key) == "" {
			continue
		}
		var count int64
		if err := DB.Model(&ChannelKey{}).Where("channel_id = ?", ch.Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue // 已迁移
		}
		now := nowMilli()
		k := &ChannelKey{
			ChannelId: ch.Id,
			KeyIndex:  0,
			Name:      "key-1",
			Key:       ch.Key,
			Status:    ChannelKeyStatusEnabled,
			Available: true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := DB.Create(k).Error; err != nil {
			return fmt.Errorf("backfill channel key for channel %d: %w", ch.Id, err)
		}
	}
	return nil
}
