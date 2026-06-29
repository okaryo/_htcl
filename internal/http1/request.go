package http1

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type Request struct {
	Method       string
	Target       string
	Version      string
	HeaderFields []HeaderField
	Body         []byte
	BodyReader   io.Reader
	BodyLength   int64
}

func NewRequest(method, target string, fields []HeaderField, body []byte) (*Request, error) {
	request := &Request{
		Method:       method,
		Target:       target,
		Version:      "HTTP/1.1",
		HeaderFields: append([]HeaderField(nil), fields...),
		Body:         append([]byte(nil), body...),
	}
	if err := request.prepare(); err != nil {
		return nil, err
	}
	return request, nil
}

func NewStreamingRequest(method, target string, fields []HeaderField, body io.Reader, bodyLength int64) (*Request, error) {
	if body == nil && bodyLength > 0 {
		return nil, fmt.Errorf("request body reader is required")
	}
	request := &Request{
		Method:       method,
		Target:       target,
		Version:      "HTTP/1.1",
		HeaderFields: append([]HeaderField(nil), fields...),
		BodyReader:   body,
		BodyLength:   bodyLength,
	}
	if err := request.prepare(); err != nil {
		return nil, err
	}
	return request, nil
}

func NewRequestForURL(method string, u *url.URL, fields []HeaderField, body []byte) (*Request, error) {
	host, err := HostHeaderForURL(u)
	if err != nil {
		return nil, err
	}

	target, err := RequestTargetForURL(u)
	if err != nil {
		return nil, err
	}

	fields = append([]HeaderField{{Name: "Host", Value: host}}, fields...)
	return NewRequest(method, target, fields, body)
}

func WriteRequest(w io.Writer, request *Request) error {
	if request == nil {
		return fmt.Errorf("request is nil")
	}
	if err := request.prepare(); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "%s %s %s\r\n", request.Method, request.Target, request.Version); err != nil {
		return err
	}
	for _, field := range request.HeaderFields {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", field.Name, field.Value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}
	if request.BodyReader != nil {
		if _, err := StreamRequestBody(w, request.BodyReader, request.BodyLength); err != nil {
			return err
		}
	} else if len(request.Body) > 0 {
		if _, err := w.Write(request.Body); err != nil {
			return err
		}
	}
	return nil
}

func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}
	return &Request{
		Method:       r.Method,
		Target:       r.Target,
		Version:      r.Version,
		HeaderFields: append([]HeaderField(nil), r.HeaderFields...),
		Body:         append([]byte(nil), r.Body...),
		BodyReader:   r.BodyReader,
		BodyLength:   r.BodyLength,
	}
}

func (r *Request) SetHeader(name, value string) {
	for i, field := range r.HeaderFields {
		if strings.EqualFold(field.Name, name) {
			r.HeaderFields[i] = HeaderField{Name: name, Value: value}
			return
		}
	}
	r.HeaderFields = append(r.HeaderFields, HeaderField{Name: name, Value: value})
}

func (r *Request) prepare() error {
	if r.Method == "" {
		return fmt.Errorf("method is required")
	}
	if strings.ContainsAny(r.Method, " \t\r\n") {
		return fmt.Errorf("method contains whitespace")
	}
	if r.Target == "" {
		return fmt.Errorf("target is required")
	}
	if !isSupportedRequestTarget(r.Target) {
		return fmt.Errorf("target must start with / or be an absolute URL")
	}
	if strings.ContainsAny(r.Target, "\r\n") {
		return fmt.Errorf("target contains line break")
	}
	if r.Version == "" {
		r.Version = "HTTP/1.1"
	}
	if r.Version != "HTTP/1.0" && r.Version != "HTTP/1.1" {
		return fmt.Errorf("unsupported HTTP version %q", r.Version)
	}

	for _, field := range r.HeaderFields {
		if err := validateHeaderFieldName(field.Name); err != nil {
			return err
		}
		if strings.ContainsAny(field.Value, "\r\n") {
			return fmt.Errorf("header field %q contains line break", field.Name)
		}
	}

	if r.Version == "HTTP/1.1" && !hasHeaderField(r.HeaderFields, "Host") {
		return fmt.Errorf("Host header is required for HTTP/1.1")
	}

	if err := r.prepareContentLength(); err != nil {
		return err
	}

	return nil
}

func isSupportedRequestTarget(target string) bool {
	return strings.HasPrefix(target, "/") ||
		strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://")
}

func (r *Request) prepareContentLength() error {
	length, ok, err := ContentLength(r.HeaderFields)
	if err != nil {
		return err
	}

	bodyLength, err := r.requestBodyLength()
	if err != nil {
		return err
	}
	if ok {
		if length != bodyLength {
			return fmt.Errorf("Content-Length %d does not match body length %d", length, bodyLength)
		}
		return nil
	}
	if bodyLength == 0 {
		return nil
	}

	r.HeaderFields = append(r.HeaderFields, HeaderField{
		Name:  "Content-Length",
		Value: strconv.FormatInt(bodyLength, 10),
	})
	return nil
}

func (r *Request) requestBodyLength() (int64, error) {
	if r.BodyReader != nil && len(r.Body) > 0 {
		return 0, fmt.Errorf("request body cannot use both bytes and reader")
	}
	if r.BodyReader != nil {
		if r.BodyLength < 0 {
			return 0, fmt.Errorf("request body length must be zero or greater")
		}
		return r.BodyLength, nil
	}
	return int64(len(r.Body)), nil
}

func StreamRequestBody(w io.Writer, r io.Reader, length int64) (int64, error) {
	if length < 0 {
		return 0, fmt.Errorf("request body length must be zero or greater")
	}
	if length == 0 {
		return 0, nil
	}

	written, err := io.CopyN(w, r, length)
	if err != nil {
		if errors.Is(err, io.EOF) && written < length {
			err = io.ErrUnexpectedEOF
		}
		return written, fmt.Errorf("stream request body: %w", err)
	}
	return written, nil
}

func hasHeaderField(fields []HeaderField, name string) bool {
	for _, field := range fields {
		if strings.EqualFold(field.Name, name) {
			return true
		}
	}
	return false
}
