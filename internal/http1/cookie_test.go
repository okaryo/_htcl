package http1

import (
	"strings"
	"testing"
	"time"
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
	if cookie.Path != "/" {
		t.Fatalf("Path = %q", cookie.Path)
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
	if cookie.MaxAge == nil || *cookie.MaxAge != 0 {
		t.Fatalf("MaxAge = %#v", cookie.MaxAge)
	}
}

func TestParseSetCookieReadsExpires(t *testing.T) {
	cookie, err := ParseSetCookie("session=abc123; Expires=Wed, 21 Oct 2037 07:28:00 UTC")
	if err != nil {
		t.Fatalf("ParseSetCookie: %v", err)
	}

	want := time.Date(2037, 10, 21, 7, 28, 0, 0, time.UTC)
	if !cookie.Expires.Equal(want) {
		t.Fatalf("Expires = %s, want %s", cookie.Expires, want)
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
	if cookies[0] != (Cookie{Name: "session", Value: "abc123", Path: "/"}) {
		t.Fatalf("cookies[0] = %#v", cookies[0])
	}
	if cookies[1] != (Cookie{Name: "theme", Value: "dark", Path: "/"}) {
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

func TestCookieJarStoresCookiesWithDefaultDomainAndPath(t *testing.T) {
	u, err := ParseURL("http://example.test/account/profile")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	var jar CookieJar
	jar.StoreForURL([]Cookie{{Name: "session", Value: "abc123"}}, u)

	cookies := jar.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("len(cookies) = %d", len(cookies))
	}
	if cookies[0].Domain != "example.test" {
		t.Fatalf("Domain = %q", cookies[0].Domain)
	}
	if !cookies[0].HostOnly {
		t.Fatal("expected host-only cookie")
	}
	if cookies[0].Path != "/account" {
		t.Fatalf("Path = %q", cookies[0].Path)
	}
}

func TestCookieJarCookiesForURLMatchesDomainAndPath(t *testing.T) {
	var jar CookieJar
	jar.Store([]Cookie{
		{Name: "host", Value: "only", Domain: "example.test", Path: "/", HostOnly: true},
		{Name: "domain", Value: "wide", Domain: "example.test", Path: "/account"},
		{Name: "other", Value: "host", Domain: "other.test", Path: "/"},
		{Name: "otherpath", Value: "nope", Domain: "example.test", Path: "/settings"},
	})

	u, err := ParseURL("http://sub.example.test/account/profile")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	if got := jar.HeaderValueForURL(u); got != "domain=wide" {
		t.Fatalf("HeaderValueForURL = %q", got)
	}
}

func TestCookieJarCookiesForURLMatchesHostOnlyCookie(t *testing.T) {
	var jar CookieJar
	jar.Store([]Cookie{
		{Name: "session", Value: "abc123", Domain: "example.test", Path: "/", HostOnly: true},
	})

	u, err := ParseURL("http://example.test/account")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	if got := jar.HeaderValueForURL(u); got != "session=abc123" {
		t.Fatalf("HeaderValueForURL = %q", got)
	}
}

func TestCookieJarSkipsExpiredCookies(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	jar := CookieJar{
		now: func() time.Time {
			return now
		},
	}
	jar.Store([]Cookie{
		{Name: "old", Value: "gone", Domain: "example.test", Path: "/", Expires: now.Add(-time.Hour)},
		{Name: "fresh", Value: "ok", Domain: "example.test", Path: "/", Expires: now.Add(time.Hour)},
	})

	u, err := ParseURL("http://example.test/")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	if got := jar.HeaderValueForURL(u); got != "fresh=ok" {
		t.Fatalf("HeaderValueForURL = %q", got)
	}
}

func TestCookieJarDeletesCookieWithMaxAgeZero(t *testing.T) {
	u, err := ParseURL("http://example.test/")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	var jar CookieJar
	jar.StoreForURL([]Cookie{{Name: "session", Value: "abc123", Path: "/"}}, u)

	zero := 0
	jar.StoreForURL([]Cookie{{Name: "session", Value: "", Path: "/", MaxAge: &zero}}, u)

	if got := jar.HeaderValueForURL(u); got != "" {
		t.Fatalf("HeaderValueForURL = %q", got)
	}
	if cookies := jar.Cookies(); len(cookies) != 0 {
		t.Fatalf("len(cookies) = %d", len(cookies))
	}
}

func TestCookieJarExpiresPositiveMaxAgeAfterStoredDuration(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	jar := CookieJar{
		now: func() time.Time {
			return now
		},
	}
	u, err := ParseURL("http://example.test/")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	maxAge := 3600
	jar.StoreForURL([]Cookie{{Name: "session", Value: "abc123", Path: "/", MaxAge: &maxAge}}, u)

	if got := jar.HeaderValueForURL(u); got != "session=abc123" {
		t.Fatalf("HeaderValueForURL before expiry = %q", got)
	}

	now = now.Add(2 * time.Hour)
	if got := jar.HeaderValueForURL(u); got != "" {
		t.Fatalf("HeaderValueForURL after expiry = %q", got)
	}
}

func TestCookieJarMaxAgeOverridesExpires(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	jar := CookieJar{
		now: func() time.Time {
			return now
		},
	}
	u, err := ParseURL("http://example.test/")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	maxAge := 3600
	jar.StoreForURL([]Cookie{{
		Name:    "session",
		Value:   "abc123",
		Path:    "/",
		Expires: now.Add(-time.Hour),
		MaxAge:  &maxAge,
	}}, u)

	if got := jar.HeaderValueForURL(u); got != "session=abc123" {
		t.Fatalf("HeaderValueForURL = %q", got)
	}
}
