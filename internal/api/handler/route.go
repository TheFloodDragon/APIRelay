package handler

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/router"
	"github.com/gin-gonic/gin"
)

type RouteHandler struct {
	modelRouter *router.ModelRouter
}

func NewRouteHandler(modelRouter *router.ModelRouter) *RouteHandler {
	return &RouteHandler{
		modelRouter: modelRouter,
	}
}

// GetAllRoutes 获取所有路由配置
func (h *RouteHandler) GetAllRoutes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"aliases":   h.modelRouter.GetAllAliases(),
		"redirects": h.modelRouter.GetAllRedirects(),
		"groups":    h.modelRouter.GetAllGroups(),
	})
}

// SetAlias 设置模型别名
func (h *RouteHandler) SetAlias(c *gin.Context) {
	var req struct {
		Alias     string `json:"alias" binding:"required"`
		RealModel string `json:"real_model" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.modelRouter.SetAlias(req.Alias, req.RealModel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "别名设置成功"})
}

// DeleteAlias 删除模型别名
func (h *RouteHandler) DeleteAlias(c *gin.Context) {
	alias := c.Param("alias")

	if err := h.modelRouter.RemoveAlias(alias); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "别名删除成功"})
}

// SetRedirect 设置模型重定向
func (h *RouteHandler) SetRedirect(c *gin.Context) {
	var req struct {
		SourceModel string `json:"source_model" binding:"required"`
		TargetModel string `json:"target_model" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.modelRouter.SetRedirect(req.SourceModel, req.TargetModel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重定向设置成功"})
}

// DeleteRedirect 删除模型重定向
func (h *RouteHandler) DeleteRedirect(c *gin.Context) {
	sourceModel := c.Param("source")

	if err := h.modelRouter.RemoveRedirect(sourceModel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重定向删除成功"})
}

// SetGroup 设置模型组
func (h *RouteHandler) SetGroup(c *gin.Context) {
	var req struct {
		GroupName string   `json:"group_name" binding:"required"`
		Models    []string `json:"models" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.modelRouter.SetGroup(req.GroupName, req.Models); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模型组设置成功"})
}

// DeleteGroup 删除模型组
func (h *RouteHandler) DeleteGroup(c *gin.Context) {
	groupName := c.Param("group")

	if err := h.modelRouter.RemoveGroup(groupName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "模型组删除成功"})
}

// ReloadRoutes 重新加载路由配置
func (h *RouteHandler) ReloadRoutes(c *gin.Context) {
	if err := h.modelRouter.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "路由配置重载成功"})
}
