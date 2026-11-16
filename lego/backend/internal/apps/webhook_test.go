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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- pathsIgnoredReason: the Root Directory and Build Filters decide together ---

// TestPathsIgnoredReason walks every cell of the decision table in
// docs/render-artifacts/auto-deploy-filters.md (w7/m428 t002), plus the
// matching details (globs, slashes, prefixes). "" deploys.
func TestPathsIgnoredReason(t *testing.T) {
	const (
		deploys = ""
		rootDir = store.EventReasonRootDirectory
		filter  = store.EventReasonBuildFilter
	)
	bf := func(paths, ignored []string) *appv1alpha1.BuildFilterSpec {
		return &appv1alpha1.BuildFilterSpec{Paths: paths, IgnoredPaths: ignored}
	}
	shared := []string{"shared/**"}
	docs := []string{"**/*.md"}
	cases := []struct {
		name    string
		rootDir string
		filter  *appv1alpha1.BuildFilterSpec
		paths   []string
		want    string
	}{
		// No changed paths in the payload: nothing to filter on, so it deploys.
		{"no paths, root directory set", "backend", nil, nil, deploys},
		{"no paths, included paths set", "", bf(shared, nil), nil, deploys},
		{"no paths, everything else set", "backend", bf(shared, docs), nil, deploys},

		// Row 1: no root directory, no filter. Every change deploys.
		{"row 1: any file", "", nil, []string{"unrelated/file.go"}, deploys},
		{"row 1: an all-empty filter is no filter", "", bf(nil, nil), []string{"unrelated/file.go"}, deploys},

		// Row 2: ignored paths only. Deploys unless every changed file is ignored.
		{"row 2: every file ignored", "", bf(nil, []string{"docs/**"}), []string{"docs/a.md", "docs/b.md"}, filter},
		{"row 2: one file not ignored", "", bf(nil, []string{"docs/**"}), []string{"docs/a.md", "src/main.go"}, deploys},

		// Row 3: included paths, no root directory. A file must be included and not ignored.
		{"row 3: an included file", "", bf([]string{"src/**"}, nil), []string{"src/app/main.go"}, deploys},
		{"row 3: no included file", "", bf([]string{"src/**"}, nil), []string{"web/index.js"}, filter},
		{"row 3: one included file among several", "", bf([]string{"src/**"}, nil), []string{"web/index.js", "src/app/main.go"}, deploys},
		{"row 3: globstar spans directories", "", bf(docs, nil), []string{"docs/guide/readme.md"}, deploys},
		{"row 3: a leading slash on the changed path", "", bf([]string{"src/**"}, nil), []string{"/src/main.go"}, deploys},
		{"row 3: included but also ignored", "", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.test.go"}, filter},
		{"row 3: included and not ignored", "", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.go"}, deploys},
		{"row 3: one triggering file among ignored ones", "", bf([]string{"src/**"}, []string{"src/**/*.test.go"}), []string{"src/app/main.test.go", "src/app/main.go"}, deploys},

		// Row 4: root directory, no filter. A file must lie under the root directory.
		{"row 4: a file under the root directory", "services/api", nil, []string{"services/api/main.go"}, deploys},
		{"row 4: a file outside it", "services/api", nil, []string{"services/web/index.js"}, rootDir},
		{"row 4: a top-level file", "services/api", nil, []string{"README.md"}, rootDir},
		{"row 4: one file under it among several", "services/api", nil, []string{"README.md", "services/api/main.go"}, deploys},
		{"row 4: a leading slash on the changed path", "services/api", nil, []string{"/services/api/main.go"}, deploys},
		{"row 4: a trailing slash on the root directory", "services/api/", nil, []string{"services/api/main.go"}, deploys},
		{"row 4: a sibling sharing the name's prefix is outside", "services/api", nil, []string{"services/api-gateway/main.go"}, rootDir},

		// Row 5: root directory and ignored paths, no included paths. A file must
		// lie under the root directory and not be ignored.
		{"row 5: under the root and not ignored", "backend", bf(nil, docs), []string{"backend/main.go"}, deploys},
		{"row 5: under the root but ignored", "backend", bf(nil, docs), []string{"backend/README.md"}, filter},
		{"row 5: outside the root, not ignored", "backend", bf(nil, docs), []string{"frontend/app.js"}, rootDir},
		{"row 5: outside the root and ignored", "backend", bf(nil, docs), []string{"README.md"}, rootDir},
		{"row 5: ignored under the root plus a file outside it (inferred)", "backend", bf(nil, docs), []string{"backend/README.md", "frontend/app.js"}, filter},
		{"row 5: an ignored file never rescues a push outside the root", "backend", bf(nil, docs), []string{"backend/README.md", "frontend/README.md"}, filter},

		// Row 6: root directory and included paths. The included paths decide,
		// wherever they point; the root directory no longer does.
		{"row 6: under the root and included", "backend", bf([]string{"backend/**", "shared/**"}, nil), []string{"backend/main.go"}, deploys},
		{"row 6: under the root, not included (the ambiguous cell)", "backend", bf(shared, nil), []string{"backend/main.go"}, filter},
		{"row 6: outside the root and included", "backend", bf(shared, nil), []string{"shared/lib.go"}, deploys},
		{"row 6: outside the root, not included", "backend", bf(shared, nil), []string{"frontend/app.js"}, filter},
		{"row 6: outside the root, included but ignored", "backend", bf(shared, docs), []string{"shared/README.md"}, filter},
		{"row 6: one included file outside the root among ignored ones", "backend", bf(shared, docs), []string{"shared/README.md", "shared/lib.go"}, deploys},
		{"row 6: under the root, included but ignored", "backend", bf([]string{"backend/**"}, docs), []string{"backend/README.md"}, filter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pathsIgnoredReason(c.rootDir, c.filter, c.paths); got != c.want {
				t.Errorf("pathsIgnoredReason(%q, %+v, %v) = %q, want %q", c.rootDir, c.filter, c.paths, got, c.want)
			}
		})
	}
}

// andRule is the decision bex made before w7/m428: the root directory and the
// build filters each had to pass, so an included path outside the root
// directory could never deploy.
func andRule(rootDir string, bf *appv1alpha1.BuildFilterSpec, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	rootOK := rootDir == ""
	filterOK := bf == nil || (len(bf.Paths) == 0 && len(bf.IgnoredPaths) == 0)
	for _, p := range paths {
		p = strings.TrimPrefix(p, "/")
		rootOK = rootOK || underRootDir(rootDir, p)
		if bf != nil && !matchesAnyGlob(bf.IgnoredPaths, p) && (len(bf.Paths) == 0 || matchesAnyGlob(bf.Paths, p)) {
			filterOK = true
		}
	}
	return rootOK && filterOK
}

// TestPathsDecisionOnlyWidensButForIgnoredFilesUnderTheRoot is the w7/m428
// property, checked over every combination of a small monorepo: a push the
// old AND rule deployed still deploys, with one exception. With a root
// directory, ignored paths and no included paths, a push whose files under the
// root directory are all ignored, and whose other files lie outside it, no
// longer deploys: Render treats a file outside the root directory as ignored
// too (the decision table's row 5, inferred cell).
func TestPathsDecisionOnlyWidensButForIgnoredFilesUnderTheRoot(t *testing.T) {
	universe := []string{"backend/main.go", "backend/README.md", "shared/lib.go", "shared/README.md", "frontend/app.js", "README.md"}
	rootDirs := []string{"", "backend"}
	includes := [][]string{nil, {"shared/**"}, {"backend/**"}, {"**/*.go"}}
	ignores := [][]string{nil, {"**/*.md"}, {"shared/**"}, {"backend/**"}}
	checked, widened, narrowed := 0, 0, 0
	for _, rd := range rootDirs {
		for _, inc := range includes {
			for _, ign := range ignores {
				var filter *appv1alpha1.BuildFilterSpec
				if inc != nil || ign != nil {
					filter = &appv1alpha1.BuildFilterSpec{Paths: inc, IgnoredPaths: ign}
				}
				for mask := 1; mask < 1<<len(universe); mask++ {
					var paths []string
					for i, p := range universe {
						if mask&(1<<i) != 0 {
							paths = append(paths, p)
						}
					}
					checked++
					before, after := andRule(rd, filter, paths), pathsIgnoredReason(rd, filter, paths) == ""
					switch {
					case !before && after:
						widened++
					case before && !after:
						narrowed++
						if !onlyIgnoredUnderRoot(rd, inc, ign, paths) {
							t.Errorf("root %q, included %v, ignored %v, paths %v: deployed before and not now", rd, inc, ign, paths)
						}
					}
				}
			}
		}
	}
	if widened == 0 || narrowed == 0 {
		t.Fatalf("checked %d pushes: widened %d, narrowed %d; the property must see both kinds of change", checked, widened, narrowed)
	}
}

// onlyIgnoredUnderRoot is the one cell where the old rule deployed and Render's
// does not: a root directory and ignored paths, no included paths, every
// changed file under the root directory ignored, and some file outside it.
func onlyIgnoredUnderRoot(rootDir string, included, ignored, paths []string) bool {
	if rootDir == "" || len(included) > 0 || len(ignored) == 0 {
		return false
	}
	outside := false
	for _, p := range paths {
		if !underRootDir(rootDir, p) {
			outside = true
		} else if !matchesAnyGlob(ignored, p) {
			return false
		}
	}
	return outside
}

// An included path outside the root directory deploys the service through the
// webhook, and an ignored path matching the same file still wins.
func TestRedeployMatchingIncludedPathOutsideRootDirDeploys(t *testing.T) {
	const repo = "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Spec.RootDir = "backend"
	app.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"shared/**"}}
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	svc, _ := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	got, _, err := h.redeployMatching(context.Background(), newPush(repo, []string{"shared/lib.go"}), "main", "")
	if err != nil || !contains(got, "api") {
		t.Fatalf("a change under an included path outside the root directory = %v (err %v), want api redeployed", got, err)
	}

	app2 := autoDeployApp("api", repo)
	app2.Spec.RootDir = "backend"
	app2.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"shared/**"}, IgnoredPaths: []string{"shared/**/*.md"}}
	app2.Labels = app.Labels
	svc2, _ := newService(nil, app2)
	svc2.EventFacts = writer
	h2 := &GitWebhook{Svc: svc2, Secret: "shh"}
	ev := newPush(repo, []string{"shared/README.md"})
	ev.DeliveryKey, ev.HeadCommit.ID = "delivery-ignored", "abc123"
	got2, _, err := h2.redeployMatching(context.Background(), ev, "main", "")
	if err != nil || len(got2) != 0 {
		t.Fatalf("an included file that is also ignored = %v (err %v), want no redeploy", got2, err)
	}
	fact, ok := writer.facts["git:delivery-ignored:srv-api:ignored"]
	if !ok || fact.Type != store.EventFactCommitIgnored || fact.ReasonCode != store.EventReasonBuildFilter {
		t.Fatalf("ignored fact = %+v (recorded %v), want commit_ignored with build_filter", fact, ok)
	}
}

