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
// commit from LatestDeployCommit — enough to unit-test Tracker.Open's carry.
type fakeDeployStore struct {
	prior   store.CommitInfo
	created []store.Deploy
	// adopted is each CreateDeploy's store.AdoptedRelease.
	adopted []int64
}

func (f *fakeDeployStore) LatestDeployCommit(_ context.Context, _ string) (store.CommitInfo, error) {
	return f.prior, nil
}

func (f *fakeDeployStore) CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error) {
	f.adopted = append(f.adopted, store.AdoptedRelease(ctx))
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
	tr.Open(context.Background(), Snapshot{appID: "srv-web", generation: 1, rolls: true}, a, store.TriggerConfigChange)
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

// w8/396: the row a saved change opens names the release the patched App
// reports adopted, so it waits behind a waiting deploy the operator already
// runs rather than replacing it.
func TestOpenNamesTheReleaseTheAppAdopted(t *testing.T) {
	ds := &fakeDeployStore{}
	a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 4}, Spec: appv1alpha1.AppSpec{Image: "web:v1"}}
	a.Status.ReleaseGeneration = 3
	(&Tracker{Store: ds}).Open(context.Background(), Snapshot{appID: "srv-web", generation: 3, rolls: true}, a, store.TriggerConfigChange)
	if len(ds.adopted) != 1 || ds.adopted[0] != 3 {
		t.Fatalf("adopted releases named = %v, want [3]", ds.adopted)
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

// TestARollingWriteOnASuspendedServiceIsHeldForTheNextDeploy is w4/m234: a
// release-rolling write on a suspended service stamped a release and opened a
// config-change row the reconciler then closed as canceled. It now opens no
// row and marks the change pending, so the operator keeps the prior release
// serving through Resume and the next deploy rolls it; a running service's
// write still opens exactly one row.
func TestARollingWriteOnASuspendedServiceIsHeldForTheNextDeploy(t *testing.T) {
	for _, suspended := range []bool{true, false} {
		t.Run(fmt.Sprintf("suspended=%v", suspended), func(t *testing.T) {
			cl, a := lockedPatchFixture(t, interceptor.Funcs{})
			if suspended {
				a.Spec.Suspended = true
				if err := cl.Update(context.Background(), a); err != nil {
					t.Fatal(err)
				}
			}
			ds := &fakeDeployStore{}
			tr := &Tracker{Store: ds}
			if err := tr.PatchLocked(context.Background(), cl, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
				a.Spec.HealthCheckPath = "/healthz"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			_, stamped := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]
			_, pending := a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]
			switch {
			case suspended && (len(ds.created) != 0 || stamped || !pending):
				t.Fatalf("suspended write: rows %d, stamped %v, pending %v; want no row, no stamp, pending", len(ds.created), stamped, pending)
			case !suspended && (len(ds.created) != 1 || !stamped || pending):
				t.Fatalf("running write: rows %d, stamped %v, pending %v; want one row, a stamp, nothing pending", len(ds.created), stamped, pending)
			}
		})
	}
}

// trackerPatches runs a settings_change write through Patch and PatchLocked.
var trackerPatches = map[string]func(*Tracker, context.Context, client.Client, *appv1alpha1.App, func(*appv1alpha1.App) error) error{
	"Patch": func(tr *Tracker, ctx context.Context, cl client.Client, a *appv1alpha1.App, m func(*appv1alpha1.App) error) error {
		return tr.Patch(ctx, cl, a, store.TriggerSettingsChange, m)
	},
	"PatchLocked": func(tr *Tracker, ctx context.Context, cl client.Client, a *appv1alpha1.App, m func(*appv1alpha1.App) error) error {
		return tr.PatchLocked(ctx, cl, a, store.TriggerSettingsChange, m)
	},
}

