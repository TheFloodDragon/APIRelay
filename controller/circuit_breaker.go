package controller

import (
	"net/http"
	"strconv"

	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/circuitbreaker"
	"github.com/gin-gonic/gin"
)

// GetChannelHealth 获取渠道健康状态
func GetChannelHealth(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel id"})
		return
	}

	health, err := model.GetChannelHealth(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, health)
}

// ResetChannelHealth 重置渠道熔断器
func ResetChannelHealth(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel id"})
		return
	}

	circuitbreaker.GetManager().ResetBreaker(id)
	c.JSON(http.StatusOK, gin.H{"message": "circuit breaker reset"})
}

// GetAllChannelHealthStats 获取所有渠道健康统计
func GetAllChannelHealthStats(c *gin.Context) {
	stats, err := model.GetAllChannelHealthStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetCircuitBreakerConfig 获取熔断器配置
func GetCircuitBreakerConfig(c *gin.Context) {
	// 从 settings 表读取
	cfg, err := model.GetSetting("circuit_breaker_config")
	if err != nil || cfg == "" {
		// 返回默认配置
		c.JSON(http.StatusOK, circuitbreaker.DefaultConfig())
		return
	}

	// 解析存储的 JSON 配置
	var cbCfg circuitbreaker.Config
	if err := model.UnmarshalSetting(cfg, &cbCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "parse config failed"})
		return
	}
	c.JSON(http.StatusOK, cbCfg)
}

// UpdateCircuitBreakerConfig 更新熔断器配置
func UpdateCircuitBreakerConfig(c *gin.Context) {
	var cfg circuitbreaker.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 存储到 settings 表
	if err := model.SaveSettingJSON("circuit_breaker_config", cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新运行时配置
	circuitbreaker.GetManager().UpdateConfig(cfg)

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}
