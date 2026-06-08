package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	relayadaptor "github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"gorm.io/gorm"
)

const modelTestConfigKey = "model_test_config"

// ModelTestConfig 是管理端真实模型短请求测试的全局默认配置。
type ModelTestConfig struct {
	TimeoutSecs         int               `json:"timeout_secs"`
	MaxRetries          int               `json:"max_retries"`
	DegradedThresholdMS int               `json:"degraded_threshold_ms"`
	TestPrompt          string            `json:"test_prompt"`
	MaxTokens           int               `json:"max_tokens"`
	Stream              bool              `json:"stream"`
	DefaultModels       map[string]string `json:"default_models"`
}

// ModelTestResult 是单次渠道模型测试结果。
type ModelTestResult struct {
	Success        bool      `json:"success"`
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	ResponseTimeMS int       `json:"response_time_ms"`
	TTFBMS         *int      `json:"ttfb_ms"`
	HTTPStatus     *int      `json:"http_status"`
	ModelUsed      string    `json:"model_used"`
	TestedAt       time.Time `json:"tested_at"`
	RetryCount     int       `json:"retry_count"`
	ErrorCategory  string    `json:"error_category"`
}

type ModelTestService struct {
	channelRepo      *repository.ChannelRepository
	configRepo       *repository.SystemConfigRepository
	modelTestLogRepo *repository.ModelTestLogRepository
}

type resolvedModelTestConfig struct {
	ModelTestConfig
	TestModel string
}

type upstreamTestResponse struct {
	StatusCode int
	Body       []byte
	LatencyMS  int
}

func NewModelTestService(
	channelRepo *repository.ChannelRepository,
	configRepo *repository.SystemConfigRepository,
	modelTestLogRepo *repository.ModelTestLogRepository,
) *ModelTestService {
	return &ModelTestService{
		channelRepo:      channelRepo,
		configRepo:       configRepo,
		modelTestLogRepo: modelTestLogRepo,
	}
}

func DefaultModelTestConfig() ModelTestConfig {
	return ModelTestConfig{
		TimeoutSecs:         45,
		MaxRetries:          2,
		DegradedThresholdMS: 6000,
		TestPrompt:          "Who are you?",
		MaxTokens:           20,
		Stream:              false,
		DefaultModels: map[string]string{
			"openai":            "gpt-4o-mini",
			"openai_compatible": "gpt-4o-mini",
			"newapi":            "gpt-4o-mini",
			"oneapi":            "gpt-4o-mini",
			"deepseek":          "deepseek-chat",
			"openrouter":        "openai/gpt-4o-mini",
			"custom":            "gpt-4o-mini",
			"codex":             "gpt-4o-mini",
			"anthropic":         "claude-3-5-haiku-latest",
			"claude":            "claude-3-5-haiku-latest",
			"gemini":            "gemini-1.5-flash",
			"google":            "gemini-1.5-flash",
		},
	}
}

func (s *ModelTestService) GetConfig() (ModelTestConfig, error) {
	cfg := DefaultModelTestConfig()
	if s.configRepo == nil {
		return cfg, nil
	}

	value, err := s.configRepo.Get(modelTestConfigKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cfg, nil
		}
		return cfg, err
	}
	if strings.TrimSpace(value) == "" {
		return cfg, nil
	}

	var saved ModelTestConfig
	if err := json.Unmarshal([]byte(value), &saved); err != nil {
		return cfg, fmt.Errorf("解析模型测试配置失败: %w", err)
	}
	return normalizeModelTestConfig(saved), nil
}

func (s *ModelTestService) SaveConfig(cfg ModelTestConfig) (ModelTestConfig, error) {
	cfg = normalizeModelTestConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	if s.configRepo == nil {
		return cfg, nil
	}
	return cfg, s.configRepo.Set(modelTestConfigKey, string(data))
}

func (s *ModelTestService) TestChannel(channelID uint) (*ModelTestResult, error) {
	channel, err := s.channelRepo.GetByID(channelID)
	if err != nil {
		return nil, err
	}

	globalCfg, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	cfg := s.resolveConfig(channel, globalCfg)
	result := s.runModelTest(channel, cfg)

	if s.modelTestLogRepo != nil {
		if err := s.modelTestLogRepo.Create(modelTestLogFromResult(channel, result)); err != nil {
			return result, err
		}
	}
	if err := s.channelRepo.UpdateHealthCheck(channel.ID, result.Status, result.TestedAt); err != nil {
		return result, err
	}
	return result, nil
}

func (s *ModelTestService) GetLogs(channelID uint, limit int) ([]model.ModelTestLog, error) {
	if s.modelTestLogRepo == nil {
		return []model.ModelTestLog{}, nil
	}
	return s.modelTestLogRepo.GetByChannel(channelID, limit)
}

