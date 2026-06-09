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
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) buildResponsesAttempt(respCtx *responsesRequestContext, candidate relayCandidate, isStream bool, kind responsesAttemptKind) (*RelayAttempt, error) {
	switch kind {
	case responsesAttemptNative:
		return rc.buildResponsesNativeAttempt(respCtx, candidate, isStream)
	default:
		return rc.buildResponsesChatBridgeAttempt(respCtx, candidate, isStream)
	}
}

func (rc *RelayController) buildResponsesNativeAttempt(respCtx *responsesRequestContext, candidate relayCandidate, isStream bool) (*RelayAttempt, error) {
	reqCtx := respCtx.RequestContext
	info := buildRelayInfo(reqCtx.Gin, reqCtx.RequestID, reqCtx.StartTime, reqCtx.Mode, reqCtx.Format, reqCtx.Meta, candidate, isStream)
	protocolAdaptor := adaptor.GetAdaptor(info.APIType)
	providerAdaptor := adaptor.AsProviderAdapter(protocolAdaptor)
	attempt := &RelayAttempt{Info: info, ProtocolAdaptor: protocolAdaptor, ProviderAdapter: providerAdaptor}

	requestBody, err := responsesNativeRequestBody(respCtx.ResponsesBody, info.ResolvedModel, isStream)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadRequest, err)
	}
	attempt.RequestBody = requestBody
	attempt.NeedsTransform = providerAdaptor.NeedsTransform(info.Channel, reqCtx.Format)

	if attempt.NeedsTransform {
		convertedBody, err := convertRelayRequest(protocolAdaptor, requestBody, info)
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
	headers, err := providerAdaptor.GetAuthHeaders(apiKey, config, constant.RelayModeResponses, isStream)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadGateway, err)
	}
	attempt.Headers = headers
	attempt.URL = providerAdaptor.BuildURL(baseURL, constant.RelayModeResponses, info.ResolvedModel, isStream)

	return attempt, nil
}

func responsesNativeRequestBody(body []byte, resolvedModel string, stream bool) ([]byte, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if resolvedModel != "" {
		payload["model"] = resolvedModel
	}
	payload["stream"] = stream
	return json.Marshal(payload)
}

func isResponsesSSE(headers http.Header, body []byte) bool {
	contentType := strings.ToLower(headers.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, ":")
}

func responsesSSEToJSON(body []byte, fallbackModel string) ([]byte, error) {
	events := parseSSEEvents(body)
	var outputText strings.Builder
	modelName := fallbackModel
	toolItems := map[string]map[string]interface{}{}
	toolOrder := make([]string, 0)

	for _, event := range events {
		if event.Data == "" || event.Data == "[DONE]" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(event.Data), &payload); err != nil {
			continue
		}
		if responseValue, ok := payload["response"].(map[string]interface{}); ok {
			if model, _ := responseValue["model"].(string); model != "" {
				modelName = model
			}
			if event.Event == "response.completed" || responseValue["status"] == "completed" {
				return json.Marshal(responseValue)
			}
		}
		if event.Event == "response.output_item.added" || event.Event == "response.output_item.done" {
			if item, ok := payload["item"].(map[string]interface{}); ok {
				if itemType, _ := item["type"].(string); itemType == "function_call" {
					id, _ := item["id"].(string)
					if id == "" {
						id, _ = item["call_id"].(string)
					}
					if id != "" {
						if _, exists := toolItems[id]; !exists {
							toolOrder = append(toolOrder, id)
						}
						toolItems[id] = mergeResponsesFunctionCallItem(toolItems[id], item)
					}
				}
			}
		}
		if event.Event == "response.function_call_arguments.delta" {
			id, _ := payload["item_id"].(string)
			if id != "" {
				if _, exists := toolItems[id]; !exists {
					toolOrder = append(toolOrder, id)
					toolItems[id] = map[string]interface{}{"id": id, "type": "function_call", "call_id": id, "status": "in_progress"}
				}
				delta, _ := payload["delta"].(string)
				old, _ := toolItems[id]["arguments"].(string)
				toolItems[id]["arguments"] = old + delta
			}
		}
		if event.Event == "response.function_call_arguments.done" {
			id, _ := payload["item_id"].(string)
			if id != "" {
				if _, exists := toolItems[id]; !exists {
					toolOrder = append(toolOrder, id)
					toolItems[id] = map[string]interface{}{"id": id, "type": "function_call", "call_id": id}
				}
				if arguments, _ := payload["arguments"].(string); arguments != "" {
					toolItems[id]["arguments"] = arguments
				}
				toolItems[id]["status"] = "completed"
			}
		}
		if delta, _ := payload["delta"].(string); delta != "" && event.Event == "response.output_text.delta" {
			outputText.WriteString(delta)
		}
		if text, _ := payload["text"].(string); text != "" && event.Event == "response.output_text.done" && outputText.Len() == 0 {
			outputText.WriteString(text)
		}
	}

	if outputText.Len() == 0 && len(toolItems) == 0 {
		return nil, fmt.Errorf("无法从 Responses SSE 聚合出完整响应")
	}
	responseID := "resp_" + timeNowCompact()
	response := baseResponsesObject(responseID, "msg_"+responseID, modelName, "completed", outputText.String())
	if len(toolItems) > 0 {
		output := make([]interface{}, 0, 1+len(toolItems))
		if outputText.Len() > 0 {
			output = append(output, response["output"].([]interface{})...)
		}
		for _, id := range toolOrder {
			item := toolItems[id]
			item["status"] = "completed"
			output = append(output, item)
		}
		response["output"] = output
	}
	return json.Marshal(response)
}

