package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/forwarder"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) handleResponsesBridge(c *gin.Context) {
	respCtx, ok := rc.newResponsesBridgeRequestContext(c)
	if !ok {
		return
	}
	if respCtx.Meta.Stream {
		rc.relayResponsesStream(respCtx)
		return
	}
	rc.relayResponsesJSON(respCtx)
}

type responsesRequestContext struct {
	*RequestContext
	ResponsesBody []byte
	ChatBody      []byte
}

func (rc *RelayController) newResponsesBridgeRequestContext(c *gin.Context) (*responsesRequestContext, bool) {
	startTime := time.Now()
	requestID := requestID(c)
	mode := constant.RelayModeResponses
	format := constant.RelayFormatOpenAIResponses

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return nil, false
	}

	chatBody, modelName, stream, err := responsesRequestToChatCompletions(body, clientRequestedEventStream(c))
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return nil, false
	}

	meta := relayRequestMeta{Model: modelName, Stream: stream}
	reqCtx := &RequestContext{
		Gin:          c,
		RequestID:    requestID,
		StartTime:    startTime,
		Mode:         mode,
		Format:       format,
		Method:       c.Request.Method,
		OriginalPath: c.Request.URL.Path,
		Endpoint:     c.Request.URL.Path,
		Query:        c.Request.URL.RawQuery,
		RawBody:      body,
		Model:        meta.Model,
		Stream:       meta.Stream,
		Headers:      c.Request.Header.Clone(),
		Body:         chatBody,
		Meta:         meta,
	}
	reqCtx.forwarderContext = reqCtx.toForwarderContext()
	if err := rc.attachCandidates(reqCtx); err != nil {
		rc.logNoChannel(c, requestID, startTime, mode, format, modelName, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return nil, false
	}
	if len(reqCtx.Candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, mode, format, modelName, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return nil, false
	}
	return &responsesRequestContext{
		RequestContext: reqCtx,
		ResponsesBody:  body,
		ChatBody:       chatBody,
	}, true
}

func (rc *RelayController) relayResponsesJSON(respCtx *responsesRequestContext) {
	resp, attempt, kind, err := rc.forwardResponsesAttempt(respCtx, false, relayJSONPreflight)
	if err != nil {
		statusCode, errMsg := relayFailureDetails(err, attempt)
		if attempt != nil {
			rc.logRequest(respCtx.Gin, attempt.Info, statusCode, errMsg)
		}
		writeFinalResponsesError(respCtx.Gin, err, errMsg, attempt != nil, statusCode)
		return
	}
	defer resp.Body.Close()

	responsesBody, err := responseProcessor.ReadAndTransform(resp, func(respBody []byte) ([]byte, error) {
		return responsesAttemptJSONBody(respCtx, attempt, kind, resp.Header, respBody)
	})
	if err != nil {
		errMsg := err.Error()
		if rc.providerRouter != nil {
			rc.providerRouter.RecordFailure(attempt.Info.Channel.ID, errMsg)
		}
		rc.logRequest(respCtx.Gin, attempt.Info, http.StatusBadGateway, errMsg)
		writeFinalResponsesError(respCtx.Gin, err, errMsg, true, http.StatusBadGateway)
		return
	}

	if rc.providerRouter != nil {
		rc.providerRouter.RecordSuccess(attempt.Info.Channel.ID)
	}
	rc.logRequest(respCtx.Gin, attempt.Info, resp.StatusCode, "")
	responseProcessor.WriteBody(respCtx.Gin.Writer, resp.StatusCode, resp.Header, "application/json", responsesBody)
}

