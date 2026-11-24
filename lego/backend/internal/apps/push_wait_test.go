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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// listingStore is recordingStore with the deploy history the Wait rule reads.
type listingStore struct {
	*recordingStore
	open []store.Deploy
}

func (s *listingStore) ListDeploys(context.Context, string, store.DeployFilter) ([]store.Deploy, error) {
	return s.open, nil
}

// w8/390: a git push behind a deploy the operator has not adopted yet waits
// at the row level (deploys.Service.DeferNewCommit, the rule every deploy
// trigger follows): the App, its rollback override and its history are left
// alone, so the deploy ahead runs. Behind nothing, the push writes through.
func TestAPushThatWaitsLeavesTheAppAlone(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, manage(autoDeployApp("web", "https://github.com/acme/web"), "srv-web"))
	before := getApp(t, cl, "web")
	var waited []string
	waits := true
	svc.DeferNewCommit = func(_ context.Context, a *appv1alpha1.App, commit store.CommitInfo) (bool, error) {
		waited = append(waited, a.Name+"@"+commit.Hash)
		return waits, nil
	}
	if _, err := svc.redeployFetched(context.Background(), getApp(t, cl, "web"), store.CommitInfo{Hash: shaA}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "web"); got.ResourceVersion != before.ResourceVersion || len(rec.deployCalls)+len(rec.imageCalls) != 0 {
		t.Fatalf("a waiting push wrote the App (buildCommit %q), %d deploy rows and %d image writes; want none", got.Spec.BuildCommit, len(rec.deployCalls), len(rec.imageCalls))
	}
	if len(waited) != 1 || waited[0] != "web@"+shaA {
		t.Fatalf("waiting deploys = %v, want web's push of %s", waited, shaA)
	}

	waits = false
	if _, err := svc.redeployFetched(context.Background(), getApp(t, cl, "web"), store.CommitInfo{Hash: shaA}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "web"); got.Spec.BuildCommit != shaA || len(rec.deployCalls) != 1 || rec.deployCalls[0].Trigger != store.TriggerNewCommit {
		t.Fatalf("a push behind nothing: buildCommit %q, rows %+v; want it written through", got.Spec.BuildCommit, rec.deployCalls)
	}
}

// w8/390: a checksPass push whose checks passed behind a deploy the operator
// has not adopted stays pending, and the recovery poll dispatches it once
// that deploy is adopted, to pend behind it.
func TestACheckedPushBehindAnUnadoptedDeployStaysPending(t *testing.T) {
	ls := &listingStore{recordingStore: &recordingStore{}, open: []store.Deploy{{ID: "dep-ahead", Status: store.DeployCreated, Generation: 2}}}
	a := checksApp("api", checksRepo, shaA)
	a.Generation = 2
	a.Labels = map[string]string{core.LabelAppID: "srv-check", store.LabelManagedBy: store.ManagedByValue}
	a.Status.ReleaseGeneration = 1
	svc, cl := newService(ls, a)
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	w := &CheckedDeployWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api"); got.Spec.BuildCommit != "" || got.Annotations[annotChecksPendingCommit] != shaA || len(ls.deployCalls) != 0 {
		t.Fatalf("checked push behind a deploy not adopted yet: buildCommit %q, pending %q, rows %d; want it still pending",
			got.Spec.BuildCommit, got.Annotations[annotChecksPendingCommit], len(ls.deployCalls))
	}

	adopted := getApp(t, cl, "api")
	adopted.Status.ReleaseGeneration = 2
	if err := cl.Update(context.Background(), adopted); err != nil {
		t.Fatal(err)
	}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api"); got.Spec.BuildCommit != shaA || got.Annotations[annotChecksPendingCommit] != "" {
		t.Fatalf("checked push once the deploy ahead was adopted: buildCommit %q, pending %q; want it dispatched",
			got.Spec.BuildCommit, got.Annotations[annotChecksPendingCommit])
	}
}

// w8/390: a Blueprint sync that changes a service behind a deploy the
// operator has not adopted yet waits for that adoption, then writes its
// release, which pends behind the deploy in flight instead of replacing it.
func TestABlueprintSyncWaitsForTheDeployAheadToBeAdopted(t *testing.T) {
	ls := &listingStore{recordingStore: &recordingStore{}, open: []store.Deploy{{ID: "dep-ahead", Status: store.DeployCreated, Generation: 2}}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	existing.Generation = 2
	existing.Status.ReleaseGeneration = 1
	svc, cl := newService(ls, existing)
	var patchedAt atomic.Int64
	svc.Client = interceptor.NewClient(cl.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			stored := &appv1alpha1.App{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), stored); err != nil {
				return err
			}
			patchedAt.Store(stored.Status.ReleaseGeneration)
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	go func() {
		time.Sleep(300 * time.Millisecond)
		adopted := &appv1alpha1.App{}
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(existing), adopted); err != nil {
			t.Error(err)
			return
		}
		adopted.Status.ReleaseGeneration = 2
		if err := cl.Update(context.Background(), adopted); err != nil {
			t.Error(err)
		}
	}()
	start := time.Now()
	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "web"); got.Spec.Image != "new:1" || len(ls.deployCalls) != 1 || ls.deployCalls[0].Trigger != store.TriggerBlueprintSync {
		t.Fatalf("sync: image %q, rows %+v; want the changed service redeployed once", got.Spec.Image, ls.deployCalls)
	}
	if took := time.Since(start); patchedAt.Load() != 2 || took < 250*time.Millisecond {
		t.Fatalf("the sync reached the App after %s with release %d adopted; want it after the operator adopted release 2", took, patchedAt.Load())
	}
}

