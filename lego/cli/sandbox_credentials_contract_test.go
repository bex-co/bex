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

package main_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sandboxCredentialsContract mirrors lego/cli/testdata/sandbox-credentials-contract.json,
// the golden bex-api's 7 sandbox-group credential routes are proven to emit
// (lego/backend/internal/sandbox/credentials_contract_test.go). Running all 7
// pinned `bex sandbox-groups credentials` commands against it ensures any CLI
// pin bump or API wire drift fails CI immediately (w2/m536 t002).
type sandboxCredentialsContract struct {
	GroupID      string `json:"groupId"`
	CredentialID string `json:"credentialId"`
	Create       struct {
		Method      string         `json:"method"`
		Path        string         `json:"path"`
		RequestBody map[string]any `json:"requestBody"`
		Status      int            `json:"status"`
		Response    map[string]any `json:"response"`
	} `json:"create"`
	List struct {
		Method   string           `json:"method"`
		Path     string           `json:"path"`
		Status   int              `json:"status"`
		Response []map[string]any `json:"response"`
	} `json:"list"`
	Get struct {
		Method   string         `json:"method"`
		Path     string         `json:"path"`
		Status   int            `json:"status"`
		Response map[string]any `json:"response"`
	} `json:"get"`
	Update struct {
		Method      string         `json:"method"`
		Path        string         `json:"path"`
		RequestBody map[string]any `json:"requestBody"`
		Status      int            `json:"status"`
		Response    map[string]any `json:"response"`
	} `json:"update"`
	Versions struct {
		Method   string           `json:"method"`
		Path     string           `json:"path"`
		Status   int              `json:"status"`
		Response []map[string]any `json:"response"`
	} `json:"versions"`
	Restore struct {
		Method      string         `json:"method"`
		Path        string         `json:"path"`
		RequestBody map[string]any `json:"requestBody"`
		Status      int            `json:"status"`
		Response    map[string]any `json:"response"`
	} `json:"restore"`
	Delete struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Status int    `json:"status"`
	} `json:"delete"`
}

func loadSandboxCredentialsContract(t *testing.T) sandboxCredentialsContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-credentials-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c sandboxCredentialsContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

type credCall struct {
	method, path, query string
	body                map[string]any
}

func credentialsContractServer(t *testing.T, c sandboxCredentialsContract) (*httptest.Server, func() []credCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []credCall
	reply := func(w http.ResponseWriter, status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		calls = append(calls, credCall{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
		mu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case c.Create.Method + " " + c.Create.Path:
			reply(w, c.Create.Status, c.Create.Response)
		case c.List.Method + " " + c.List.Path:
			reply(w, c.List.Status, c.List.Response)
		case c.Get.Method + " " + c.Get.Path:
			reply(w, c.Get.Status, c.Get.Response)
		case c.Update.Method + " " + c.Update.Path:
			reply(w, c.Update.Status, c.Update.Response)
		case c.Versions.Method + " " + c.Versions.Path:
			reply(w, c.Versions.Status, c.Versions.Response)
		case c.Restore.Method + " " + c.Restore.Path:
			reply(w, c.Restore.Status, c.Restore.Response)
		case c.Delete.Method + " " + c.Delete.Path:
			w.WriteHeader(c.Delete.Status)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []credCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]credCall(nil), calls...)
	}
}

