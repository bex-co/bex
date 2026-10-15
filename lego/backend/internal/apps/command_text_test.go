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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The authored commands of w8/m68; only the quoted control survives trimming.
var authoredCommands = map[string]string{
	"escaped space":  `printf '%s' tail\ `,
	"escaped tab":    "printf '%s' tail\\\t",
	"quoted control": `printf '%s' 'tail '`,
}

// commandField is one command a service stores, as the pinned CLI sends it.
type commandField struct {
	name    string
	wireKey string // pre-deploy sits at serviceDetails, the rest in envSpecificDetails
	cron    bool
	stored  func(appv1alpha1.AppSpec) string
}

var commandFields = []commandField{
	{"buildCommand", "buildCommand", false, func(s appv1alpha1.AppSpec) string { return s.BuildCommand }},
	{"startCommand", "startCommand", false, func(s appv1alpha1.AppSpec) string { return s.StartCommand }},
	{"preDeployCommand", "preDeployCommand", false, func(s appv1alpha1.AppSpec) string { return s.PreDeployCommand }},
	{"cron command", "startCommand", true, func(s appv1alpha1.AppSpec) string { return s.Command }},
}

// details places command where the pinned CLI sends this field.
func (f commandField) details(command string) map[string]any {
	if f.wireKey == "preDeployCommand" {
		return map[string]any{f.wireKey: command}
	}
	return map[string]any{"envSpecificDetails": map[string]any{f.wireKey: command}}
}

// restFixture creates a native web service or cron through REST, with
// command (when non-nil) in the create body.
func (f commandField) restFixture(t *testing.T, command *string) (*Service, *http.ServeMux, string) {
	t.Helper()
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	kind, name := "web_service", "web"
	env := map[string]any{"buildCommand": "npm ci", "startCommand": "node server.js"}
	details := map[string]any{"runtime": "node", "plan": "free", "envSpecificDetails": env}
	if f.cron {
		kind, name = "cron_job", "nightly"
		details["schedule"] = "0 0 1 1 *"
	}
	if command != nil {
		if f.wireKey == "preDeployCommand" {
			details[f.wireKey] = *command
		} else {
			env[f.wireKey] = *command
		}
	}
	rec := serveJSON(t, mux, http.MethodPost, "/v1/services",
		map[string]any{"name": name, "type": kind, "repo": "https://github.com/bex-co/bex", "serviceDetails": details})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	return svc, mux, name
}

func serveJSON(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(raw))))
	return rec
}

func assertStoredCommand(t *testing.T, svc *Service, name string, f commandField, want string) {
	t.Helper()
	if got := f.stored(getApp(t, svc.Client, name).Spec); got != want {
		t.Errorf("%s %s = %q, want %q", name, f.name, got, want)
	}
}

// Every REST command write keeps nonblank text byte for byte (w8/m68).
func TestRESTCommandWritesKeepAuthoredText(t *testing.T) {
	for _, f := range commandFields {
		for variant, command := range authoredCommands {
			t.Run("create/"+f.name+"/"+variant, func(t *testing.T) {
				svc, _, name := f.restFixture(t, &command)
				assertStoredCommand(t, svc, name, f, command)
			})
			t.Run("update/"+f.name+"/"+variant, func(t *testing.T) {
				svc, mux, name := f.restFixture(t, nil)
				if rec := serveJSON(t, mux, http.MethodPatch, "/v1/services/"+name, map[string]any{"serviceDetails": f.details(command)}); rec.Code != http.StatusOK {
					t.Fatalf("PATCH: %d %s", rec.Code, rec.Body)
				}
				assertStoredCommand(t, svc, name, f, command)
			})
		}
		t.Run("whitespace-only update clears/"+f.name, func(t *testing.T) {
			before := "echo before"
			svc, mux, name := f.restFixture(t, &before)
			if rec := serveJSON(t, mux, http.MethodPatch, "/v1/services/"+name, map[string]any{"serviceDetails": f.details(" \t ")}); rec.Code != http.StatusOK {
				t.Fatalf("PATCH: %d %s", rec.Code, rec.Body)
			}
			assertStoredCommand(t, svc, name, f, "")
		})
	}
}

