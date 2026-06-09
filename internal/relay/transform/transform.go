package transform

import (
	"fmt"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
)

// Request 在调用方协议与上游协议之间转换非流式/流式请求体。
func Request(body []byte, upstreamType constant.APIType, mode constant.RelayMode, callerFormat constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	switch upstreamType {
	case constant.APITypeOpenAI:
		return RequestToOpenAI(body, mode, callerFormat, meta)
	case constant.APITypeAnthropic:
		return RequestToAnthropic(body, mode, callerFormat, meta)
	case constant.APITypeGemini:
		return RequestToGemini(body, mode, callerFormat, meta)
	default:
		return body, nil
	}
}

// Response 在上游协议与调用方协议之间转换非流式响应体。
func Response(body []byte, upstreamType constant.APIType, mode constant.RelayMode, callerFormat constant.RelayFormat) ([]byte, error) {
	switch upstreamType {
	case constant.APITypeOpenAI:
		return ResponseFromOpenAI(body, mode, callerFormat)
	case constant.APITypeAnthropic:
		return ResponseFromAnthropic(body, mode, callerFormat)
	case constant.APITypeGemini:
		return ResponseFromGemini(body, mode, callerFormat)
	default:
		return body, nil
	}
}

func RequestToOpenAI(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	if mode == constant.RelayModeCountTokens {
		return nil, fmt.Errorf("%s is not supported for openai channels yet", mode)
	}
	switch callerFormat {
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		return body, nil
	case constant.RelayFormatAnthropic:
		if !mode.IsChatLike() {
			return nil, fmt.Errorf("%s is not supported for anthropic caller format on openai channels yet", mode)
		}
		chatReq, err := protocol.AnthropicMessagesRequestToProtocol(body)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToOpenAIChatRequest(chatReq)
	case constant.RelayFormatGemini:
		if !mode.IsChatLike() {
			return nil, fmt.Errorf("%s is not supported for gemini caller format on openai channels yet", mode)
		}
		chatReq, err := protocol.GeminiGenerateContentRequestToProtocol(body, meta.Model, meta.Stream)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToOpenAIChatRequest(chatReq)
	default:
		return body, nil
	}
}

func ResponseFromOpenAI(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() {
		return body, nil
	}
	switch callerFormat {
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		return body, nil
	case constant.RelayFormatAnthropic:
		chatResp, err := protocol.OpenAIChatResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToAnthropicMessagesResponse(chatResp)
	case constant.RelayFormatGemini:
		chatResp, err := protocol.OpenAIChatResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToGeminiGenerateContentResponse(chatResp)
	default:
		return body, nil
	}
}

func RequestToAnthropic(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	if mode == constant.RelayModeCountTokens {
		return nil, fmt.Errorf("%s is not supported for anthropic channels yet", mode)
	}
	if !mode.IsChatLike() {
		return nil, fmt.Errorf("%s is not supported for anthropic channels yet", mode)
	}
	switch callerFormat {
	case constant.RelayFormatAnthropic:
		return body, nil
	case constant.RelayFormatOpenAI:
		chatReq, err := protocol.OpenAIChatRequestToProtocol(body)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToAnthropicMessagesRequest(chatReq)
	case constant.RelayFormatGemini:
		chatReq, err := protocol.GeminiGenerateContentRequestToProtocol(body, meta.Model, meta.Stream)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToAnthropicMessagesRequest(chatReq)
	case constant.RelayFormatOpenAIResponses:
		return nil, fmt.Errorf("responses is not supported for anthropic channels yet")
	default:
		return nil, fmt.Errorf("%s caller format is not supported for anthropic channels yet", callerFormat)
	}
}

func ResponseFromAnthropic(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() {
		return body, nil
	}
	switch callerFormat {
	case constant.RelayFormatAnthropic:
		return body, nil
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		chatResp, err := protocol.AnthropicMessagesResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToOpenAIChatResponse(chatResp)
	case constant.RelayFormatGemini:
		chatResp, err := protocol.AnthropicMessagesResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToGeminiGenerateContentResponse(chatResp)
	default:
		return body, nil
	}
}

func RequestToGemini(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	if mode == constant.RelayModeCountTokens {
		if callerFormat == constant.RelayFormatGemini {
			return body, nil
		}
		return nil, fmt.Errorf("%s caller format is not supported for gemini countTokens yet", callerFormat)
	}
	if !mode.IsChatLike() {
		return nil, fmt.Errorf("%s is not supported for gemini channels yet", mode)
	}
	switch callerFormat {
	case constant.RelayFormatGemini:
		return body, nil
	case constant.RelayFormatOpenAI:
		chatReq, err := protocol.OpenAIChatRequestToProtocol(body)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToGeminiGenerateContentRequest(chatReq)
	case constant.RelayFormatAnthropic:
		chatReq, err := protocol.AnthropicMessagesRequestToProtocol(body)
		if err != nil {
			return nil, err
		}
		applyRequestMeta(chatReq, meta)
		return protocol.ProtocolToGeminiGenerateContentRequest(chatReq)
	case constant.RelayFormatOpenAIResponses:
		return nil, fmt.Errorf("responses is not supported for gemini channels yet")
	default:
		return nil, fmt.Errorf("%s caller format is not supported for gemini channels yet", callerFormat)
	}
}

func ResponseFromGemini(body []byte, mode constant.RelayMode, callerFormat constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() {
		return body, nil
	}
	switch callerFormat {
	case constant.RelayFormatGemini:
		return body, nil
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		chatResp, err := protocol.GeminiGenerateContentResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToOpenAIChatResponse(chatResp)
	case constant.RelayFormatAnthropic:
		chatResp, err := protocol.GeminiGenerateContentResponseToProtocol(body)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToAnthropicMessagesResponse(chatResp)
	default:
		return body, nil
	}
}

func applyRequestMeta(req *protocol.ChatRequest, meta protocol.RequestMeta) {
	if req == nil {
		return
	}
	if meta.Model != "" {
		req.Model = meta.Model
	}
	if meta.Stream {
		req.Stream = true
	}
}
