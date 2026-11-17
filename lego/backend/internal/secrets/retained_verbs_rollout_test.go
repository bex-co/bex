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
// bulk replace first move it into the store in a write of its own that removes
// the literal from the pod template. That write used to roll a release and open
// a deploy row of its own, for a staged (REST) write that asked for none and,
// with the verb's own roll after it, a second one for a deploying (GraphQL,
// MCP) write (w21/106). These tests run the verbs on a control-plane service
// with a deploy recorder and a clock that moves, as a real one does: the
// verb's own roll is a restartedAt that differs from the adoption's.

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
// MESSAGE, with a deploy recorder and a clock that moves between reads.
func retainedTracked() (*Service, *recordingDeploys, *versionedFakeSecretStore) {
	rec := &recordingDeploys{}
	kv := newVersionedFakeSecretStore()
	svc := newService(kv, retainedManagedApp("bp"))
	svc.Rollout = &rollout.Tracker{Store: rec}
	now := fixedNow()
	svc.Clock = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	return svc, rec, kv
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
			svc, rec, kv := retainedTracked()
			w := serveREST(svc, verb.method, verb.path, verb.body)
			if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
				t.Fatalf("%s %s = %d: %s", verb.method, verb.path, w.Code, w.Body.String())
			}
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
// rolls the release it asked for exactly once: the adoption's own write is held,
// and the verb's write stamps the release and consumes the hold.
func TestADeployingWriteOfARetainedLiteralRollsOnce(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, svc *Service, rec *recordingDeploys, kv *versionedFakeSecretStore, verb retainedVerb) {
		t.Helper()
		if len(rec.rows) != 1 || rec.rows[0].Trigger != store.TriggerConfigChange {
			t.Fatalf("%s opened %+v, want exactly one config_change deploy", verb.name, rec.rows)
		}
		app := getApp(t, svc.Client, "bp")
		if app.Annotations[appv1alpha1.AnnotationReleaseGeneration] == "" || app.Spec.RestartedAt == "" || app.Spec.EnvFromSecret != "bp-env" {
			t.Errorf("%s did not roll the release it asked for: release generation %q, restartedAt %q, envFromSecret %q",
				verb.name, app.Annotations[appv1alpha1.AnnotationReleaseGeneration], app.Spec.RestartedAt, app.Spec.EnvFromSecret)
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
			svc, rec, kv := retainedTracked()
			if err := verb.call(ctx, svc); err != nil {
				t.Fatal(err)
			}
			check(t, svc, rec, kv, verb)
		})
		t.Run(verb.name+"/GraphQL", func(t *testing.T) {
			svc, rec, kv := retainedTracked()
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
			check(t, svc, rec, kv, verb)
		})
		t.Run(verb.name+"/MCP", func(t *testing.T) {
			svc, rec, kv := retainedTracked()
			res, err := mcpSession(t, svc).CallTool(ctx, &mcp.CallToolParams{Name: verb.mcpTool, Arguments: verb.mcpArgs})
			if err != nil || res.IsError {
				t.Fatalf("call %s: err=%v isErr=%v", verb.mcpTool, err, res != nil && res.IsError)
			}
			check(t, svc, rec, kv, verb)
		})
	}
}

// A Blueprint sync that re-declares the name after the verb read the App but
// before the adoption's App patch lands is not overwritten, as it is not on the
// environment patch: the verb is refused and rolled back, the manifest's literal
// stays what the sync wrote, and nothing is stored beside it that the process
// would never see. The verb used to drop the literal the manifest had just
// written.
func TestAVerbDoesNotDropALiteralTheManifestRedeclaredMeanwhile(t *testing.T) {
	ctx := context.Background()
	for _, verb := range retainedVerbs {
		for _, staged := range []bool{true, false} {
			name := verb.name + "/deploying"
			if staged {
				name = verb.name + "/staged"
			}
			t.Run(name, func(t *testing.T) {
				svc, rec, kv := retainedTracked()
				kv.m[envPath("bp")] = map[string]string{"OTHER": "2"}
				kv.versions[envPath("bp")] = 1
				// The sync lands as soon as the store has taken the adoption.
				kv.afterCAS = func(string, uint64) {
					a := getApp(t, svc.Client, "bp")
					a.Annotations[core.AnnotationBlueprintEnv] = "KEEPME,MESSAGE"
					a.Spec.Env[0].Value = "manifest wins"
					if err := svc.Client.Update(ctx, a); err != nil {
						t.Error(err)
					}
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
				if got := specEnvNames(getApp(t, svc.Client, "bp")); got["MESSAGE"] != "manifest wins" || got["KEEPME"] != "1" {
					t.Errorf("spec.Env = %v, want the manifest's literals as the sync left them", got)
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

// A failure of the adoption's App patch leaves the literal where it was and the
// store as it was, and opens no deploy, whatever the verb.
func TestAFailedAdoptionOfARetainedLiteralLeavesItWhereItWas(t *testing.T) {
	ctx := context.Background()
	for _, verb := range retainedVerbs {
		t.Run(verb.name, func(t *testing.T) {
			svc, rec, kv := retainedTracked()
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
			if got := specEnvNames(getApp(t, svc.Client, "bp")); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the literal kept", got)
			}
			if len(rec.rows) != 0 {
				t.Errorf("the failed write opened %d deploys", len(rec.rows))
			}
		})
	}
}
