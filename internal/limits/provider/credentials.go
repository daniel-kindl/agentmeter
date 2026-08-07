package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credential is one agent's bearer token and, where the vendor needs it, the
// account the token belongs to.
type Credential struct {
	Token   string
	Account string
}

// credentialPaths resolves where a token may live. An explicitly configured
// path replaces the search entirely: an operator who names a file means that
// file, and silently falling back to the agent's default would read a
// credential they did not point at.
func credentialPaths(configRoots []string, name, configured string) []string {
	if configured != "" {
		return []string{configured}
	}
	paths := make([]string, 0, len(configRoots))
	for _, root := range configRoots {
		paths = append(paths, filepath.Join(root, name))
	}
	return paths
}

// ClaudeCredential reads Claude Code's OAuth access token.
//
// The environment variable wins so an operator can supply a token without
// agentmeter touching the credentials file at all. On macOS Claude Code stores
// credentials in the keychain rather than on disk; that path is not read here,
// so those operators must use the environment variable.
func ClaudeCredential(configRoots []string, configuredPath string) func() (Credential, error) {
	return func() (Credential, error) {
		if token := strings.TrimSpace(os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")); token != "" {
			return Credential{Token: token}, nil
		}
		var missing []string
		for _, path := range credentialPaths(configRoots, ".credentials.json", configuredPath) {
			data, err := os.ReadFile(path) //nolint:gosec // an operator-configured agent config root
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, path)
				continue
			}
			if err != nil {
				return Credential{}, fmt.Errorf("read Claude credentials at %s", path)
			}
			var file struct {
				OAuth struct {
					AccessToken string `json:"accessToken"`
					ExpiresAt   int64  `json:"expiresAt"`
				} `json:"claudeAiOauth"`
			}
			if json.Unmarshal(data, &file) != nil {
				return Credential{}, fmt.Errorf("Claude credentials at %s are not readable JSON", path)
			}
			token := strings.TrimSpace(file.OAuth.AccessToken)
			if token == "" {
				return Credential{}, fmt.Errorf("Claude credentials at %s contain no access token", path)
			}
			// agentmeter deliberately does not perform a refresh grant: that
			// would rewrite the operator's credentials file behind their back.
			if file.OAuth.ExpiresAt > 0 && time.UnixMilli(file.OAuth.ExpiresAt).Before(time.Now()) {
				return Credential{}, errors.New("Claude credentials have expired: run Claude Code to refresh them")
			}
			return Credential{Token: token}, nil
		}
		return Credential{}, fmt.Errorf("no Claude credentials found (looked in %s); set CLAUDE_CODE_OAUTH_TOKEN to supply one",
			strings.Join(missing, ", "))
	}
}

// CodexCredential reads the Codex CLI's ChatGPT access token and account.
func CodexCredential(configRoot, configuredPath string) func() (Credential, error) {
	return func() (Credential, error) {
		path := filepath.Join(configRoot, "auth.json")
		if configuredPath != "" {
			path = configuredPath
		}
		data, err := os.ReadFile(path) //nolint:gosec // an operator-configured agent config root
		if errors.Is(err, os.ErrNotExist) {
			return Credential{}, fmt.Errorf("no Codex credentials at %s: run codex login", path)
		}
		if err != nil {
			return Credential{}, fmt.Errorf("read Codex credentials at %s", path)
		}
		var file struct {
			Tokens struct {
				AccessToken string `json:"access_token"`
				AccountID   string `json:"account_id"`
			} `json:"tokens"`
		}
		if json.Unmarshal(data, &file) != nil {
			return Credential{}, fmt.Errorf("Codex credentials at %s are not readable JSON", path)
		}
		token := strings.TrimSpace(file.Tokens.AccessToken)
		if token == "" {
			// An API-key-only auth.json is valid for running Codex but carries
			// no subscription the usage endpoint can report on.
			return Credential{}, fmt.Errorf("Codex credentials at %s hold no ChatGPT access token: run codex login to sign in with a ChatGPT account", path)
		}
		return Credential{Token: token, Account: strings.TrimSpace(file.Tokens.AccountID)}, nil
	}
}
