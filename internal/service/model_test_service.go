package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

type ModelTestService struct {
	modelRepo       *repository.ModelRepository
	settingsService *SettingsService
	httpClient      *http.Client
}

type ModelTestRequest struct {
	ChannelID       uint     `json:"channel_id"`
	Prompt          string   `json:"prompt"`
	TimeoutMS       int      `json:"timeout_ms"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Temperature     *float64 `json:"temperature"`
}

type ModelTestResult struct {
	OK            bool   `json:"ok"`
	ModelID       uint   `json:"model_id"`
	Model         string `json:"model"`
	ResolvedModel string `json:"resolved_model"`
	ChannelID     uint   `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ChannelType   string `json:"channel_type"`
	LatencyMS     int64  `json:"latency_ms"`
	StatusCode    int    `json:"status_code"`
	Content       string `json:"content"`
	Error         string `json:"error"`
}

type ModelTestChannel struct {
	ModelID      uint           `json:"model_id"`
	ModelName    string         `json:"model_name"`
	DisplayName  string         `json:"display_name"`
	TestEnabled  bool           `json:"test_enabled"`
	RouteEnabled bool           `json:"route_enabled"`
	Channel      *model.Channel `json:"channel"`
}

func NewModelTestService(modelRepo *repository.ModelRepository, settingsService *SettingsService, httpClient *http.Client) *ModelTestService {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &ModelTestService{modelRepo: modelRepo, settingsService: settingsService, httpClient: httpClient}
}

func (s *ModelTestService) GetTestChannels(modelID uint) ([]ModelTestChannel, error) {
	baseModel, err := s.modelRepo.GetByID(modelID)
	if err != nil {
		return nil, err
	}
	candidates, err := s.testCandidates(baseModel)
	if err != nil {
		return nil, err
	}
	items := make([]ModelTestChannel, 0, len(candidates))
	for i := range candidates {
		m := candidates[i]
		items = append(items, ModelTestChannel{
			ModelID:      m.ID,
			ModelName:    m.Name,
			DisplayName:  effectiveDisplayName(&m),
			TestEnabled:  m.TestEnabled,
			RouteEnabled: m.Enabled,
			Channel:      m.Channel,
		})
	}
	return items, nil
}

func (s *ModelTestService) TestModel(ctx context.Context, modelID uint, req ModelTestRequest) (*ModelTestResult, int, error) {
	baseModel, err := s.modelRepo.GetByID(modelID)
	if err != nil {
		return nil, http.StatusNotFound, fmt.Errorf("模型不存在")
	}
	if !baseModel.TestEnabled {
		return nil, http.StatusBadRequest, fmt.Errorf("该模型未允许测试")
	}

	settings := DefaultSettings()
	if s.settingsService != nil {
		if loaded, err := s.settingsService.GetSettings(); err == nil {
			settings = loaded
		}
	}
	req = mergeModelTestRequest(req, settings.ModelTest)

	target, err := s.resolveTargetModel(baseModel, req.ChannelID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	if target.Channel == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("测试渠道不存在")
	}

	result := &ModelTestResult{
		ModelID:       target.ID,
		Model:         effectiveDisplayName(baseModel),
		ResolvedModel: target.Name,
		ChannelID:     target.ChannelID,
		ChannelName:   target.Channel.Name,
		ChannelType:   target.Channel.Type,
	}

	body, err := buildOpenAIChatTestBody(target.Name, req.Prompt, req.MaxOutputTokens, req.Temperature)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	apiType := constant.APITypeFromChannelType(target.Channel.Type)
	protocolAdaptor := adaptor.GetAdaptor(apiType)
	providerAdaptor := adaptor.AsProviderAdapter(protocolAdaptor)
	convertedBody, err := protocolAdaptor.ConvertRequest(body, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("构造测试请求失败: %w", err)
	}
	baseURL, err := providerAdaptor.ExtractBaseURL(target.Channel)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	apiKey, config := providerAdaptor.ExtractAuth(target.Channel)
	url := providerAdaptor.BuildURL(baseURL, constant.RelayModeChatCompletions, target.Name, false)
	headers, err := providerAdaptor.GetAuthHeaders(apiKey, config, constant.RelayModeChatCompletions, false)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, url, bytes.NewReader(convertedBody))
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	httpReq.Header = headers
	start := time.Now()
	resp, err := s.httpClient.Do(httpReq)
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result, http.StatusOK, nil
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		result.Error = err.Error()
		return result, http.StatusOK, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = upstreamErrorMessage(protocolAdaptor, respBody)
		if retried, retryErr := s.retryMinimalTestRequest(timeoutCtx, protocolAdaptor, target, url, headers, req.Prompt, result); retryErr == nil && retried != nil {
			return retried, http.StatusOK, nil
		}
		return result, http.StatusOK, nil
	}
	if err := fillSuccessfulTestResult(result, protocolAdaptor, respBody); err != nil {
		return result, http.StatusOK, nil
	}
	return result, http.StatusOK, nil
}

