package middleware

import "testing"

func TestBearerToken(t *testing.T) {
	if bearerToken("Bearer abc123") != "abc123" {
		t.Fatal("expected token")
	}
	if bearerToken("") != "" {
		t.Fatal("empty header")
	}
	if bearerToken("Basic x") != "" {
		t.Fatal("wrong scheme")
	}
}
