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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// staleAllowChecker models the round-15 #4 window: cached Check still allows
// while CheckFresh already says the membership is gone.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

type countingFetcher struct {
	fakeBlueprintFetcher
	n int
}

func (f *countingFetcher) ResolveBlueprintCommit(ctx context.Context, tenantID, repo, branch string) (string, error) {
	f.n++
	return f.fakeBlueprintFetcher.ResolveBlueprintCommit(ctx, tenantID, repo, branch)
}

func (f *countingFetcher) FetchBlueprintFileAtCommit(ctx context.Context, tenantID, repo, commitSHA, filePath string) (string, error) {
	f.n++
	return f.fakeBlueprintFetcher.FetchBlueprintFileAtCommit(ctx, tenantID, repo, commitSHA, filePath)
}

// PreviewBlueprint must not fetch private repo contents on a stale positive.
func TestPreviewBlueprintFailsClosedOnFreshRevocation(t *testing.T) {
	fetcher := &countingFetcher{fakeBlueprintFetcher: fakeBlueprintFetcher{contents: stackManifest, sha: "abc1234"}}
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Authz: staleAllowChecker{}},
		GitFetcher: fetcher,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "developer", Method: "session"})
	if _, err := svc.PreviewBlueprint(ctx, "", "https://github.com/a/app", "main", ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("stale-positive preview = %v, want ErrForbidden", err)
	}
	if fetcher.n != 0 {
		t.Fatalf("GitFetcher called %d times after fresh deny; want 0", fetcher.n)
	}
}

// Get/list keep metadata for a viewer whose cached Check still allows, but
// blank Manifest so the stored private contents do not ride PositiveTTL.
func TestBlueprintManifestBlankedOnFreshRevocation(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	svc := &Service{
		Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws, Authz: staleAllowChecker{}},
		Blueprints: newFakeBlueprintStore(store.Blueprint{
			ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
			Branch: "main", Manifest: stackManifest, Status: "active", Name: "app",
		}),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if v, err := svc.GetBlueprintByID(ctx, "blp-1", "tea-a"); err != nil || v.Manifest != "" || v.Name != "app" {
		t.Fatalf("stale-positive get = manifest %q name %q (%v), want blank manifest with metadata intact", v.Manifest, v.Name, err)
	}
	if vs, err := svc.ListBlueprints(ctx, "tea-a"); err != nil || len(vs) != 1 || vs[0].Manifest != "" {
		t.Fatalf("stale-positive list = %+v (%v), want one view with a blank manifest", vs, err)
	}
}
