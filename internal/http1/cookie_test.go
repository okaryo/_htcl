package http1

import (
	"strings"
	"testing"
)

func TestParseSetCookieReadsNameValueBeforeAttributes(t *testing.T) {
	cookie, err := ParseSetCookie("session=abc123; Path=/; HttpOnly")
	if err != nil {
		t.Fatalf("ParseSetCookie: %v", err)
	}
	if cookie.Name != "session" {
		t.Fatalf("Name = %q", cookie.Name)
	}
	if cookie.Value != "abc123" {
		t.Fatalf("Value = %q", cookie.Value)
	}
}

func TestParseSetCookieAllowsEmptyValue(t *testing.T) {
	cookie, err := ParseSetCookie("session=; Max-Age=0")
	if err != nil {
		t.Fatalf("ParseSetCookie: %v", err)
	}
	if cookie.Name != "session" {
		t.Fatalf("Name = %q", cookie.Name)
	}
	if cookie.Value != "" {
		t.Fatalf("Value = %q", cookie.Value)
	}
}

func TestParseSetCookieRejectsInvalidPair(t *testing.T) {
	_, err := ParseSetCookie("HttpOnly")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "missing name=value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSetCookieRejectsInvalidName(t *testing.T) {
	_, err := ParseSetCookie("bad name=value")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "cookie name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCookiesFromSetCookieHeaders(t *testing.T) {
	fields := []HeaderField{
		{Name: "Content-Type", Value: "text/plain"},
		{Name: "Set-Cookie", Value: "session=abc123; Path=/; HttpOnly"},
		{Name: "set-cookie", Value: "theme=dark; Path=/"},
	}

	cookies, err := CookiesFromSetCookieHeaders(fields)
	if err != nil {
		t.Fatalf("CookiesFromSetCookieHeaders: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("len(cookies) = %d", len(cookies))
	}
	if cookies[0] != (Cookie{Name: "session", Value: "abc123"}) {
		t.Fatalf("cookies[0] = %#v", cookies[0])
	}
	if cookies[1] != (Cookie{Name: "theme", Value: "dark"}) {
		t.Fatalf("cookies[1] = %#v", cookies[1])
	}
}

func TestCookieHeaderValue(t *testing.T) {
	got := CookieHeaderValue([]Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "theme", Value: "dark"},
	})
	if got != "session=abc123; theme=dark" {
		t.Fatalf("CookieHeaderValue = %q", got)
	}
}

func TestCookieJarStoresAndReplacesCookiesByName(t *testing.T) {
	var jar CookieJar
	jar.Store([]Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "theme", Value: "dark"},
	})
	jar.Store([]Cookie{
		{Name: "session", Value: "def456"},
	})

	got := jar.Cookies()
	if len(got) != 2 {
		t.Fatalf("len(cookies) = %d", len(got))
	}
	if got[0] != (Cookie{Name: "session", Value: "def456"}) {
		t.Fatalf("cookies[0] = %#v", got[0])
	}
	if got[1] != (Cookie{Name: "theme", Value: "dark"}) {
		t.Fatalf("cookies[1] = %#v", got[1])
	}
	if header := jar.HeaderValue(); header != "session=def456; theme=dark" {
		t.Fatalf("HeaderValue = %q", header)
	}
}

func TestCookieJarStoresCookiesFromResponse(t *testing.T) {
	response := &Response{
		HeaderFields: []HeaderField{
			{Name: "Set-Cookie", Value: "session=abc123; Path=/"},
			{Name: "Set-Cookie", Value: "theme=dark; Path=/"},
		},
	}

	var jar CookieJar
	if err := jar.StoreFromResponse(response); err != nil {
		t.Fatalf("StoreFromResponse: %v", err)
	}
	if got := jar.HeaderValue(); got != "session=abc123; theme=dark" {
		t.Fatalf("HeaderValue = %q", got)
	}
}

func TestCookieJarCookiesReturnsCopy(t *testing.T) {
	var jar CookieJar
	jar.Store([]Cookie{{Name: "session", Value: "abc123"}})

	cookies := jar.Cookies()
	cookies[0] = Cookie{Name: "session", Value: "changed"}

	if got := jar.HeaderValue(); got != "session=abc123" {
		t.Fatalf("HeaderValue = %q", got)
	}
}
