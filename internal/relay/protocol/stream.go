package protocol

import (
	"bytes"
	"encoding/json"
)

// AnthropicStreamEncoder 将通用流事件编码为 Anthropic Messages SSE，并维护 message/content block 状态。
type AnthropicStreamEncoder struct {
	started        bool
	contentStarted bool
	stopped        bool
	id             string
	model          string
}

func NewAnthropicStreamEncoder() *AnthropicStreamEncoder {
	return &AnthropicStreamEncoder{}
}

func (e *AnthropicStreamEncoder) Encode(event StreamEvent) ([]byte, error) {
	if e.stopped {
		return nil, nil
	}
	var out bytes.Buffer

	if event.ID != "" {
		e.id = event.ID
	}
	if event.Model != "" {
		e.model = event.Model
	}

	if event.Start || event.Content != "" || event.FinishReason != "" || event.Done {
		if err := e.ensureStarted(&out); err != nil {
			return nil, err
		}
	}

	if event.Content != "" {
		if err := e.ensureContentStarted(&out, event.Index); err != nil {
			return nil, err
		}
		if err := writeAnthropicSSE(&out, "content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": event.Index,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": event.Content,
			},
		}); err != nil {
			return nil, err
		}
	}

	if event.FinishReason != "" || event.Done {
		if err := e.stop(&out, event.FinishReason); err != nil {
			return nil, err
		}
	}

	return out.Bytes(), nil
}

func (e *AnthropicStreamEncoder) ensureStarted(out *bytes.Buffer) error {
	if e.started {
		return nil
	}
	if e.id == "" {
		e.id = generatedID("msg")
	}
	e.started = true
	return writeAnthropicSSE(out, "message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            e.id,
			"type":          "message",
			"role":          "assistant",
			"model":         e.model,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	})
}

func (e *AnthropicStreamEncoder) ensureContentStarted(out *bytes.Buffer, index int) error {
	if e.contentStarted {
		return nil
	}
	e.contentStarted = true
	return writeAnthropicSSE(out, "content_block_start", map[string]interface{}{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]interface{}{
			"type": "text",
			"text": "",
		},
	})
}

func (e *AnthropicStreamEncoder) stop(out *bytes.Buffer, reason string) error {
	if e.stopped {
		return nil
	}
	if e.contentStarted {
		if err := writeAnthropicSSE(out, "content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}); err != nil {
			return err
		}
	}
	if err := writeAnthropicSSE(out, "message_delta", map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason":   finishReasonToAnthropic(reason),
			"stop_sequence": nil,
		},
		"usage": map[string]interface{}{
			"output_tokens": 0,
		},
	}); err != nil {
		return err
	}
	if err := writeAnthropicSSE(out, "message_stop", map[string]interface{}{
		"type": "message_stop",
	}); err != nil {
		return err
	}
	e.stopped = true
	return nil
}

func writeAnthropicSSE(out *bytes.Buffer, eventName string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	out.WriteString("event: ")
	out.WriteString(eventName)
	out.WriteByte('\n')
	out.WriteString("data: ")
	out.Write(payloadBytes)
	out.WriteString("\n\n")
	return nil
}
