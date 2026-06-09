package response

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const DefaultStreamFirstByteTimeout = 5 * time.Second

var ErrStreamNoData = errors.New("stream ended before first byte")

type preflightReadResult struct {
	n   int
	err error
}

type PreflightStreamBody struct {
	body  io.ReadCloser
	first []byte
	off   int
}

func PrepareStreamBody(ctx context.Context, body io.ReadCloser, timeout time.Duration) (io.ReadCloser, error) {
	if body == nil {
		return nil, fmt.Errorf("stream response body is nil")
	}
	if timeout <= 0 {
		timeout = DefaultStreamFirstByteTimeout
	}

	buffer := make([]byte, 1)
	resultCh := make(chan preflightReadResult, 1)
	go func() {
		n, err := body.Read(buffer)
		resultCh <- preflightReadResult{n: n, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		if result.n > 0 {
			return &PreflightStreamBody{body: body, first: append([]byte(nil), buffer[:result.n]...)}, nil
		}
		if result.err == nil {
			result.err = ErrStreamNoData
		}
		return nil, result.err
	case <-ctx.Done():
		_ = body.Close()
		return nil, ctx.Err()
	case <-timer.C:
		_ = body.Close()
		return nil, fmt.Errorf("stream first byte timeout after %s", timeout)
	}
}

func (b *PreflightStreamBody) Read(p []byte) (int, error) {
	if b.off < len(b.first) {
		n := copy(p, b.first[b.off:])
		b.off += n
		return n, nil
	}
	return b.body.Read(p)
}

func (b *PreflightStreamBody) Close() error {
	return b.body.Close()
}

func CopyPassthroughStream(dst io.Writer, src io.Reader, flush func()) error {
	buffer := make([]byte, 4096)
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, err := dst.Write(buffer[:n]); err != nil {
				return err
			}
			if flush != nil {
				flush()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}
