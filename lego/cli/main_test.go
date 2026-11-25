package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/cli/internal/bridge"
	"github.com/bex-co/bex/lego/cli/internal/code"
)

var bexBinary string

// testBexVersion and testUpstreamVersion are injected into the test binary
// the same way cli-release.yml and scripts/bex-cli-build.sh inject them.
const (
	testBexVersion      = "1.2.3"
	testUpstreamVersion = "2.22.0"
)

func TestMain(m *testing.M) {
	// The launcher test-binary must never emit telemetry at the harness
	// itself: every child below inherits this process's environment, and an
	// emission would phone the test's stub API (or worse, the default
	// api.bex.co when a test sets no BEX_HOST). The dedicated sending test
	// overrides this back to blank (bridge: blank counts as unset).
	if err := os.Setenv("BEX_CLI_DISABLE_ANALYTICS", "1"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "bex-cli-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	bexBinary = filepath.Join(dir, "bex")
	build := exec.Command("go", "build",
		"-ldflags", "-X main.bexVersion="+testBexVersion+" -X github.com/render-oss/cli/pkg/cfg.Version="+testUpstreamVersion,
		"-o", bexBinary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build bex: %v\n%s", err, output)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestBexBinaryUsesBexConfigurationForRequests(t *testing.T) {
	var gotPath, gotAuthorization string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/owners" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
	}))
	t.Cleanup(api.Close)

	binary := buildBex()

	home := t.TempDir()
	command := exec.Command(binary, "workspaces", "-o", "json")
	command.Env = append(withoutRenderEnv(os.Environ()),
		"HOME="+home,
		"BEX_HOST="+api.URL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("bex workspaces: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if gotPath != "/v1/owners" {
		t.Errorf("request path = %q, want /v1/owners", gotPath)
	}
	if gotAuthorization != "Bearer test-access-token" {
		t.Errorf("authorization = %q", gotAuthorization)
	}
	if !strings.Contains(stdout.String(), "tea-bex") {
		t.Errorf("output did not include workspace: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".render", "cli.yaml")); !os.IsNotExist(err) {
		t.Errorf("Render config was unexpectedly created: %v", err)
	}
}

// w7/m426: an API key is one variable, as RENDER_API_KEY is for Render's CLI.
// The launcher sends it as the bearer with no login and no exchange: the only
// request is the command's own. Highest first: the user's own RENDER_API_KEY,
// BEX_API_KEY, BEX_ACCESS_TOKEN, then a stored login.
func TestBexBinarySendsAnAPIKeyAsTheBearer(t *testing.T) {
	const (
		bexKey    = "BEX_API_KEY=bex_test-key-id_test-key-secret"
		bexToken  = "BEX_ACCESS_TOKEN=test-access-token"
		renderKey = "RENDER_API_KEY=rnd_user"
		keyBearer = "Bearer bex_test-key-id_test-key-secret"
	)
	for _, tc := range []struct {
		name   string
		stored bool // a stored login, which every environment credential outranks
		env    []string
		want   string
	}{
		{name: "an API key alone", env: []string{bexKey}, want: keyBearer},
		{name: "an API key outranks a stored login", stored: true, env: []string{bexKey}, want: keyBearer},
		{name: "an API key outranks an access token", env: []string{bexToken, bexKey}, want: keyBearer},
		{name: "a blank API key leaves the access token in force", env: []string{bexToken, "BEX_API_KEY="}, want: "Bearer test-access-token"},
		{name: "the user's own upstream key outranks both", env: []string{bexToken, bexKey, renderKey}, want: "Bearer rnd_user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
			}))
			t.Cleanup(api.Close)

			home := t.TempDir()
			if tc.stored {
				if err := os.MkdirAll(filepath.Join(home, ".bex"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".bex", "cli.yaml"), []byte("version: 1\napi:\n  key: stored-bex-token\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(buildBex(), "workspaces", "-o", "json")
			command.Env = append(withoutRenderEnv(os.Environ()), "HOME="+home, "BEX_HOST="+api.URL+"/v1/", "BEX_CLI_DISABLE_ANALYTICS=1")
			command.Env = append(command.Env, tc.env...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("bex workspaces: %v\n%s", err, output)
			}
			if want := []string{"GET /v1/owners " + tc.want}; !slices.Equal(requests, want) {
				t.Errorf("requests = %q, want %q", requests, want)
			}
			if !strings.Contains(string(output), "tea-bex") {
				t.Errorf("output did not include the workspace: %s", output)
			}
		})
	}
}

