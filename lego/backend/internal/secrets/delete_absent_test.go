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
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's DELETE .../env-vars/{key} answers 404 for a key that is not there and
// changes nothing (api-docs.render.com/reference/delete-env-var documents the 404
// and no other effect; the same holds for a secret file). A DELETE here used to
// re-project the reduced set before it knew the key existed, which patched the
// App: over REST (staged) it saved a change, over GraphQL and MCP it bumped
// restartedAt, set spec.envFromSecret, stamped a release generation and opened a
// config_change deploy, on every service (w21/109). It now projects an absent
// key's delete only when the derived Secret really carries the key, the repair of
// a delete an older release left half done (TestDeleteEnvVarAlreadyAbsentRepairsStaleProjection).

// absentKind is one kind of per-service child the delete verbs remove.
type absentKind struct {
	name       string
	storePath  string
	secretName string
	stored     map[string]string // what the store holds before the delete
	missing    string            // a name nothing holds
	stale      string            // a name only the derived Secret may hold
	call       func(ctx context.Context, svc *Service, name string) error
	restPath   func(name string) string
	graphql    func(name string) string
	mcpTool    string
	mcpArgs    func(name string) map[string]any
	// refer points a projected App at the derived Secret, as a service that has
	// rolled once does.
	refer func(a *appv1alpha1.App)
}

var absentKinds = []absentKind{
	{
		name: "env var", storePath: envPath("bp"), secretName: "bp-env",
		stored: map[string]string{"KEEP": "1"}, missing: "NOPE", stale: "REVOKED",
		call:     func(ctx context.Context, svc *Service, name string) error { return svc.DeleteEnvVar(ctx, "bp", name) },
		restPath: func(name string) string { return "/v1/services/bp/env-vars/" + name },
		graphql:  func(name string) string { return `mutation { deleteEnvVar(serviceId:"bp", key:"` + name + `") }` },
		mcpTool:  "delete_env_var",
		mcpArgs:  func(name string) map[string]any { return map[string]any{"serviceId": "bp", "key": name} },
		refer:    func(a *appv1alpha1.App) { a.Spec.EnvFromSecret = "bp-env" },
	},
	{
		name: "secret file", storePath: filesPath("bp"), secretName: "bp-files",
		stored: map[string]string{"keep.txt": "1"}, missing: "nope.txt", stale: "revoked.txt",
		call: func(ctx context.Context, svc *Service, name string) error {
			return svc.DeleteSecretFile(ctx, "bp", name)
		},
		restPath: func(name string) string { return "/v1/services/bp/secret-files/" + name },
		graphql:  func(name string) string { return `mutation { deleteSecretFile(serviceId:"bp", name:"` + name + `") }` },
		mcpTool:  "delete_secret_file",
		mcpArgs:  func(name string) map[string]any { return map[string]any{"serviceId": "bp", "name": name} },
		refer:    func(a *appv1alpha1.App) { a.Spec.FilesFromSecrets = []string{"bp-files"} },
	},
}

// secretWriteCounter counts the Secret writes that land.
type secretWriteCounter struct {
	client.Client
	writes int
}

func (c *secretWriteCounter) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Secret); ok {
		c.writes++
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *secretWriteCounter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Secret); ok {
		c.writes++
	}
	return c.Client.Update(ctx, obj, opts...)
}

// absentRig is a control-plane service with a deploy recorder, a clock that
// moves, and a count of the store writes, App patches and Secret writes that
// land.
type absentRig struct {
	svc     *Service
	rec     *recordingDeploys
	kv      *versionedFakeSecretStore
	patches *patchCountingClient
	secrets *secretWriteCounter
	before  *appv1alpha1.App
	kind    absentKind
	// the store as the verb sees it: its map and how many writes it took
	storeMap    func() map[string]string
	storeWrites func() int
}

