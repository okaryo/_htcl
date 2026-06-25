package http1

import "strings"

func IsRedirectStatus(statusCode int) bool {
	switch statusCode {
	case 301, 302, 303, 307, 308:
		return true
	default:
		return false
	}
}

func HeaderValue(fields []HeaderField, name string) (string, bool) {
	for _, field := range fields {
		if strings.EqualFold(field.Name, name) {
			return field.Value, true
		}
	}
	return "", false
}

func (r *Response) HeaderValue(name string) (string, bool) {
	if r == nil {
		return "", false
	}
	return HeaderValue(r.HeaderFields, name)
}

func (r *Response) RedirectLocation() (string, bool) {
	if r == nil || !IsRedirectStatus(r.StatusCode) {
		return "", false
	}

	location, ok := r.HeaderValue("Location")
	if !ok || location == "" {
		return "", false
	}
	return location, true
}
