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
	// 第二批保持现有行为：失败继续尝试下一个候选。
	// 第三/六批再接入 CC-Switch 风格 retryable/non-retryable 策略。
	return true
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
