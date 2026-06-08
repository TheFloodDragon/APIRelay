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

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) handleResponsesBridgeWithApp(c *gin.Context, app constant.RelayApp) {
	startTime := time.Now()
	requestID := requestID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return
	}

	chatBody, modelName, stream, err := responsesRequestToChatCompletions(body, clientRequestedEventStream(c))
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}

	meta := relayRequestMeta{Model: modelName, Stream: stream}
	candidates, err := rc.resolveCandidates(modelName)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, modelName, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return
	}
	if len(candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, modelName, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return
	}

	if stream {
		rc.relayResponsesStream(c, requestID, startTime, app, meta, chatBody, candidates)
		return
	}
	rc.relayResponsesJSON(c, requestID, startTime, app, meta, chatBody, candidates)
}

func (rc *RelayController) relayResponsesJSON(c *gin.Context, requestID string, startTime time.Time, app constant.RelayApp, meta relayRequestMeta, chatBody []byte, candidates []relayCandidate) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range candidates {
		info := buildRelayInfo(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, meta, candidate, false)
		protocolAdaptor := adaptor.GetAdaptor(info.APIType)

		requestBody, err := bodyWithResolvedModel(chatBody, info.ResolvedModel, constant.RelayFormatOpenAI)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, http.StatusBadRequest, lastErrMsg)
			continue
		}

		convertedBody, err := convertResponsesUpstreamRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		attemptedUpstream = true
		headers := http.Header{}
		protocolAdaptor.SetupHeaders(headers, info.Channel.APIKey, constant.RelayModeChatCompletions)
		url := responsesUpstreamURL(protocolAdaptor, info, false)

		statusCode, respBody, err := rc.httpClient.DoJSON(c.Request.Context(), c.Request.Method, url, headers, convertedBody, timeoutForChannel(info.Channel))
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			chatResp, err := protocolAdaptor.ConvertResponse(respBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
			if err != nil {
				lastErr = err
				lastErrMsg = err.Error()
				rc.logRequest(c, info, http.StatusBadGateway, lastErrMsg)
				continue
			}
			responsesBody, err := chatCompletionsResponseToResponses(chatResp, meta.Model)
			if err != nil {
				lastErr = err
				lastErrMsg = err.Error()
				rc.logRequest(c, info, http.StatusBadGateway, lastErrMsg)
				continue
			}

			rc.logRequest(c, info, statusCode, "")
			c.Data(statusCode, "application/json", responsesBody)
			return
		}

		lastErr = nil
		lastErrMsg = protocolAdaptor.ErrorMessage(respBody)
		if lastErrMsg == "" {
			lastErrMsg = string(respBody)
		}
		rc.logRequest(c, info, statusCode, lastErrMsg)
	}

	writeFinalRelayError(c, lastErr, lastErrMsg, attemptedUpstream)
}

func (rc *RelayController) relayResponsesStream(c *gin.Context, requestID string, startTime time.Time, app constant.RelayApp, meta relayRequestMeta, chatBody []byte, candidates []relayCandidate) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range candidates {
		info := buildRelayInfo(c, requestID, startTime, app, constant.RelayModeResponses, constant.RelayFormatOpenAIResponses, meta, candidate, true)
		protocolAdaptor := adaptor.GetAdaptor(info.APIType)

		requestBody, err := bodyWithResolvedModel(chatBody, info.ResolvedModel, constant.RelayFormatOpenAI)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, http.StatusBadRequest, lastErrMsg)
			continue
		}

		convertedBody, err := convertResponsesUpstreamRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		attemptedUpstream = true
		headers := http.Header{}
		protocolAdaptor.SetupHeaders(headers, info.Channel.APIKey, constant.RelayModeChatCompletions)
		headers.Set("Accept", "text/event-stream")
		url := responsesUpstreamURL(protocolAdaptor, info, true)

		resp, err := rc.httpClient.DoStream(c.Request.Context(), c.Request.Method, url, headers, convertedBody, timeoutForChannel(info.Channel))
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, 0, lastErrMsg)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errorBody, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				lastErrMsg = readErr.Error()
			} else {
				lastErr = nil
				lastErrMsg = protocolAdaptor.ErrorMessage(errorBody)
				if lastErrMsg == "" {
					lastErrMsg = string(errorBody)
				}
			}
			rc.logRequest(c, info, resp.StatusCode, lastErrMsg)
			continue
		}

		writeStreamHeaders(c, resp.StatusCode)
		copyErr := copyResponsesStream(c, resp.Body, protocolAdaptor, info.ResolvedModel)
		_ = resp.Body.Close()
		if copyErr != nil {
			lastErr = copyErr
			lastErrMsg = copyErr.Error()
			rc.logRequest(c, info, resp.StatusCode, lastErrMsg)
			return
		}

		rc.logRequest(c, info, resp.StatusCode, "")
		return
	}

	writeFinalRelayError(c, lastErr, lastErrMsg, attemptedUpstream)
}

func convertResponsesUpstreamRequest(protocolAdaptor adaptor.Adaptor, requestBody []byte, info *relayinfo.RelayInfo) ([]byte, error) {
	meta := protocol.RequestMeta{Model: info.ResolvedModel, Stream: info.IsStream}
	if metaAware, ok := protocolAdaptor.(adaptor.RequestMetaAwareAdaptor); ok {
		return metaAware.ConvertRequestWithMeta(requestBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI, meta)
	}
	return protocolAdaptor.ConvertRequest(requestBody, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}

func responsesUpstreamURL(protocolAdaptor adaptor.Adaptor, info *relayinfo.RelayInfo, stream bool) string {
	if urlAdaptor, ok := protocolAdaptor.(adaptor.ModelAwareURLAdaptor); ok {
		return urlAdaptor.GetRequestURLWithModel(info.Channel.BaseURL, constant.RelayModeChatCompletions, info.ResolvedModel, stream)
	}
	return protocolAdaptor.GetRequestURL(info.Channel.BaseURL, constant.RelayModeChatCompletions)
}

func copyResponsesStream(c *gin.Context, body io.Reader, protocolAdaptor adaptor.Adaptor, modelName string) error {
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
		convertedChunk, err := protocolAdaptor.ConvertStreamChunk(chunkBytes, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
		if err != nil {
			return err
		}
		for _, content := range extractChatStreamContent(convertedChunk) {
			if content == "" {
				continue
			}
			if err := emitter.delta(content); err != nil {
				return err
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

	copyIfPresent(chatReq, raw, "temperature", "temperature")
	copyIfPresent(chatReq, raw, "top_p", "top_p")
	copyIfPresent(chatReq, raw, "max_output_tokens", "max_tokens")
	copyIfPresent(chatReq, raw, "max_tokens", "max_tokens")
	copyIfPresent(chatReq, raw, "tools", "tools")

	chatBody, err := json.Marshal(chatReq)
	return chatBody, modelName, stream, err
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
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			return
		}
		content, _ := delta["content"].(string)
		contents = append(contents, content)
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

func copyIfPresent(dst map[string]interface{}, src map[string]interface{}, srcKey, dstKey string) {
	if value, ok := src[srcKey]; ok {
		dst[dstKey] = value
	}
}
