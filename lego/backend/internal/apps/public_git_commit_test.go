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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type publicRepoCommitResolver struct {
	commit               store.CommitInfo
	workspace, repo, ref string
	calls                int
}

func (r *publicRepoCommitResolver) ResolveCommit(_ context.Context, workspace, repo, ref string) (store.CommitInfo, bool, error) {
	r.workspace, r.repo, r.ref = workspace, repo, ref
	r.calls++
	return r.commit, true, nil
}

// The anonymous smart-HTTP resolver supplies a SHA without GitHub's message or
// author metadata. The first build must use that same SHA, regardless of host.
func TestCreatePublicNonGitHubRepoPinsTheResolvedFirstDeploy(t *testing.T) {
	const tip = "87e8c32b8918a836c09475e0f668ea7c436d3758"
	for _, tc := range []struct{ name, repo, branch string }{
		{"GitLab main", "https://gitlab.com/pages/plain-html", "main"},
		{"Codeberg explicit branch", "https://codeberg.org/acme/site", "release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
			resolver := &publicRepoCommitResolver{commit: store.CommitInfo{Hash: tip}}
			svc.Commits = resolver
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
			_, err := svc.create(ctx, CreateRequest{Name: "web", Repo: tc.repo, Branch: tc.branch, Runtime: "docker"})
			if err != nil {
				t.Fatalf("create public repo: %v", err)
			}
			if resolver.calls != 1 || resolver.workspace != "tea-a" || resolver.repo != tc.repo || resolver.ref != tc.branch {
				t.Fatalf("resolution request=%+v", resolver)
			}
			if len(rec.appCreates) != 1 {
				t.Fatalf("created rows=%d, want 1", len(rec.appCreates))
			}
			row := rec.appCreates[0]
			app := getTenantApp(t, cl, "tea-a", "web")
			if row.Repo != tc.repo || row.Branch != tc.branch || row.FirstDeployCommit.Hash != tip || row.FirstDeployCommit.Message != "" {
				t.Fatalf("first deploy metadata=%+v", row)
			}
			if app.Spec.Repo != tc.repo || app.Spec.Branch != tc.branch || app.Spec.BuildCommit != tip {
				t.Fatalf("build source=%+v", app.Spec)
			}
		})
	}
}
