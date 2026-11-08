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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// blueprintAPI answers POST /v1/blueprints with outcome, for a Blueprint whose
// data is deliberately full of render-shaped values that must survive.
func blueprintAPI(t *testing.T, outcome string) *httptest.Server {
	t.Helper()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/blueprints" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"blueprint": map[string]any{
				"id": "exs-runtime1", "name": "render-x", "repo": "https://render.com/acme/render-x",
				"branch": "main", "path": "render.yaml", "ownerId": "tea-bex", "status": "created", "autoSync": true,
			},
			"planningOutcome": outcome,
			"sync":            nil,
			"notices": []map[string]any{{
				"severity": "warning",
				"message":  "service render-x reads RENDER_EXTERNAL_URL from render.yaml",
			}},
		})
	}))
	t.Cleanup(api.Close)
	return api
}

// runAgainst runs bex against api and returns stdout and stderr; the exit
// code is the caller's concern (a failed plan exits 1 by design).
func runAgainst(t *testing.T, apiURL string, args ...string) (string, string) {
	t.Helper()
	command := bexAgainst(t, apiURL, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	_ = command.Run()
	return stdout.String(), stderr.String()
}

// The data values a rewrite must never touch.
var renderShapedData = []string{"render-x", "render.yaml", "https://render.com/acme/render-x", "RENDER_EXTERNAL_URL"}

func TestBlueprintNextStepsNameBex(t *testing.T) {
	create := []string{"ea", "blueprints", "create", "--repo", "https://github.com/acme/render-x", "--branch", "main"}

	t.Run("text", func(t *testing.T) {
		stdout, stderr := runAgainst(t, blueprintAPI(t, "planned").URL, append(create, "-o", "text")...)
		if !strings.Contains(stdout, "Next:\n  bex ea blueprints approve exs-runtime1") {
			t.Fatalf("text next step does not name bex:\n%s\nstderr: %s", stdout, stderr)
		}
		if strings.Contains(stdout, "render ea ") {
			t.Fatalf("text output still names render:\n%s", stdout)
		}
		for _, data := range renderShapedData {
			if !strings.Contains(stdout, data) {
				t.Errorf("text output lost data %q:\n%s", data, stdout)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		stdout, stderr := runAgainst(t, blueprintAPI(t, "no_changes").URL, append(create, "-o", "json")...)
		var out struct {
			Data struct {
				Blueprint struct{ Name, Repo, Path string }
				Notices   []struct{ Message string }
			}
			Meta struct{ Next string }
		}
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("json output: %v\n%s\nstderr: %s", err, stdout, stderr)
		}
		want := `Nothing to approve. If this is unexpected, update "render.yaml" and push your changes to the remote "main" branch, then re-plan with "bex ea blueprints plan exs-runtime1".`
		if out.Meta.Next != want {
			t.Fatalf("meta.next = %q, want %q", out.Meta.Next, want)
		}
		if bp := out.Data.Blueprint; bp.Name != "render-x" || bp.Repo != "https://render.com/acme/render-x" || bp.Path != "render.yaml" ||
			len(out.Data.Notices) != 1 || out.Data.Notices[0].Message != "service render-x reads RENDER_EXTERNAL_URL from render.yaml" {
			t.Fatalf("data changed: %+v", out.Data)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		stdout, stderr := runAgainst(t, blueprintAPI(t, "failed").URL, append(create, "-o", "yaml")...)
		var out struct {
			Data struct {
				Blueprint struct{ Name, Path string }
			}
			Meta struct{ Next string }
		}
		if err := yaml.Unmarshal([]byte(stdout), &out); err != nil {
			t.Fatalf("yaml output: %v\n%s\nstderr: %s", err, stdout, stderr)
		}
		if out.Meta.Next != `fix the errors above, then run "bex ea blueprints plan exs-runtime1"` {
			t.Fatalf("meta.next = %q", out.Meta.Next)
		}
		if out.Data.Blueprint.Name != "render-x" || out.Data.Blueprint.Path != "render.yaml" {
			t.Fatalf("data changed: %+v", out.Data)
		}
	})
}

func TestWorkspaceCurrentWithoutWorkspaceNamesBex(t *testing.T) {
	api := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(api.Close)
	command := bexAgainst(t, api.URL, "workspace", "current", "-o", "text")
	home := envValue(command.Env, "HOME")
	if err := os.WriteFile(filepath.Join(home, ".bex", "cli.yaml"), []byte("version: 1\napi:\n  key: test-access-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil {
		t.Fatal("workspace current without a workspace must fail")
	}
	if !strings.Contains(stderr.String(), "Use `bex workspace set` to set a workspace") || strings.Contains(stderr.String(), "`render workspace set`") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v
		}
	}
	return ""
}
