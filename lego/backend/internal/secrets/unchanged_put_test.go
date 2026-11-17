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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render documents no effect for a PUT of what the service already holds: the
// env-var and secret-file PUTs answer 200 (201 for a secret file) with the saved
// value and deploy nothing ("Changes will not be deployed automatically",
// api-docs.render.com/reference/update-env-vars-for-service). Here such a PUT
// patched the App on every service: over REST (staged) it stamped
// app.bex.co/saved-config-revision, over GraphQL and MCP it bumped restartedAt,
// stamped a release generation and opened a config_change deploy, rolling the
// workload for a write that stored nothing (w21/110). PatchEnvironment has
// skipped an unchanged environment since w4/m122. These tests run the verbs on a
// control-plane service with a deploy recorder and a clock that moves, as
// delete_absent_test.go does, and count what each call writes.

// putKind is one verb's write of the value its store already holds ("1"), or of
// another ("2"), through each surface.
type putKind struct {
	name       string
	storePath  string
	secretName string
	stored     map[string]string
	projection projectionKind
	// refer and unrefer point the App at the derived Secret, or away from it.
	refer, unrefer func(a *appv1alpha1.App)
	// referenced reports what a staged write leaves, which holds the reference
	// back as a pending one when no release consumed the Secret.
	referenced func(a *appv1alpha1.App) bool
	call       func(ctx context.Context, svc *Service, value string) error
	method     string
	path       string
	body       func(value string) string
	status     int
	// graphql and mcpTool are empty for the verb that has no such surface.
	graphql func(value string) string
	mcpTool string
	mcpArgs func(value string) map[string]any
}

var putKinds = []putKind{
	{
		name: "PUT one variable", storePath: envPath("bp"), secretName: "bp-env", stored: map[string]string{"KEEP": "1"}, projection: envProjection,
		refer:   func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bp-env" },
		unrefer: func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "" },
		referenced: func(a *appv1alpha1.App) bool {
			return a.Spec.EnvFromSecret == "bp-env" || a.Annotations[appv1alpha1.PendingEnvSecretAnnotation] == "bp-env"
		},
		call: func(ctx context.Context, svc *Service, value string) error {
			_, err := svc.SetEnvVar(ctx, "bp", "KEEP", EnvVarWrite{Value: value})
			return err
		},
		method: http.MethodPut, path: "/v1/services/bp/env-vars/KEEP", status: http.StatusOK,
		body: func(value string) string { return `{"value":"` + value + `"}` },
		graphql: func(value string) string {
			return `mutation { setEnvVar(serviceId:"bp", key:"KEEP", value:"` + value + `") }`
		},
		mcpTool: "set_env_var",
		mcpArgs: func(value string) map[string]any {
			return map[string]any{"serviceId": "bp", "key": "KEEP", "value": value}
		},
	},
	{
		name: "PUT the whole variable set", storePath: envPath("bp"), secretName: "bp-env", stored: map[string]string{"KEEP": "1"}, projection: envProjection,
		refer:   func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bp-env" },
		unrefer: func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "" },
		referenced: func(a *appv1alpha1.App) bool {
			return a.Spec.EnvFromSecret == "bp-env" || a.Annotations[appv1alpha1.PendingEnvSecretAnnotation] == "bp-env"
		},
		call: func(ctx context.Context, svc *Service, value string) error {
			_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "KEEP", Value: value}})
			return err
		},
		method: http.MethodPut, path: "/v1/services/bp/env-vars", status: http.StatusOK,
		body: func(value string) string { return `[{"key":"KEEP","value":"` + value + `"}]` },
		graphql: func(value string) string {
			return `mutation { setEnvVars(serviceId:"bp", envVars:[{key:"KEEP", value:"` + value + `"}]) }`
		},
		mcpTool: "update_env_vars",
		mcpArgs: func(value string) map[string]any {
			return map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "KEEP", "value": value}}}
		},
	},
	{
		name: "PUT one secret file", storePath: filesPath("bp"), secretName: "bp-files", stored: map[string]string{"keep.txt": "1"}, projection: filesProjection,
		refer:   func(a *appv1alpha1.App) { a.Spec.FilesFromSecrets = []string{"bp-files"} },
		unrefer: func(a *appv1alpha1.App) { a.Spec.FilesFromSecrets = nil },
		referenced: func(a *appv1alpha1.App) bool {
			return slices.Contains(a.Spec.FilesFromSecrets, "bp-files") || a.Annotations[appv1alpha1.PendingFilesSecretAnnotation] == "bp-files"
		},
		call: func(ctx context.Context, svc *Service, value string) error {
			_, err := svc.SetSecretFile(ctx, "bp", "keep.txt", value)
			return err
		},
		method: http.MethodPut, path: "/v1/services/bp/secret-files/keep.txt", status: http.StatusCreated,
		body: func(value string) string { return `{"content":"` + value + `"}` },
		graphql: func(value string) string {
			return `mutation { setSecretFile(serviceId:"bp", name:"keep.txt", content:"` + value + `") }`
		},
		mcpTool: "set_secret_file",
		mcpArgs: func(value string) map[string]any {
			return map[string]any{"serviceId": "bp", "name": "keep.txt", "content": value}
		},
	},
	{
		name: "PUT the whole secret-file set", storePath: filesPath("bp"), secretName: "bp-files", stored: map[string]string{"keep.txt": "1"}, projection: filesProjection,
		refer:   func(a *appv1alpha1.App) { a.Spec.FilesFromSecrets = []string{"bp-files"} },
		unrefer: func(a *appv1alpha1.App) { a.Spec.FilesFromSecrets = nil },
		referenced: func(a *appv1alpha1.App) bool {
			return slices.Contains(a.Spec.FilesFromSecrets, "bp-files") || a.Annotations[appv1alpha1.PendingFilesSecretAnnotation] == "bp-files"
		},
		call: func(ctx context.Context, svc *Service, value string) error {
			_, err := svc.SetSecretFiles(ctx, "bp", []SecretFileView{{Name: "keep.txt", Content: value}})
			return err
		},
		method: http.MethodPut, path: "/v1/services/bp/secret-files", status: http.StatusOK,
		body: func(value string) string { return `[{"name":"keep.txt","content":"` + value + `"}]` },
	},
}

