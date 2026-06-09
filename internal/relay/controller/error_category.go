package controller

import "net/http"

type RelayErrorCategory string

const (
	RelayErrorRetryable    RelayErrorCategory = "retryable"
	RelayErrorNonRetryable RelayErrorCategory = "non_retryable"
)

func categorizeHTTPStatus(statusCode int) RelayErrorCategory {
	switch statusCode {
	case http.StatusBadRequest,
		http.StatusMethodNotAllowed,
		http.StatusNotAcceptable,
		http.StatusRequestEntityTooLarge,
		http.StatusRequestURITooLong,
		http.StatusUnsupportedMediaType,
		http.StatusUnprocessableEntity,
		http.StatusNotImplemented:
		return RelayErrorNonRetryable
	default:
		return RelayErrorRetryable
	}
}

func shouldTryNextCandidate(statusCode int, err error) bool {
	if err != nil {
		return true
	}
	return categorizeHTTPStatus(statusCode) == RelayErrorRetryable
}

func isSuccessfulStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func shouldRecordCircuitFailure(statusCode int, err error) bool {
	if err != nil {
		if statusCode == 0 || isSuccessfulStatus(statusCode) {
			return true
		}
	}
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}
