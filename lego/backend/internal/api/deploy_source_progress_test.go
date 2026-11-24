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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/renderlog"
)

type builtConformDeployStore struct {
	conformDeployStore
	built map[string]bool
}

func (s *builtConformDeployStore) BuiltDeployIDs(_ context.Context, _ string, _ int) (map[string]bool, error) {
	return s.built, nil
}

// w32/m608: the composition root wires each deploy row's own Repo and Branch
// into logs.DeployProgress, so switching a service's --repo, --branch, or
// --image preserves historical build progress lines and log IDs, pre-migration
// rows fall back to App.Spec, and GET /v1/services/{id}/deploys leaks no repo
// or branch fields.
func TestDeploySourceProgressHistoryKeepsEachDeploysOwnSource(t *testing.T) {
	const (
		repoA = "https://github.com/bex-co/bex"
		repoB = "https://github.com/expressjs/serve-static"
		img   = "docker.io/library/nginx:1.27-alpine"
	)
	base := time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC)
	at := func(sec int) *time.Time { ts := base.Add(time.Duration(sec) * time.Second); return &ts }

	depRepoA := store.Deploy{
		ID: "dep-repo-a", AppID: "web", Status: store.DeployDeactivated,
		Repo: repoA, Branch: "main", Commit: "1111111aaaa",
		CreatedAt: base, StartedAt: at(2), FinishedAt: at(20),
		Rollout: store.DeployRollout{StartedAt: at(15)},
	}
	depRepoB := store.Deploy{
		ID: "dep-repo-b", AppID: "web", Status: store.DeployDeactivated,
		Repo: repoB, Branch: "release", Commit: "",
		CreatedAt: *at(30), StartedAt: at(32), FinishedAt: at(50),
		Rollout: store.DeployRollout{StartedAt: at(45)},
	}
	depImage := store.Deploy{
		ID: "dep-image", AppID: "web", Status: store.DeployLive,
		Image:     img,
		CreatedAt: *at(60), StartedAt: at(62), FinishedAt: at(80),
	}
	depLegacy := store.Deploy{
		ID: "dep-legacy", AppID: "web", Status: store.DeployDeactivated,
		Commit:    "2222222bbbb",
		CreatedAt: *at(-60), StartedAt: at(-58), FinishedAt: at(-40),
	}

	ds := &builtConformDeployStore{
		conformDeployStore: conformDeployStore{byApp: map[string][]store.Deploy{"web": {depImage, depRepoB, depRepoA, depLegacy}}},
		built:              map[string]bool{depRepoA.ID: true, depRepoB.ID: true, depLegacy.ID: true},
	}

	query := func(repo, branch, image string) map[string][]logs.LogEntry {
		t.Helper()
		app := sampleApp("web")
		app.Spec.Repo = repo
		app.Spec.Branch = branch
		app.Spec.Image = image
		server := NewServer(&core.Base{Client: fakeClient(app), Namespace: "default"}, Deps{
			DeployStore: ds,
			LogHistory:  func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) { return nil, nil },
		})
		entries, err := server.Logs.QueryLogs(context.Background(), logs.LogQuery{
			App: "web", Types: []string{logs.LogTypeBuild},
			Since: base.Add(-2 * time.Minute), End: base.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		byDeploy := map[string][]logs.LogEntry{}
		for _, e := range entries {
			byDeploy[e.Labels[logs.LabelBuild]] = append(byDeploy[e.Labels[logs.LabelBuild]], e)
		}
		return byDeploy
	}

	// Read while the App is on repoA@main vs after switching to a prebuilt image.
	onRepoA := query(repoA, "main", "")
	onImage := query("", "", img)

	for _, id := range []string{depRepoA.ID, depRepoB.ID, depImage.ID} {
		if len(onRepoA[id]) == 0 || !slices.EqualFunc(onRepoA[id], onImage[id], func(a, b logs.LogEntry) bool {
			return a.Message == b.Message && a.Timestamp == b.Timestamp && a.Labels[logs.LabelInstance] == b.Labels[logs.LabelInstance]
		}) {
			t.Fatalf("%s drifted across source switch:\nonRepoA=%+v\nonImage=%+v", id, onRepoA[id], onImage[id])
		}
	}

	messages := func(entries []logs.LogEntry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Message
		}
		return out
	}
	deployingLine := renderlog.PlatformStep.Line("Deploying...")

	if msgs := messages(onImage[depRepoA.ID]); !slices.Contains(msgs, "==> Build queued") ||
		!slices.Contains(msgs, "==> Building from "+repoA+"@1111111") ||
		!slices.Contains(msgs, deployingLine) {
		t.Fatalf("dep-repo-a after image switch = %q", msgs)
	}
	if msgs := messages(onImage[depRepoB.ID]); !slices.Contains(msgs, "==> Building from "+repoB+"@release") ||
		!slices.Contains(msgs, deployingLine) {
		t.Fatalf("dep-repo-b after image switch = %q", msgs)
	}
	if msgs := messages(onRepoA[depImage.ID]); !slices.Contains(msgs, "==> Deploy queued") ||
		!slices.Contains(msgs, "==> Deploying image "+img) {
		t.Fatalf("dep-image while App on repoA = %q", msgs)
	}
	if msgs := messages(onRepoA[depLegacy.ID]); !slices.Contains(msgs, "==> Building from "+repoA+"@2222222") {
		t.Fatalf("pre-migration fallback = %q", msgs)
	}

	// Public REST deploy list must expose neither repo nor branch (and stays
	// Render OpenAPI-conformant via serverWith/buildHandler).
	app := sampleApp("web")
	if app.Labels == nil {
		app.Labels = map[string]string{}
	}
	app.Labels[store.LabelAppID] = "web"
	app.Spec.Repo = repoA
	app.Spec.Branch = "main"
	h, _ := serverWith(t, &core.Base{Client: fakeClient(app), Namespace: "default"}, Deps{DeployStore: ds})
	rec := do(t, h, http.MethodGet, "/v1/services/web/deploys", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/services/web/deploys = %d %s", rec.Code, rec.Body)
	}
	var items []struct {
		Deploy map[string]any `json:"deploy"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || len(items) == 0 {
		t.Fatalf("decode deploys (%d): %v %s", len(items), err, rec.Body)
	}
	for _, item := range items {
		if _, ok := item.Deploy["repo"]; ok {
			t.Fatalf("public deploy DTO leaked repo: %+v", item.Deploy)
		}
		if _, ok := item.Deploy["branch"]; ok {
			t.Fatalf("public deploy DTO leaked branch: %+v", item.Deploy)
		}
	}
}