// rig builds the control-plane service these tests run on: the kind's store map
// as the service holds it, its derived Secret projected and the App referring
// to it (a service that has rolled once), with or without a retained variable.
func (k putKind) rig(t *testing.T, retained bool) *absentRig {
	t.Helper()
	return newAbsentRig(t, absentKind{name: k.name, storePath: k.storePath, secretName: k.secretName, stored: k.stored, refer: k.refer}, retained, true)
}

// rebase makes the service as it now stands the one a call is judged against:
// the App and the counters start over.
func (r *absentRig) rebase(t *testing.T) {
	t.Helper()
	r.before = getApp(t, r.svc.Client, "bp")
	r.patches.patches, r.secrets.writes, r.rec.rows = 0, 0, nil
	r.kv.casCalls = 0
	r.kind.stored = maps.Clone(r.storeMap())
}

// editApp changes the App behind the service's counters, as a state an earlier
// write left, and rebases.
func (r *absentRig) editApp(t *testing.T, edit func(a *appv1alpha1.App)) {
	t.Helper()
	a := getApp(t, r.patches.Client, "bp")
	edit(a)
	if err := r.patches.Client.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	r.rebase(t)
}

// editSecret changes the derived Secret behind the counters, as a state an
// earlier write left, and rebases.
func (r *absentRig) editSecret(t *testing.T, edit func(data map[string][]byte)) {
	t.Helper()
	r.editSecretObject(t, func(sec *corev1.Secret) { edit(sec.Data) })
}

// editSecretObject is editSecret for a change to more than the data.
func (r *absentRig) editSecretObject(t *testing.T, edit func(sec *corev1.Secret)) {
	t.Helper()
	sec := &corev1.Secret{}
	if err := r.patches.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: r.kind.secretName}, sec); err != nil {
		t.Fatal(err)
	}
	edit(sec)
	if err := r.patches.Client.Update(context.Background(), sec); err != nil {
		t.Fatal(err)
	}
	r.rebase(t)
}

const identicalPut = "an identical PUT"

