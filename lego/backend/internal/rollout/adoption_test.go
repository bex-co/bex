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

package rollout

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// listingDeployStore adds the deploy history the Wait rule reads.
type listingDeployStore struct {
	fakeDeployStore
	open []store.Deploy
}

func (f *listingDeployStore) ListDeploys(_ context.Context, _ string, _ store.DeployFilter) ([]store.Deploy, error) {
	return f.open, nil
}

// adoptionFixture is a live service (release 1 adopted) whose next deploy,
// release 2, is executing. patchedAt records the release the operator had
// adopted when the tracked write reached the API server.
func adoptionFixture(t *testing.T, labels map[string]string) (client.Client, *appv1alpha1.App, *listingDeployStore, *atomic.Int64) {
	t.Helper()
	var patchedAt atomic.Int64
	patchedAt.Store(-1)
	cl, a := lockedPatchFixture(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			stored := &appv1alpha1.App{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), stored); err != nil {
				return err
			}
			if err := c.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			patchedAt.Store(stored.Status.ReleaseGeneration)
			return nil
		},
	})
	for k, v := range labels {
		a.Labels[k] = v
	}
	a.Status.ReleaseGeneration = 1
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	ds := &listingDeployStore{open: []store.Deploy{{ID: "dep-ahead", Status: store.DeployCreated, Generation: 2}}}
	return cl, a, ds, &patchedAt
}

// adoptAfter records the operator adopting release gen after d.
func adoptAfter(t *testing.T, cl client.Client, a *appv1alpha1.App, gen int64, d time.Duration) {
	t.Helper()
	key := client.ObjectKeyFromObject(a)
	go func() {
		time.Sleep(d)
		live := &appv1alpha1.App{}
		if err := cl.Get(context.Background(), key, live); err != nil {
			t.Error(err)
			return
		}
		live.Status.ReleaseGeneration = gen
		if err := cl.Update(context.Background(), live); err != nil {
			t.Error(err)
		}
	}()
}

func healthCheck(a *appv1alpha1.App) error {
	a.Spec.HealthCheckPath = "/healthz"
	return nil
}

// w8/390: a configuration save that rolls a release behind a deploy the
// operator has not adopted yet waits for that adoption, then writes through
// and pends behind the release in flight, instead of replacing it before the
// operator could pin it.
func TestARollingWriteWaitsForTheDeployAheadToBeAdopted(t *testing.T) {
	for name, patch := range trackerPatches {
		t.Run(name, func(t *testing.T) {
			cl, a, ds, patchedAt := adoptionFixture(t, nil)
			adoptAfter(t, cl, a, 2, 300*time.Millisecond)
			start := time.Now()
			if err := patch(&Tracker{Store: ds, AdoptionWait: 10 * time.Second}, context.Background(), cl, a, healthCheck); err != nil {
				t.Fatal(err)
			}
			if got := patchedAt.Load(); got != 2 || time.Since(start) < 250*time.Millisecond {
				t.Fatalf("the save reached the App with release %d adopted after %s; want it after the operator adopted release 2", got, time.Since(start))
			}
			if len(ds.created) != 1 || a.Spec.HealthCheckPath != "/healthz" {
				t.Fatalf("rows %d, healthCheckPath %q; want the save written and recorded once", len(ds.created), a.Spec.HealthCheckPath)
			}
		})
	}
}

