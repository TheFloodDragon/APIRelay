package controller

import "net/http"

func (rc *RelayController) relayJSON(reqCtx *RequestContext) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range reqCtx.Candidates {
		attempt, err := rc.buildRelayAttempt(reqCtx, candidate, false)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := relayAttemptErrorStatus(err, http.StatusBadGateway)
			if attempt != nil {
				rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
			}
			continue
		}

		attemptedUpstream = true
		statusCode, respBody, err := rc.httpClient.DoJSON(
			reqCtx.Gin.Request.Context(),
			reqCtx.Method,
			attempt.URL,
			attempt.Headers,
			attempt.ConvertedBody,
			timeoutForChannel(attempt.Info.Channel),
		)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.recordCircuitFailure(attempt.Info, statusCode, err)
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
			continue
		}

		if isSuccessfulStatus(statusCode) {
			convertedResp, err := relayResponseBody(attempt, respBody)
			if err != nil {
				lastErr = err
				lastErrMsg = err.Error()
				rc.recordCircuitFailure(attempt.Info, http.StatusBadGateway, err)
				rc.logRequest(reqCtx.Gin, attempt.Info, http.StatusBadGateway, lastErrMsg)
				continue
			}
			rc.recordCircuitSuccess(attempt.Info)
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, "")
			reqCtx.Gin.Data(statusCode, "application/json", convertedResp)
			return
		}

		rc.recordCircuitFailure(attempt.Info, statusCode, nil)
		lastErr = nil
		lastErrMsg = attempt.ProtocolAdaptor.ErrorMessage(respBody)
		if lastErrMsg == "" {
			lastErrMsg = string(respBody)
		}
		rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
	}

	writeFinalRelayError(reqCtx.Gin, lastErr, lastErrMsg, attemptedUpstream)
}
