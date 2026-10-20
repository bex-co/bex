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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func orderingBlueprintFixture(t *testing.T) (*Service, *fakeBlueprintStore, store.Blueprint, *advancingBlueprintFetcher) {
	t.Helper()
	b := store.Blueprint{ID: "bp-ordering", TenantID: "tea-a", Name: "ordering", Repo: "https://github.com/acme/app", Branch: "main", Path: CanonicalBlueprintFilename, AutoSync: true, Status: store.BlueprintStatusInSync, Manifest: "services: []\n"}
	svc, fs := ownershipService(t, b)
	git := &advancingBlueprintFetcher{headSHA: shaB, byCommit: map[string]string{
		shaA:                    strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:old"),
		shaB:                    strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:current"),
		strings.Repeat("c", 40): strings.ReplaceAll(ownershipManifest, "nginx:1", "nginx:later"),
	}}
	svc.GitFetcher = git
	return svc, fs, b, git
}

func claimOrderingIntent(t *testing.T, fs *fakeBlueprintStore, b store.Blueprint, commit, digest string) store.BlueprintAutoSyncIntent {
	t.Helper()
	if _, err := fs.EnqueueBlueprintAutoSyncIntent(context.Background(), store.BlueprintAutoSyncIntent{BlueprintID: b.ID, TenantID: b.TenantID, DeliveryDigest: digest, CommitSHA: commit, Path: b.Path}); err != nil {
		t.Fatal(err)
	}
	intents, err := fs.ClaimBlueprintAutoSyncIntents(context.Background(), 1, time.Minute)
	if err != nil || len(intents) != 1 {
		t.Fatalf("claim = %+v, %v", intents, err)
	}
	return intents[0]
}

func assertOrderingBlueprintApplied(t *testing.T, svc *Service, fs *fakeBlueprintStore, b store.Blueprint, git *advancingBlueprintFetcher, commit, image string) {
	t.Helper()
	live, err := fs.GetBlueprint(context.Background(), b.ID, b.TenantID)
	if err != nil || live.Manifest != git.byCommit[commit] {
		t.Fatalf("applied manifest = %q, err=%v; want current source %s", live.Manifest, err, commit)
	}
	app := getTenantApp(t, svc.Client, b.TenantID, "web")
	if app.Spec.Image != image {
		t.Fatalf("applied image = %q, want %q", app.Spec.Image, image)
	}
	if len(fs.insertedSyncs) == 0 || fs.insertedSyncs[len(fs.insertedSyncs)-1].CommitID != commit {
		t.Fatalf("sync history = %+v, want applied commit %s", fs.insertedSyncs, commit)
	}
}

func TestBlueprintAutoSyncDelayedIntentUsesCurrentBranch(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	intent := claimOrderingIntent(t, fs, b, shaA, "delayed-A")
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaB, "nginx:current")
	if git.resolves == 0 {
		t.Fatal("automatic sync never checked the authoritative branch")
	}
	run := fs.insertedSyncs[len(fs.insertedSyncs)-1]
	if !strings.Contains(run.Note, shaA) || !strings.Contains(run.Note, shaB) {
		t.Fatalf("sync note hides accepted/applied source difference: %q", run.Note)
	}
	for _, accepted := range fs.autoSyncIntents {
		if accepted.ID == intent.ID && (accepted.CommitSHA != shaA || accepted.State != store.BlueprintAutoSyncIntentCompleted) {
			t.Fatalf("source convergence rewrote or failed to settle accepted provenance: %+v", accepted)
		}
	}
}

func TestBlueprintAutoSyncReorderedDeliveriesDoNotRollback(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	w := &BlueprintAutoSyncWorker{Svc: svc}
	current := claimOrderingIntent(t, fs, b, shaB, "current-B")
	w.process(context.Background(), current)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaB, "nginx:current")
	older := claimOrderingIntent(t, fs, b, shaA, "late-A")
	w.process(context.Background(), older)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaB, "nginx:current")
	for _, commit := range git.fetched {
		if commit != shaB {
			t.Fatalf("reordered delivery fetched obsolete source %s; fetched=%v", commit, git.fetched)
		}
	}
	for _, run := range fs.insertedSyncs {
		if run.CommitID != shaB {
			t.Fatalf("automatic sync history attributed current manifest to %s", run.CommitID)
		}
	}
}

func TestBlueprintAutoSyncFollowsForcePushedBranch(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	git.headSHA = shaA
	intent := claimOrderingIntent(t, fs, b, shaB, "before-force-push")
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaA, "nginx:old")
}

