package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/gin-gonic/gin"
)

type KeyHandler struct {
	keyRepo *repository.APIKeyRepository
}

func NewKeyHandler(keyRepo *repository.APIKeyRepository) *KeyHandler {
	return &KeyHandler{keyRepo: keyRepo}
}

// GetKeys 获取密钥列表
func (h *KeyHandler) GetKeys(c *gin.Context) {
	keys, err := h.keyRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取密钥列表失败: " + err.Error()})
		return
	}

	// 隐藏完整密钥
	for i := range keys {
		keys[i].Key = maskKey(keys[i].Key)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
	})
}

// CreateKey 创建密钥
func (h *KeyHandler) CreateKey(c *gin.Context) {
	var req struct {
		Name          string               `json:"name"`
		AllowedModels model.JSONStringList `json:"allowed_models"`
		IPWhitelist   model.JSONStringList `json:"ip_whitelist"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	key := &model.APIKey{
		Key:           generateAPIKey(),
		Name:          req.Name,
		Enabled:       true,
		AllowedModels: req.AllowedModels,
		IPWhitelist:   req.IPWhitelist,
	}

	if err := h.keyRepo.Create(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建密钥失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    key,
	})
}

// DeleteKey 删除密钥
func (h *KeyHandler) DeleteKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的密钥ID"})
		return
	}

	if err := h.keyRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除密钥失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

func generateAPIKey() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return "sk-ar-" + hex.EncodeToString(bytes)
}

func maskKey(key string) string {
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}
	return key[:8] + strings.Repeat("*", len(key)-12) + key[len(key)-4:]
}
