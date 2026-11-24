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
	"slices"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func gitBuildSourceCarrier(id, tenant, repo, branch string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:       id,
			Namespace:  "default",
			Generation: 2,
			Labels: map[string]string{
				core.LabelTenant:     tenant,
				core.LabelAppID:      id,
				store.LabelManagedBy: store.ManagedByValue,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: "Mono",
			Repo:        repo,
			Branch:      branch,
			Runtime:     "node",
			AutoDeploy:  true,
		},
	}
}

func imageBuildSourceCarrier(id, tenant, image string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:       id,
			Namespace:  "default",
			Generation: 2,
			Labels: map[string]string{
				core.LabelTenant:     tenant,
				core.LabelAppID:      id,
				store.LabelManagedBy: store.ManagedByValue,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: "ImgSource",
			Image:       image,
			AutoDeploy:  false,
		},
	}
}

type countingStartedNotifier struct {
	calls atomic.Int32
}

func (n *countingStartedNotifier) NotifyDeployStarted(context.Context, string, string, string) {
	n.calls.Add(1)
}

type stubBillingGuard struct {
	err error
}

func (g stubBillingGuard) CheckBillingMutationAllowed(context.Context, string) error { return g.err }

// w2/m610: signed pushes reach git-backed Build Source carriers, honor branch,
// rootDir, buildFilter (paths + ignoredPaths), Render skip phrases, and billing
// enforcement, ship staged env-group links, ignore image-backed Build Sources,
// and never leak service-only commit_ignored facts or deploy-started emails.
func TestWebhookBuildSourceGitPushMatchingAndFilters(t *testing.T) {
	const (
		secret = "s3cr3t"
		repo   = "https://github.com/acme/mono"
		sha    = "3597548b2424b9dee1ce4b07820b0f8cb007f747"
	)

	makePush := func(ref, msg string, paths []string) []byte {
		t.Helper()
		b, err := json.Marshal(map[string]any{
			"ref":        ref,
			"after":      sha,
			"repository": map[string]string{"clone_url": repo},
			"commits":    []map[string]any{{"modified": paths}},
			"head_commit": map[string]string{
				"id":        sha,
				"url":       repo + "/commit/" + sha,
				"message":   msg,
				"timestamp": "2026-11-23T12:00:00Z",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("matching push builds carrier, records new_commit with empty scope, ships staged env groups, and skips service deploy email", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-git1", "tea-a", repo, "main")
		carrier.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{
			Paths:        []string{"services/**"},
			IgnoredPaths: []string{"**/*.md"},
		}
		rollout.StageGroupLink(carrier, "evg-shared-env", "evg-shared-files", true)
		imgCarrier := imageBuildSourceCarrier("bds-img1", "tea-a", "ghcr.io/acme/app:v1")

		st := &recordingStore{}
		svc, cl := newService(st, carrier, imgCarrier)
		notifier := &countingStartedNotifier{}
		svc.StartedNotifier = notifier
		facts := &recordingFactWriter{}
		svc.EventFacts = facts
		h := &GitWebhook{Svc: svc, Secret: secret}

		rec := postPush(t, h, secret, "push", makePush("refs/heads/main", "feat: update api", []string{"services/api/main.go"}))
		if rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}

		if len(st.deployCalls) != 1 {
			t.Fatalf("carrier deploy calls = %d, want 1: %+v", len(st.deployCalls), st.deployCalls)
		}
		d := st.deployCalls[0]
		if d.AppID != "bds-git1" || d.Trigger != store.TriggerNewCommit || d.Commit != sha || d.CommitMessage != "feat: update api" || d.BuildDeployScope != "" {
			t.Fatalf("carrier deploy row = %+v, want bds-git1 new_commit commit=%s empty build_deploy_scope", d, sha)
		}
		got := getApp(t, cl, "bds-git1")
		if got.Spec.BuildCommit != sha || got.Spec.RestartedAt == "" {
			t.Fatalf("carrier CR after push: buildCommit=%q restartedAt=%q", got.Spec.BuildCommit, got.Spec.RestartedAt)
		}
		if !slices.Contains(got.Spec.EnvFromSecrets, "evg-shared-env") || got.Annotations[appv1alpha1.AnnotationPendingGroupLinks] != "" {
			t.Fatalf("staged group link not shipped on push: envFrom=%v pending=%q", got.Spec.EnvFromSecrets, got.Annotations[appv1alpha1.AnnotationPendingGroupLinks])
		}
		if getApp(t, cl, "bds-img1").Spec.RestartedAt != "" {
			t.Fatal("image-backed build source must ignore git push")
		}
		if len(facts.facts) != 0 {
			t.Fatalf("unexpected service event facts: %+v", facts.facts)
		}
		if notifier.calls.Load() != 0 {
			t.Fatalf("carrier push must not fire service deploy-started notification; got %d", notifier.calls.Load())
		}
	})

	t.Run("git-backed carrier with AutoDeploy=false still auto-builds on push", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-legacy", "tea-a", repo, "main")
		carrier.Spec.AutoDeploy = false
		st := &recordingStore{}
		svc, _ := newService(st, carrier)
		h := &GitWebhook{Svc: svc, Secret: secret}

		rec := postPush(t, h, secret, "push", makePush("refs/heads/main", "build me", []string{"main.go"}))
		if rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
		if len(st.deployCalls) != 1 || st.deployCalls[0].AppID != "bds-legacy" {
			t.Fatalf("deploy calls = %+v, want 1 for bds-legacy", st.deployCalls)
		}
	})

	negatives := []struct {
		name    string
		setup   func(c *appv1alpha1.App, svc *Service)
		ref     string
		message string
		paths   []string
	}{
		{
			name:    "wrong branch",
			ref:     "refs/heads/develop",
			message: "feat: other branch",
			paths:   []string{"services/api/main.go"},
		},
		{
			name: "buildFilter.paths miss",
			setup: func(c *appv1alpha1.App, _ *Service) {
				c.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"services/**"}}
			},
			ref:     "refs/heads/main",
			message: "feat: web",
			paths:   []string{"frontend/app.tsx"},
		},
		{
			name: "buildFilter.ignoredPaths hit",
			setup: func(c *appv1alpha1.App, _ *Service) {
				c.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{
					Paths:        []string{"services/**"},
					IgnoredPaths: []string{"**/*.md"},
				}
			},
			ref:     "refs/heads/main",
			message: "docs: readme",
			paths:   []string{"services/README.md"},
		},
		{
			name: "rootDir miss",
			setup: func(c *appv1alpha1.App, _ *Service) {
				c.Spec.RootDir = "backend"
			},
			ref:     "refs/heads/main",
			message: "feat: mobile",
			paths:   []string{"mobile/App.tsx"},
		},
		{
			name:    "skip render phrase",
			ref:     "refs/heads/main",
			message: "chore: release notes [skip render]",
			paths:   []string{"services/api/main.go"},
		},
		{
			name:    "skip cd phrase case-insensitive",
			ref:     "refs/heads/main",
			message: "chore: tooling [CD SKIP]",
			paths:   []string{"services/api/main.go"},
		},
		{
			name: "billing enforced workspace",
			setup: func(_ *appv1alpha1.App, svc *Service) {
				svc.Billing = stubBillingGuard{err: core.ErrBillingEnforced}
			},
			ref:     "refs/heads/main",
			message: "feat: unpaid",
			paths:   []string{"services/api/main.go"},
		},
		{
			name: "image-backed build source even with stray repo",
			setup: func(c *appv1alpha1.App, _ *Service) {
				c.Spec.Image = "ghcr.io/acme/app:v1"
			},
			ref:     "refs/heads/main",
			message: "feat: image carrier",
			paths:   []string{"services/api/main.go"},
		},
	}

	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			carrier := gitBuildSourceCarrier("bds-neg", "tea-a", repo, "main")
			st := &recordingStore{}
			svc, cl := newService(st, carrier)
			facts := &recordingFactWriter{}
			svc.EventFacts = facts
			if tc.setup != nil {
				tc.setup(carrier, svc)
				if err := cl.Update(context.Background(), carrier); err != nil {
					t.Fatal(err)
				}
			}
			h := &GitWebhook{Svc: svc, Secret: secret}

			rec := postPush(t, h, secret, "push", makePush(tc.ref, tc.message, tc.paths))
			if rec.Code != http.StatusOK {
				t.Fatalf("push => %d: %s", rec.Code, rec.Body)
			}
			if len(st.deployCalls) != 0 {
				t.Fatalf("expected 0 carrier deploy calls, got %+v", st.deployCalls)
			}
			if getApp(t, cl, "bds-neg").Spec.RestartedAt != "" {
				t.Fatalf("carrier CR should not be bumped on skipped push")
			}
			if len(facts.facts) != 0 {
				t.Fatalf("skipped carrier push must not write service_event_facts, got %+v", facts.facts)
			}
		})
	}
}

