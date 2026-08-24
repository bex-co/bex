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

package envgroups

import (
	"context"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// optional_environmentid_test.go is w6/m48: environmentId is an optional
// foreign key (EnvGroupView.EnvironmentID is a bare Go string, "" meaning "no
// Environment"), and GraphQL must resolve that unset state to `null` — the
// same thing REST already does via `omitempty` — not to the raw Go zero value.
// Executed through the actual composed GraphQL schema (graphql.Do), not just a
// direct resolver call, because the defect lived in the nested-field
// resolution graphql-go performs for envGroupGQLType's "environmentId" entry;
// calling the top-level "envGroup" verb's Resolve alone never exercises it.

func envGroupSchema(t *testing.T, svc *Service) *graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return &schema
}

func TestGraphQL_EnvironmentIDNullsWhenUnset(t *testing.T) {
	svc := newService(newFakeStore())
	workspaceScoped, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: "workspace-scoped"})
	if err != nil {
		t.Fatalf("create workspace-scoped group: %v", err)
	}

	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelEnvironment: "env-alpha"}
	scoped := newService(newFakeStore(), web)
	scoped.EnvironmentWorkspace = func(_ context.Context, environmentID string) (string, error) {
		if environmentID != "env-alpha" {
			return "", core.ErrNotFound
		}
		return "", nil
	}
	environmentScoped, err := scoped.CreateEnvGroup(context.Background(),
		CreateEnvGroupRequest{Name: "environment-scoped", EnvironmentID: "env-alpha"})
	if err != nil {
		t.Fatalf("create environment-scoped group: %v", err)
	}

	t.Run("unset environmentId resolves to null, not empty string", func(t *testing.T) {
		schema := envGroupSchema(t, svc)
		res := graphql.Do(graphql.Params{Schema: *schema, Context: context.Background(),
			RequestString: `{ envGroup(id: "` + workspaceScoped.ID + `") { environmentId } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("query: %v", res.Errors)
		}
		got := res.Data.(map[string]any)["envGroup"].(map[string]any)["environmentId"]
		if got != nil {
			t.Errorf("environmentId = %#v, want nil — a bare Go zero-value string leaked over the "+
				"wire, disagreeing with REST's omitempty omission of the same field for the same group", got)
		}
	})

	t.Run("set environmentId still resolves to its real value", func(t *testing.T) {
		schema := envGroupSchema(t, scoped)
		res := graphql.Do(graphql.Params{Schema: *schema, Context: context.Background(),
			RequestString: `{ envGroup(id: "` + environmentScoped.ID + `") { environmentId } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("query: %v", res.Errors)
		}
		got := res.Data.(map[string]any)["envGroup"].(map[string]any)["environmentId"]
		if got != "env-alpha" {
			t.Errorf("environmentId = %#v, want \"env-alpha\" — the null-for-unset fix must not "+
				"null a field that legitimately has a value", got)
		}
	})
}