func (s *ModelTestService) resolveConfig(channel *model.Channel, global ModelTestConfig) resolvedModelTestConfig {
	global = normalizeModelTestConfig(global)
	cfg := resolvedModelTestConfig{ModelTestConfig: global}
	cfg.TestModel = resolveTestModel(channel, global)

	if channel == nil || channel.Config == nil {
		return cfg
	}
	raw, ok := channel.Config["test_config"]
	if !ok || raw == nil {
		return cfg
	}
	testConfig, ok := raw.(map[string]interface{})
	if !ok {
		return cfg
	}
	if enabled, ok := testConfig["enabled"].(bool); ok && !enabled {
		return cfg
	}
	if modelName := stringFromMap(testConfig, "test_model"); modelName != "" {
		cfg.TestModel = modelName
	}
	if timeout := intFromMap(testConfig, "timeout_secs"); timeout > 0 {
		cfg.TimeoutSecs = timeout
	}
	if prompt := stringFromMap(testConfig, "test_prompt"); prompt != "" {
		cfg.TestPrompt = prompt
	}
	if threshold := intFromMap(testConfig, "degraded_threshold_ms"); threshold > 0 {
		cfg.DegradedThresholdMS = threshold
	}
	if _, exists := testConfig["max_retries"]; exists {
		if maxRetries := intFromMap(testConfig, "max_retries"); maxRetries >= 0 {
			cfg.MaxRetries = maxRetries
		}
	}
	if maxTokens := intFromMap(testConfig, "max_tokens"); maxTokens > 0 {
		cfg.MaxTokens = maxTokens
	}
	if stream, ok := testConfig["stream"].(bool); ok {
		cfg.Stream = stream
	}
	return cfg
}

func (s *ModelTestService) runModelTest(channel *model.Channel, cfg resolvedModelTestConfig) *ModelTestResult {
	testedAt := time.Now()
	if strings.TrimSpace(cfg.TestModel) == "" {
		return &ModelTestResult{
			Success:       false,
			Status:        "unhealthy",
			Message:       "未配置测试模型，且渠道模型列表为空",
			ModelUsed:     "",
			TestedAt:      testedAt,
			ErrorCategory: "model_not_found",
		}
	}

	var lastResult *ModelTestResult
	attempts := cfg.MaxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := s.sendTestRequest(channel, cfg)
		if err != nil {
			lastResult = resultFromError(err, cfg.TestModel, testedAt, attempt)
		} else {
			lastResult = resultFromUpstream(resp, cfg, testedAt, attempt)
		}

		if !shouldRetry(lastResult.ErrorCategory) {
			break
		}
	}
	return lastResult
}

func (s *ModelTestService) sendTestRequest(channel *model.Channel, cfg resolvedModelTestConfig) (*upstreamTestResponse, error) {
	apiType := constant.APITypeFromChannelType(channel.Type)
	reqBody, err := buildOpenAIStyleTestBody(cfg.TestModel, cfg.TestPrompt, cfg.MaxTokens, false)
	if err != nil {
		return nil, err
	}

	adaptor := relayadaptor.GetAdaptor(apiType)
	mode := constant.RelayModeChatCompletions
	format := constant.RelayFormatOpenAI

	if metaAdaptor, ok := adaptor.(relayadaptor.RequestMetaAwareAdaptor); ok {
		reqBody, err = metaAdaptor.ConvertRequestWithMeta(reqBody, mode, format, protocol.RequestMeta{Model: cfg.TestModel, Stream: false})
	} else {
		reqBody, err = adaptor.ConvertRequest(reqBody, mode, format)
	}
	if err != nil {
		return nil, fmt.Errorf("unsupported_protocol: %w", err)
	}

	requestURL := adaptor.GetRequestURL(channel.BaseURL, mode)
	if modelURLAdaptor, ok := adaptor.(relayadaptor.ModelAwareURLAdaptor); ok {
		requestURL = modelURLAdaptor.GetRequestURLWithModel(channel.BaseURL, mode, cfg.TestModel, false)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSecs)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	adaptor.SetupHeaders(req.Header, channel.APIKey, mode)

	client := &http.Client{Timeout: time.Duration(cfg.TimeoutSecs) * time.Second}
	startedAt := time.Now()
	resp, err := client.Do(req)
	latency := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &upstreamTestResponse{StatusCode: resp.StatusCode, Body: body, LatencyMS: latency}, nil
}

func normalizeModelTestConfig(cfg ModelTestConfig) ModelTestConfig {
	defaults := DefaultModelTestConfig()
	if cfg.TimeoutSecs <= 0 {
		cfg.TimeoutSecs = defaults.TimeoutSecs
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.DegradedThresholdMS <= 0 {
		cfg.DegradedThresholdMS = defaults.DegradedThresholdMS
	}
	if strings.TrimSpace(cfg.TestPrompt) == "" {
		cfg.TestPrompt = defaults.TestPrompt
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaults.MaxTokens
	}
	if cfg.DefaultModels == nil {
		cfg.DefaultModels = map[string]string{}
	}
	for key, value := range defaults.DefaultModels {
		if strings.TrimSpace(cfg.DefaultModels[key]) == "" {
			cfg.DefaultModels[key] = value
		}
	}
	// 第一批先统一使用非流式真实短请求，后续再补 SSE TTFB。
	cfg.Stream = false
	return cfg
}

func resolveTestModel(channel *model.Channel, cfg ModelTestConfig) string {
	if channel == nil {
		return ""
	}
	channelType := strings.ToLower(strings.TrimSpace(channel.Type))
	if channelType == "" {
		channelType = "openai_compatible"
	}
	if modelName := strings.TrimSpace(cfg.DefaultModels[channelType]); modelName != "" {
		return modelName
	}
	apiType := constant.APITypeFromChannelType(channelType).String()
	if modelName := strings.TrimSpace(cfg.DefaultModels[apiType]); modelName != "" {
		return modelName
	}
	if len(channel.Models) > 0 {
		return strings.TrimSpace(channel.Models[0])
	}
	return ""
}

func buildOpenAIStyleTestBody(modelName, prompt string, maxTokens int, stream bool) ([]byte, error) {
	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
		"stream":     stream,
	}
	return json.Marshal(body)
}

