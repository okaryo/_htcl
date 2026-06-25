package http1

import "testing"

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
