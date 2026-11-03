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
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w2/065: Render's update-blueprint takes a branch (the branch must hold the
// Blueprint file; no sync is planned) and every Blueprint object carries its
// workspace as ownerId.
func TestUpdateBlueprintBranch(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	st := newFakeBlueprintStore()
	svc := newBlueprintService(st, fakeWorkspace{"user-a": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{contents: stackManifest}
	created := store.Blueprint{ID: "blp-00000000000000000001", TenantID: "tea-a", Name: "app",
		Repo: "https://github.com/a/app", Branch: "main", Path: "render.yaml", Status: store.BlueprintStatusInSync}
	st.blueprints[created.ID] = created
	if got, err := svc.GetBlueprintByID(ctx, created.ID, "tea-a"); err != nil || got.OwnerID != "tea-a" {
		t.Fatalf("read view = %+v err=%v, want ownerId tea-a", got, err)
	}
	syncsBefore := len(st.syncs)

	staging := "staging"
	view, err := svc.UpdateBlueprint(ctx, created.ID, "tea-a", UpdateBlueprintRequest{Branch: &staging})
	if err != nil || view.Branch != "staging" || view.OwnerID != "tea-a" {
		t.Fatalf("update = %+v err=%v, want branch staging owned by tea-a", view, err)
	}
	if len(st.syncs) != syncsBefore {
		t.Error("changing the branch planned or ran a sync")
	}

	blank := "  "
	if _, err := svc.UpdateBlueprint(ctx, created.ID, "tea-a", UpdateBlueprintRequest{Branch: &blank}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("blank branch: err = %v, want bad request", err)
	}

	svc.GitFetcher = fakeBlueprintFetcher{err: fmt.Errorf("github: file not found: %w", &github.APIError{Status: 404})}
	empty := "empty"
	_, err = svc.UpdateBlueprint(ctx, created.ID, "tea-a", UpdateBlueprintRequest{Branch: &empty})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != string(BlueprintFetchFileNotFound) {
		t.Fatalf("branch without the file: err = %v, want %s", err, BlueprintFetchFileNotFound)
	}
	if got, _ := svc.GetBlueprintByID(ctx, created.ID, "tea-a"); got.Branch != "staging" {
		t.Errorf("a refused branch change was written: branch %q", got.Branch)
	}
}
