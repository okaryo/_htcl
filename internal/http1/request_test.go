package http1

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
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
