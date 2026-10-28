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

package deploys

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestTriggerPublicNonGitHubRepoPinsTheResolvedDeploy(t *testing.T) {
	const tip = "87e8c32b8918a836c09475e0f668ea7c436d3758"
	for _, tc := range []struct{ name, repo, branch, explicitRef, wantRef string }{
		{name: "GitLab default branch", repo: "https://gitlab.com/pages/plain-html", wantRef: "main"},
		{name: "Codeberg saved branch", repo: "https://codeberg.org/acme/site", branch: "release", wantRef: "release"},
		{name: "GitLab explicit ref", repo: "https://gitlab.com/pages/plain-html", branch: "main", explicitRef: tip, wantRef: tip},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := newFakeStore()
			app := repoApp("web", "srv-public", tc.branch)
			app.Spec.Repo = tc.repo
			resolver := &fakeResolver{commit: store.CommitInfo{Hash: tip}, ok: true}
			svc, cl := newService(ds, app)
			svc.Commits = resolver
			deploy, err := svc.Trigger(context.Background(), "web", TriggerParams{CommitID: tc.explicitRef})
			if err != nil {
				t.Fatalf("trigger public repo: %v", err)
			}
			if resolver.calls != 1 || resolver.gotWorkspace != "tea-acme" || resolver.gotRepo != tc.repo || resolver.gotRef != tc.wantRef {
				t.Fatalf("resolution request=%+v", resolver)
			}
			row, err := ds.GetDeploy(context.Background(), "srv-public", deploy.ID)
			if err != nil {
				t.Fatal(err)
			}
			if deploy.CommitID != tip || deploy.CommitMessage != "" || row.Commit != tip || row.CommitMessage != "" {
				t.Fatalf("SHA-only resolver metadata was lost: deploy=%+v row=%+v", deploy, row)
			}
			if got := getApp(t, cl, "web"); got.Spec.Repo != tc.repo || got.Spec.BuildCommit != tip {
				t.Fatalf("build source=%+v", got.Spec)
			}
			wire := toRenderDeploy(deploy)
			if wire.Commit == nil || wire.Commit.ID != tip {
				t.Fatalf("Render deploy omitted SHA-only commit: %+v", wire)
			}
		})
	}
}
