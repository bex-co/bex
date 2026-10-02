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

package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The unconditional REST matrix covers intent and persistence; this opt-in
// check exercises the actual imported CLI's create builder and text formatter.
func TestOfficialCLIPostgresDefaultAccessReadback(t *testing.T) {
	bin := os.Getenv("RENDER_CLI_BIN")
	if bin == "" {
		t.Skip("set RENDER_CLI_BIN to an official Render or Bex binary")
	}
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	configDir := t.TempDir()
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RENDER_HOST=" + server.URL + "/v1/", "BEX_HOST=" + server.URL + "/v1/", "RENDER_API_KEY=local-contract-fixture", "RENDER_WORKSPACE=default", "RENDER_CLI_CONFIG_PATH=" + filepath.Join(configDir, "cli.yaml"), "RENDER_CLI_CONFIG_DIR=" + configDir, "BEX_CLI_CONFIG_DIR=" + configDir, "RENDER_CLI_DISABLE_ANALYTICS=true", "BEX_CLI_DISABLE_ANALYTICS=true", "DO_NOT_TRACK=1"}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("CLI %v: %v: %s", args, err, out)
		}
		return out
	}
	out := run("postgres", "create", "--name", "default-access", "--plan", "free", "--confirm", "-o", "json")
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	if data, ok := envelope["data"]; ok {
		out = data
	}
	var pg PostgresView
	if err := json.Unmarshal(out, &pg); err != nil {
		t.Fatal(err)
	}
	if pg.ID == "" || len(pg.IPAllowList) != 2 {
		t.Fatalf("default create rules = %+v", pg.IPAllowList)
	}
	text := string(run("postgres", "get", pg.ID, "-o", "text"))
	if strings.Contains(text, "external connections blocked") || !strings.Contains(text, "0.0.0.0/0") || !strings.Contains(text, "::/0") {
		t.Fatalf("default readback: %s", text)
	}
	run("postgres", "update", pg.ID, "--clear-ip-allow-list", "--confirm", "-o", "json")
	if text := string(run("postgres", "get", pg.ID, "-o", "text")); !strings.Contains(text, "external connections blocked") {
		t.Fatalf("cleared readback: %s", text)
	}
	run("postgres", "delete", pg.ID, "--confirm", "-o", "json")
	if rec := serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("cleanup: %d", rec.Code)
	}
}