// TestRedeployMatchingComposesBuildFilterWithRootDir: with no included paths
// the root directory decides, and an ignored path inside it still wins.
func TestRedeployMatchingComposesBuildFilterWithRootDir(t *testing.T) {
	const repo = "https://github.com/x/mono"
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "api", "default"
	app.Spec = appv1alpha1.AppSpec{
		Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true,
		BuildFilter: &appv1alpha1.BuildFilterSpec{IgnoredPaths: []string{"services/api/**/*.md"}},
	}
	svc, _ := newService(nil, app)
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	// Inside rootDir but only a doc change the filter ignores => no redeploy.
	ignored := newPush(repo, []string{"services/api/README.md"})
	got, _, err := h.redeployMatching(context.Background(), ignored, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if contains(got, "api") {
		t.Errorf("an in-rootDir push matching only ignoredPaths must not redeploy: %v", got)
	}

	// Inside rootDir and not ignored => redeploy.
	included := newPush(repo, []string{"services/api/main.go"})
	got2, _, err := h.redeployMatching(context.Background(), included, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(got2, "api") {
		t.Errorf("an in-rootDir, non-ignored push must redeploy: %v", got2)
	}
}

func TestPushEventChangedPaths(t *testing.T) {
	var ev pushEvent
	ev.Commits = append(ev.Commits, struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{Added: []string{"a.go"}, Removed: []string{"b.go"}, Modified: []string{"c.go"}})
	ev.Commits = append(ev.Commits, struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{Modified: []string{"services/api/main.go"}})

	got := ev.changedPaths()
	want := []string{"a.go", "b.go", "c.go", "services/api/main.go"}
	if len(got) != len(want) {
		t.Fatalf("changedPaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("changedPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

type recordingFactWriter struct {
	facts map[string]store.ServiceEventFact
}

func (w *recordingFactWriter) InsertServiceEventFact(_ context.Context, fact store.ServiceEventFact) (bool, error) {
	if w.facts == nil {
		w.facts = map[string]store.ServiceEventFact{}
	}
	if _, exists := w.facts[fact.SourceKey]; exists {
		return false, nil
	}
	w.facts[fact.SourceKey] = fact
	return true, nil
}

func TestIgnoredCommitFactIsTypedAndDeliveryIdempotent(t *testing.T) {
	const repo = "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Spec.RootDir = "services/api"
	app.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-api",
	}
	svc, _ := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: "shh"}
	ev := newPush(repo, []string{"services/web/index.js"})
	ev.DeliveryKey = "delivery-1"
	ev.After = "abc123"
	ev.HeadCommit.ID = "abc123"
	ev.HeadCommit.URL = "https://github.com/x/mono/commit/abc123"

	for range 2 {
		if redeployed, _, err := h.redeployMatching(context.Background(), ev, "main", ""); err != nil || len(redeployed) != 0 {
			t.Fatalf("redeploy = %v, err = %v, want ignored", redeployed, err)
		}
	}
	if len(writer.facts) != 1 {
		t.Fatalf("fact count = %d, want one across delivery retry", len(writer.facts))
	}
	for _, fact := range writer.facts {
		if fact.Type != store.EventFactCommitIgnored || fact.ReasonCode != store.EventReasonRootDirectory || fact.CommitID != "abc123" {
			t.Fatalf("ignored fact = %+v", fact)
		}
	}
}

// TestWebhookBranchDeleteRecordsFactAndDisablesAutoDeploy proves w7/m66's
// branch_deleted: a `delete` event for a tracked branch records the typed fact
// once (idempotent across a delivery retry) and disables the service's
// auto-deploy — the branch it built from is gone.
func TestWebhookBranchDeleteRecordsFactAndDisablesAutoDeploy(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-api",
	}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "main",
		"ref_type":   "branch",
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		req.Header.Set("X-GitHub-Event", "delete")
		req.Header.Set("X-GitHub-Delivery", "del-1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := range 2 { // deliver twice: the fact must be idempotent
		if rec := send(); rec.Code != http.StatusOK {
			t.Fatalf("delivery %d => 200, got %d: %s", i, rec.Code, rec.Body)
		}
	}
	if len(writer.facts) != 1 {
		t.Fatalf("fact count = %d, want 1 across delivery retry", len(writer.facts))
	}
	for _, fact := range writer.facts {
		if fact.Type != store.EventFactBranchDeleted || fact.BranchFrom != "main" {
			t.Fatalf("branch_deleted fact = %+v", fact)
		}
	}
	if getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("branch delete must disable auto-deploy for a service tracking the deleted branch")
	}
}

// TestWebhookBranchDeleteVariants covers the push-with-deleted signal (git push
// --delete) and the tag-delete no-op.
func TestWebhookBranchDeleteVariants(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"

	t.Run("push with deleted=true records the fact", func(t *testing.T) {
		app := autoDeployApp("api", repo)
		app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
		svc, cl := newService(nil, app)
		writer := &recordingFactWriter{}
		svc.EventFacts = writer
		h := &GitWebhook{Svc: svc, Secret: secret}

		body, _ := json.Marshal(map[string]any{
			"ref":        "refs/heads/main",
			"deleted":    true,
			"after":      "0000000000000000000000000000000000000000",
			"repository": map[string]string{"clone_url": repo},
		})
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
		}
		if len(writer.facts) != 1 {
			t.Fatalf("fact count = %d, want 1", len(writer.facts))
		}
		if getApp(t, cl, "api").Spec.AutoDeploy {
			t.Error("push --delete of the tracked branch must disable auto-deploy")
		}
	})

	t.Run("tag delete is a no-op", func(t *testing.T) {
		app := autoDeployApp("api", repo)
		app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
		svc, cl := newService(nil, app)
		writer := &recordingFactWriter{}
		svc.EventFacts = writer
		h := &GitWebhook{Svc: svc, Secret: secret}

		body, _ := json.Marshal(map[string]any{
			"ref":        "v1.0.0",
			"ref_type":   "tag",
			"repository": map[string]string{"clone_url": repo},
		})
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		req.Header.Set("X-Hub-Signature-256", sign(secret, body))
		req.Header.Set("X-GitHub-Event", "delete")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
		}
		if len(writer.facts) != 0 {
			t.Fatalf("tag delete recorded %d facts, want 0", len(writer.facts))
		}
		if !getApp(t, cl, "api").Spec.AutoDeploy {
			t.Error("tag delete must not touch auto-deploy")
		}
	})
}

