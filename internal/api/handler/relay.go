package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/adapter"
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/TheFloodDragon/APIRelay/internal/router"
	"github.com/TheFloodDragon/APIRelay/internal/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

type RelayHandler struct {
	scheduler   *scheduler.Scheduler
	logRepo     *repository.LogRepository
	modelRepo   *repository.ModelRepository
	modelRouter *router.ModelRouter
}

func NewRelayHandler(scheduler *scheduler.Scheduler, logRepo *repository.LogRepository, modelRepo *repository.ModelRepository, modelRouter *router.ModelRouter) *RelayHandler {
	return &RelayHandler{
		scheduler:   scheduler,
		logRepo:     logRepo,
		modelRepo:   modelRepo,
		modelRouter: modelRouter,
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
			"id":       m.Name,
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

// Responses OpenAI Responses API 兼容入口。
// 当前实现会将 Responses 请求转换为 Chat Completions 请求，并把响应再转换回 Responses 格式。
func (h *RelayHandler) Responses(c *gin.Context) {
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

	chatBody, modelName, stream, err := responsesRequestToChatCompletions(body)
	if err != nil {
		h.logRequest(nil, "", c.Request.Method, c.Request.URL.Path, 400, 0, time.Since(startTime), err.Error(), c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	resolvedModels, err := h.modelRouter.ResolveModel(modelName)
	if err != nil {
		h.logRequest(nil, modelName, c.Request.Method, c.Request.URL.Path, 400, 0, time.Since(startTime), err.Error(), c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	var allChannels []model.Channel
	for _, resolvedModel := range resolvedModels {
		channels, err := h.scheduler.GetAllChannelsForModel(resolvedModel)
		if err == nil && len(channels) > 0 {
			allChannels = append(allChannels, channels...)
		}
	}

	if len(allChannels) == 0 {
		h.logRequest(nil, modelName, c.Request.Method, c.Request.URL.Path, 404, 0, time.Since(startTime), "没有可用的渠道", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "没有找到支持该模型的渠道",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	if stream {
		var lastErr error
		var lastErrMsg string
		for _, channel := range allChannels {
			statusCode, errMsg, err := h.forwardResponsesStreamRequest(c, &channel, chatBody, modelName)
			latency := time.Since(startTime)

			if err == nil && statusCode >= 200 && statusCode < 300 {
				h.logRequest(&channel.ID, modelName, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, "", c.ClientIP())
				return
			}

			lastErr = err
			lastErrMsg = errMsg
			if lastErrMsg == "" {
				lastErrMsg = failureDetails(err)
			}
			h.logRequest(&channel.ID, modelName, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, lastErrMsg, c.ClientIP())
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "所有渠道请求失败",
				"type":    "api_error",
				"details": streamFailureDetails(lastErr, lastErrMsg),
			},
		})
		return
	}

	var lastErr error
	for _, channel := range allChannels {
		statusCode, respBody, err := h.forwardRequestWithAdapter(&channel, c.Request.Method, "/chat/completions", chatBody, c.Request.Header)
		latency := time.Since(startTime)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			responsesBody, convertErr := chatCompletionsResponseToResponses(respBody, modelName)
			if convertErr != nil {
				h.logRequest(&channel.ID, modelName, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, convertErr.Error(), c.ClientIP())
				c.JSON(http.StatusBadGateway, gin.H{
					"error": gin.H{
						"message": convertErr.Error(),
						"type":    "api_error",
					},
				})
				return
			}

			h.logRequest(&channel.ID, modelName, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, "", c.ClientIP())
			c.Data(statusCode, "application/json", responsesBody)
			return
		}

		lastErr = err
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			errMsg = string(respBody)
		}
		h.logRequest(&channel.ID, modelName, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, errMsg, c.ClientIP())
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": "所有渠道请求失败",
			"type":    "api_error",
			"details": failureDetails(lastErr),
		},
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

	// 解析请求获取模型名和流式参数
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
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

	// 应用模型路由（别名、重定向、模型组）
	resolvedModels, err := h.modelRouter.ResolveModel(req.Model)
	if err != nil {
		h.logRequest(nil, req.Model, c.Request.Method, c.Request.URL.Path, 400, 0, time.Since(startTime), err.Error(), c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 为所有解析后的模型选择渠道
	var allChannels []model.Channel
	for _, modelName := range resolvedModels {
		channels, err := h.scheduler.GetAllChannelsForModel(modelName)
		if err == nil && len(channels) > 0 {
			allChannels = append(allChannels, channels...)
		}
	}

	if len(allChannels) == 0 {
		h.logRequest(nil, req.Model, c.Request.Method, c.Request.URL.Path, 404, 0, time.Since(startTime), "没有可用的渠道", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "没有找到支持该模型的渠道",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 流式响应透传。注意：一旦开始向客户端写入 SSE，就不能再切换渠道重试。
	if req.Stream {
		var lastErr error
		var lastErrMsg string
		for _, channel := range allChannels {
			statusCode, errMsg, err := h.forwardStreamRequest(c, &channel, "/chat/completions", body)
			latency := time.Since(startTime)

			if err == nil && statusCode >= 200 && statusCode < 300 {
				h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, "", c.ClientIP())
				return
			}

			lastErr = err
			lastErrMsg = errMsg
			if lastErrMsg == "" {
				lastErrMsg = failureDetails(err)
			}
			h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, lastErrMsg, c.ClientIP())
		}

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "所有渠道请求失败",
				"type":    "api_error",
				"details": streamFailureDetails(lastErr, lastErrMsg),
			},
		})
		return
	}

	// 尝试多个渠道（使用协议适配器）
	var lastErr error
	for _, channel := range allChannels {
		statusCode, respBody, err := h.forwardRequestWithAdapter(&channel, c.Request.Method, "/chat/completions", body, c.Request.Header)
		latency := time.Since(startTime)

		if err == nil && statusCode >= 200 && statusCode < 300 {
			// 成功
			h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, "", c.ClientIP())

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
		h.logRequest(&channel.ID, req.Model, c.Request.Method, c.Request.URL.Path, statusCode, int(latency.Milliseconds()), latency, errMsg, c.ClientIP())
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

// forwardRequest 转发普通 JSON 请求到目标渠道
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

// forwardRequestWithAdapter 使用协议适配器转发请求
func (h *RelayHandler) forwardRequestWithAdapter(channel *model.Channel, method, path string, body []byte, headers http.Header) (int, []byte, error) {
	// 获取对应的协议适配器
	protocolAdapter := adapter.GetAdapter(channel.Type)

	// 如果需要协议转换
	if protocolAdapter.NeedsConversion() {
		// 解析原始请求
		var openaiReq interface{}
		if err := json.Unmarshal(body, &openaiReq); err != nil {
			return 0, nil, err
		}

		// 转换请求格式
		convertedReq, err := protocolAdapter.ConvertRequest(openaiReq)
		if err != nil {
			return 0, nil, err
		}

		// 重新序列化
		convertedBody, err := json.Marshal(convertedReq)
		if err != nil {
			return 0, nil, err
		}
		body = convertedBody
	}

	// 发送请求
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

	respBody := resp.Body()

	// 如果需要协议转换，转换响应格式
	if protocolAdapter.NeedsConversion() && resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		convertedResp, err := protocolAdapter.ConvertResponse(bytes.NewReader(respBody))
		if err != nil {
			return resp.StatusCode(), respBody, err
		}
		respBody = convertedResp
	}

	return resp.StatusCode(), respBody, nil
}

// forwardStreamRequest 转发并透传 SSE 流式请求到目标渠道（带协议适配）
func (h *RelayHandler) forwardStreamRequest(c *gin.Context, channel *model.Channel, path string, body []byte) (int, string, error) {
	// 获取协议适配器
	protocolAdapter := adapter.GetAdapter(channel.Type)

	// 如果需要协议转换，转换请求
	if protocolAdapter.NeedsConversion() {
		var openaiReq interface{}
		if err := json.Unmarshal(body, &openaiReq); err != nil {
			return 0, "", err
		}

		convertedReq, err := protocolAdapter.ConvertRequest(openaiReq)
		if err != nil {
			return 0, "", err
		}

		convertedBody, err := json.Marshal(convertedReq)
		if err != nil {
			return 0, "", err
		}
		body = convertedBody
	}

	baseURL := channel.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Duration(channel.Timeout) * time.Millisecond
	client := &http.Client{Transport: transport}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.APIKey)
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(upstreamReq)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(errorBody), nil
	}

	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(resp.StatusCode)

	buffer := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]

			// 如果需要协议转换，转换流式块
			if protocolAdapter.NeedsConversion() {
				convertedChunk, err := protocolAdapter.ConvertStreamChunk(chunk)
				if err == nil {
					chunk = convertedChunk
				}
			}

			if _, writeErr := writer.Write(chunk); writeErr != nil {
				// 响应头和部分数据已经发送，不能再切换渠道或改写错误响应。
				return resp.StatusCode, "", nil
			}
			writer.Flush()
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			// 同上：流已开始，直接结束当前响应。
			return resp.StatusCode, "", nil
		}
	}

	return resp.StatusCode, "", nil
}

