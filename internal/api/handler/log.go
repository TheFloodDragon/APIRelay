package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/apirelay/internal/repository"
)

type LogHandler struct {
	logRepo *repository.LogRepository
}

func NewLogHandler(logRepo *repository.LogRepository) *LogHandler {
	return &LogHandler{logRepo: logRepo}
}

// GetLogs 获取请求日志
func (h *LogHandler) GetLogs(c *gin.Context) {
	limit := parseIntQuery(c, "limit", 50)
	offset := parseIntQuery(c, "offset", 0)

	logs, err := h.logRepo.GetAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志失败: " + err.Error()})
		return
	}

	count, _ := h.logRepo.Count()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   count,
	})
}

func parseIntQuery(c *gin.Context, name string, defaultValue int) int {
	value := c.Query(name)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
