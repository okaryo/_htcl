package http1

import (
	"fmt"
	"strings"
)

type Cookie struct {
	Name  string
	Value string
}

type CookieJar struct {
	cookies []Cookie
}

func ParseSetCookie(value string) (Cookie, error) {
	pair, _, _ := strings.Cut(value, ";")
	name, cookieValue, ok := strings.Cut(strings.TrimSpace(pair), "=")
	if !ok {
		return Cookie{}, fmt.Errorf("Set-Cookie is missing name=value pair")
	}
	name = strings.TrimSpace(name)
	if err := validateCookieName(name); err != nil {
		return Cookie{}, err
	}
	return Cookie{
		Name:  name,
		Value: strings.TrimSpace(cookieValue),
	}, nil
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

func (j *CookieJar) Cookies() []Cookie {
	if j == nil {
		return nil
	}
	return append([]Cookie(nil), j.cookies...)
}

func (j *CookieJar) HeaderValue() string {
	if j == nil {
		return ""
	}
	return CookieHeaderValue(j.cookies)
}

func (j *CookieJar) store(next Cookie) {
	for i, cookie := range j.cookies {
		if cookie.Name == next.Name {
			j.cookies[i] = next
			return
		}
	}
	j.cookies = append(j.cookies, next)
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
