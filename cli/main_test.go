package main_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bexBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bex-cli-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	bexBinary = filepath.Join(dir, "bex")
	build := exec.Command("go", "build", "-o", bexBinary, ".")
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

func buildBex() string {
	return bexBinary
}
