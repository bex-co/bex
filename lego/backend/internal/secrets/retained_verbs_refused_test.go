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
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A variable a Blueprint dropped stays on spec.Env until a write names it. The
// single-variable verbs and the bulk replace make that move inside their own
// write (w21/108), so a write the API refuses moves nothing: the literal stays
// on the service, nothing is stored, no App patch lands, and the service carries
// no saved change that waits for a deploy. They used to move the literal first,
// before the value, the aggregate quota or the key was checked, and left
// app.bex.co/pending-source-generation set behind a 400 or a 404.

// refusedRetainedWrite is a write of the retained MESSAGE (or of a name that is
// not there) that the API refuses, through each surface.
type refusedRetainedWrite struct {
	name   string
	stored map[string]string // the store before the write
	// REST
	method, path, body string
	wantStatus         int
	// GraphQL's mutation and MCP's tool and arguments
	graphql string
	mcpTool string
	mcpArgs map[string]any
}

// nearlyFullEnvValue fills the aggregate quota (512 KiB, keys and values) up to
// 100 bytes: the retained MESSAGE (21 bytes) still fits beside it, a write of a
// longer value does not.
func nearlyFullEnvValue() string { return strings.Repeat("x", maxSecretMapBytes-100) }

func refusedRetainedWrites() []refusedRetainedWrite {
	over := strings.Repeat("y", maxSecretMapBytes)
	longer := strings.Repeat("z", 200)
	return []refusedRetainedWrite{
		{
			name: "PUT one with value and generateValue", stored: map[string]string{"OTHER": "2"},
			method: http.MethodPut, path: "/v1/services/bp/env-vars/MESSAGE", body: `{"value":"x","generateValue":true}`, wantStatus: http.StatusBadRequest,
			graphql: `mutation { setEnvVar(serviceId:"bp", key:"MESSAGE", value:"x", generateValue:true) }`,
			mcpTool: "set_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "MESSAGE", "value": "x", "generateValue": true},
		},
		{
			name: "PUT one over the aggregate quota", stored: map[string]string{"BIG": nearlyFullEnvValue()},
			method: http.MethodPut, path: "/v1/services/bp/env-vars/MESSAGE", body: fmt.Sprintf(`{"value":%q}`, longer), wantStatus: http.StatusBadRequest,
			graphql: fmt.Sprintf(`mutation { setEnvVar(serviceId:"bp", key:"MESSAGE", value:%q) }`, longer),
			mcpTool: "set_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "MESSAGE", "value": longer},
		},
		{
			name: "PUT all with value and generateValue", stored: map[string]string{"OTHER": "2"},
			method: http.MethodPut, path: "/v1/services/bp/env-vars", body: `[{"key":"MESSAGE","value":"x","generateValue":true}]`, wantStatus: http.StatusBadRequest,
			graphql: `mutation { setEnvVars(serviceId:"bp", envVars:[{key:"MESSAGE", value:"x", generateValue:true}]) }`,
			mcpTool: "update_env_vars", mcpArgs: map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "MESSAGE", "value": "x", "generateValue": true}}},
		},
		{
			name: "PUT all with an invalid name", stored: map[string]string{"OTHER": "2"},
			method: http.MethodPut, path: "/v1/services/bp/env-vars", body: `[{"key":"BAD KEY","value":"x"}]`, wantStatus: http.StatusBadRequest,
			graphql: `mutation { setEnvVars(serviceId:"bp", envVars:[{key:"BAD KEY", value:"x"}]) }`,
			mcpTool: "update_env_vars", mcpArgs: map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "BAD KEY", "value": "x"}}},
		},
		{
			name: "PUT all over the aggregate quota", stored: map[string]string{"OTHER": "2"},
			method: http.MethodPut, path: "/v1/services/bp/env-vars", body: fmt.Sprintf(`[{"key":"MESSAGE","value":%q}]`, over), wantStatus: http.StatusBadRequest,
			graphql: fmt.Sprintf(`mutation { setEnvVars(serviceId:"bp", envVars:[{key:"MESSAGE", value:%q}]) }`, over),
			mcpTool: "update_env_vars", mcpArgs: map[string]any{"serviceId": "bp", "envVars": []map[string]any{{"key": "MESSAGE", "value": over}}},
		},
		{
			name: "DELETE of a name that does not exist", stored: map[string]string{"OTHER": "2"},
			method: http.MethodDelete, path: "/v1/services/bp/env-vars/NOPE", wantStatus: http.StatusNotFound,
			graphql: `mutation { deleteEnvVar(serviceId:"bp", key:"NOPE") }`,
			mcpTool: "delete_env_var", mcpArgs: map[string]any{"serviceId": "bp", "key": "NOPE"},
		},
	}
}