func responsesAttemptJSONBody(respCtx *responsesRequestContext, attempt *RelayAttempt, kind responsesAttemptKind, headers http.Header, respBody []byte) ([]byte, error) {
	if kind == responsesAttemptNative {
		if isResponsesSSE(headers, respBody) {
			return responsesSSEToJSON(respBody, attempt.Info.ResolvedModel)
		}
		return respBody, nil
	}

	chatResp := respBody
	if attempt.NeedsTransform {
		var err error
		chatResp, err = attempt.ProtocolAdaptor.ConvertResponse(respBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
		if err != nil {
			return nil, err
		}
	}
	return chatCompletionsResponseToResponses(chatResp, respCtx.Meta.Model)
}

func (rc *RelayController) relayResponsesStream(respCtx *responsesRequestContext) {
	resp, attempt, kind, err := rc.forwardResponsesAttempt(respCtx, true, relayStreamPreflight)
	if err != nil {
		statusCode, errMsg := relayFailureDetails(err, attempt)
		if attempt != nil {
			rc.logRequest(respCtx.Gin, attempt.Info, statusCode, errMsg)
		}
		writeFinalResponsesError(respCtx.Gin, err, errMsg, attempt != nil, statusCode)
		return
	}

	writeStreamHeaders(respCtx.Gin, resp.StatusCode, resp.Header)
	copyErr := copyResponsesAttemptStream(respCtx, attempt, kind, resp)
	_ = resp.Body.Close()
	if copyErr != nil {
		errMsg := copyErr.Error()
		if rc.providerRouter != nil {
			rc.providerRouter.RecordFailure(attempt.Info.Channel.ID, errMsg)
		}
		rc.logRequest(respCtx.Gin, attempt.Info, resp.StatusCode, errMsg)
		return
	}

	if rc.providerRouter != nil {
		rc.providerRouter.RecordSuccess(attempt.Info.Channel.ID)
	}
	rc.logRequest(respCtx.Gin, attempt.Info, resp.StatusCode, "")
}

func (rc *RelayController) forwardResponsesAttempt(respCtx *responsesRequestContext, isStream bool, preflight forwarder.ResponsePreflight) (*http.Response, *RelayAttempt, responsesAttemptKind, error) {
	selectedKind := responsesAttemptChatBridge
	resp, attempt, err := rc.forwardRelayAttempt(
		respCtx.RequestContext,
		isStream,
		func(provider model.Channel) (*RelayAttempt, error) {
			candidate, ok := respCtx.candidateForProvider(provider.ID)
			if !ok {
				return nil, newNonRetryableBuildError(http.StatusNotFound, "provider does not support requested model")
			}
			candidate.Channel = provider
			var lastErr error
			for _, kind := range responsesAttemptOrder(candidate) {
				attempt, err := rc.buildResponsesAttempt(respCtx, candidate, isStream, kind)
				if err == nil {
					selectedKind = kind
					return attempt, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		preflight,
	)
	return resp, attempt, selectedKind, err
}

func copyResponsesAttemptStream(respCtx *responsesRequestContext, attempt *RelayAttempt, kind responsesAttemptKind, resp *http.Response) error {
	if kind == responsesAttemptNative {
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			return copyNativeResponsesStream(respCtx.Gin, resp.Body)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return writeResponsesJSONAsStream(respCtx.Gin, body, attempt.Info.ResolvedModel)
	}
	return copyResponsesStream(respCtx.Gin, resp.Body, attempt, attempt.Info.ResolvedModel)
}

func (rc *RelayController) buildResponsesChatBridgeAttempt(respCtx *responsesRequestContext, candidate relayCandidate, isStream bool) (*RelayAttempt, error) {
	reqCtx := respCtx.RequestContext
	info := buildRelayInfo(reqCtx.Gin, reqCtx.RequestID, reqCtx.StartTime, reqCtx.Mode, reqCtx.Format, reqCtx.Meta, candidate, isStream)
	protocolAdaptor := adaptor.GetAdaptor(info.APIType)
	providerAdaptor := adaptor.AsProviderAdapter(protocolAdaptor)
	attempt := &RelayAttempt{Info: info, ProtocolAdaptor: protocolAdaptor, ProviderAdapter: providerAdaptor}

	requestBody, err := bodyWithResolvedModel(respCtx.ChatBody, info.RequestedModel, info.ResolvedModel, constant.RelayFormatOpenAI)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadRequest, err)
	}
	attempt.RequestBody = requestBody
	attempt.NeedsTransform = providerAdaptor.NeedsTransform(info.Channel, constant.RelayFormatOpenAI)

	if attempt.NeedsTransform {
		convertedBody, err := convertResponsesUpstreamRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			return attempt, newRelayAttemptBuildError(statusCode, err)
		}
		attempt.ConvertedBody = convertedBody
	} else {
		attempt.ConvertedBody = requestBody
	}

	baseURL, err := providerAdaptor.ExtractBaseURL(info.Channel)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadGateway, err)
	}
	apiKey, config := providerAdaptor.ExtractAuth(info.Channel)
	headers, err := providerAdaptor.GetAuthHeaders(apiKey, config, constant.RelayModeChatCompletions, isStream)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadGateway, err)
	}
	attempt.Headers = headers
	attempt.URL = providerAdaptor.BuildURL(baseURL, constant.RelayModeChatCompletions, info.ResolvedModel, isStream)

	return attempt, nil
}