// forwardResponsesStreamRequest 将上游 Chat Completions SSE 转换为 Responses API SSE。
func (h *RelayHandler) forwardResponsesStreamRequest(c *gin.Context, channel *model.Channel, body []byte, modelName string) (int, string, error) {
	protocolAdapter := adapter.GetAdapter(channel.Type)
	if protocolAdapter.NeedsConversion() {
		var openaiReq interface{}
		if err := json.Unmarshal(body, &openaiReq); err != nil {
			return 0, "", err
		}

		convertedReq, err := protocolAdapter.ConvertRequest(openaiReq)
		if err != nil {
			return 0, "", err
		}

		convertedBody, err := json.Marshal(convertedReq)
		if err != nil {
			return 0, "", err
		}
		body = convertedBody
	}

	baseURL := channel.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = time.Duration(channel.Timeout) * time.Millisecond
	client := &http.Client{Transport: transport}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.APIKey)
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(upstreamReq)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(errorBody), nil
	}

	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(resp.StatusCode)

	responseID := "resp_" + time.Now().Format("20060102150405.000000000")
	messageID := "msg_" + time.Now().Format("20060102150405.000000000")
	emitter := newResponsesStreamEmitter(writer, responseID, messageID, modelName)
	if err := emitter.start(); err != nil {
		return resp.StatusCode, "", nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		chunkBytes := []byte("data: " + data + "\n\n")
		if protocolAdapter.NeedsConversion() {
			convertedChunk, err := protocolAdapter.ConvertStreamChunk(chunkBytes)
			if err == nil {
				chunkBytes = convertedChunk
			}
		}

		for _, content := range extractChatStreamContent(chunkBytes) {
			if content == "" {
				continue
			}
			if err := emitter.delta(content); err != nil {
				return resp.StatusCode, "", nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		_ = emitter.complete()
		return resp.StatusCode, "", nil
	}

	if err := emitter.complete(); err != nil {
		return resp.StatusCode, "", nil
	}

	return resp.StatusCode, "", nil
}

func responsesRequestToChatCompletions(body []byte) ([]byte, string, bool, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", false, err
	}

	modelName, _ := raw["model"].(string)
	if modelName == "" {
		return nil, "", false, errMissingModel()
	}

	messages := make([]map[string]interface{}, 0)
	if instructions, _ := raw["instructions"].(string); instructions != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": instructions})
	}

	messages = append(messages, responsesInputToMessages(raw["input"])...)
	if len(messages) == 0 {
		return nil, "", false, errMissingInput()
	}

	stream, _ := raw["stream"].(bool)
	chatReq := map[string]interface{}{
		"model":    modelName,
		"messages": messages,
		"stream":   stream,
	}

	copyIfPresent(chatReq, raw, "temperature", "temperature")
	copyIfPresent(chatReq, raw, "top_p", "top_p")
	copyIfPresent(chatReq, raw, "max_output_tokens", "max_tokens")
	copyIfPresent(chatReq, raw, "max_tokens", "max_tokens")
	copyIfPresent(chatReq, raw, "tools", "tools")

	chatBody, err := json.Marshal(chatReq)
	return chatBody, modelName, stream, err
}