func TestBexBinaryReadsStoredBexConfig(t *testing.T) {
	var gotAuthorization string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/owners" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
	}))
	t.Cleanup(api.Close)

	binary := buildBex()

	home := t.TempDir()
	configPath := filepath.Join(home, ".bex", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "version: 1\napi:\n  key: stored-bex-token\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	renderConfigPath := filepath.Join(home, ".render", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(renderConfigPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const renderConfig = "version: 1\napi:\n  key: render-token-must-not-be-read\n"
	if err := os.WriteFile(renderConfigPath, []byte(renderConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(binary, "workspaces", "-o", "json")
	command.Env = append(withoutRenderEnv(os.Environ()), "HOME="+home, "BEX_HOST="+api.URL+"/v1/")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	if gotAuthorization != "Bearer stored-bex-token" {
		t.Errorf("authorization = %q", gotAuthorization)
	}
	if got, err := os.ReadFile(renderConfigPath); err != nil || string(got) != renderConfig {
		t.Errorf("Render config was read or changed: content=%q err=%v", got, err)
	}
}

func TestBexBinaryMissingCredentialsDoesNotFallBackToRenderConfig(t *testing.T) {
	binary := buildBex()
	home := t.TempDir()
	renderConfigPath := filepath.Join(home, ".render", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(renderConfigPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const renderConfig = "version: 1\napi:\n  key: render-token-must-not-be-read\n"
	if err := os.WriteFile(renderConfigPath, []byte(renderConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(binary, "workspaces", "-o", "json")
	command.Env = append(withoutRenderEnv(os.Environ()), "HOME="+home)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("bex workspaces unexpectedly succeeded without Bex credentials")
	}
	// Run-time command references name bex (w2/m263 D4=A).
	if !strings.Contains(string(output), "run `bex login` to authenticate") {
		t.Errorf("missing credential output = %q", output)
	}
	if got, err := os.ReadFile(renderConfigPath); err != nil || string(got) != renderConfig {
		t.Errorf("Render config was read or changed: content=%q err=%v", got, err)
	}
}

func TestBexBinaryDeviceLoginRefreshAndLogoutStayInBexConfig(t *testing.T) {
	var refreshed, revoked bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/device-grant":
			_, _ = w.Write([]byte(`{"device_code":"device","user_code":"BEX-123","verification_uri":"https://dashboard.example.test/auth/device","verification_uri_complete":"https://dashboard.example.test/auth/device?user_code=BEX-123","expires_in":30,"interval":1}`))
		case "/v1/device-token":
			_, _ = w.Write([]byte(`{"access_token":"initial-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"initial-refresh-token"}`))
		case "/v1/token/refresh/":
			refreshed = true
			_, _ = w.Write([]byte(`{"access_token":"refreshed-access-token","token_type":"Bearer","expires_in":3600,"refresh_token":"refreshed-refresh-token"}`))
		case "/v1/owners":
			if r.Header.Get("Authorization") != "Bearer refreshed-access-token" {
				http.Error(w, "unexpected bearer", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
		case "/v1/oauth/revoke":
			if r.Header.Get("Authorization") != "Bearer refreshed-access-token" {
				http.Error(w, "unexpected bearer", http.StatusUnauthorized)
				return
			}
			revoked = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	binary := buildBex()

	home := t.TempDir()
	renderConfigPath := filepath.Join(home, ".render", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(renderConfigPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const renderConfig = "render-config-must-not-be-used\n"
	if err := os.WriteFile(renderConfigPath, []byte(renderConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	openerDir := t.TempDir()
	opener := filepath.Join(openerDir, "open")
	if err := os.WriteFile(opener, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	environment := append(withoutRenderEnv(os.Environ()), "HOME="+home, "BEX_HOST="+api.URL+"/v1/", "PATH="+openerDir+":"+os.Getenv("PATH"))
	login := exec.Command(binary, "login")
	login.Env = environment
	loginOutput, err := login.CombinedOutput()
	if err != nil {
		t.Fatalf("bex login: %v\n%s", err, loginOutput)
	}
	if strings.Contains(string(loginOutput), "initial-access-token") || strings.Contains(string(loginOutput), "initial-refresh-token") {
		t.Fatalf("bex login printed credential material: %s", loginOutput)
	}

	configPath := filepath.Join(home, ".bex", "cli.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Bex config missing: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("Bex config permissions = %04o, want %04o", got, want)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "initial-refresh-token") {
		t.Fatal("Bex config did not store refresh token")
	}
	expiredConfig := regexp.MustCompile(`(?m)^  expires_at: \d+$`).ReplaceAllString(string(config), "  expires_at: 1")
	if err := os.WriteFile(configPath, []byte(expiredConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	workspaces := exec.Command(binary, "workspaces", "-o", "json")
	workspaces.Env = environment
	if output, err := workspaces.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces after refresh: %v\n%s", err, output)
	}
	if !refreshed {
		t.Error("expired Bex config did not refresh")
	}
	config, err = os.ReadFile(configPath)
	if err != nil || !strings.Contains(string(config), "refreshed-refresh-token") {
		t.Errorf("refreshed config = %q, err=%v", config, err)
	}

	logout := exec.Command(binary, "logout")
	logout.Env = environment
	output, err := logout.CombinedOutput()
	if err != nil {
		t.Fatalf("bex logout: %v\n%s", err, output)
	}
	// Run-time copy names Bex, as Render's own CLI names Render (w2/073).
	if !strings.Contains(string(output), "Successfully logged out of Bex.\nRun `bex login` to log back in.") || strings.Contains(string(output), "Render") {
		t.Errorf("bex logout output = %q", output)
	}
	if !revoked {
		t.Error("bex logout did not revoke the stored Bex access token")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("Bex config exists after logout: %v", err)
	}
	if got, err := os.ReadFile(renderConfigPath); err != nil || string(got) != renderConfig {
		t.Errorf("Render config was read or changed: content=%q err=%v", got, err)
	}
}

func TestBexBinaryHonorsRenderConfigDirWithoutTouchingHomeBex(t *testing.T) {
	var seen []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/owners", "/v1/owners/tea-bex":
			if r.URL.Path == "/v1/owners/tea-bex" {
				_, _ = w.Write([]byte(`{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}`))
				return
			}
			_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	binary := buildBex()
	home := t.TempDir()
	bexConfig := filepath.Join(home, ".bex", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(bexConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	const sentinel = "version: 1\nworkspace: tea-sentinel\nworkspace_name: Must Not Change\napi:\n  key: stored-bex-token\n"
	if err := os.WriteFile(bexConfig, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	renderDir := t.TempDir()

	command := exec.Command(binary, "workspace", "set", "tea-bex")
	command.Env = append(withoutRenderEnv(os.Environ()),
		"HOME="+home,
		"BEX_HOST="+api.URL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
		"RENDER_CLI_CONFIG_DIR="+renderDir,
		"BEX_NO_UPDATE_NOTIFIER=1",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bex workspace set: %v\npaths=%v\n%s", err, seen, output)
	}
	if got, err := os.ReadFile(bexConfig); err != nil || string(got) != sentinel {
		t.Errorf("HOME/.bex/cli.yaml changed: content=%q err=%v", got, err)
	}
	written, err := os.ReadFile(filepath.Join(renderDir, "cli.yaml"))
	if err != nil {
		t.Fatalf("expected write under RENDER_CLI_CONFIG_DIR: %v\n%s", err, output)
	}
	if !strings.Contains(string(written), "tea-bex") {
		t.Errorf("isolated config missing workspace: %s", written)
	}
}

func TestBexBinaryRelocatesStateWithConfigDirNotBesideExactPath(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	configDir := t.TempDir()
	seedNoticeMarkerIn(t, filepath.Join(configDir, "state", "analytics"))

	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = telemetryEnv(t, home, api.URL, "BEX_CLI_CONFIG_DIR="+configDir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(configDir, "state", "installation-id.txt")); err != nil {
		t.Fatalf("state missing under BEX_CLI_CONFIG_DIR: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".render", "state")); !os.IsNotExist(err) {
		t.Errorf("~/.render/state was created: %v", err)
	}

	exact := filepath.Join(t.TempDir(), "only.yaml")
	pathCmd := exec.Command(buildBex(), "workspaces", "-o", "json")
	pathCmd.Env = telemetryEnv(t, home, api.URL, "BEX_CLI_CONFIG_PATH="+exact)
	if output, err := pathCmd.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces with PATH: %v\n%s", err, output)
	}
	if _, err := os.Stat(exact); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(exact))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "only.yaml" {
			t.Errorf("unexpected sibling next to exact config path: %s", e.Name())
		}
	}
}

// seedNoticeMarkerIn records, under the analytics state dir, that both the
// upstream and the Bex telemetry notices were shown, so a run outside CI
// passes the disclosure gate (ADR113 D5).
func seedNoticeMarkerIn(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"notice-shown", "bex-notice-shown"} {
		if err := os.WriteFile(filepath.Join(dir, marker), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// withoutRenderEnv is the environment a child bex starts from: the test
// process's own, less what would make one run differ from another. A test that
// wants one of the dropped variables passes it after this (the pseudo-terminal
// runs pass TERM=xterm; TestCIBypassesTheDisclosureGate passes CI=true).
func withoutRenderEnv(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(name, "RENDER_") || slices.Contains(droppedEnv, name) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// droppedEnv is what withoutRenderEnv drops by name, besides every RENDER_*.
//
//   - The two Bex credential variables, so a test sends only the credential it
//     sets: a developer who exports BEX_API_KEY must not have it outrank a
//     test's BEX_ACCESS_TOKEN or stored login, nor reach a test's stub server.
//   - What the root command and the color library under it read to decide how
//     a run looks, so a laptop, a pseudo-terminal and a runner behave alike:
//     CI (the root reads it as "not a terminal" and never asks; GitHub Actions
//     sets it), TERM (dumb reads the same way), and COLORTERM, NO_COLOR,
//     CLICOLOR and CLICOLOR_FORCE (whether the output carries color escapes).
//   - Every coding provider's key variables, read from the catalog so a new
//     provider is covered: a developer's ZAI_API_KEY would otherwise turn
//     `bex code`'s "no key" hint into "key from ZAI_API_KEY (environment)"
//     (w4/727). A test that needs a key sets it after this.
var droppedEnv = append([]string{
	"BEX_API_KEY", "BEX_ACCESS_TOKEN",
	"BEX_S3_ACCESS_KEY_ID", "BEX_S3_SECRET_ACCESS_KEY",
	"CI", "TERM", "COLORTERM", "NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
}, providerKeyEnvs()...)

// providerKeyEnvs is every variable a coding provider reads its key from.
func providerKeyEnvs() []string {
	var names []string
	for _, provider := range code.Catalog() {
		names = append(names, provider.KeyEnvs...)
	}
	return names
}

// updateTestEnv strips everything that gates or targets the update check so
// each test controls those inputs explicitly (CI, which silences the check, is
// already gone: see withoutRenderEnv).
func updateTestEnv(home string) []string {
	filtered := make([]string, 0, len(os.Environ()))
	for _, item := range withoutRenderEnv(os.Environ()) {
		name, _, _ := strings.Cut(item, "=")
		switch name {
		case "HOME", "BEX_NO_UPDATE_NOTIFIER", "BEX_UPDATE_API_URL":
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, "HOME="+home)
}

// TestChildEnvironmentsIgnoreTheRunnersOutputSettings: whatever CI, TERM or
// color settings the process running the tests exports, no helper hands one to
// a child bex, so a test sees the same run on a laptop, in a pseudo-terminal
// and on a CI runner (w21/115: bexAgainst inherited CI=true, the root saw no
// terminal and never asked). The names are those the root command reads: CI and
// TERM in pkg/command/outputresolver.go and pkg/cfg, the rest in termenv.
// Every coding provider's key variable is ambient too, taken from the catalog,
// so a provider whose key a helper would hand on fails here (w4/727).
func TestChildEnvironmentsIgnoreTheRunnersOutputSettings(t *testing.T) {
	ambient := map[string]string{
		"CI": "true", "TERM": "dumb", "COLORTERM": "truecolor",
		"NO_COLOR": "1", "CLICOLOR": "0", "CLICOLOR_FORCE": "1",
		"RENDER_OUTPUT": "json", "BEX_API_KEY": "ambient-key",
	}
	for _, provider := range code.Catalog() {
		for _, name := range provider.KeyEnvs {
			ambient[name] = "ambient-" + provider.Name + "-key"
		}
	}
	for name, value := range ambient {
		t.Setenv(name, value)
	}
	const apiURL = "http://127.0.0.1:1"
	for name, env := range map[string][]string{
		"withoutRenderEnv": withoutRenderEnv(os.Environ()),
		"bexAgainst":       bexAgainst(t, apiURL, "workspaces").Env,
		"telemetryEnv":     telemetryEnv(t, t.TempDir(), apiURL),
		"s3QuietEnv":       s3QuietEnv(t, t.TempDir(), apiURL, apiURL),
		"updateTestEnv":    updateTestEnv(t.TempDir()),
	} {
		for _, item := range env {
			if key, _, _ := strings.Cut(item, "="); ambient[key] != "" {
				t.Errorf("%s hands a child %s", name, item)
			}
		}
		if !slices.ContainsFunc(env, func(item string) bool { return strings.HasPrefix(item, "PATH=") }) {
			t.Errorf("%s dropped PATH too", name)
		}
	}
}

// releasesServer serves a bex-co/bex releases list whose newest stable
// bex-cli release is v9.9.9, counting requests. The counter is atomic: the
// handler runs on the server goroutine while the test reads the count, and
// the only ordering edge is the child process's exit.
func releasesServer(t *testing.T, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/repos/bex-co/bex/releases" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"tag_name": "bex-cli/v9.9.9", "html_url": "https://example.test/releases/bex-cli-v9.9.9", "draft": false, "prerelease": false},
			{"tag_name": "operator/v99.0.0", "html_url": "https://example.test/releases/operator", "draft": false, "prerelease": false}
		]`))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestBexVersionOwnsTheVersionPath(t *testing.T) {
	var requests atomic.Int32
	api := releasesServer(t, &requests)
	home := t.TempDir()

	run := func() string {
		command := exec.Command(buildBex(), "--version")
		command.Env = append(updateTestEnv(home), "BEX_UPDATE_API_URL="+api.URL)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("bex --version: %v\n%s", err, output)
		}
		return string(output)
	}

	output := run()
	if !strings.Contains(output, "bex v"+testBexVersion+"\n") {
		t.Errorf("missing bex identity line:\n%s", output)
	}
	if !strings.Contains(output, "compatible with Render CLI v"+testUpstreamVersion) {
		t.Errorf("missing compatibility line:\n%s", output)
	}
	if !strings.Contains(output, "v"+testBexVersion+" → v9.9.9") || !strings.Contains(output, "https://example.test/releases/bex-cli-v9.9.9") {
		t.Errorf("missing bex upgrade hint:\n%s", output)
	}
	if !strings.HasSuffix(output, "\nTo upgrade, run: bex upgrade\n") {
		t.Errorf("version hint does not say how to upgrade:\n%s", output)
	}
	if strings.Contains(output, "render v") {
		t.Errorf("upstream version handler ran:\n%s", output)
	}
	if strings.Contains(output, "bex v"+testBexVersion+" (Render CLI") {
		t.Errorf("old single-line version format still present:\n%s", output)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}

	// Second run inside the cache window: same answer, no network call.
	output = run()
	if got := requests.Load(); got != 1 {
		t.Errorf("cache hit still made a request (requests = %d)", got)
	}
	if !strings.Contains(output, "v9.9.9") {
		t.Errorf("cached upgrade hint missing:\n%s", output)
	}
}

func TestBexVersionCheckSilencedByCIAndOptOut(t *testing.T) {
	for _, gate := range []string{"CI=1", "BEX_NO_UPDATE_NOTIFIER=1"} {
		var requests atomic.Int32
		api := releasesServer(t, &requests)
		command := exec.Command(buildBex(), "-v")
		command.Env = append(updateTestEnv(t.TempDir()), "BEX_UPDATE_API_URL="+api.URL, gate)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("[%s] bex -v: %v\n%s", gate, err, output)
		}
		if !strings.Contains(string(output), "bex v"+testBexVersion) {
			t.Errorf("[%s] version line missing:\n%s", gate, output)
		}
		if strings.Contains(string(output), "9.9.9") {
			t.Errorf("[%s] update hint printed despite gate:\n%s", gate, output)
		}
		if got := requests.Load(); got != 0 {
			t.Errorf("[%s] gated run still made %d network request(s)", gate, got)
		}
	}
}

func TestVersionFlagAfterSubcommandReachesUpstream(t *testing.T) {
	command := exec.Command(buildBex(), "services", "-v")
	command.Env = updateTestEnv(t.TempDir())
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("services -v unexpectedly succeeded:\n%s", output)
	}
	if strings.Contains(string(output), "bex v"+testBexVersion) {
		t.Errorf("bex intercepted a post-subcommand flag:\n%s", output)
	}
}

func TestNormalCommandsMakeNoUpdateCheckOffTTY(t *testing.T) {
	var requests atomic.Int32
	api := releasesServer(t, &requests)
	command := exec.Command(buildBex(), "--help")
	command.Env = append(updateTestEnv(t.TempDir()), "BEX_UPDATE_API_URL="+api.URL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex --help: %v\n%s", err, output)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("non-TTY command run made %d update request(s)", got)
	}
}

func TestBexHelpChromeIsBranded(t *testing.T) {
	command := exec.Command(buildBex(), "--help")
	command.Env = updateTestEnv(t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bex --help: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "bex CLI v"+testBexVersion) {
		t.Errorf("help missing bex CLI chrome:\n%s", text)
	}
	if strings.Contains(text, "Render CLI v") {
		t.Errorf("help still shows Render CLI chrome:\n%s", text)
	}
	if !strings.Contains(text, "Usage:\n  bex") && !strings.Contains(text, "USAGE\n  bex") {
		// Upstream template uses styled "USAGE" then "  bex".
		if !strings.Contains(text, "bex") || strings.Contains(text, "  render ") {
			t.Errorf("help usage path not bex:\n%s", text)
		}
	}
	if strings.Contains(text, "  render services") {
		t.Errorf("help examples still use render:\n%s", text)
	}
	if !strings.Contains(text, "upgrade") || !strings.Contains(text, "code") {
		t.Errorf("ungrouped Bex commands missing from help:\n%s", text)
	}
	// Upstream v2.29 points agents at `render skills install` and Render's
	// skills; bex's `skills` installs the bex pack (w8/m67, w8/m143).
	if !strings.Contains(text, "bex skills install --output json") || strings.Contains(text, "Render's agent skills") {
		t.Errorf("agent hint not Bex-branded:\n%s", text)
	}
}

func TestBexDocsCommandPointsAtBex(t *testing.T) {
	command := exec.Command(buildBex(), "docs", "--help")
	command.Env = updateTestEnv(t.TempDir())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bex docs --help: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "Bex docs") && !strings.Contains(text, "Bex CLI guide") {
		t.Errorf("docs help not Bex-branded:\n%s", text)
	}
	if strings.Contains(text, "render.com/docs") {
		t.Errorf("docs help still mentions render.com/docs:\n%s", text)
	}
	if strings.Contains(text, "render docs") {
		t.Errorf("docs examples still use render:\n%s", text)
	}
}

func buildBex() string {
	return bexBinary
}

// telemetryStub records POST /cli-telemetry-events bodies while serving the
// minimal workspace list the CLI needs to complete a command, plus any routes
// a test scripts (an agent session, an automation run).
type telemetryStub struct {
	mu sync.Mutex
	// headers records the release header seen on every request, telemetry or
	// not, so a test can prove the stamp reaches both request classes.
	headers []string
	bodies  []string
	// routes answers each path with its JSON body; set before the server starts.
	routes map[string]string
}

func (s *telemetryStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.headers = append(s.headers, r.URL.Path+" "+r.Header.Get(bridge.VersionHeader))
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/owners":
			_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
		case "/v1/cli-telemetry-events":
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.bodies = append(s.bodies, string(body))
			s.mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"cte-test"}`))
		default:
			body, ok := s.routes[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(body))
		}
	})
}

func (s *telemetryStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

// seedNoticeMarker pre-creates the one-time-notice markers under the test
// HOME so a headless run proceeds to the send path: outside CI nothing is sent
// until a marker proves the Bex notice was shown once.
func seedNoticeMarker(t *testing.T, home string) {
	t.Helper()
	seedNoticeMarkerIn(t, filepath.Join(home, ".bex", "state", "analytics"))
}

func TestBexEmitsTelemetryToBexAPI(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = telemetryEnv(t, home, api.URL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}

	event := awaitEvents(t, stub, 1)[0]
	// Upstream reports the full command path including the binary name —
	// which the branding overlay renamed, so a bex binary reports "bex …".
	if event["command"] != "bex workspaces" {
		t.Errorf("command = %v, want bex workspaces", event["command"])
	}
	if event["cli_version"] != testUpstreamVersion {
		t.Errorf("cli_version = %v, want %v", event["cli_version"], testUpstreamVersion)
	}
	if event["installation_id"] == "" {
		t.Errorf("installation_id missing: %v", event)
	}
	bexID, err := os.ReadFile(filepath.Join(home, ".bex", "state", "installation-id.txt"))
	if err != nil {
		t.Fatalf("expected installation id under $HOME/.bex/state: %v", err)
	}
	if event["installation_id"] != strings.TrimSpace(string(bexID)) {
		t.Errorf("installation_id = %v, want %s from $HOME/.bex/state", event["installation_id"], bexID)
	}
	if _, err := os.Stat(filepath.Join(home, ".render", "state")); !os.IsNotExist(err) {
		t.Errorf("~/.render/state was created: %v", err)
	}
	if event["exit_code"] != float64(0) {
		t.Errorf("exit_code = %v, want 0", event["exit_code"])
	}
}

func TestBexOptOutSuppressesTelemetry(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = telemetryEnv(t, home, api.URL, "BEX_CLI_DISABLE_ANALYTICS=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	time.Sleep(3 * time.Second)
	if got := stub.count(); got != 0 {
		t.Errorf("telemetry events = %d, want 0 under opt-out", got)
	}
}

// stamps returns the recorded "<path> <release header>" lines.
func (s *telemetryStub) stamps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.headers...)
}

// events returns every captured telemetry body, decoded.
func (s *telemetryStub) events(t *testing.T) []map[string]any {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	decoded := make([]map[string]any, 0, len(s.bodies))
	for _, body := range s.bodies {
		var event map[string]any
		if err := json.Unmarshal([]byte(body), &event); err != nil {
			t.Fatalf("decode telemetry body: %v", err)
		}
		decoded = append(decoded, event)
	}
	return decoded
}

// awaitEvents waits for the detached sender, then holds still long enough that
// a second (wrongly emitted) event would also have landed — so "exactly one"
// means one, not one-so-far.
func awaitEvents(t *testing.T, stub *telemetryStub, want int) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for stub.count() < want && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	events := stub.events(t)
	if len(events) != want {
		t.Fatalf("telemetry events = %d, want %d: %v", len(events), want, events)
	}
	return events
}

// telemetryEnv is the child environment for a launcher run that is allowed to
// report: BEX_CLI_DISABLE_ANALYTICS="" re-enables sending past TestMain's
// harness-wide opt-out (blank counts as unset in the bridge). CI is not
// inherited (withoutRenderEnv), because it bypasses the disclosure gate and
// runners set it: a test that wants CI passes it in extra.
func telemetryEnv(t *testing.T, home, apiURL string, extra ...string) []string {
	t.Helper()
	return append(append(withoutRenderEnv(os.Environ()),
		"HOME="+home,
		"BEX_HOST="+apiURL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
		"BEX_CLI_DISABLE_ANALYTICS=",
	), extra...)
}

// stubClaudeOnPath installs a harmless `claude` so a provider launch reaches
// its exec instead of failing on a missing binary, and returns the PATH entry.
func stubClaudeOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestBexStampsReleaseHeaderOnEveryControlPlaneRequest pins the mechanism the
// whole release axis rests on (w5/m94): wrapping http.DefaultTransport reaches
// both the command's own API call and the telemetry POST, which is sent by a
// detached subprocess that re-executes this binary.
func TestBexStampsReleaseHeaderOnEveryControlPlaneRequest(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = telemetryEnv(t, home, api.URL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	awaitEvents(t, stub, 1)

	stamps := stub.stamps()
	var sawAPI, sawTelemetry bool
	for _, stamp := range stamps {
		switch stamp {
		case "/v1/owners " + testBexVersion:
			sawAPI = true
		case "/v1/cli-telemetry-events " + testBexVersion:
			sawTelemetry = true
		}
	}
	if !sawAPI {
		t.Errorf("release header missing from the API call; saw %v", stamps)
	}
	if !sawTelemetry {
		t.Errorf("release header missing from the detached telemetry POST; saw %v", stamps)
	}
}

// TestBexVersionEmitsExactlyOneVersionEvent covers a path upstream cannot see:
// the root version answer exits before cmd.Execute(), so nothing else reports
// it.
func TestBexVersionEmitsExactlyOneVersionEvent(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "--version")
	// Keeps the release-channel update check off the network. CI would too,
	// but it also bypasses the disclosure gate this run must pass.
	command.Env = telemetryEnv(t, home, api.URL, "BEX_NO_UPDATE_NOTIFIER=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex --version: %v\n%s", err, output)
	}

	event := awaitEvents(t, stub, 1)[0]
	if event["completion_kind"] != "version" {
		t.Errorf("completion_kind = %v, want version", event["completion_kind"])
	}
	if event["exit_code"] != float64(0) {
		t.Errorf("exit_code = %v, want 0", event["exit_code"])
	}
}

// TestBexProviderLaunchEmitsExactlyOneEvent covers the other invisible path: the
// launcher replaces its own process, so upstream's post-run hook never fires.
func TestBexProviderLaunchEmitsExactlyOneEvent(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "glm")
	command.Env = telemetryEnv(t, home, api.URL,
		"PATH="+stubClaudeOnPath(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ZAI_API_KEY=test-provider-key")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex glm: %v\n%s", err, output)
	}

	event := awaitEvents(t, stub, 1)[0]
	if event["command"] != "bex glm" {
		t.Errorf("command = %v, want bex glm", event["command"])
	}
	// Only the matched command names may travel; the launched agent's
	// arguments, prompts and paths never do.
	if path, _ := event["command"].(string); strings.ContainsAny(path, "/-") {
		t.Errorf("command %q looks like it carries an argument", path)
	}
}

// TestBexFailedLaunchIsReportedOnceByUpstream is the no-double-count half. With
// no claude to exec, the process is never replaced, so Cobra finishes normally
// and upstream's hook reports it — the launcher must not add a second event.
func TestBexFailedLaunchIsReportedOnceByUpstream(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)

	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "glm")
	command.Env = telemetryEnv(t, home, api.URL, "PATH="+t.TempDir())
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("bex glm succeeded with no claude on PATH:\n%s", output)
	}

	event := awaitEvents(t, stub, 1)[0]
	if event["command"] != "bex glm" {
		t.Errorf("command = %v, want bex glm", event["command"])
	}
	if event["completion_kind"] != "execution_error" {
		t.Errorf("completion_kind = %v, want execution_error", event["completion_kind"])
	}
}

// TestBexOptOutSuppressesLauncherNativeTelemetry proves the new emitters are
// gated by the same consent the imported commands obey — they delegate to
// upstream's sender rather than reimplementing the policy.
func TestBexOptOutSuppressesLauncherNativeTelemetry(t *testing.T) {
	for _, optOut := range []string{"BEX_CLI_DISABLE_ANALYTICS=1", "DO_NOT_TRACK=1", "RENDER_CLI_DISABLE_ANALYTICS=1"} {
		t.Run(strings.SplitN(optOut, "=", 2)[0], func(t *testing.T) {
			stub := &telemetryStub{}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)

			home := t.TempDir()
			seedNoticeMarker(t, home)

			version := exec.Command(buildBex(), "--version")
			version.Env = telemetryEnv(t, home, api.URL, "BEX_NO_UPDATE_NOTIFIER=1", optOut)
			if output, err := version.CombinedOutput(); err != nil {
				t.Fatalf("bex --version: %v\n%s", err, output)
			}

			launch := exec.Command(buildBex(), "glm")
			launch.Env = telemetryEnv(t, home, api.URL,
				"PATH="+stubClaudeOnPath(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
				"ZAI_API_KEY=test-provider-key", optOut)
			if output, err := launch.CombinedOutput(); err != nil {
				t.Fatalf("bex glm: %v\n%s", err, output)
			}

			time.Sleep(3 * time.Second)
			if got := stub.count(); got != 0 {
				t.Errorf("telemetry events = %d, want 0 under %s", got, optOut)
			}
		})
	}
}

// TestBexCompletionBranding: upstream registers the Postgres/Key Value children
// after branding.Apply, and successful completion prints their Short without
// HelpFunc/UsageFunc (w2/043). A real launcher subprocess exercises the actual
// initialization order a preconstructed cobra tree would mask.
func TestBexCompletionBranding(t *testing.T) {
	values := []string{"create", "delete", "get", "list", "resume", "suspend", "update"}
	for _, tc := range []struct{ selector, product string }{
		{"postgres", "Bex Postgres"}, {"pg", "Bex Postgres"},
		{"keyvalues", "Bex Key Value"}, {"kv", "Bex Key Value"},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			stdout, stderr := runBex(t, "__complete", tc.selector, "")
			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if len(lines) != len(values)+1 || lines[len(lines)-1] != ":4" {
				t.Fatalf("completion protocol changed:\n%s", stdout)
			}
			for i, value := range values {
				name, description, ok := strings.Cut(lines[i], "\t")
				if !ok || name != value || !strings.Contains(description, tc.product) || strings.Contains(description, "Render") {
					t.Errorf("line %d = %q, want %s with a %s description", i, lines[i], value, tc.product)
				}
			}
			if stderr != "Completion ended with directive: ShellCompDirectiveNoFileComp\n" {
				t.Errorf("stderr = %q", stderr)
			}
		})
	}
	stdout, _ := runBex(t, "__completeNoDesc", "pg", "")
	if want := strings.Join(append(values, ":4"), "\n") + "\n"; stdout != want {
		t.Errorf("__completeNoDesc = %q, want %q", stdout, want)
	}
}

// runBex runs the launcher in a fresh process with update checks and analytics
// off, failing the test on a non-zero exit.
func runBex(t *testing.T, args ...string) (string, string) {
	t.Helper()
	command := exec.Command(buildBex(), args...)
	for _, item := range updateTestEnv(t.TempDir()) {
		if !strings.HasPrefix(item, "BEX_") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "BEX_NO_UPDATE_NOTIFIER=1", "BEX_CLI_DISABLE_ANALYTICS=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// Exercise the imported tree in fresh processes, including commands registered
// after branding.Apply and flag descriptions rendered by upstream's template.
func TestBexNestedHelp(t *testing.T) {
	cases := []struct {
		command string
		want    []string
		absent  []string
	}{
		{"workspace set", []string{"$HOME/.bex/cli.yaml", "$HOME/.bex/state", "BEX_CLI_CONFIG_DIR", "BEX_CLI_CONFIG_PATH", "RENDER_CLI_CONFIG_PATH", "RENDER_CLI_CONFIG_DIR"}, []string{"$HOME/.render"}},
		{"blueprints validate", []string{"render.yaml", "bex blueprints validate"}, []string{"bex.yaml"}},
		// w7/045: double-quoted command references and "your Render workspace"
		// in the sandbox family (top-level since v2.35.0; it was `ea sandboxes`),
		// and the render.com job-plan link (whose replacement must not cite a
		// nonexistent `services get`, w2/044).
		{"sandboxes", []string{"your Bex workspace", "\"bex sandbox-groups\"", "\"bex sandboxes snapshots create\"", "\"bex sandboxes create --snapshot-id\""}, []string{"Render workspace", "\"render "}},
		{"sandboxes snapshots", []string{"bex sandboxes snapshots create sbx-abc123"}, []string{"Render workspace", "render "}},
		{"sandboxes snapshots create", nil, []string{"render "}},
		{"sandbox-groups", []string{"your Bex workspace"}, []string{"Render workspace", "\"render "}},
		// v2.34.0 piped image-import example starts mid-line.
		{"sandbox-groups images", []string{"| bex sandbox-groups images import"}, []string{"render "}},
		{"sandbox-groups images import", []string{"| bex sandbox-groups images import"}, []string{"render "}},
		// v2.37.0 (w22/m444): image list and delete, and the interactive
		// `sandboxes list` with its `-o text` table.
		{"sandbox-groups images", []string{"bex sandbox-groups images list", "bex sandbox-groups images delete sbb-abc123 --confirm"}, []string{"render "}},
		{"sandbox-groups images list", []string{"--name <NAME>", "--status <STATUS>", "\"bex sandboxes create --image NAME\"", "\"bex sandbox-groups images get IMAGE_ID\""}, []string{"render ", "Render"}},
		{"sandbox-groups images delete", []string{"bex sandbox-groups images delete sbb-abc123 --confirm", "Without --confirm, this command previews"}, []string{"render ", "Render"}},
		{"sandboxes list", []string{"bex sandboxes list -o text", "interactive list"}, []string{"render ", "Render"}},
		// v2.38.0 (w22/m604): a group's project environment.
		{"sandbox-groups", []string{"bex sandbox-groups environment set sbg-abc123 production", "bex sandbox-groups environment remove sbg-abc123"}, []string{"render "}},
		{"sandbox-groups environment set", []string{"bex sandbox-groups environment set sbg-abc123 production", "an ID or a name in the active workspace"}, []string{"render ", "Render"}},
		{"sandbox-groups environment remove", []string{"bex sandbox-groups environment remove sbg-abc123 -o json"}, []string{"render ", "Render"}},
		{"sandbox-groups create", []string{"--environment <ENVIRONMENT>", "bex sandbox-groups create --name agents --environment production", "\"bex sandboxes create\""}, []string{"render ", "Render"}},
		// v2.35.0 `ea blueprints`: bex's state dir and config-dir input.
		{"ea blueprints create", []string{"~/.bex/state/manual-env-vars/", "BEX_CLI_CONFIG_DIR", "Bex reads the pushed repository", "render.yaml"}, []string{"~/.render", "RENDER_CLI_CONFIG_DIR", "Render ", "render "}},
		{"ea blueprints plan", []string{"~/.bex/state/manual-env-vars/", "BEX_CLI_CONFIG_DIR", "\"bex ea blueprints approve\""}, []string{"~/.render", "RENDER_CLI_CONFIG_DIR", "Render ", "render "}},
		{"ea blueprints approve", []string{"~/.bex/state/manual-env-vars/", "BEX_CLI_CONFIG_DIR", "\"bex ea blueprints plan\""}, []string{"~/.render", "RENDER_CLI_CONFIG_DIR", "Render ", "render "}},
		{"ea blueprints update", []string{"~/.bex/state/manual-env-vars/", "\"bex ea blueprints approve\""}, []string{"~/.render", "Render ", "render "}},
		{"jobs create", []string{"bex jobs create srv-abc123", "--plan-id plan-srv-006"}, []string{"render.com/docs", "render jobs create", "services get"}},
		// w32/008: the v2.35.0 help pages that linked render.com/docs name the
		// Bex page on the same topic.
		{"ssh", []string{"your Bex account", "https://bex.co/docs/platform/ssh#register-a-public-key"}, []string{"render.com/docs"}},
		{"ea blueprints", []string{"https://bex.co/docs/platform/blueprints"}, []string{"render.com/docs"}},
	}
	for _, resource := range []string{"postgres", "pg", "keyvalues", "kv"} {
		for _, action := range []string{"create", "get", "update", "delete", "suspend", "resume"} {
			cases = append(cases, struct {
				command string
				want    []string
				absent  []string
			}{
				resource + " " + action, []string{"bex workspace set"}, []string{"render workspace set"},
			})
		}
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			stdout, stderr := runBex(t, append(strings.Fields(tc.command), "--help")...)
			if stderr != "" {
				t.Errorf("unexpected stderr: %s", stderr)
			}
			text := strings.Join(strings.Fields(stdout), " ")
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Errorf("help missing %q:\n%s", want, text)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(text, absent) {
					t.Errorf("help contains %q:\n%s", absent, text)
				}
			}
		})
	}
}

// helpSubcommand matches one row of a help page's command list.
var helpSubcommand = regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*) `)

// helpTree returns every command path the built launcher lists in its help,
// below the root, with each page's text. It reads the pages the way a user
// does, so commands upstream registers during Execute are included. The
// provider launchers are left out: they hand --help to Claude Code.
func helpTree(t *testing.T) map[string]string {
	t.Helper()
	passThrough := map[string]bool{}
	for _, p := range code.Catalog() {
		passThrough[p.Name] = true
	}
	pages := map[string]string{}
	var walk func(path string)
	walk = func(path string) {
		if passThrough[path] {
			return
		}
		stdout, _ := runBex(t, append(strings.Fields(path), "--help")...)
		pages[path] = stdout
		_, list, _ := strings.Cut(stdout, "\nSUBCOMMANDS\n")
		list, _, _ = strings.Cut(list, "\nFLAGS\n")
		for _, m := range helpSubcommand.FindAllStringSubmatch(list, -1) {
			walk(strings.TrimSpace(path + " " + m[1]))
		}
	}
	walk("")
	return pages
}

// A pin bump can bring a render.com/docs link into a command's help; v2.35.0
// brought two and no test noticed (w32/008). A served command's link needs its
// Bex page in branding.helpDocsLinks.
func TestNoHelpLinksRenderDocs(t *testing.T) {
	pages := helpTree(t)
	for _, known := range []string{"ssh", "ea blueprints", "jobs create", "sandbox-groups images import"} {
		if _, ok := pages[known]; !ok {
			t.Fatalf("the help walk did not reach %q (%d pages): the help layout changed", known, len(pages))
		}
	}
	for path, text := range pages {
		if strings.Contains(text, "render.com/docs") {
			t.Errorf("`bex %s --help` links render.com/docs:\n%s", path, text)
		}
	}
}

// TestBexLoginReportsBexReleaseNotRenderCLI pins the banner upstream prints
// after a successful login: it advertised render-oss/cli releases and
// render.com install docs; bex reports its own bex-cli release instead, even
// off a TTY, and only once login actually authenticated.
func TestBexLoginReportsBexReleaseNotRenderCLI(t *testing.T) {
	deviceAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/device-grant":
			_, _ = w.Write([]byte(`{"device_code":"device","user_code":"BEX-123","verification_uri":"https://dashboard.example.test/auth/device","verification_uri_complete":"https://dashboard.example.test/auth/device?user_code=BEX-123","expires_in":30,"interval":1}`))
		case "/v1/device-token":
			_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600,"refresh_token":"refresh"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(deviceAPI.Close)
	openerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(openerDir, "open"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	run := func(gate string, args ...string) (string, int32) {
		var requests atomic.Int32
		releases := releasesServer(t, &requests)
		command := exec.Command(buildBex(), args...)
		command.Env = append(updateTestEnv(t.TempDir()),
			"BEX_UPDATE_API_URL="+releases.URL,
			"BEX_HOST="+deviceAPI.URL+"/v1/",
			"PATH="+openerDir+":"+os.Getenv("PATH"))
		if gate != "" {
			command.Env = append(command.Env, gate)
		}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("bex %v: %v\n%s", args, err, output)
		}
		return string(output), requests.Load()
	}

	output, _ := run("", "login")
	if !strings.Contains(output, "Login successful!") {
		t.Fatalf("login did not complete:\n%s", output)
	}
	if !strings.Contains(output, "Complete login in the Bex dashboard with code: BEX-123") ||
		!strings.Contains(output, "https://dashboard.example.test/auth/device?user_code=BEX-123") ||
		strings.Contains(output, "Render Dashboard") {
		t.Errorf("login prompt did not preserve the Bex identity and exact device URL/code:\n%s", output)
	}
	if !strings.Contains(output, "v"+testBexVersion+" → v9.9.9") || !strings.Contains(output, "https://example.test/releases/bex-cli-v9.9.9") {
		t.Errorf("login did not report the newest bex release:\n%s", output)
	}
	// The test binary lives in a temp dir, i.e. an install-script-style path.
	if !strings.HasSuffix(output, "\nTo upgrade, run: bex upgrade\n") {
		t.Errorf("login did not say how to upgrade:\n%s", output)
	}
	if strings.Contains(output, "render v") || strings.Contains(output, "render.com/docs/cli") {
		t.Errorf("login advertised the Render CLI:\n%s", output)
	}
	for _, format := range []string{"text", "interactive"} {
		output, _ := run("CI=1", "login", "-o", format)
		if !strings.Contains(output, "Complete login in the Bex dashboard with code: BEX-123") ||
			!strings.Contains(output, "https://dashboard.example.test/auth/device?user_code=BEX-123") ||
			strings.Contains(output, "Render Dashboard") {
			t.Errorf("login -o %s did not preserve the Bex prompt and device URL/code:\n%s", format, output)
		}
	}

	if output, _ = run("", "login", "--help"); strings.Contains(output, "9.9.9") {
		t.Errorf("login --help printed an update hint:\n%s", output)
	}

	output, requests := run("CI=1", "login")
	if strings.Contains(output, "9.9.9") || requests != 0 {
		t.Errorf("CI login checked for updates (requests=%d):\n%s", requests, output)
	}
}

// TestStaleWorkspaceNameBlocksOnlyConsumers: a BEX_WORKSPACE name that no
// longer resolves must not block the commands that recover from it — help,
// version, inventory, identity, explicit selection, completion, an explicit
// --workspace, a typed-id read — while a command that consumes the active
// workspace still refuses (w8/064).
func TestStaleWorkspaceNameBlocksOnlyConsumers(t *testing.T) {
	const (
		teamID = "tea-12345678901234567890"
		dbID   = "dpg-12345678901234567890"
		kvID   = "red-12345678901234567890"
		srvID  = "srv-12345678901234567890"
		sbgID  = "sbg-12345678901234567890"
		sbbID  = "sbb-12345678901234567890"
		crdID  = "crd-12345678901234567890"
		exsID  = "exs-12345678901234567890"
		agsID  = "ags-12345678901234567890"
		sbxID  = "sbx-12345678901234567890"
		snpID  = "snp-12345678901234567890"
		evmID  = "evm-12345678901234567890"
		autID  = "aut-12345678901234567890"
		sklID  = "skl-12345678901234567890"
		secID  = "sec-12345678901234567890"
		objID  = "obj-12345678901234567890"
		bktID  = "bkt-12345678901234567890"
	)
	var mu sync.Mutex
	var nameLookups, sentOwners, leakedOwners []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if owner := r.URL.Query().Get("ownerId"); owner == "stale-ws" || owner == "dup-ws" {
			mu.Lock()
			leakedOwners = append(leakedOwners, r.Method+" "+r.URL.RequestURI())
			mu.Unlock()
		}
		if strings.Contains(r.URL.Path, "/stale-ws/") || strings.Contains(r.URL.Path, "/dup-ws/") {
			mu.Lock()
			leakedOwners = append(leakedOwners, r.Method+" "+r.URL.RequestURI())
			mu.Unlock()
		}
		switch r.URL.Path {
		case "/v1/owners":
			name := r.URL.Query().Get("name")
			if name != "" {
				mu.Lock()
				nameLookups = append(nameLookups, name)
				mu.Unlock()
			}
			switch name {
			case "stale-ws":
				_, _ = w.Write([]byte(`[]`))
			case "dup-ws":
				_, _ = w.Write([]byte(`[{"owner":{"id":"` + teamID + `","name":"dup-ws","type":"team"},"cursor":"a"},{"owner":{"id":"tea-09876543210987654321","name":"dup-ws","type":"team"},"cursor":"b"}]`))
			default:
				_, _ = w.Write([]byte(`[{"owner":{"id":"` + teamID + `","name":"known","type":"team"},"cursor":"a"}]`))
			}
		case "/v1/owners/" + teamID:
			_, _ = w.Write([]byte(`{"id":"` + teamID + `","name":"known","type":"team"}`))
		case "/v1/users":
			_, _ = w.Write([]byte(`{"id":"usr-12345678901234567890","email":"qa@example.test","name":"qa"}`))
		case "/v1/postgres/" + dbID:
			_, _ = w.Write([]byte(`{"id":"` + dbID + `","name":"db","plan":"free","version":"18","status":"available","databaseName":"d","databaseUser":"u","ipAllowList":[],"readReplicas":[],"suspended":"not_suspended"}`))
		case "/v1/key-value/" + kvID:
			_, _ = w.Write([]byte(`{"id":"` + kvID + `","name":"kv","plan":"free","status":"available","ipAllowList":[],"suspended":"not_suspended"}`))
		case "/v1/sandbox-groups/" + sbgID:
			_, _ = w.Write([]byte(`{"id":"` + sbgID + `","name":"extra","isDefault":false}`))
		case "/v1/sandbox-groups/" + sbgID + "/builds/" + sbbID:
			_, _ = w.Write([]byte(`{"id":"` + sbbID + `","name":"img","status":"succeeded","sandboxGroupId":"` + sbgID + `"}`))
		case "/v1/sandbox-groups/" + sbgID + "/credentials/" + crdID:
			_, _ = w.Write([]byte(`{"id":"` + crdID + `","domain":"api.example.com","enabled":true,"injection":{"type":"header","name":"Authorization"}}`))
		case "/v1/agent-sessions/" + agsID:
			_, _ = w.Write([]byte(`{"id":"` + agsID + `","ownerId":"` + teamID + `","state":"completed","turns":1}`))
		case "/v1/blueprints/" + exsID + "/plan":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"blueprint not found"}`))
		case "/v1/blueprints/validate":
			owner := r.FormValue("ownerId")
			mu.Lock()
			sentOwners = append(sentOwners, owner)
			mu.Unlock()
			if owner != teamID {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"fixture expects the resolved workspace id"}`))
				return
			}
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/v1/postgres", "/v1/key-value":
			if r.Method != http.MethodPost {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			var body struct {
				OwnerID string `json:"ownerId"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			sentOwners = append(sentOwners, body.OwnerID)
			mu.Unlock()
			// Refuse every create: the test reads the request, never a resource.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"fixture refuses creates"}`))
		default: // any other collection (services and its project/environment reads)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(api.Close)
	blueprint := filepath.Join(t.TempDir(), "render.yaml")
	if err := os.WriteFile(blueprint, []byte("services:\n  - type: web\n    name: x\n    runtime: image\n    image:\n      url: nginx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runWithEnv := func(workspace string, extraEnv []string, args ...string) (int, string) {
		t.Helper()
		command := exec.Command(buildBex(), args...)
		command.Env = append(withoutRenderEnv(updateTestEnv(t.TempDir())),
			"HOME="+t.TempDir(), "BEX_HOST="+api.URL+"/v1/", "BEX_ACCESS_TOKEN=test-access-token",
			"BEX_WORKSPACE="+workspace, "BEX_NO_UPDATE_NOTIFIER=1", "BEX_CLI_DISABLE_ANALYTICS=1")
		command.Env = append(command.Env, extraEnv...)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		err := command.Run()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return code, stderr.String()
	}
	run := func(workspace string, args ...string) (int, string) {
		t.Helper()
		return runWithEnv(workspace, nil, args...)
	}
	drain := func(recorded *[]string) []string {
		mu.Lock()
		defer mu.Unlock()
		out := *recorded
		*recorded = nil
		return out
	}
	lookups := func() int { return len(drain(&nameLookups)) }

	openerDir := t.TempDir()
	for _, tool := range []string{"open", "xdg-open"} {
		if err := os.WriteFile(filepath.Join(openerDir, tool), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	localFile := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(localFile, []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claudeDir := stubClaudeOnPath(t)
	s3KeyEnv := []string{
		"BEX_S3_ACCESS_KEY_ID=s3k-12345678901234567890",
		"BEX_S3_SECRET_ACCESS_KEY=test-s3-secret",
		"BEX_S3_ENDPOINT=http://127.0.0.1:1",
	}

	for _, workspace := range []string{"stale-ws", "dup-ws", "known"} {
		for _, args := range [][]string{
			{"--help"}, {"services", "--help"}, {"--version"},
			{"workspaces", "-o", "json"}, {"whoami", "-o", "json"},
			{"workspace", "set", teamID, "-o", "json"},
			{"completion", "bash"}, {"__complete", "postgres", ""},
			{"blueprints", "validate", blueprint, "--workspace", teamID, "-o", "json"},
			{"blueprints", "validate", blueprint, "-w" + teamID, "-o", "json"},
			{"pg"}, {"workspace"},
			// bex's groups run only to refuse a word (w8/368)
			{"agent"}, {"agent", "secrets"}, {"s3"}, {"s3", "cors"}, {"s3", "lifecycle"}, {"s3", "keys"},
			// local-only commands added after w8/064 (w32/116)
			{"skills"}, {"skills", "list", "-o", "json"},
			{"code"}, {"code", "keys"},
			{"analytics", "notice"},
		} {
			if code, stderr := run(workspace, args...); code != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %v exited %d: %s", workspace, args, code, stderr)
			}
			if n := lookups(); n != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %v looked the name up %d times; it never reads the active workspace", workspace, args, n)
			}
		}

		// Provider launchers from code.Catalog() are purely local.
		for _, p := range code.Catalog() {
			env := []string{"PATH=" + claudeDir + string(os.PathListSeparator) + os.Getenv("PATH")}
			if len(p.KeyEnvs) > 0 {
				env = append(env, p.KeyEnvs[0]+"=test-provider-key")
			}
			if code, stderr := runWithEnv(workspace, env, p.Name, "--version"); code != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %s --version exited %d: %s", workspace, p.Name, code, stderr)
			}
			if n := lookups(); n != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %s looked the name up %d times; provider launchers never read the active workspace", workspace, p.Name, n)
			}
		}

		// `bex docs` opens docs in a browser and never reads the workspace.
		if code, stderr := runWithEnv(workspace, []string{"PATH=" + openerDir + string(os.PathListSeparator) + os.Getenv("PATH")}, "docs"); code != 0 {
			t.Errorf("BEX_WORKSPACE=%s bex docs exited %d: %s", workspace, code, stderr)
		}
		if n := lookups(); n != 0 {
			t.Errorf("BEX_WORKSPACE=%s bex docs looked the name up %d times", workspace, n)
		}

		// Commands that fail for their own non-workspace reasons in non-TTY /
		// missing-file / missing-arg / unreachable-local-server test runs must
		// still reach their own handlers with zero /v1/owners?name= lookups.
		// A --local command dials the dev server's URL, http://localhost:1/v1/…;
		// the dial error names whichever address localhost resolved to first
		// ([::1] on the CI runner and here), so match the URL, not the address.
		const localServer = ":1/v1/"
		for _, tc := range []struct {
			args     []string
			extraEnv []string
			wantErr  string
		}{
			{args: []string{"edit", localFile}, wantErr: "interactive terminal"},
			{args: []string{localFile}, wantErr: "interactive terminal"},
			{args: []string{"analytics", "send", filepath.Join(t.TempDir(), "missing-event.json")}, wantErr: ""},
			{args: []string{"workflows", "init", "-o", "json"}, wantErr: "not supported"},
			{args: []string{"workflows", "dev"}, wantErr: "requires at least 1 arg"},
			{args: []string{"workflows", "tasks", "list", "--local", "--port", "1", "-o", "json"}, wantErr: localServer},
			{args: []string{"workflows", "tasks", "runs", "list", "--local", "--port", "1", "-o", "json"}, wantErr: localServer},
			{args: []string{"workflows", "start", "my-task", "--local", "--port", "1", "--input", "[]", "-o", "json"}, wantErr: localServer},
			{args: []string{"workflows", "cancel", "trn-1234", "--local", "--port", "1", "-o", "json"}, wantErr: localServer},
			{args: []string{"workflows", "flows", "list", "--local", "--port", "1", "-o", "json"}, wantErr: localServer},
			{args: []string{"s3", "ls", "-o", "json"}, extraEnv: s3KeyEnv, wantErr: ""},
			{args: []string{"s3", "info", "s3://photos/cat.jpg", "-o", "json"}, extraEnv: s3KeyEnv, wantErr: ""},
			{args: []string{"s3", "info", "-o", "json", "s3://photos/cat.jpg"}, extraEnv: s3KeyEnv, wantErr: ""},
			{args: []string{"s3", "cors", "get", "photos", "-o", "json"}, extraEnv: s3KeyEnv, wantErr: ""},
		} {
			code, stderr := runWithEnv(workspace, tc.extraEnv, tc.args...)
			if code == 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %v unexpectedly exited 0", workspace, tc.args)
			}
			if strings.Contains(stderr, "BEX_WORKSPACE") {
				t.Errorf("BEX_WORKSPACE=%s bex %v failed on workspace resolution: %s", workspace, tc.args, stderr)
			}
			if tc.wantErr != "" && !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("BEX_WORKSPACE=%s bex %v stderr = %q, want substring %q", workspace, tc.args, stderr, tc.wantErr)
			}
			if n := lookups(); n != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %v looked the name up %d times; it never reads the active workspace", workspace, tc.args, n)
			}
		}

		// `bex s3 env` with explicit S3 env keys succeeds with zero workspace lookups.
		if code, stderr := runWithEnv(workspace, s3KeyEnv, "s3", "env", "-o", "json"); code != 0 {
			t.Errorf("BEX_WORKSPACE=%s bex s3 env (with S3 keys) exited %d: %s", workspace, code, stderr)
		}
		if n := lookups(); n != 0 {
			t.Errorf("BEX_WORKSPACE=%s bex s3 env (with S3 keys) looked the name up %d times", workspace, n)
		}
	}

	for _, workspace := range []string{"stale-ws", "dup-ws"} {
		// Direct-ID commands whose API route does not read config.WorkspaceID()
		// try the name once but are not blocked by an unresolvable one (w8/064, w32/117).
		for _, args := range [][]string{
			{"pg", "get", "-o", "json", dbID},
			{"kv", "get", "-o", "json", kvID},
			{"deploys", "list", srvID, "-o", "json"},
			{"sandbox-groups", "delete", sbgID, "-o", "json"},
			{"sandbox-groups", "images", "get", sbbID, "--group", sbgID, "-o", "json"},
			{"sandbox-groups", "builds", "get", sbbID, "--group", sbgID, "-o", "json"},
			{"sandbox-groups", "credentials", "get", crdID, "--group", sbgID, "-o", "json"},
			{"agent", "get", agsID, "-o", "json"},
		} {
			if code, stderr := run(workspace, args...); code != 0 {
				t.Errorf("BEX_WORKSPACE=%s bex %v exited %d: %s", workspace, args, code, stderr)
			}
			if n := lookups(); n != 1 {
				t.Errorf("BEX_WORKSPACE=%s direct-id %v made %d name lookups, want it to try once", workspace, args, n)
			}
		}
		// `ea blueprints plan <exs-id>` also addresses the blueprint directly without ownerId.
		if code, stderr := run(workspace, "ea", "blueprints", "plan", exsID, "-o", "json"); code == 0 || strings.Contains(stderr, "BEX_WORKSPACE") {
			t.Errorf("BEX_WORKSPACE=%s bex ea blueprints plan %s = %d %q, want direct blueprint error (not BEX_WORKSPACE)", workspace, exsID, code, stderr)
		}
		if n := lookups(); n != 1 {
			t.Errorf("BEX_WORKSPACE=%s ea blueprints plan made %d name lookups, want 1", workspace, n)
		}
	}

	// A final-empty --workspace falls back to the active workspace, so its
	// name must still resolve to the ID upstream sends as ownerId; a final
	// nonempty one is used as given (w8/078).
	owners := func() []string { return drain(&sentOwners) }
	_ = owners()
	for _, tc := range []struct {
		args    []string
		lookups int
	}{
		{[]string{"blueprints", "validate", blueprint}, 1},
		{[]string{"blueprints", "validate", blueprint, "--workspace="}, 1},
		{[]string{"blueprints", "validate", blueprint, "--workspace", ""}, 1},
		{[]string{"blueprints", "validate", blueprint, "-w", ""}, 1},
		{[]string{"blueprints", "validate", blueprint, "--workspace", teamID, "--workspace="}, 1},
		{[]string{"blueprints", "validate", blueprint, "--workspace=", "--workspace", teamID}, 0},
	} {
		args := append(tc.args, "-o", "json")
		if code, stderr := run("known", args...); code != 0 {
			t.Errorf("BEX_WORKSPACE=known bex %v exited %d: %s", args, code, stderr)
		}
		if n := lookups(); n != tc.lookups {
			t.Errorf("BEX_WORKSPACE=known bex %v made %d name lookups, want %d", args, n, tc.lookups)
		}
		if got := owners(); len(got) != 1 || got[0] != teamID {
			t.Errorf("BEX_WORKSPACE=known bex %v sent ownerId %q, want %s", args, got, teamID)
		}
	}
	for _, create := range [][]string{
		{"pg", "create", "--name", "db", "--plan", "free", "--version", "18"},
		{"kv", "create", "--name", "kv", "--plan", "free"},
	} {
		for _, workspace := range [][]string{nil, {"--workspace="}} {
			args := append(append(append([]string(nil), create...), workspace...), "--confirm", "-o", "json")
			_, _ = run("known", args...) // the fixture refuses the create itself
			if n := lookups(); n != 1 {
				t.Errorf("BEX_WORKSPACE=known bex %v made %d name lookups, want 1", args, n)
			}
			if got := owners(); len(got) != 1 || got[0] != teamID {
				t.Errorf("BEX_WORKSPACE=known bex %v sent ownerId %q, want %s", args, got, teamID)
			}
		}
	}

	// Consumers still refuse an unresolvable name rather than guess, including
	// commands that superficially resemble exempted ones (`ea objects --local`,
	// remote `workflows`, `s3` without both env keys, workspace-scoped `s3`
	// management commands even when S3 env keys are set) and typed-ID commands
	// that still read the active workspace as ownerId (w32/117).
	objectOut := filepath.Join(t.TempDir(), "downloaded.bin")
	for _, tc := range []struct {
		args     []string
		extraEnv []string
	}{
		{args: []string{"services", "-o", "json"}},
		{args: []string{"workspace", "current", "-o", "json"}},
		{args: []string{"ea", "objects", "list", "--local", "--region", "frankfurt", "-o", "json"}},
		{args: []string{"workflows", "tasks", "list", "-o", "json"}},
		{args: []string{"s3", "env", "-o", "json"}},
		{args: []string{"s3", "env", "-o", "json"}, extraEnv: []string{"BEX_S3_ACCESS_KEY_ID=s3k-only"}},
		{args: []string{"s3", "keys", "list", "-o", "json"}, extraEnv: s3KeyEnv},
		{args: []string{"s3", "public", "photos", "status", "-o", "json"}, extraEnv: s3KeyEnv},
		{args: []string{"s3", "info", "photos", "-o", "json"}, extraEnv: s3KeyEnv},
		{args: []string{"s3", "info", "s3://photos", "-o", "json"}, extraEnv: s3KeyEnv},
		// Typed-ID commands that still pass config.WorkspaceID() as ownerId (w32/117).
		{args: []string{"sandboxes", "stop", sbxID, "--confirm", "-o", "json"}},
		{args: []string{"sandboxes", "exec", sbxID, "--", "true"}},
		{args: []string{"sandboxes", "snapshots", "create", sbxID, "--kind", "filesystem", "-o", "json"}},
		{args: []string{"sandboxes", "snapshots", "get", snpID, "-o", "json"}},
		{args: []string{"sandboxes", "snapshots", "get", snpID, "--group", sbgID, "-o", "json"}},
		{args: []string{"sandboxes", "snapshots", "delete", snpID, "--confirm", "-o", "json"}},
		{args: []string{"sandboxes", "snapshots", "delete", snpID, "--group", sbgID, "--confirm", "-o", "json"}},
		{args: []string{"sandbox-groups", "images", "get", sbbID, "-o", "json"}},
		{args: []string{"sandbox-groups", "images", "delete", sbbID, "--confirm", "-o", "json"}},
		{args: []string{"sandbox-groups", "builds", "get", sbbID, "-o", "json"}},
		{args: []string{"sandbox-groups", "credentials", "get", crdID, "-o", "json"}},
		{args: []string{"sandbox-groups", "environment", "set", sbgID, evmID, "-o", "json"}},
		{args: []string{"pg", "get", dbID, "--project", "proj", "-o", "json"}},
		{args: []string{"pg", "get", dbID, "--environment", "prod", "-o", "json"}},
		{args: []string{"kv", "get", kvID, "--project", "proj", "-o", "json"}},
		{args: []string{"kv", "get", kvID, "--environment", "prod", "-o", "json"}},
		{args: []string{"agent", "automations", "get", autID, "-o", "json"}},
		{args: []string{"agent", "skills", "get", sklID, "-o", "json"}},
		{args: []string{"agent", "secrets", "delete", secID, "--confirm", "-o", "json"}},
		{args: []string{"ea", "objects", "get", objID, "--file", objectOut, "--region", "frankfurt", "-o", "json"}},
		{args: []string{"s3", "info", bktID, "-o", "json"}},
		{args: []string{"s3", "public", bktID, "status", "-o", "json"}},
		{args: []string{"s3", "keys", "revoke", bktID, "--confirm", "-o", "json"}},
	} {
		code, stderr := runWithEnv("stale-ws", tc.extraEnv, tc.args...)
		if code != 1 || !strings.Contains(stderr, `no workspace named "stale-ws"`) {
			t.Errorf("stale bex %v = %d %q, want the unknown-name refusal", tc.args, code, stderr)
		}
		if n := lookups(); n != 1 {
			t.Errorf("stale bex %v made %d name lookups, want 1", tc.args, n)
		}
	}
	if leaked := drain(&leakedOwners); len(leaked) != 0 {
		t.Errorf("unresolved BEX_WORKSPACE leaked into API requests: %v", leaked)
	}
	if code, stderr := run("dup-ws", "services", "-o", "json"); code != 1 || !strings.Contains(stderr, `several workspaces are named "dup-ws"`) {
		t.Errorf("ambiguous services = %d %q, want the ambiguity refusal", code, stderr)
	}
	_ = lookups()
	if code, stderr := run("known", "services", "-o", "json"); code != 0 {
		t.Errorf("known services exited %d: %s", code, stderr)
	}
	_ = lookups()

	// A detached `bex analytics send` child process spawned by a telemetry-emitting
	// command must not perform an extra GET /v1/owners?name= lookup even when
	// BEX_WORKSPACE holds a valid workspace name (w32/116).
	t.Run("detached analytics send makes zero workspace lookups", func(t *testing.T) {
		var mu sync.Mutex
		var telemetryLookups []string
		stub := &telemetryStub{}
		baseHandler := stub.handler()
		telemetryAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/owners" && r.URL.Query().Get("name") != "" {
				mu.Lock()
				telemetryLookups = append(telemetryLookups, r.URL.Query().Get("name"))
				mu.Unlock()
			}
			baseHandler.ServeHTTP(w, r)
		}))
		t.Cleanup(telemetryAPI.Close)

		home := t.TempDir()
		seedNoticeMarker(t, home)
		command := exec.Command(buildBex(), "workspaces", "-o", "json")
		command.Env = telemetryEnv(t, home, telemetryAPI.URL, "BEX_WORKSPACE=known")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("bex workspaces: %v\n%s", err, output)
		}
		awaitEvents(t, stub, 1)
		mu.Lock()
		got := append([]string(nil), telemetryLookups...)
		mu.Unlock()
		if len(got) != 0 {
			t.Errorf("detached analytics send made %d /v1/owners?name= lookups (%v), want 0", len(got), got)
		}
	})

	// A name that only starts with the workspace id prefix is still a name:
	// it used to be sent as ownerId and refused (w32/012).
	_, _ = lookups(), owners() // forget what the runs above recorded
	if code, stderr := run("tea-house", "blueprints", "validate", blueprint, "-o", "json"); code != 0 {
		t.Errorf("BEX_WORKSPACE=tea-house blueprints validate exited %d: %s", code, stderr)
	}
	if looked, sent := drain(&nameLookups), owners(); !slices.Equal(looked, []string{"tea-house"}) || !slices.Equal(sent, []string{teamID}) {
		t.Errorf("BEX_WORKSPACE=tea-house looked up %q and sent ownerId %q, want one lookup of the name and %s", looked, sent, teamID)
	}
}

