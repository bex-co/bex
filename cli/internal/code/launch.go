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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// launch resolves the provider key (environment → stored → first-launch
// capture), then replaces this process with Claude Code pointed at the
// provider, in an isolated per-provider configuration directory.
func launch(p Provider, args []string, errOut io.Writer) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("Claude Code is not installed (no `claude` on PATH); install it with `npm install -g @anthropic-ai/claude-code` or see https://claude.com/claude-code")
	}

	base, err := baseDir(os.LookupEnv, os.UserHomeDir)
	if err != nil {
		return err
	}
	configDir := filepath.Join(base, "claude-"+p.Name)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", configDir, err)
	}

	// Informational invocations need no credential; hand them straight
	// through with the isolated configuration directory.
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "-v", "--version":
			return execClaude(claudePath, args, launchEnv(os.Environ(), p, configDir, ""))
		}
	}

	stored, err := loadStoredKeys(keysPath(base))
	if err != nil {
		return err
	}
	key, _ := providerKey(p, os.LookupEnv, stored)
	if key == "" {
		key, err = captureKey(p, keysPath(base), errOut)
		if err != nil {
			return err
		}
	}

	return execClaude(claudePath, args, launchEnv(os.Environ(), p, configDir, key))
}

// launchEnv builds the child environment: the provider's Anthropic-compatible
// endpoint, the isolated configuration directory, and the model slots.
// Inherited values of everything being set — plus ANTHROPIC_API_KEY, which
// would fight the injected token — are stripped so the personal Claude Code
// setup cannot leak in.
func launchEnv(environment []string, p Provider, configDir, key string) []string {
	sets := [][2]string{
		{"CLAUDE_CONFIG_DIR", configDir},
		{"ANTHROPIC_BASE_URL", p.BaseURL},
		{"ANTHROPIC_MODEL", p.DefaultModel},
		{"ANTHROPIC_SMALL_FAST_MODEL", p.SmallFastModel},
		{"ANTHROPIC_DEFAULT_OPUS_MODEL", p.DefaultModel},
		{"ANTHROPIC_DEFAULT_SONNET_MODEL", p.DefaultModel},
		{"ANTHROPIC_DEFAULT_HAIKU_MODEL", p.SmallFastModel},
	}
	if key != "" {
		sets = append(sets, [2]string{"ANTHROPIC_AUTH_TOKEN", key})
	}
	drop := map[string]bool{"ANTHROPIC_API_KEY": true, "ANTHROPIC_AUTH_TOKEN": true}
	for _, kv := range sets {
		drop[kv[0]] = true
	}
	out := make([]string, 0, len(environment)+len(sets))
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if drop[name] {
			continue
		}
		out = append(out, item)
	}
	for _, kv := range sets {
		out = append(out, kv[0]+"="+kv[1])
	}
	return out
}

func execClaude(claudePath string, args []string, env []string) error {
	if err := syscall.Exec(claudePath, append([]string{"claude"}, args...), env); err != nil {
		return fmt.Errorf("exec %s: %w", claudePath, err)
	}
	return nil
}

// captureKey is the first-launch flow: open the provider's key console, take
// one hidden paste, verify it live against the provider, store it owner-only.
// Prompts go to stderr so they never mix with piped output.
func captureKey(p Provider, storePath string, errOut io.Writer) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("no %s API key configured; run `bex code keys set %s` (or set one of %s)",
			p.DisplayName, p.Name, strings.Join(p.KeyEnvs, ", "))
	}
	fmt.Fprintf(errOut, "No %s API key configured. Create one at:\n  %s\n", p.DisplayName, p.ConsoleURL)
	openBrowser(p.ConsoleURL)
	key, err := readSecret(errOut, fmt.Sprintf("Paste the %s API key (input hidden): ", p.Name))
	if err != nil {
		return "", err
	}
	if err := verifyAndStoreKey(p, storePath, key, errOut); err != nil {
		return "", err
	}
	return key, nil
}

// verifyAndStoreKey validates the key live, then persists it.
func verifyAndStoreKey(p Provider, storePath, key string, errOut io.Writer) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	fmt.Fprintf(errOut, "Verifying against %s… ", p.BaseURL)
	if err := validateKey(p, key); err != nil {
		fmt.Fprintln(errOut, "✗")
		return err
	}
	fmt.Fprintln(errOut, "✓")
	stored, err := loadStoredKeys(storePath)
	if err != nil {
		return err
	}
	stored[p.Name] = key
	if err := saveStoredKeys(storePath, stored); err != nil {
		return err
	}
	fmt.Fprintf(errOut, "Stored in %s — the environment (%s) overrides it any time.\n",
		storePath, strings.Join(p.KeyEnvs, ", "))
	return nil
}

// validateKey sends one minimal Messages request. Only an explicit
// authentication rejection fails validation; any other response proves the
// credential was accepted (model or quota errors surface later, verbatim,
// inside Claude Code).
func validateKey(p Provider, key string) error {
	body, err := json.Marshal(map[string]any{
		"model":      p.DefaultModel,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("x-api-key", key)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("reach %s: %w", p.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("%s rejected the key (HTTP %d): %s", p.DisplayName, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// readSecret reads a credential without echoing when stdin is a terminal,
// falling back to a plain line read for piped input.
func readSecret(errOut io.Writer, prompt string) (string, error) {
	fmt.Fprint(errOut, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(errOut)
		if err != nil {
			return "", fmt.Errorf("read key: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("read key: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// openBrowser is best-effort; the URL is already printed for manual use.
func openBrowser(url string) {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "linux":
		name = "xdg-open"
	default:
		return
	}
	if path, err := exec.LookPath(name); err == nil {
		_ = exec.Command(path, url).Start()
	}
}
