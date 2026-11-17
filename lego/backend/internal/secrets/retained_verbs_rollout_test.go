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
	"errors"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's env-var REST writes are saved and not deployed ("Changes will not be
// deployed automatically. Instead you must call the deploy API"). A variable a
// Blueprint dropped stays on spec.Env, and the single-variable verbs and the
// bulk replace move it into the store inside their own write, in the same locked
// App patch that removes the literal from the pod template (w21/108). That
// removal rolls a release: a staged (REST) write asks for none, so it is held for
// the next deploy and opens none, and a deploying (GraphQL, MCP) write rolls
// exactly once (w21/106). These tests run the verbs on a control-plane service
// with a deploy recorder and a clock that moves, as a real one does, and count
// what each verb writes: one store write and one App patch.

// retainedVerb is one write of the retained MESSAGE, through each surface.
type retainedVerb struct {
	name string
	// REST
	method, path, body string
	// the Service verb, for a staged context
	call func(ctx context.Context, svc *Service) error
	// GraphQL's mutation and MCP's tool and arguments
	graphql string
	mcpTool string
	mcpArgs map[string]any
	wantEnv map[string]string // the store after the write
}

var retainedVerbs = []retainedVerb{
	{
		name: "PUT one", method: http.MethodPut, path: "/v1/services/bp/env-vars/MESSAGE", body: `{"value":"changed"}`,
		call: func(ctx context.Context, svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "bp", "MESSAGE", EnvVarWrite{Value: "changed"})
			return err
		},
		graphql: `mutation { setEnvVar(serviceId:"bp", key:"MESSAGE", value:"changed") }`,
		mcpTool: "set_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "MESSAGE", "value": "changed"},
		wantEnv: map[string]string{"MESSAGE": "changed"},
	},
	{
		name: "DELETE one", method: http.MethodDelete, path: "/v1/services/bp/env-vars/MESSAGE",
		call: func(ctx context.Context, svc *Service) error {
			return svc.DeleteEnvVar(ctx, "bp", "MESSAGE")
		},
		graphql: `mutation { deleteEnvVar(serviceId:"bp", key:"MESSAGE") }`,
		mcpTool: "delete_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "MESSAGE"},
		wantEnv: map[string]string{},
	},
	{
		name: "PUT all", method: http.MethodPut, path: "/v1/services/bp/env-vars", body: `[{"key":"MESSAGE","value":"kept"},{"key":"OTHER","value":"2"}]`,
		call: func(ctx context.Context, svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "MESSAGE", Value: "kept"}, {Key: "OTHER", Value: "2"}})
			return err
		},
		graphql: `mutation { setEnvVars(serviceId:"bp", envVars:[{key:"MESSAGE", value:"kept"},{key:"OTHER", value:"2"}]) }`,
		mcpTool: "update_env_vars", mcpArgs: map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "MESSAGE", "value": "kept"}, {"key": "OTHER", "value": "2"}}},
		wantEnv: map[string]string{"MESSAGE": "kept", "OTHER": "2"},
	},
}

// retainedTracked is a control-plane Blueprint service whose sync dropped
// MESSAGE, with a deploy recorder, a clock that moves between reads and a count
// of the App patches that land.
func retainedTracked() (*Service, *recordingDeploys, *versionedFakeSecretStore, *patchCountingClient) {
	rec := &recordingDeploys{}
	kv := newVersionedFakeSecretStore()
	svc := newService(kv, retainedManagedApp("bp"))
	svc.Rollout = &rollout.Tracker{Store: rec}
	now := fixedNow()
	svc.Clock = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	counting := &patchCountingClient{Client: svc.Client}
	svc.Client = counting
	return svc, rec, kv, counting
}

// requireOneWrite fails unless the verb wrote the store once and patched the App
// once: the move rides the verb's own write.
func requireOneWrite(t *testing.T, verb retainedVerb, kv *versionedFakeSecretStore, counting *patchCountingClient) {
	t.Helper()
	if kv.casCalls != 1 || kv.versions[envPath("bp")] != 1 {
		t.Errorf("%s made %d store writes, reaching revision %d, want one write to revision 1", verb.name, kv.casCalls, kv.versions[envPath("bp")])
	}
	if counting.patches != 1 {
		t.Errorf("%s patched the App %d times, want once", verb.name, counting.patches)
	}
}

