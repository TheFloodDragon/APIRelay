package adaptor

import (
	"net/http"
	"sync"
	"time"
)

var (
	defaultClient *http.Client
	clientOnce    sync.Once
)

// HTTPClient 返回共享的 http.Client。
func HTTPClient() *http.Client {
	clientOnce.Do(func() {
		defaultClient = &http.Client{
			Timeout: 0, // 流式不超时，由 context 控制
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return defaultClient
}
