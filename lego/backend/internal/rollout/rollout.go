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
// (appv1alpha1.SpecRollsRelease), so the two planes cannot drift.
package rollout

import (
	"context"
	"log"
	"strconv"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DeployStore is the one store method this package needs: opening the row.
// *store.PGStore satisfies it, as does apps.IntentStore's own superset.
type DeployStore interface {
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error)
}

// Tracker opens deploy-history rows for spec patches that roll a release. A nil
// Tracker (or one with no Store) is the honest degrade for CR-only mode and for
// tests with no control-plane database: patches still apply, they are simply
// not recorded — the pre-w6/m51 behavior.
type Tracker struct {
	Store DeployStore
}

// Snapshot is an App CR's identity taken immediately BEFORE a spec mutation:
// what the spec was, which metadata.generation it carried, and which
// control-plane row it belongs to. Compare it against the mutated App to learn
// whether the pending patch is a rollout.
type Snapshot struct {
	appID      string
	generation int64
	spec       appv1alpha1.AppSpec
	rolls      bool
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

// Stamp reports whether the mutation a now carries rolls a new release, and
// when it does stamps the release-generation annotation so the deploy row
// created after the patch lands names the same release the operator will
// reconcile — the same guard deploys.triggerFetched uses against an operational
// mutation racing in between (see requestedReleaseGeneration in the operator).
//
// It must be called after the mutation and before the Patch, so the annotation
// rides the same write. A hand-applied App (no control-plane row) has no deploy
// history to keep, so it is left alone.
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
	a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(s.generation+1, 10)
	return true
}

// Patch is the whole dance in one call: snapshot a, apply mutate, stamp the
// release generation if the result rolls, merge-patch through cl, and open the
// deploy row the rollout owes. Every App-spec writer outside the deploy verbs
// goes through it, so none of them can quietly become a rollout nothing
// recorded — the bug w6/m51 was filed for.
//
// A nil Tracker still applies the patch; it just records nothing.
func (t *Tracker) Patch(ctx context.Context, cl client.Client, a *appv1alpha1.App, trigger string, mutate func(*appv1alpha1.App) error) error {
	before := Before(a)
	base := client.MergeFrom(a.DeepCopy())
	if err := mutate(a); err != nil {
		return err
	}
	tracked := before.Stamp(a)
	if err := cl.Patch(ctx, a, base); err != nil {
		return err
	}
	if tracked {
		t.Open(ctx, before, a, trigger)
	}
	return nil
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
	// Inside a Batch, one request's several field writes are ONE rollout: hold
	// the newest and let the flush record it, so a Settings save that changes
	// four fields does not read back as four deploys, three of them canceled.
	if b, ok := ctx.Value(batchKey{}).(*batch); ok {
		b.hold(t, snapshot, a, trigger)
		return
	}
	t.open(ctx, snapshot, a, trigger)
}

func (t *Tracker) open(ctx context.Context, snapshot Snapshot, a *appv1alpha1.App, trigger string) {
	// max(after, before+1): a real API server has already incremented
	// metadata.generation on the patch; the fake client used off-cluster and in
	// tests has not (deploys.patchedGeneration, the same monotonic fallback).
	generation := max(a.Generation, snapshot.generation+1)
	if _, err := t.Store.CreateDeploy(ctx, snapshot.appID, trigger, a.Spec.Image, generation, store.CommitInfo{}); err != nil {
		log.Printf("rollout: record %s deploy for %s: %v", trigger, a.Name, err)
	}
}

type batchKey struct{}

// batch holds the newest rollout recorded under a Batch context. Only the last
// one survives: the operator reconciles the final spec, so the final release
// generation is the one the row must be filed under for the reconciler to close
// it. Earlier writes in the same request are steps toward it, not deploys of
// their own.
type batch struct {
	mu       sync.Mutex
	tracker  *Tracker
	snapshot Snapshot
	app      *appv1alpha1.App
	trigger  string
}

func (b *batch) hold(t *Tracker, snapshot Snapshot, a *appv1alpha1.App, trigger string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tracker, b.snapshot, b.app, b.trigger = t, snapshot, a, trigger
}

// Batch scopes a request whose several App patches are one rollout — a Render
// PATCH /v1/services/{id} or the update_service MCP tool, each of which applies
// its fields as an ordered table of individual setters. Every tracked patch
// under the returned context collapses into a single deploy row, written by the
// returned flush. Callers defer the flush: an op table that fails partway
// through has still rolled the service for the fields it did apply, and that
// rollout is still owed its row.
func Batch(ctx context.Context) (context.Context, func()) {
	b := &batch{}
	batched := context.WithValue(ctx, batchKey{}, b)
	return batched, func() { b.flush(batched) }
}

func (b *batch) flush(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tracker == nil {
		return
	}
	b.tracker.open(ctx, b.snapshot, b.app, b.trigger)
	b.tracker, b.app = nil, nil
}
