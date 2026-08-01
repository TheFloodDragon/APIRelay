package adaptor

import (
	"bufio"
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

	// gzip.NewReader / zlib.NewReader 都会预读并消费头部字节，失败后无法回退。
	// 若此时直接返回，resp.Body 的读取位置已前移，调用方会拿到缺少前几字节的残缺流，
	// 表现为「上游确实返回了 JSON，却报 convert response 失败」且被判为不可重试。
	//
	// 因此用 bufio.Reader 包裹并先 Peek 校验魔数：Peek 不消费字节，
	// 无论是否解压，后续读取都能从完整的流开始。
	buffered := bufio.NewReader(resp.Body)
	// 解压器构造失败时交还这个可重放的流，让上层按未压缩内容处理。
	passthrough := &wrappedReadCloser{r: io.NopCloser(buffered), underlying: resp.Body}

	var newBody io.ReadCloser
	switch enc {
	case "gzip":
		// gzip 魔数 0x1f 0x8b。
		if head, err := buffered.Peek(2); err != nil || head[0] != 0x1f || head[1] != 0x8b {
			resp.Body = passthrough
			return
		}
		zr, err := gzip.NewReader(buffered)
		if err != nil {
			resp.Body = passthrough
			return
		}
		newBody = &wrappedReadCloser{r: zr, underlying: resp.Body}
	case "deflate":
		// HTTP 的 deflate 绝大多数为 zlib 封装（RFC 1950）：低 4 位为压缩方法 8，
		// 且 (CMF<<8 | FLG) 必须能被 31 整除。
		head, err := buffered.Peek(2)
		if err != nil || head[0]&0x0f != 8 || (uint16(head[0])<<8|uint16(head[1]))%31 != 0 {
			resp.Body = passthrough
			return
		}
		zr, err := zlib.NewReader(buffered)
		if err != nil {
			resp.Body = passthrough
			return
		}
		newBody = &wrappedReadCloser{r: zr, underlying: resp.Body}
	default:
		// 未知编码（br / zstd 等）：不解压，但仍需交还完整可读的流。
		resp.Body = passthrough
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
