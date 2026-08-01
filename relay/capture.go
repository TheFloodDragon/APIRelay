package relay

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/relaycommon"
	"github.com/gin-gonic/gin"
)

// sanitizeHeaders 脱敏指定的 header 键
func sanitizeHeaders(headers http.Header, sanitizedKeys []string) map[string]string {
	result := make(map[string]string)
	keySet := make(map[string]struct{})
	for _, k := range sanitizedKeys {
		keySet[strings.ToLower(k)] = struct{}{}
	}
	for k, vals := range headers {
		lowerKey := strings.ToLower(k)
		if _, sensitive := keySet[lowerKey]; sensitive {
			result[k] = "[REDACTED]"
		} else {
			result[k] = strings.Join(vals, ", ")
		}
	}
	return result
}

// captureClientRequest 采集客户端请求（在 handle 入口调用）
func captureClientRequest(c *gin.Context, body []byte, cfg *model.LoggingConfig) map[string]string {
	if !cfg.RecordClientRequest {
		return nil
	}
	return sanitizeHeaders(c.Request.Header, cfg.SanitizedHeaderKeys)
}

// captureUpstreamRequest 采集上游请求（在 DoRequest 前调用）
func captureUpstreamRequest(req *http.Request, body []byte, cfg *model.LoggingConfig) (map[string]string, []byte) {
	if !cfg.RecordUpstreamRequest {
		return nil, nil
	}
	headers := sanitizeHeaders(req.Header, cfg.SanitizedHeaderKeys)
	return headers, body
}

// maxCapturedBodyBytes 单个采集字段的字节上限。
//
// 开启完整日志时，流式请求会同时持有多份正文副本（上游流缓冲 + 客户端响应缓冲）。
// 没有上限时，一次长会话流式响应就能让单请求常驻内存达到正文的数倍。
// 超限后停止累积并追加截断标记，保留可诊断的开头部分。
const maxCapturedBodyBytes = 1 * 1024 * 1024

var capturedTruncationMarker = []byte("\n...[truncated by apirelay: capture size limit reached]")

// boundedBuffer 是带字节上限的采集缓冲：超限后丢弃后续写入，但不报错（采集不应影响转发）。
type boundedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	if limit <= 0 {
		limit = maxCapturedBodyBytes
	}
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	// 始终返回 len(p)：TeeReader 会把短写当作错误并中断上游读取。
	if remaining := b.limit - b.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:remaining])
			b.truncated = true
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

// Bytes 返回采集内容；被截断时追加显式标记，避免误读为完整正文。
func (b *boundedBuffer) Bytes() []byte {
	out := make([]byte, 0, b.buf.Len()+len(capturedTruncationMarker))
	out = append(out, b.buf.Bytes()...)
	if b.truncated {
		out = append(out, capturedTruncationMarker...)
	}
	return out
}

// appendCapped 在总长不超过 limit 的前提下追加数据，超限时只追加一次截断标记。
func appendCapped(dst, src []byte, limit int) []byte {
	if limit <= 0 {
		limit = maxCapturedBodyBytes
	}
	if len(dst) >= limit {
		if !bytes.HasSuffix(dst, capturedTruncationMarker) {
			dst = append(dst, capturedTruncationMarker...)
		}
		return dst
	}
	if remaining := limit - len(dst); len(src) > remaining {
		dst = append(dst, src[:remaining]...)
		return append(dst, capturedTruncationMarker...)
	}
	return append(dst, src...)
}

// teeReadCloser 包装 TeeReader，读取时同步把数据写入采集缓冲。
type teeReadCloser struct {
	io.Reader
	closer io.Closer
}

func (t *teeReadCloser) Close() error {
	return t.closer.Close()
}

// shouldCaptureFullLog 判断是否需要采集完整日志
func shouldCaptureFullLog() bool {
	cfg := model.GetLoggingConfig()
	return cfg != nil && cfg.Enabled
}

// initFullLogCapture 初始化 FullLogCapture（在 handle 入口调用）
func initFullLogCapture(c *gin.Context, body []byte) *relaycommon.FullLogCapture {
	if !shouldCaptureFullLog() {
		return nil
	}
	cfg := model.GetLoggingConfig()
	capture := &relaycommon.FullLogCapture{}
	if cfg.RecordClientRequest {
		capture.ClientMethod = c.Request.Method
		capture.ClientPath = c.Request.URL.Path
		capture.ClientQuery = c.Request.URL.RawQuery
		capture.ClientBody = body
		capture.ClientHeaders = captureClientRequest(c, body, cfg)
	}
	return capture
}

// captureResponseWriter 包装 gin.ResponseWriter，在不改变写出和 Flush 语义的前提下采集客户端响应。
type captureResponseWriter struct {
	gin.ResponseWriter
	capture *relaycommon.FullLogCapture
	cfg     *model.LoggingConfig
}

func (w *captureResponseWriter) syncMeta() {
	if w.capture == nil || w.cfg == nil || !w.cfg.RecordClientResp {
		return
	}
	w.capture.ClientRespStatus = w.ResponseWriter.Status()
	w.capture.ClientRespHeaders = sanitizeHeaders(w.ResponseWriter.Header(), w.cfg.SanitizedHeaderKeys)
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 && w.capture != nil && w.cfg != nil && w.cfg.RecordClientResp {
		// 有上限地累积：流式响应逐块写出，无上限 append 会随会话长度线性增长。
		w.capture.ClientRespBody = appendCapped(w.capture.ClientRespBody, data[:n], maxCapturedBodyBytes)
	}
	w.syncMeta()
	return n, err
}

func (w *captureResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.syncMeta()
}
