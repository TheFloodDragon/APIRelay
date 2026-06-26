package model

// Ability 是 (group, model) -> channel 的倒排索引，用于按模型快速选渠道。
type Ability struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Group     string `json:"group" gorm:"size:64;index:idx_group_model,priority:1"`
	Model     string `json:"model" gorm:"size:128;index:idx_group_model,priority:2"`
	ChannelId int    `json:"channel_id" gorm:"index"`
	// Enabled 不设 gorm default，否则创建禁用渠道的 Ability（Enabled=false）时
	// 会被 GORM 当作未设置而回退默认 true，导致禁用渠道仍被选中。
	Enabled  bool `json:"enabled" gorm:"index"`
	Priority int  `json:"priority" gorm:"default:0;index"`
	Weight   int  `json:"weight" gorm:"default:1"`
}

// WildcardModel 通配符模型名，渠道支持该模型时可服务任意模型请求。
const WildcardModel = "*"

// SyncChannelAbilities 重建某渠道的 Ability 索引（每个 model 一行）。
func SyncChannelAbilities(c *Channel) error {
	if err := DB.Where("channel_id = ?", c.Id).Delete(&Ability{}).Error; err != nil {
		return err
	}
	enabled := c.Status == ChannelStatusEnabled
	var abilities []Ability
	for _, m := range c.EnabledModelNames() {
		abilities = append(abilities, Ability{
			Group:     c.Group,
			Model:     m,
			ChannelId: c.Id,
			Enabled:   enabled,
			Priority:  c.Priority,
			Weight:    c.Weight,
		})
	}
	if len(abilities) == 0 {
		return nil
	}
	return DB.Create(&abilities).Error
}

// ResyncAllAbilities 重建所有渠道的 Ability 索引（启动时调用，自愈历史脏数据）。
func ResyncAllAbilities() error {
	if DB == nil {
		return nil
	}
	var channels []*Channel
	if err := DB.Find(&channels).Error; err != nil {
		return err
	}
	for _, c := range channels {
		if err := SyncChannelAbilities(c); err != nil {
			return err
		}
	}
	return nil
}

// ChannelCandidate 是一个候选渠道及其调度元数据。
type ChannelCandidate struct {
	Channel  *Channel
	Priority int
	Weight   int
}

// GetChannelCandidates 返回某 group+model 下全部可用候选渠道（含优先级与权重）。
// 同时匹配精确模型名与通配符模型 "*"。结果未排序，由调度层处理分层与加权。
func GetChannelCandidates(group, model string) ([]ChannelCandidate, error) {
	var abilities []Ability
	err := DB.Where("`group` = ? AND model IN ? AND enabled = ?",
		group, []string{model, WildcardModel}, true).
		Find(&abilities).Error
	if err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	// 同一渠道可能因精确+通配两条记录重复，去重并取较高优先级
	best := make(map[int]Ability, len(abilities))
	ids := make([]int, 0, len(abilities))
	for _, a := range abilities {
		if cur, ok := best[a.ChannelId]; !ok || a.Priority > cur.Priority {
			if !ok {
				ids = append(ids, a.ChannelId)
			}
			best[a.ChannelId] = a
		}
	}

	var channels []*Channel
	if err := DB.Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	chMap := make(map[int]*Channel, len(channels))
	for _, c := range channels {
		chMap[c.Id] = c
	}

	out := make([]ChannelCandidate, 0, len(ids))
	for _, id := range ids {
		c, ok := chMap[id]
		if !ok {
			continue
		}
		a := best[id]
		out = append(out, ChannelCandidate{Channel: c, Priority: a.Priority, Weight: a.Weight})
	}
	return out, nil
}

// GetAvailableModels 返回某分组下所有启用渠道支持的模型列表（去重）。
func GetAvailableModels(group string) ([]string, error) {
	var abilities []Ability
	err := DB.Where("`group` = ? AND enabled = ? AND model != ?",
		group, true, WildcardModel).
		Distinct("model").
		Order("model").
		Find(&abilities).Error
	if err != nil {
		return nil, err
	}

	models := make([]string, 0, len(abilities))
	for _, a := range abilities {
		models = append(models, a.Model)
	}
	return models, nil
}

// ModelAvailability 描述某模型在系统中的可用性诊断信息。
type ModelAvailability struct {
	// EnabledProviders 在【请求分组】下已启用、可服务该模型的供应商名。
	EnabledProviders []string
	// OtherGroupProviders 在【其它分组】下配置了该模型的供应商名（分组不匹配）。
	OtherGroupProviders []string
	// DisabledProviders 配置了该模型但当前被禁用的供应商名。
	DisabledProviders []string
	// HasWildcard 请求分组下是否存在通配符 "*" 渠道。
	HasWildcard bool
}

// DiagnoseModel 诊断某 group+model 为何不可用，用于生成可操作的错误提示。
func DiagnoseModel(group, modelName string) ModelAvailability {
	var diag ModelAvailability
	if DB == nil {
		return diag
	}

	// 1) 请求分组下已启用、命中该模型（精确或通配）的渠道
	var enabled []Ability
	DB.Where("`group` = ? AND model IN ? AND enabled = ?",
		group, []string{modelName, WildcardModel}, true).Find(&enabled)
	enabledIDs := map[int]struct{}{}
	for _, a := range enabled {
		if a.Model == WildcardModel {
			diag.HasWildcard = true
		}
		enabledIDs[a.ChannelId] = struct{}{}
	}
	diag.EnabledProviders = channelNames(enabledIDs)

	// 2) 其它分组下配置了该模型的渠道（分组写错的常见情形）
	var otherGroup []Ability
	DB.Where("`group` <> ? AND model = ?", group, modelName).Find(&otherGroup)
	ogIDs := map[int]struct{}{}
	for _, a := range otherGroup {
		ogIDs[a.ChannelId] = struct{}{}
	}
	diag.OtherGroupProviders = channelNames(ogIDs)

	// 3) 配置了该模型但被禁用的渠道（请求分组）
	var disabled []Ability
	DB.Where("`group` = ? AND model = ? AND enabled = ?", group, modelName, false).Find(&disabled)
	disIDs := map[int]struct{}{}
	for _, a := range disabled {
		disIDs[a.ChannelId] = struct{}{}
	}
	diag.DisabledProviders = channelNames(disIDs)

	return diag
}

// channelNames 按 ID 集合查询渠道名（去重、稳定输出）。
func channelNames(ids map[int]struct{}) []string {
	if len(ids) == 0 {
		return nil
	}
	idList := make([]int, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	var channels []*Channel
	if err := DB.Where("id IN ?", idList).Order("id asc").Find(&channels).Error; err != nil {
		return nil
	}
	names := make([]string, 0, len(channels))
	for _, c := range channels {
		names = append(names, c.Name)
	}
	return names
}
