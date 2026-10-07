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
	"errors"
	"fmt"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeDeployStore records CreateDeploy arguments and returns a configured prior
// commit from LatestDeployCommit — enough to unit-test Tracker.open's carry.
type fakeDeployStore struct {
	prior   store.CommitInfo
	created []store.Deploy
}

func (f *fakeDeployStore) LatestDeployCommit(_ context.Context, _ string) (store.CommitInfo, error) {
	return f.prior, nil
}

func (f *fakeDeployStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string) (store.Deploy, error) {
	d := store.Deploy{
		ID: "dep-recorded", AppID: appID, Trigger: trigger, Image: image, Generation: generation,
		Commit: commit.Hash, CommitMessage: commit.Message, CommitAuthorAt: commit.AuthorAt, TriggeredBy: triggeredBy,
	}
	f.created = append(f.created, d)
	return d, nil
}

func openConfigChange(t *testing.T, ds *fakeDeployStore) {
	t.Helper()
	tr := &Tracker{Store: ds}
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 2},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	tr.open(context.Background(), Snapshot{appID: "srv-web", generation: 1, rolls: true}, a, store.TriggerConfigChange)
	if len(ds.created) != 1 {
		t.Fatalf("CreateDeploy calls = %d, want 1", len(ds.created))
	}
}

// TestOpenCarriesPriorCommit: a config_change re-roll of a repo-backed release
// must record the same commit id+message(+authorAt) the prior deploy carried.
func TestOpenCarriesPriorCommit(t *testing.T) {
	authorAt := time.Date(2026, 9, 13, 9, 35, 0, 0, time.UTC)
	prior := store.CommitInfo{Hash: "5ef5e18799fa7edbc6477ca3128c78686833b06b", Message: "update", AuthorAt: &authorAt}
	ds := &fakeDeployStore{prior: prior}
	openConfigChange(t, ds)
	got := ds.created[0]
	if got.Commit != prior.Hash || got.CommitMessage != prior.Message {
		t.Fatalf("carried commit = %q/%q, want %q/%q", got.Commit, got.CommitMessage, prior.Hash, prior.Message)
	}
	if got.CommitAuthorAt == nil || !got.CommitAuthorAt.Equal(authorAt) {
		t.Fatalf("carried authorAt = %v, want %v", got.CommitAuthorAt, authorAt)
	}
	if got.Trigger != store.TriggerConfigChange {
		t.Errorf("trigger = %q, want %q", got.Trigger, store.TriggerConfigChange)
	}
}

// TestOpenKeepsEmptyCommitWithoutPrior: image-backed services (no prior commit
// row) must still open an empty-commit config_change — never synthesize one.
func TestOpenKeepsEmptyCommitWithoutPrior(t *testing.T) {
	ds := &fakeDeployStore{}
	openConfigChange(t, ds)
	got := ds.created[0]
	if got.Commit != "" || got.CommitMessage != "" || got.CommitAuthorAt != nil {
		t.Fatalf("commit = %+v, want empty CommitInfo for image-backed / no prior", got)
	}
}

func lockedPatchFixture(t *testing.T, funcs interceptor.Funcs) (client.Client, *appv1alpha1.App) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{
			store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-web",
		}},
		Spec: appv1alpha1.AppSpec{Image: "web:v1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithInterceptorFuncs(funcs).Build()
	read := &appv1alpha1.App{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), read); err != nil {
		t.Fatal(err)
	}
	return cl, read
}

