package http1

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
)

func TestWriteRequestSerializesRequestLineHeadersAndBody(t *testing.T) {
	request, err := NewRequest("POST", "/submit?debug=1", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "User-Agent", Value: "htcl-test"},
	}, []byte("hello"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	var out bytes.Buffer
	if err := WriteRequest(&out, request); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	want := "" +
		"POST /submit?debug=1 HTTP/1.1\r\n" +
		"Host: example.test\r\n" +
		"User-Agent: htcl-test\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"
	if got := out.String(); got != want {
		t.Fatalf("request mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestNewRequestForURLSetsHostAndTarget(t *testing.T) {
	u, err := url.Parse("http://example.test:8080/search?q=hello%20world")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}

	request, err := NewRequestForURL("GET", u, []HeaderField{
		{Name: "Connection", Value: "close"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequestForURL: %v", err)
	}

	var out bytes.Buffer
	if err := WriteRequest(&out, request); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	got := out.String()
	if !strings.HasPrefix(got, "GET /search?q=hello%20world HTTP/1.1\r\n") {
		t.Fatalf("request target mismatch:\n%s", got)
	}
	if !strings.Contains(got, "Host: example.test:8080\r\n") {
		t.Fatalf("Host header mismatch:\n%s", got)
	}
}

func TestWriteRequestStreamsRequestBody(t *testing.T) {
	request, err := NewStreamingRequest("POST", "/upload", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, iotest.OneByteReader(strings.NewReader("hello")), 5)
	if err != nil {
		t.Fatalf("NewStreamingRequest: %v", err)
	}

	var out bytes.Buffer
	if err := WriteRequest(&out, request); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	want := "" +
		"POST /upload HTTP/1.1\r\n" +
		"Host: example.test\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"
	if got := out.String(); got != want {
		t.Fatalf("request mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestStreamRequestBodyCopiesOnlyDeclaredLength(t *testing.T) {
	input := strings.NewReader("helloextra")
	var out bytes.Buffer

	written, err := StreamRequestBody(&out, input, 5)
	if err != nil {
		t.Fatalf("StreamRequestBody: %v", err)
	}
	if written != 5 {
		t.Fatalf("written = %d, want 5", written)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("body = %q", got)
	}
	remaining, err := io.ReadAll(input)
	if err != nil {
		t.Fatalf("ReadAll remaining: %v", err)
	}
	if got := string(remaining); got != "extra" {
		t.Fatalf("remaining = %q", got)
	}
}

func TestStreamRequestBodyRejectsIncompleteBody(t *testing.T) {
	var out bytes.Buffer
	written, err := StreamRequestBody(&out, strings.NewReader("he"), 5)
	if err == nil {
		t.Fatal("expected an error")
	}
	if written != 2 {
		t.Fatalf("written = %d, want 2", written)
	}
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteRequestAcceptsAbsoluteFormTarget(t *testing.T) {
	request, err := NewRequest("GET", "http://example.test/search?q=hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	var out bytes.Buffer
	if err := WriteRequest(&out, request); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	if !strings.HasPrefix(out.String(), "GET http://example.test/search?q=hello HTTP/1.1\r\n") {
		t.Fatalf("request target mismatch:\n%s", out.String())
	}
}

func TestNewRequestRejectsMissingHostForHTTP11(t *testing.T) {
	_, err := NewRequest("GET", "/", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Host header is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRequestRejectsInvalidTarget(t *testing.T) {
	_, err := NewRequest("GET", "hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "target must start with / or be an absolute URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRequestRejectsContentLengthMismatch(t *testing.T) {
	_, err := NewRequest("POST", "/", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Content-Length", Value: "10"},
	}, []byte("hello"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not match body length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStreamingRequestRejectsContentLengthMismatch(t *testing.T) {
	_, err := NewStreamingRequest("POST", "/", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Content-Length", Value: "10"},
	}, strings.NewReader("hello"), 5)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not match body length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStreamingRequestRejectsMissingBodyReader(t *testing.T) {
	_, err := NewStreamingRequest("POST", "/", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil, 5)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "request body reader is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
