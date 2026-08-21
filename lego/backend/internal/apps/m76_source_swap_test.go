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

// m76_source_swap_test.go pins the "Update Source" transition matrix (ADR026 §8
// / w5/m76) through the GraphQL surface the dashboard drives — setRepo /
// setImage / setBranch all delegate to SetSourceAndRegistryCredential, so these
// guarantee the source-kind switch clears the other kind, autoDeploy is never
// touched, and no deploy is triggered (the store harness has no deploy plane —
// the source change is a pure spec patch, Render's "changes are not deployed
// automatically" semantics).

func sourceSwapSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func mustMutate(t *testing.T, schema graphql.Schema, query string) {
	t.Helper()
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("gql %q: %v", query, res.Errors)
	}
}

// TestGraphQLSetImageSwitchesRepoToImage: setImage on a repo-backed service
// clears the repo/branch source and leaves autoDeploy untouched.
func TestGraphQLSetImageSwitchesRepoToImage(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "release"))
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setImage(id: "web", image: "nginx:stable") { imagePath repo } }`)

	spec := getApp(t, cl, "web").Spec
	if spec.Image != "nginx:stable" || spec.Repo != "" {
		t.Fatalf("after setImage: image=%q repo=%q, want nginx:stable + empty repo", spec.Image, spec.Repo)
	}
	if !spec.AutoDeploy {
		t.Error("setImage must not touch autoDeploy")
	}
}

// TestGraphQLSetRepoSwitchesImageToRepo: setRepo on an image-backed service
// clears the image and adopts the repo (default branch applied).
func TestGraphQLSetRepoSwitchesImageToRepo(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = ""
	app.Spec.Image = "nginx:old"
	svc, cl := newService(nil, app)
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setRepo(id: "web", repo: "https://github.com/acme/api") { repo imagePath } }`)

	spec := getApp(t, cl, "web").Spec
	if spec.Repo != "https://github.com/acme/api" || spec.Image != "" {
		t.Fatalf("after setRepo: repo=%q image=%q, want the repo + empty image", spec.Repo, spec.Image)
	}
}

// TestGraphQLSetImageRejectsInvalidRef: a shell-metacharacter image is refused
// and the source is left unchanged (ValidImage, mirroring setBranch's guard).
func TestGraphQLSetImageRejectsInvalidRef(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	schema := sourceSwapSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setImage(id: "web", image: "nginx; rm -rf /") { imagePath } }`})
	if len(res.Errors) == 0 {
		t.Fatal("setImage with an invalid ref succeeded; want an error")
	}
	if spec := getApp(t, cl, "web").Spec; spec.Image != "" || spec.Repo != "https://github.com/x/mono" {
		t.Errorf("rejected setImage still mutated the source: image=%q repo=%q", spec.Image, spec.Repo)
	}
}

// TestGraphQLSetRepoThenBranchIsRepoRepoThenBranchOnly: two repo-backed edits in
// a row — re-point the repo (default branch), then change only the branch —
// each preserving the other field, with autoDeploy intact throughout.
func TestGraphQLSetRepoThenBranchOnly(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/one", "main"))
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setRepo(id: "web", repo: "https://github.com/x/two") { repo } }`)
	if spec := getApp(t, cl, "web").Spec; spec.Repo != "https://github.com/x/two" {
		t.Fatalf("repo re-point: repo=%q", spec.Repo)
	}

	mustMutate(t, schema, `mutation { setBranch(id: "web", branch: "develop") { branch } }`)
	spec := getApp(t, cl, "web").Spec
	if spec.Repo != "https://github.com/x/two" || spec.Branch != "develop" {
		t.Fatalf("branch-only edit: repo=%q branch=%q, want two + develop", spec.Repo, spec.Branch)
	}
	if !spec.AutoDeploy {
		t.Error("autoDeploy dropped across the source edits")
	}
}
