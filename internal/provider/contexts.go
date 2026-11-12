package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type contextScope struct {
	Tenant  string  `json:"tenantId"`
	Team    *string `json:"teamId"`
	Billing string  `json:"billingAccountId"`
}
type localContext struct {
	Version  int          `json:"version"`
	Name     string       `json:"name"`
	Endpoint string       `json:"apiBase"`
	Scope    contextScope `json:"scope"`
	Key      string       `json:"ingestKey"`
}

var contextNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var organizationPattern = regexp.MustCompile(`^tm_[a-zA-Z0-9_-]{1,128}$`)

func readContextFile(file string, private bool) (*localContext, error) {
	absolute, err := filepath.Abs(file)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace path")
	}
	current := filepath.VolumeName(absolute) + string(os.PathSeparator)
	for _, segment := range strings.Split(strings.TrimPrefix(absolute, current), string(os.PathSeparator)) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("workspace context is unavailable")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("workspace paths cannot contain symbolic links")
		}
	}
	f, err := os.Open(absolute)
	if err != nil {
		return nil, fmt.Errorf("workspace context is unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 || (private && (!contextFileOwned(info) || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0))) {
		return nil, fmt.Errorf("workspace context must be a private regular file within 64 KiB")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(raw) > 65536 {
		return nil, fmt.Errorf("cannot read bounded workspace context")
	}
	var shape map[string]json.RawMessage
	var scopeShape map[string]json.RawMessage
	if json.Unmarshal(raw, &shape) != nil || json.Unmarshal(shape["scope"], &scopeShape) != nil {
		return nil, fmt.Errorf("invalid workspace scope")
	}
	if _, ok := scopeShape["teamId"]; !ok {
		return nil, fmt.Errorf("workspace scope must explicitly select personal or organization")
	}
	var value localContext
	if json.Unmarshal(raw, &value) != nil || value.Version != 1 || !contextNamePattern.MatchString(value.Name) || value.Scope.Tenant == "" || value.Scope.Billing == "" || len(value.Scope.Tenant) > 256 || len(value.Scope.Billing) > 256 || (value.Scope.Team != nil && !organizationPattern.MatchString(*value.Scope.Team)) {
		return nil, fmt.Errorf("invalid or unsupported workspace context; upgrade Connect and the provider")
	}
	return &value, nil
}
func repositoryContext() (*localContext, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for depth := 0; depth < 64; depth++ {
		file := filepath.Join(dir, ".codetelemetry", "workspace.json")
		if _, err := os.Lstat(file); err == nil {
			return readContextFile(file, false)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return nil, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
	return nil, fmt.Errorf("workspace binding search exceeded 64 parents")
}
func configuredClient(ctx context.Context, endpoint, token, organization, workspace string) (*client, error) {
	if organization != "" && !organizationPattern.MatchString(organization) {
		return nil, fmt.Errorf("organization_id must be an explicit tm_ organization ID")
	}
	binding, err := repositoryContext()
	if err != nil {
		return nil, err
	}
	if workspace == "" && binding != nil {
		workspace = binding.Name
	}
	if workspace == "" {
		c, err := newClient(endpoint, token)
		if err != nil {
			return nil, err
		}
		c.binding, err = workspaceBinding(c.endpoint, token)
		if err != nil {
			return nil, err
		}
		c.organization = organization
		if organization != "" {
			sum := sha256.Sum256([]byte(c.binding + "\n" + organization))
			c.binding = hex.EncodeToString(sum[:])
		}
		return c, nil
	}
	if !contextNamePattern.MatchString(workspace) {
		return nil, fmt.Errorf("invalid workspace name")
	}
	home := os.Getenv("CODETELEMETRY_CONTEXT_HOME")
	if home == "" {
		home = os.Getenv("XDG_CONFIG_HOME")
		if home == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			home = filepath.Join(userHome, ".config")
		}
		home = filepath.Join(home, "codetelemetry", "workspaces")
	}
	if !filepath.IsAbs(home) {
		return nil, fmt.Errorf("workspace context directory must be absolute")
	}
	saved, err := readContextFile(filepath.Join(home, workspace+".json"), true)
	if err != nil {
		return nil, err
	}
	if saved.Name != workspace || !strings.HasPrefix(saved.Key, "cot_") || len(saved.Key) < 24 || len(saved.Key) > 256 {
		return nil, fmt.Errorf("invalid workspace credentials")
	}
	team := ""
	if saved.Scope.Team != nil {
		team = *saved.Scope.Team
	}
	if (endpoint != "" && strings.TrimRight(endpoint, "/") != saved.Endpoint) || (organization != "" && organization != team) || (token != "" && token != saved.Key) {
		return nil, fmt.Errorf("explicit configuration conflicts with the saved workspace")
	}
	if binding != nil && (binding.Name != saved.Name || binding.Endpoint != saved.Endpoint || binding.Scope.Tenant != saved.Scope.Tenant || binding.Scope.Billing != saved.Scope.Billing || (binding.Scope.Team == nil) != (saved.Scope.Team == nil) || (binding.Scope.Team != nil && *binding.Scope.Team != team)) {
		return nil, fmt.Errorf("repository binding and saved workspace disagree")
	}
	c, err := newClient(saved.Endpoint, saved.Key)
	if err != nil {
		return nil, err
	}
	c.ingest = true
	c.expected = &saved.Scope
	c.organization = team
	var verified struct {
		State string       `json:"state"`
		Scope contextScope `json:"scope"`
	}
	// Budget exhaustion must not prevent configuring a policy. Scope is still verified.
	if err := c.verifyContext(ctx, saved.Scope, &verified); err != nil {
		return nil, err
	}
	if (verified.State != "scope_verified" && verified.State != "budget_exhausted") || verified.Scope.Tenant != saved.Scope.Tenant || verified.Scope.Billing != saved.Scope.Billing || (verified.Scope.Team == nil) != (saved.Scope.Team == nil) || (verified.Scope.Team != nil && *verified.Scope.Team != team) {
		return nil, fmt.Errorf("workspace credential scope changed; reconnect before applying")
	}
	raw, _ := json.Marshal([]string{c.endpoint, "context-v1", saved.Scope.Tenant, team, saved.Scope.Billing})
	sum := sha256.Sum256(raw)
	c.binding = hex.EncodeToString(sum[:])
	return c, nil
}