// w2/m610: GitHub App-signed pushes follow renamed repositories by GitHub
// repository ID on git-backed Build Sources and stamp the ID on the first
// URL-matched push.
func TestWebhookBuildSourceFollowsRenamedRepoByGitHubID(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	t.Run("follows rename by signed repo id", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-mv", "tea-a", renamedFrom, "main")
		core.SetGitHubRepoID(carrier, renamedID)
		st := newRepoIDStore(map[string]repoIDRow{"bds-mv": {repo: renamedFrom, id: renamedID}})
		svc, cl := newService(st, carrier)
		h := &GitWebhook{
			Svc:           svc,
			GitHubSecret:  renameSecret,
			Multitenant:   true,
			Installations: fakeInstallations{100: {"tea-a"}},
		}

		body, err := json.Marshal(map[string]any{
			"ref":          "refs/heads/main",
			"after":        sha,
			"installation": map[string]int64{"id": 100},
			"repository": map[string]any{
				"id":        renamedID,
				"clone_url": renamedTo + ".git",
				"html_url":  renamedTo,
			},
			"commits": []map[string]any{{"modified": []string{"main.go"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if rec := postPush(t, h, renameSecret, "push", body); rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
		if got := getApp(t, cl, "bds-mv"); got.Spec.Repo != renamedTo || got.Spec.BuildCommit != sha {
			t.Fatalf("carrier after renamed push: repo=%q buildCommit=%q, want %q / %q", got.Spec.Repo, got.Spec.BuildCommit, renamedTo, sha)
		}
		if st.rows["bds-mv"].repo != renamedTo || st.moves != 1 {
			t.Fatalf("store row = %+v moves=%d, want moved to %s", st.rows["bds-mv"], st.moves, renamedTo)
		}
		if len(st.deployCalls) != 1 || st.deployCalls[0].AppID != "bds-mv" {
			t.Fatalf("deploy calls = %+v, want 1 for bds-mv", st.deployCalls)
		}
	})

	t.Run("learns repo id on first URL-matched GitHub App push", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-learn", "tea-a", renamedFrom, "main")
		st := newRepoIDStore(map[string]repoIDRow{"bds-learn": {repo: renamedFrom, id: 0}})
		svc, _ := newService(st, carrier)
		h := &GitWebhook{
			Svc:           svc,
			GitHubSecret:  renameSecret,
			Multitenant:   true,
			Installations: fakeInstallations{100: {"tea-a"}},
		}
		body, _ := json.Marshal(map[string]any{
			"ref":          "refs/heads/main",
			"after":        sha,
			"installation": map[string]int64{"id": 100},
			"repository": map[string]any{
				"id":        renamedID,
				"clone_url": renamedFrom + ".git",
				"html_url":  renamedFrom,
			},
			"commits": []map[string]any{{"modified": []string{"main.go"}}},
		})
		if rec := postPush(t, h, renameSecret, "push", body); rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
		if st.rows["bds-learn"].id != renamedID {
			t.Fatalf("learned repo id = %d, want %d", st.rows["bds-learn"].id, renamedID)
		}
	})
}

// w2/m610: switching an image-backed Build Source to git enables AutoDeploy on
// the carrier CR (and switching back to image clears it).
func TestWebhookBuildSourceUpdateImageToGitSetsAutoDeploy(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	created, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a",
		Name:    "Switchable",
		Image:   &BuildSourceImage{ImageURL: "nginx:1.27"},
	})
	if err != nil {
		t.Fatal(err)
	}
	readCarrier := func() *appv1alpha1.App {
		t.Helper()
		_, a, err := svc.buildSourceApp(ctx, core.RelCanView, created.ID)
		if err != nil {
			t.Fatalf("buildSourceApp(%s): %v", created.ID, err)
		}
		return a
	}
	if readCarrier().Spec.AutoDeploy {
		t.Fatal("image-backed build source must start with AutoDeploy=false")
	}

	if _, err := svc.UpdateBuildSource(ctx, created.ID, UpdateBuildSourceRequest{
		Git: &BuildSourceGit{RepoURL: "https://github.com/acme/mono", Branch: "main", Runtime: "docker"},
	}); err != nil {
		t.Fatalf("UpdateBuildSource image->git: %v", err)
	}
	if !readCarrier().Spec.AutoDeploy {
		t.Fatal("switching image build source to git must set AutoDeploy=true")
	}

	if _, err := svc.UpdateBuildSource(ctx, created.ID, UpdateBuildSourceRequest{
		Image: &BuildSourceImage{ImageURL: "nginx:1.28"},
	}); err != nil {
		t.Fatalf("UpdateBuildSource git->image: %v", err)
	}
	if readCarrier().Spec.AutoDeploy {
		t.Fatal("switching git build source to image must clear AutoDeploy")
	}
}

// w2/m610: branch deletion neither disables AutoDeploy on a Build Source
// carrier nor emits service-only branch_deleted facts (so recreating the branch
// resumes auto-builds), while ordinary services keep branch_deleted handling;
// and checksPass never enlists or mutates Build Source carriers.
func TestWebhookBuildSourceBranchDeleteAndChecksPassIsolation(t *testing.T) {
	const (
		secret = "s3cr3t"
		repo   = "https://github.com/acme/mono"
		sha    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	t.Run("branch delete ignores carrier, allocates no replay claim for carrier-only repo, and keeps future pushes working", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-del", "tea-a", repo, "main")
		st := &recordingStore{}
		svc, cl := newService(st, carrier)
		facts := &recordingFactWriter{}
		svc.EventFacts = facts
		replays := &fakeReplayGuard{}
		h := &GitWebhook{Svc: svc, Secret: secret, Replays: replays}

		delBody, _ := json.Marshal(map[string]any{
			"ref":        "main",
			"ref_type":   "branch",
			"repository": map[string]string{"clone_url": repo},
		})
		if rec := postPushDelivery(t, h, secret, "delete", "del-1", delBody); rec.Code != http.StatusOK {
			t.Fatalf("delete event => %d: %s", rec.Code, rec.Body)
		}
		pushDelBody, _ := json.Marshal(map[string]any{
			"ref":        "refs/heads/main",
			"deleted":    true,
			"after":      "0000000000000000000000000000000000000000",
			"repository": map[string]string{"clone_url": repo},
		})
		if rec := postPushDelivery(t, h, secret, "push", "del-2", pushDelBody); rec.Code != http.StatusOK {
			t.Fatalf("push delete => %d: %s", rec.Code, rec.Body)
		}
		if !getApp(t, cl, "bds-del").Spec.AutoDeploy {
			t.Fatal("branch deletion must not disable AutoDeploy on a Build Source carrier")
		}
		if len(facts.facts) != 0 {
			t.Fatalf("branch deletion on carrier must not emit branch_deleted facts: %+v", facts.facts)
		}
		if replays.claimed() != 0 {
			t.Fatalf("carrier-only branch deletion must not claim replay ledger; claimed=%d", replays.claimed())
		}

		// Recreated branch push triggers a build cleanly.
		recreateBody, _ := json.Marshal(map[string]any{
			"ref":        "refs/heads/main",
			"after":      sha,
			"repository": map[string]string{"clone_url": repo},
			"commits":    []map[string]any{{"modified": []string{"main.go"}}},
		})
		if rec := postPushDelivery(t, h, secret, "push", "push-after-recreate", recreateBody); rec.Code != http.StatusOK {
			t.Fatalf("push after recreate => %d: %s", rec.Code, rec.Body)
		}
		if len(st.deployCalls) != 1 || st.deployCalls[0].AppID != "bds-del" {
			t.Fatalf("recreated branch push should build carrier, got %+v", st.deployCalls)
		}
	})

	t.Run("branch delete still disables AutoDeploy and emits fact for a sibling service sharing the repo", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-sib", "tea-a", repo, "main")
		web := autoDeployApp("web", repo)
		web.Labels = map[string]string{
			core.LabelTenant:     "tea-a",
			core.LabelAppID:      "srv-web",
			store.LabelManagedBy: store.ManagedByValue,
		}
		svc, cl := newService(nil, carrier, web)
		facts := &recordingFactWriter{}
		svc.EventFacts = facts
		h := &GitWebhook{Svc: svc, Secret: secret}

		delBody, _ := json.Marshal(map[string]any{
			"ref":        "main",
			"ref_type":   "branch",
			"repository": map[string]string{"clone_url": repo},
		})
		if rec := postPushDelivery(t, h, secret, "delete", "del-sib", delBody); rec.Code != http.StatusOK {
			t.Fatalf("delete => %d: %s", rec.Code, rec.Body)
		}
		if getApp(t, cl, "web").Spec.AutoDeploy {
			t.Error("sibling service should still have AutoDeploy disabled on branch delete")
		}
		if !getApp(t, cl, "bds-sib").Spec.AutoDeploy {
			t.Error("carrier must keep AutoDeploy enabled on branch delete")
		}
		if len(facts.facts) != 1 {
			t.Fatalf("want 1 branch_deleted fact (for srv-web only), got %+v", facts.facts)
		}
		for _, f := range facts.facts {
			if f.AppID != "srv-web" || f.Type != store.EventFactBranchDeleted {
				t.Fatalf("unexpected fact: %+v", f)
			}
		}
	})

	t.Run("checksPass never enlists or mutates a Build Source carrier", func(t *testing.T) {
		carrier := gitBuildSourceCarrier("bds-chk", "tea-a", repo, "main")
		carrier.Spec.AutoDeployTrigger = checksPassTrigger
		st := &recordingStore{}
		svc, cl := newService(st, carrier)
		svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
		h := &GitWebhook{
			Svc:           svc,
			GitHubSecret:  secret,
			Installations: fakeInstallations{100: {"tea-a"}},
		}

		// Push builds carrier directly without entering checksPass pending state.
		pushBody, _ := json.Marshal(map[string]any{
			"ref":          "refs/heads/main",
			"after":        sha,
			"installation": map[string]int64{"id": 100},
			"repository":   map[string]any{"clone_url": repo},
			"commits":      []map[string]any{{"modified": []string{"main.go"}}},
		})
		if rec := postPush(t, h, secret, "push", pushBody); rec.Code != http.StatusOK {
			t.Fatalf("push => %d: %s", rec.Code, rec.Body)
		}
		got := getApp(t, cl, "bds-chk")
		if got.Annotations[annotChecksPendingCommit] != "" || got.Annotations[annotChecksPendingSource] != "" {
			t.Fatalf("carrier entered checksPass pending state: %+v", got.Annotations)
		}
		if len(st.deployCalls) != 1 || st.deployCalls[0].AppID != "bds-chk" {
			t.Fatalf("carrier with stray checksPass trigger should still build directly: %+v", st.deployCalls)
		}

		// Even if pre-annotated, serveCheckSuite and CheckedDeployWorker ignore carriers.
		got.Annotations[annotChecksPendingCommit] = sha
		if err := cl.Update(context.Background(), got); err != nil {
			t.Fatal(err)
		}
		suiteBody, _ := json.Marshal(map[string]any{
			"action":       "completed",
			"installation": map[string]int64{"id": 100},
			"repository":   map[string]any{"clone_url": repo},
			"check_suite":  map[string]any{"head_sha": sha},
		})
		if rec := postPush(t, h, secret, "check_suite", suiteBody); rec.Code != http.StatusOK {
			t.Fatalf("check_suite => %d: %s", rec.Code, rec.Body)
		}
		if err := (&CheckedDeployWorker{Svc: svc}).tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(st.deployCalls) != 1 {
			t.Fatalf("serveCheckSuite/CheckedDeployWorker must not open a second deploy on a carrier; calls=%d", len(st.deployCalls))
		}
	})
}
