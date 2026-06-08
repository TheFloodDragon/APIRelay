package controller

import (
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) relayJSON(c *gin.Context, requestID string, startTime time.Time, app constant.RelayApp, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, body []byte, candidates []relayCandidate) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range candidates {
		info := buildRelayInfo(c, requestID, startTime, app, mode, format, meta, candidate, false)
		protocolAdaptor := adaptor.GetAdaptor(info.APIType)

		requestBody, err := bodyWithResolvedModel(body, info.ResolvedModel, format)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, http.StatusBadRequest, lastErrMsg)
			continue
		}

		convertedBody, err := convertRelayRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		attemptedUpstream = true
		headers := http.Header{}
		protocolAdaptor.SetupHeaders(headers, info.Channel.APIKey, mode)
		url := requestURL(protocolAdaptor, info, false)

		statusCode, respBody, err := rc.httpClient.DoJSON(c.Request.Context(), c.Request.Method, url, headers, convertedBody, timeoutForChannel(info.Channel))
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			convertedResp, err := protocolAdaptor.ConvertResponse(respBody, mode, format)
			if err != nil {
				lastErr = err
				lastErrMsg = err.Error()
				rc.logRequest(c, info, http.StatusBadGateway, lastErrMsg)
				continue
			}
			rc.logRequest(c, info, statusCode, "")
			c.Data(statusCode, "application/json", convertedResp)
			return
		}

		lastErr = nil
		lastErrMsg = protocolAdaptor.ErrorMessage(respBody)
		if lastErrMsg == "" {
			lastErrMsg = string(respBody)
		}
		rc.logRequest(c, info, statusCode, lastErrMsg)
	}

	writeFinalRelayError(c, lastErr, lastErrMsg, attemptedUpstream)
}

func requestURL(protocolAdaptor adaptor.Adaptor, info *relayinfo.RelayInfo, stream bool) string {
	if urlAdaptor, ok := protocolAdaptor.(adaptor.ModelAwareURLAdaptor); ok {
		return urlAdaptor.GetRequestURLWithModel(info.Channel.BaseURL, info.RelayMode, info.ResolvedModel, stream)
	}
	return protocolAdaptor.GetRequestURL(info.Channel.BaseURL, info.RelayMode)
}