// TestASaveOnlyRollingWriteIsHeldForTheNextDeploy is w8/m135: under
// WithSaveOnly (Render's Update Service), a release-rolling write is saved and
// held exactly like a suspended one — no row, no release stamp, the pending
// marker set — through both Patch and PatchLocked, while an operational write
// marks nothing pending.
func TestASaveOnlyRollingWriteIsHeldForTheNextDeploy(t *testing.T) {
	for name, patch := range trackerPatches {
		t.Run(name, func(t *testing.T) {
			cl, a := lockedPatchFixture(t, interceptor.Funcs{})
			ds := &fakeDeployStore{}
			tr := &Tracker{Store: ds}
			ctx := WithSaveOnly(context.Background())
			if err := patch(tr, ctx, cl, a, func(a *appv1alpha1.App) error {
				a.Spec.HealthCheckPath = "/hz2"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			_, stamped := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]
			if len(ds.created) != 0 || stamped || a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
				t.Fatalf("save-only write: rows %d, stamped %v, annotations %v; want no row, no stamp, pending",
					len(ds.created), stamped, a.Annotations)
			}
			if a.Spec.HealthCheckPath != "/hz2" {
				t.Fatalf("healthCheckPath = %q, want the saved /hz2", a.Spec.HealthCheckPath)
			}

			cl, a = lockedPatchFixture(t, interceptor.Funcs{})
			if err := patch(tr, ctx, cl, a, func(a *appv1alpha1.App) error {
				a.Spec.Replicas = 3
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, pending := a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; pending || len(ds.created) != 0 {
				t.Fatalf("an operational save-only write marked a release pending or opened a row")
			}
		})
	}
}

// TestStagedGroupLinksShipWithTheNextRollingWriteOnly (w4/m239): a staged link
// changes no release-identity field, a write that does not roll leaves it
// staged, and the write that rolls folds it into the spec under its own row.
func TestStagedGroupLinksShipWithTheNextRollingWriteOnly(t *testing.T) {
	cl, a := lockedPatchFixture(t, interceptor.Funcs{})
	before := a.Spec.DeepCopy()
	StageGroupLink(a, "evg-1-env", "evg-1-files", true)
	if appv1alpha1.SpecRollsRelease(*before, a.Spec) {
		t.Fatal("staging a link moved the release identity")
	}
	if env, files := EffectiveGroupSecrets(a); len(env) != 1 || len(files) != 1 {
		t.Fatalf("effective refs = %v %v, want the staged link", env, files)
	}
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	ds := &fakeDeployStore{}
	tr := &Tracker{Store: ds}

	if err := tr.PatchLocked(context.Background(), cl, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		a.Spec.Replicas = 2 // operational: not a release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(ds.created) != 0 || len(a.Spec.EnvFromSecrets) != 0 || a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] == "" {
		t.Fatalf("a non-rolling write shipped the staged link: rows %d spec %v", len(ds.created), a.Spec.EnvFromSecrets)
	}

	if err := tr.PatchLocked(context.Background(), cl, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		a.Spec.HealthCheckPath = "/healthz"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(ds.created) != 1 || len(a.Spec.EnvFromSecrets) != 1 || len(a.Spec.FilesFromSecrets) != 1 || a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] != "" {
		t.Fatalf("the rolling write did not carry the link: rows %d spec %+v annotations %v", len(ds.created), a.Spec, a.Annotations)
	}
}

func TestStagedGroupLinkRoundTrips(t *testing.T) {
	a := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{EnvFromSecrets: []string{"evg-1-env"}, FilesFromSecrets: []string{"evg-1-files"}}}
	StageGroupLink(a, "evg-1-env", "evg-1-files", false)
	if env, _ := EffectiveGroupSecrets(a); len(env) != 0 || len(a.Spec.EnvFromSecrets) != 1 {
		t.Fatalf("staged unlink: effective %v spec %v", env, a.Spec.EnvFromSecrets)
	}
	StageGroupLink(a, "evg-1-env", "evg-1-files", true)
	if a.Annotations[appv1alpha1.AnnotationPendingGroupLinks] != "" {
		t.Fatalf("unlink then relink of a mounted group left %v staged", a.Annotations)
	}
}

// fakeCloneCredential is a CloneCredential that records the repo of each mint
// and how many App patches had landed when it ran.
type fakeCloneCredential struct {
	err     error
	patches *int
	mints   []string
	// patchesAtMint is *patches at each mint.
	patchesAtMint []int
}

func (f *fakeCloneCredential) EnsureAppCloneSecret(_ context.Context, a *appv1alpha1.App) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.mints = append(f.mints, a.Spec.Repo)
	f.patchesAtMint = append(f.patchesAtMint, *f.patches)
	return appv1alpha1.CloneSecretName(a.Name), nil
}

