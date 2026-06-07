package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/yourusername/apirelay/internal/model"
	"github.com/yourusername/apirelay/internal/repository"
	"github.com/yourusername/apirelay/internal/scheduler"
)

type RelayHandler struct {
	scheduler *scheduler.Scheduler
	logRepo   *repository.LogRepository
	modelRepo *repository.ModelRepository
}

func NewRelayHandler(scheduler *scheduler.Scheduler, logRepo *repository.LogRepository, modelRepo *repository.ModelRepository) *RelayHandler {
	return &RelayHandler{
		scheduler: scheduler,
		logRepo:   logRepo,
		modelRepo: modelRepo,
	}
}

// GetModels 获取可用模型列表
func (h *RelayHandler) GetModels(c *gin.Context) {
	models, err := h.modelRepo.GetEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "获取模型列表失败",
				"type":    "internal_error",
			},
		})
		return
	}

	// 转换为 OpenAI 格式
	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		data = append(data, gin.H{
			"id":      m.Name,
			"object":  "model",
			"created": m.CreatedAt.Unix(),
			"owned_by": "apirelay",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// ChatCompletions 聊天补全接口
func (h *RelayHandler) ChatCompletions(c *gin.Context) {
	startTime := time.Now()

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "读取请求失败",
				"type":    "invalid_request_error",
			},
		})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// 解析请求获取模型名
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "请求格式错误",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "缺少 model 参数",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 选择渠道
	channels, err := h.scheduler.GetAllChannelsForModel(req.Model)
	if err != nil || len(channels) == 0 {
		h.logRequest(nil, req.Model, c.Request.Method, c.Request.URL.Path, 404, 0, time.Since(startTime), "没有可用的渠道", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "没有找到支持该模型的渠道",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 尝试多个渠道
	var lastErr error
	for _, channel := range channels {
		statusCode, respBody, err := h.forwardRequest(&channel, c.Request.Method, "/chat/completions", body, c.Request.Header)
		latency := time.Since(startTime)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			// 成功
			h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), "", c.ClientIP())
			
			// 返回响应
			c.Data(statusCode, "application/json", respBody)
			return
		}

		lastErr = err
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = string(respBody)
		}
		h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), errMsg, c.ClientIP())
	}

	// 所有渠道都失败
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": "所有渠道请求失败",
			"type":    "api_error",
			"details": failureDetails(lastErr),
		},
	})
}

// Completions 文本补全接口
func (h *RelayHandler) Completions(c *gin.Context) {
	startTime := time.Now()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "读取请求失败",
				"type":    "invalid_request_error",
			},
		})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "请求格式错误",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	channels, err := h.scheduler.GetAllChannelsForModel(req.Model)
	if err != nil || len(channels) == 0 {
		h.logRequest(nil, req.Model, c.Request.Method, c.Request.URL.Path, 404, 0, time.Since(startTime), "没有可用的渠道", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "没有找到支持该模型的渠道",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	var lastErr error
	for _, channel := range channels {
		statusCode, respBody, err := h.forwardRequest(&channel, c.Request.Method, "/completions", body, c.Request.Header)
		latency := time.Since(startTime)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), "", c.ClientIP())
			c.Data(statusCode, "application/json", respBody)
			return
		}

		lastErr = err
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), errMsg, c.ClientIP())
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": "所有渠道请求失败",
			"type":    "api_error",
			"details": failureDetails(lastErr),
		},
	})
}

// Embeddings 嵌入接口
func (h *RelayHandler) Embeddings(c *gin.Context) {
	startTime := time.Now()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "读取请求失败",
				"type":    "invalid_request_error",
			},
		})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "请求格式错误",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	channels, err := h.scheduler.GetAllChannelsForModel(req.Model)
	if err != nil || len(channels) == 0 {
		h.logRequest(nil, req.Model, c.Request.Method, c.Request.URL.Path, 404, 0, time.Since(startTime), "没有可用的渠道", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "没有找到支持该模型的渠道",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	var lastErr error
	for _, channel := range channels {
		statusCode, respBody, err := h.forwardRequest(&channel, c.Request.Method, "/embeddings", body, c.Request.Header)
		latency := time.Since(startTime)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), "", c.ClientIP())
			c.Data(statusCode, "application/json", respBody)
			return
		}

		lastErr = err
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), time.Since(startTime), errMsg, c.ClientIP())
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": "所有渠道请求失败",
			"type":    "api_error",
			"details": failureDetails(lastErr),
		},
	})
}

// forwardRequest 转发请求到目标渠道
func (h *RelayHandler) forwardRequest(channel *model.Channel, method, path string, body []byte, headers http.Header) (int, []byte, error) {
	client := resty.New()
	client.SetTimeout(time.Duration(channel.Timeout) * time.Millisecond)

	baseURL := channel.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req := client.R().
		SetHeader("Authorization", "Bearer "+channel.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body)

	resp, err := req.Execute(method, baseURL+path)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode(), resp.Body(), nil
}

// logRequest 记录请求日志
func (h *RelayHandler) logRequest(channelID *uint, model, method, path string, statusCode, latency int, duration time.Duration, errMsg, ip string) {
	log := &model.RequestLog{
		ChannelID:  channelID,
		Model:      model,
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Latency:    int(duration.Milliseconds()),
		Error:      errMsg,
		IP:         ip,
	}
	_ = h.logRepo.Create(log)
}

func failureDetails(err error) string {
	if err == nil {
		return "上游渠道返回非成功状态码"
	}
	return err.Error()
}
