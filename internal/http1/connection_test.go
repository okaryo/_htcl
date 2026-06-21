package http1

import "testing"

func TestShouldCloseConnectionForHTTP11DefaultsToReusable(t *testing.T) {
	response := &Response{
		Version: "HTTP/1.1",
	}

	if response.ShouldCloseConnection() {
		t.Fatal("HTTP/1.1 without Connection: close should be reusable")
	}
}

func TestShouldCloseConnectionHonorsCloseToken(t *testing.T) {
	response := &Response{
		Version: "HTTP/1.1",
		HeaderFields: []HeaderField{
			{Name: "Connection", Value: "upgrade, close"},
		},
	}

	if !response.ShouldCloseConnection() {
		t.Fatal("Connection: close should close the connection")
	}
}

func TestShouldCloseConnectionForHTTP10DefaultsToClose(t *testing.T) {
	response := &Response{
		Version: "HTTP/1.0",
	}

	if !response.ShouldCloseConnection() {
		t.Fatal("HTTP/1.0 without keep-alive should close the connection")
	}
}

func TestShouldCloseConnectionHonorsHTTP10KeepAlive(t *testing.T) {
	response := &Response{
		Version: "HTTP/1.0",
		HeaderFields: []HeaderField{
			{Name: "Connection", Value: "keep-alive"},
		},
	}

	if response.ShouldCloseConnection() {
		t.Fatal("HTTP/1.0 with Connection: keep-alive should be reusable")
	}
}

func TestHasConnectionTokenIsCaseInsensitive(t *testing.T) {
	fields := []HeaderField{
		{Name: "connection", Value: "Keep-Alive, Upgrade"},
	}

	if !HasConnectionToken(fields, "keep-alive") {
		t.Fatal("expected keep-alive token")
	}
	if !HasConnectionToken(fields, "upgrade") {
		t.Fatal("expected upgrade token")
	}
}
