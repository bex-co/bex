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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/cli/internal/bridge"
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
	if !strings.Contains(string(output), "run `render login` to authenticate") {
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
	if output, err := logout.CombinedOutput(); err != nil {
		t.Fatalf("bex logout: %v\n%s", err, output)
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

func withoutRenderEnv(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, item := range environment {
		if strings.HasPrefix(item, "RENDER_") {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// updateTestEnv strips everything that gates or targets the update check so
// each test controls those inputs explicitly. CI in particular is set on
// GitHub Actions runners and would silence the check.
func updateTestEnv(home string) []string {
	filtered := make([]string, 0, len(os.Environ()))
	for _, item := range withoutRenderEnv(os.Environ()) {
		name, _, _ := strings.Cut(item, "=")
		switch name {
		case "CI", "HOME", "BEX_NO_UPDATE_NOTIFIER", "BEX_UPDATE_API_URL":
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, "HOME="+home)
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
// minimal workspace list the CLI needs to complete a command.
type telemetryStub struct {
	mu sync.Mutex
	// headers records the release header seen on every request, telemetry or
	// not, so a test can prove the stamp reaches both request classes.
	headers []string
	bodies  []string
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
			http.NotFound(w, r)
		}
	})
}

func (s *telemetryStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

// seedNoticeMarker pre-creates the upstream one-time-notice marker under the
// test HOME so a headless run proceeds to the send path: without a TTY the
// sender skips delivery until a marker proves the notice was shown once.
func seedNoticeMarker(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, ".render", "state", "analytics")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notice-shown"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
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
// harness-wide opt-out (blank counts as unset in the bridge).
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
	// CI=1 keeps the release-channel update check off the network without
	// touching the telemetry path.
	command.Env = telemetryEnv(t, home, api.URL, "CI=1")
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
			version.Env = telemetryEnv(t, home, api.URL, "CI=1", optOut)
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

// Exercise the imported tree in fresh processes, including commands registered
// after branding.Apply and flag descriptions rendered by upstream's template.
func TestBexNestedHelp(t *testing.T) {
	cases := []struct {
		command string
		want    []string
		absent  []string
	}{
		{"workspace set", []string{"$HOME/.bex/cli.yaml", "BEX_CLI_CONFIG_DIR", "BEX_CLI_CONFIG_PATH", "takes precedence over BEX_CLI_CONFIG_DIR", "RENDER_CLI_CONFIG_PATH overrides both Bex inputs"}, []string{"$HOME/.render", "RENDER_CLI_CONFIG_DIR"}},
		{"blueprints validate", []string{"render.yaml", "bex blueprints validate"}, []string{"bex.yaml"}},
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
			command := exec.Command(buildBex(), append(strings.Fields(tc.command), "--help")...)
			for _, item := range updateTestEnv(t.TempDir()) {
				if !strings.HasPrefix(item, "BEX_") {
					command.Env = append(command.Env, item)
				}
			}
			command.Env = append(command.Env, "BEX_NO_UPDATE_NOTIFIER=1", "BEX_CLI_DISABLE_ANALYTICS=1")
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				t.Fatalf("help failed: %v\n%s", err, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %s", stderr.String())
			}
			text := strings.Join(strings.Fields(stdout.String()), " ")
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