// A PUT of what the service already holds answers as it did (200, 201 for a
// secret file, the saved value) and changes nothing, through every surface, on a
// plain service and on one with a retained variable the write does not name.
func TestAPutThatStoresNothingNewChangesNothing(t *testing.T) {
	ctx := context.Background()
	for _, k := range putKinds {
		for _, retained := range []bool{false, true} {
			prefix := k.name + "/plain/"
			if retained {
				prefix = k.name + "/retained variable/"
				if k.name == "PUT the whole variable set" {
					// A replace decides every variable, so it moves the retained
					// literal: that is a change (TestAPutThatMovesARetainedLiteralStillChanges).
					continue
				}
			}
			t.Run(prefix+"REST", func(t *testing.T) {
				r := k.rig(t, retained)
				w := serveREST(r.svc, k.method, k.path, k.body("1"))
				if w.Code != k.status {
					t.Fatalf("%s %s = %d, want %d: %.200s", k.method, k.path, w.Code, k.status, w.Body.String())
				}
				var got any
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || !strings.Contains(w.Body.String(), `"1"`) {
					t.Errorf("the answer is %q, want the saved value", w.Body.String())
				}
				r.requireNothingChangedBy(t, identicalPut)
			})
			for _, staged := range []bool{true, false} {
				name, callCtx := prefix+"Service/deploying", ctx
				if staged {
					name, callCtx = prefix+"Service/staged", WithStagedWrite(ctx)
				}
				t.Run(name, func(t *testing.T) {
					r := k.rig(t, retained)
					if err := k.call(callCtx, r.svc, "1"); err != nil {
						t.Fatalf("the write = %v, want success", err)
					}
					r.requireNothingChangedBy(t, identicalPut)
				})
			}
			if k.graphql == nil {
				continue
			}
			t.Run(prefix+"GraphQL", func(t *testing.T) {
				r := k.rig(t, retained)
				res := graphql.Do(graphql.Params{Schema: absentGraphQLSchema(t, r.svc), Context: ctx, RequestString: k.graphql("1")})
				if res.HasErrors() {
					t.Fatalf("GraphQL write = %+v, want success", res.Errors)
				}
				r.requireNothingChangedBy(t, identicalPut)
			})
			t.Run(prefix+"MCP", func(t *testing.T) {
				r := k.rig(t, retained)
				res, err := mcpSession(t, r.svc).CallTool(ctx, &mcp.CallToolParams{Name: k.mcpTool, Arguments: k.mcpArgs("1")})
				if err != nil || res == nil || res.IsError {
					t.Fatalf("MCP write = %+v, %v, want success", res, err)
				}
				r.requireNothingChangedBy(t, identicalPut)
			})
		}
	}
}

// A PUT that changes the map is as it was: one store write, one App patch and
// the new value in the derived Secret. A staged write (REST) opens no deploy and
// stamps the saved change; a deploying write (GraphQL, MCP) opens exactly one.
func TestAPutThatChangesTheMapIsUnchanged(t *testing.T) {
	ctx := context.Background()
	for _, k := range putKinds {
		check := func(t *testing.T, r *absentRig, staged bool) {
			t.Helper()
			if r.kv.casCalls != 1 || r.patches.patches != 1 {
				t.Errorf("the write made %d store writes and %d App patches, want one each", r.kv.casCalls, r.patches.patches)
			}
			for key := range k.stored {
				if got := secretData(t, r.svc.Client, k.secretName)[key]; got != "2" {
					t.Errorf("the derived Secret holds %s = %q, want the new value", key, got)
				}
			}
			app := getApp(t, r.svc.Client, "bp")
			if staged {
				if len(r.rec.rows) != 0 || app.Spec.RestartedAt != "" || app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] == "" {
					t.Errorf("the staged write opened %d deploys, restartedAt %q, saved-config-revision %q, want no deploy and a saved change",
						len(r.rec.rows), app.Spec.RestartedAt, app.Annotations[appv1alpha1.AnnotationSavedConfigRevision])
				}
				return
			}
			if len(r.rec.rows) != 1 || r.rec.rows[0].Trigger != store.TriggerConfigChange || app.Spec.RestartedAt == "" {
				t.Errorf("the write opened %+v with restartedAt %q, want one config_change deploy and a roll", r.rec.rows, app.Spec.RestartedAt)
			}
		}
		t.Run(k.name+"/REST", func(t *testing.T) {
			r := k.rig(t, false)
			if w := serveREST(r.svc, k.method, k.path, k.body("2")); w.Code != k.status {
				t.Fatalf("%s %s = %d, want %d: %.200s", k.method, k.path, w.Code, k.status, w.Body.String())
			}
			check(t, r, true)
		})
		t.Run(k.name+"/Service", func(t *testing.T) {
			r := k.rig(t, false)
			if err := k.call(ctx, r.svc, "2"); err != nil {
				t.Fatal(err)
			}
			check(t, r, false)
		})
		if k.graphql == nil {
			continue
		}
		t.Run(k.name+"/GraphQL", func(t *testing.T) {
			r := k.rig(t, false)
			if res := graphql.Do(graphql.Params{Schema: absentGraphQLSchema(t, r.svc), Context: ctx, RequestString: k.graphql("2")}); res.HasErrors() {
				t.Fatal(res.Errors)
			}
			check(t, r, false)
		})
		t.Run(k.name+"/MCP", func(t *testing.T) {
			r := k.rig(t, false)
			if res, err := mcpSession(t, r.svc).CallTool(ctx, &mcp.CallToolParams{Name: k.mcpTool, Arguments: k.mcpArgs("2")}); err != nil || res.IsError {
				t.Fatalf("MCP write = %+v, %v", res, err)
			}
			check(t, r, false)
		})
	}
}