// w8/390: the wait never refuses a save. One the operator does not adopt in
// time writes through as before; Render's Override policy replaces the
// deploy in flight at once; and a deploy the operator already adopted, a
// deploy waiting in the latest-pending slot behind a release in flight, or a
// first release (the operator has adopted nothing yet, and its first release
// reads the saved settings) does not hold it.
func TestARollingWriteDoesNotWaitWhereNothingCanBeReplaced(t *testing.T) {
	cases := map[string]struct {
		labels   map[string]string
		adopted  int64
		open     []store.Deploy
		wait     time.Duration
		minWait  time.Duration
		maxWait  time.Duration
		released int64
	}{
		"not adopted in time": {adopted: 1, wait: 300 * time.Millisecond, minWait: 300 * time.Millisecond, maxWait: 5 * time.Second, released: 1},
		"override policy": {labels: map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride},
			adopted: 1, wait: 10 * time.Second, maxWait: 2 * time.Second, released: 1},
		"already adopted": {adopted: 2, wait: 10 * time.Second, maxWait: 2 * time.Second, released: 2},
		"a deploy pending behind one in flight": {adopted: 2, open: []store.Deploy{
			{ID: "dep-ahead", Status: store.DeployBuildInProgress, Generation: 2},
			{ID: "dep-pending", Status: store.DeployQueued, Generation: 3, OverlapPending: true},
		}, wait: 10 * time.Second, maxWait: 2 * time.Second, released: 2},
		"first release": {adopted: 0, wait: 10 * time.Second, maxWait: 2 * time.Second, released: 0},
		"wait off":      {adopted: 1, wait: -1, maxWait: 2 * time.Second, released: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cl, a, ds, patchedAt := adoptionFixture(t, tc.labels)
			a.Status.ReleaseGeneration = tc.adopted
			if err := cl.Update(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if tc.open != nil {
				ds.open = tc.open
			}
			start := time.Now()
			if err := (&Tracker{Store: ds, AdoptionWait: tc.wait}).PatchLocked(context.Background(), cl, a, store.TriggerSettingsChange, healthCheck); err != nil {
				t.Fatal(err)
			}
			if took := time.Since(start); took < tc.minWait || took > tc.maxWait || patchedAt.Load() != tc.released {
				t.Fatalf("the save took %s and reached the App with release %d adopted; want %s–%s and %d", took, patchedAt.Load(), tc.minWait, tc.maxWait, tc.released)
			}
			if len(ds.created) != 1 {
				t.Fatalf("rows %d, want the save recorded once", len(ds.created))
			}
		})
	}
}

// w8/396: once the deploy ahead has ended, the deploy waiting in the
// latest-pending slot is the one in progress, and the operator adopts it on
// its next pass. A save written before then replaced it in the operator,
// although the deploy row had already moved on to it. The save waits for that
// adoption whether the reconciler has recorded the deploy ahead (no row is
// executing) or not yet (the App reports its release served).
func TestARollingWriteWaitsForTheWaitingDeployOnceTheDeployAheadEnded(t *testing.T) {
	cases := map[string][]store.Deploy{
		"recorded": {{ID: "dep-waiting", Status: store.DeployQueued, Generation: 2, OverlapPending: true}},
		"served, not yet recorded": {
			{ID: "dep-ahead", Status: store.DeployUpdateInProgress, Generation: 1},
			{ID: "dep-waiting", Status: store.DeployQueued, Generation: 2, OverlapPending: true},
		},
	}
	for name, open := range cases {
		t.Run(name, func(t *testing.T) {
			cl, a, ds, patchedAt := adoptionFixture(t, nil)
			a.Status.ActiveRevision = "rev-1"
			if err := cl.Update(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			ds.open = open
			adoptAfter(t, cl, a, 2, 300*time.Millisecond)
			start := time.Now()
			if err := (&Tracker{Store: ds, AdoptionWait: 10 * time.Second}).PatchLocked(context.Background(), cl, a, store.TriggerSettingsChange, healthCheck); err != nil {
				t.Fatal(err)
			}
			if got := patchedAt.Load(); got != 2 || time.Since(start) < 250*time.Millisecond {
				t.Fatalf("the save reached the App with release %d adopted after %s; want it after the operator adopted the waiting release 2", got, time.Since(start))
			}
		})
	}
}

// w8/390: a write that does not roll a release has nothing to replace and
// never waits.
func TestAWriteThatRollsNothingDoesNotWait(t *testing.T) {
	cl, a, ds, _ := adoptionFixture(t, nil)
	start := time.Now()
	err := (&Tracker{Store: ds, AdoptionWait: 10 * time.Second}).PatchLocked(context.Background(), cl, a, store.TriggerSettingsChange, func(a *appv1alpha1.App) error {
		a.Annotations = map[string]string{"example.com/note": "x"}
		return nil
	})
	if err != nil || time.Since(start) > 2*time.Second || len(ds.created) != 0 {
		t.Fatalf("err %v, took %s, rows %d; want an untracked write at once", err, time.Since(start), len(ds.created))
	}
}

// w8/390: a staged write (a Render-shaped service PATCH) waits on the
// operator's status, which only the API server's copy carries.
func TestAStagedRollingWriteWaitsOnTheLiveApp(t *testing.T) {
	cl, a, ds, _ := adoptionFixture(t, nil)
	stage := NewStage(cl, a)
	adoptAfter(t, cl, a, 2, 300*time.Millisecond)
	start := time.Now()
	staged := a.DeepCopy()
	if err := (&Tracker{Store: ds, AdoptionWait: 10 * time.Second}).Patch(context.Background(), stage, staged, store.TriggerSettingsChange, healthCheck); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 250*time.Millisecond || took > 5*time.Second {
		t.Fatalf("the staged save took %s; want it to wait for the live App's adoption", took)
	}
}