// A staged (REST) write of a retained variable opens no deploy and rolls
// nothing: the running release keeps its references and its literal, the
// literal's removal and the stored value are held for the next deploy, and that
// deploy takes both. A delete of the retained variable is gone from the next
// release, and a write of it carries the value the write stored.
func TestAStagedWriteOfARetainedLiteralOpensNoDeploy(t *testing.T) {
	ctx := context.Background()
	for _, verb := range retainedVerbs {
		t.Run(verb.name, func(t *testing.T) {
			svc, rec, kv, counting := retainedTracked()
			w := serveREST(svc, verb.method, verb.path, verb.body)
			if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
				t.Fatalf("%s %s = %d: %s", verb.method, verb.path, w.Code, w.Body.String())
			}
			requireOneWrite(t, verb, kv, counting)
			if len(rec.rows) != 0 {
				t.Fatalf("the staged %s opened %d deploys (%+v), want none: Render's env-var writes deploy nothing", verb.name, len(rec.rows), rec.rows)
			}
			app := getApp(t, svc.Client, "bp")
			if app.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "" || app.Spec.RestartedAt != "" || app.Spec.EnvFromSecret != "" {
				t.Errorf("the staged %s rolled a release: release generation %q, restartedAt %q, envFromSecret %q",
					verb.name, app.Annotations[appv1alpha1.AnnotationReleaseGeneration], app.Spec.RestartedAt, app.Spec.EnvFromSecret)
			}
			if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
				t.Errorf("the literal's removal is not held for the next deploy: %v", app.Annotations)
			}
			if got := specEnvNames(app); len(got) != 1 || got["KEEPME"] != "1" {
				t.Errorf("spec.Env = %v, want only the declared KEEPME", got)
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, verb.wantEnv) {
				t.Errorf("the store holds %v, want %v", got, verb.wantEnv)
			}
			if got := secretData(t, svc.Client, "bp-env"); !maps.Equal(got, verb.wantEnv) {
				t.Errorf("the projected Secret holds %v, want %v", got, verb.wantEnv)
			}
			if want, ok := verb.wantEnv["MESSAGE"]; ok {
				if got, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); err != nil || got.Value != want {
					t.Errorf("MESSAGE = %+v (err %v), want %q", got, err, want)
				}
			} else if _, err := svc.GetEnvVar(ctx, "bp", "MESSAGE"); !errors.Is(err, core.ErrNotFound) {
				t.Errorf("after the delete GetEnvVar(MESSAGE) = %v, want not found", err)
			}

			// The next deploy is the one that rolls, and it takes the held
			// configuration: the Secret with the stored value, no literal.
			if _, err := svc.PatchEnvironment(ctx, "bp", EnvironmentPatch{
				SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{{Key: "NEXT", Value: "1", ValueSet: true}},
			}); err != nil {
				t.Fatal(err)
			}
			if len(rec.rows) != 1 || rec.rows[0].Trigger != store.TriggerConfigChange {
				t.Fatalf("the next deploy opened %+v, want one config_change row", rec.rows)
			}
			next := getApp(t, svc.Client, "bp")
			want := maps.Clone(verb.wantEnv)
			want["NEXT"] = "1"
			if next.Spec.EnvFromSecret != "bp-env" || !maps.Equal(secretData(t, svc.Client, "bp-env"), want) {
				t.Errorf("the next release consumes %q holding %v, want bp-env holding %v", next.Spec.EnvFromSecret, secretData(t, svc.Client, "bp-env"), want)
			}
			if _, still := specEnvNames(next)["MESSAGE"]; still {
				t.Error("the next release still carries the MESSAGE literal")
			}
		})
	}
}