// A retained literal's move is a change even when the store's bytes are equal
// (w21/108): the App loses an env entry. A write that names the literal, or
// replaces the set, still rides its own store write and App patch, a staged one
// opens no deploy and a deploying one exactly one (w21/106), and the same write
// again is then identical and changes nothing.
func TestAPutThatMovesARetainedLiteralStillChanges(t *testing.T) {
	ctx := context.Background()
	const literal = "hello from bex"
	moves := []struct {
		name    string
		call    func(ctx context.Context, svc *Service) error
		rest    func(svc *Service) int
		graphql string
		mcpTool string
		mcpArgs map[string]any
	}{
		{
			name: "PUT one",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.SetEnvVar(ctx, "bp", "MESSAGE", EnvVarWrite{Value: literal})
				return err
			},
			rest: func(svc *Service) int {
				return serveREST(svc, http.MethodPut, "/v1/services/bp/env-vars/MESSAGE", `{"value":"`+literal+`"}`).Code
			},
			graphql: `mutation { setEnvVar(serviceId:"bp", key:"MESSAGE", value:"` + literal + `") }`,
			mcpTool: "set_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "MESSAGE", "value": literal},
		},
		{
			name: "PUT all",
			call: func(ctx context.Context, svc *Service) error {
				_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "KEEP", Value: "1"}, {Key: "MESSAGE", Value: literal}})
				return err
			},
			rest: func(svc *Service) int {
				return serveREST(svc, http.MethodPut, "/v1/services/bp/env-vars", `[{"key":"KEEP","value":"1"},{"key":"MESSAGE","value":"`+literal+`"}]`).Code
			},
			graphql: `mutation { setEnvVars(serviceId:"bp", envVars:[{key:"KEEP", value:"1"},{key:"MESSAGE", value:"` + literal + `"}]) }`,
			mcpTool: "update_env_vars", mcpArgs: map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "KEEP", "value": "1"}, {"key": "MESSAGE", "value": literal}}},
		},
	}
	// The store already holds the literal's value, so the write's bytes equal it.
	kind := absentKind{
		name: "env var", storePath: envPath("bp"), secretName: "bp-env",
		stored: map[string]string{"KEEP": "1", "MESSAGE": literal},
		refer:  func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bp-env" },
	}
	for _, mv := range moves {
		check := func(t *testing.T, r *absentRig, staged bool, again func()) {
			t.Helper()
			if r.kv.casCalls != 1 || r.patches.patches != 1 {
				t.Errorf("the move made %d store writes and %d App patches, want one each", r.kv.casCalls, r.patches.patches)
			}
			if _, still := specEnvNames(getApp(t, r.svc.Client, "bp"))["MESSAGE"]; still {
				t.Error("the write left the literal on the service")
			}
			wantRows := 1
			if staged {
				wantRows = 0
			}
			if len(r.rec.rows) != wantRows {
				t.Errorf("the move opened %d deploys (%+v), want %d", len(r.rec.rows), r.rec.rows, wantRows)
			}
			r.rebase(t)
			again()
			r.requireNothingChangedBy(t, "the same write once the literal is moved")
		}
		t.Run(mv.name+"/REST", func(t *testing.T) {
			r := newAbsentRig(t, kind, true, true)
			if code := mv.rest(r.svc); code != http.StatusOK {
				t.Fatalf("REST write = %d, want 200", code)
			}
			check(t, r, true, func() {
				if code := mv.rest(r.svc); code != http.StatusOK {
					t.Fatalf("REST write again = %d, want 200", code)
				}
			})
		})
		t.Run(mv.name+"/Service/deploying", func(t *testing.T) {
			r := newAbsentRig(t, kind, true, true)
			if err := mv.call(ctx, r.svc); err != nil {
				t.Fatal(err)
			}
			check(t, r, false, func() {
				if err := mv.call(ctx, r.svc); err != nil {
					t.Fatal(err)
				}
			})
		})
		t.Run(mv.name+"/GraphQL", func(t *testing.T) {
			r := newAbsentRig(t, kind, true, true)
			run := func() {
				if res := graphql.Do(graphql.Params{Schema: absentGraphQLSchema(t, r.svc), Context: ctx, RequestString: mv.graphql}); res.HasErrors() {
					t.Fatal(res.Errors)
				}
			}
			run()
			check(t, r, false, run)
		})
		t.Run(mv.name+"/MCP", func(t *testing.T) {
			r := newAbsentRig(t, kind, true, true)
			run := func() {
				if res, err := mcpSession(t, r.svc).CallTool(ctx, &mcp.CallToolParams{Name: mv.mcpTool, Arguments: mv.mcpArgs}); err != nil || res.IsError {
					t.Fatalf("MCP write = %+v, %v", res, err)
				}
			}
			run()
			check(t, r, false, run)
		})
	}
}

