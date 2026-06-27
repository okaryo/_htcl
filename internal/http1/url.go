package http1

import (
	"fmt"
	"net"
	"net/url"
)

func ParseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("URL scheme is required")
	}
	if _, ok := DefaultPort(u.Scheme); !ok {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if u.User != nil {
		return nil, fmt.Errorf("URL user info is not supported")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("URL host name is required")
	}
	return u, nil
}

func DefaultPort(scheme string) (string, bool) {
	switch scheme {
	case "http":
		return "80", true
	case "https":
		return "443", true
	default:
		return "", false
	}
}

func TCPAddressForURL(u *url.URL) (string, error) {
	if u == nil {
		return "", fmt.Errorf("URL is nil")
	}

	port := u.Port()
	if port == "" {
		defaultPort, ok := DefaultPort(u.Scheme)
		if !ok {
			return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
		}
		port = defaultPort
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("URL host name is required")
	}
	return net.JoinHostPort(host, port), nil
}

func HostHeaderForURL(u *url.URL) (string, error) {
	if u == nil {
		return "", fmt.Errorf("URL is nil")
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL host is required")
	}
	return u.Host, nil
}

func RequestTargetForURL(u *url.URL) (string, error) {
	if u == nil {
		return "", fmt.Errorf("URL is nil")
	}

	target := u.EscapedPath()
	if target == "" {
		target = "/"
	}
	if u.RawQuery != "" {
		target += "?" + u.RawQuery
	}
	return target, nil
}

func AbsoluteRequestTargetForURL(u *url.URL) (string, error) {
	if u == nil {
		return "", fmt.Errorf("URL is nil")
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("URL scheme is required")
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL host is required")
	}

	target, err := RequestTargetForURL(u)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host + target, nil
}
