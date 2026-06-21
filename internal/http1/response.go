package http1

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxLineLength = 8192

type HeaderField struct {
	Name  string
	Value string
}

type Response struct {
	Version      string
	StatusCode   int
	ReasonPhrase string
	HeaderFields []HeaderField
	Body         []byte
}

type LineReader struct {
	reader    *bufio.Reader
	maxLength int
}

func NewLineReader(r *bufio.Reader) *LineReader {
	return &LineReader{
		reader:    r,
		maxLength: MaxLineLength,
	}
}

func (r *LineReader) ReadLine() (string, error) {
	var line []byte
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("incomplete line: %w", err)
			}
			return "", err
		}

		line = append(line, b)
		if len(line) > r.maxLength {
			return "", fmt.Errorf("line exceeds %d bytes", r.maxLength)
		}
		if b == '\n' {
			break
		}
	}

	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", fmt.Errorf("line must end with CRLF")
	}
	return string(line[:len(line)-2]), nil
}

func ReadResponse(r io.Reader) (*Response, error) {
	buffered := bufio.NewReader(r)
	lineReader := NewLineReader(buffered)

	statusLine, err := lineReader.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("read status line: %w", err)
	}

	response, err := ParseStatusLine(statusLine)
	if err != nil {
		return nil, fmt.Errorf("parse status line: %w", err)
	}

	headers, err := ReadHeaderFields(lineReader)
	if err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}
	response.HeaderFields = headers

	if err := rejectUnsupportedTransferEncoding(headers); err != nil {
		return nil, err
	}

	length, ok, err := ContentLength(headers)
	if err != nil {
		return nil, err
	}
	if ok {
		body, err := ReadFixedBody(buffered, length)
		if err != nil {
			return nil, err
		}
		response.Body = body
	}

	return response, nil
}

func ParseStatusLine(line string) (*Response, error) {
	version, rest, ok := strings.Cut(line, " ")
	if !ok {
		return nil, fmt.Errorf("missing status code")
	}
	if version != "HTTP/1.0" && version != "HTTP/1.1" {
		return nil, fmt.Errorf("unsupported HTTP version %q", version)
	}

	codeText, reason, _ := strings.Cut(rest, " ")
	if len(codeText) != 3 {
		return nil, fmt.Errorf("status code must be three digits")
	}
	code, err := strconv.Atoi(codeText)
	if err != nil {
		return nil, fmt.Errorf("invalid status code %q", codeText)
	}

	return &Response{
		Version:      version,
		StatusCode:   code,
		ReasonPhrase: reason,
	}, nil
}

func ReadHeaderFields(r *LineReader) ([]HeaderField, error) {
	var fields []HeaderField
	for {
		line, err := r.ReadLine()
		if err != nil {
			return nil, err
		}
		if line == "" {
			return fields, nil
		}

		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed header field %q", line)
		}
		name = strings.TrimSpace(name)
		if err := validateHeaderFieldName(name); err != nil {
			return nil, err
		}

		fields = append(fields, HeaderField{
			Name:  name,
			Value: strings.TrimSpace(value),
		})
	}
}

func ContentLength(fields []HeaderField) (int64, bool, error) {
	var length int64
	var seen bool

	for _, field := range fields {
		if !strings.EqualFold(field.Name, "Content-Length") {
			continue
		}

		current, err := strconv.ParseInt(field.Value, 10, 64)
		if err != nil || current < 0 {
			return 0, false, fmt.Errorf("invalid Content-Length %q", field.Value)
		}
		if seen && current != length {
			return 0, false, fmt.Errorf("conflicting Content-Length values")
		}

		length = current
		seen = true
	}

	return length, seen, nil
}

func ReadFixedBody(r io.Reader, length int64) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	if length > int64(int(length)) {
		return nil, fmt.Errorf("Content-Length is too large for this platform")
	}

	body := make([]byte, int(length))
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read fixed response body: %w", err)
	}
	return body, nil
}

func rejectUnsupportedTransferEncoding(fields []HeaderField) error {
	for _, field := range fields {
		if !strings.EqualFold(field.Name, "Transfer-Encoding") {
			continue
		}
		if strings.EqualFold(field.Value, "identity") {
			continue
		}
		return fmt.Errorf("unsupported Transfer-Encoding %q", field.Value)
	}
	return nil
}

func validateHeaderFieldName(name string) error {
	if name == "" {
		return fmt.Errorf("header field name is empty")
	}
	if strings.ContainsAny(name, " \t\r\n:") {
		return fmt.Errorf("header field name %q is invalid", name)
	}
	return nil
}
