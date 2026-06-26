package http1

import (
	"strings"
	"testing"
)

func TestIsRedirectStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		want       bool
	}{
		{200, false},
		{300, false},
		{301, true},
		{302, true},
		{303, true},
		{304, false},
		{307, true},
		{308, true},
		{404, false},
	}

	for _, tt := range tests {
		if got := IsRedirectStatus(tt.statusCode); got != tt.want {
			t.Fatalf("IsRedirectStatus(%d) = %v, want %v", tt.statusCode, got, tt.want)
		}
	}
}

func TestHeaderValueMatchesCaseInsensitively(t *testing.T) {
	fields := []HeaderField{
		{Name: "Content-Type", Value: "text/plain"},
		{Name: "location", Value: "/next"},
	}

	got, ok := HeaderValue(fields, "Location")
	if !ok {
		t.Fatal("expected header value")
	}
	if got != "/next" {
		t.Fatalf("HeaderValue = %q", got)
	}
}

func TestRedirectLocationReturnsLocationForRedirectResponse(t *testing.T) {
	response := &Response{
		StatusCode: 302,
		HeaderFields: []HeaderField{
			{Name: "Location", Value: "/next"},
		},
	}

	got, ok := response.RedirectLocation()
	if !ok {
		t.Fatal("expected redirect location")
	}
	if got != "/next" {
		t.Fatalf("RedirectLocation = %q", got)
	}
}

func TestRedirectLocationRejectsNonRedirectResponse(t *testing.T) {
	response := &Response{
		StatusCode: 200,
		HeaderFields: []HeaderField{
			{Name: "Location", Value: "/next"},
		},
	}

	if got, ok := response.RedirectLocation(); ok {
		t.Fatalf("RedirectLocation = %q, want none", got)
	}
}

func TestRedirectLocationRequiresLocationHeader(t *testing.T) {
	response := &Response{
		StatusCode: 302,
	}

	if got, ok := response.RedirectLocation(); ok {
		t.Fatalf("RedirectLocation = %q, want none", got)
	}
}

func TestResolveRedirectURL(t *testing.T) {
	base, err := ParseURL("http://example.test/docs/page?old=1")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	tests := map[string]string{
		"next":                         "http://example.test/docs/next",
		"/login":                       "http://example.test/login",
		"?page=2":                      "http://example.test/docs/page?page=2",
		"https://other.test/new-place": "https://other.test/new-place",
	}

	for location, want := range tests {
		t.Run(location, func(t *testing.T) {
			got, err := ResolveRedirectURL(base, location)
			if err != nil {
				t.Fatalf("ResolveRedirectURL: %v", err)
			}
			if got.String() != want {
				t.Fatalf("ResolveRedirectURL = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestResolveRedirectURLRejectsEmptyLocation(t *testing.T) {
	base, err := ParseURL("http://example.test/")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	_, err = ResolveRedirectURL(base, "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "redirect Location is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveRedirectURLRejectsNilBaseURL(t *testing.T) {
	_, err := ResolveRedirectURL(nil, "/next")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "base URL is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedirectRequestBehavior(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		method     string
		wantMethod string
		wantBody   bool
		wantOK     bool
	}{
		{name: "301 GET", statusCode: 301, method: "GET", wantMethod: "GET", wantBody: false, wantOK: true},
		{name: "302 HEAD", statusCode: 302, method: "HEAD", wantMethod: "HEAD", wantBody: false, wantOK: true},
		{name: "303 POST", statusCode: 303, method: "POST", wantMethod: "GET", wantBody: false, wantOK: true},
		{name: "302 POST", statusCode: 302, method: "POST", wantMethod: "GET", wantBody: false, wantOK: true},
		{name: "307 POST", statusCode: 307, method: "POST", wantMethod: "POST", wantBody: true, wantOK: true},
		{name: "308 PATCH", statusCode: 308, method: "PATCH", wantMethod: "PATCH", wantBody: true, wantOK: true},
		{name: "200", statusCode: 200, method: "GET", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMethod, gotBody, gotOK := RedirectRequestBehavior(tt.statusCode, tt.method)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotMethod != tt.wantMethod {
				t.Fatalf("method = %q, want %q", gotMethod, tt.wantMethod)
			}
			if gotBody != tt.wantBody {
				t.Fatalf("keepBody = %v, want %v", gotBody, tt.wantBody)
			}
		})
	}
}
