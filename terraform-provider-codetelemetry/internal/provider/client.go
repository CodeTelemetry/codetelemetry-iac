package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 2 * 1024 * 1024

type client struct {
	endpoint, token string
	binding         string
	http            *http.Client
}

func newClient(endpoint, token string) (*client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.RawPath != "" || u.ForceQuery || strings.Contains(endpoint, "#") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("endpoint must be an absolute service origin without credentials, path, query, or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return nil, fmt.Errorf("endpoint must use HTTPS (HTTP is allowed only for loopback development)")
	}
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return nil, fmt.Errorf("a nonempty Cognito ID token is required")
	}
	return &client{endpoint: u.Scheme + "://" + u.Host, token: token, http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// No retries: a timed-out write may already have committed. Never include remote
// bodies or transport errors in diagnostics: either can contain credentials.
func (c *client) request(ctx context.Context, method, path string, body, result any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cannot encode request")
		}
		if len(data) > 350000 {
			return fmt.Errorf("request exceeds 350000 bytes")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot construct request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Code Telemetry request failed; check connectivity, timeout, and credentials")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusConflict {
			return fmt.Errorf("Code Telemetry returned HTTP 409: configuration changed; refresh and plan again")
		}
		return fmt.Errorf("Code Telemetry returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("cannot read Code Telemetry response")
	}
	if len(raw) > maxResponseBytes {
		return fmt.Errorf("Code Telemetry response exceeds 2 MiB")
	}
	if result != nil && json.Unmarshal(raw, result) != nil {
		return fmt.Errorf("invalid Code Telemetry JSON response")
	}
	return nil
}
