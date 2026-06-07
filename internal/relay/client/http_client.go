package client

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

// HTTPClient 统一封装 relay 出口 HTTP 传输。
type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient() *HTTPClient {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &HTTPClient{
		client: &http.Client{Transport: transport},
	}
}

func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, headers http.Header, body []byte, timeout time.Duration) (int, []byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	copyHeaders(req.Header, headers)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func (c *HTTPClient) DoStream(ctx context.Context, method, url string, headers http.Header, body []byte, timeout time.Duration) (*http.Response, error) {
	// 流式响应不能把渠道 timeout 作为整个响应体的 deadline，否则长输出会在
	// response.completed / [DONE] 前被 context 截断。连接阶段已有 Dial/TLS 超时，
	// 客户端断开时 ctx 仍会取消请求。
	_ = timeout

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, headers)

	return c.client.Do(req)
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