// newAbsentRig builds the service: with a retained variable or none, with the
// derived Secret projected (holding the store's keys and, projected with extra,
// the stale ones) or not yet written.
func newAbsentRig(t *testing.T, k absentKind, retained, projected bool, stale ...string) *absentRig {
	t.Helper()
	app := managedApp("bp")
	if retained {
		app = retainedManagedApp("bp")
	}
	objs := []client.Object{app}
	if projected {
		k.refer(app)
		data := map[string][]byte{}
		for name, value := range k.stored {
			data[name] = []byte(value)
		}
		for _, name := range stale {
			data[name] = []byte("stale")
		}
		objs = append(objs, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: k.secretName, Namespace: "default"}, Data: data})
	}
	kv := newVersionedFakeSecretStore()
	kv.m[k.storePath] = maps.Clone(k.stored)
	kv.versions[k.storePath] = 1
	rec := &recordingDeploys{}
	svc := newService(kv, objs...)
	svc.Rollout = &rollout.Tracker{Store: rec}
	now := fixedNow()
	svc.Clock = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	patches := &patchCountingClient{Client: svc.Client}
	secrets := &secretWriteCounter{Client: patches}
	svc.Client = secrets
	return &absentRig{
		svc: svc, rec: rec, kv: kv, patches: patches, secrets: secrets, kind: k, before: getApp(t, svc.Client, "bp"),
		storeMap: func() map[string]string { return kv.m[k.storePath] }, storeWrites: func() int { return kv.casCalls },
	}
}

// requireNothingChanged fails unless the DELETE left the service exactly as it
// was: no App patch (so no restartedAt bump, no envFromSecret, no release
// generation and no saved-config or pending-env-secret annotation), no deploy row,
// no store write and no Secret write.
func (r *absentRig) requireNothingChanged(t *testing.T) {
	t.Helper()
	after := getApp(t, r.svc.Client, "bp")
	if !equality.Semantic.DeepEqual(after.Spec, r.before.Spec) || !maps.Equal(after.Annotations, r.before.Annotations) {
		t.Errorf("the DELETE of a name that is not there changed the App: restartedAt %q, envFromSecret %q, release generation %q, saved-config-revision %q, pending-env-secret %q, annotations %v",
			after.Spec.RestartedAt, after.Spec.EnvFromSecret, after.Annotations[appv1alpha1.AnnotationReleaseGeneration],
			after.Annotations[appv1alpha1.AnnotationSavedConfigRevision], after.Annotations[appv1alpha1.PendingEnvSecretAnnotation], after.Annotations)
	}
	if r.patches.patches != 0 {
		t.Errorf("the DELETE of a name that is not there patched the App %d times, want none", r.patches.patches)
	}
	if len(r.rec.rows) != 0 {
		t.Errorf("the DELETE of a name that is not there opened %d deploys (%+v), want none", len(r.rec.rows), r.rec.rows)
	}
	if r.storeWrites() != 0 || !maps.Equal(r.storeMap(), r.kind.stored) {
		t.Errorf("the DELETE of a name that is not there wrote the store (%d writes, now %v)", r.storeWrites(), r.storeMap())
	}
	if r.secrets.writes != 0 {
		t.Errorf("the DELETE of a name that is not there wrote the derived Secret %d times, want none", r.secrets.writes)
	}
}

// absentShapes are the services a DELETE runs against: a plain one and one that
// holds a retained variable, each before its first projection and after it.
var absentShapes = []struct {
	name                string
	retained, projected bool
}{
	{"plain", false, false},
	{"plain and projected", false, true},
	{"retained variable", true, false},
	{"retained variable and projected", true, true},
}

