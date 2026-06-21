package http1

import (
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

func NewRequestForURL(method string, u *url.URL, fields []HeaderField, body []byte) (*Request, error) {
	if u == nil {
		return nil, fmt.Errorf("URL is nil")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}

	target := u.EscapedPath()
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}

	fields = append([]HeaderField{{Name: "Host", Value: u.Host}}, fields...)
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
	if len(request.Body) > 0 {
		if _, err := w.Write(request.Body); err != nil {
			return err
		}
	}
	return nil
}

func (r *Request) prepare() error {
	if r.Method == "" {
		return fmt.Errorf("method is required")
	}
	if strings.ContainsAny(r.Method, " \t\r\n") {
		return fmt.Errorf("method contains whitespace")
	}
	if r.Target == "" || !strings.HasPrefix(r.Target, "/") {
		return fmt.Errorf("target must start with /")
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

func (r *Request) prepareContentLength() error {
	length, ok, err := ContentLength(r.HeaderFields)
	if err != nil {
		return err
	}

	bodyLength := int64(len(r.Body))
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

func hasHeaderField(fields []HeaderField, name string) bool {
	for _, field := range fields {
		if strings.EqualFold(field.Name, name) {
			return true
		}
	}
	return false
}
