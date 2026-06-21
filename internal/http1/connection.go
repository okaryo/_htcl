package http1

import "strings"

func ShouldCloseConnection(version string, fields []HeaderField) bool {
	if HasConnectionToken(fields, "close") {
		return true
	}
	if version == "HTTP/1.0" {
		return !HasConnectionToken(fields, "keep-alive")
	}
	if version == "HTTP/1.1" {
		return false
	}
	return true
}

func HasConnectionToken(fields []HeaderField, token string) bool {
	for _, field := range fields {
		if !strings.EqualFold(field.Name, "Connection") {
			continue
		}
		for _, value := range strings.Split(field.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(value), token) {
				return true
			}
		}
	}
	return false
}

func (r *Response) ShouldCloseConnection() bool {
	if r == nil {
		return true
	}
	return ShouldCloseConnection(r.Version, r.HeaderFields)
}
