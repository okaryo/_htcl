package http1

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
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

func TestReadResponseDecodesChunkedBody(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"5\r\nhello\r\n" +
		"6;ignored=true\r\n world\r\n" +
		"0\r\n" +
		"X-Trailer: ignored\r\n" +
		"\r\n"

	response, err := ReadResponse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if got := string(response.Body); got != "hello world" {
		t.Fatalf("Body = %q", got)
	}
}

func TestReadResponseDecodesGzipChunkedBody(t *testing.T) {
	body := gzipBytes(t, "hello")
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"Content-Encoding: gzip\r\n" +
		"\r\n" +
		fmt.Sprintf("%x\r\n", len(body)) +
		string(body) + "\r\n" +
		"0\r\n" +
		"\r\n"

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
		"Transfer-Encoding: gzip\r\n" +
		"\r\n"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "unsupported Transfer-Encoding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsInvalidChunkSize(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"nope\r\n"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "invalid chunk size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadResponseRejectsMalformedChunkTerminator(t *testing.T) {
	input := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"5\r\nhelloXX"

	_, err := ReadResponse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "chunk data must end with CRLF") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStreamFixedBodyCopiesOnlyContentLength(t *testing.T) {
	input := strings.NewReader("helloextra")
	var out bytes.Buffer

	written, err := StreamFixedBody(&out, input, 5)
	if err != nil {
		t.Fatalf("StreamFixedBody: %v", err)
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

func TestStreamFixedBodyHandlesPartialReads(t *testing.T) {
	var out bytes.Buffer
	written, err := StreamFixedBody(&out, iotest.OneByteReader(strings.NewReader("hello")), 5)
	if err != nil {
		t.Fatalf("StreamFixedBody: %v", err)
	}
	if written != 5 {
		t.Fatalf("written = %d, want 5", written)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("body = %q", got)
	}
}

func TestStreamFixedBodyRejectsIncompleteBody(t *testing.T) {
	var out bytes.Buffer
	written, err := StreamFixedBody(&out, strings.NewReader("he"), 5)
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

func TestStreamFixedBodyWithProgressReportsCopiedBytes(t *testing.T) {
	var out bytes.Buffer
	var progress []Progress

	written, err := StreamFixedBodyWithProgress(&out, iotest.OneByteReader(strings.NewReader("hello")), 5, func(next Progress) {
		progress = append(progress, next)
	})
	if err != nil {
		t.Fatalf("StreamFixedBodyWithProgress: %v", err)
	}
	if written != 5 {
		t.Fatalf("written = %d, want 5", written)
	}
	want := []Progress{
		{Written: 1, Total: 5},
		{Written: 2, Total: 5},
		{Written: 3, Total: 5},
		{Written: 4, Total: 5},
		{Written: 5, Total: 5},
	}
	if fmt.Sprint(progress) != fmt.Sprint(want) {
		t.Fatalf("progress = %+v, want %+v", progress, want)
	}
}

func TestStreamFixedBodyWithProgressReportsZeroLengthBody(t *testing.T) {
	var out bytes.Buffer
	var progress []Progress

	written, err := StreamFixedBodyWithProgress(&out, strings.NewReader("extra"), 0, func(next Progress) {
		progress = append(progress, next)
	})
	if err != nil {
		t.Fatalf("StreamFixedBodyWithProgress: %v", err)
	}
	if written != 0 {
		t.Fatalf("written = %d, want 0", written)
	}
	want := []Progress{{Written: 0, Total: 0}}
	if fmt.Sprint(progress) != fmt.Sprint(want) {
		t.Fatalf("progress = %+v, want %+v", progress, want)
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