func convertResponsesUpstreamRequest(protocolAdaptor adaptor.Adaptor, requestBody []byte, info *relayinfo.RelayInfo) ([]byte, error) {
	meta := protocol.RequestMeta{Model: info.ResolvedModel, Stream: info.IsStream}
	if metaAware, ok := protocolAdaptor.(adaptor.RequestMetaAwareAdaptor); ok {
		return metaAware.ConvertRequestWithMeta(requestBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI, meta)
	}
	return protocolAdaptor.ConvertRequest(requestBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}

func copyResponsesStream(c *gin.Context, body io.Reader, attempt *RelayAttempt, modelName string) error {
	responseID := "resp_" + time.Now().Format("20060102150405.000000000")
	messageID := "msg_" + time.Now().Format("20060102150405.000000000")
	emitter := newResponsesStreamEmitter(c.Writer, responseID, messageID, modelName)
	if err := emitter.start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		chunkBytes := []byte(line + "\n")
		convertedChunk := chunkBytes
		if attempt != nil && attempt.NeedsTransform {
			var err error
			convertedChunk, err = attempt.ProtocolAdaptor.ConvertStreamChunk(chunkBytes, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
			if err != nil {
				return err
			}
		}
		for _, delta := range extractChatStreamDeltas(convertedChunk) {
			if delta.Content != "" {
				if err := emitter.delta(delta.Content); err != nil {
					return err
				}
			}
			for _, toolCall := range delta.ToolCalls {
				if err := emitter.toolCallDelta(toolCall); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = emitter.complete()
		return err
	}
	return emitter.complete()
}

func responsesRequestToChatCompletions(body []byte, streamRequested bool) ([]byte, string, bool, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, "", false, err
	}

	modelName, _ := raw["model"].(string)
	if modelName == "" {
		return nil, "", false, fmt.Errorf("缺少 model 参数")
	}

	messages := make([]map[string]interface{}, 0)
	if instructions, _ := raw["instructions"].(string); instructions != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": instructions})
	}

	messages = append(messages, responsesInputToMessages(raw["input"])...)
	if len(messages) == 0 {
		return nil, "", false, fmt.Errorf("缺少 input 参数")
	}

	stream := responseStreamRequested(raw["stream"], streamRequested)
	chatReq := map[string]interface{}{
		"model":    modelName,
		"messages": messages,
		"stream":   stream,
	}

	usesCompletionTokens := usesCompletionTokenLimit(modelName)
	if !usesCompletionTokens {
		copyIfPresent(chatReq, raw, "temperature", "temperature")
		copyIfPresent(chatReq, raw, "top_p", "top_p")
	}
	if maxOutputTokens, ok := raw["max_output_tokens"]; ok {
		if usesCompletionTokens {
			chatReq["max_completion_tokens"] = maxOutputTokens
		} else {
			chatReq["max_tokens"] = maxOutputTokens
		}
	}
	if !usesCompletionTokens {
		copyIfPresent(chatReq, raw, "max_tokens", "max_tokens")
	}
	copyIfPresent(chatReq, raw, "max_completion_tokens", "max_completion_tokens")
	for _, key := range responseChatPassthroughFields(modelName) {
		copyIfPresent(chatReq, raw, key, key)
	}
	if tools, ok := raw["tools"]; ok {
		chatTools := responsesToolsToChatTools(tools)
		if len(chatTools) > 0 {
			chatReq["tools"] = chatTools
		}
	}
	if hasChatTools(chatReq) {
		copyIfPresent(chatReq, raw, "tool_choice", "tool_choice")
		copyIfPresent(chatReq, raw, "parallel_tool_calls", "parallel_tool_calls")
	}
	if stream {
		ensureStreamOptionsIncludeUsage(chatReq)
	}

	chatBody, err := json.Marshal(chatReq)
	return chatBody, modelName, stream, err
}

func usesCompletionTokenLimit(modelName string) bool {
	model := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") || strings.HasPrefix(model, "gpt-5")
}

func responseChatPassthroughFields(modelName string) []string {
	// GPT-5 / o 系列 Chat Completions 端点通常只接受更窄的参数集合。
	// 兼容上游经常会因 temperature/top_p/stop/logprobs 等旧 Chat 参数返回 400/openai_error。
	if usesCompletionTokenLimit(modelName) {
		return []string{"metadata", "n", "response_format", "seed", "service_tier", "stream_options", "user"}
	}
	return []string{"frequency_penalty", "logit_bias", "logprobs", "metadata", "n", "presence_penalty", "response_format", "seed", "service_tier", "stop", "stream_options", "top_logprobs", "user"}
}

func hasChatTools(chatReq map[string]interface{}) bool {
	tools, ok := chatReq["tools"].([]interface{})
	return ok && len(tools) > 0
}

func ensureStreamOptionsIncludeUsage(chatReq map[string]interface{}) {
	if existing, ok := chatReq["stream_options"].(map[string]interface{}); ok {
		existing["include_usage"] = true
		return
	}
	chatReq["stream_options"] = map[string]interface{}{"include_usage": true}
}

func clientRequestedEventStream(c *gin.Context) bool {
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "text/event-stream") || strings.EqualFold(c.Query("stream"), "true")
}

