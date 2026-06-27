package http1

import (
	"strings"
	"testing"
	"time"
)

func TestCacheControlNoCacheHeader(t *testing.T) {
	got := CacheControlNoCacheHeader()
	if got != (HeaderField{Name: "Cache-Control", Value: "no-cache"}) {
		t.Fatalf("CacheControlNoCacheHeader = %#v", got)
	}
}

func TestIfNoneMatchHeader(t *testing.T) {
	got, err := IfNoneMatchHeader(`"abc123"`)
	if err != nil {
		t.Fatalf("IfNoneMatchHeader: %v", err)
	}
	if got != (HeaderField{Name: "If-None-Match", Value: `"abc123"`}) {
		t.Fatalf("IfNoneMatchHeader = %#v", got)
	}
}

func TestIfNoneMatchHeaderRejectsLineBreak(t *testing.T) {
	_, err := IfNoneMatchHeader("\"abc\"\r\nX-Bad: true")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "line break") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIfModifiedSinceHeaderUsesHTTPDate(t *testing.T) {
	date := time.Date(2030, 1, 2, 3, 4, 5, 0, time.FixedZone("JST", 9*60*60))

	got := IfModifiedSinceHeader(date)
	if got != (HeaderField{Name: "If-Modified-Since", Value: "Tue, 01 Jan 2030 18:04:05 GMT"}) {
		t.Fatalf("IfModifiedSinceHeader = %#v", got)
	}
}