func responsesInputToMessages(input interface{}) []map[string]interface{} {
	switch value := input.(type) {
	case string:
		if value == "" {
			return nil
		}
		return []map[string]interface{}{{"role": "user", "content": value}}
	case []interface{}:
		messages := make([]map[string]interface{}, 0, len(value))
		for _, item := range value {
			message, ok := responseInputItemToMessage(item)
			if ok {
				messages = append(messages, message)
			}
		}
		return messages
	default:
		return nil
	}
}

func responseInputItemToMessage(item interface{}) (map[string]interface{}, bool) {
	inputItem, ok := item.(map[string]interface{})
	if !ok {
		return nil, false
	}

	role, _ := inputItem["role"].(string)
	if role == "" {
		role = "user"
	}

	content := responseContentToText(inputItem["content"])
	if content == "" {
		content, _ = inputItem["text"].(string)
	}
	if content == "" {
		return nil, false
	}

	if role == "developer" {
		role = "system"
	}
	if role == "model" {
		role = "assistant"
	}

	return map[string]interface{}{"role": role, "content": content}, true
}

func responseContentToText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			contentPart, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, _ := contentPart["text"].(string); text != "" {
				parts = append(parts, text)
				continue
			}
			if text, _ := contentPart["input_text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func chatCompletionsResponseToResponses(respBody []byte, requestedModel string) ([]byte, error) {
	var chatResp map[string]interface{}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, err
	}

	outputText := ""
	if choices, ok := chatResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				outputText, _ = message["content"].(string)
			}
		}
	}

	responseID, _ := chatResp["id"].(string)
	if responseID == "" {
		responseID = "resp_" + time.Now().Format("20060102150405.000000000")
	}
	modelName, _ := chatResp["model"].(string)
	if modelName == "" {
		modelName = requestedModel
	}

	response := baseResponsesObject(responseID, "msg_"+responseID, modelName, "completed", outputText)
	if usage, ok := chatResp["usage"].(map[string]interface{}); ok {
		response["usage"] = map[string]interface{}{
			"input_tokens":  usage["prompt_tokens"],
			"output_tokens": usage["completion_tokens"],
			"total_tokens":  usage["total_tokens"],
		}
	}

	return json.Marshal(response)
}