// retainedWithStore is retainedTracked over a store that already holds stored.
func retainedWithStore(stored map[string]string) (*Service, *recordingDeploys, *versionedFakeSecretStore, *patchCountingClient) {
	svc, rec, kv, counting := retainedTracked()
	kv.m[envPath("bp")] = maps.Clone(stored)
	kv.versions[envPath("bp")] = 1
	return svc, rec, kv, counting
}

// requireNothingMoved fails unless the refused write left the service as it was:
// the literal on spec.Env, nothing saved for a deploy, nothing stored, and, for
// every refusal but the DELETE of a name that is not there, no App patch, no
// release rolled and no deploy opened.
//
// That DELETE still runs the repair of a stale projection (a Secret key the
// store no longer holds, pinned by TestDeleteEnvVarAlreadyAbsentRepairsStaleProjection),
// which patches the App, and rolls a release on GraphQL and MCP. It is the
// verb's own behavior for any service, retained variable or not (filed as its
// own note), so what is asserted of it is what w21/108 owns: it moves no
// literal and saves no change that waits for a deploy.
func requireNothingMoved(t *testing.T, tc refusedRetainedWrite, svc *Service, rec *recordingDeploys, kv *versionedFakeSecretStore, counting *patchCountingClient) {
	t.Helper()
	app := getApp(t, svc.Client, "bp")
	if got := specEnvNames(app); len(got) != 2 || got["MESSAGE"] != "hello from bex" || got["KEEPME"] != "1" {
		t.Errorf("spec.Env = %v, want the retained literal still on the service beside the declared one", got)
	}
	if held := app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; held != "" {
		t.Errorf("the refused write left a saved change waiting for a deploy: pending-source-generation %q", held)
	}
	if got := kv.m[envPath("bp")]; !maps.Equal(got, tc.stored) {
		t.Errorf("the store holds %d keys, want what it held before the refused write", len(got))
	}
	if kv.casCalls != 0 {
		t.Errorf("the refused write made %d store writes, want none", kv.casCalls)
	}
	if tc.method == http.MethodDelete {
		return
	}
	if app.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "" || app.Spec.RestartedAt != "" || app.Spec.EnvFromSecret != "" {
		t.Errorf("the refused write rolled a release: generation %q, restartedAt %q, envFromSecret %q",
			app.Annotations[appv1alpha1.AnnotationReleaseGeneration], app.Spec.RestartedAt, app.Spec.EnvFromSecret)
	}
	if len(rec.rows) != 0 {
		t.Errorf("the refused write opened %d deploys", len(rec.rows))
	}
	if counting != nil && counting.patches != 0 {
		t.Errorf("the refused write patched the App %d times, want none", counting.patches)
	}
}

