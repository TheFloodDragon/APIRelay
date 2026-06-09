package controller

import (
	"errors"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/model"
)

func (rc *RelayController) relayJSON(reqCtx *RequestContext) {
	resp, attempt, err := rc.forwardRelayAttempt(
		reqCtx,
		false,
		func(provider model.Channel) (*RelayAttempt, error) {
			candidate, ok := reqCtx.candidateForProvider(provider.ID)
			if !ok {
				return nil, newNonRetryableBuildError(http.StatusNotFound, "provider does not support requested model")
			}
			candidate.Channel = provider
			return rc.buildRelayAttempt(reqCtx, candidate, false)
		},
		relayJSONPreflight,
	)
	if err != nil {
		statusCode, errMsg := relayFailureDetails(err, attempt)
		if attempt != nil {
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, errMsg)
		}
		writeFinalRelayError(reqCtx.Gin, err, errMsg, attempt != nil)
		return
	}
	defer resp.Body.Close()

	convertedResp, err := responseProcessor.ReadAndTransform(resp, func(respBody []byte) ([]byte, error) {
		return relayResponseBody(attempt, respBody)
	})
	if err != nil {
		errMsg := err.Error()
		if attempt != nil {
			if rc.providerRouter != nil {
				rc.providerRouter.RecordFailure(attempt.Info.Channel.ID, errMsg)
			}
			rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, errMsg)
		}
		writeFinalRelayError(reqCtx.Gin, err, errMsg, true)
		return
	}

	if rc.providerRouter != nil && attempt != nil {
		rc.providerRouter.RecordSuccess(attempt.Info.Channel.ID)
	}

	if attempt != nil {
		rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, "")
	}
	responseProcessor.WriteBody(reqCtx.Gin.Writer, resp.StatusCode, resp.Header, "application/json", convertedResp)
}

func relayFailureDetails(err error, attempt *RelayAttempt) (int, string) {
	statusCode := http.StatusServiceUnavailable
	message := ""
	var upstreamErr *relayUpstreamError
	if errors.As(err, &upstreamErr) {
		statusCode = upstreamErr.statusCode
		message = upstreamErr.Error()
	} else if err != nil {
		message = err.Error()
	}
	var buildErr *nonRetryableBuildError
	if errors.As(err, &buildErr) && buildErr.statusCode != 0 {
		statusCode = buildErr.statusCode
	}
	if attempt != nil && attempt.Info != nil {
		statusCode = relayAttemptErrorStatus(err, statusCode)
	}
	return statusCode, message
}
