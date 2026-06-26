package adaptor

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
)

// DecompressResponse 根据 Content-Encoding 解压上游响应体，对齐 sub2api 的
// decompressResponseBody，处理 gzip / deflate（zlib 与 raw deflate 均兼容）。
//
// 说明：Go 的 http.Transport 在调用方未显式设置 Accept-Encoding 时，会自动
// 请求并透明解压 gzip（此时 Content-Encoding 已被移除）。本函数作为兜底，
// 处理上游强制返回压缩、或使用 deflate 等非自动解压编码的情况。
//
// 解压后会：
//   - 用解压流替换 resp.Body（惰性解压，保持流式语义）
//   - 删除 Content-Encoding / Content-Length 头（内容已变化）
//
// 若没有压缩或编码未知（如 br / zstd），则原样返回，交由上层处理。
func DecompressResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	enc := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if enc == "" || enc == "identity" {
		return
	}

	var newBody io.ReadCloser
	switch enc {
	case "gzip":
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return
		}
		newBody = &wrappedReadCloser{r: zr, underlying: resp.Body}
	case "deflate":
		// HTTP 的 deflate 绝大多数为 zlib 封装（RFC 1950）。zlib.NewReader 会预读
		// 头部，若失败说明不是 zlib 流，此时不冒险回退（避免读到错位字节导致乱码），
		// 直接放弃解压交由上层处理。
		zr, err := zlib.NewReader(resp.Body)
		if err != nil {
			return
		}
		newBody = &wrappedReadCloser{r: zr, underlying: resp.Body}
	default:
		// 未知编码（br / zstd 等）：不处理
		return
	}

	resp.Body = newBody
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")
	resp.ContentLength = -1
	resp.Uncompressed = true
}

// wrappedReadCloser 同时关闭解压器和底层连接，避免连接泄漏。
type wrappedReadCloser struct {
	r          io.ReadCloser
	underlying io.ReadCloser
}

func (w *wrappedReadCloser) Read(p []byte) (int, error) { return w.r.Read(p) }

func (w *wrappedReadCloser) Close() error {
	err := w.r.Close()
	if cerr := w.underlying.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}
