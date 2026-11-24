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

// Package rollout records the deploy-history row a spec patch owes the user.
//
// bex has no way to change a service's release without rolling it: any App CR
// spec change that moves the operator's artifact or release identity re-enters
// the build/deploy path (docs/ADR060, the operator's release_identity.go). Until
// w6/m51, only the explicit deploy verbs (deploys.Trigger, Rollback, the git
// webhook, create) opened a deploys row for that — so a Settings-page edit, an
// env var write, or an env-group link rebuilt the service invisibly: nothing in
// the Deploys tab or Events feed, nothing to retry or roll back, and a failed
// one left the service reporting Building with no visible next step.
//
// This package is the shared seam every remaining App-spec writer patches
// through. It answers one question — does this patch roll a new release? — from
// the same classification the operator fingerprints with
// (appv1alpha1.SpecRollsRelease), so the two planes cannot drift. A rolling
// patch on a suspended service, or one made under WithSaveOnly (Render's
// Update Service, w8/m135), is held for the next deploy instead: the operator
// keeps serving the release it runs until a deploy verb stamps a newer one.
package rollout

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DeployStore is the store seam this package needs: looking up a prior commit
// and opening the row. *store.PGStore satisfies it, as does apps.IntentStore's
// own superset.
type DeployStore interface {
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string, clearCache ...bool) (store.Deploy, error)
	// LatestDeployCommit returns the newest non-empty commit for the app, or
	// zero CommitInfo.
	LatestDeployCommit(ctx context.Context, appID string) (store.CommitInfo, error)
}

// ErrNoDeployHistory reports that this Tracker cannot read deploy history
// (CR-only mode, a hand-applied App), so the live release is unknown.
var ErrNoDeployHistory = errors.New("rollout: no deploy history")

