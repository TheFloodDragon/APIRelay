package response

import (
	"io"
	"net/http"
	"strconv"
)

type BodyTransformer func([]byte) ([]byte, error)

type Processor struct{}

func NewProcessor() *Processor {
	return &Processor{}
}

func (p *Processor) ReadAndTransform(resp *http.Response, transform BodyTransformer) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if transform == nil {
		return body, nil
	}
	return transform(body)
}

func (p *Processor) WriteBody(w http.ResponseWriter, statusCode int, headers http.Header, contentType string, body []byte) {
	CopyHeaders(w.Header(), headers)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func WriteStreamHeaders(w http.ResponseWriter, statusCode int, headers http.Header) {
	CopyHeaders(w.Header(), headers)
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(statusCode)
}

func StripHopByHopHeaders(headers http.Header) http.Header {
	cleaned := headers.Clone()
	for _, header := range hopByHopHeaders {
		cleaned.Del(header)
	}
	for _, header := range headers.Values("Connection") {
		for _, token := range splitHeaderTokens(header) {
			cleaned.Del(token)
		}
	}
	return cleaned
}

func CopyHeaders(dst, src http.Header) {
	for key, values := range StripHopByHopHeaders(src) {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func splitHeaderTokens(value string) []string {
	tokens := make([]string, 0)
	start := 0
	for i := 0; i <= len(value); i++ {
		if i != len(value) && value[i] != ',' {
			continue
		}
		token := trimHeaderToken(value[start:i])
		if token != "" {
			tokens = append(tokens, token)
		}
		start = i + 1
	}
	return tokens
}

func trimHeaderToken(value string) string {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}
