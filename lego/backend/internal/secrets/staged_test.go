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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/186: Render's REST env-var and secret-file verbs save without deploying.
// Every one stores the value and leaves spec.restartedAt, the spec and any
// deploy row alone, while a plain (GraphQL/MCP) call still rolls.
func TestStagedWritesSaveWithoutRollout(t *testing.T) {
	const running = "running-release"
	steps := []struct {
		name  string
		write func(svc *Service, staged bool) error
		check func(t *testing.T, store *fakeSecretStore)
	}{
		{"PUT env-vars", func(svc *Service, staged bool) error {
			_, err := svc.SetEnvVars(ctxFor(staged), "web", []EnvVarView{{Key: "A", Value: "two"}})
			return err
		}, func(t *testing.T, store *fakeSecretStore) {
			if store.m[envPath("web")]["A"] != "two" {
				t.Fatalf("env not stored: %v", store.m[envPath("web")])
			}
		}},
		{"PUT env-var", func(svc *Service, staged bool) error {
			_, err := svc.SetEnvVar(ctxFor(staged), "web", "B", EnvVarWrite{Value: "three"})
			return err
		}, func(t *testing.T, store *fakeSecretStore) {
			if store.m[envPath("web")]["B"] != "three" {
				t.Fatalf("env not stored: %v", store.m[envPath("web")])
			}
		}},
		{"DELETE env-var", func(svc *Service, staged bool) error {
			return svc.DeleteEnvVar(ctxFor(staged), "web", "A")
		}, func(t *testing.T, store *fakeSecretStore) {
			if _, ok := store.m[envPath("web")]["A"]; ok {
				t.Fatalf("env not deleted: %v", store.m[envPath("web")])
			}
		}},
		{"PUT secret-files", func(svc *Service, staged bool) error {
			_, err := svc.SetSecretFiles(ctxFor(staged), "web", []SecretFileView{{Name: "f", Content: "x"}})
			return err
		}, func(t *testing.T, store *fakeSecretStore) {
			if store.m[filesPath("web")]["f"] != "x" {
				t.Fatalf("files not stored: %v", store.m[filesPath("web")])
			}
		}},
		{"PUT secret-file", func(svc *Service, staged bool) error {
			_, err := svc.SetSecretFile(ctxFor(staged), "web", "g", "y")
			return err
		}, func(t *testing.T, store *fakeSecretStore) {
			if store.m[filesPath("web")]["g"] != "y" {
				t.Fatalf("file not stored: %v", store.m[filesPath("web")])
			}
		}},
		{"DELETE secret-file", func(svc *Service, staged bool) error {
			return svc.DeleteSecretFile(ctxFor(staged), "web", "g")
		}, func(t *testing.T, store *fakeSecretStore) {
			if _, ok := store.m[filesPath("web")]["g"]; ok {
				t.Fatalf("file not deleted: %v", store.m[filesPath("web")])
			}
		}},
	}
	for _, staged := range []bool{true, false} {
		app := managedApp("web")
		app.Generation = 7
		app.Spec.EnvFromSecret = "web-env"
		app.Spec.FilesFromSecrets = []string{"web-files"}
		app.Spec.RestartedAt = running
		app.Status = appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-7"}
		store := newFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"A": "one"}
		store.m[filesPath("web")] = map[string]string{}
		svc := newService(store, app)
		svc.Rollout = &rollout.Tracker{Store: &recordingDeploys{}}
		for _, step := range steps {
			t.Run(step.name, func(t *testing.T) {
				if err := step.write(svc, staged); err != nil {
					t.Fatal(err)
				}
				step.check(t, store)
				after := getApp(t, svc.Client, "web")
				rolled := after.Spec.RestartedAt != running
				if staged && rolled {
					t.Fatalf("a staged %s rolled the service (restartedAt %q)", step.name, after.Spec.RestartedAt)
				}
				if !staged && !rolled {
					t.Fatalf("an unstaged %s must keep its immediate rollout", step.name)
				}
				if staged && after.Annotations[appv1alpha1.AnnotationSavedConfigRevision] == "" {
					t.Fatalf("a staged %s published no saved-configuration notification", step.name)
				}
			})
			if !staged {
				// Re-arm the marker so the next unstaged step proves its own roll.
				app := getApp(t, svc.Client, "web")
				app.Spec.RestartedAt = running
				if err := svc.Client.Update(auditCtx(), app); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func ctxFor(staged bool) context.Context {
	ctx := auditCtx()
	if staged {
		return WithStagedWrite(ctx)
	}
	return ctx
}

// w2/m254 t007: GraphQL reaches a build source's environment with its bds- id
// as serviceId, and every write is staged — saved without building the carrier,
// as REST's /v1/build-sources routes and Render do. A deploying save is
// refused rather than building it.
func TestBuildSourceEnvWritesOverGraphQLNeverBuild(t *testing.T) {
	const running = "running-release"
	id := ids.New(ids.BuildSource)
	carrier := managedApp(id)
	carrier.Labels[store.LabelAppID] = id
	carrier.Spec.Type = appv1alpha1.TypeBuildSource
	carrier.Spec.EnvFromSecret = id + "-env"
	carrier.Spec.RestartedAt = running
	carrier.Status = appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-1"}
	secretStore := newFakeSecretStore()
	secretStore.m[envPath(id)] = map[string]string{}
	svc := newService(secretStore, carrier)
	deploys := &recordingDeploys{}
	svc.Rollout = &rollout.Tracker{Store: deploys}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(q string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: auditCtx(), RequestString: q})
	}
	if res := do(`mutation { setEnvVar(serviceId:"` + id + `", key:"NODE_ENV", value:"production") }`); res.HasErrors() {
		t.Fatal(res.Errors)
	}
	if secretStore.m[envPath(id)]["NODE_ENV"] != "production" {
		t.Fatalf("env not stored: %v", secretStore.m[envPath(id)])
	}
	if got := getApp(t, svc.Client, id).Spec.RestartedAt; got != running {
		t.Fatalf("an env write rolled the build source (restartedAt %q)", got)
	}
	res := do(`mutation { patchServiceEnvironment(serviceId:"` + id + `", saveMode:"deploy", envVars:[{key:"A", value:"1"}]) { rolledOut } }`)
	if !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "without building") {
		t.Fatalf("deploying save on a build source = %+v, want refused", res.Errors)
	}
	if got := getApp(t, svc.Client, id).Spec.RestartedAt; got != running {
		t.Fatalf("a refused save rolled the build source (restartedAt %q)", got)
	}
}