// ServedRelease returns the App's live deploy when it carries the resolved
// image the operator served — the immutable artifact a no-build release reuses.
// nil means the App has no live build.
func (t *Tracker) ServedRelease(ctx context.Context, a *appv1alpha1.App) (*store.Deploy, error) {
	lister, appID := t.history(a)
	if lister == nil {
		return nil, ErrNoDeployHistory
	}
	rows, err := lister.ListDeploys(ctx, appID, store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
	if err != nil || len(rows) == 0 || rows[0].ResolvedImage == "" {
		return nil, err
	}
	return &rows[0], nil
}

// history is the reader of a's deploy history and its row id, or nil when
// this Tracker cannot read it.
func (t *Tracker) history(a *appv1alpha1.App) (DeployLister, string) {
	if t == nil || a == nil {
		return nil, ""
	}
	lister, _ := t.Store.(DeployLister)
	appID := store.ManagedAppID(a.Labels)
	if lister == nil || appID == "" {
		return nil, ""
	}
	return lister, appID
}

// Tracker opens deploy-history rows for spec patches that roll a release. A nil
// Tracker (or one with no Store) is the honest degrade for CR-only mode and for
// tests with no control-plane database: patches still apply, they are simply
// not recorded — the pre-w6/m51 behavior.
type Tracker struct {
	Store DeployStore
	// CloneSecrets refreshes a repo-backed App's private-repo clone credential
	// before a write that rolls a release which may build (w32/047). A GitHub
	// installation token lives an hour, so a release whose build runs on the
	// last deploy trigger's token fails at checkout. nil: no refresh (GitHub
	// off, or a Tracker built for a test).
	CloneSecrets CloneCredential
	// AdoptionWait bounds how long a write that rolls a release waits for the
	// operator to adopt the deploy ahead of it (AwaitAdoption, w8/390): zero
	// is DefaultAdoptionWait, negative no wait.
	AdoptionWait time.Duration
}

// awaitAdoption holds a write through this Tracker, as Stamp decided it,
// behind a deploy the operator has not adopted: one that rolls a release
// (AwaitAdoption), or one that held its change for the next deploy and would
// otherwise hold that deploy too (AwaitAdoptionBeforeHold, w6/163).
func (t *Tracker) awaitAdoption(ctx context.Context, cl client.Reader, a *appv1alpha1.App, before *Snapshot) error {
	if t == nil || (!before.rolls && !before.pends) {
		return nil
	}
	lister, _ := t.Store.(DeployLister)
	if before.pends {
		return AwaitAdoptionBeforeHold(ctx, cl, lister, a, t.AdoptionWait)
	}
	return AwaitAdoption(ctx, cl, lister, a, t.AdoptionWait)
}

// CloneCredential is the clone-credential mint every deploy trigger uses
// (apps.Service's bridge, wired in the composition root).
type CloneCredential interface {
	// EnsureAppCloneSecret writes a fresh installation token into a's clone
	// Secret and answers its name, or "" when no GitHub connection grants
	// a's repo (a public repo, or GitHub off). An error is a GitHub failure a
	// write must not swallow into a stale or absent credential.
	EnsureAppCloneSecret(ctx context.Context, a *appv1alpha1.App) (string, error)
}

// cloneRefresh mints a write's clone credential once, even when a locked
// patch runs its mutation again after losing a race. It mints again only
// when the retry found another repo.
type cloneRefresh struct {
	repo, secret string
}

// refresh mints a's clone credential, before the patch that bumps its
// generation, when the write rolls a release that may build: a repo-backed App
// whose release does not rerun an existing artifact (SelectReleaseImage). A
// settings_change, a config_change with no live build to rerun and an
// env-group release are each a deploy, and each builds with the token in
// a.Spec.CloneSecret, exactly as a deploy trigger's build does.
func (t *Tracker) refresh(ctx context.Context, c *cloneRefresh, before Snapshot, a *appv1alpha1.App) error {
	if t == nil || t.CloneSecrets == nil || !before.rolls || a.Spec.Repo == "" || rerunsArtifact(before, a) {
		return nil
	}
	if c.repo != a.Spec.Repo {
		secret, err := t.CloneSecrets.EnsureAppCloneSecret(ctx, a)
		if err != nil {
			return err
		}
		c.repo, c.secret = a.Spec.Repo, secret
	}
	if c.secret != "" {
		a.Spec.CloneSecret = c.secret
	}
	return nil
}

// rerunsArtifact reports whether the release a's patch opens reruns an
// existing artifact rather than building: an environment "Save and deploy"
// selects the build serving now (SelectServedBuild), so no checkout runs.
func rerunsArtifact(before Snapshot, a *appv1alpha1.App) bool {
	ref := a.Spec.ReleaseConfig
	return ref != nil && ref.Generation == before.generation+1 && ref.Image != ""
}

// Snapshot is an App CR's identity taken immediately BEFORE a spec mutation:
// what the spec was, which metadata.generation it carried, and which
// control-plane row it belongs to. Compare it against the mutated App to learn
// whether the pending patch is a rollout.
type Snapshot struct {
	appID      string
	generation int64
	spec       appv1alpha1.AppSpec
	// rolls is Stamp's verdict: the write stamped a release, so Open files
	// its row. A change held on a suspended App, or saved only, does not roll.
	rolls bool
	// saveOnly holds a rolling change for the next deploy instead of
	// stamping one (WithSaveOnly).
	saveOnly bool
	// pends is Stamp's verdict for a save-only write to a service that
	// deploys: it held a rolling change for the next deploy.
	pends bool
}

type saveOnlyKey struct{}

// WithSaveOnly marks ctx so a rolling write through Patch or PatchLocked is
// saved and held for the next deploy rather than deployed: Render's Update
// Service contract, "Changes will not be deployed automatically. Instead you
// must call the deploy API" (w8/m135). Every REST PATCH /v1/services/{id} and
// MCP update_service row runs under it, a plan change included (w6/m609); the
// dashboard's GraphQL Settings saves do not, so they keep saving and
// deploying.
func WithSaveOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, saveOnlyKey{}, true)
}

// SaveOnly reports whether ctx is WithSaveOnly's: a write a verb makes under
// it waits for the next deploy, so the verb's own effects of a change that
// applies at once wait too (apps.SetPlan, w6/m609).
func SaveOnly(ctx context.Context) bool {
	held, _ := ctx.Value(saveOnlyKey{}).(bool)
	return held
}

// Before captures a. Callers take it before mutating, exactly where they take
// the merge-patch base.
func Before(a *appv1alpha1.App) Snapshot {
	if a == nil {
		return Snapshot{}
	}
	return Snapshot{
		appID:      store.ManagedAppID(a.Labels),
		generation: a.Generation,
		spec:       *a.Spec.DeepCopy(),
	}
}

