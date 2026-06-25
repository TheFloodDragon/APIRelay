package adaptor

import (
	"bufio"
	"strings"
)

// ScanSSE 读取 SSE 流，对每条 data 行回调 fn(data)。
// 返回 fn 的 error 时中止。自动跳过空行、注释、event: 行。
// 当 data == "[DONE]" 时停止。
func ScanSSE(r *bufio.Scanner, fn func(data string) error) error {
	r.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for r.Scan() {
		line := strings.TrimRight(r.Text(), "\r")
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		if err := fn(data); err != nil {
			return err
		}
	}
	return r.Err()
}
