package http1

import (
	"strings"
	"testing"
)

func TestBasicAuthorizationValue(t *testing.T) {
	got, err := BasicAuthorizationValue("alice", "secret")
	if err != nil {
		t.Fatalf("BasicAuthorizationValue: %v", err)
	}
	if got != "Basic YWxpY2U6c2VjcmV0" {
		t.Fatalf("BasicAuthorizationValue = %q", got)
	}
}

func TestBasicAuthorizationValueAllowsColonInPassword(t *testing.T) {
	got, err := BasicAuthorizationValue("alice", "sec:ret")
	if err != nil {
		t.Fatalf("BasicAuthorizationValue: %v", err)
	}
	if got != "Basic YWxpY2U6c2VjOnJldA==" {
		t.Fatalf("BasicAuthorizationValue = %q", got)
	}
}

func TestBasicAuthorizationValueRejectsLineBreaks(t *testing.T) {
	_, err := BasicAuthorizationValue("alice\n", "secret")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "username contains line break") {
		t.Fatalf("unexpected error: %v", err)
	}
}
