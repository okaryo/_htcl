package http1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
)

type ErrorKind string

const (
	ErrorKindNetwork     ErrorKind = "network"
	ErrorKindTimeout     ErrorKind = "timeout"
	ErrorKindProtocol    ErrorKind = "protocol"
	ErrorKindApplication ErrorKind = "application"
)

type ErrorPhase string

const (
	ErrorPhaseDial           ErrorPhase = "dial"
	ErrorPhaseTLSHandshake   ErrorPhase = "tls_handshake"
	ErrorPhaseSerialize      ErrorPhase = "serialize_request"
	ErrorPhaseWriteRequest   ErrorPhase = "write_request"
	ErrorPhaseReadResponse   ErrorPhase = "read_response"
	ErrorPhaseResponseStatus ErrorPhase = "response_status"
)

type ClientError struct {
	Kind  ErrorKind
	Phase ErrorPhase
	Err   error
}

func (e *ClientError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Phase == "" {
		return fmt.Sprintf("%s error: %v", e.Kind, e.Err)
	}
	return fmt.Sprintf("%s error during %s: %v", e.Kind, e.Phase, e.Err)
}

func (e *ClientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ResponseStatusError(response *Response) error {
	if response == nil || response.StatusCode < 400 {
		return nil
	}
	return &ClientError{
		Kind:  ErrorKindApplication,
		Phase: ErrorPhaseResponseStatus,
		Err:   fmt.Errorf("HTTP status %d %s", response.StatusCode, response.ReasonPhrase),
	}
}

func classifyClientError(phase ErrorPhase, err error) error {
	if err == nil {
		return nil
	}

	kind := ErrorKindNetwork
	if isTimeoutError(err) {
		kind = ErrorKindTimeout
	} else if phase == ErrorPhaseSerialize || (phase == ErrorPhaseReadResponse && !hasNetworkError(err) && !isConnectionEOF(err)) {
		kind = ErrorKindProtocol
	}

	return &ClientError{
		Kind:  kind,
		Phase: phase,
		Err:   err,
	}
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func hasNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}

func isConnectionEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
