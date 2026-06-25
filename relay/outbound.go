package relay

import (
	"net/http"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/relay/apicompat"

	"github.com/gin-gonic/gin"
)

// Outbound 负责按【对外协议】序列化错误、非流式响应与流式响应。
// 这是协议解耦的出站侧：与上游 Adaptor（入站侧）对称。
type Outbound interface {
	// WriteError 输出该协议的错误响应。
	WriteError(c *gin.Context, status int, msg string)
	// WriteResponse 输出该协议的非流式响应。
	WriteResponse(c *gin.Context, r *dto.UnifiedResponse, model string)
	// NewStream 创建该协议的流式写出器。
	NewStream(c *gin.Context, requestID, model string) StreamWriter
}

// StreamWriter 逐增量写出对外协议的 SSE。
type StreamWriter interface {
	WriteChunk(c *gin.Context, chunk *dto.UnifiedStreamChunk) error
	Finish(c *gin.Context) error
}

// GetOutbound 按对外协议返回出站序列化器。
func GetOutbound(ep constant.EndpointType) Outbound {
	switch ep {
	case constant.EndpointAnthropic:
		return anthropicOutbound{}
	case constant.EndpointResponses:
		return responsesOutbound{}
	default:
		return openaiOutbound{}
	}
}

// ---------------------------------------------------------------------------
// OpenAI 出站
// ---------------------------------------------------------------------------

type openaiOutbound struct{}

func (openaiOutbound) WriteError(c *gin.Context, status int, msg string) {
	c.JSON(status, dto.OpenAIErrorResponse{Error: dto.OpenAIError{Message: msg, Type: errTypeForStatus(status)}})
}

func (openaiOutbound) WriteResponse(c *gin.Context, r *dto.UnifiedResponse, model string) {
	c.JSON(http.StatusOK, apicompat.IRToOpenAIResponse(r, model))
}

func (openaiOutbound) NewStream(c *gin.Context, requestID, model string) StreamWriter {
	setSSEHeaders(c)
	return &openaiStreamWriter{state: apicompat.NewOpenAIStreamState(requestID, model)}
}

type openaiStreamWriter struct {
	state *apicompat.OpenAIStreamState
}

func (w *openaiStreamWriter) WriteChunk(c *gin.Context, chunk *dto.UnifiedStreamChunk) error {
	data := w.state.Chunk(chunk)
	if data == nil {
		return nil
	}
	return writeSSE(c, "", data)
}

func (w *openaiStreamWriter) Finish(c *gin.Context) error {
	return writeSSERaw(c, "data: [DONE]\n\n")
}

// ---------------------------------------------------------------------------
// Anthropic 出站
// ---------------------------------------------------------------------------

type anthropicOutbound struct{}

func (anthropicOutbound) WriteError(c *gin.Context, status int, msg string) {
	c.JSON(status, dto.AnthropicErrorResponse{
		Type:  "error",
		Error: dto.AnthropicError{Type: errTypeForStatus(status), Message: msg},
	})
}

func (anthropicOutbound) WriteResponse(c *gin.Context, r *dto.UnifiedResponse, model string) {
	c.JSON(http.StatusOK, apicompat.IRToAnthropicResponse(r, model))
}

func (anthropicOutbound) NewStream(c *gin.Context, requestID, model string) StreamWriter {
	setSSEHeaders(c)
	return &anthropicStreamWriter{state: apicompat.NewAnthropicStreamState(requestID, model)}
}

type anthropicStreamWriter struct {
	state *apicompat.AnthropicStreamState
}

func (w *anthropicStreamWriter) WriteChunk(c *gin.Context, chunk *dto.UnifiedStreamChunk) error {
	for _, ev := range w.state.Delta(chunk) {
		if err := writeSSE(c, ev.Event, ev.Data); err != nil {
			return err
		}
	}
	return nil
}

func (w *anthropicStreamWriter) Finish(c *gin.Context) error {
	for _, ev := range w.state.End() {
		if err := writeSSE(c, ev.Event, ev.Data); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Responses 出站
// ---------------------------------------------------------------------------

type responsesOutbound struct{}

func (responsesOutbound) WriteError(c *gin.Context, status int, msg string) {
	c.JSON(status, dto.ResponsesErrorResponse{Error: dto.ResponsesError{Message: msg, Type: errTypeForStatus(status)}})
}

func (responsesOutbound) WriteResponse(c *gin.Context, r *dto.UnifiedResponse, model string) {
	c.JSON(http.StatusOK, apicompat.IRToResponsesResponse(r, model))
}

func (responsesOutbound) NewStream(c *gin.Context, requestID, model string) StreamWriter {
	setSSEHeaders(c)
	return &responsesStreamWriter{state: apicompat.NewResponsesStreamState(requestID, model)}
}

type responsesStreamWriter struct {
	state *apicompat.ResponsesStreamState
}

func (w *responsesStreamWriter) WriteChunk(c *gin.Context, chunk *dto.UnifiedStreamChunk) error {
	for _, ev := range w.state.Delta(chunk) {
		if err := writeSSE(c, ev.Event, ev.Data); err != nil {
			return err
		}
	}
	return nil
}

func (w *responsesStreamWriter) Finish(c *gin.Context) error {
	for _, ev := range w.state.End() {
		if err := writeSSE(c, ev.Event, ev.Data); err != nil {
			return err
		}
	}
	return writeSSERaw(c, "data: [DONE]\n\n")
}

// ---------------------------------------------------------------------------
// SSE 写出辅助
// ---------------------------------------------------------------------------

func writeSSE(c *gin.Context, event string, data []byte) error {
	if event != "" {
		if _, err := c.Writer.Write([]byte("event: " + event + "\n")); err != nil {
			return err
		}
	}
	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := c.Writer.Write(data); err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return err
	}
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func writeSSERaw(c *gin.Context, s string) error {
	if _, err := c.Writer.Write([]byte(s)); err != nil {
		return err
	}
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func errTypeForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "authentication_error"
	case status == http.StatusForbidden:
		return "permission_error"
	case status == http.StatusTooManyRequests:
		return "rate_limit_error"
	case status >= 500:
		return "service_unavailable"
	default:
		return "invalid_request_error"
	}
}
