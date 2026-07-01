package http1

import (
	"errors"
	"testing"
	"time"
)

func TestIsIdempotentMethod(t *testing.T) {
	tests := map[string]bool{
		"GET":     true,
		"HEAD":    true,
		"PUT":     true,
		"DELETE":  true,
		"OPTIONS": true,
		"TRACE":   true,
		"get":     true,
		"POST":    false,
		"PATCH":   false,
		"CONNECT": false,
		"CUSTOM":  false,
	}

	for method, want := range tests {
		t.Run(method, func(t *testing.T) {
			if got := IsIdempotentMethod(method); got != want {
				t.Fatalf("IsIdempotentMethod(%q) = %v, want %v", method, got, want)
			}
		})
	}
}

func TestShouldRetryUsesMethodAndErrorClassification(t *testing.T) {
	tests := []struct {
		name   string
		method string
		err    error
		want   bool
	}{
		{
			name:   "idempotent network dial failure",
			method: "GET",
			err: &ClientError{
				Kind:  ErrorKindNetwork,
				Phase: ErrorPhaseDial,
				Err:   errors.New("connection refused"),
			},
			want: true,
		},
		{
			name:   "idempotent read timeout",
			method: "GET",
			err: &ClientError{
				Kind:  ErrorKindTimeout,
				Phase: ErrorPhaseReadResponse,
				Err:   errors.New("timeout"),
			},
			want: true,
		},
		{
			name:   "non-idempotent network failure",
			method: "POST",
			err: &ClientError{
				Kind:  ErrorKindNetwork,
				Phase: ErrorPhaseReadResponse,
				Err:   errors.New("connection reset"),
			},
			want: false,
		},
		{
			name:   "protocol failure",
			method: "GET",
			err: &ClientError{
				Kind:  ErrorKindProtocol,
				Phase: ErrorPhaseReadResponse,
				Err:   errors.New("malformed response"),
			},
			want: false,
		},
		{
			name:   "application failure",
			method: "GET",
			err: &ClientError{
				Kind:  ErrorKindApplication,
				Phase: ErrorPhaseResponseStatus,
				Err:   errors.New("HTTP 503"),
			},
			want: false,
		},
		{
			name:   "unclassified error",
			method: "GET",
			err:    errors.New("plain error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRetry(tt.method, tt.err); got != tt.want {
				t.Fatalf("ShouldRetry(%q, %v) = %v, want %v", tt.method, tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryBackoffGrowsAndCaps(t *testing.T) {
	tests := map[int]time.Duration{
		-1: 0,
		0:  100 * time.Millisecond,
		1:  200 * time.Millisecond,
		2:  400 * time.Millisecond,
		10: 2 * time.Second,
	}

	for attempt, want := range tests {
		t.Run(want.String(), func(t *testing.T) {
			if got := RetryBackoff(attempt); got != want {
				t.Fatalf("RetryBackoff(%d) = %s, want %s", attempt, got, want)
			}
		})
	}
}
