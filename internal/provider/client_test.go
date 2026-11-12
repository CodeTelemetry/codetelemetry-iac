package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientEndpointValidation(t *testing.T) {
	for _, endpoint := range []string{"", "https://user:pass@example.com", "https://example.com/api", "https://example.com?token=secret", "http://example.com", "file:///tmp/config", "https://example.com?", "https://example.com#", "https://example.com/%2f", "https://:443"} {
		if _, err := newClient(endpoint, "token"); err == nil {
			t.Errorf("accepted %q", endpoint)
		}
	}
	for _, endpoint := range []string{"https://example.com", "http://localhost:3000", "http://127.0.0.1:3000", "http://[::1]:3000"} {
		if _, err := newClient(endpoint, "token"); err != nil {
			t.Errorf("rejected %q: %v", endpoint, err)
		}
	}
}
func TestClientBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"conflict", 409, "secret-body", "refresh and plan again"},
		{"unauthorized", 401, "secret-body", "HTTP 401"},
		{"malformed", 200, "secret-body", "invalid Code Telemetry JSON"},
		{"oversize", 200, strings.Repeat("x", maxResponseBytes+1), "exceeds 2 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Header.Get("Authorization") != "Bearer secret-token" {
					t.Error("missing authorization")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c, _ := newClient(server.URL, "secret-token")
			var result map[string]any
			err := c.request(context.Background(), "GET", "/api/alert-rules", nil, &result)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("diagnostic leaked response/token")
			}
		})
	}
}
func TestClientDoesNotFollowRedirects(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("followed redirect") }))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer origin.Close()
	c, _ := newClient(origin.URL, "token")
	if err := c.request(context.Background(), "GET", "/api/alert-rules", nil, nil); err == nil {
		t.Fatal("accepted redirect")
	}
}
func TestClientHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	c, _ := newClient(server.URL, "token")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := c.request(ctx, "GET", "/api/alert-rules", nil, nil); err == nil {
		t.Fatal("ignored deadline")
	}
}