// beforeIn is Before for a write made under ctx, which may be save-only.
// A Stage's publish re-stamps only what its staged writes already stamped,
// so it uses Before.
func beforeIn(ctx context.Context, a *appv1alpha1.App) Snapshot {
	s := Before(a)
	s.saveOnly = SaveOnly(ctx)
	return s
}

// Stamp reports whether the mutation a now carries rolls a new release, and
// when it does stamps the release-generation annotation so the deploy row
// created after the patch lands names the same release the operator will
// reconcile — the same guard deploys.triggerFetched uses against an operational
// mutation racing in between (see requestedReleaseGeneration in the operator).
//
// It must be called after the mutation and before the Patch, so the annotation
// rides the same write. A hand-applied App (no control-plane row) has no deploy
// history to keep, so it is left alone. On a suspended App, and for a
// save-only write (WithSaveOnly), a rolling change is held for the next deploy
// instead (the pending-source annotation), and Stamp reports false: nothing
// deploys while the service is suspended, or until the caller deploys.
func (s *Snapshot) Stamp(a *appv1alpha1.App) bool {
	if s == nil || a == nil || s.appID == "" {
		return false
	}
	s.rolls = appv1alpha1.SpecRollsRelease(s.spec, a.Spec)
	if !s.rolls {
		return false
	}
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	if a.Spec.Suspended || s.saveOnly {
		// A suspended service deploys nothing (Trigger and Rollback refuse it),
		// so a write that would roll a release is held for the next deploy
		// instead: no release stamp and no deploy row, which the reconciler
		// would only close as canceled. The operator keeps the prior release
		// serving through Resume, and the next deploy rolls the saved change —
		// the annotation the source-update hold (apps.markSourcePending) writes.
		a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = strconv.FormatInt(s.generation+1, 10)
		s.rolls = false
		s.pends = !a.Spec.Suspended
		return false
	}
	// The release this write mints carries the env-group links staged while the
	// service had Auto-Deploy off (w4/m239), its group refs in creation order
	// (w4/m355).
	PrepareGroupRefsForRelease(a)
	a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(s.generation+1, 10)
	return true
}

// Patch is the whole dance in one call: snapshot a, apply mutate, stamp the
// release generation if the result rolls, merge-patch through cl, and open the
// deploy row the rollout owes. Every App-spec writer outside the deploy verbs
// goes through it, so none of them can quietly become a rollout nothing
// recorded — the bug w6/m51 was filed for.
//
// A rolling patch of a repo-backed App refreshes its clone credential first
// (refresh); a mint failure fails the write before anything is patched. A
// settings_change of a prebuilt image keeps the image its release in force
// selected (keepSelectedImage).
//
// A nil Tracker still applies the patch; it just records nothing.
func (t *Tracker) Patch(ctx context.Context, cl client.Client, a *appv1alpha1.App, trigger string, mutate func(*appv1alpha1.App) error) error {
	before := beforeIn(ctx, a)
	base := client.MergeFrom(a.DeepCopy())
	if err := mutate(a); err != nil {
		return err
	}
	if err := t.keepSelectedImage(ctx, trigger, before, a); err != nil {
		return err
	}
	tracked := before.Stamp(a)
	if err := t.refresh(ctx, &cloneRefresh{}, before, a); err != nil {
		return err
	}
	if err := t.awaitAdoption(ctx, cl, a, &before); err != nil {
		return err
	}
	if err := cl.Patch(ctx, a, base); err != nil {
		return err
	}
	if tracked {
		t.Open(ctx, before, a, trigger)
	}
	return nil
}

// lockedPatchAttempts bounds PatchAppLocked's re-reads under contention.
const lockedPatchAttempts = 5

