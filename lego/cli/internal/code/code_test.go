/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package code

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

func TestCatalogShape(t *testing.T) {
	catalog := Catalog()
	if len(catalog) != 4 {
		t.Fatalf("catalog has %d providers, want 4", len(catalog))
	}
	seen := map[string]bool{}
	for _, p := range catalog {
		if seen[p.Name] {
			t.Errorf("duplicate provider name %q", p.Name)
		}
		seen[p.Name] = true
		if !strings.HasPrefix(p.BaseURL, "https://") || !strings.HasPrefix(p.ConsoleURL, "https://") {
			t.Errorf("%s: URLs must be https (%q, %q)", p.Name, p.BaseURL, p.ConsoleURL)
		}
		if len(p.KeyEnvs) == 0 || p.DefaultModel == "" || p.SmallFastModel == "" || p.DisplayName == "" {
			t.Errorf("%s: incomplete provider entry: %+v", p.Name, p)
		}
	}
	for _, name := range []string{"glm", "muse", "kimi", "deepseek"} {
		if !seen[name] {
			t.Errorf("catalog missing provider %q", name)
		}
	}
}

func TestBaseDirDefaultAndOverride(t *testing.T) {
	base, err := baseDir(lookupFrom(nil), func() (string, error) { return "/home/alice", nil })
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/home/alice", ".bex", "code"); base != want {
		t.Errorf("base = %q, want %q", base, want)
	}
	base, err = baseDir(lookupFrom(map[string]string{"BEX_CODE_HOME": "/tmp/code"}), func() (string, error) {
		t.Fatal("userHome consulted despite override")
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if base != "/tmp/code" {
		t.Errorf("base = %q, want /tmp/code", base)
	}
}

func TestStoredKeysRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.toml")
	in := map[string]string{"glm": `k"ey\x`, "deepseek": "ds", "legacy": "old"}
	if err := saveStoredKeys(path, in); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("permissions = %04o, want 0600", got)
	}
	out, err := loadStoredKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range in {
		if out[name] != want {
			t.Errorf("%s = %q, want %q", name, out[name], want)
		}
	}
}

func TestLoadStoredKeysMissingFileIsEmpty(t *testing.T) {
	keys, err := loadStoredKeys(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys=%v err=%v, want empty and nil", keys, err)
	}
}

func TestProviderKeyPrecedence(t *testing.T) {
	glm := *findProvider("glm")
	key, source := providerKey(glm, lookupFrom(map[string]string{"ZAI_API_KEY": "env-key"}), map[string]string{"glm": "stored-key"})
	if key != "env-key" || source != "ZAI_API_KEY" {
		t.Errorf("env should win: key=%q source=%q", key, source)
	}
	key, source = providerKey(glm, lookupFrom(map[string]string{"GLM_API_KEY": "fallback"}), nil)
	if key != "fallback" || source != "GLM_API_KEY" {
		t.Errorf("fallback env: key=%q source=%q", key, source)
	}
	key, source = providerKey(glm, lookupFrom(map[string]string{"ZAI_API_KEY": ""}), map[string]string{"glm": "stored-key"})
	if key != "stored-key" || source != "stored" {
		t.Errorf("empty env must fall through to store: key=%q source=%q", key, source)
	}
	if key, source = providerKey(glm, lookupFrom(nil), nil); key != "" || source != "" {
		t.Errorf("nothing configured: key=%q source=%q", key, source)
	}
}

func TestLaunchEnvIsolatesClaudeConfiguration(t *testing.T) {
	glm := *findProvider("glm")
	env := launchEnv([]string{
		"PATH=/bin",
		"CLAUDE_CONFIG_DIR=/home/alice/.claude",
		"ANTHROPIC_API_KEY=personal-anthropic-key",
		"ANTHROPIC_AUTH_TOKEN=inherited-token",
		"ANTHROPIC_MODEL=claude-opus",
	}, glm, "/home/alice/.bex/code/claude-glm", "glm-key")
	joined := strings.Join(env, "\n")
	for _, want := range []string{
		"CLAUDE_CONFIG_DIR=/home/alice/.bex/code/claude-glm",
		"ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic",
		"ANTHROPIC_AUTH_TOKEN=glm-key",
		"ANTHROPIC_MODEL=glm-5.2",
		"ANTHROPIC_SMALL_FAST_MODEL=glm-5-turbo",
		"ANTHROPIC_DEFAULT_OPUS_MODEL=glm-5.2",
		"ANTHROPIC_DEFAULT_SONNET_MODEL=glm-5.2",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL=glm-5-turbo",
		"PATH=/bin",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("env missing %q:\n%s", want, joined)
		}
	}
	for _, leak := range []string{"personal-anthropic-key", "inherited-token", "/home/alice/.claude", "claude-opus"} {
		if strings.Contains(joined, leak) {
			t.Errorf("inherited value leaked %q:\n%s", leak, joined)
		}
	}
}

func TestLaunchEnvWithoutKeyOmitsAuthToken(t *testing.T) {
	glm := *findProvider("glm")
	env := launchEnv([]string{"ANTHROPIC_AUTH_TOKEN=inherited"}, glm, "/dir", "")
	if strings.Contains(strings.Join(env, "\n"), "ANTHROPIC_AUTH_TOKEN") {
		t.Errorf("auth token present in keyless env: %v", env)
	}
}

func TestValidateKeyAcceptsAndRejects(t *testing.T) {
	var gotPath, gotAuth, gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("anthropic-version")
		if r.Header.Get("x-api-key") == "good" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	t.Cleanup(server.Close)
	p := *findProvider("glm")
	p.BaseURL = server.URL

	if err := validateKey(p, "good"); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotAuth != "Bearer good" {
		t.Errorf("authorization = %q", gotAuth)
	}
	if gotVersion != "2023-06-01" {
		t.Errorf("anthropic-version = %q", gotVersion)
	}
	err := validateKey(p, "bad")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("invalid key accepted or unclear error: %v", err)
	}
}

func TestValidateKeyToleratesNonAuthErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model overloaded", http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)
	p := *findProvider("glm")
	p.BaseURL = server.URL
	if err := validateKey(p, "any"); err != nil {
		t.Errorf("non-auth error must not fail validation: %v", err)
	}
}

func TestCommandsCoverHubAndAllProviders(t *testing.T) {
	commands := Commands()
	names := map[string]bool{}
	for _, c := range commands {
		names[strings.Fields(c.Use)[0]] = true
	}
	for _, want := range []string{"code", "glm", "muse", "kimi", "deepseek"} {
		if !names[want] {
			t.Errorf("missing command %q (have %v)", want, names)
		}
	}
}
