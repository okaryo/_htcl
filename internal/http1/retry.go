package http1

import "strings"

func IsIdempotentMethod(method string) bool {
	switch {
	case strings.EqualFold(method, "GET"):
		return true
	case strings.EqualFold(method, "HEAD"):
		return true
	case strings.EqualFold(method, "PUT"):
		return true
	case strings.EqualFold(method, "DELETE"):
		return true
	case strings.EqualFold(method, "OPTIONS"):
		return true
	case strings.EqualFold(method, "TRACE"):
		return true
	default:
		return false
	}
}
