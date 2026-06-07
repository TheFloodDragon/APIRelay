package router

import (
	"fmt"
	"sync"

	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

// ModelRouter 模型路由器 - 实现本地路由和模型映射功能
type ModelRouter struct {
	modelRepo *repository.ModelRepository
	mu        sync.RWMutex

	// 模型别名映射：alias -> realModel
	aliases map[string]string

	// 模型重定向映射：sourceModel -> targetModel
	redirects map[string]string

	// 模型组映射：groupName -> []realModels
	groups map[string][]string
}

// NewModelRouter 创建模型路由器
func NewModelRouter(modelRepo *repository.ModelRepository) *ModelRouter {
	router := &ModelRouter{
		modelRepo: modelRepo,
		aliases:   make(map[string]string),
		redirects: make(map[string]string),
		groups:    make(map[string][]string),
	}

	// 初始化时加载配置
	router.reload()

	return router
}

// ResolveModel 解析模型名称，应用别名、重定向和路由规则
func (r *ModelRouter) ResolveModel(requestedModel string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 检查别名
	if realModel, ok := r.aliases[requestedModel]; ok {
		requestedModel = realModel
	}

	// 2. 检查重定向
	if targetModel, ok := r.redirects[requestedModel]; ok {
		requestedModel = targetModel
	}

	// 3. 检查模型组
	if groupModels, ok := r.groups[requestedModel]; ok {
		if len(groupModels) == 0 {
			return nil, fmt.Errorf("模型组 %s 为空", requestedModel)
		}
		return groupModels, nil
	}

	// 4. 返回单一模型
	return []string{requestedModel}, nil
}

// SetAlias 设置模型别名
func (r *ModelRouter) SetAlias(alias, realModel string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if alias == "" || realModel == "" {
		return fmt.Errorf("别名和真实模型名不能为空")
	}

	r.aliases[alias] = realModel
	return r.saveToDatabase()
}

// SetRedirect 设置模型重定向
func (r *ModelRouter) SetRedirect(sourceModel, targetModel string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sourceModel == "" || targetModel == "" {
		return fmt.Errorf("源模型和目标模型名不能为空")
	}

	// 防止循环重定向
	if r.hasCircularRedirect(sourceModel, targetModel) {
		return fmt.Errorf("检测到循环重定向")
	}

	r.redirects[sourceModel] = targetModel
	return r.saveToDatabase()
}

// SetGroup 设置模型组
func (r *ModelRouter) SetGroup(groupName string, models []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if groupName == "" {
		return fmt.Errorf("模型组名不能为空")
	}

	if len(models) == 0 {
		return fmt.Errorf("模型组至少需要一个模型")
	}

	r.groups[groupName] = models
	return r.saveToDatabase()
}

// RemoveAlias 删除别名
func (r *ModelRouter) RemoveAlias(alias string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.aliases, alias)
	return r.saveToDatabase()
}

// RemoveRedirect 删除重定向
func (r *ModelRouter) RemoveRedirect(sourceModel string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.redirects, sourceModel)
	return r.saveToDatabase()
}

// RemoveGroup 删除模型组
func (r *ModelRouter) RemoveGroup(groupName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.groups, groupName)
	return r.saveToDatabase()
}

// GetAllAliases 获取所有别名
func (r *ModelRouter) GetAllAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.aliases {
		result[k] = v
	}
	return result
}

// GetAllRedirects 获取所有重定向
func (r *ModelRouter) GetAllRedirects() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.redirects {
		result[k] = v
	}
	return result
}

// GetAllGroups 获取所有模型组
func (r *ModelRouter) GetAllGroups() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)
	for k, v := range r.groups {
		result[k] = append([]string{}, v...)
	}
	return result
}

// Reload 重新加载路由配置
func (r *ModelRouter) Reload() error {
	return r.reload()
}

// reload 从数据库加载配置
func (r *ModelRouter) reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 从 system_config 表加载配置
	config, err := r.loadFromDatabase()
	if err != nil {
		return err
	}

	r.aliases = config.Aliases
	r.redirects = config.Redirects
	r.groups = config.Groups

	return nil
}

// hasCircularRedirect 检测循环重定向
func (r *ModelRouter) hasCircularRedirect(source, target string) bool {
	visited := make(map[string]bool)
	current := target

	for {
		if current == source {
			return true
		}

		if visited[current] {
			return false
		}

		visited[current] = true

		next, ok := r.redirects[current]
		if !ok {
			return false
		}

		current = next
	}
}

// RouteConfig 路由配置
type RouteConfig struct {
	Aliases   map[string]string   `json:"aliases"`
	Redirects map[string]string   `json:"redirects"`
	Groups    map[string][]string `json:"groups"`
}

// saveToDatabase 保存到数据库
func (r *ModelRouter) saveToDatabase() error {
	// 使用 SystemConfig 表存储路由配置
	config := RouteConfig{
		Aliases:   r.aliases,
		Redirects: r.redirects,
		Groups:    r.groups,
	}

	// 这里需要序列化并保存到 system_config 表
	// 实际实现需要调用 repository
	return nil
}

// loadFromDatabase 从数据库加载
func (r *ModelRouter) loadFromDatabase() (*RouteConfig, error) {
	// 从 system_config 表加载路由配置
	// 如果不存在则返回空配置
	return &RouteConfig{
		Aliases:   make(map[string]string),
		Redirects: make(map[string]string),
		Groups:    make(map[string][]string),
	}, nil
}
