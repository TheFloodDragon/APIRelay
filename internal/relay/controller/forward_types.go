package controller

import (
	"errors"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
)

type RelayAttempt struct {
	Info            *relayinfo.RelayInfo
	ProtocolAdaptor adaptor.Adaptor
	RequestBody     []byte
	ConvertedBody   []byte
	Headers         http.Header
	URL             string
}

type relayAttemptBuildError struct {
	statusCode int
	err        error
}

func newRelayAttemptBuildError(statusCode int, err error) error {
	return &relayAttemptBuildError{statusCode: statusCode, err: err}
}

func (e *relayAttemptBuildError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *relayAttemptBuildError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func relayAttemptErrorStatus(err error, fallback int) int {
	var buildErr *relayAttemptBuildError
	if errors.As(err, &buildErr) && buildErr.statusCode != 0 {
		return buildErr.statusCode
	}
	return fallback
}

func buildUpstreamHeaders(
	protocolAdaptor adaptor.Adaptor,
	apiKey string,
	mode constant.RelayMode,
	stream bool,
) http.Header {
	headers := http.Header{}
	protocolAdaptor.SetupHeaders(headers, apiKey, mode)
	if stream {
		headers.Set("Accept", "text/event-stream")
	}
	return headers
}

func (rc *RelayController) buildRelayAttempt(
	reqCtx *RequestContext,
	candidate relayCandidate,
	isStream bool,
) (*RelayAttempt, error) {
	info := buildRelayInfo(reqCtx.Gin, reqCtx.RequestID, reqCtx.StartTime, reqCtx.App, reqCtx.Mode, reqCtx.Format, reqCtx.Meta, candidate, isStream)
	protocolAdaptor := adaptor.GetAdaptor(info.APIType)
	attempt := &RelayAttempt{Info: info, ProtocolAdaptor: protocolAdaptor}

	requestBody, err := bodyWithResolvedModel(reqCtx.Body, info.ResolvedModel, reqCtx.Format)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadRequest, err)
	}
	attempt.RequestBody = requestBody

	convertedBody, err := convertRelayRequest(protocolAdaptor, requestBody, info)
	if err != nil {
		statusCode := http.StatusBadGateway
		if isUnsupportedRelayModeError(err) {
			statusCode = http.StatusBadRequest
		}
		return attempt, newRelayAttemptBuildError(statusCode, err)
	}
	attempt.ConvertedBody = convertedBody
	attempt.Headers = buildUpstreamHeaders(protocolAdaptor, info.Channel.APIKey, reqCtx.Mode, isStream)
	attempt.URL = requestURL(protocolAdaptor, info, isStream)

	return attempt, nil
}
