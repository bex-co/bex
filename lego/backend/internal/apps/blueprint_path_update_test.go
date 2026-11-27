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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// repoFiles is a repository by branch, each holding a set of file paths, that
// answers a missing branch or file as GitHub does. reads counts every read.
type repoFiles struct {
	branches map[string][]string
	reads    *int
}

func (f repoFiles) ResolveBlueprintCommit(_ context.Context, _, _, branch string) (string, error) {
	*f.reads++
	if _, ok := f.branches[branch]; !ok {
		return "", github.ErrBranchNotFound
	}
	return branch, nil
}

func (f repoFiles) FetchBlueprintFileAtCommit(_ context.Context, _, _, commit, filePath string) (string, error) {
	*f.reads++
	for _, held := range f.branches[commit] {
		if held == filePath {
			return stackManifest, nil
		}
	}
	return "", &github.APIError{Status: http.StatusNotFound}
}

// w8/418, w3/m679: Render's update-blueprint refuses a path whose file is not
// on the branch (404 when the branch is unchanged, 400 when it changes too),
// so a typo never replaces a working Blueprint's source.
func TestUpdateBlueprintPathMustHoldTheFile(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	st := newFakeBlueprintStore()
	svc := newBlueprintService(st, fakeWorkspace{"user-a": "tea-a"})
	reads := 0
	svc.GitFetcher = repoFiles{reads: &reads, branches: map[string][]string{
		"main":    {"render.yaml", "infra/render.yaml"},
		"staging": {"render.yaml"},
	}}
	bp := store.Blueprint{ID: "blp-00000000000000000001", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/a/app", Branch: "main", Path: "render.yaml", Status: store.BlueprintStatusInSync}
	st.blueprints[bp.ID] = bp
	source := func() (string, string) {
		got := st.blueprints[bp.ID]
		return got.Branch, got.Path
	}
	syncs := len(st.syncs)

	_, err := svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Path: new("nope.yaml")})
	var c *core.CodedError
	if !errors.As(err, &c) || c.Code != string(BlueprintFetchFileNotFound) || !errors.Is(err, core.ErrNotFound) ||
		c.Error() != `Blueprint file "nope.yaml" not found on branch "main".` {
		t.Fatalf("a path with no file: err = %#v, want 404 %s naming the path and branch", err, BlueprintFetchFileNotFound)
	}
	if branch, path := source(); branch != "main" || path != "render.yaml" {
		t.Fatalf("a refused path was written: %s %s", branch, path)
	}

	if view, err := svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Path: new("infra/render.yaml")}); err != nil || view.Path != "infra/render.yaml" {
		t.Fatalf("a path that holds the file = %+v err=%v, want it saved", view, err)
	}

	// Moving to a branch without the file at the new path is the branch's
	// refusal, 400, and changes neither.
	_, err = svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Branch: new("staging"), Path: new("infra/render.yaml")})
	if !errors.As(err, &c) || c.Code != string(BlueprintFetchFileNotFound) || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a new branch without the file: err = %#v, want 400 %s", err, BlueprintFetchFileNotFound)
	}
	_, err = svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Branch: new("gone")})
	if !errors.As(err, &c) || c.Code != string(BlueprintFetchBranchNotFound) || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a branch that does not exist: err = %#v, want 400 %s", err, BlueprintFetchBranchNotFound)
	}
	if branch, path := source(); branch != "main" || path != "infra/render.yaml" {
		t.Fatalf("a refused branch change was written: %s %s", branch, path)
	}
	if view, err := svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Branch: new("staging"), Path: new("render.yaml")}); err != nil ||
		view.Branch != "staging" || view.Path != "render.yaml" {
		t.Fatalf("branch and path together = %+v err=%v, want both saved", view, err)
	}

	// The unchanged source is not read again: a rename, or a path the
	// Blueprint already has.
	reads = 0
	if _, err := svc.UpdateBlueprint(ctx, bp.ID, "tea-a", UpdateBlueprintRequest{Name: new("renamed"), Path: new("render.yaml"), Branch: new("staging")}); err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Errorf("an update that changes no source read the repository %d times", reads)
	}
	if len(st.syncs) != syncs {
		t.Error("an update planned or ran a sync")
	}
}

// REST and MCP refuse the same path, and neither saves it or plans.
func TestUpdateBlueprintPathRefusalOnRESTAndMCP(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "app", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	id := res.Blueprint.ID
	reads := 0
	svc.GitFetcher = repoFiles{reads: &reads, branches: map[string][]string{"main": {"render.yaml"}}}
	syncs := len(st.syncs)

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPatch, "/v1/blueprints/"+id, strings.NewReader(`{"path":"nope.yaml"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `Blueprint file \"nope.yaml\" not found on branch \"main\".`) {
		t.Fatalf("REST PATCH a path with no file = %d %s, want 404 naming the file", rec.Code, rec.Body)
	}

	_, _, err = svc.updateBlueprintTool(ctx, updateBlueprintArgs{BlueprintID: id, Path: new("nope.yaml")})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("MCP update_blueprint a path with no file: err = %v, want the same refusal", err)
	}
	if got := st.blueprints[ids.BlueprintStorageID(id)]; got.Path != "render.yaml" {
		t.Fatalf("a refused path was written: %q", got.Path)
	}
	if len(st.syncs) != syncs {
		t.Error("a refused update planned a sync")
	}
}