func responseStreamRequested(value interface{}, headerRequested bool) bool {
	// Responses API 的部分客户端通过 Accept: text/event-stream 或 query 参数
	// 表达流式意图，而请求体里可能没有 stream 字段（或保留默认 false）。
	// 一旦客户端按 SSE 解析，返回普通 JSON 会导致 “No Responses API events were parsed”。
	if headerRequested {
		return true
	}
	if stream, ok := value.(bool); ok {
		return stream
	}
	if stream, ok := value.(string); ok {
		return strings.EqualFold(stream, "true")
	}
	return false
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

	itemType, _ := inputItem["type"].(string)
	if itemType == "function_call_output" {
		callID, _ := inputItem["call_id"].(string)
		if callID == "" {
			callID, _ = inputItem["tool_call_id"].(string)
		}
		output := responseContentToText(inputItem["output"])
		if output == "" {
			output = responseContentToText(inputItem["content"])
		}
		if output == "" {
			output = responseJSONString(inputItem["output"])
		}
		if callID == "" || output == "" {
			return nil, false
		}
		return map[string]interface{}{"role": "tool", "tool_call_id": callID, "content": output}, true
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
	var toolCalls []interface{}
	if choices, ok := chatResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				outputText, _ = message["content"].(string)
				if calls, ok := message["tool_calls"].([]interface{}); ok {
					toolCalls = calls
				}
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
	if len(toolCalls) > 0 {
		response["output"] = chatToolCallsToResponsesOutput(toolCalls)
		response["output_text"] = outputText
	}
	if usage, ok := chatResp["usage"].(map[string]interface{}); ok {
		response["usage"] = map[string]interface{}{
			"input_tokens":  usage["prompt_tokens"],
			"output_tokens": usage["completion_tokens"],
			"total_tokens":  usage["total_tokens"],
		}
	}

	return json.Marshal(response)
}

type chatStreamDelta struct {
	Content      string
	ToolCalls    []map[string]interface{}
	FinishReason string
}

func extractChatStreamContent(chunk []byte) []string {
	deltas := extractChatStreamDeltas(chunk)
	contents := make([]string, 0, len(deltas))
	for _, delta := range deltas {
		contents = append(contents, delta.Content)
	}
	return contents
}

func extractChatStreamDeltas(chunk []byte) []chatStreamDelta {
	scanner := bufio.NewScanner(bytes.NewReader(chunk))
	deltas := make([]chatStreamDelta, 0)
	var dataLines []string

	flushData := func() {
		if len(dataLines) == 0 {
			return
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" || data == "[DONE]" {
			return
		}

		var chatChunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chatChunk); err != nil {
			return
		}
		choices, ok := chatChunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			return
		}
		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			return
		}
		deltaMap, ok := choice["delta"].(map[string]interface{})
		if !ok {
			return
		}
		delta := chatStreamDelta{}
		delta.Content, _ = deltaMap["content"].(string)
		if finishReason, _ := choice["finish_reason"].(string); finishReason != "" {
			delta.FinishReason = finishReason
		}
		if calls, ok := deltaMap["tool_calls"].([]interface{}); ok {
			for _, value := range calls {
				if call, ok := value.(map[string]interface{}); ok {
					delta.ToolCalls = append(delta.ToolCalls, normalizeChatToolCallDelta(call))
				}
			}
		}
		deltas = append(deltas, delta)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			flushData()
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flushData()
	return deltas
}

