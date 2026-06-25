package main

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunWritesHTTPRequestAndPrintsParsedResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = run([]string{
		"-addr", listener.Addr().String(),
		"-host", "example.test",
		"-target", "/hello?name=htcl",
		"-timeout", "2s",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.HasPrefix(request, "GET /hello?name=htcl HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", request)
	}
	if !strings.Contains(request, "Host: example.test\r\n") {
		t.Fatalf("missing Host header:\n%s", request)
	}
	if !strings.Contains(request, "Connection: close\r\n") {
		t.Fatalf("missing Connection: close header:\n%s", request)
	}

	got := stdout.String()
	if !strings.Contains(got, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("response status was not printed:\n%s", got)
	}
	if !strings.HasSuffix(got, "hello") {
		t.Fatalf("response body mismatch:\n%s", got)
	}
}

func TestRunAcceptsURL(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/hello?name=htcl"
	err = run([]string{"-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.HasPrefix(request, "GET /hello?name=htcl HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", request)
	}
	if !strings.Contains(request, "Host: "+listener.Addr().String()+"\r\n") {
		t.Fatalf("missing Host header:\n%s", request)
	}
}

func TestRunAcceptsMethod(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/status"
	err = run([]string{"-method", "HEAD", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.HasPrefix(request, "HEAD /status HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", request)
	}
}

func TestRunAcceptsCustomHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/headers"
	err = run([]string{
		"-header", "X-Trace: abc123",
		"-header", "User-Agent: custom-client/1.0",
		"-timeout", "2s",
		rawURL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.Contains(request, "X-Trace: abc123\r\n") {
		t.Fatalf("missing custom header:\n%s", request)
	}
	if !strings.Contains(request, "User-Agent: custom-client/1.0\r\n") {
		t.Fatalf("missing overridden User-Agent header:\n%s", request)
	}
	if strings.Contains(request, "User-Agent: htcl/0.1\r\n") {
		t.Fatalf("default User-Agent was not overridden:\n%s", request)
	}
}

func TestRunAcceptsBody(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/submit"
	err = run([]string{
		"-method", "POST",
		"-body", "hello",
		"-timeout", "2s",
		rawURL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.HasPrefix(request, "POST /submit HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", request)
	}
	if !strings.Contains(request, "Content-Length: 5\r\n") {
		t.Fatalf("missing Content-Length header:\n%s", request)
	}
	if !strings.HasSuffix(request, "\r\n\r\nhello") {
		t.Fatalf("request body mismatch:\n%s", request)
	}
}

func TestRunRejectsMismatchedContentLength(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{
		"-method", "POST",
		"-body", "hello",
		"-header", "Content-Length: 4",
		"http://example.test/",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Content-Length 4 does not match body length 5") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRejectsMalformedHeaderFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-header", "X-Trace", "http://example.test/"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "header must use Name: value form") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRejectsHTTPSUntilTLSIsImplemented(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"https://example.test/"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "https URLs require TLS support") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRejectsTargetWithoutLeadingSlash(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-target", "hello"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "target must start with /") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func serveOnce(t *testing.T, listener net.Listener, requests chan<- string) {
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
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Errorf("read request line: %v", err)
			return
		}
		request.WriteString(line)
		if name, value, ok := strings.Cut(strings.TrimRight(line, "\r\n"), ":"); ok && strings.EqualFold(name, "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				t.Errorf("parse Content-Length: %v", err)
				return
			}
		}
		if line == "\r\n" {
			break
		}
	}
	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		request.Write(body)
	}
	requests <- request.String()

	response := "HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello"
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
	}
}
