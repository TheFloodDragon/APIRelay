package adaptor

import (
	"bufio"
	"io"
	"strings"
)

// 流式扫描缓冲区上限，对齐 sub2api 的大缓冲，避免超长单行（如大段工具调用 JSON）截断。
const (
	sseInitialBuffer = 64 * 1024
	sseMaxBuffer     = 16 * 1024 * 1024
)

// newSSEScanner 构造带大缓冲的逐行扫描器。
func newSSEScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, sseInitialBuffer), sseMaxBuffer)
	return sc
}

// StreamRawLines 逐行读取 SSE 流，对【每一行】原样回调 onLine（包括 event: 行、
// 空行、注释行、data: 行、[DONE] 行），行尾的 \r 会被去除但不做其他改写。
//
// 这是真正的逐行原样转发：调用方负责决定如何处理每一行。与 ScanSSE 不同，
// 它不会丢弃 event:/空行/注释行——这对 Anthropic、Responses 等强依赖 event 行的
// 协议至关重要。
//
// 回调返回 error 时中止并返回该 error。
func StreamRawLines(r io.Reader, onLine func(line string) error) error {
	sc := newSSEScanner(r)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if err := onLine(line); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ScanSSE 读取 SSE 流，对每条 data 行回调 fn(data)。
// 返回 fn 的 error 时中止。自动跳过空行、注释、event: 行。
// 当 data == "[DONE]" 时停止。
//
// 注意：ScanSSE 仅适用于【纯解析】场景（如跨协议时提取内容/usage）。
// 需要原样透传时请使用 StreamRawLines，否则会丢失 event 行导致客户端无法解析。
func ScanSSE(r *bufio.Scanner, fn func(data string) error) error {
	r.Buffer(make([]byte, 0, sseInitialBuffer), sseMaxBuffer)
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

// ParseSSEData 从一行 SSE 文本中提取 data 部分。
// 返回 (data, true) 表示这是一条 data 行；否则 ok 为 false。
func ParseSSEData(line string) (data string, ok bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}
