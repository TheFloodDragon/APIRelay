package controller

import (
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/relay/client"
	"github.com/TheFloodDragon/APIRelay/internal/relay/forwarder"
	providerrouter "github.com/TheFloodDragon/APIRelay/internal/relay/router"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/gin-gonic/gin"
)

// RelayController 是 /v1/* 入口的新转发控制器。
type RelayController struct {
	channelRepo    *repository.ChannelRepository
	httpClient     *client.HTTPClient
	logRepo        *repository.LogRepository
	modelRepo      *repository.ModelRepository
	providerRouter *providerrouter.ProviderRouter
	forwarder      *forwarder.Forwarder
	circuitBreaker *CircuitBreaker
}

func NewRelayController(
	channelRepo *repository.ChannelRepository,
	httpClient *client.HTTPClient,
	logRepo *repository.LogRepository,
	modelRepo *repository.ModelRepository,
	providerRouter *providerrouter.ProviderRouter,
	forwarder *forwarder.Forwarder,
) *RelayController {
	return &RelayController{
		channelRepo:    channelRepo,
		httpClient:     httpClient,
		logRepo:        logRepo,
		modelRepo:      modelRepo,
		providerRouter: providerRouter,
		forwarder:      forwarder,
		circuitBreaker: NewCircuitBreaker(),
	}
}

// GetModels 获取 OpenAI 兼容的可用模型列表（仅返回启用的模型，使用显示名称去重）。
func (rc *RelayController) GetModels(c *gin.Context) {
	models, err := rc.publicModels()
	if err != nil {
		writeRelayError(c, http.StatusInternalServerError, "获取模型列表失败", "internal_error", err.Error())
		return
	}

	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		data = append(data, openAIModelObject(m))
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// GetModel 获取单个 OpenAI 兼容模型元数据。
func (rc *RelayController) GetModel(c *gin.Context) {
	modelID := normalizePublicModelID(c.Param("model"))
	modelRecord, ok, err := rc.findPublicModel(modelID)
	if err != nil {
		writeRelayError(c, http.StatusInternalServerError, "获取模型失败", "internal_error", err.Error())
		return
	}
	if !ok {
		writeRelayError(c, http.StatusNotFound, "模型不存在", "invalid_request_error", "")
		return
	}

	c.JSON(http.StatusOK, openAIModelObject(modelRecord))
}

// GetGeminiModels 获取 Gemini 兼容的可用模型列表。
func (rc *RelayController) GetGeminiModels(c *gin.Context) {
	models, err := rc.publicModels()
	if err != nil {
		writeRelayError(c, http.StatusInternalServerError, "获取模型列表失败", "internal_error", err.Error())
		return
	}

	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		data = append(data, geminiModelObject(m))
	}

	c.JSON(http.StatusOK, gin.H{"models": data})
}

// GetGeminiModel 获取单个 Gemini 兼容模型元数据。
func (rc *RelayController) GetGeminiModel(c *gin.Context) {
	rc.writeGeminiModel(c, c.Param("modelPath"))
}

func (rc *RelayController) writeGeminiModel(c *gin.Context, modelID string) {
	modelID = normalizePublicModelID(modelID)
	modelRecord, ok, err := rc.findPublicModel(modelID)
	if err != nil {
		writeGeminiError(c, http.StatusInternalServerError, "获取模型失败", "INTERNAL")
		return
	}
	if !ok {
		writeGeminiError(c, http.StatusNotFound, "模型不存在", "NOT_FOUND")
		return
	}

	c.JSON(http.StatusOK, geminiModelObject(modelRecord))
}

type publicModel struct {
	ID      string
	Created int64
}

func (rc *RelayController) publicModels() ([]publicModel, error) {
	models, err := rc.modelRepo.GetEnabled()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	data := make([]publicModel, 0, len(models))
	for _, m := range models {
		modelID := publicModelID(m.Name, m.DisplayName)
		if modelID == "" || seen[modelID] {
			continue
		}
		seen[modelID] = true
		data = append(data, publicModel{ID: modelID, Created: m.CreatedAt.Unix()})
	}
	return data, nil
}

func (rc *RelayController) findPublicModel(modelID string) (publicModel, bool, error) {
	models, err := rc.publicModels()
	if err != nil {
		return publicModel{}, false, err
	}
	for _, m := range models {
		if m.ID == modelID {
			return m, true, nil
		}
	}
	return publicModel{}, false, nil
}

func publicModelID(name, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return name
}

func normalizePublicModelID(modelID string) string {
	modelID = strings.TrimSpace(strings.TrimPrefix(modelID, "/"))
	modelID = strings.TrimPrefix(modelID, "models/")
	return modelID
}

func openAIModelObject(model publicModel) gin.H {
	return gin.H{
		"id":       model.ID,
		"object":   "model",
		"created":  model.Created,
		"owned_by": "apirelay",
	}
}

func geminiModelObject(model publicModel) gin.H {
	return gin.H{
		"name":                       "models/" + normalizePublicModelID(model.ID),
		"version":                    "",
		"displayName":                model.ID,
		"description":                "Routed by APIRelay",
		"inputTokenLimit":            0,
		"outputTokenLimit":           0,
		"supportedGenerationMethods": []string{"generateContent", "streamGenerateContent"},
	}
}
