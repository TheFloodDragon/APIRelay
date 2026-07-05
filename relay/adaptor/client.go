package adaptor

import (
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	defaultClient *http.Client
	clientOnce    sync.Once
)

const (
	upstreamDialTimeout           = 30 * time.Second
	upstreamTLSHandshakeTimeout   = 10 * time.Second
	upstreamResponseHeaderTimeout = 120 * time.Second
	upstreamIdleConnTimeout       = 90 * time.Second
)

// HTTPClient 返回共享的 http.Client。
func HTTPClient() *http.Client {
	clientOnce.Do(func() {
		defaultClient = &http.Client{
			Timeout: 0, // 流式不设置整请求超时，由 relay context 控制
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   upstreamDialTimeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       upstreamIdleConnTimeout,
				TLSHandshakeTimeout:   upstreamTLSHandshakeTimeout,
				ResponseHeaderTimeout: upstreamResponseHeaderTimeout,
				ExpectContinueTimeout: 1 * time.Second,
			},
		}
	})
	return defaultClient
}
