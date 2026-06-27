package http1

import "testing"

func TestParseURLRequiresSupportedSchemeAndHost(t *testing.T) {
	tests := []string{
		"example.test/",
		"ftp://example.test/",
		"http:///hello",
		"http://user@example.test/",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseURL(input); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestURLHelpersDeriveAddressHostAndTarget(t *testing.T) {
	u, err := ParseURL("http://example.test:8080/search?q=hello%20world#ignored")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	address, err := TCPAddressForURL(u)
	if err != nil {
		t.Fatalf("TCPAddressForURL: %v", err)
	}
	if address != "example.test:8080" {
		t.Fatalf("address = %q", address)
	}

	host, err := HostHeaderForURL(u)
	if err != nil {
		t.Fatalf("HostHeaderForURL: %v", err)
	}
	if host != "example.test:8080" {
		t.Fatalf("host = %q", host)
	}

	target, err := RequestTargetForURL(u)
	if err != nil {
		t.Fatalf("RequestTargetForURL: %v", err)
	}
	if target != "/search?q=hello%20world" {
		t.Fatalf("target = %q", target)
	}

	absoluteTarget, err := AbsoluteRequestTargetForURL(u)
	if err != nil {
		t.Fatalf("AbsoluteRequestTargetForURL: %v", err)
	}
	if absoluteTarget != "http://example.test:8080/search?q=hello%20world" {
		t.Fatalf("absolute target = %q", absoluteTarget)
	}
}

func TestURLHelpersApplyDefaultPorts(t *testing.T) {
	tests := map[string]string{
		"http://example.test/":  "example.test:80",
		"https://example.test/": "example.test:443",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			u, err := ParseURL(input)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			got, err := TCPAddressForURL(u)
			if err != nil {
				t.Fatalf("TCPAddressForURL: %v", err)
			}
			if got != want {
				t.Fatalf("address = %q, want %q", got, want)
			}
		})
	}
}

func TestURLHelpersHandleMissingPathAsSlash(t *testing.T) {
	u, err := ParseURL("http://example.test")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	target, err := RequestTargetForURL(u)
	if err != nil {
		t.Fatalf("RequestTargetForURL: %v", err)
	}
	if target != "/" {
		t.Fatalf("target = %q", target)
	}

	absoluteTarget, err := AbsoluteRequestTargetForURL(u)
	if err != nil {
		t.Fatalf("AbsoluteRequestTargetForURL: %v", err)
	}
	if absoluteTarget != "http://example.test/" {
		t.Fatalf("absolute target = %q", absoluteTarget)
	}
}

func TestURLHelpersUseEscapedPath(t *testing.T) {
	u, err := ParseURL("http://example.test/a b")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	target, err := RequestTargetForURL(u)
	if err != nil {
		t.Fatalf("RequestTargetForURL: %v", err)
	}
	if target != "/a%20b" {
		t.Fatalf("target = %q", target)
	}
}
