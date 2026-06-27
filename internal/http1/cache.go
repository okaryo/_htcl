package http1

import (
	"fmt"
	"strings"
	"time"
)

func CacheControlNoCacheHeader() HeaderField {
	return HeaderField{Name: "Cache-Control", Value: "no-cache"}
}

func IfNoneMatchHeader(etag string) (HeaderField, error) {
	if etag == "" {
		return HeaderField{}, fmt.Errorf("ETag is required")
	}
	if strings.ContainsAny(etag, "\r\n") {
		return HeaderField{}, fmt.Errorf("ETag contains line break")
	}
	return HeaderField{Name: "If-None-Match", Value: etag}, nil
}

func IfModifiedSinceHeader(t time.Time) HeaderField {
	return HeaderField{Name: "If-Modified-Since", Value: HTTPDate(t)}
}

func HTTPDate(t time.Time) string {
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}