// TestPatchLockedDecidesAgainAfterAConflict (w5/m131): a write that loses the
// App race re-reads the App and runs its mutate again, so its patch lands on
// what the winner left instead of overwriting it, and the rollout it requests
// is recorded once.
func TestPatchLockedDecidesAgainAfterAConflict(t *testing.T) {
	cl, read := lockedPatchFixture(t, interceptor.Funcs{})
	winner := read.DeepCopy()
	winner.Spec.FilesFromSecrets = []string{"web-files"}
	if err := cl.Update(context.Background(), winner); err != nil {
		t.Fatal(err)
	}
	ds := &fakeDeployStore{}
	var seen [][]string
	err := (&Tracker{Store: ds}).PatchLocked(context.Background(), cl, read, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		seen = append(seen, a.Spec.FilesFromSecrets)
		a.Spec.FilesFromSecrets = append(a.Spec.FilesFromSecrets, "web-group-files")
		a.Spec.RestartedAt = fmt.Sprintf("restart-%d", len(seen))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	live := &appv1alpha1.App{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(read), live); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || len(seen[1]) != 1 || seen[1][0] != "web-files" {
		t.Fatalf("mutate saw %v, want a second run on the winner's App", seen)
	}
	if got := live.Spec.FilesFromSecrets; len(got) != 2 || got[0] != "web-files" || live.Spec.RestartedAt != "restart-2" {
		t.Fatalf("live App = %v %q, want the winner's reference kept and the retry's write", got, live.Spec.RestartedAt)
	}
	if len(ds.created) != 1 {
		t.Fatalf("deploy rows = %d, want one for the patch that landed", len(ds.created))
	}
}

// TestPatchLockedGivesUpUnderPersistentContention (w5/m131): a write that
// keeps losing answers core.ErrConflict after lockedPatchAttempts tries, not
// the raw Kubernetes conflict an API surface would report as an internal
// error, and records no rollout.
func TestPatchLockedGivesUpUnderPersistentContention(t *testing.T) {
	cl, read := lockedPatchFixture(t, interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return apierrors.NewConflict(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, "web", fmt.Errorf("changed"))
		},
	})
	ds := &fakeDeployStore{}
	runs := 0
	err := (&Tracker{Store: ds}).PatchLocked(context.Background(), cl, read, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		runs++
		a.Spec.RestartedAt = "restart"
		return nil
	})
	if !errors.Is(err, core.ErrConflict) || runs != lockedPatchAttempts || len(ds.created) != 0 {
		t.Fatalf("err = %v after %d runs and %d rows, want the conflict after %d runs and no row", err, runs, len(ds.created), lockedPatchAttempts)
	}
}

// TestPatchLockedLeavesARecreatedNamesakeAlone (w5/157): a write whose App was
// deleted and recreated under its name before its patch landed lost the
// patch's lock, re-read the App by name and ran its mutate again on the
// namesake. It now refuses with core.ErrServiceReplaced after one run, keeps
// its own App rather than the namesake's, and records no rollout.
func TestPatchLockedLeavesARecreatedNamesakeAlone(t *testing.T) {
	recreated := false
	cl, read := lockedPatchFixture(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if !recreated {
				recreated = true
				if err := c.Delete(ctx, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}); err != nil {
					return err
				}
				namesake := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "uid-namesake"},
					Spec:       appv1alpha1.AppSpec{Image: "web:namesake"},
				}
				if err := c.Create(ctx, namesake); err != nil {
					return err
				}
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	ds := &fakeDeployStore{}
	runs := 0
	err := (&Tracker{Store: ds}).PatchLocked(context.Background(), cl, read, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		runs++
		a.Spec.RestartedAt = "restart"
		return nil
	})
	if !errors.Is(err, core.ErrServiceReplaced) || runs != 1 || len(ds.created) != 0 {
		t.Fatalf("err = %v after %d runs and %d rows, want the replaced-service conflict after one run and no row", err, runs, len(ds.created))
	}
	if read.UID == "uid-namesake" || read.Spec.Image != "web:v1" {
		t.Fatalf("the write's App became %s %q, want its own", read.UID, read.Spec.Image)
	}
	live := &appv1alpha1.App{}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(read), live); err != nil {
		t.Fatal(err)
	}
	if live.UID != "uid-namesake" || live.Spec.RestartedAt != "" {
		t.Fatalf("the namesake = %s restarted at %q, want it untouched", live.UID, live.Spec.RestartedAt)
	}
}
