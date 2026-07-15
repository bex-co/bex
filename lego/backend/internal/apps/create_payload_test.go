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
	}{{name: result.Databases[0].ID, object: &appv1alpha1.Database{}}, {name: "cache", object: &appv1alpha1.KeyValue{}}} {
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: item.name}, item.object); err != nil {
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

func TestSecretFilesThreadThroughGraphQLAndMCPCreate(t *testing.T) {
	t.Run("GraphQL", func(t *testing.T) {
		seeder := &recordingSecretFileSeeder{}
		svc, _ := newService(nil)
		svc.SecretFileSeeder = seeder
		schema, err := appsGQLSchema(svc)
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{
			Schema: schema,
			RequestString: `mutation {
  createService(name: "gql-web", image: "nginx:alpine", secretFiles: [{name: "token", content: "gql-secret"}]) { id }
}`,
			Context: context.Background(),
		})
		if len(result.Errors) > 0 {
			t.Fatal(result.Errors)
		}
		if seeder.service != "gql-web" || len(seeder.files) != 1 || seeder.files[0].Content != "gql-secret" {
			t.Fatalf("GraphQL seed = service %q files %#v", seeder.service, seeder.files)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		seeder := &recordingSecretFileSeeder{}
		svc, _ := newService(nil)
		svc.SecretFileSeeder = seeder
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("create_web_service", map[string]any{
			"name":         "mcp-web",
			"image":        "nginx:alpine",
			"runtime":      "image",
			"buildCommand": "",
			"startCommand": "",
			"secretFiles": []any{
				map[string]any{"name": "token", "content": "mcp-secret"},
			},
		})
		if seeder.service != "mcp-web" || len(seeder.files) != 1 || seeder.files[0].Content != "mcp-secret" {
			t.Fatalf("MCP seed = service %q files %#v", seeder.service, seeder.files)
		}
	})
}
