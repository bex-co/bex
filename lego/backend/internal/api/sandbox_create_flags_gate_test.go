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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// TestSandboxCLIv235FlagsReachNamedAnswersThroughTheComposedServer drives the
// exact bodies and paths the pinned Render CLI v2.35 sends for its sandbox
// flags through the real composed handler — auth gate, Render request
// validator, strict decoding — so a refusal that a bare-mux test proves can
// not be pre-empted by a generic validator or `unknown field` 400 (w2/m263).
// The group routes reach the Service through the same path (w22/m311); with
// no control-plane store they answer a named 503.
func TestSandboxCLIv235FlagsReachNamedAnswersThroughTheComposedServer(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a refused sandbox request reached OpenSandbox: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)
	srv := NewServer(&core.Base{
		Client: fakeClient(sampleApp("web")), Namespace: "default",
		Workspace: fakeWorkspace{"client-1": "tea-a"}, Authz: &fakeChecker{allow: true},
	}, Deps{
		SandboxClient:          sandbox.NewClient(upstream.URL),
		SandboxTemplates:       map[string]sandbox.Template{"base": {Image: "alpine:3", CPU: "500m", Memory: "512Mi"}},
		SandboxDefaultTemplate: "base",
		SandboxDefaultPlan:     sandbox.PlanStarter,
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	handler, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}

	group := "sbg-" + strings.Repeat("a", 20)
	sbx := "sbx-" + strings.Repeat("b", 20)
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"--idle-timeout 100", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","idleTimeoutSeconds":100}`, 400, "SANDBOX_IDLE_TIMEOUT_INVALID"},
		// Groups are served since w22/m311; this server has no control-plane
		// store, so a request that names one is a named 503, never a create
		// outside any group and never an unfiltered list.
		{"--group", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","sandboxGroupId":"` + group + `"}`, 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		// A malformed id is refused by the Render gate before the Service, and
		// still carries Render's own code, as GraphQL and MCP do.
		{"--group malformed", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","sandboxGroupId":"nope"}`, 400, "invalid_sandbox_group_id"},
		{"list --group malformed", http.MethodGet, "/v1/sandboxes?ownerId=tea-a&sandboxGroupId=nope", "", 400, "invalid_sandbox_group_id"},
		{"sandbox-groups retrieve malformed", http.MethodGet, "/v1/sandbox-groups/nope", "", 400, "invalid_sandbox_group_id"},
		{"sandbox-groups update malformed", http.MethodPatch, "/v1/sandbox-groups/nope", `{"concurrencyLimit":5}`, 400, "invalid_sandbox_group_id"},
		// The pinned schema requires a concurrencyLimit of at least 1, so the gate
		// refuses these before the Service does; the code is the one GraphQL and
		// MCP answer with (the cross-surface table test drives REST on a bare mux,
		// where only the Service's own check is reachable).
		{"sandbox-groups update limit zero", http.MethodPatch, "/v1/sandbox-groups/" + group, `{"concurrencyLimit":0}`, 400, "SANDBOX_GROUP_LIMIT_INVALID"},
		{"sandbox-groups update limit negative", http.MethodPatch, "/v1/sandbox-groups/" + group, `{"concurrencyLimit":-3}`, 400, "SANDBOX_GROUP_LIMIT_INVALID"},
		{"sandbox-groups update limit absent", http.MethodPatch, "/v1/sandbox-groups/" + group, `{}`, 400, "SANDBOX_GROUP_LIMIT_INVALID"},
		{"--image by id", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","buildId":"sbb-` + strings.Repeat("c", 20) + `"}`, 400, "SANDBOX_IMAGES_UNSUPPORTED"},
		{"--image by name", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","buildName":"python"}`, 400, "SANDBOX_IMAGES_UNSUPPORTED"},
		{"unknown plan", http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","plan":"16c-32g"}`, 400, "SANDBOX_PLAN_INVALID"},
		{"list --group", http.MethodGet, "/v1/sandboxes?ownerId=tea-a&sandboxGroupId=" + group, "", 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		{"sandbox-groups list", http.MethodGet, "/v1/sandbox-groups?ownerId=tea-a", "", 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		{"sandbox-groups create", http.MethodPost, "/v1/sandbox-groups", `{"ownerId":"tea-a","name":"agents"}`, 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		{"sandbox-groups retrieve", http.MethodGet, "/v1/sandbox-groups/" + group, "", 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		{"sandbox-groups update", http.MethodPatch, "/v1/sandbox-groups/" + group, `{"concurrencyLimit":5}`, 503, "SANDBOX_GROUPS_UNAVAILABLE"},
		{"sandbox-groups images import", http.MethodPost, "/v1/sandbox-groups/" + group + "/builds", `{"name":"python","sourceImage":"python:3.13"}`, 404, "SANDBOX_IMAGES_UNSUPPORTED"},
		{"sandbox-groups images get", http.MethodGet, "/v1/sandbox-groups/" + group + "/builds/sbb-" + strings.Repeat("c", 20), "", 404, "SANDBOX_IMAGES_UNSUPPORTED"},
		{"sandboxes snapshots create", http.MethodPost, "/v1/sandboxes/" + sbx + "/snapshots?ownerId=tea-a", `{"name":"gold"}`, 404, "SANDBOX_SNAPSHOTS_UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+testToken)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			var got struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &got)
			if rec.Code != tc.status || got.Code != tc.code {
				t.Fatalf("= %d %s, want %d %s", rec.Code, rec.Body.String(), tc.status, tc.code)
			}
			if strings.Contains(got.Message, "unknown field") {
				t.Fatalf("message leaks the wire key: %q", got.Message)
			}
		})
	}
}
