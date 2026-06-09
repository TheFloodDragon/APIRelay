package controller

import (
	"context"
	"errors"
	"io"
	"time"

	relayresponse "github.com/TheFloodDragon/APIRelay/internal/relay/response"
)

const defaultStreamFirstByteTimeout = relayresponse.DefaultStreamFirstByteTimeout

var errStreamNoData = relayresponse.ErrStreamNoData

type preflightReadResult struct {
	n   int
	err error
}

type preflightStreamBody = relayresponse.PreflightStreamBody

func prepareStreamBody(ctx context.Context, body io.ReadCloser, timeout time.Duration) (io.ReadCloser, error) {
	prepared, err := relayresponse.PrepareStreamBody(ctx, body, timeout)
	if errors.Is(err, relayresponse.ErrStreamNoData) {
		return nil, errStreamNoData
	}
	return prepared, err
}