// repoPatchFixture is lockedPatchFixture for a repo-backed service whose App
// names no clone Secret yet, counting the App patches that reach the API
// server.
func repoPatchFixture(t *testing.T) (client.Client, *appv1alpha1.App, *fakeCloneCredential) {
	t.Helper()
	patches := 0
	cl, a := lockedPatchFixture(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			patches++
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	a.Spec.Image, a.Spec.Repo = "", "https://github.com/acme/private"
	if err := cl.Update(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	return cl, a, &fakeCloneCredential{patches: &patches}
}

// TestARollingWriteMintsTheCloneCredentialBeforeItsPatch is w32/047: a
// settings_change on a private-repo service whose release must build cloned
// with the last deploy trigger's installation token, expired an hour after it
// was minted, and its build failed at checkout. A rolling write now mints the
// credential before the patch that bumps the generation, as a deploy trigger
// does, and names its Secret in the same patch.
func TestARollingWriteMintsTheCloneCredentialBeforeItsPatch(t *testing.T) {
	for name, patch := range trackerPatches {
		t.Run(name, func(t *testing.T) {
			cl, a, cc := repoPatchFixture(t)
			ds := &fakeDeployStore{}
			if err := patch(&Tracker{Store: ds, CloneSecrets: cc}, context.Background(), cl, a, func(a *appv1alpha1.App) error {
				a.Spec.BuildCommand = "make build"
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if len(cc.mints) != 1 || cc.mints[0] != "https://github.com/acme/private" || cc.patchesAtMint[0] != 0 {
				t.Fatalf("mints %v with %v patches landed, want one mint of the repo before any patch", cc.mints, cc.patchesAtMint)
			}
			live := &appv1alpha1.App{}
			if err := cl.Get(context.Background(), client.ObjectKeyFromObject(a), live); err != nil {
				t.Fatal(err)
			}
			if live.Spec.CloneSecret != "web-clone" || live.Spec.BuildCommand != "make build" {
				t.Fatalf("live spec cloneSecret %q buildCommand %q, want the minted Secret in the rolling patch", live.Spec.CloneSecret, live.Spec.BuildCommand)
			}
			if len(ds.created) != 1 || ds.created[0].Trigger != store.TriggerSettingsChange {
				t.Fatalf("deploy rows %+v, want one settings_change", ds.created)
			}
		})
	}
}

// A write that builds nothing mints nothing: a deploy trigger's token reaches
// the build of every release that does build.
func TestAWriteThatBuildsNothingMintsNoCloneCredential(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		before func(*appv1alpha1.App)
		mutate func(*appv1alpha1.App)
		// rolls: the write still stamps a release, which builds nothing.
		rolls bool
	}{
		{name: "operational", ctx: context.Background(), mutate: func(a *appv1alpha1.App) { a.Spec.Replicas = 3 }},
		{name: "save only", ctx: WithSaveOnly(context.Background()), mutate: func(a *appv1alpha1.App) { a.Spec.BuildCommand = "make build" }},
		{name: "suspended", ctx: context.Background(),
			before: func(a *appv1alpha1.App) { a.Spec.Suspended = true },
			mutate: func(a *appv1alpha1.App) { a.Spec.BuildCommand = "make build" }},
		{name: "image-backed", ctx: context.Background(),
			before: func(a *appv1alpha1.App) { a.Spec.Repo, a.Spec.Image = "", "web:v1" },
			mutate: func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" }, rolls: true},
		// An environment "Save and deploy" reruns the build serving now.
		{name: "reruns the served build", ctx: context.Background(), mutate: func(a *appv1alpha1.App) {
			a.Spec.RestartedAt = "restart"
			SelectReleaseImage(a, "registry/web@sha256:abc")
		}, rolls: true},
	} {
		for name, patch := range trackerPatches {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				cl, a, cc := repoPatchFixture(t)
				if tc.before != nil {
					tc.before(a)
					if err := cl.Update(context.Background(), a); err != nil {
						t.Fatal(err)
					}
				}
				if err := patch(&Tracker{Store: &fakeDeployStore{}, CloneSecrets: cc}, tc.ctx, cl, a, func(a *appv1alpha1.App) error {
					tc.mutate(a)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if _, stamped := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; stamped != tc.rolls {
					t.Fatalf("release stamped = %v, want %v", stamped, tc.rolls)
				}
				if len(cc.mints) != 0 || a.Spec.CloneSecret != "" {
					t.Fatalf("mints %v, cloneSecret %q; want none", cc.mints, a.Spec.CloneSecret)
				}
			})
		}
	}
}

// A mint failure fails the write before anything is patched or recorded, as
// a deploy trigger's does: a private repo never builds on a stale credential.
func TestACloneCredentialMintFailureFailsTheWrite(t *testing.T) {
	for name, patch := range trackerPatches {
		t.Run(name, func(t *testing.T) {
			cl, a, cc := repoPatchFixture(t)
			cc.err = errors.New("github down")
			ds := &fakeDeployStore{}
			err := patch(&Tracker{Store: ds, CloneSecrets: cc}, context.Background(), cl, a, func(a *appv1alpha1.App) error {
				a.Spec.BuildCommand = "make build"
				return nil
			})
			if !errors.Is(err, cc.err) || *cc.patches != 0 || len(ds.created) != 0 {
				t.Fatalf("err %v after %d patches and %d rows, want the mint's error and nothing written", err, *cc.patches, len(ds.created))
			}
		})
	}
}

// A locked write that loses the App race runs its mutation again but mints
// once.
func TestALockedWriteMintsOnceAcrossARetry(t *testing.T) {
	cl, read, cc := repoPatchFixture(t)
	winner := read.DeepCopy()
	winner.Spec.FilesFromSecrets = []string{"web-files"}
	if err := cl.Update(context.Background(), winner); err != nil {
		t.Fatal(err)
	}
	runs := 0
	if err := (&Tracker{Store: &fakeDeployStore{}, CloneSecrets: cc}).PatchLocked(context.Background(), cl, read, store.TriggerSettingsChange, func(a *appv1alpha1.App) error {
		runs++
		a.Spec.BuildCommand = "make build"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if runs != 2 || len(cc.mints) != 1 || read.Spec.CloneSecret != "web-clone" {
		t.Fatalf("%d runs, mints %v, cloneSecret %q; want two runs, one mint, the Secret named", runs, cc.mints, read.Spec.CloneSecret)
	}
}
