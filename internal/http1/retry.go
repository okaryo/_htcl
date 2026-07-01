package http1

import (
	"errors"
	"strings"
	"time"
)

func IsIdempotentMethod(method string) bool {
	switch {
	case strings.EqualFold(method, "GET"):
		return true
	case strings.EqualFold(method, "HEAD"):
		return true
	case strings.EqualFold(method, "PUT"):
		return true
	case strings.EqualFold(method, "DELETE"):
		return true
	case strings.EqualFold(method, "OPTIONS"):
		return true
	case strings.EqualFold(method, "TRACE"):
		return true
	default:
		return false
	}
}

func ShouldRetry(method string, err error) bool {
	if err == nil || !IsIdempotentMethod(method) {
		return false
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		return false
	}

	switch clientErr.Kind {
	case ErrorKindNetwork, ErrorKindTimeout:
		return clientErr.Phase == ErrorPhaseDial ||
			clientErr.Phase == ErrorPhaseTLSHandshake ||
			clientErr.Phase == ErrorPhaseWriteRequest ||
			clientErr.Phase == ErrorPhaseReadResponse
	default:
		return false
	}
}

func RetryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		return 0
	}

	delay := 100 * time.Millisecond
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= 2*time.Second {
			return 2 * time.Second
		}
	}
	return delay
}