// w8/396: a push, a checked push and a Blueprint sync that write through name
// the release the App reports adopted to the row they open, so the row waits
// behind a waiting deploy the operator already runs instead of replacing it.
func TestWritesThroughNameTheReleaseTheAppAdopted(t *testing.T) {
	ctx := context.Background()
	named := func(t *testing.T, rec *recordingStore, want int64) {
		t.Helper()
		if len(rec.adopted) != 1 || rec.adopted[0] != want {
			t.Fatalf("adopted releases named = %v, want [%d]", rec.adopted, want)
		}
	}
	t.Run("push", func(t *testing.T) {
		rec := &recordingStore{}
		a := manage(autoDeployApp("web", "https://github.com/acme/web"), "srv-web")
		a.Status.ReleaseGeneration = 3
		svc, cl := newService(rec, a)
		if _, err := svc.redeployFetched(ctx, getApp(t, cl, "web"), store.CommitInfo{Hash: shaA}); err != nil {
			t.Fatal(err)
		}
		named(t, rec, 3)
	})
	t.Run("checked push", func(t *testing.T) {
		ls := &listingStore{recordingStore: &recordingStore{}}
		a := checksApp("api", checksRepo, shaA)
		a.Generation = 2
		a.Labels = map[string]string{core.LabelAppID: "srv-check", store.LabelManagedBy: store.ManagedByValue}
		a.Status.ReleaseGeneration = 2
		svc, _ := newService(ls, a)
		svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
		if err := (&CheckedDeployWorker{Svc: svc}).tick(ctx); err != nil {
			t.Fatal(err)
		}
		named(t, ls.recordingStore, 2)
	})
	t.Run("blueprint sync", func(t *testing.T) {
		ls := &listingStore{recordingStore: &recordingStore{}}
		existing := managedApp("web", "srv-1")
		existing.Spec.Image = "old:1"
		existing.Generation = 2
		existing.Status.ReleaseGeneration = 2
		svc, _ := newService(ls, existing)
		changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		named(t, ls.recordingStore, 2)
	})
}

// w6/163: Render's Update Service (REST PATCH /v1/services/{id}, MCP
// update_service) saved within seconds of a dashboard Settings save that
// deploys waits for the operator to adopt that deploy before it holds its own
// change for the next deploy. Held first, the change held that deploy too:
// the operator releases no deploy stamped below a pending save, so it stayed
// created until another deploy canceled it. A saved setting and a saved
// source (Update Source) wait alike, and the deploy ahead keeps its stamp. A
// plan saves only too (w6/m609), so it waits alike and stays pending behind
// that deploy.
func TestAnUpdateServiceSaveWaitsForTheDeployAheadToBeAdopted(t *testing.T) {
	delay, image, plan := int32(120), "new:1", "standard"
	for name, patch := range map[string]ServicePatch{
		"setting": {MaxShutdownDelaySeconds: &delay},
		"source":  {Image: &image},
		"plan":    {Plan: &plan},
	} {
		t.Run(name, func(t *testing.T) {
			ls := &listingStore{recordingStore: &recordingStore{}, open: []store.Deploy{{ID: "dep-settings", Status: store.DeployCreated, Generation: 3}}}
			a := managedApp("web", "srv-1")
			a.Spec.Image = "old:1"
			a.Generation = 3
			a.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "3"}
			a.Status.ReleaseGeneration = 1
			svc, cl := newService(ls, a)
			var mu sync.Mutex
			var writtenAt []int64
			svc.Client = interceptor.NewClient(cl.(client.WithWatch), interceptor.Funcs{
				Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					stored := &appv1alpha1.App{}
					if err := c.Get(ctx, client.ObjectKeyFromObject(obj), stored); err != nil {
						return err
					}
					// The preflight's admission dry-run writes nothing.
					var po client.PatchOptions
					if po.ApplyOptions(opts); len(po.DryRun) == 0 {
						mu.Lock()
						writtenAt = append(writtenAt, stored.Status.ReleaseGeneration)
						mu.Unlock()
					}
					return c.Patch(ctx, obj, patch, opts...)
				},
			})
			go func() {
				time.Sleep(300 * time.Millisecond)
				adopted := getApp(t, cl, "web")
				adopted.Status.ReleaseGeneration = 3
				if err := cl.Update(context.Background(), adopted); err != nil {
					t.Error(err)
				}
			}()
			start := time.Now()
			if _, err := svc.ApplyServicePatch(context.Background(), "web", patch); err != nil {
				t.Fatalf("ApplyServicePatch: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if took := time.Since(start); len(writtenAt) == 0 || took < 250*time.Millisecond {
				t.Fatalf("the save wrote the App %d times after %s; want it written after the operator adopted release 3", len(writtenAt), took)
			}
			for _, gen := range writtenAt {
				if gen != 3 {
					t.Fatalf("the save wrote the App with releases %v adopted; want every write after release 3 was", writtenAt)
				}
			}
			got := getApp(t, cl, "web")
			if got.Annotations[appv1alpha1.AnnotationReleaseGeneration] != "3" || got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] != "4" || len(ls.deployCalls) != 0 {
				t.Fatalf("annotations %v, rows %d; want release 3 still stamped and the save pending at 4, with no deploy of its own", got.Annotations, len(ls.deployCalls))
			}
			if patch.Plan != nil && got.Spec.Tier != plan {
				t.Fatalf("spec.tier = %q, want the saved %q", got.Spec.Tier, plan)
			}
		})
	}
}