// A deploying write (GraphQL, MCP, and a Service caller with no staged context)
// rolls the release it asked for exactly once: the one App patch that removes the
// literal is the one that stamps the release, so nothing is held behind it.
func TestADeployingWriteOfARetainedLiteralRollsOnce(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, svc *Service, rec *recordingDeploys, kv *versionedFakeSecretStore, counting *patchCountingClient, verb retainedVerb) {
		t.Helper()
		requireOneWrite(t, verb, kv, counting)
		if len(rec.rows) != 1 || rec.rows[0].Trigger != store.TriggerConfigChange {
			t.Fatalf("%s opened %+v, want exactly one config_change deploy", verb.name, rec.rows)
		}
		app := getApp(t, svc.Client, "bp")
		if app.Annotations[appv1alpha1.AnnotationReleaseGeneration] == "" || app.Spec.RestartedAt == "" || app.Spec.EnvFromSecret != "bp-env" {
			t.Errorf("%s did not roll the release it asked for: release generation %q, restartedAt %q, envFromSecret %q",
				verb.name, app.Annotations[appv1alpha1.AnnotationReleaseGeneration], app.Spec.RestartedAt, app.Spec.EnvFromSecret)
		}
		if held := app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; held != "" {
			t.Errorf("%s left the literal's removal held (pending-source-generation %q) behind the release it rolled", verb.name, held)
		}
		if got := specEnvNames(app); len(got) != 1 || got["KEEPME"] != "1" {
			t.Errorf("spec.Env = %v, want only the declared KEEPME", got)
		}
		if got := kv.m[envPath("bp")]; !maps.Equal(got, verb.wantEnv) {
			t.Errorf("the store holds %v, want %v", got, verb.wantEnv)
		}
		if got := secretData(t, svc.Client, "bp-env"); !maps.Equal(got, verb.wantEnv) {
			t.Errorf("the projected Secret holds %v, want %v", got, verb.wantEnv)
		}
	}
	for _, verb := range retainedVerbs {
		t.Run(verb.name+"/service", func(t *testing.T) {
			svc, rec, kv, counting := retainedTracked()
			if err := verb.call(ctx, svc); err != nil {
				t.Fatal(err)
			}
			check(t, svc, rec, kv, counting, verb)
		})
		t.Run(verb.name+"/GraphQL", func(t *testing.T) {
			svc, rec, kv, counting := retainedTracked()
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: verb.graphql}); res.HasErrors() {
				t.Fatal(res.Errors)
			}
			check(t, svc, rec, kv, counting, verb)
		})
		t.Run(verb.name+"/MCP", func(t *testing.T) {
			svc, rec, kv, counting := retainedTracked()
			res, err := mcpSession(t, svc).CallTool(ctx, &mcp.CallToolParams{Name: verb.mcpTool, Arguments: verb.mcpArgs})
			if err != nil || res.IsError {
				t.Fatalf("call %s: err=%v isErr=%v", verb.mcpTool, err, res != nil && res.IsError)
			}
			check(t, svc, rec, kv, counting, verb)
		})
	}
}

// A Blueprint sync that re-declares the name after the verb read the App but
// before the verb's App patch lands is not overwritten, as it is not on the
// environment patch: the verb is refused and the store write rolled back, the
// manifest's literal stays what the sync wrote, and nothing is stored beside it
// that the process would never see. The verb used to drop the literal the
// manifest had just written. The sync lands in each gap the verb has: right after
// its read of the store, and (the verbs that write the store before the App) right
// after the store has taken the write. A DELETE patches the App first and commits
// the store last, so the second gap does not exist for it.
func TestAVerbDoesNotDropALiteralTheManifestRedeclaredMeanwhile(t *testing.T) {
	ctx := context.Background()
	for _, verb := range retainedVerbs {
		for _, gap := range []string{"after the read", "after the store write"} {
			if gap == "after the store write" && verb.method == http.MethodDelete {
				continue
			}
			for _, staged := range []bool{true, false} {
				name := verb.name + "/" + gap + "/deploying"
				if staged {
					name = verb.name + "/" + gap + "/staged"
				}
				t.Run(name, func(t *testing.T) {
					svc, rec, kv, _ := retainedTracked()
					kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
					kv.versions[envPath("bp")] = 1
					sync := func() {
						a := getApp(t, svc.Client, "bp")
						a.Annotations[core.AnnotationBlueprintEnv] = "KEEPME,MESSAGE"
						a.Spec.Env[0].Value = "manifest wins"
						if err := svc.Client.Update(ctx, a); err != nil {
							t.Error(err)
						}
					}
					if gap == "after the read" {
						kv.afterGet = sync
					} else {
						kv.afterCAS = func(string, uint64) { sync() }
					}
					callCtx := ctx
					if staged {
						callCtx = WithStagedWrite(ctx)
					}
					err := verb.call(callCtx, svc)
					var coded *core.CodedError
					if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
						t.Fatalf("%s the manifest overtook = %v, want %s", verb.name, err, CodeEnvVarManifestManaged)
					}
					app := getApp(t, svc.Client, "bp")
					if got := specEnvNames(app); got["MESSAGE"] != "manifest wins" || got["KEEPME"] != "1" {
						t.Errorf("spec.Env = %v, want the manifest's literals as the sync left them", got)
					}
					if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "" {
						t.Errorf("the refused write left a saved change waiting for a deploy: %v", app.Annotations)
					}
					if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"OTHER": "2"}) {
						t.Errorf("the store holds %v, want what it held before the refused write", got)
					}
					if len(rec.rows) != 0 {
						t.Errorf("the refused write opened %d deploys", len(rec.rows))
					}
				})
			}
		}
	}
}

