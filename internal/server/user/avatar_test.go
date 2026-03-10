package user

import (
	"context"
	"net/http"
	"testing"
)

func TestNewGravatarRequestUsesFixedOrigin(t *testing.T) {
	req, err := newGravatarRequest(context.Background(), "Person@example.com", 128)
	if err != nil {
		t.Fatalf("newGravatarRequest() error = %v", err)
	}

	if req.Method != http.MethodGet {
		t.Fatalf("expected GET method, got %q", req.Method)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("expected https scheme, got %q", req.URL.Scheme)
	}
	if req.URL.Host != "www.gravatar.com" {
		t.Fatalf("expected gravatar host, got %q", req.URL.Host)
	}
	if req.URL.Path != "/avatar/"+gravatarHash("Person@example.com") {
		t.Fatalf("unexpected avatar path %q", req.URL.Path)
	}
	if got := req.URL.Query().Get("s"); got != "128" {
		t.Fatalf("expected size query to be 128, got %q", got)
	}
	if got := req.URL.Query().Get("d"); got != "mp" {
		t.Fatalf("expected default query to be mp, got %q", got)
	}
}