// A write the API refuses moves nothing, whichever surface it came in by.
func TestARefusedWriteOfARetainedVariableMovesNothing(t *testing.T) {
	ctx := context.Background()
	for _, tc := range refusedRetainedWrites() {
		t.Run(tc.name+"/REST", func(t *testing.T) {
			svc, rec, kv, counting := retainedWithStore(tc.stored)
			w := serveREST(svc, tc.method, tc.path, tc.body)
			if w.Code != tc.wantStatus {
				t.Fatalf("%s %s = %d, want %d: %.200s", tc.method, tc.path, w.Code, tc.wantStatus, w.Body.String())
			}
			requireNothingMoved(t, tc, svc, rec, kv, counting)
		})
		t.Run(tc.name+"/GraphQL", func(t *testing.T) {
			svc, rec, kv, counting := retainedWithStore(tc.stored)
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
				Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: tc.graphql}); !res.HasErrors() {
				t.Fatal("GraphQL accepted a write the API refuses")
			}
			requireNothingMoved(t, tc, svc, rec, kv, counting)
		})
		t.Run(tc.name+"/MCP", func(t *testing.T) {
			svc, rec, kv, counting := retainedWithStore(tc.stored)
			res, err := mcpSession(t, svc).CallTool(ctx, &mcp.CallToolParams{Name: tc.mcpTool, Arguments: tc.mcpArgs})
			if err == nil && (res == nil || !res.IsError) {
				t.Fatal("MCP accepted a write the API refuses")
			}
			requireNothingMoved(t, tc, svc, rec, kv, counting)
		})
	}
}

// The refusal does not depend on a revision-aware store: a store without
// versions (a test double, or a store that predates them) moves nothing either.
func TestARefusedWriteOfARetainedVariableMovesNothingOnAnUnversionedStore(t *testing.T) {
	for _, tc := range refusedRetainedWrites() {
		t.Run(tc.name, func(t *testing.T) {
			kv := newFakeSecretStore()
			kv.m[envPath("bp")] = maps.Clone(tc.stored)
			svc := newService(kv, retainedManagedApp("bp"))
			w := serveREST(svc, tc.method, tc.path, tc.body)
			if w.Code != tc.wantStatus {
				t.Fatalf("%s %s = %d, want %d: %.200s", tc.method, tc.path, w.Code, tc.wantStatus, w.Body.String())
			}
			app := getApp(t, svc.Client, "bp")
			if got := specEnvNames(app); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the retained literal kept", got)
			}
			if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "" {
				t.Errorf("the refused write left pending-source-generation set: %v", app.Annotations)
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, tc.stored) {
				t.Errorf("the refused write changed the store (%d keys)", len(got))
			}
		})
	}
}

// A store that refuses the write (an outage, a conflict it cannot get past)
// moves nothing either: the literal is still on the service and nothing is
// saved. A DELETE is not in this list: it converges in the other order, the
// projection and the App first (so a revoked value stops reaching the pods
// whatever the store does) and the store commit last, as for every variable, and
// a retained literal is one of those the delete removes first.
func TestAStoreFailureOnAWriteOfARetainedVariableMovesNothing(t *testing.T) {
	for _, verb := range retainedVerbs {
		if verb.method == http.MethodDelete {
			continue
		}
		t.Run(verb.name, func(t *testing.T) {
			svc, rec, kv, counting := retainedWithStore(map[string]string{"OTHER": "2"})
			kv.failCASCall = 1
			if err := verb.call(WithStagedWrite(context.Background()), svc); err == nil {
				t.Fatal("the write succeeded although the store refused it")
			}
			if got := kv.m[envPath("bp")]; !maps.Equal(got, map[string]string{"OTHER": "2"}) {
				t.Errorf("the store holds %v, want what it held before the failed write", got)
			}
			app := getApp(t, svc.Client, "bp")
			if got := specEnvNames(app); got["MESSAGE"] != "hello from bex" {
				t.Errorf("spec.Env = %v, want the literal kept", got)
			}
			if app.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "" {
				t.Errorf("the failed write left a saved change: %v", app.Annotations)
			}
			if len(rec.rows) != 0 || counting.patches != 0 {
				t.Errorf("the failed write opened %d deploys and patched the App %d times", len(rec.rows), counting.patches)
			}
		})
	}
}