type responsesStreamEmitter struct {
	writer          gin.ResponseWriter
	responseID      string
	messageID       string
	modelName       string
	sequence        int
	collectedText   strings.Builder
	toolCalls       map[int]map[string]interface{}
	toolOutputIndex map[int]int
}

func newResponsesStreamEmitter(writer gin.ResponseWriter, responseID, messageID, modelName string) *responsesStreamEmitter {
	return &responsesStreamEmitter{
		writer:          writer,
		responseID:      responseID,
		messageID:       messageID,
		modelName:       modelName,
		toolCalls:       map[int]map[string]interface{}{},
		toolOutputIndex: map[int]int{},
	}
}

func (e *responsesStreamEmitter) start() error {
	if err := e.write("response.created", map[string]interface{}{
		"type":     "response.created",
		"response": baseResponsesObject(e.responseID, e.messageID, e.modelName, "in_progress", ""),
	}); err != nil {
		return err
	}
	if err := e.write("response.in_progress", map[string]interface{}{
		"type":     "response.in_progress",
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
	e.collectedText.WriteString(content)
	return e.write("response.output_text.delta", map[string]interface{}{
		"type":          "response.output_text.delta",
		"item_id":       e.messageID,
		"output_index":  0,
		"content_index": 0,
		"delta":         content,
	})
}

func (e *responsesStreamEmitter) toolCallDelta(delta map[string]interface{}) error {
	index := chatToolCallIndex(delta)
	current := e.toolCalls[index]
	current = mergeChatToolCallDelta(current, delta)
	e.toolCalls[index] = current
	outputIndex, ok := e.toolOutputIndex[index]
	if !ok {
		outputIndex = 1 + len(e.toolOutputIndex)
		e.toolOutputIndex[index] = outputIndex
		item := chatToolCallToResponsesItem(current)
		if err := e.write("response.output_item.added", map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": outputIndex,
			"item":         item,
		}); err != nil {
			return err
		}
	}
	_, arguments := chatToolCallNameAndArguments(delta)
	if arguments != "" {
		if err := e.write("response.function_call_arguments.delta", map[string]interface{}{
			"type":         "response.function_call_arguments.delta",
			"item_id":      responseToolCallID(current),
			"output_index": outputIndex,
			"delta":        arguments,
		}); err != nil {
			return err
		}
	}
	return nil
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
	for index, toolCall := range e.toolCalls {
		outputIndex := e.toolOutputIndex[index]
		item := chatToolCallToResponsesItem(toolCall)
		if err := e.write("response.function_call_arguments.done", map[string]interface{}{
			"type":         "response.function_call_arguments.done",
			"item_id":      responseToolCallID(toolCall),
			"output_index": outputIndex,
			"arguments":    item["arguments"],
		}); err != nil {
			return err
		}
		item["status"] = "completed"
		if err := e.write("response.output_item.done", map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": outputIndex,
			"item":         item,
		}); err != nil {
			return err
		}
	}
	completed := baseResponsesObject(e.responseID, e.messageID, e.modelName, "completed", outputText)
	if len(e.toolCalls) > 0 {
		completed["output"] = e.completedOutputItems(outputText)
		completed["output_text"] = outputText
	}
	return e.write("response.completed", map[string]interface{}{
		"type":     "response.completed",
		"response": completed,
	})
}