// Render's six skip phrases (render.com/docs/deploys § Skipping an auto-deploy)
// and messages that only look like one (w7/m428 t001).
var (
	renderSkipPhrases = []string{"[skip render]", "[render skip]", "[skip deploy]", "[deploy skip]", "[skip cd]", "[cd skip]"}
	skipNearMisses    = []string{
		"ordinary release", "[skip-render]", "skip render", "render skip", "[skip  render]", "[skipdeploy]",
		"(skip cd)", "[skip ci]", "[ci skip]", "[skip bex]", "[skip render", "skip render]",
	}
)

func TestCommitSkipPhrases(t *testing.T) {
	for _, phrase := range renderSkipPhrases {
		for _, message := range []string{phrase, strings.ToUpper(phrase), "docs: fix a typo " + phrase, phrase + " docs\n\nlonger body", "subject\n\nbody " + phrase} {
			if !commitHasSkipPhrase(message) {
				t.Errorf("%q did not match", message)
			}
		}
	}
	for _, message := range skipNearMisses {
		if commitHasSkipPhrase(message) {
			t.Errorf("%q matched a skip phrase", message)
		}
	}
}

// Each skip phrase skips the deploy and records commit_ignored with the
// skip-phrase reason, on the commit path and on the checks-pass path (where the
// commit is not even awaited); a near-miss still deploys or is awaited.
func TestSkipPhrasesSkipBothAutoDeployPaths(t *testing.T) {
	const repo = "https://github.com/x/mono"
	labels := map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	push := func(message string) pushEvent {
		ev := newPush(repo, []string{"main.go"})
		ev.DeliveryKey, ev.HeadCommit.ID, ev.HeadCommit.Message = "delivery-"+message, shaA, message
		return ev
	}
	for _, trigger := range []string{"commit", checksPassTrigger} {
		newApp := func() *appv1alpha1.App {
			a := autoDeployApp("api", repo)
			a.Labels = labels
			if trigger == checksPassTrigger {
				a.Spec.AutoDeployTrigger = checksPassTrigger
			}
			return a
		}
		for _, phrase := range renderSkipPhrases {
			t.Run(trigger+" "+phrase, func(t *testing.T) {
				svc, cl := newService(nil, newApp())
				writer := &recordingFactWriter{}
				svc.EventFacts = writer
				h := &GitWebhook{Svc: svc, Secret: "shh"}
				ev := push("chore: bump the docs " + phrase)
				redeployed, _, err := h.redeployMatching(context.Background(), ev, "main", "")
				if err != nil || len(redeployed) != 0 {
					t.Fatalf("redeployed %v (err %v), want none", redeployed, err)
				}
				got := getApp(t, cl, "api")
				if got.Spec.RestartedAt != "" || got.Annotations[annotChecksPendingCommit] != "" {
					t.Errorf("restartedAt %q, awaited %q: a skipped commit must neither deploy nor wait for checks", got.Spec.RestartedAt, got.Annotations[annotChecksPendingCommit])
				}
				fact, ok := writer.facts["git:"+ev.DeliveryKey+":srv-api:ignored"]
				if !ok || fact.Type != store.EventFactCommitIgnored || fact.ReasonCode != store.EventReasonSkipPhrase || fact.CommitID != shaA {
					t.Errorf("ignored fact = %+v (recorded %v), want commit_ignored with skip_phrase for %s", fact, ok, shaA)
				}
			})
		}
		for _, message := range skipNearMisses {
			t.Run(trigger+" near miss "+message, func(t *testing.T) {
				svc, cl := newService(nil, newApp())
				writer := &recordingFactWriter{}
				svc.EventFacts = writer
				h := &GitWebhook{Svc: svc, Secret: "shh"}
				redeployed, _, err := h.redeployMatching(context.Background(), push(message), "main", "")
				if err != nil {
					t.Fatal(err)
				}
				got := getApp(t, cl, "api")
				if trigger == checksPassTrigger {
					if got.Annotations[annotChecksPendingCommit] != shaA {
						t.Errorf("awaited %q, want %s awaiting its checks", got.Annotations[annotChecksPendingCommit], shaA)
					}
				} else if !contains(redeployed, "api") {
					t.Errorf("redeployed %v, want api", redeployed)
				}
				if len(writer.facts) != 0 {
					t.Errorf("recorded %v, want no commit_ignored", writer.facts)
				}
			})
		}
	}
}