// What an identical PUT still repairs, so a retry of a write that committed the
// store and not its projection converges (nothing writes the store: it already
// holds the map). The derived Secret that is not the store's map, or does not
// exist, or the App that does not refer to the Secret of a map that holds
// variables, is projected once: a staged retry saves the change for the next
// deploy, a deploying one rolls once. The same PUT again is then identical.
func TestAnIdenticalPutRepairsAProjectionAnEarlierAttemptLeftBehind(t *testing.T) {
	ctx := context.Background()
	for _, k := range putKinds {
		{
			for _, state := range []struct {
				name  string
				apply func(t *testing.T, r *absentRig)
			}{
				{"no Secret", func(t *testing.T, r *absentRig) {
					sec := &corev1.Secret{}
					sec.Name, sec.Namespace = k.secretName, "default"
					if err := r.patches.Client.Delete(ctx, sec); err != nil {
						t.Fatal(err)
					}
					r.rebase(t)
				}},
				{"a Secret with a variable the store lost", func(t *testing.T, r *absentRig) {
					r.editSecret(t, func(data map[string][]byte) { data["REVOKED"] = []byte("stale") })
				}},
				{"a Secret holding another value", func(t *testing.T, r *absentRig) {
					r.editSecret(t, func(data map[string][]byte) {
						for key := range data {
							data[key] = []byte("old")
						}
					})
				}},
				{"a Secret missing a variable", func(t *testing.T, r *absentRig) {
					r.editSecret(t, func(data map[string][]byte) {
						for key := range data {
							delete(data, key)
						}
					})
				}},
				{"a Secret an earlier revision projected", func(t *testing.T, r *absentRig) {
					// The state a write that committed the store and died before its
					// projection leaves: the store is ahead of the Secret.
					r.kv.versions[k.storePath] = 3
					r.editSecretObject(t, func(sec *corev1.Secret) {
						sec.Data["REVOKED"] = []byte("stale")
						setProjectedRevision(sec, k.projection, committedAt(2))
					})
				}},
				{"an App that does not refer to the Secret", func(t *testing.T, r *absentRig) { r.editApp(t, k.unrefer) }},
			} {
				for _, staged := range []bool{true, false} {
					name := k.name + "/" + state.name + "/deploying"
					callCtx := ctx
					if staged {
						name, callCtx = k.name+"/"+state.name+"/staged", WithStagedWrite(ctx)
					}
					t.Run(name, func(t *testing.T) {
						r := k.rig(t, false)
						state.apply(t, r)
						if err := k.call(callCtx, r.svc, "1"); err != nil {
							t.Fatalf("the retry = %v, want success", err)
						}
						if r.kv.casCalls != 0 || !maps.Equal(r.storeMap(), k.stored) {
							t.Errorf("the retry wrote the store (%d writes, now %v)", r.kv.casCalls, r.storeMap())
						}
						if got := secretData(t, r.svc.Client, k.secretName); !maps.Equal(got, k.stored) {
							t.Errorf("the derived Secret holds %v, want the store's map %v", got, k.stored)
						}
						app := getApp(t, r.svc.Client, "bp")
						if !k.referenced(app) {
							t.Error("the App does not refer to the Secret after the retry")
						}
						if r.patches.patches != 1 {
							t.Errorf("the retry patched the App %d times, want once", r.patches.patches)
						}
						if staged {
							if len(r.rec.rows) != 0 || app.Spec.RestartedAt != "" || app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] == "" {
								t.Errorf("the staged retry opened %d deploys, restartedAt %q, saved-config-revision %q, want no deploy and a saved change",
									len(r.rec.rows), app.Spec.RestartedAt, app.Annotations[appv1alpha1.AnnotationSavedConfigRevision])
							}
						} else if len(r.rec.rows) != 1 || app.Spec.RestartedAt == "" {
							t.Errorf("the retry opened %+v with restartedAt %q, want one deploy and a roll", r.rec.rows, app.Spec.RestartedAt)
						}
						// The repair is done: the same PUT again changes nothing.
						r.rebase(t)
						if err := k.call(callCtx, r.svc, "1"); err != nil {
							t.Fatal(err)
						}
						r.requireNothingChangedBy(t, "the same PUT after its repair")
					})
				}
			}
		}
	}
}