func TestPinnedSandboxCredentialsAllSevenCommands(t *testing.T) {
	c := loadSandboxCredentialsContract(t)
	srv, calls := credentialsContractServer(t, c)
	secretVal, _ := c.Create.RequestBody["value"].(string)

	assertNoSecretOrRenderLeak := func(t *testing.T, label, stdout, stderr string) {
		t.Helper()
		combined := stdout + "\n" + stderr
		if secretVal != "" && strings.Contains(combined, secretVal) {
			t.Errorf("%s leaked secret value in CLI output", label)
		}
		if strings.Contains(combined, "render sandbox-groups") {
			t.Errorf("%s leaked bare Render CLI branding: %q", label, combined)
		}
	}

	// 1. create (JSON)
	stdout, stderr, err := runBexAgainst(t, srv.URL,
		"sandbox-groups", "credentials", "create", c.CredentialID,
		"--group", c.GroupID,
		"--domain", "api.example.com",
		"--value", secretVal,
		"--injection-type", "header",
		"--injection-name", "Authorization",
		"--injection-template", "Bearer ${SECRET}",
		"--methods", "GET,POST",
		"--paths", "/v1/*,/v2/**",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("create failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	assertNoSecretOrRenderLeak(t, "create", stdout, stderr)
	var created map[string]any
	if err := json.Unmarshal([]byte(stdout), &created); err != nil {
		t.Fatalf("create json: %v (%q)", err, stdout)
	}
	if created["id"] != c.CredentialID || created["domain"] != "api.example.com" || created["enabled"] != true {
		t.Fatalf("create decoded unexpected object: %v", created)
	}

	// 2. list (JSON + text)
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "list", "--group", c.GroupID, "-o", "json")
	if err != nil {
		t.Fatalf("list json failed: %v\n%s", err, stderr)
	}
	assertNoSecretOrRenderLeak(t, "list json", stdout, stderr)
	var listed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil || len(listed) != 1 || listed[0]["id"] != c.CredentialID {
		t.Fatalf("list json = %q (%v)", stdout, err)
	}
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "list", "--group", c.GroupID, "-o", "text")
	if err != nil || !strings.Contains(stdout, c.CredentialID) {
		t.Fatalf("list text failed: err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	assertNoSecretOrRenderLeak(t, "list text", stdout, stderr)

	// 3. get (JSON)
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "get", c.CredentialID, "--group", c.GroupID, "-o", "json")
	if err != nil {
		t.Fatalf("get failed: %v\n%s", err, stderr)
	}
	assertNoSecretOrRenderLeak(t, "get", stdout, stderr)
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || got["id"] != c.CredentialID {
		t.Fatalf("get json = %q (%v)", stdout, err)
	}

	// 4. update (JSON: disable + clear methods/paths)
	stdout, stderr, err = runBexAgainst(t, srv.URL,
		"sandbox-groups", "credentials", "update", c.CredentialID,
		"--group", c.GroupID,
		"--enabled=false",
		"--methods=",
		"--paths=",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("update failed: %v\n%s", err, stderr)
	}
	assertNoSecretOrRenderLeak(t, "update", stdout, stderr)
	var updated map[string]any
	if err := json.Unmarshal([]byte(stdout), &updated); err != nil || updated["enabled"] != false {
		t.Fatalf("update json = %q (%v)", stdout, err)
	}

	// 5. versions (JSON)
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "versions", c.CredentialID, "--group", c.GroupID, "-o", "json")
	if err != nil {
		t.Fatalf("versions failed: %v\n%s", err, stderr)
	}
	assertNoSecretOrRenderLeak(t, "versions", stdout, stderr)
	var versions []map[string]any
	if err := json.Unmarshal([]byte(stdout), &versions); err != nil || len(versions) != 2 {
		t.Fatalf("versions json = %q (%v)", stdout, err)
	}

	// 6. restore (JSON)
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "restore", c.CredentialID, "--group", c.GroupID, "--version", "1", "-o", "json")
	if err != nil {
		t.Fatalf("restore failed: %v\n%s", err, stderr)
	}
	assertNoSecretOrRenderLeak(t, "restore", stdout, stderr)
	var restored map[string]any
	if err := json.Unmarshal([]byte(stdout), &restored); err != nil {
		t.Fatalf("restore json = %q (%v)", stdout, err)
	}
	ver, _ := restored["version"].(map[string]any)
	if ver["restoredFromVersionId"] != float64(1) {
		t.Fatalf("restore version = %v, want restoredFromVersionId=1", ver)
	}

	// 7. delete (--confirm, 202 empty body)
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "credentials", "delete", c.CredentialID, "--group", c.GroupID, "--confirm", "-o", "text")
	if err != nil {
		t.Fatalf("delete failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	assertNoSecretOrRenderLeak(t, "delete", stdout, stderr)

	// Verify exact HTTP method, path, and key request payload fields sent by the pinned CLI.
	seen := calls()
	if len(seen) != 8 {
		t.Fatalf("recorded %d API calls, want 8 (list called twice): %+v", len(seen), seen)
	}
	if seen[0].method != c.Create.Method || seen[0].path != c.Create.Path {
		t.Errorf("create call = %s %s, want %s %s", seen[0].method, seen[0].path, c.Create.Method, c.Create.Path)
	}
	if seen[0].body["id"] != c.CredentialID || seen[0].body["domain"] != "api.example.com" || seen[0].body["value"] != secretVal {
		t.Errorf("create request body = %v", seen[0].body)
	}
	if seen[4].method != c.Update.Method || seen[4].path != c.Update.Path || seen[4].body["enabled"] != false {
		t.Errorf("update call = %+v, want %s %s with enabled=false", seen[4], c.Update.Method, c.Update.Path)
	}
	if seen[6].method != c.Restore.Method || seen[6].path != c.Restore.Path || seen[6].body["versionId"] != float64(1) {
		t.Errorf("restore call = %+v, want %s %s with versionId=1", seen[6], c.Restore.Method, c.Restore.Path)
	}
	if seen[7].method != c.Delete.Method || seen[7].path != c.Delete.Path {
		t.Errorf("delete call = %s %s, want %s %s", seen[7].method, seen[7].path, c.Delete.Method, c.Delete.Path)
	}
}

func TestPinnedSandboxCredentialsHelpUsesBexBranding(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	for _, args := range [][]string{
		{"sandbox-groups", "credentials", "--help"},
		{"sandbox-groups", "credentials", "create", "--help"},
		{"sandbox-groups", "credentials", "list", "--help"},
		{"sandbox-groups", "credentials", "get", "--help"},
		{"sandbox-groups", "credentials", "update", "--help"},
		{"sandbox-groups", "credentials", "versions", "--help"},
		{"sandbox-groups", "credentials", "restore", "--help"},
		{"sandbox-groups", "credentials", "delete", "--help"},
	} {
		stdout, stderr, err := runBexAgainst(t, srv.URL, args...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, stderr)
		}
		if strings.Contains(stdout+stderr, "render sandbox-groups") {
			t.Errorf("%v help leaked bare render branding:\n%s", args, stdout)
		}
		if !strings.Contains(stdout, "bex sandbox-groups credentials") {
			t.Errorf("%v help missing bex branding:\n%s", args, stdout)
		}
	}
}
