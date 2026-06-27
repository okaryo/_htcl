package http1

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadResponseParsesStatusHeadersAndFixedBody(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"

	response, err := ReadResponse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}

	if response.Version != "HTTP/1.1" {
		t.Fatalf("Version = %q", response.Version)
	}
	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if response.ReasonPhrase != "OK" {
		t.Fatalf("ReasonPhrase = %q", response.ReasonPhrase)
	}
	if len(response.HeaderFields) != 2 {
		t.Fatalf("HeaderFields length = %d", len(response.HeaderFields))
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}
}

func TestReadResponseHandlesPartialReads(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"

	response, err := ReadResponse(iotest.OneByteReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}
}

func TestReadResponseDecodesGzipBody(t *testing.T) {
	body := gzipBytes(t, "hello")
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Encoding: gzip\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n", len(body)) +
		"\r\n" +
		string(body)

	response, err := ReadResponse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}
}

func TestReadResponseRejectsUnsupportedContentEncoding(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Encoding: br\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "unsupported Content-Encoding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsMalformedStatusLine(t *testing.T) {
	_, err := ReadResponse(strings.NewReader("NOPE\r\n\r\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "parse status line") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsMalformedHeader(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length 5\r\n" +
		"\r\n" +
		"hello"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "malformed header field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsInvalidContentLength(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length: nope\r\n" +
		"\r\n"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "invalid Content-Length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsIncompleteFixedBody(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"he"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsUnsupportedTransferEncoding(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "unsupported Transfer-Encoding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func gzipBytes(t *testing.T, value string) []byte {
	t.Helper()

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return compressed.Bytes()
}