// --- redeployMatching end-to-end: in-root vs out-of-root push ---

func newPush(cloneURL string, changed []string) pushEvent {
	ev := pushEvent{Ref: "refs/heads/main", After: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	ev.Repository.CloneURL = cloneURL
	ev.Commits = []struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	}{{Modified: changed}}
	return ev
}

func TestRedeployMatchingScopesToRootDir(t *testing.T) {
	const repo = "https://github.com/x/mono"
	scoped := &appv1alpha1.App{}
	scoped.Name, scoped.Namespace = "api", "default"
	scoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true}

	unscoped := &appv1alpha1.App{}
	unscoped.Name, unscoped.Namespace = "whole-repo", "default"
	unscoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}

	svc, _ := newService(nil, scoped, unscoped)
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	outside := newPush(repo, []string{"services/web/index.js"})
	redeployed, _, err := h.redeployMatching(context.Background(), outside, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if contains(redeployed, "api") {
		t.Errorf("push outside rootDir must not redeploy the scoped App: redeployed=%v", redeployed)
	}
	if !contains(redeployed, "whole-repo") {
		t.Errorf("push outside rootDir must still redeploy an App with no rootDir: redeployed=%v", redeployed)
	}

	inside := newPush(repo, []string{"services/api/main.go"})
	redeployed2, _, err := h.redeployMatching(context.Background(), inside, "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(redeployed2, "api") {
		t.Errorf("push inside rootDir must redeploy the scoped App: redeployed=%v", redeployed2)
	}
	if !contains(redeployed2, "whole-repo") {
		t.Errorf("push inside rootDir must still redeploy an App with no rootDir: redeployed=%v", redeployed2)
	}
}