// GraphQL and MCP reach the same setters, so they keep the same bytes.
func TestGraphQLAndMCPCommandWritesKeepAuthoredText(t *testing.T) {
	ctx := context.Background()
	command := authoredCommands["escaped space"]
	webAndCron := func(t *testing.T) *Service {
		t.Helper()
		svc, _ := newService(nil)
		for _, req := range []CreateRequest{
			{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/bex-co/bex", Runtime: "node",
				BuildCommand: "npm ci", StartCommand: "node server.js"},
			{Name: "nightly", Type: appv1alpha1.TypeCronJob, Repo: "https://github.com/bex-co/bex", Runtime: "node",
				BuildCommand: "npm ci", StartCommand: "node job.js", Schedule: "0 0 1 1 *", Command: "node job.js"},
		} {
			if _, err := svc.Create(ctx, req); err != nil {
				t.Fatalf("Create %s: %v", req.Name, err)
			}
		}
		return svc
	}
	build, start, preDeploy, cron := commandFields[0], commandFields[1], commandFields[2], commandFields[3]

	for _, tc := range []struct {
		mutation, name string
		field          commandField
	}{
		{`setBuildCommand(id: "web", command: $c) { id }`, "web", build},
		{`setStartCommand(id: "web", command: $c) { id }`, "web", start},
		{`setPreDeployCommand(id: "web", command: $c) { id }`, "web", preDeploy},
		{`updateCronJob(id: "nightly", schedule: "0 0 1 1 *", command: $c) { id }`, "nightly", cron},
		{`setStartCommand(id: "nightly", command: $c) { id }`, "nightly", cron},
	} {
		t.Run("GraphQL "+tc.mutation, func(t *testing.T) {
			svc := webAndCron(t)
			res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: ctx,
				RequestString: `mutation($c: String!) { ` + tc.mutation + ` }`, VariableValues: map[string]any{"c": command}})
			if len(res.Errors) > 0 {
				t.Fatalf("%s: %v", tc.mutation, res.Errors)
			}
			assertStoredCommand(t, svc, tc.name, tc.field, command)
		})
	}

	for _, tc := range []struct {
		arg, name string
		field     commandField
	}{
		{"buildCommand", "web", build},
		{"startCommand", "web", start},
		{"preDeployCommand", "web", preDeploy},
		{"command", "nightly", cron},
	} {
		t.Run("MCP update_service "+tc.arg, func(t *testing.T) {
			svc := webAndCron(t)
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			call("update_service", map[string]any{"serviceId": tc.name, tc.arg: command})
			assertStoredCommand(t, svc, tc.name, tc.field, command)
		})
	}
}

// A whitespace-only command on create is no command, so a reader never sees
// a blank override it would run (w8/m68).
func TestWhitespaceCreateCommandsAreEmpty(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	for _, req := range []CreateRequest{
		{Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/site", PublishPath: "dist", BuildCommand: " \t "},
		{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/web", Runtime: "docker", StartCommand: "  ", PreDeployCommand: " "},
	} {
		if _, err := svc.Create(ctx, req); err != nil {
			t.Fatalf("Create %s: %v", req.Name, err)
		}
		if spec := getApp(t, cl, req.Name).Spec; spec.BuildCommand != "" || spec.StartCommand != "" || spec.PreDeployCommand != "" {
			t.Errorf("%s stored build %q start %q pre-deploy %q, want all empty", req.Name, spec.BuildCommand, spec.StartCommand, spec.PreDeployCommand)
		}
	}
}

// A Blueprint's commands reach the same create, so the one-time hook and the
// regular pre-deploy command keep their authored bytes and a resync is a no-op.
func TestBlueprintCommandsKeepAuthoredText(t *testing.T) {
	command := authoredCommands["escaped tab"]
	manifest := fmt.Sprintf("services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: nginx:1.26}\n    initialDeployHook: %q\n    preDeployCommand: %q\n", command, command)
	svc, cl := newService(nil)
	ctx := context.Background()
	for range 2 {
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		if spec := getApp(t, cl, "web").Spec; spec.InitialDeployHook != command || spec.PreDeployCommand != command {
			t.Errorf("hook %q / preDeployCommand %q, want %q for both", spec.InitialDeployHook, spec.PreDeployCommand, command)
		}
	}
}