// A failure of the verb's App patch leaves the literal where it was and the
// store as it was, and opens no deploy, whatever the verb.
func TestAFailedWriteOfARetainedLiteralLeavesItWhereItWas(t *testing.T) {
	ctx := context.Background()
	for _, verb := range retainedVerbs {
		t.Run(verb.name, func(t *testing.T) {
			svc, rec, kv, _ := retainedTracked()
			kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
			kv.versions[envPath("bp")] = 1
			svc.Client = &patchCountingClient{Client: svc.Client, fail: errors.New("injected App patch failure")}
			if err := verb.call(WithStagedWrite(ctx), svc); err == nil {
				t.Fatal("the write succeeded although the App patch failed")
			}
			svc.Client = svc.Client.(*patchCountingClient).Client
			if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"OTHER": "2"}) {
				t.Errorf("the store holds %v, want what it held before the failed write", got)
			}
			app := getApp(t, svc.Client, "bp")
			if got := specEnvNames(app); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the literal kept", got)
			}
			if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "" {
				t.Errorf("the failed write left a saved change waiting for a deploy: %v", app.Annotations)
			}
			if len(rec.rows) != 0 {
				t.Errorf("the failed write opened %d deploys", len(rec.rows))
			}
		})
	}
}

// A write that leaves the stored map as it was still moves the literal: the
// store already held a value for the name (a stored duplicate beside the literal,
// which shadowed it), and the write sets that same value, replaces the set with
// itself or deletes a name only the literal held. The store is still written once
// (the revision advances, as every moved literal advances it) and the literal
// leaves spec.Env.
func TestAWriteThatLeavesTheStoredMapAsItWasStillMovesTheLiteral(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		stored map[string]string
		call   func(svc *Service) error
		want   map[string]string
	}{
		{"PUT one of the stored value", map[string]string{"MESSAGE": "hello from bex"}, func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "bp", "MESSAGE", EnvVarWrite{Value: "hello from bex"})
			return err
		}, map[string]string{"MESSAGE": "hello from bex"}},
		{"PUT all of the stored set", map[string]string{"MESSAGE": "hello from bex"}, func(svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "bp", []EnvVarView{{Key: "MESSAGE", Value: "hello from bex"}})
			return err
		}, map[string]string{"MESSAGE": "hello from bex"}},
		{"PUT all of nothing over an empty store", nil, func(svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "bp", nil)
			return err
		}, map[string]string{}},
		{"DELETE of a name only the literal held", map[string]string{"OTHER": "2"}, func(svc *Service) error {
			return svc.DeleteEnvVar(ctx, "bp", "MESSAGE")
		}, map[string]string{"OTHER": "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, kv, counting := retainedTracked()
			if tc.stored != nil {
				kv.m[envPath("bp")] = maps.Clone(tc.stored)
				kv.versions[envPath("bp")] = 3
			}
			if err := tc.call(svc); err != nil {
				t.Fatal(err)
			}
			want := uint64(1)
			if tc.stored != nil {
				want = 4
			}
			if kv.casCalls != 1 || kv.versions[envPath("bp")] != want || counting.patches != 1 {
				t.Errorf("%d store writes to revision %d and %d App patches, want one write to revision %d and one patch", kv.casCalls, kv.versions[envPath("bp")], counting.patches, want)
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, tc.want) {
				t.Errorf("the store holds %v, want %v", got, tc.want)
			}
			if got := specEnvNames(getApp(t, svc.Client, "bp")); len(got) != 1 || got["KEEPME"] != "1" {
				t.Errorf("spec.Env = %v, want the literal gone and the declared KEEPME kept", got)
			}
		})
	}
}
