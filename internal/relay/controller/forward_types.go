package controller

import (
	"errors"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/relayinfo"
)

type RelayAttempt struct {
	Info            *relayinfo.RelayInfo
	ProtocolAdaptor adaptor.Adaptor
	ProviderAdapter adaptor.ProviderAdapter
	RequestBody     []byte
	ConvertedBody   []byte
	Headers         http.Header
	URL             string
	NeedsTransform  bool
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

func (rc *RelayController) buildRelayAttempt(
	reqCtx *RequestContext,
	candidate relayCandidate,
	isStream bool,
) (*RelayAttempt, error) {
	info := buildRelayInfo(reqCtx.Gin, reqCtx.RequestID, reqCtx.StartTime, reqCtx.Mode, reqCtx.Format, reqCtx.Meta, candidate, isStream)
	protocolAdaptor := adaptor.GetAdaptor(info.APIType)
	providerAdaptor := adaptor.AsProviderAdapter(protocolAdaptor)
	attempt := &RelayAttempt{Info: info, ProtocolAdaptor: protocolAdaptor, ProviderAdapter: providerAdaptor}

	requestBody, err := bodyWithResolvedModel(reqCtx.Body, info.RequestedModel, info.ResolvedModel, reqCtx.Format)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadRequest, err)
	}
	attempt.RequestBody = requestBody
	attempt.NeedsTransform = providerAdaptor.NeedsTransform(info.Channel, reqCtx.Format)

	if attempt.NeedsTransform {
		convertedBody, err := convertRelayRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			return attempt, newRelayAttemptBuildError(statusCode, err)
		}
		attempt.ConvertedBody = convertedBody
	} else {
		attempt.ConvertedBody = requestBody
	}

	baseURL, err := providerAdaptor.ExtractBaseURL(info.Channel)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadGateway, err)
	}
	apiKey, config := providerAdaptor.ExtractAuth(info.Channel)
	headers, err := providerAdaptor.GetAuthHeaders(apiKey, config, reqCtx.Mode, isStream)
	if err != nil {
		return attempt, newRelayAttemptBuildError(http.StatusBadGateway, err)
	}
	attempt.Headers = headers
	attempt.URL = providerAdaptor.BuildURL(baseURL, reqCtx.Mode, info.ResolvedModel, isStream)

	return attempt, nil
}