type parsedSSEEvent struct {
	Event string
	Data  string
}

func parseSSEEvents(body []byte) []parsedSSEEvent {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	events := make([]parsedSSEEvent, 0)
	currentEvent := ""
	dataLines := make([]string, 0)

	flush := func() {
		if currentEvent == "" && len(dataLines) == 0 {
			return
		}
		events = append(events, parsedSSEEvent{Event: currentEvent, Data: strings.Join(dataLines, "\n")})
		currentEvent = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	return events
}

func copyNativeResponsesStream(c *gin.Context, body io.Reader) error {
	buffer := make([]byte, 4096)
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			if _, err := c.Writer.Write(buffer[:n]); err != nil {
				return err
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

func writeResponsesJSONAsStream(c *gin.Context, body []byte, fallbackModel string) error {
	outputText, modelName := responsesOutputTextAndModel(body, fallbackModel)
	responseID := "resp_" + timeNowCompact()
	messageID := "msg_" + timeNowCompact()
	emitter := newResponsesStreamEmitter(c.Writer, responseID, messageID, modelName)
	if err := emitter.start(); err != nil {
		return err
	}
	if outputText != "" {
		if err := emitter.delta(outputText); err != nil {
			return err
		}
	}
	return emitter.complete()
}

func responsesOutputTextAndModel(body []byte, fallbackModel string) (string, string) {
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return string(body), fallbackModel
	}
	modelName, _ := response["model"].(string)
	if modelName == "" {
		modelName = fallbackModel
	}
	if outputText, _ := response["output_text"].(string); outputText != "" {
		return outputText, modelName
	}
	return responsesOutputText(response), modelName
}

func responsesOutputText(response map[string]interface{}) string {
	outputs, ok := response["output"].([]interface{})
	if !ok {
		return ""
	}
	parts := make([]string, 0)
	for _, output := range outputs {
		outputMap, ok := output.(map[string]interface{})
		if !ok {
			continue
		}
		contents, ok := outputMap["content"].([]interface{})
		if !ok {
			continue
		}
		for _, content := range contents {
			contentMap, ok := content.(map[string]interface{})
			if !ok {
				continue
			}
			if text, _ := contentMap["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "")
}

func responsesUpstreamErrorMessage(protocolAdaptor adaptor.Adaptor, respBody []byte, statusCode int) string {
	if statusCode == http.StatusRequestEntityTooLarge {
		return "请求体过大，上游网关拒绝处理，请减少上下文或输出长度后重试"
	}
	lastErrMsg := ""
	if protocolAdaptor != nil {
		lastErrMsg = protocolAdaptor.ErrorMessage(respBody)
	}
	if lastErrMsg == "" {
		lastErrMsg = string(respBody)
	}
	return lastErrMsg
}

func writeFinalResponsesError(c *gin.Context, lastErr error, lastErrMsg string, attemptedUpstream bool, lastStatusCode int) {
	if lastStatusCode == http.StatusRequestEntityTooLarge {
		writeRelayError(c, http.StatusRequestEntityTooLarge, "请求体过大，上游网关拒绝处理", "request_too_large", lastErrMsg)
		return
	}
	if attemptedUpstream && lastStatusCode >= http.StatusBadRequest && lastStatusCode < http.StatusInternalServerError {
		writeRelayError(c, lastStatusCode, "上游渠道请求失败", "upstream_error", lastErrMsg)
		return
	}
	writeFinalRelayError(c, lastErr, lastErrMsg, attemptedUpstream)
}

func timeNowCompact() string {
	return strings.ReplaceAll(strings.ReplaceAll(time.Now().Format("20060102150405.000000000"), ".", ""), "-", "")
}
