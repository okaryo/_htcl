package http1

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestClientErrorClassifiesDialFailureAsNetwork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()

	client := Client{Timeout: time.Second}
	_, err = client.Do(address, newTestRequest(t, "/"))
	if err == nil {
		t.Fatal("expected an error")
	}

	assertClientError(t, err, ErrorKindNetwork, ErrorPhaseDial)
}

func TestClientErrorClassifiesReadTimeoutAsTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = readHeaderBlock(bufio.NewReader(conn))
		time.Sleep(200 * time.Millisecond)
	}()

	client := Client{Timeout: 20 * time.Millisecond}
	_, err = client.Do(listener.Addr().String(), newTestRequest(t, "/"))
	if err == nil {
		t.Fatal("expected an error")
	}

	assertClientError(t, err, ErrorKindTimeout, ErrorPhaseReadResponse)
}

func TestClientErrorClassifiesMalformedResponseAsProtocol(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = readHeaderBlock(bufio.NewReader(conn))
		_, _ = conn.Write([]byte("NOPE\r\n\r\n"))
	}()

	client := Client{Timeout: time.Second}
	_, err = client.Do(listener.Addr().String(), newTestRequest(t, "/"))
	if err == nil {
		t.Fatal("expected an error")
	}

	assertClientError(t, err, ErrorKindProtocol, ErrorPhaseReadResponse)
}

func TestClientErrorClassifiesReadResponseEOFAsNetwork(t *testing.T) {
	err := classifyClientError(ErrorPhaseReadResponse, fmt.Errorf("read HTTP response: %w", io.EOF))
	assertClientError(t, err, ErrorKindNetwork, ErrorPhaseReadResponse)
}

func TestResponseStatusErrorClassifiesHTTPStatusAsApplication(t *testing.T) {
	response := &Response{
		Version:      "HTTP/1.1",
		StatusCode:   503,
		ReasonPhrase: "Service Unavailable",
	}

	err := ResponseStatusError(response)
	if err == nil {
		t.Fatal("expected an error")
	}
	assertClientError(t, err, ErrorKindApplication, ErrorPhaseResponseStatus)

	if err := ResponseStatusError(&Response{StatusCode: 200, ReasonPhrase: "OK"}); err != nil {
		t.Fatalf("200 response produced error: %v", err)
	}
}

func assertClientError(t *testing.T, err error, kind ErrorKind, phase ErrorPhase) {
	t.Helper()

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T: %v", err, err)
	}
	if clientErr.Kind != kind {
		t.Fatalf("Kind = %q, want %q", clientErr.Kind, kind)
	}
	if clientErr.Phase != phase {
		t.Fatalf("Phase = %q, want %q", clientErr.Phase, phase)
	}
}