// TestWebhookHTTPParsesCommitPaths exercises the real JSON wire shape (a raw
// push payload's "commits[].modified", not a hand-built pushEvent) through
// ServeHTTP end to end, so a tag/parsing regression in pushEvent would fail
// here even though TestRedeployMatchingScopesToRootDir builds pushEvent directly.
func TestWebhookHTTPParsesCommitPaths(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	scoped := &appv1alpha1.App{}
	scoped.Name, scoped.Namespace = "api", "default"
	scoped.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", RootDir: "services/api", AutoDeploy: true}
	svc, cl := newService(nil, scoped)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "refs/heads/main",
		"after":      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"repository": map[string]string{"clone_url": repo},
		"commits":    []map[string]any{{"modified": []string{"services/web/index.js"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("a push whose commits[].modified sits outside rootDir must not redeploy")
	}
}

// autoDeployApp is a repo-backed app that opts into push redeploys.
func autoDeployApp(name, repo string) *appv1alpha1.App {
	a := &appv1alpha1.App{}
	a.Name, a.Namespace = name, "default"
	a.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: true}
	return a
}

type webhookStartedCall struct{ tenantID, appName string }

type blockingWebhookStartedNotifier struct {
	calls   chan webhookStartedCall
	release chan struct{}
}

func (n *blockingWebhookStartedNotifier) NotifyDeployStarted(_ context.Context, tenantID, appName, _ string) {
	n.calls <- webhookStartedCall{tenantID: tenantID, appName: appName}
	<-n.release
}

func postPush(t *testing.T, h *GitWebhook, sigSecret, event string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	if sigSecret != "" {
		req.Header.Set("X-Hub-Signature-256", sign(sigSecret, body))
	}
	if event != "" {
		req.Header.Set("X-GitHub-Event", event)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestWebhookRejectsHeaderSwappedDeleteBodyAsPush pins codex-security round 12,
// finding 10: the HMAC covers the body, not the X-GitHub-Event header, so a
// captured signed delete delivery replayed with the header stripped (the
// pre-fix Gitea fallback) or forced to "push" must NOT be reinterpreted as a
// push — it is a 400 before scope resolution, never redeploys, and the
// legitimate delete delivery afterwards still processes (the replays were
// rejected before claiming anything).
func TestWebhookRejectsHeaderSwappedDeleteBodyAsPush(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	deleteBody, err := json.Marshal(map[string]any{
		"ref":        "main",
		"ref_type":   "branch",
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"", "push"} {
		rec := postPush(t, h, secret, event, deleteBody)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("header %q on delete body => 400, got %d: %s", event, rec.Code, rec.Body)
		}
		if getApp(t, cl, "api").Spec.RestartedAt != "" {
			t.Fatalf("header %q reinterpreted a delete body as a push", event)
		}
	}
	// A body with no ref at all (e.g. a ping payload) is likewise not a push.
	pingBody, _ := json.Marshal(map[string]any{"zen": "Keep it logically awesome.", "hook_id": 1})
	if rec := postPush(t, h, secret, "push", pingBody); rec.Code != http.StatusBadRequest {
		t.Fatalf("ping body under push header => 400, got %d: %s", rec.Code, rec.Body)
	}

	// The legitimate delete delivery still processes after the replays.
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(deleteBody)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, deleteBody))
	req.Header.Set("X-GitHub-Event", "delete")
	req.Header.Set("X-GitHub-Delivery", "del-legit")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legitimate delete => 200, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("legitimate branch delete must disable auto-deploy")
	}
}

