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
	"testing"

	"github.com/graphql-go/graphql"
)

// setbranch_test.go covers the narrow branch setter (w5/m48/t005, Render's
// editable Branch field). REST PATCH `branch` already flows through
// SetSourceAndRegistryCredential (covered by the source-update tests); these
// pin the GraphQL and MCP projections of the same verb so the three surfaces
// cannot drift.

func TestGraphQLSetBranch(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setBranch(id: "web", branch: "release") { branch repo } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setBranch"].(map[string]any)
	if got["branch"] != "release" {
		t.Errorf("branch = %v, want release", got["branch"])
	}
	if got["repo"] != "https://github.com/x/mono" {
		t.Errorf("repo changed by a branch-only edit: %+v", got)
	}
	if spec := getApp(t, cl, "web").Spec; spec.Branch != "release" || spec.Repo != "https://github.com/x/mono" {
		t.Errorf("spec repo/branch = %q/%q, want unchanged repo + release", spec.Repo, spec.Branch)
	}
}

func TestGraphQLSetBranchRejectsInvalidRef(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setBranch(id: "web", branch: "bad branch") { branch } }`})
	if len(res.Errors) == 0 {
		t.Fatalf("setBranch with an invalid ref succeeded; want an error")
	}
	if got := getApp(t, cl, "web").Spec.Branch; got != "main" {
		t.Errorf("spec.branch = %q after a rejected edit, want main", got)
	}
}

func TestMCPSetBranch(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	// The set_branch tool delegates to the shared source verb.
	branch := "release"
	v, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", nil, nil, &branch, nil)
	if err != nil {
		t.Fatalf("SetSourceAndRegistryCredential: %v", err)
	}
	if out := toRenderService(v); out.Branch != "release" {
		t.Errorf("set_branch (via renderService projection) branch = %q, want release", out.Branch)
	}
	if got := getApp(t, cl, "web").Spec.Branch; got != "release" {
		t.Errorf("spec.branch = %q, want release", got)
	}

	// An explicit empty branch restores the default — the setter family's
	// "empty clears to the default" convention (cf. set_build_command).
	empty := ""
	if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", nil, nil, &empty, nil); err != nil {
		t.Fatalf("clear branch: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Branch; got != "main" {
		t.Errorf("spec.branch after clear = %q, want main (default)", got)
	}
}
