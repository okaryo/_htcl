package http1

import (
	"fmt"
	"net/url"
	"strings"
)

type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	HostOnly bool
}

type CookieJar struct {
	cookies []Cookie
}

func ParseSetCookie(value string) (Cookie, error) {
	parts := strings.Split(value, ";")
	name, cookieValue, ok := strings.Cut(strings.TrimSpace(parts[0]), "=")
	if !ok {
		return Cookie{}, fmt.Errorf("Set-Cookie is missing name=value pair")
	}
	name = strings.TrimSpace(name)
	if err := validateCookieName(name); err != nil {
		return Cookie{}, err
	}
	cookie := Cookie{
		Name:  name,
		Value: strings.TrimSpace(cookieValue),
	}
	for _, part := range parts[1:] {
		attrName, attrValue, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(attrName)) {
		case "domain":
			cookie.Domain = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(attrValue), "."))
		case "path":
			path := strings.TrimSpace(attrValue)
			if strings.HasPrefix(path, "/") {
				cookie.Path = path
			}
		}
	}
	return cookie, nil
}

func CookiesFromSetCookieHeaders(fields []HeaderField) ([]Cookie, error) {
	var cookies []Cookie
	for _, field := range fields {
		if !strings.EqualFold(field.Name, "Set-Cookie") {
			continue
		}

		cookie, err := ParseSetCookie(field.Value)
		if err != nil {
			return nil, err
		}
		cookies = append(cookies, cookie)
	}
	return cookies, nil
}

func (r *Response) Cookies() ([]Cookie, error) {
	if r == nil {
		return nil, nil
	}
	return CookiesFromSetCookieHeaders(r.HeaderFields)
}

func CookieHeaderValue(cookies []Cookie) string {
	var pairs []string
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		pairs = append(pairs, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(pairs, "; ")
}

func (j *CookieJar) Store(cookies []Cookie) {
	if j == nil {
		return
	}
	for _, cookie := range cookies {
		j.store(cookie)
	}
}

func (j *CookieJar) StoreFromResponse(response *Response) error {
	cookies, err := response.Cookies()
	if err != nil {
		return err
	}
	j.Store(cookies)
	return nil
}

func (j *CookieJar) StoreFromResponseURL(response *Response, u *url.URL) error {
	cookies, err := response.Cookies()
	if err != nil {
		return err
	}
	j.StoreForURL(cookies, u)
	return nil
}

func (j *CookieJar) StoreForURL(cookies []Cookie, u *url.URL) {
	if j == nil {
		return
	}
	for _, cookie := range cookies {
		j.store(cookieForURL(cookie, u))
	}
}

func (j *CookieJar) Cookies() []Cookie {
	if j == nil {
		return nil
	}
	return append([]Cookie(nil), j.cookies...)
}

func (j *CookieJar) CookiesForURL(u *url.URL) []Cookie {
	if j == nil {
		return nil
	}
	var cookies []Cookie
	for _, cookie := range j.cookies {
		if cookieMatchesURL(cookie, u) {
			cookies = append(cookies, cookie)
		}
	}
	return cookies
}

func (j *CookieJar) HeaderValue() string {
	if j == nil {
		return ""
	}
	return CookieHeaderValue(j.cookies)
}

func (j *CookieJar) HeaderValueForURL(u *url.URL) string {
	if j == nil {
		return ""
	}
	return CookieHeaderValue(j.CookiesForURL(u))
}

func (j *CookieJar) store(next Cookie) {
	for i, cookie := range j.cookies {
		if cookie.Name == next.Name && cookie.Domain == next.Domain && cookie.Path == next.Path {
			j.cookies[i] = next
			return
		}
	}
	j.cookies = append(j.cookies, next)
}

func cookieForURL(cookie Cookie, u *url.URL) Cookie {
	if u == nil {
		return cookie
	}
	if cookie.Domain == "" {
		cookie.Domain = strings.ToLower(u.Hostname())
		cookie.HostOnly = true
	}
	if cookie.Path == "" {
		cookie.Path = defaultCookiePath(u)
	}
	return cookie
}

func cookieMatchesURL(cookie Cookie, u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if cookie.Domain != "" {
		domain := strings.ToLower(cookie.Domain)
		if cookie.HostOnly {
			if host != domain {
				return false
			}
		} else if host != domain && !strings.HasSuffix(host, "."+domain) {
			return false
		}
	}

	return cookiePathMatches(cookie.Path, u.EscapedPath())
}

func defaultCookiePath(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" || path[0] != '/' {
		return "/"
	}
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return "/"
	}
	return path[:index]
}

func cookiePathMatches(cookiePath, requestPath string) bool {
	if cookiePath == "" {
		return true
	}
	if requestPath == "" {
		requestPath = "/"
	}
	if cookiePath == "/" || requestPath == cookiePath {
		return true
	}
	if strings.HasPrefix(requestPath, cookiePath) {
		return strings.HasSuffix(cookiePath, "/") || requestPath[len(cookiePath)] == '/'
	}
	return false
}

func validateCookieName(name string) error {
	if name == "" {
		return fmt.Errorf("cookie name is empty")
	}
	if strings.ContainsAny(name, " \t\r\n;=") {
		return fmt.Errorf("cookie name %q is invalid", name)
	}
	return nil
}
