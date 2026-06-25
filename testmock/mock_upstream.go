//go:build ignore

// 一个最小的模拟 OpenAI 上游，用于端到端冒烟测试。
// 运行：go run testmock/mock_upstream.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		stream, _ := req["stream"].(bool)
		model, _ := req["model"].(string)
		fmt.Printf("[mock] received model=%s stream=%v auth=%s\n", model, stream, r.Header.Get("Authorization"))

		// 测试故障转移：模型名以 "fail-" 开头时返回 503
		if len(model) >= 5 && model[:5] == "fail-" {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":{"message":"mock upstream unavailable","type":"service_unavailable"}}`)
			return
		}

		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			chunks := []string{"Hello", ", ", "world", "!"}
			for _, ck := range chunks {
				payload := fmt.Sprintf(`{"id":"x","object":"chat.completion.chunk","model":%q,"choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`, model, ck)
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
			fmt.Fprintf(w, "data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":4,\"total_tokens\":9}}\n\n", model)
			flusher.Flush()
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := fmt.Sprintf(`{"id":"chatcmpl-mock","object":"chat.completion","created":%d,"model":%q,"choices":[{"index":0,"message":{"role":"assistant","content":"Hello from mock"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`, time.Now().Unix(), model)
		fmt.Fprint(w, resp)
	})

	http.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		stream, _ := req["stream"].(bool)
		model, _ := req["model"].(string)
		fmt.Printf("[mock-anthropic] model=%s stream=%v x-api-key=%s\n", model, stream, r.Header.Get("x-api-key"))

		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			send := func(event, data string) {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
				flusher.Flush()
			}
			send("message_start", fmt.Sprintf(`{"type":"message_start","message":{"id":"msg_mock","type":"message","role":"assistant","model":%q,"content":[],"usage":{"input_tokens":6,"output_tokens":0}}}`, model))
			send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			for _, ck := range []string{"Hello", " from", " Claude"} {
				send("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, ck))
				time.Sleep(40 * time.Millisecond)
			}
			send("content_block_stop", `{"type":"content_block_stop","index":0}`)
			send("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`)
			send("message_stop", `{"type":"message_stop"}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"msg_mock","type":"message","role":"assistant","model":%q,"content":[{"type":"text","text":"Hello from Claude mock"}],"stop_reason":"end_turn","usage":{"input_tokens":6,"output_tokens":4}}`, model)
	})

	// OpenAI 风格模型列表
	http.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"id":"gpt-4o","object":"model"},{"id":"gpt-4o-mini","object":"model"},{"id":"o3-mini","object":"model"}]}`)
	})

	fmt.Println("[mock] listening on :9999")
	_ = http.ListenAndServe(":9999", nil)
}
