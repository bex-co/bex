package main_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bexSandboxID is a sandbox id in exactly the shape bex-api mints
// (lego/backend/internal/id: "sbx-" + a 20-char xid). It is written out rather
// than imported because the CLI module deliberately depends on no bex server
// package; bexSandboxIDShape below keeps this literal honest against the
// documented contract.
const bexSandboxID = "sbx-daoco4pjg4r481aljit0"

var bexSandboxIDShape = regexp.MustCompile(`^sbx-[0-9a-v]{20}$`)

// The pinned CLI parses a copy argument's sandbox id CLIENT-SIDE
// (cmd/sandboxcopy.go: `^(sbx-[A-Za-z0-9]+):(.*)$`) and never sends the
// request when it does not match. Production once minted the substrate's bare
// UUID as the public id, so `ea sandboxes copy` failed before any byte left the
// machine (w9/m94). These tests drive the real bex binary so a pin bump — or a
// server regression back to UUIDs — is caught here, not in a live hunt.

func TestSandboxCopyAcceptsBexMintedID(t *testing.T) {
	if !bexSandboxIDShape.MatchString(bexSandboxID) {
		t.Fatalf("fixture id %q is not a bex id (sbx- + 20-char xid)", bexSandboxID)
	}
	var gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/files/upload/token") {
			gotPath = r.URL.Path
		}
		// w7/m150 owns the transfer routes; until they exist the server 404s.
		// Reaching the server at all is what this test asserts.
		http.NotFound(w, r)
	}))
	t.Cleanup(api.Close)

	local := filepath.Join(t.TempDir(), "probe.txt")
	if err := os.WriteFile(local, []byte("probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, _ := runBexCopy(t, api.URL, local, bexSandboxID+":/tmp/probe.txt")
	if strings.Contains(stderr, "must be a sandbox path") {
		t.Fatalf("pinned client rejected a bex-minted id client-side:\n%s", stderr)
	}
	want := "/v1/sandboxes/" + bexSandboxID + "/files/upload/token"
	if gotPath != want {
		t.Errorf("upload token path = %q, want %q\nstdout:\n%s\nstderr:\n%s", gotPath, want, stdout, stderr)
	}
}

func TestSandboxCopyRejectsBareUUID(t *testing.T) {
	var hits int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.NotFound(w, r)
	}))
	t.Cleanup(api.Close)

	local := filepath.Join(t.TempDir(), "probe.txt")
	if err := os.WriteFile(local, []byte("probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The exact id shape production used to mint (live hunt 2026-09-20). The
	// client refuses it before any request — which is why bex-api must never
	// return one.
	_, stderr, err := runBexCopy(t, api.URL, local, "c372c97e-980b-4573-9aae-4c82b8ad1c4b:/tmp/probe.txt")
	if err == nil {
		t.Fatalf("copy with a bare UUID unexpectedly succeeded:\n%s", stderr)
	}
	if !strings.Contains(stderr, "must be a sandbox path") {
		t.Errorf("stderr = %q, want the pinned client's sandbox-path rejection", stderr)
	}
	if hits != 0 {
		t.Errorf("client sent %d request(s) for an unparseable id; it must refuse locally", hits)
	}
}

func runBexCopy(t *testing.T, apiURL, src, dst string) (stdout, stderr string, err error) {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, ".bex", "cli.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// A copy needs a selected workspace before it addresses a sandbox.
	config := "version: 1\nworkspace: tea-bex\nworkspace_name: Bex\napi:\n  key: test-access-token\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(buildBex(), "ea", "sandboxes", "copy", src, dst, "-o", "json")
	command.Env = append(withoutRenderEnv(os.Environ()),
		"HOME="+home,
		"BEX_HOST="+apiURL+"/v1/",
		"BEX_ACCESS_TOKEN=test-access-token",
	)
	var out, errOut bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errOut
	err = command.Run()
	return out.String(), errOut.String(), err
}