// A push-shaped body replayed under the delete header is a signed no-op, not a
// branch deletion (the symmetric half of the shape binding).
func TestWebhookDeleteHeaderOnPushBodyIsNoOp(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/mono"
	app := autoDeployApp("api", repo)
	app.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-api"}
	svc, cl := newService(nil, app)
	writer := &recordingFactWriter{}
	svc.EventFacts = writer
	h := &GitWebhook{Svc: svc, Secret: secret}

	rec := postPush(t, h, secret, "delete", pushBody(t, repo, "refs/heads/main"))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete header on push body => 200 no-op, got %d: %s", rec.Code, rec.Body)
	}
	if len(writer.facts) != 0 {
		t.Fatalf("push body under delete header recorded %+v, want none", writer.facts)
	}
	if !getApp(t, cl, "api").Spec.AutoDeploy {
		t.Error("push body under delete header must not disable auto-deploy")
	}
}

func TestWebhookAcceptsEitherKey(t *testing.T) {
	const repo = "https://github.com/x/app"
	body := pushBody(t, repo, "refs/heads/main")

	// GitHub App key alone (BEX_WEBHOOK_SECRET unset): a GitHub-App-signed push
	// still redeploys.
	svc, cl := newService(nil, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusOK {
		t.Fatalf("github key => %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.RestartedAt == "" {
		t.Error("github-key push should redeploy")
	}

	// Both keys set: valid under EITHER key is accepted.
	for _, key := range []string{"manual-key", "gh-key"} {
		svc, cl := newService(nil, autoDeployApp("api", repo))
		h := &GitWebhook{Svc: svc, Secret: "manual-key", GitHubSecret: "gh-key"}
		if rec := postPush(t, h, key, "push", body); rec.Code != http.StatusOK {
			t.Fatalf("key %s => %d", key, rec.Code)
		}
		if getApp(t, cl, "api").Spec.RestartedAt == "" {
			t.Errorf("push signed with %s should redeploy", key)
		}
	}
}

func TestWebhookNotifiesDeployStartedOffRequestPath(t *testing.T) {
	const repo = "https://github.com/x/app"
	a := autoDeployApp("api", repo)
	a.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc, _ := newService(nil, a)
	notifier := &blockingWebhookStartedNotifier{
		calls:   make(chan webhookStartedCall, 1),
		release: make(chan struct{}),
	}
	defer close(notifier.release)
	svc.StartedNotifier = notifier
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	body := pushBody(t, repo, "refs/heads/main")

	response := make(chan *httptest.ResponseRecorder, 1)
	go func() { response <- postPush(t, h, "gh-key", "push", body) }()

	select {
	case rec := <-response:
		if rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("git webhook blocked on deploy-start notification delivery")
	}
	select {
	case got := <-notifier.calls:
		if got != (webhookStartedCall{tenantID: "tea-a", appName: "api"}) {
			t.Errorf("started notification = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("git webhook returned without issuing a deploy-start notification")
	}
}

func TestWebhookRejectsUnknownKey(t *testing.T) {
	body := pushBody(t, "https://github.com/x/app", "refs/heads/main")
	svc, _ := newService(nil, autoDeployApp("api", "https://github.com/x/app"))
	h := &GitWebhook{Svc: svc, Secret: "manual-key", GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "attacker-key", "push", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-key push => %d, want 401", rec.Code)
	}
}

func TestWebhookGitHubLifecycleEventsAreNoOp(t *testing.T) {
	const repo = "https://github.com/x/app"
	body := pushBody(t, repo, "refs/heads/main")
	svc, cl := newService(nil, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}

	// A validly-signed ping (or installation) is a 200 no-op, not a 401 and not a redeploy.
	for _, event := range []string{"ping", "installation", "installation_repositories"} {
		rec := postPush(t, h, "gh-key", event, body)
		if rec.Code != http.StatusOK {
			t.Errorf("%s => %d, want 200", event, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ignored") {
			t.Errorf("%s body = %s, want ignored", event, rec.Body.String())
		}
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("lifecycle events must not redeploy")
	}

	// An unsigned ping is still rejected — the signature gate runs first.
	if rec := postPush(t, h, "", "ping", body); rec.Code != http.StatusUnauthorized {
		t.Errorf("unsigned ping => %d, want 401", rec.Code)
	}
}

func TestWebhookAutoDeployFalseSuppressesGitHubPush(t *testing.T) {
	const repo = "https://github.com/x/app"
	// AutoDeploy off: a valid GitHub-App-signed push must NOT redeploy.
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "api", "default"
	app.Spec = appv1alpha1.AppSpec{Repo: repo, Branch: "main", AutoDeploy: false}
	svc, cl := newService(nil, app)
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}

	rec := postPush(t, h, "gh-key", "push", pushBody(t, repo, "refs/heads/main"))
	if rec.Code != http.StatusOK {
		t.Fatalf("push => %d, want 200", rec.Code)
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("autoDeploy:false must suppress the GitHub-key push redeploy")
	}
}

func TestWebhook503WhenNoKeys(t *testing.T) {
	svc, _ := newService(nil)
	h := &GitWebhook{Svc: svc} // neither key set
	body := pushBody(t, "https://github.com/x/app", "refs/heads/main")
	if rec := postPush(t, h, "anything", "push", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no keys => %d, want 503", rec.Code)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// --- push-to-deploy opens a deploy row (trigger "new_commit") ---

// TestWebhookPushOpensDeployRow proves a push on a store-managed App opens a
// deploys row through the real wire shape (raw JSON with head_commit, HMAC
// signature, ServeHTTP end to end): trigger "new_commit", the requested
// release generation, and the pushed head commit as provenance — plus the
// release-generation annotation on the CR, the same stamp deploys.Trigger
// writes. The row is what puts push-to-deploy in the deploy history and what
// replaces (via CreateDeploy's latest-pending slot) any queued deploy the push
// supersedes, without preempting the release already executing.
func TestWebhookPushOpensDeployRow(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/app"
	app := autoDeployApp("api", repo)
	app.Generation = 4
	// Both labels: the app-id alone names a public id; only managed-by proves it
	// is a Postgres source row (managedAppID's own gate).
	app.Labels = map[string]string{
		core.LabelAppID:      "srv-x1",
		store.LabelManagedBy: store.ManagedByValue,
	}
	st := &recordingStore{}
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body, err := json.Marshal(map[string]any{
		"ref":        "refs/heads/main",
		"after":      "3597548b2424b9dee1ce4b07820b0f8cb007f747",
		"repository": map[string]string{"clone_url": repo},
		"commits":    []map[string]any{{"modified": []string{"main.go"}}},
		"head_commit": map[string]string{
			"id":        "3597548b2424b9dee1ce4b07820b0f8cb007f747",
			"message":   "fix(open-ledger): isolate registry from client bundle",
			"timestamp": "2026-07-19T06:58:44Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec := postPush(t, h, secret, "push", body); rec.Code != http.StatusOK {
		t.Fatalf("push => %d: %s", rec.Code, rec.Body)
	}

	if len(st.deployCalls) != 1 {
		t.Fatalf("deploy rows opened = %d, want exactly 1: %+v", len(st.deployCalls), st.deployCalls)
	}
	d := st.deployCalls[0]
	if d.AppID != "srv-x1" || d.Trigger != store.TriggerNewCommit {
		t.Errorf("deploy row = {AppID:%s Trigger:%s}, want {srv-x1 new_commit}", d.AppID, d.Trigger)
	}
	if d.Generation != 5 {
		t.Errorf("deploy row generation = %d, want previous generation + 1 = 5", d.Generation)
	}
	if d.Commit != "3597548b2424b9dee1ce4b07820b0f8cb007f747" || d.CommitMessage != "fix(open-ledger): isolate registry from client bundle" {
		t.Errorf("deploy row commit = {%s %q}, want the pushed head commit", d.Commit, d.CommitMessage)
	}
	a := getApp(t, cl, "api")
	if got := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != "5" {
		t.Errorf("release-generation annotation = %q, want 5", got)
	}
	if a.Spec.RestartedAt == "" {
		t.Error("push must still bump restartedAt")
	}
}

func TestWebhookPushAfterRepoRollbackClearsImageOverride(t *testing.T) {
	const secret, repo = "s3cr3t", "https://github.com/x/app"
	app := autoDeployApp("api", repo)
	app.Spec.Image = "zot.test/api:gen-4" // exact-image override left by Rollback
	app.Labels = map[string]string{
		core.LabelAppID:      "srv-x1",
		store.LabelManagedBy: store.ManagedByValue,
	}
	st := &recordingStore{}
	svc, cl := newService(st, app)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body := pushBody(t, repo, "refs/heads/main")
	if rec := postPush(t, h, secret, "push", body); rec.Code != http.StatusOK {
		t.Fatalf("push => %d: %s", rec.Code, rec.Body)
	}
	if len(st.imageCalls) != 1 || st.imageCalls[0].id != "srv-x1" || st.imageCalls[0].image != "" {
		t.Fatalf("stored image clears = %+v, want one empty-image write for srv-x1", st.imageCalls)
	}
	if got := getApp(t, cl, "api").Spec.Image; got != "" {
		t.Errorf("spec.image after push = %q, want cleared", got)
	}
	if len(st.deployCalls) != 1 || st.deployCalls[0].Image != "" {
		t.Fatalf("source deploy rows = %+v, want one row with empty image", st.deployCalls)
	}
}

// TestWebhookPushCROnlyAppSkipsDeployRow: a hand-applied App (no bex.co/app-id
// label) keeps the plain restartedAt bump — redeployed, no row, no error.
func TestWebhookPushCROnlyAppSkipsDeployRow(t *testing.T) {
	const repo = "https://github.com/x/app"
	st := &recordingStore{}
	svc, cl := newService(st, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, Secret: "shh"}

	redeployed, _, err := h.redeployMatching(context.Background(), newPush(repo, []string{"main.go"}), "main", "")
	if err != nil {
		t.Fatalf("redeployMatching: %v", err)
	}
	if !contains(redeployed, "api") {
		t.Fatalf("CR-only app must still redeploy: %v", redeployed)
	}
	if len(st.deployCalls) != 0 {
		t.Errorf("CR-only app must open no deploy row: %+v", st.deployCalls)
	}
	if getApp(t, cl, "api").Spec.RestartedAt == "" {
		t.Error("CR-only app must still bump restartedAt")
	}
}

// TestPushEventCommitInfo pins the wire→provenance lift: head_commit becomes
// the deploy row's CommitInfo, degrading field-by-field — absent head_commit
// is the zero value (the resolver fallback kicks in), a bad timestamp keeps
// hash+message and just drops authorAt. Offsets normalize to UTC.
func TestPushEventCommitInfo(t *testing.T) {
	var ev pushEvent
	payload := `{"ref":"refs/heads/main","head_commit":{"id":"abc123","message":"automated blog post","timestamp":"2026-07-19T08:58:44+02:00"}}`
	payload = `{"ref":"refs/heads/main","after":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","head_commit":{"id":"abc123","message":"automated blog post","timestamp":"2026-07-19T08:58:44+02:00"}}`
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatal(err)
	}
	info := ev.commitInfo()
	if info.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || info.Message != "automated blog post" {
		t.Errorf("commitInfo = {%s %q}, want the head commit", info.Hash, info.Message)
	}
	want := time.Date(2026, 7, 19, 6, 58, 44, 0, time.UTC)
	if info.AuthorAt == nil || !info.AuthorAt.Equal(want) {
		t.Errorf("authorAt = %v, want %v", info.AuthorAt, want)
	}

	if got := (pushEvent{}).commitInfo(); got.Hash != "" || got.Message != "" || got.AuthorAt != nil {
		t.Errorf("absent head_commit => zero CommitInfo, got %+v", got)
	}

	ev.HeadCommit.Timestamp = "not-a-time"
	info = ev.commitInfo()
	if info.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || info.AuthorAt != nil {
		t.Errorf("bad timestamp must keep hash and drop authorAt, got %+v", info)
	}
}
