package http1

import "testing"

func TestIsIdempotentMethod(t *testing.T) {
	tests := map[string]bool{
		"GET":     true,
		"HEAD":    true,
		"PUT":     true,
		"DELETE":  true,
		"OPTIONS": true,
		"TRACE":   true,
		"get":     true,
		"POST":    false,
		"PATCH":   false,
		"CONNECT": false,
		"CUSTOM":  false,
	}

	for method, want := range tests {
		t.Run(method, func(t *testing.T) {
			if got := IsIdempotentMethod(method); got != want {
				t.Fatalf("IsIdempotentMethod(%q) = %v, want %v", method, got, want)
			}
		})
	}
}
