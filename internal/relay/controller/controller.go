package controller

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/TheFloodDragon/APIRelay/internal/relay/client"
	"github.com/TheFloodDragon/APIRelay/internal/router"
	"github.com/TheFloodDragon/APIRelay/internal/scheduler"
	"github.com/gin-gonic/gin"
)

// RelayController 是 /v1/* 入口的新转发控制器。
type RelayController struct {
	scheduler   *scheduler.Scheduler
	modelRouter *router.ModelRouter
	httpClient  *client.HTTPClient
	logRepo     *repository.LogRepository
	modelRepo   *repository.ModelRepository
}

func NewRelayController(
	scheduler *scheduler.Scheduler,
	modelRouter *router.ModelRouter,
	httpClient *client.HTTPClient,
	logRepo *repository.LogRepository,
	modelRepo *repository.ModelRepository,
) *RelayController {
	return &RelayController{
		scheduler:   scheduler,
		modelRouter: modelRouter,
		httpClient:  httpClient,
		logRepo:     logRepo,
		modelRepo:   modelRepo,
	}
}

// GetModels 获取 OpenAI 兼容的可用模型列表（仅返回启用的模型，使用显示名称去重）
func (rc *RelayController) GetModels(c *gin.Context) {
	models, err := rc.modelRepo.GetEnabled()
	if err != nil {
		writeRelayError(c, http.StatusInternalServerError, "获取模型列表失败", "internal_error", err.Error())
		return
	}

	// 使用显示名称去重
	seen := make(map[string]bool)
	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		displayName := m.DisplayName
		if displayName == "" {
			displayName = m.Name // 回退到上游模型名
		}
		if seen[displayName] {
			continue
		}
		seen[displayName] = true

		data = append(data, gin.H{
			"id":       displayName,
			"object":   "model",
			"created":  m.CreatedAt.Unix(),
			"owned_by": "apirelay",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}