// absentGraphQLSchema is the schema holding the secrets verbs' mutations.
func absentGraphQLSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{"ok": &graphql.Field{Type: graphql.Boolean}}}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// A DELETE of a name that neither the store, the derived Secret nor spec.Env
// holds is not found on every surface and changes nothing.
func TestDeleteOfAnAbsentNameChangesNothing(t *testing.T) {
	ctx := context.Background()
	for _, k := range absentKinds {
		for _, shape := range absentShapes {
			prefix := k.name + "/" + shape.name + "/"
			t.Run(prefix+"REST", func(t *testing.T) {
				r := newAbsentRig(t, k, shape.retained, shape.projected)
				w := serveREST(r.svc, http.MethodDelete, k.restPath(k.missing), "")
				if w.Code != http.StatusNotFound {
					t.Fatalf("DELETE %s = %d, want 404: %.200s", k.restPath(k.missing), w.Code, w.Body.String())
				}
				r.requireNothingChanged(t)
			})
			for _, staged := range []bool{true, false} {
				name := prefix + "Service/deploying"
				callCtx := ctx
				if staged {
					name, callCtx = prefix+"Service/staged", WithStagedWrite(ctx)
				}
				t.Run(name, func(t *testing.T) {
					r := newAbsentRig(t, k, shape.retained, shape.projected)
					if err := k.call(callCtx, r.svc, k.missing); !errors.Is(err, core.ErrNotFound) {
						t.Fatalf("delete = %v, want not found", err)
					}
					r.requireNothingChanged(t)
				})
			}
			t.Run(prefix+"GraphQL", func(t *testing.T) {
				r := newAbsentRig(t, k, shape.retained, shape.projected)
				schema := absentGraphQLSchema(t, r.svc)
				res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: k.graphql(k.missing)})
				if !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "not found") {
					t.Fatalf("GraphQL delete = %+v, want a not-found error", res)
				}
				r.requireNothingChanged(t)
			})
			t.Run(prefix+"MCP", func(t *testing.T) {
				r := newAbsentRig(t, k, shape.retained, shape.projected)
				res, err := mcpSession(t, r.svc).CallTool(ctx, &mcp.CallToolParams{Name: k.mcpTool, Arguments: k.mcpArgs(k.missing)})
				if err == nil && (res == nil || !res.IsError) {
					t.Fatal("MCP accepted a delete of a name that is not there")
				}
				r.requireNothingChanged(t)
			})
		}
	}
}

// The retained literal stays on the service beside the declared one when a
// delete of another name is refused, and the delete of the retained name itself
// is still a found key (w21/108): only the literal holds it, the store does not.
func TestDeleteOfAnAbsentNameLeavesTheRetainedLiteral(t *testing.T) {
	r := newAbsentRig(t, absentKinds[0], true, false)
	if err := r.svc.DeleteEnvVar(context.Background(), "bp", "NOPE"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete = %v, want not found", err)
	}
	if got := specEnvNames(getApp(t, r.svc.Client, "bp")); len(got) != 2 || got["MESSAGE"] != "hello from bex" || got["KEEPME"] != "1" {
		t.Errorf("spec.Env = %v, want both literals still on the service", got)
	}
	if held := getApp(t, r.svc.Client, "bp").Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; held != "" {
		t.Errorf("a refused delete left a saved change waiting for a deploy: %q", held)
	}

	// The retained name is found with the store and the Secret not holding it.
	if err := r.svc.DeleteEnvVar(context.Background(), "bp", "MESSAGE"); err != nil {
		t.Fatalf("delete of the name only the literal holds = %v, want success", err)
	}
	if r.kv.casCalls != 1 || r.patches.patches != 1 {
		t.Errorf("the delete of the retained literal made %d store writes and %d App patches, want one each", r.kv.casCalls, r.patches.patches)
	}
	if _, still := specEnvNames(getApp(t, r.svc.Client, "bp"))["MESSAGE"]; still {
		t.Error("the delete left the literal on the service")
	}
}

