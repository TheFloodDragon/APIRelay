package adaptor

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
	"testing"
)

func responseWithEncoding(enc string, body []byte) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	if enc != "" {
		resp.Header.Set("Content-Encoding", enc)
	}
	return resp
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

func TestDecompressResponseGzip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	resp := responseWithEncoding("gzip", buf.Bytes())
	DecompressResponse(resp)

	if got := readAll(t, resp); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
	if resp.Header.Get("Content-Encoding") != "" {
		t.Fatal("Content-Encoding should be removed after decompression")
	}
	if !resp.Uncompressed {
		t.Fatal("Uncompressed should be true")
	}
}

func TestDecompressResponseDeflate(t *testing.T) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write([]byte("hello deflate")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	resp := responseWithEncoding("deflate", buf.Bytes())
	DecompressResponse(resp)

	if got := readAll(t, resp); got != "hello deflate" {
		t.Fatalf("body = %q", got)
	}
}

// 上游声明了压缩编码但正文其实没压缩（或压缩非法）时，必须交还完整可读的原始字节。
// 此前实现会让 gzip.NewReader 吃掉头部后直接返回，调用方拿到残缺流，
// 表现为「上游返回了 JSON 却报 convert response 失败」且不可重试。
func TestDecompressResponseKeepsBodyIntactOnInvalidGzip(t *testing.T) {
	payload := `{"error":{"message":"upstream is not actually gzipped"}}`
	resp := responseWithEncoding("gzip", []byte(payload))
	DecompressResponse(resp)

	if got := readAll(t, resp); got != payload {
		t.Fatalf("body = %q, want untouched payload %q", got, payload)
	}
}

func TestDecompressResponseKeepsBodyIntactOnInvalidDeflate(t *testing.T) {
	payload := `plain text body that is not zlib`
	resp := responseWithEncoding("deflate", []byte(payload))
	DecompressResponse(resp)

	if got := readAll(t, resp); got != payload {
		t.Fatalf("body = %q, want untouched payload %q", got, payload)
	}
}

// 未知编码（br/zstd）不解压，但同样必须保证正文完整。
func TestDecompressResponseKeepsBodyIntactOnUnknownEncoding(t *testing.T) {
	payload := strings.Repeat("brotli-ish-bytes", 8)
	resp := responseWithEncoding("br", []byte(payload))
	DecompressResponse(resp)

	if got := readAll(t, resp); got != payload {
		t.Fatalf("body = %q, want untouched payload", got)
	}
	if resp.Header.Get("Content-Encoding") != "br" {
		t.Fatal("unknown encoding header should be preserved for upper layers")
	}
}

func TestDecompressResponseNoEncodingIsNoop(t *testing.T) {
	payload := `{"plain":true}`
	resp := responseWithEncoding("", []byte(payload))
	DecompressResponse(resp)

	if got := readAll(t, resp); got != payload {
		t.Fatalf("body = %q", got)
	}
}

// 空正文不应 panic（Peek 会返回 io.EOF）。
func TestDecompressResponseEmptyBody(t *testing.T) {
	resp := responseWithEncoding("gzip", nil)
	DecompressResponse(resp)

	if got := readAll(t, resp); got != "" {
		t.Fatalf("body = %q, want empty", got)
	}
}