func resultFromUpstream(resp *upstreamTestResponse, cfg resolvedModelTestConfig, testedAt time.Time, retryCount int) *ModelTestResult {
	statusCode := resp.StatusCode
	result := &ModelTestResult{
		Success:        false,
		Status:         "unhealthy",
		ResponseTimeMS: resp.LatencyMS,
		HTTPStatus:     &statusCode,
		ModelUsed:      cfg.TestModel,
		TestedAt:       testedAt,
		RetryCount:     retryCount,
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.ErrorCategory = classifyHTTPError(resp.StatusCode, resp.Body)
		result.Message = upstreamErrorMessage(resp.Body, fmt.Sprintf("上游返回 HTTP %d", resp.StatusCode))
		return result
	}

	if !isValidJSONResponse(resp.Body) {
		result.ErrorCategory = "invalid_response"
		result.Message = "上游响应格式无法解析"
		return result
	}

	result.Success = true
	if resp.LatencyMS > cfg.DegradedThresholdMS {
		result.Status = "degraded"
		result.Message = "模型测试通过，但响应延迟较高"
		return result
	}
	result.Status = "healthy"
	result.Message = "模型测试通过"
	return result
}

func resultFromError(err error, modelName string, testedAt time.Time, retryCount int) *ModelTestResult {
	category := classifyNetworkError(err)
	message := err.Error()
	if strings.HasPrefix(message, "unsupported_protocol:") {
		message = strings.TrimSpace(strings.TrimPrefix(message, "unsupported_protocol:"))
	}
	return &ModelTestResult{
		Success:       false,
		Status:        "unhealthy",
		Message:       message,
		ModelUsed:     modelName,
		TestedAt:      testedAt,
		RetryCount:    retryCount,
		ErrorCategory: category,
	}
}

func classifyHTTPError(statusCode int, body []byte) string {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "auth_failed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusNotFound:
		return "model_not_found"
	}
	bodyText := strings.ToLower(string(body))
	if strings.Contains(bodyText, "model") && (strings.Contains(bodyText, "not found") || strings.Contains(bodyText, "does not exist") || strings.Contains(bodyText, "not exist")) {
		return "model_not_found"
	}
	if statusCode >= 500 {
		return "server_error"
	}
	return "invalid_response"
}

func classifyNetworkError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unsupported_protocol") {
		return "unsupported_protocol"
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "timeout") || strings.Contains(message, "deadline exceeded") {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return classifyNetworkError(urlErr.Err)
	}
	return "network_error"
}

func shouldRetry(category string) bool {
	switch category {
	case "timeout", "network_error", "server_error", "rate_limited":
		return true
	default:
		return false
	}
}

func isValidJSONResponse(body []byte) bool {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	if _, hasError := payload["error"]; hasError {
		return false
	}
	return true
}

func upstreamErrorMessage(body []byte, fallback string) string {
	if len(body) == 0 {
		return fallback
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		text := strings.TrimSpace(string(body))
		if text != "" {
			return text
		}
		return fallback
	}
	if errorValue, ok := payload["error"]; ok {
		switch typed := errorValue.(type) {
		case map[string]interface{}:
			if message, ok := typed["message"].(string); ok && strings.TrimSpace(message) != "" {
				return message
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		}
	}
	if message, ok := payload["message"].(string); ok && strings.TrimSpace(message) != "" {
		return message
	}
	return fallback
}

func modelTestLogFromResult(channel *model.Channel, result *ModelTestResult) *model.ModelTestLog {
	responseTime := result.ResponseTimeMS
	return &model.ModelTestLog{
		ChannelID:      channel.ID,
		ChannelName:    channel.Name,
		Status:         result.Status,
		Success:        result.Success,
		Message:        result.Message,
		ResponseTimeMS: &responseTime,
		TTFBMS:         result.TTFBMS,
		HTTPStatus:     result.HTTPStatus,
		ModelUsed:      result.ModelUsed,
		RetryCount:     result.RetryCount,
		ErrorCategory:  result.ErrorCategory,
		TestedAt:       result.TestedAt,
	}
}

func stringFromMap(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intFromMap(values map[string]interface{}, key string) int {
	value, ok := values[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		var parsed int
		_, _ = fmt.Sscanf(typed, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}