// A name the derived Secret still carries after the store lost it (a delete an
// older release left half done, which committed its source CAS before the
// projection) is projected clean and the workload rolled, on whichever surface
// the retry comes in: a REST (staged) retry holds the change for the next deploy,
// a GraphQL or MCP retry rolls exactly once. Either still answers not found.
func TestDeleteOfANameOnlyTheSecretHoldsRepairsItOnce(t *testing.T) {
	ctx := context.Background()
	for _, k := range absentKinds {
		for _, retained := range []bool{false, true} {
			prefix := k.name + "/"
			if retained {
				prefix += "retained variable/"
			}
			check := func(t *testing.T, r *absentRig, staged bool) {
				t.Helper()
				if data := secretData(t, r.svc.Client, k.secretName); !maps.Equal(data, k.stored) {
					t.Errorf("the derived Secret holds %v, want only what the store holds %v", data, k.stored)
				}
				if r.patches.patches != 1 {
					t.Errorf("the repair patched the App %d times, want once", r.patches.patches)
				}
				if r.kv.casCalls != 0 || !maps.Equal(r.kv.m[k.storePath], k.stored) {
					t.Errorf("the repair wrote the store (%d writes, now %v)", r.kv.casCalls, r.kv.m[k.storePath])
				}
				app := getApp(t, r.svc.Client, "bp")
				if staged {
					if len(r.rec.rows) != 0 || app.Spec.RestartedAt != "" || app.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "" {
						t.Errorf("the staged repair rolled a release: %d deploys, restartedAt %q, generation %q",
							len(r.rec.rows), app.Spec.RestartedAt, app.Annotations[appv1alpha1.AnnotationReleaseGeneration])
					}
					if app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] == "" {
						t.Error("the staged repair left no saved change for the status comparison")
					}
					return
				}
				if len(r.rec.rows) != 1 || r.rec.rows[0].Trigger != store.TriggerConfigChange || app.Spec.RestartedAt == "" {
					t.Errorf("the repair opened %+v with restartedAt %q, want one config_change deploy and a roll", r.rec.rows, app.Spec.RestartedAt)
				}
			}
			t.Run(prefix+"REST", func(t *testing.T) {
				r := newAbsentRig(t, k, retained, true, k.stale)
				if w := serveREST(r.svc, http.MethodDelete, k.restPath(k.stale), ""); w.Code != http.StatusNotFound {
					t.Fatalf("DELETE = %d, want 404: %.200s", w.Code, w.Body.String())
				}
				check(t, r, true)
			})
			t.Run(prefix+"Service", func(t *testing.T) {
				r := newAbsentRig(t, k, retained, true, k.stale)
				if err := k.call(ctx, r.svc, k.stale); !errors.Is(err, core.ErrNotFound) {
					t.Fatalf("delete = %v, want not found", err)
				}
				check(t, r, false)
			})
			t.Run(prefix+"GraphQL", func(t *testing.T) {
				r := newAbsentRig(t, k, retained, true, k.stale)
				schema := absentGraphQLSchema(t, r.svc)
				if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: k.graphql(k.stale)}); !res.HasErrors() {
					t.Fatal("GraphQL delete of a stale name reported success")
				}
				check(t, r, false)
			})
			t.Run(prefix+"MCP", func(t *testing.T) {
				r := newAbsentRig(t, k, retained, true, k.stale)
				res, err := mcpSession(t, r.svc).CallTool(ctx, &mcp.CallToolParams{Name: k.mcpTool, Arguments: k.mcpArgs(k.stale)})
				if err == nil && (res == nil || !res.IsError) {
					t.Fatal("MCP delete of a stale name reported success")
				}
				check(t, r, false)
			})
		}
	}
}