// A Secret at the store's own revision that holds another map is a projection
// that lost to a concurrent write, not a retry's to overwrite: one revision holds
// one map. The PUT answers the conflict, as it did, and changes nothing.
func TestAnIdenticalPutOverASecretAtTheStoresRevisionWithOtherDataConflicts(t *testing.T) {
	for _, k := range putKinds {
		t.Run(k.name, func(t *testing.T) {
			r := k.rig(t, false)
			r.editSecretObject(t, func(sec *corev1.Secret) {
				sec.Data["REVOKED"] = []byte("other")
				setProjectedRevision(sec, k.projection, committedAt(1))
			})
			err := k.call(context.Background(), r.svc, "1")
			if !errors.Is(err, core.ErrConflict) {
				t.Fatalf("the PUT = %v, want a conflict", err)
			}
			if r.patches.patches != 0 || len(r.rec.rows) != 0 || r.kv.casCalls != 0 {
				t.Errorf("the refused PUT made %d App patches, %d deploys and %d store writes, want none", r.patches.patches, len(r.rec.rows), r.kv.casCalls)
			}
		})
	}
}

// The same holds on a store without versions (a test double, or a store that
// predates them): an identical PUT changes nothing, one with a stale Secret
// repairs it.
func TestAnIdenticalPutOnAnUnversionedStore(t *testing.T) {
	ctx := context.Background()
	for _, k := range putKinds {
		for _, stale := range []bool{false, true} {
			name := k.name + "/nothing to repair"
			if stale {
				name = k.name + "/stale Secret"
			}
			t.Run(name, func(t *testing.T) {
				r := k.rig(t, false)
				kv := newFakeSecretStore()
				kv.m[k.storePath] = maps.Clone(k.stored)
				r.svc.Store = kv
				r.storeMap, r.storeWrites = func() map[string]string { return kv.m[k.storePath] }, func() int { return kv.puts }
				if stale {
					r.editSecret(t, func(data map[string][]byte) { data["REVOKED"] = []byte("stale") })
				}
				if err := k.call(ctx, r.svc, "1"); err != nil {
					t.Fatalf("the PUT = %v, want success", err)
				}
				if !stale {
					r.requireNothingChangedBy(t, identicalPut)
					return
				}
				if r.storeWrites() != 0 || !maps.Equal(r.storeMap(), k.stored) {
					t.Errorf("the repair wrote the store (%d puts, now %v)", r.storeWrites(), r.storeMap())
				}
				if got := secretData(t, r.svc.Client, k.secretName); r.patches.patches != 1 || !maps.Equal(got, k.stored) {
					t.Errorf("the repair patched the App %d times and left the Secret holding %v, want once and %v", r.patches.patches, got, k.stored)
				}
			})
		}
	}
}

