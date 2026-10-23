package provider

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// This is an accidental-scope-change guard, not JWT authentication. The service
// still verifies signatures, issuer, audience, expiry, and authorization. Read
// only stable identity claims so routine token renewal preserves resource state.
func workspaceBinding(endpoint, token string) (string, error) {
	const maxTokenBytes = 16384
	if len(token) > maxTokenBytes {
		return "", fmt.Errorf("Cognito ID token exceeds 16384 bytes")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return "", fmt.Errorf("a Cognito ID token with issuer, subject, and token_use=id is required")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid Cognito ID token claims")
	}
	// JWT claim names are exact. A struct decoder's case folding would allow
	// an unrelated SUB/ISS field to shadow the service's lowercase identity.
	var claims map[string]json.RawMessage
	if json.Unmarshal(raw, &claims) != nil {
		return "", fmt.Errorf("invalid Cognito ID token claims")
	}
	var issuer, subject, use string
	if json.Unmarshal(claims["iss"], &issuer) != nil || json.Unmarshal(claims["sub"], &subject) != nil || json.Unmarshal(claims["token_use"], &use) != nil || issuer == "" || subject == "" || use != "id" {
		return "", fmt.Errorf("a Cognito ID token with issuer, subject, and token_use=id is required")
	}
	scope, _ := json.Marshal([]string{endpoint, issuer, subject})
	digest := sha256.Sum256(scope)
	return hex.EncodeToString(digest[:]), nil
}

func (c *client) checkWorkspace(prior types.String) error {
	// Legacy states have no binding. Preserve readability and establish it on the
	// first refresh. Users must verify their original credentials before upgrading.
	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueString() != c.binding {
		return fmt.Errorf("workspace identity changed; refusing to read or write a different workspace. Restore the original endpoint and credentials, or use a separate state/stack and import intentionally")
	}
	return nil
}
