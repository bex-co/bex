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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// advancingBlueprintFetcher starts Resolve at headSHA while Fetch serves
// byCommit — models a branch move between preview and confirm (w8/m41).
type advancingBlueprintFetcher struct {
	headSHA  string
	byCommit map[string]string
	resolves int
	fetched  []string
}

func (f *advancingBlueprintFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	f.resolves++
	return f.headSHA, nil
}

func (f *advancingBlueprintFetcher) FetchBlueprintFileAtCommit(_ context.Context, _, _, commit, _ string) (string, error) {
	f.fetched = append(f.fetched, commit)
	contents, ok := f.byCommit[commit]
	if !ok {
		return "", fmt.Errorf("bad request: unknown commit %s", commit)
	}
	return contents, nil
}

const (
	reviewedCommitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	movedCommitB    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	manifestAtA     = "services:\n  - {name: whoami-a, type: web, runtime: image, image: {url: whoami:a}}\n"
	manifestAtB     = "services:\n  - {name: whoami-b, type: web, runtime: image, image: {url: whoami:b}}\n"
)

func TestSyncBlueprintReviewedSourcePinsCommitAcrossBranchMove(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: manifestAtA,
		Status: "active", Name: "app",
	})
	fetcher := &advancingBlueprintFetcher{
		headSHA: movedCommitB,
		byCommit: map[string]string{
			reviewedCommitA: manifestAtA,
			movedCommitB:    manifestAtB,
		},
	}
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
		GitFetcher:      fetcher,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	reviewed := &ReviewedBlueprintSource{
		Repo: "https://github.com/a/app", Path: CanonicalBlueprintFilename, CommitID: reviewedCommitA,
	}

	res, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "", reviewed)
	if err != nil {
		t.Fatalf("pinned sync: %v", err)
	}
	if fetcher.resolves != 0 {
		t.Fatalf("pinned sync resolved HEAD %d times, want 0", fetcher.resolves)
	}
	if len(fetcher.fetched) != 1 || fetcher.fetched[0] != reviewedCommitA {
		t.Fatalf("fetched = %v, want [%s]", fetcher.fetched, reviewedCommitA)
	}
	if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != reviewedCommitA {
		t.Fatalf("history commit = %+v, want %s", fs.insertedSyncs, reviewedCommitA)
	}
	for _, svcView := range res.Stack.Services {
		if svcView.Name == "whoami-b" {
			t.Fatalf("applied unreviewed HEAD service %q", svcView.Name)
		}
	}
}

func TestSyncBlueprintOmitReviewedFollowsMovedHead(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: manifestAtA,
		Status: "active", Name: "app",
	})
	fetcher := &advancingBlueprintFetcher{
		headSHA: movedCommitB,
		byCommit: map[string]string{
			reviewedCommitA: manifestAtA,
			movedCommitB:    manifestAtB,
		},
	}
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
		GitFetcher:      fetcher,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "", nil); err != nil {
		t.Fatalf("unpinned sync: %v", err)
	}
	if fetcher.resolves != 1 {
		t.Fatalf("unpinned resolves = %d, want 1", fetcher.resolves)
	}
	if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != movedCommitB {
		t.Fatalf("history commit = %+v, want HEAD %s", fs.insertedSyncs, movedCommitB)
	}
}

func TestSyncBlueprintReviewedPathMismatchConflicts(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: "deploy/render.yaml", Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	fetcher := &advancingBlueprintFetcher{
		headSHA: movedCommitB,
		byCommit: map[string]string{
			reviewedCommitA: stackManifest,
		},
	}
	svc := &Service{
		Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints: fs, DomainOwnership: allowDomainOwnership{}, GitFetcher: fetcher,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	reviewed := &ReviewedBlueprintSource{
		Repo: "https://github.com/a/app", Path: CanonicalBlueprintFilename, CommitID: reviewedCommitA,
	}

	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "", reviewed)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_SOURCE_CHANGED" {
		t.Fatalf("path mismatch = %v, want BLUEPRINT_SOURCE_CHANGED", err)
	}
	if len(fetcher.fetched) != 0 {
		t.Fatalf("must not fetch after path mismatch; fetched %v", fetcher.fetched)
	}
}

func TestSyncBlueprintReviewedWithBexYAMLIsBadRequest(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	svc := &Service{
		Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints: fs, DomainOwnership: allowDomainOwnership{},
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	reviewed := &ReviewedBlueprintSource{
		Path: CanonicalBlueprintFilename, CommitID: reviewedCommitA,
	}
	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", stackManifest, "", reviewed); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bexYaml+reviewed = %v, want ErrBadRequest", err)
	}
}

func TestReviewedSourceAcrossAdapters(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	newFixture := func() (*Service, *fakeBlueprintStore) {
		fs := newFakeBlueprintStore(store.Blueprint{
			ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
			Branch: "main", Path: CanonicalBlueprintFilename, Manifest: manifestAtA,
			Status: "active", Name: "app",
		})
		svc := &Service{
			Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
			Blueprints:      fs,
			DomainOwnership: allowDomainOwnership{},
			GitFetcher: &advancingBlueprintFetcher{
				headSHA: movedCommitB,
				byCommit: map[string]string{
					reviewedCommitA: manifestAtA,
					movedCommitB:    manifestAtB,
				},
			},
		}
		return svc, fs
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	t.Run("REST", func(t *testing.T) {
		svc, fs := newFixture()
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		body := fmt.Sprintf(`{"ownerId":"tea-a","commitId":%q,"path":%q,"repo":%q}`,
			reviewedCommitA, CanonicalBlueprintFilename, "https://github.com/a/app")
		req := httptest.NewRequest(http.MethodPost, "/v1/blueprints/blp-1/sync", strings.NewReader(body))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("REST status = %d body %s", rec.Code, rec.Body.String())
		}
		if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != reviewedCommitA {
			t.Fatalf("REST history = %+v", fs.insertedSyncs)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, fs := newFixture()
		schema := blueprintSchema(t, svc)
		q := fmt.Sprintf(`mutation { syncBlueprint(id: "blp-1", ownerId: "tea-a", commitId: %q, path: %q, repo: %q) { blueprint { id } } }`,
			reviewedCommitA, CanonicalBlueprintFilename, "https://github.com/a/app")
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL: %v", res.Errors)
		}
		if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != reviewedCommitA {
			t.Fatalf("GraphQL history = %+v", fs.insertedSyncs)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, fs := newFixture()
		mcpCtx := core.WithWorkspace(ctx, "tea-a")
		srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		serverT, clientT := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(mcpCtx, serverT, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(mcpCtx, clientT, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { _ = cs.Close() })
		res, err := cs.CallTool(mcpCtx, &mcp.CallToolParams{
			Name: "sync_blueprint",
			Arguments: map[string]any{
				"id":       "blp-1",
				"commitId": reviewedCommitA,
				"path":     CanonicalBlueprintFilename,
				"repo":     "https://github.com/a/app",
			},
		})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		if res.IsError {
			t.Fatalf("MCP tool error: %+v", res)
		}
		if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != reviewedCommitA {
			t.Fatalf("MCP history = %+v", fs.insertedSyncs)
		}
	})
}
