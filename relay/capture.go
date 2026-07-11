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

// captureUpstreamResponse 采集上游响应（在 DoRequest 后、body 读取时采集）
func captureUpstreamResponse(resp *http.Response, cfg *model.LoggingConfig) (map[string]string, io.ReadCloser) {
	if !cfg.RecordUpstreamResp {
		return nil, resp.Body
	}
	headers := sanitizeHeaders(resp.Header, cfg.SanitizedHeaderKeys)
	// 使用 TeeReader 采集 body 同时不影响后续读取
	var buf bytes.Buffer
	teeBody := io.TeeReader(resp.Body, &buf)
	return headers, &teeReadCloser{Reader: teeBody, buf: &buf, closer: resp.Body}
}

// teeReadCloser 包装 TeeReader，读完后将采集的数据保存到 FullLogCapture
type teeReadCloser struct {
	io.Reader
	buf    *bytes.Buffer
	closer io.Closer
}

func (t *teeReadCloser) Close() error {
	return t.closer.Close()
}

// captureClientResponse 采集返回客户端的响应（在写出前调用）
func captureClientResponse(c *gin.Context, body []byte, cfg *model.LoggingConfig) map[string]string {
	if !cfg.RecordClientResp {
		return nil
	}
	return sanitizeHeaders(c.Writer.Header(), cfg.SanitizedHeaderKeys)
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
		w.capture.ClientRespBody = append(w.capture.ClientRespBody, data[:n]...)
	}
	w.syncMeta()
	return n, err
}

func (w *captureResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.syncMeta()
}
