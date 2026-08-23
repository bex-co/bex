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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type blueprintGroupingTestStore struct {
	*recordingStore
	projects     []store.Project
	environments []store.Environment
	aclWrites    int
}

func (s *blueprintGroupingTestStore) ListProjects(_ context.Context, tenantID string) ([]store.Project, error) {
	var out []store.Project
	for _, project := range s.projects {
		if project.TenantID == tenantID {
			out = append(out, project)
		}
	}
	return out, nil
}

func (s *blueprintGroupingTestStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	project := store.Project{ID: ids.New(ids.Project), TenantID: tenantID, Name: name}
	s.projects = append(s.projects, project)
	return project, nil
}

func (s *blueprintGroupingTestStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	var out []store.Environment
	for _, environment := range s.environments {
		if environment.ProjectID == projectID {
			out = append(out, environment)
		}
	}
	return out, nil
}

func (s *blueprintGroupingTestStore) CreateEnvironment(_ context.Context, projectID, tenantID, name string) (store.Environment, error) {
	environment := store.Environment{ID: ids.New(ids.Environment), ProjectID: projectID, TenantID: tenantID, Name: name}
	s.environments = append(s.environments, environment)
	return environment, nil
}

func (s *blueprintGroupingTestStore) SetEnvironmentACL(_ context.Context, environmentID, protectedStatus string, isolated bool, ipAllowList []core.IPAllowListEntry) error {
	s.aclWrites++
	for i := range s.environments {
		if s.environments[i].ID == environmentID {
			s.environments[i].ProtectedStatus = protectedStatus
			s.environments[i].NetworkIsolationEnabled = isolated
			s.environments[i].IPAllowList = ipAllowList
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *blueprintGroupingTestStore) CountWorkspaceGroupings(_ context.Context, tenantID string) (int, int, error) {
	var projects, environments int
	for _, project := range s.projects {
		if project.TenantID == tenantID {
			projects++
		}
	}
	for _, environment := range s.environments {
		if environment.TenantID == tenantID {
			environments++
		}
	}
	return projects, environments, nil
}

func (*blueprintGroupingTestStore) SetAppEnvironment(context.Context, string, string, string) error {
	return nil
}

func (s *blueprintGroupingTestStore) ResolveForCreate(_ context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	for _, environment := range s.environments {
		if environment.ID != environmentID {
			continue
		}
		if environment.TenantID != workspaceID {
			return core.EnvironmentAssignment{}, core.ErrForbidden
		}
		return core.EnvironmentAssignment{
			ID:                      environment.ID,
			ProjectID:               environment.ProjectID,
			WorkspaceID:             environment.TenantID,
			NetworkIsolationEnabled: environment.NetworkIsolationEnabled,
		}, nil
	}
	return core.EnvironmentAssignment{}, core.ErrNotFound
}

func TestDeployStackUsesCanonicalBlueprintEnvironmentNesting(t *testing.T) {
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	manifest := `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        networking:
          isolation: enabled
        permissions:
          protection: enabled
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
          - type: keyvalue
            name: cache
            plan: starter
            ipAllowList: []
        databases:
          - name: postgres
            plan: basic-256mb
`

	result, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Services) != 1 || len(result.Databases) != 1 || len(result.KeyValues) != 1 {
		t.Fatalf("stack result = %+v", result)
	}
	if len(st.projects) != 1 || len(st.environments) != 1 {
		t.Fatalf("group rows = projects %#v environments %#v", st.projects, st.environments)
	}
	environment := st.environments[0]
	if environment.ProtectedStatus != "protected" || !environment.NetworkIsolationEnabled {
		t.Fatalf("environment controls = %+v", environment)
	}
	if len(st.appCreates) != 1 || st.appCreates[0].ProjectID != environment.ProjectID || st.appCreates[0].EnvironmentID != environment.ID {
		t.Fatalf("service store association = %#v", st.appCreates)
	}
	app := getTenantApp(t, cl, "tea-a", "web")
	if app.Labels[core.LabelProject] != environment.ProjectID || app.Labels[core.LabelEnvironment] != environment.ID || app.Labels[core.LabelNetworkIsolation] != environment.ID {
		t.Fatalf("service labels = %#v", app.Labels)
	}
	for _, item := range []struct {
		name   string
		object client.Object
	}{{name: result.Databases[0].ID, object: &appv1alpha1.Database{}}, {name: result.KeyValues[0].ID, object: &appv1alpha1.KeyValue{}}} {
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: item.name}, item.object); err != nil {
			t.Fatal(err)
		}
		if item.object.GetLabels()[core.LabelProject] != environment.ProjectID || item.object.GetLabels()[core.LabelEnvironment] != environment.ID {
			t.Fatalf("%T labels = %#v", item.object, item.object.GetLabels())
		}
	}
}

func TestBlueprintRejectsCreateBodyOnlyFieldsByName(t *testing.T) {
	for name, manifest := range map[string]string{
		"secretFiles": `services:
  - type: web
    name: web
    runtime: image
    image: {url: nginx:alpine}
    secretFiles: [{name: token, content: never-print-this}]
`,
		"environmentId": `services:
  - type: web
    name: web
    runtime: image
    image: {url: nginx:alpine}
    environmentId: env-staging
`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseStack(DeployRequest{Manifest: manifest})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), name) {
				t.Fatalf("parse error = %v, want named bad request", err)
			}
			if strings.Contains(err.Error(), "never-print-this") {
				t.Fatalf("secret content leaked in error: %v", err)
			}
		})
	}
}

