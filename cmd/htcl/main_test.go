package main

import (
	"bufio"
	"bytes"
	"fmt"
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

func TestRunFollowsOneRedirectForGETURL(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveRedirectThenOK(t, listener, requests, "/second")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/first"
	err = run([]string{"-follow", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	first := <-requests
	if !strings.HasPrefix(first, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request line mismatch:\n%s", first)
	}
	second := <-requests
	if !strings.HasPrefix(second, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request line mismatch:\n%s", second)
	}
	got := stdout.String()
	if !strings.Contains(got, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("final response status was not printed:\n%s", got)
	}
	if !strings.HasSuffix(got, "hello") {
		t.Fatalf("final response body mismatch:\n%s", got)
	}
	if !strings.Contains(stderr.String(), "following redirect to http://"+listener.Addr().String()+"/second") {
		t.Fatalf("missing redirect log:\n%s", stderr.String())
	}
}

func TestRunFollowsRedirectChainUpToLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 3)
	go serveRedirectsThenOK(t, listener, requests, []string{"/second", "/third"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/first"
	err = run([]string{"-follow", "-max-redirects", "2", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	first := <-requests
	if !strings.HasPrefix(first, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request line mismatch:\n%s", first)
	}
	second := <-requests
	if !strings.HasPrefix(second, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request line mismatch:\n%s", second)
	}
	third := <-requests
	if !strings.HasPrefix(third, "GET /third HTTP/1.1\r\n") {
		t.Fatalf("third request line mismatch:\n%s", third)
	}
	if !strings.HasSuffix(stdout.String(), "hello") {
		t.Fatalf("final response body mismatch:\n%s", stdout.String())
	}
}

func TestRunStopsAtRedirectLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveRedirects(t, listener, requests, []string{"/second", "/third"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/first"
	err = run([]string{"-follow", "-max-redirects", "1", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "stopped after 1 redirects") {
		t.Fatalf("unexpected error: %v", err)
	}

	first := <-requests
	if !strings.HasPrefix(first, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request line mismatch:\n%s", first)
	}
	second := <-requests
	if !strings.HasPrefix(second, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request line mismatch:\n%s", second)
	}
}

func TestRunRejectsNegativeRedirectLimit(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-follow", "-max-redirects", "-1", "http://example.test/"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "-max-redirects must be zero or greater") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDoesNotFollowRedirectByDefault(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveRedirectOnce(t, listener, requests, "/second")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/first"
	err = run([]string{"-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	request := <-requests
	if !strings.HasPrefix(request, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", request)
	}
	got := stdout.String()
	if !strings.Contains(got, "HTTP/1.1 302 Found\r\n") {
		t.Fatalf("redirect response status was not printed:\n%s", got)
	}
	if !strings.Contains(got, "Location: /second\r\n") {
		t.Fatalf("redirect response Location was not printed:\n%s", got)
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

func TestRunChangesPostToGetFor303Redirect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveRedirectThenOKWithStatus(t, listener, requests, 303, "/done")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/submit"
	err = run([]string{
		"-follow",
		"-method", "POST",
		"-body", "hello",
		"-timeout", "2s",
		rawURL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	first := <-requests
	if !strings.HasPrefix(first, "POST /submit HTTP/1.1\r\n") {
		t.Fatalf("first request line mismatch:\n%s", first)
	}
	if !strings.HasSuffix(first, "\r\n\r\nhello") {
		t.Fatalf("first request body mismatch:\n%s", first)
	}
	second := <-requests
	if !strings.HasPrefix(second, "GET /done HTTP/1.1\r\n") {
		t.Fatalf("second request line mismatch:\n%s", second)
	}
	if strings.Contains(second, "Content-Length: 5\r\n") {
		t.Fatalf("second request kept original Content-Length:\n%s", second)
	}
	if strings.HasSuffix(second, "\r\n\r\nhello") {
		t.Fatalf("second request kept original body:\n%s", second)
	}
}

func TestRunPreservesPostBodyFor307Redirect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveRedirectThenOKWithStatus(t, listener, requests, 307, "/done")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/submit"
	err = run([]string{
		"-follow",
		"-method", "POST",
		"-body", "hello",
		"-timeout", "2s",
		rawURL,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}

	first := <-requests
	if !strings.HasPrefix(first, "POST /submit HTTP/1.1\r\n") {
		t.Fatalf("first request line mismatch:\n%s", first)
	}
	second := <-requests
	if !strings.HasPrefix(second, "POST /done HTTP/1.1\r\n") {
		t.Fatalf("second request line mismatch:\n%s", second)
	}
	if !strings.Contains(second, "Content-Length: 5\r\n") {
		t.Fatalf("second request missing Content-Length:\n%s", second)
	}
	if !strings.HasSuffix(second, "\r\n\r\nhello") {
		t.Fatalf("second request body mismatch:\n%s", second)
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

func TestRunOutputBodyOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/hello"
	err = run([]string{"-output", "body", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}
	<-requests

	if got := stdout.String(); got != "hello" {
		t.Fatalf("body output mismatch: %q", got)
	}
}

func TestRunOutputHeadersOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/hello"
	err = run([]string{"-output", "headers", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}
	<-requests

	got := stdout.String()
	if !strings.Contains(got, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("headers output missing status line:\n%s", got)
	}
	if !strings.Contains(got, "Content-Length: 5\r\n") {
		t.Fatalf("headers output missing header:\n%s", got)
	}
	if strings.Contains(got, "hello") {
		t.Fatalf("headers output included body:\n%s", got)
	}
	if !strings.HasSuffix(got, "\r\n\r\n") {
		t.Fatalf("headers output missing final blank line:\n%s", got)
	}
}

func TestRunOutputStatusOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveOnce(t, listener, requests)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rawURL := "http://" + listener.Addr().String() + "/hello"
	err = run([]string{"-output", "status", "-timeout", "2s", rawURL}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr.String())
	}
	<-requests

	if got := stdout.String(); got != "HTTP/1.1 200 OK\r\n" {
		t.Fatalf("status output mismatch: %q", got)
	}
}

func TestRunRejectsUnsupportedOutputMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-output", "json", "http://example.test/"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `unsupported output mode "json"`) {
		t.Fatalf("unexpected error: %v", err)
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

	request, err := readHTTPRequest(conn)
	if err != nil {
		t.Errorf("read request: %v", err)
		return
	}
	requests <- request

	response := "HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello"
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func serveRedirectOnce(t *testing.T, listener net.Listener, requests chan<- string, location string) {
	t.Helper()

	serveRedirectOnceWithStatus(t, listener, requests, 302, location)
}

func serveRedirectOnceWithStatus(t *testing.T, listener net.Listener, requests chan<- string, statusCode int, location string) {
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

	request, err := readHTTPRequest(conn)
	if err != nil {
		t.Errorf("read request: %v", err)
		return
	}
	requests <- request

	response := fmt.Sprintf("HTTP/1.1 %03d %s\r\nLocation: %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", statusCode, redirectReason(statusCode), location)
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func serveRedirectThenOK(t *testing.T, listener net.Listener, requests chan<- string, location string) {
	t.Helper()

	serveRedirectOnce(t, listener, requests, location)
	serveOnce(t, listener, requests)
}

func serveRedirectThenOKWithStatus(t *testing.T, listener net.Listener, requests chan<- string, statusCode int, location string) {
	t.Helper()

	serveRedirectOnceWithStatus(t, listener, requests, statusCode, location)
	serveOnce(t, listener, requests)
}

func serveRedirectsThenOK(t *testing.T, listener net.Listener, requests chan<- string, locations []string) {
	t.Helper()

	serveRedirects(t, listener, requests, locations)
	serveOnce(t, listener, requests)
}

func serveRedirects(t *testing.T, listener net.Listener, requests chan<- string, locations []string) {
	t.Helper()

	for _, location := range locations {
		serveRedirectOnce(t, listener, requests, location)
	}
}

func redirectReason(statusCode int) string {
	switch statusCode {
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 303:
		return "See Other"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	default:
		return "Redirect"
	}
}

func readHTTPRequest(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	var request strings.Builder
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read request line: %w", err)
		}
		request.WriteString(line)
		if name, value, ok := strings.Cut(strings.TrimRight(line, "\r\n"), ":"); ok && strings.EqualFold(name, "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return "", fmt.Errorf("parse Content-Length: %w", err)
			}
		}
		if line == "\r\n" {
			break
		}
	}
	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return "", fmt.Errorf("read request body: %w", err)
		}
		request.Write(body)
	}
	return request.String(), nil
}
