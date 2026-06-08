package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const defaultStreamFirstByteTimeout = 5 * time.Second

var errStreamNoData = errors.New("stream ended before first byte")

type preflightReadResult struct {
	n   int
	err error
}

type preflightStreamBody struct {
	body  io.ReadCloser
	first []byte
	off   int
}

func prepareStreamBody(ctx context.Context, body io.ReadCloser, timeout time.Duration) (io.ReadCloser, error) {
	if body == nil {
		return nil, fmt.Errorf("stream response body is nil")
	}
	if timeout <= 0 {
		timeout = defaultStreamFirstByteTimeout
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
			return &preflightStreamBody{body: body, first: append([]byte(nil), buffer[:result.n]...)}, nil
		}
		if result.err == nil {
			result.err = errStreamNoData
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

func (b *preflightStreamBody) Read(p []byte) (int, error) {
	if b.off < len(b.first) {
		n := copy(p, b.first[b.off:])
		b.off += n
		return n, nil
	}
	return b.body.Read(p)
}

func (b *preflightStreamBody) Close() error {
	return b.body.Close()
}