func extractChatStreamContent(chunk []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(chunk))
	contents := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chatChunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chatChunk); err != nil {
			continue
		}
		choices, ok := chatChunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}
		content, _ := delta["content"].(string)
		contents = append(contents, content)
	}
	return contents
}

type responsesStreamEmitter struct {
	writer        gin.ResponseWriter
	responseID    string
	messageID     string
	modelName     string
	sequence      int
	collectedText strings.Builder
}

func newResponsesStreamEmitter(writer gin.ResponseWriter, responseID, messageID, modelName string) *responsesStreamEmitter {
	return &responsesStreamEmitter{
		writer:     writer,
		responseID: responseID,
		messageID:  messageID,
		modelName:  modelName,
	}
}

func (e *responsesStreamEmitter) start() error {
	if err := e.write("response.created", map[string]interface{}{
		"type":     "response.created",
		"response": baseResponsesObject(e.responseID, e.messageID, e.modelName, "in_progress", ""),
	}); err != nil {
		return err
	}
	if err := e.write("response.output_item.added", map[string]interface{}{
		"type":         "response.output_item.added",
		"output_index": 0,
		"item": map[string]interface{}{
			"id":      e.messageID,
			"type":    "message",
			"status":  "in_progress",
			"role":    "assistant",
			"content": []interface{}{},
		},
	}); err != nil {
		return err
	}
	return e.write("response.content_part.added", map[string]interface{}{
		"type":          "response.content_part.added",
		"item_id":       e.messageID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]interface{}{
			"type":        "output_text",
			"text":        "",
			"annotations": []interface{}{},
		},
	})
}

func (e *responsesStreamEmitter) delta(content string) error {
	e.sequence++
	e.collectedText.WriteString(content)
	return e.write("response.output_text.delta", map[string]interface{}{
		"type":            "response.output_text.delta",
		"item_id":         e.messageID,
		"output_index":    0,
		"content_index":   0,
		"delta":           content,
		"sequence_number": e.sequence,
	})
}

