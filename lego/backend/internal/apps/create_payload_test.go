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

// denyRelationChecker allows every relation except one — the role-ladder fake
// for a caller who holds can_create but not the admin can_manage.
type denyRelationChecker struct{ deny string }

func (c denyRelationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != c.deny, nil
}

func seededProtectedEnvironmentStore() (*blueprintGroupingTestStore, store.Environment) {
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	project := store.Project{ID: ids.New(ids.Project), TenantID: "tea-a", Name: "platform"}
	environment := store.Environment{
		ID:                      ids.New(ids.Environment),
		ProjectID:               project.ID,
		TenantID:                "tea-a",
		Name:                    "production",
		ProtectedStatus:         core.ProtectedStatusProtected,
		NetworkIsolationEnabled: true,
	}
	st.projects = []store.Project{project}
	st.environments = []store.Environment{environment}
	return st, environment
}

// round-21 finding 1: a Blueprint that names an existing protected + isolated
// environment but OMITS the permissions/networking blocks must PRESERVE the
// current controls, never silently downgrade them. A downgrade clears the App's
// network-isolation label, which makes the operator delete its NetworkPolicy.
func TestBlueprintSyncPreservesOmittedEnvironmentControls(t *testing.T) {
	st, environment := seededProtectedEnvironmentStore()
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	manifest := `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	got := st.environments[0]
	if got.ProtectedStatus != core.ProtectedStatusProtected || !got.NetworkIsolationEnabled {
		t.Fatalf("omitted blocks downgraded the existing environment: %+v", got)
	}
	if st.aclWrites != 0 {
		t.Fatalf("preserving unchanged controls must not write the ACL, got %d writes", st.aclWrites)
	}
	app := getTenantApp(t, cl, "tea-a", "web")
	if app.Labels[core.LabelNetworkIsolation] != environment.ID {
		t.Fatalf("network-isolation label not preserved: %#v", app.Labels)
	}
}

// round-21 finding 1: changing an EXISTING environment's admin-classified
// protected-environment ACL through the declarative apply must clear the same
// can_manage gate environments.Service.SetACL enforces — a developer holding
// only can_create must not downgrade it, and the store row must be untouched.
func TestBlueprintSyncRequiresCanManageToChangeExistingEnvironmentACL(t *testing.T) {
	st, _ := seededProtectedEnvironmentStore()
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	svc.Authz = denyRelationChecker{deny: core.RelCanManage}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	manifest := `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        permissions:
          protection: disabled
        networking:
          isolation: disabled
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
`
	_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden downgrading an existing environment ACL without can_manage, got %v", err)
	}
	if got := st.environments[0]; got.ProtectedStatus != core.ProtectedStatusProtected || !got.NetworkIsolationEnabled {
		t.Fatalf("ACL was changed despite denied can_manage: %+v", got)
	}
	if st.aclWrites != 0 {
		t.Fatalf("denied ACL change must not reach the store, got %d writes", st.aclWrites)
	}
}

// workspaceRecordingEnvReader records the workspace bound on the context each
// nested env-var read receives.
type workspaceRecordingEnvReader struct{ gotWorkspace string }

func (r *workspaceRecordingEnvReader) EnvVarKeys(ctx context.Context, _ string) ([]core.EnvVar, error) {
	r.gotWorkspace = core.NamedWorkspace(ctx)
	return nil, nil
}

func (r *workspaceRecordingEnvReader) EnvVarValue(ctx context.Context, _, _ string) (core.EnvVar, error) {
	r.gotWorkspace = core.NamedWorkspace(ctx)
	return core.EnvVar{}, nil
}

// round-21 finding 6: the nested GraphQL secret resolvers re-resolve the parent
// service by its workspace-scoped public name. The GraphQL execution context
// carries no workspace, so without rebinding, a same-named service in the
// caller's DEFAULT workspace would win over the workspace the parent field
// selected. The resolver must bind the parent AppView's own OwnerID onto the
// context so the name lookup resolves inside the right workspace.
func TestNestedSecretResolversBindParentWorkspace(t *testing.T) {
	reader := &workspaceRecordingEnvReader{}
	// A GraphQL execution context: env-var reader wired, but NO workspace named.
	ctx := core.WithEnvVars(context.Background(), reader)

	if _, err := envVarKeysResolve(graphql.ResolveParams{
		Source:  AppView{Name: "web", OwnerID: "tea-b"},
		Context: ctx,
	}); err != nil {
		t.Fatal(err)
	}
	if reader.gotWorkspace != "tea-b" {
		t.Fatalf("envVarKeys bound workspace = %q, want tea-b (the parent's OwnerID)", reader.gotWorkspace)
	}

	reader.gotWorkspace = ""
	if _, err := envVarValueResolve(graphql.ResolveParams{
		Source:  AppView{Name: "web", OwnerID: "tea-b"},
		Context: ctx,
		Args:    map[string]any{"key": "SECRET"},
	}); err != nil {
		t.Fatal(err)
	}
	if reader.gotWorkspace != "tea-b" {
		t.Fatalf("envVarValue bound workspace = %q, want tea-b", reader.gotWorkspace)
	}

	// A hand-applied App (no OwnerID) leaves the context unchanged — matching the
	// bare-name resolution those Apps already use.
	reader.gotWorkspace = "sentinel"
	if _, err := envVarKeysResolve(graphql.ResolveParams{
		Source:  AppView{Name: "web"},
		Context: ctx,
	}); err != nil {
		t.Fatal(err)
	}
	if reader.gotWorkspace != "" {
		t.Fatalf("unlabeled parent must not bind a workspace, got %q", reader.gotWorkspace)
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
