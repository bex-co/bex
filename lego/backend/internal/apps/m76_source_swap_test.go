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
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// m76_source_swap_test.go pins the "Update Source" transition matrix (ADR026 §8
// / w5/m76) through the GraphQL surface the dashboard drives — setRepo /
// setImage / setBranch all delegate to SetSourceAndRegistryCredential, so these
// guarantee the source-kind switch clears the other kind, autoDeploy is never
// touched, and no deploy row is opened. The pending-source generation marker
// tells the operator to retain the active artifact until the next deploy.

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
	st := &recordingStore{}
	app := managedRepoApp("web")
	app.Spec.Repo = "https://github.com/x/mono"
	app.Spec.Branch = "release"
	app.Spec.AutoDeploy = true
	svc, cl := newService(st, app)
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setImage(id: "web", image: "nginx:stable") { imagePath repo } }`)

	spec := getApp(t, cl, "web").Spec
	if spec.Image != "nginx:stable" || spec.Repo != "" {
		t.Fatalf("after setImage: image=%q repo=%q, want nginx:stable + empty repo", spec.Image, spec.Repo)
	}
	if !spec.AutoDeploy {
		t.Error("setImage must not touch autoDeploy")
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("setImage opened deploy rows: %+v", st.deployCalls)
	}
	got := getApp(t, cl, "web")
	if got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Fatal("setImage did not mark the source as pending for the next deploy")
	}
	if got.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "" {
		t.Fatal("setImage stamped a release generation and would deploy immediately")
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

// TestGraphQLSetRepoThenBranchIsRepoRepoThenBranchOnly: setRepo atomically
// carries the dialog's repo+branch pair, then setBranch can still change only
// the branch while preserving the repository.
func TestGraphQLSetRepoThenBranchOnly(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/one", "main"))
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setRepo(id: "web", repo: "https://github.com/x/two", branch: "release") { repo branch } }`)
	if spec := getApp(t, cl, "web").Spec; spec.Repo != "https://github.com/x/two" || spec.Branch != "release" {
		t.Fatalf("repo re-point: repo=%q branch=%q", spec.Repo, spec.Branch)
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

func TestGraphQLSetRepoNoOpDoesNotMarkSourcePending(t *testing.T) {
	app := repoApp("web", "https://github.com/x/one", "main")
	svc, cl := newService(nil, app)
	schema := sourceSwapSchema(t, svc)

	mustMutate(t, schema, `mutation { setRepo(id: "web", repo: "https://github.com/x/one", branch: "main") { repo branch } }`)
	got := getApp(t, cl, "web")
	if got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "" {
		t.Fatal("re-saving the current source marked a deploy pending")
	}
}

func TestMCPUpdateServiceSwitchesSourceWithoutDeploying(t *testing.T) {
	st := &recordingStore{}
	app := managedRepoApp("web")
	app.Spec.AutoDeploy = true
	svc, _ := newService(st, app)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("update_service", map[string]any{
		"serviceId": "web",
		"image":     "nginx:stable",
	})

	spec := getApp(t, svc.Client, "web").Spec
	if spec.Image != "nginx:stable" || spec.Repo != "" || !spec.AutoDeploy {
		t.Fatalf("MCP source swap = image %q repo %q autoDeploy %v", spec.Image, spec.Repo, spec.AutoDeploy)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("MCP source swap opened deploy rows: %+v", st.deployCalls)
	}
}

func TestSourceValidationFailurePreservesAppAndCredentials(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			app := managedRepoApp("web")
			app.Labels[core.LabelTenant] = "tea-service-owner"
			app.Spec.CloneSecret = "web-clone"
			st := &recordingStore{}
			svc, cl := newService(st, app)
			gh := &fakeCloneTokens{validateErr: errors.Join(core.ErrBadRequest, errors.New("repository is not accessible through this workspace's GitHub connections or as public Git"))}
			svc.GitHub = gh
			if err := svc.writeCloneSecret(context.Background(), "default", "web-clone", "web", "old-token"); err != nil {
				t.Fatal(err)
			}
			before := getApp(t, cl, "web")
			if surface == "REST" {
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{"repo":"https://github.com/stranger/private","branch":"release"}`)))
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "GitHub connections") {
					t.Fatalf("response %d: %s", rec.Code, rec.Body.String())
				}
			} else if surface == "GraphQL" {
				res := graphql.Do(graphql.Params{Schema: sourceSwapSchema(t, svc), Context: context.Background(), RequestString: `mutation {setRepo(id:"web",repo:"https://github.com/stranger/private",branch:"release"){repo}}`})
				if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "GitHub connections") {
					t.Fatalf("errors: %v", res.Errors)
				}
			} else {
				ctx := context.Background()
				server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
				svc.RegisterMCP(server)
				serverT, clientT := mcp.NewInMemoryTransports()
				ss, err := server.Connect(ctx, serverT, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = ss.Close() })
				cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = cs.Close() })
				res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{"serviceId": "web", "repo": "https://github.com/stranger/private", "branch": "release"}})
				if err != nil || !res.IsError {
					t.Fatalf("MCP rejection: result=%v err=%v", res, err)
				}
			}
			if gh.validateCalls != 1 || gh.lastWorkspace != "tea-service-owner" || gh.lastRepo != "https://github.com/stranger/private" {
				t.Fatalf("wrong validation scope: %+v", gh)
			}
			if !reflect.DeepEqual(before, getApp(t, cl, "web")) {
				t.Fatal("rejected source changed the App or pending marker")
			}
			if value, ok := cloneSecretValue(t, cl, "web-clone"); !ok || value != "old-token" {
				t.Fatal("rejected source changed active clone credentials")
			}
			if gh.calls != 0 || len(st.deployCalls) != 0 {
				t.Fatal("source validation minted deploy credentials or opened a deploy")
			}
		})
	}
}
