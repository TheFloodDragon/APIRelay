package controller

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
)

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
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			convertedResp, err := attempt.ProtocolAdaptor.ConvertResponse(respBody, reqCtx.Mode, reqCtx.Format)
			if err != nil {
				lastErr = err
				lastErrMsg = err.Error()
				rc.logRequest(reqCtx.Gin, attempt.Info, http.StatusBadGateway, lastErrMsg)
				continue
			}
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, "")
			reqCtx.Gin.Data(statusCode, "application/json", convertedResp)
			return
		}

		lastErr = nil
		lastErrMsg = attempt.ProtocolAdaptor.ErrorMessage(respBody)
		if lastErrMsg == "" {
			lastErrMsg = string(respBody)
		}
		rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
	}

	writeFinalRelayError(reqCtx.Gin, lastErr, lastErrMsg, attemptedUpstream)
}

func requestURL(protocolAdaptor adaptor.Adaptor, info *relayinfo.RelayInfo, stream bool) string {
	if urlAdaptor, ok := protocolAdaptor.(adaptor.ModelAwareURLAdaptor); ok {
		return urlAdaptor.GetRequestURLWithModel(info.Channel.BaseURL, info.RelayMode, info.ResolvedModel, stream)
	}
	return protocolAdaptor.GetRequestURL(info.Channel.BaseURL, info.RelayMode)
}
