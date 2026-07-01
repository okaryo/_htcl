package http1

import "time"

type DebugEventName string

const (
	DebugEventDialStart         DebugEventName = "dial_start"
	DebugEventDialDone          DebugEventName = "dial_done"
	DebugEventTLSHandshakeStart DebugEventName = "tls_handshake_start"
	DebugEventTLSHandshakeDone  DebugEventName = "tls_handshake_done"
	DebugEventWriteRequestStart DebugEventName = "write_request_start"
	DebugEventWriteRequestDone  DebugEventName = "write_request_done"
	DebugEventReadResponseStart DebugEventName = "read_response_start"
	DebugEventReadResponseDone  DebugEventName = "read_response_done"
	DebugEventConnectionReused  DebugEventName = "connection_reused"
	DebugEventConnectionIdle    DebugEventName = "connection_idle"
)

type DebugEvent struct {
	Time       time.Time
	Name       DebugEventName
	Phase      ErrorPhase
	Address    string
	Method     string
	Target     string
	StatusCode int
	Reusable   bool
	Err        error
}

type DebugLogger func(DebugEvent)