// Everything a service is born with that does NOT live on its App spec — the
// official CLI's `secretFiles` and (since w6/m45 t002) the create payload's
// literal `envVars` — must reach the create-time seeder on ALL THREE public
// surfaces. Creation-time env vars used to be baked onto spec.Env and nowhere
// else, so `envVars`/the Environment tab never saw them; a copy left behind on
// spec.Env would also shadow the projected Secret in the container (Kubernetes
// env beats envFrom), silently defeating every later edit and delete.
func TestCreateTimeSecretsThreadThroughEverySurface(t *testing.T) {
	const (
		fileContent = "top-secret"
		envValue    = "marker-v1"
	)
	assertSeeded := func(t *testing.T, seeder *recordingCreateSecretsSeeder, name string) {
		t.Helper()
		if seeder.service != name {
			t.Fatalf("seeded service = %q, want %q", seeder.service, name)
		}
		if len(seeder.files) != 1 || seeder.files[0].Name != "token" || seeder.files[0].Content != fileContent {
			t.Fatalf("seeded files = %#v", seeder.files)
		}
		if len(seeder.env) != 1 || seeder.env["MESSAGE"] != envValue {
			t.Fatalf("seeded env = %#v", seeder.env)
		}
	}

	t.Run("REST", func(t *testing.T) {
		seeder := &recordingCreateSecretsSeeder{}
		svc, cl := newService(nil)
		svc.CreateSecrets = seeder
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		body := `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine"},` +
			`"secretFiles":[{"name":"token","content":"` + fileContent + `"}],` +
			`"envVars":[{"key":"MESSAGE","value":"` + envValue + `"}]}`
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
		}
		assertSeeded(t, seeder, "web")
		if got := getApp(t, cl, "web"); len(got.Spec.Env) != 0 || got.Spec.EnvFromSecret != "web-env" {
			t.Fatalf("literal left on spec.Env (shadows the store): env=%#v envFromSecret=%q", got.Spec.Env, got.Spec.EnvFromSecret)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		seeder := &recordingCreateSecretsSeeder{}
		svc, _ := newService(nil)
		svc.CreateSecrets = seeder
		schema, err := appsGQLSchema(svc)
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{
			Schema: schema,
			RequestString: `mutation {
  createService(name: "gql-web", image: "nginx:alpine",
    secretFiles: [{name: "token", content: "` + fileContent + `"}],
    envVars: [{key: "MESSAGE", value: "` + envValue + `"}]) { id }
}`,
			Context: context.Background(),
		})
		if len(result.Errors) > 0 {
			t.Fatal(result.Errors)
		}
		assertSeeded(t, seeder, "gql-web")
	})

	t.Run("MCP", func(t *testing.T) {
		seeder := &recordingCreateSecretsSeeder{}
		svc, _ := newService(nil)
		svc.CreateSecrets = seeder
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("create_web_service", map[string]any{
			"name":         "mcp-web",
			"image":        "nginx:alpine",
			"runtime":      "image",
			"buildCommand": "",
			"startCommand": "",
			"secretFiles":  []any{map[string]any{"name": "token", "content": fileContent}},
			"envVars":      []any{map[string]any{"key": "MESSAGE", "value": envValue}},
		})
		assertSeeded(t, seeder, "mcp-web")
	})
}

// Only what the env store can represent moves: a ValueFrom entry is a Secret
// key reference (the shape a bex.yml fromDatabase reference resolves to) and a
// name outside core.ValidEnvKey would fail the projection Secret's write — both
// keep their spec-only behavior rather than newly failing a working create.
func TestCreationTimeEnvVarsLeaveReferencesAndUnprojectableNamesOnTheSpec(t *testing.T) {
	seeder := &recordingCreateSecretsSeeder{}
	svc, cl := newService(nil)
	svc.CreateSecrets = seeder
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Image: "nginx:alpine",
		Env: []appv1alpha1.EnvVar{
			{Name: "MESSAGE", Value: "marker"},
			{Name: "bad-key", Value: "kept"},
			{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{
				SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "db-app", Key: "uri"},
			}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if len(seeder.env) != 1 || seeder.env["MESSAGE"] != "marker" {
		t.Fatalf("seeded env = %#v, want only the projectable literal", seeder.env)
	}
	got := getApp(t, cl, "web")
	if len(got.Spec.Env) != 2 || got.Spec.Env[0].Name != "bad-key" || got.Spec.Env[1].Name != "DATABASE_URL" {
		t.Fatalf("spec.Env = %#v, want the unprojectable name and the reference kept in order", got.Spec.Env)
	}
}

// With no secrets store wired (OpenBao off), a create keeps its literals on
// spec.Env exactly as before — the fix must not turn a working dev/hand-applied
// create into an ErrSecretsUnavailable failure.
func TestCreationTimeEnvVarsStayOnSpecWithoutASecretsStore(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Image: "nginx:alpine",
		Env: []appv1alpha1.EnvVar{{Name: "MESSAGE", Value: "marker"}},
	}); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, "web")
	if len(got.Spec.Env) != 1 || got.Spec.Env[0].Value != "marker" || got.Spec.EnvFromSecret != "" {
		t.Fatalf("store-less create = env %#v envFromSecret %q", got.Spec.Env, got.Spec.EnvFromSecret)
	}
}