// PatchLocked is Patch under an optimistic lock (w5/m131), through
// PatchAppLocked: a write that loses the App race decides again from what the
// winner left rather than overwriting it. The deploy row is opened once, for
// the patch that lands.
func (t *Tracker) PatchLocked(ctx context.Context, cl client.Client, a *appv1alpha1.App, trigger string, mutate func(*appv1alpha1.App) error) error {
	var before Snapshot
	var clone cloneRefresh
	tracked := false
	if err := PatchAppLocked(ctx, cl, a, func(a *appv1alpha1.App) error {
		before = beforeIn(ctx, a)
		if err := mutate(a); err != nil {
			return err
		}
		if err := t.keepSelectedImage(ctx, trigger, before, a); err != nil {
			return err
		}
		tracked = before.Stamp(a)
		if err := t.refresh(ctx, &clone, before, a); err != nil {
			return err
		}
		// A wait that ends on the operator's adoption loses this patch's
		// resourceVersion race, so the mutation runs again on the adopted App.
		return t.awaitAdoption(ctx, cl, a, &before)
	}); err != nil {
		return err
	}
	if tracked {
		t.Open(ctx, before, a, trigger)
	}
	return nil
}

// PatchAppLocked applies mutate to a and merge-patches it with the
// resourceVersion a was read at (w5/m131). On a conflict it re-reads a and runs
// mutate again, so mutate must be safe to run more than once. A write that
// keeps losing answers core.ErrConflict after lockedPatchAttempts tries. A
// re-read that finds another App at a's name answers core.ErrServiceReplaced
// without running mutate again, leaving a with the mutation that never landed
// (w5/157).
func PatchAppLocked(ctx context.Context, cl client.Client, a *appv1alpha1.App, mutate func(*appv1alpha1.App) error) error {
	for attempt := 1; ; attempt++ {
		base := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
		if err := mutate(a); err != nil {
			return err
		}
		err := cl.Patch(ctx, a, base)
		if !apierrors.IsConflict(err) {
			return err
		}
		if attempt == lockedPatchAttempts {
			return fmt.Errorf("%w: too many concurrent updates to service %q; retry", core.ErrConflict, core.DisplayedAppName(a))
		}
		if err := core.RereadApp(ctx, cl, a); err != nil {
			return err
		}
	}
}

// Open records the deploy row for a stamped patch that has been applied. a must
// be the App as the API server returned it, so its bumped metadata.generation
// is the one the row is filed under. Callers that own their own merge-patch
// (because they need a different patch base, or a view of the result) use it
// directly; everyone else uses Patch.
//
// Failure is logged, never returned: the spec patch already succeeded and the
// rebuild is already rolling, so refusing the caller's verb would report a
// failure that did not happen. The reconciler's superseded-row handling is the
// backstop for the row this one would have replaced — the same trade-off the
// git-webhook redeploy path makes.
func (t *Tracker) Open(ctx context.Context, snapshot Snapshot, a *appv1alpha1.App, trigger string) {
	if t == nil || t.Store == nil || a == nil || !snapshot.rolls || snapshot.appID == "" {
		return
	}
	// max(after, before+1): a real API server has already incremented
	// metadata.generation on the patch; the fake client used off-cluster and in
	// tests has not (deploys.patchedGeneration, the same monotonic fallback).
	generation := max(a.Generation, snapshot.generation+1)
	// Carry the prior release's commit onto config_change (and any other
	// Tracker-opened) rows so history still answers "what code is live?".
	// Read here — not inside CreateDeploy — so callers that pass empty
	// intentionally keep "none". Image-backed apps with no prior commit stay
	// empty; never synthesize one.
	commit := store.CommitInfo{}
	if prior, err := t.Store.LatestDeployCommit(ctx, snapshot.appID); err != nil {
		log.Printf("rollout: prior commit for %s: %v", a.Name, err)
	} else if prior.Hash != "" {
		commit = prior
	}
	// A release selecting an existing artifact (w4/m180) records that image, so
	// history shows no build, and the commit the artifact was built from.
	image := a.Spec.Image
	if ref := a.Spec.ReleaseConfig; ref != nil && ref.Generation == generation && ref.Image != "" {
		image = ref.Image
		if live, err := t.ServedRelease(ctx, a); err == nil && live != nil && live.ResolvedImage == ref.Image && live.Commit != "" {
			commit = live.CommitInfo()
		}
	}
	// A waiting deploy the patched App reports adopted is in flight, so this
	// release waits behind it rather than replacing it (w8/396).
	if _, err := t.Store.CreateDeploy(store.WithAdoptedRelease(ctx, a), snapshot.appID, trigger, image, generation, commit, core.SubjectFrom(ctx), false); err != nil {
		log.Printf("rollout: record %s deploy for %s: %v", trigger, a.Name, err)
	}
}