// The same holds without store versions (a test double, or a store that predates
// them): nothing to repair moves nothing, a stale Secret is repaired.
func TestDeleteOfAnAbsentNameOnAnUnversionedStore(t *testing.T) {
	ctx := context.Background()
	for _, k := range absentKinds {
		for _, stale := range []bool{false, true} {
			name := k.name + "/nothing to repair"
			var extra []string
			if stale {
				name, extra = k.name+"/stale Secret", []string{k.stale}
			}
			t.Run(name, func(t *testing.T) {
				r := newAbsentRig(t, k, false, true, extra...)
				kv := newFakeSecretStore()
				kv.m[k.storePath] = maps.Clone(k.stored)
				r.svc.Store = kv
				r.storeMap, r.storeWrites = func() map[string]string { return kv.m[k.storePath] }, func() int { return kv.puts }
				missing := k.missing
				if stale {
					missing = k.stale
				}
				if err := k.call(ctx, r.svc, missing); !errors.Is(err, core.ErrNotFound) {
					t.Fatalf("delete = %v, want not found", err)
				}
				if !stale {
					r.requireNothingChanged(t)
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

// secretReadFailure refuses to read a Secret, as an API server that is down does.
type secretReadFailure struct{ client.Client }

func (c *secretReadFailure) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Secret); ok {
		return apierrors.NewServiceUnavailable("injected Secret read failure")
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// A Secret that cannot be read cannot be told from one that holds a revoked
// value, so the delete answers the failure rather than not found, and changes
// nothing.
func TestDeleteOfAnAbsentNameFailsWhenTheSecretCannotBeRead(t *testing.T) {
	for _, k := range absentKinds {
		t.Run(k.name, func(t *testing.T) {
			r := newAbsentRig(t, k, false, true)
			r.svc.Client = &secretReadFailure{Client: r.svc.Client}
			err := k.call(context.Background(), r.svc, k.missing)
			if err == nil || errors.Is(err, core.ErrNotFound) {
				t.Fatalf("delete = %v, want the Secret read failure", err)
			}
			r.svc.Client = r.secrets
			r.requireNothingChanged(t)
		})
	}
}

// A writer that commits the name between the delete's read of the store and its
// read of the Secret is not undone: the delete read the map before the write, so
// it answers not found as it would had it run first, and the repair it then sees
// (the writer's own projection, a newer revision than the one it read) is
// refused as superseded, so the writer's value survives in the store and in the
// Secret.
func TestDeleteOfANameAWriterCommitsMeanwhileKeepsTheWritersValue(t *testing.T) {
	k := absentKinds[0]
	r := newAbsentRig(t, k, false, true)
	r.kv.afterGet = func() {
		if _, err := r.svc.SetEnvVar(context.Background(), "bp", "NEW", EnvVarWrite{Value: "written"}); err != nil {
			t.Errorf("the concurrent write: %v", err)
		}
	}
	if err := r.svc.DeleteEnvVar(context.Background(), "bp", "NEW"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete = %v, want not found (it read the map before the write)", err)
	}
	if got := r.kv.m[k.storePath]; got["NEW"] != "written" {
		t.Errorf("the store holds %v, want the writer's NEW kept", got)
	}
	if got := secretData(t, r.svc.Client, k.secretName); got["NEW"] != "written" || got["KEEP"] != "1" {
		t.Errorf("the derived Secret holds %v, want the writer's NEW kept beside KEEP", got)
	}
}

// The invalid-name branch of DeleteEnvVar (debris a create stored before names
// were validated) patches the App only when it strips a literal: a missing
// invalid name patches nothing.
func TestDeleteOfAMissingInvalidNamePatchesNothing(t *testing.T) {
	a := managedApp("bp")
	a.Spec.Env = []appv1alpha1.EnvVar{{Name: "BAD KEY", Value: "y"}, {Name: "MESSAGE", Value: "kept"}}
	svc := newService(newVersionedFakeSecretStore(), a)
	counting := &patchCountingClient{Client: svc.Client}
	svc.Client = counting
	if err := svc.DeleteEnvVar(context.Background(), "bp", "BAD KEY TOO"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("delete = %v, want not found", err)
	}
	if counting.patches != 0 {
		t.Errorf("the delete of a missing invalid name patched the App %d times, want none", counting.patches)
	}
	if err := svc.DeleteEnvVar(context.Background(), "bp", "BAD KEY"); err != nil {
		t.Fatalf("delete of the debris literal = %v, want success", err)
	}
	if counting.patches != 1 {
		t.Errorf("stripping the debris literal patched the App %d times, want once", counting.patches)
	}
}