func TestBlueprintAutoSyncPendingIntentFollowsHeadWithoutAnotherBlueprintChange(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	shaC := strings.Repeat("c", 40)
	// C changes application code only. Its unchanged Blueprint does not produce
	// another intent, so discarding B would lose the accepted configuration sync.
	git.headSHA = shaC
	git.byCommit[shaC] = git.byCommit[shaB]
	intent := claimOrderingIntent(t, fs, b, shaB, "blueprint-change-B")
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaC, "nginx:current")
	if !slices.Equal(git.fetched, []string{shaC}) {
		t.Fatalf("fetched=%v, want current immutable source C", git.fetched)
	}
}

func assertOrderingIntentRetryable(t *testing.T, svc *Service, fs *fakeBlueprintStore, b store.Blueprint, intent store.BlueprintAutoSyncIntent) {
	t.Helper()
	live, err := fs.GetBlueprint(context.Background(), b.ID, b.TenantID)
	if err != nil || live.Manifest != b.Manifest || live.ActiveRunID != "" {
		t.Fatalf("retryable source failure staged a manifest or stranded execution: %+v, %v", live, err)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("retryable source failure mutated workloads: %+v, %v", apps.Items, err)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, current := range fs.autoSyncIntents {
		if current.ID == intent.ID {
			if current.State != store.BlueprintAutoSyncIntentClaimed || current.CompletedAt != nil {
				t.Fatalf("retryable source failure consumed accepted work: %+v", current)
			}
			return
		}
	}
	t.Fatalf("accepted intent %s disappeared", intent.ID)
}

func TestBlueprintAutoSyncHeadAdvanceDuringFetchRetriesCurrentSource(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	shaC := strings.Repeat("c", 40)
	intent := claimOrderingIntent(t, fs, b, shaB, "head-advances")
	git.afterFetch = func() { git.headSHA = shaC }
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
	assertOrderingIntentRetryable(t, svc, fs, b, intent)
	git.afterFetch = nil
	// Recovery holds only the durable acceptance and claim, not a fetched body.
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaC, "nginx:later")
}

func TestBlueprintAutoSyncResolveFailureLeavesAcceptedWorkRecoverable(t *testing.T) {
	for _, phase := range []string{"before fetch", "after fetch"} {
		t.Run(phase, func(t *testing.T) {
			svc, fs, b, git := orderingBlueprintFixture(t)
			intent := claimOrderingIntent(t, fs, b, shaA, "source-outage")
			if phase == "before fetch" {
				git.resolveErr = errors.New("provider temporarily unavailable")
			} else {
				git.afterFetch = func() { git.resolveErr = errors.New("provider temporarily unavailable") }
			}
			(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
			assertOrderingIntentRetryable(t, svc, fs, b, intent)
			git.resolveErr = nil
			git.afterFetch = nil
			(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
			assertOrderingBlueprintApplied(t, svc, fs, b, git, shaB, "nginx:current")
		})
	}
}

func TestBlueprintManualReviewedSyncKeepsExactPinWhenBranchMoves(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	_, err := svc.SyncBlueprint(ownershipCtx(), b.ID, "", "", "", &ReviewedBlueprintSource{Repo: b.Repo, Path: b.Path, CommitID: shaA})
	if err != nil {
		t.Fatal(err)
	}
	assertOrderingBlueprintApplied(t, svc, fs, b, git, shaA, "nginx:old")
	if git.resolves != 0 || !slices.Equal(git.fetched, []string{shaA}) {
		t.Fatalf("reviewed pin followed branch instead: resolutions=%d fetched=%v", git.resolves, git.fetched)
	}
}

func TestBlueprintAutoSyncRevalidatesPolicyAndSourceAfterFetch(t *testing.T) {
	for _, change := range []string{"Auto Sync off", "path changed", "disconnected", "replaced"} {
		t.Run(change, func(t *testing.T) {
			svc, fs, b, git := orderingBlueprintFixture(t)
			intent := claimOrderingIntent(t, fs, b, shaB, "settings-race")
			git.afterFetch = func() {
				fs.mu.Lock()
				defer fs.mu.Unlock()
				live := fs.blueprints[b.ID]
				switch change {
				case "Auto Sync off":
					live.AutoSync = false
				case "path changed":
					live.Path = "ops/render.yaml"
				case "disconnected":
					live.Status = "disconnected"
				case "replaced":
					delete(fs.blueprints, b.ID)
					live.ID = "bp-replacement"
				}
				fs.blueprints[live.ID] = live
			}
			(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intent)
			var apps appv1alpha1.AppList
			if err := svc.Client.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
				t.Fatalf("stale fetched source mutated workloads after %s: %+v, %v", change, apps.Items, err)
			}
			for _, live := range fs.blueprints {
				if live.Manifest != b.Manifest || live.ActiveRunID != "" {
					t.Fatalf("stale fetched source changed Blueprint after %s: %+v", change, live)
				}
			}
			for _, run := range fs.syncs {
				if run.State == store.BlueprintSyncStateSuccess {
					t.Fatalf("stale fetched source reported success after %s: %+v", change, run)
				}
			}
		})
	}
}