func (s *ModelTestService) retryMinimalTestRequest(ctx context.Context, protocolAdaptor adaptor.Adaptor, target *model.Model, url string, headers http.Header, prompt string, previous *ModelTestResult) (*ModelTestResult, error) {
	minimalBody, err := buildOpenAIChatTestBody(target.Name, prompt, 0, nil)
	if err != nil {
		return nil, err
	}
	convertedBody, err := protocolAdaptor.ConvertRequest(minimalBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(convertedBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header = headers.Clone()
	result := *previous
	start := time.Now()
	resp, err := s.httpClient.Do(httpReq)
	result.LatencyMS += time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = previous.Error + "；最小测试请求仍失败: " + upstreamErrorMessage(protocolAdaptor, respBody)
		return &result, nil
	}
	if err := fillSuccessfulTestResult(&result, protocolAdaptor, respBody); err != nil {
		result.Error = "默认测试参数被上游拒绝，最小测试请求成功但解析响应失败: " + err.Error()
		return &result, nil
	}
	result.Error = "默认测试参数被上游拒绝，已自动使用最小测试请求重试成功"
	return &result, nil
}

func fillSuccessfulTestResult(result *ModelTestResult, protocolAdaptor adaptor.Adaptor, respBody []byte) error {
	convertedResp, err := protocolAdaptor.ConvertResponse(respBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
	if err != nil {
		result.Error = fmt.Sprintf("解析测试响应失败: %v", err)
		result.Content = strings.TrimSpace(string(respBody))
		return err
	}
	content, err := extractOpenAIChatContent(convertedResp)
	if err != nil {
		result.Error = err.Error()
		result.Content = strings.TrimSpace(string(convertedResp))
		return err
	}
	result.OK = true
	result.Content = content
	return nil
}

func upstreamErrorMessage(protocolAdaptor adaptor.Adaptor, respBody []byte) string {
	message := ""
	if protocolAdaptor != nil {
		message = protocolAdaptor.ErrorMessage(respBody)
	}
	if message == "" {
		message = string(respBody)
	}
	return strings.TrimSpace(message)
}

func (s *ModelTestService) testCandidates(baseModel *model.Model) ([]model.Model, error) {
	return s.modelRepo.GetTestCandidatesByDisplayName(effectiveDisplayName(baseModel))
}

func (s *ModelTestService) resolveTargetModel(baseModel *model.Model, channelID uint) (*model.Model, error) {
	candidates, err := s.testCandidates(baseModel)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("同一调用名下没有允许测试的模型记录")
	}
	if channelID == 0 {
		return &candidates[0], nil
	}
	for i := range candidates {
		if candidates[i].ChannelID == channelID {
			return &candidates[i], nil
		}
	}
	return nil, fmt.Errorf("所选渠道不属于该调用名的可测试模型记录")
}

func mergeModelTestRequest(req ModelTestRequest, settings ModelTestSettings) ModelTestRequest {
	settings = sanitizeModelTestSettings(settings)
	if strings.TrimSpace(req.Prompt) == "" {
		req.Prompt = settings.DefaultPrompt
	}
	if req.TimeoutMS <= 0 {
		req.TimeoutMS = settings.TimeoutMS
	}
	if req.MaxOutputTokens <= 0 {
		req.MaxOutputTokens = settings.MaxOutputTokens
	}
	if req.Temperature == nil {
		t := settings.Temperature
		req.Temperature = &t
	}
	return req
}

func buildOpenAIChatTestBody(modelName, prompt string, maxTokens int, temperature *float64) ([]byte, error) {
	chatReq := protocol.ChatRequest{
		Model:       modelName,
		Temperature: temperature,
		Messages: []protocol.ChatMessage{
			{Role: "user", Content: prompt},
		},
	}
	if maxTokens > 0 {
		chatReq.MaxTokens = &maxTokens
	}
	return protocol.ProtocolToOpenAIChatRequest(&chatReq)
}

func extractOpenAIChatContent(body []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("响应中没有 choices")
	}
	content := contentToText(payload.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("响应内容为空")
	}
	return content, nil
}

func contentToText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func effectiveDisplayName(m *model.Model) string {
	if m == nil {
		return ""
	}
	if strings.TrimSpace(m.DisplayName) != "" {
		return m.DisplayName
	}
	return m.Name
}
