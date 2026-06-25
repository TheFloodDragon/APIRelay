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

	fmt.Println("[mock] listening on :9999")
	_ = http.ListenAndServe(":9999", nil)
}
