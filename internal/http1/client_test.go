package http1

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientDoSendsRequestReadsResponseAndClosesConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	closed := make(chan bool, 1)
	go serveClientOnce(t, listener, requests, closed)

	request, err := NewRequest("GET", "/hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	response, err := client.Do(listener.Addr().String(), request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}

	gotRequest := <-requests
	if !strings.HasPrefix(gotRequest, "GET /hello HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Connection: close\r\n") {
		t.Fatalf("missing Connection: close header:\n%s", gotRequest)
	}
	if !<-closed {
		t.Fatal("client did not close the connection")
	}
}

func TestClientDoOverridesConnectionKeepAliveForOneShotRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	closed := make(chan bool, 1)
	go serveClientOnce(t, listener, requests, closed)

	request, err := NewRequest("GET", "/hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Connection", Value: "keep-alive"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	if _, err := client.Do(listener.Addr().String(), request); err != nil {
		t.Fatalf("Do: %v", err)
	}

	gotRequest := <-requests
	if strings.Contains(gotRequest, "Connection: keep-alive\r\n") {
		t.Fatalf("kept caller Connection header:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Connection: close\r\n") {
		t.Fatalf("missing Connection: close header:\n%s", gotRequest)
	}
	if got := request.HeaderFields[1].Value; got != "keep-alive" {
		t.Fatalf("Client.Do mutated caller request header to %q", got)
	}
	<-closed
}

func TestConnectionRoundTripCanUseSameTCPConnectionTwice(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveTwoRequestsOnOneConnection(t, listener, requests)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	first := newTestRequest(t, "/first")
	second := newTestRequest(t, "/second")

	firstResponse, err := connection.RoundTrip(first)
	if err != nil {
		t.Fatalf("first RoundTrip: %v", err)
	}
	if got := string(firstResponse.Body); got != "one" {
		t.Fatalf("first body = %q", got)
	}
	if !connection.Reusable() {
		t.Fatal("connection should be reusable after first response")
	}

	secondResponse, err := connection.RoundTrip(second)
	if err != nil {
		t.Fatalf("second RoundTrip: %v", err)
	}
	if got := string(secondResponse.Body); got != "two" {
		t.Fatalf("second body = %q", got)
	}
	if !connection.Reusable() {
		t.Fatal("connection should be reusable after second response")
	}

	firstRequest := <-requests
	if !strings.HasPrefix(firstRequest, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request mismatch:\n%s", firstRequest)
	}
	secondRequest := <-requests
	if !strings.HasPrefix(secondRequest, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request mismatch:\n%s", secondRequest)
	}
}

func TestConnectionRoundTripMarksConnectionNotReusableForRequestClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	request, err := NewRequest("GET", "/close", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Connection", Value: "close"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(request); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after request Connection: close")
	}
	<-requests
}

func TestConnectionRoundTripMarksConnectionNotReusableForResponseClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(newTestRequest(t, "/close")); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after response Connection: close")
	}
	<-requests
}

func TestConnectionRoundTripMarksConnectionNotReusableAfterError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"not an HTTP response\r\n\r\n",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(newTestRequest(t, "/bad")); err == nil {
		t.Fatal("expected an error")
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after response parse error")
	}
	<-requests
}

func serveClientOnce(t *testing.T, listener net.Listener, requests chan<- string, closed chan<- bool) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Errorf("read request line: %v", err)
			return
		}
		request.WriteString(line)
		if line == "\r\n" {
			break
		}
	}
	requests <- request.String()

	response := "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello"
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
		return
	}

	_, err = reader.ReadByte()
	closed <- err == io.EOF
}

func serveTwoRequestsOnOneConnection(t *testing.T, listener net.Listener, requests chan<- string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	serveResponsesOnReader(t, conn, reader, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\none",
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\ntwo",
	})
}

func serveResponsesOnOneConnection(t *testing.T, listener net.Listener, requests chan<- string, responses []string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	serveResponsesOnReader(t, conn, reader, requests, responses)
}

func serveResponsesOnReader(t *testing.T, conn net.Conn, reader *bufio.Reader, requests chan<- string, responses []string) {
	t.Helper()

	for _, response := range responses {
		request, err := readHeaderBlock(reader)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		requests <- request

		if _, err := io.WriteString(conn, response); err != nil {
			t.Errorf("write response: %v", err)
			return
		}
	}
}

func readHeaderBlock(reader *bufio.Reader) (string, error) {
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		request.WriteString(line)
		if line == "\r\n" {
			return request.String(), nil
		}
	}
}

func newTestRequest(t *testing.T, target string) *Request {
	t.Helper()

	request, err := NewRequest("GET", target, []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return request
}