// TestPostgresHelpNeedsNoConnectionInfo: psql/pgcli help with a database
// selector prints help with no request even while the database's CA is
// unavailable, and a real session is still refused (w8/065).
func TestPostgresHelpNeedsNoConnectionInfo(t *testing.T) {
	const dbID = "dpg-12345678901234567890"
	var mu sync.Mutex
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"external endpoint is not provisioned"}`))
	}))
	t.Cleanup(api.Close)
	run := func(args ...string) (int, string) {
		t.Helper()
		home := t.TempDir()
		command := exec.Command(buildBex(), args...)
		command.Env = append(withoutRenderEnv(updateTestEnv(t.TempDir())),
			"HOME="+home, "BEX_HOST="+api.URL+"/v1/", "BEX_ACCESS_TOKEN=test-access-token",
			"BEX_WORKSPACE=tea-12345678901234567890", "PGSSLROOTCERT=", "BEX_NO_UPDATE_NOTIFIER=1", "BEX_CLI_DISABLE_ANALYTICS=1")
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if _, statErr := os.Stat(filepath.Join(home, ".bex", "postgres")); code == 0 && statErr == nil {
			t.Errorf("%v left trust material under the home directory", args)
		}
		return code, stdout.String() + stderr.String()
	}
	served := func() int {
		mu.Lock()
		defer mu.Unlock()
		n := requests
		requests = 0
		return n
	}

	for _, args := range [][]string{
		{"psql", dbID, "--help"}, {"psql", "--help", dbID}, {"psql", dbID, "-h"}, {"pgcli", dbID, "--help"}, {"psql", "--help"},
	} {
		if code, out := run(args...); code != 0 || !strings.Contains(out, "bex "+args[0]+" [postgresID|postgresName]") {
			t.Errorf("bex %v = %d, want help:\n%s", args, code, out)
		}
		if n := served(); n != 0 {
			t.Errorf("bex %v made %d requests; help needs none", args, n)
		}
	}
	if code, out := run("psql", dbID, "-c", "SELECT 1", "-o", "text"); code != 1 || !strings.Contains(out, "TLS server CA is not available yet") {
		t.Errorf("real session = %d %q, want the unavailable-CA refusal", code, out)
	}
	if n := served(); n != 1 {
		t.Errorf("real session made %d connection-info requests, want 1", n)
	}
}