func (e *responsesStreamEmitter) complete() error {
	outputText := e.collectedText.String()
	if err := e.write("response.output_text.done", map[string]interface{}{
		"type":          "response.output_text.done",
		"item_id":       e.messageID,
		"output_index":  0,
		"content_index": 0,
		"text":          outputText,
	}); err != nil {
		return err
	}
	if err := e.write("response.content_part.done", map[string]interface{}{
		"type":          "response.content_part.done",
		"item_id":       e.messageID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]interface{}{
			"type":        "output_text",
			"text":        outputText,
			"annotations": []interface{}{},
		},
	}); err != nil {
		return err
	}
	if err := e.write("response.output_item.done", map[string]interface{}{
		"type":         "response.output_item.done",
		"output_index": 0,
		"item": map[string]interface{}{
			"id":     e.messageID,
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []interface{}{
				map[string]interface{}{
					"type":        "output_text",
					"text":        outputText,
					"annotations": []interface{}{},
				},
			},
		},
	}); err != nil {
		return err
	}
	return e.write("response.completed", map[string]interface{}{
		"type":     "response.completed",
		"response": baseResponsesObject(e.responseID, e.messageID, e.modelName, "completed", outputText),
	})
}

func (e *responsesStreamEmitter) write(eventName string, payload map[string]interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := e.writer.Write([]byte("event: " + eventName + "\n")); err != nil {
		return err
	}
	if _, err := e.writer.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := e.writer.Write(payloadBytes); err != nil {
		return err
	}
	if _, err := e.writer.Write([]byte("\n\n")); err != nil {
		return err
	}
	e.writer.Flush()
	return nil
}

func baseResponsesObject(responseID, messageID, modelName, status, outputText string) map[string]interface{} {
	return map[string]interface{}{
		"id":         responseID,
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     status,
		"model":      modelName,
		"output": []interface{}{
			map[string]interface{}{
				"id":     messageID,
				"type":   "message",
				"status": status,
				"role":   "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "output_text",
						"text":        outputText,
						"annotations": []interface{}{},
					},
				},
			},
		},
		"output_text": outputText,
	}
}

func copyIfPresent(dst map[string]interface{}, src map[string]interface{}, srcKey, dstKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}

func errMissingModel() error {
	return &relayError{message: "缺少 model 参数"}
}

func errMissingInput() error {
	return &relayError{message: "缺少 input 参数"}
}

type relayError struct {
	message string
}

func (e *relayError) Error() string {
	return e.message
}

// logRequest 记录请求日志
func (h *RelayHandler) logRequest(channelID *uint, modelName, method, path string, statusCode, latency int, duration time.Duration, errMsg, ip string) {
	latencyMS := int(duration.Milliseconds())
	requestLog := &model.RequestLog{
		ChannelID:  channelID,
		Model:      modelName,
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Latency:    latencyMS,
		Error:      errMsg,
		IP:         ip,
	}

	logErr := h.logRepo.Create(requestLog)
	if logErr != nil {
		log.Printf("[MODEL] model=%s channel_id=%v method=%s path=%s status=%d latency=%dms ip=%s error=%q log_error=%q",
			modelName,
			logChannelID(channelID),
			method,
			path,
			statusCode,
			latencyMS,
			ip,
			errMsg,
			logErr.Error(),
		)
		return
	}

	log.Printf("[MODEL] model=%s channel_id=%v method=%s path=%s status=%d latency=%dms ip=%s error=%q",
		modelName,
		logChannelID(channelID),
		method,
		path,
		statusCode,
		latencyMS,
		ip,
		errMsg,
	)
}

func logChannelID(channelID *uint) interface{} {
	if channelID == nil {
		return "-"
	}
	return *channelID
}

func failureDetails(err error) string {
	if err == nil {
		return "上游渠道返回非成功状态码"
	}
	return err.Error()
}

func streamFailureDetails(err error, errMsg string) string {
	if errMsg != "" {
		return errMsg
	}
	return failureDetails(err)
}