func (e *responsesStreamEmitter) completedOutputItems(outputText string) []interface{} {
	items := []interface{}{
		map[string]interface{}{
			"id":     e.messageID,
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "output_text", "text": outputText, "annotations": []interface{}{}},
			},
		},
	}
	for index, toolCall := range e.toolCalls {
		item := chatToolCallToResponsesItem(toolCall)
		item["status"] = "completed"
		if outputIndex, ok := e.toolOutputIndex[index]; ok {
			item["output_index"] = outputIndex
		}
		items = append(items, item)
	}
	return items
}

func (e *responsesStreamEmitter) write(eventName string, payload map[string]interface{}) error {
	e.sequence++
	payload["sequence_number"] = e.sequence

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
	now := time.Now().Unix()
	var completedAt interface{}
	if status == "completed" {
		completedAt = now
	}

	output := []interface{}{}
	if status == "completed" || outputText != "" {
		output = append(output, map[string]interface{}{
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
		})
	}

	return map[string]interface{}{
		"id":                   responseID,
		"object":               "response",
		"created_at":           now,
		"completed_at":         completedAt,
		"status":               status,
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         nil,
		"max_output_tokens":    nil,
		"metadata":             map[string]interface{}{},
		"model":                modelName,
		"output":               output,
		"output_text":          outputText,
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"reasoning": map[string]interface{}{
			"effort":  nil,
			"summary": nil,
		},
		"store":       false,
		"temperature": nil,
		"text": map[string]interface{}{
			"format": map[string]interface{}{"type": "text"},
		},
		"tool_choice": "auto",
		"tools":       []interface{}{},
		"top_p":       nil,
		"truncation":  "disabled",
		"usage":       nil,
		"user":        nil,
	}
}

func responsesToolsToChatTools(value interface{}) []interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	tools := make([]interface{}, 0, len(items))
	for _, item := range items {
		tool, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		toolType, _ := tool["type"].(string)
		if toolType == "function" && tool["function"] != nil {
			tools = append(tools, tool)
			continue
		}
		if name, _ := tool["name"].(string); name != "" {
			function := map[string]interface{}{
				"name":        name,
				"description": tool["description"],
				"parameters":  tool["parameters"],
			}
			if function["parameters"] == nil {
				function["parameters"] = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			tools = append(tools, map[string]interface{}{"type": "function", "function": function})
		}
	}
	return tools
}