// An empty set stored over an empty set changes nothing: a service that never
// had variables or files gains no empty Secret and no reference, and one whose
// set was emptied keeps the empty Secret it left (an emptied env set still refers
// to it, an emptied file set does not mount it) and is not patched again.
func TestAnEmptyPutOverAnEmptySetChangesNothing(t *testing.T) {
	ctx := context.Background()
	for _, k := range []putKind{putKinds[1], putKinds[3]} {
		for _, emptied := range []bool{false, true} {
			name := k.name + "/never set"
			if emptied {
				name = k.name + "/emptied"
			}
			t.Run(name, func(t *testing.T) {
				empty := absentKind{name: k.name, storePath: k.storePath, secretName: k.secretName, stored: map[string]string{}, refer: k.refer}
				r := newAbsentRig(t, empty, false, emptied)
				if emptied && k.storePath == filesPath("bp") {
					r.editApp(t, k.unrefer)
				}
				var err error
				if k.storePath == envPath("bp") {
					_, err = r.svc.SetEnvVars(ctx, "bp", nil)
				} else {
					_, err = r.svc.SetSecretFiles(ctx, "bp", nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				r.requireNothingChangedBy(t, "an empty PUT over an empty set")
			})
		}
	}
}

// Invalid-name debris a create stored on spec.Env before names were validated
// is cleared by a whole-set replace (w8/027), an edit of the App that is a
// change even when the store's map is as it was; the replace again is identical.
func TestAWholeSetPutStillClearsDebrisFromSpecEnv(t *testing.T) {
	r := putKinds[1].rig(t, false)
	r.editApp(t, func(a *appv1alpha1.App) {
		a.Spec.Env = []appv1alpha1.EnvVar{{Name: "BAD KEY", Value: "y"}}
	})
	if err := putKinds[1].call(context.Background(), r.svc, "1"); err != nil {
		t.Fatal(err)
	}
	if r.patches.patches != 1 || len(specEnvNames(getApp(t, r.svc.Client, "bp"))) != 0 {
		t.Fatalf("the replace made %d App patches and left spec.Env %v, want one and the debris gone", r.patches.patches, specEnvNames(getApp(t, r.svc.Client, "bp")))
	}
	r.rebase(t)
	if err := putKinds[1].call(context.Background(), r.svc, "1"); err != nil {
		t.Fatal(err)
	}
	r.requireNothingChangedBy(t, "the same replace once the debris is gone")
}

// A Secret that cannot be read cannot be told from one an earlier attempt left
// behind, so the PUT answers the failure and changes nothing.
func TestAnIdenticalPutFailsWhenTheSecretCannotBeRead(t *testing.T) {
	for _, k := range putKinds {
		t.Run(k.name, func(t *testing.T) {
			r := k.rig(t, false)
			r.svc.Client = &secretReadFailure{Client: r.svc.Client}
			err := k.call(context.Background(), r.svc, "1")
			if err == nil || errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("the PUT = %v, want the Secret read failure", err)
			}
			r.svc.Client = r.secrets
			r.requireNothingChangedBy(t, "a PUT whose Secret cannot be read")
		})
	}
}

// A PUT that stores nothing records no event, as PatchEnvironment records none
// for an unchanged environment (w4/m122): the audit rows say what a write did,
// and this one did nothing. One that changes the map records its rows.
func TestAPutThatStoresNothingNewRecordsNoEvent(t *testing.T) {
	for _, k := range []putKind{putKinds[0], putKinds[2]} {
		t.Run(k.name, func(t *testing.T) {
			sink := &recordingAuditSink{}
			r := k.rig(t, false)
			r.svc.Authz, r.svc.Audit = &fakeChecker{allow: true}, sink
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: "oauth2"})
			if err := k.call(ctx, r.svc, "1"); err != nil {
				t.Fatal(err)
			}
			if len(sink.events) != 0 {
				t.Errorf("the identical PUT recorded %d events (%+v), want none", len(sink.events), sink.events)
			}
			if err := k.call(ctx, r.svc, "2"); err != nil {
				t.Fatal(err)
			}
			if len(sink.events) == 0 {
				t.Error("the PUT that changed the map recorded no event")
			}
		})
	}
}