func chatToolCallsToResponsesOutput(toolCalls []interface{}) []interface{} {
	output := make([]interface{}, 0, len(toolCalls))
	for _, value := range toolCalls {
		toolCall, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		item := chatToolCallToResponsesItem(toolCall)
		item["status"] = "completed"
		output = append(output, item)
	}
	return output
}

func normalizeChatToolCallDelta(delta map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if index, ok := delta["index"]; ok {
		out["index"] = index
	}
	if id, _ := delta["id"].(string); id != "" {
		out["id"] = id
	}
	if typ, _ := delta["type"].(string); typ != "" {
		out["type"] = typ
	} else {
		out["type"] = "function"
	}
	if function, ok := delta["function"].(map[string]interface{}); ok {
		fn := map[string]interface{}{}
		if name, _ := function["name"].(string); name != "" {
			fn["name"] = name
		}
		if arguments, _ := function["arguments"].(string); arguments != "" {
			fn["arguments"] = arguments
		}
		if len(fn) > 0 {
			out["function"] = fn
		}
	}
	return out
}

func mergeChatToolCallDelta(existing map[string]interface{}, delta map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = map[string]interface{}{"type": "function", "function": map[string]interface{}{}}
	}
	if id, _ := delta["id"].(string); id != "" {
		existing["id"] = id
	}
	if typ, _ := delta["type"].(string); typ != "" {
		existing["type"] = typ
	}
	fn, _ := existing["function"].(map[string]interface{})
	if fn == nil {
		fn = map[string]interface{}{}
		existing["function"] = fn
	}
	if deltaFn, _ := delta["function"].(map[string]interface{}); deltaFn != nil {
		if name, _ := deltaFn["name"].(string); name != "" {
			fn["name"] = name
		}
		if args, _ := deltaFn["arguments"].(string); args != "" {
			old, _ := fn["arguments"].(string)
			fn["arguments"] = old + args
		}
	}
	return existing
}

func chatToolCallIndex(toolCall map[string]interface{}) int {
	switch value := toolCall["index"].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		intValue, _ := value.Int64()
		return int(intValue)
	default:
		return 0
	}
}

func chatToolCallNameAndArguments(toolCall map[string]interface{}) (string, string) {
	function, _ := toolCall["function"].(map[string]interface{})
	name, _ := function["name"].(string)
	arguments, _ := function["arguments"].(string)
	return name, arguments
}

func responseToolCallID(toolCall map[string]interface{}) string {
	id, _ := toolCall["id"].(string)
	if id == "" {
		id = "fc_" + time.Now().Format("20060102150405.000000000")
		toolCall["id"] = id
	}
	return id
}

func chatToolCallToResponsesItem(toolCall map[string]interface{}) map[string]interface{} {
	id := responseToolCallID(toolCall)
	name, arguments := chatToolCallNameAndArguments(toolCall)
	return map[string]interface{}{
		"id":        id,
		"type":      "function_call",
		"status":    "in_progress",
		"call_id":   id,
		"name":      name,
		"arguments": arguments,
	}
}

func responseJSONString(value interface{}) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func mergeResponsesFunctionCallItem(existing map[string]interface{}, item map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = map[string]interface{}{"type": "function_call"}
	}
	for _, key := range []string{"id", "type", "status", "call_id", "name"} {
		if value, ok := item[key]; ok && value != nil && value != "" {
			existing[key] = value
		}
	}
	if arguments, _ := item["arguments"].(string); arguments != "" {
		existing["arguments"] = arguments
	}
	if existing["status"] == nil {
		existing["status"] = "in_progress"
	}
	if existing["call_id"] == nil {
		existing["call_id"] = existing["id"]
	}
	return existing
}

func copyIfPresent(dst map[string]interface{}, src map[string]interface{}, srcKey, dstKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}
