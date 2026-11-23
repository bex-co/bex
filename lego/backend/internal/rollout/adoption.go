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
	"log"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Wait overlap policy lets the deploy in progress finish before the
// most recently triggered one runs (render.com/docs/deploys). The operator
// holds a release in flight only once it has adopted it into
// status.releaseGeneration (releaseInFlight), so a release written to the App
// before then replaces it, and the deploy ahead closes "Superseded by …"
// without running. One rule covers every writer (w8/387, w8/390): a release
// behind a deploy the operator has not adopted waits. Once the deploy ahead
// has ended, that includes the deploy waiting in the latest-pending slot,
// which is the one in progress then (w8/396).
//
//   - A deploy request (a trigger verb, the deploy hook, a git push, a
//     build-source fan-out) waits at the row level: deploys.Service opens it
//     as a queued deferred deploy that changes no App, and the
//     deferred-deploy worker runs it once the deploy ahead has ended.
//   - A saved change that rolls a release (a configuration save through a
//     Tracker, a Blueprint sync) cannot be held on the App without the
//     operator rendering the deploy ahead from it, so it waits here
//     (AwaitAdoption) until the deploy ahead is adopted, then writes through
//     and pends behind the operator's pin.
//   - A change saved for the next deploy (Render's Update Service and Update
//     Source: a save-only write through a Tracker, the source-pending
//     marker) waits the same way under either policy
//     (AwaitAdoptionBeforeHold, w6/163). The operator releases no deploy
//     stamped below a pending save, so written first the save held the
//     deploy ahead too, left created until another deploy canceled it.
//     Written once that deploy is adopted, the save leaves it running the
//     settings it was requested with and waits for the next deploy.

// DeployLister is the deploy-history read ServedRelease and the Wait rule
// need; *store.PGStore provides it.
type DeployLister interface {
	ListDeploys(ctx context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error)
}

// DeployAheadUnadopted reports that the deploy in progress for a carries a
// release the operator has not adopted yet. That is the deploy executing, or,
// once the deploy ahead of it has ended, the deploy waiting in the
// latest-pending slot: under Render's Wait it is the one in progress then, and
// the operator adopts it on its next pass (w8/396). The deploy ahead has ended
// when no row is executing, or when the App reports the release of the one
// still recorded executing served. A deploy waiting behind a release still in
// flight is not in progress, so a newer write replaces it, as Render's newest
// pending deploy replaces an older one. A stale App read errs toward waiting.
func DeployAheadUnadopted(ctx context.Context, lister DeployLister, a *appv1alpha1.App) (bool, error) {
	executing, waiting, aheadEnded, err := scanAhead(ctx, lister, a)
	if err != nil {
		return false, err
	}
	return executing || (aheadEnded && waiting), nil
}

// HoldAheadUnadopted reports that a change held for a's next deploy must wait:
// an executing release the operator has not adopted, or a release waiting in
// the latest-pending slot it has not adopted. Unlike DeployAheadUnadopted it
// does not ask whether the deploy ahead has ended: a held change starts no
// release, so it replaces nothing, and written before the waiting release is
// adopted the pending save holds that release too — or, once the deploy ahead
// is adopted, the waiting release runs and consumes the save, deploying
// settings it was requested without. The executing row's adoption is not the
// all-clear while a newer release still waits unadopted behind it.
func HoldAheadUnadopted(ctx context.Context, lister DeployLister, a *appv1alpha1.App) (bool, error) {
	executing, waiting, _, err := scanAhead(ctx, lister, a)
	if err != nil {
		return false, err
	}
	return executing || waiting, nil
}

// scanAhead reads a's open deploys once and reports the three facts the wait
// rules decide on: an executing release ahead of the adopted one, a release
// waiting in the latest-pending slot ahead of it, and whether the deploy
// ahead has ended (no row executing, or the App serving the still-recorded
// row's release).
func scanAhead(ctx context.Context, lister DeployLister, a *appv1alpha1.App) (executing, waiting, aheadEnded bool, err error) {
	appID := a.Labels[store.LabelAppID]
	if lister == nil || appID == "" {
		return false, false, true, nil
	}
	open, err := lister.ListDeploys(ctx, appID, store.DeployFilter{Statuses: store.OpenDeployStatuses()})
	if err != nil {
		return false, false, true, err
	}
	served, _ := appv1alpha1.RevisionGeneration(a.Status.ActiveRevision)
	aheadEnded = true
	for _, o := range open {
		switch {
		case o.OverlapPending:
			waiting = waiting || o.Generation > a.Status.ReleaseGeneration
		case o.Generation > a.Status.ReleaseGeneration:
			executing = true
		case o.Generation <= 0 || o.Generation > served:
			aheadEnded = false
		}
	}
	return executing, waiting, aheadEnded, nil
}

// DefaultAdoptionWait bounds AwaitAdoption. The operator adopts a release
// within a second or two of its write; one it has not adopted by then is
// stuck, and the write goes through as it did before w8/390.
const DefaultAdoptionWait = 15 * time.Second

// adoptionPoll is how often AwaitAdoption re-reads the App.
const adoptionPoll = 200 * time.Millisecond

// AwaitAdoption holds a write that rolls a's release while the deploy ahead of
// it carries a release the operator has not adopted (DeployAheadUnadopted),
// re-reading the App until it has, the deploy ahead has ended, or wait has
// passed (DefaultAdoptionWait when zero, no wait when negative). The write
// then pends behind the release in flight instead of replacing it.
//
// It never refuses the write: a read that fails, a service the operator has
// never adopted a release of, and Render's Override policy (the newest
// release replaces the one in flight at once) write through as before. A
// service's first release reads the settings saved before it starts
// (firstReleaseReadsSavedSource in the operator), so the settings a create
// writes after its first deploy opened, such as a Blueprint's copied
// values, still reach that deploy. a itself is not changed.
func AwaitAdoption(ctx context.Context, cl client.Reader, lister DeployLister, a *appv1alpha1.App, wait time.Duration) error {
	if a.Labels[appv1alpha1.LabelOverlappingDeployPolicy] == appv1alpha1.OverlappingDeployPolicyOverride {
		return nil
	}
	return awaitAdoption(ctx, cl, lister, a, wait, DeployAheadUnadopted)
}

// AwaitAdoptionBeforeHold is AwaitAdoption for a write that holds a change for
// a's next deploy (w6/163): a save-only write, or the source-pending marker.
// It waits under Render's Override policy too, since a held change starts no
// release that could replace the deploy ahead; it would only hold it. And it
// waits on the hold rule (HoldAheadUnadopted) rather than the deploy-ahead
// rule: a rolling write pends behind the release in flight, but a held change
// starts no release, so the executing row's adoption is not its all-clear
// while a newer release still waits unadopted behind it.
func AwaitAdoptionBeforeHold(ctx context.Context, cl client.Reader, lister DeployLister, a *appv1alpha1.App, wait time.Duration) error {
	return awaitAdoption(ctx, cl, lister, a, wait, HoldAheadUnadopted)
}

// awaitAdoption holds the calling write while ahead reports an unadopted
// release ahead of a, re-reading the App until none is, or wait has passed
// (DefaultAdoptionWait when zero, no wait when negative).
//
// It never refuses the write: a read that fails, a service the operator has
// never adopted a release of, and a wait that runs out write through as
// before, so a stuck operator cannot wedge saves. a itself is not changed.
func awaitAdoption(ctx context.Context, cl client.Reader, lister DeployLister, a *appv1alpha1.App, wait time.Duration, ahead func(context.Context, DeployLister, *appv1alpha1.App) (bool, error)) error {
	if wait < 0 || lister == nil || a.Labels[store.LabelAppID] == "" {
		return nil
	}
	if wait == 0 {
		wait = DefaultAdoptionWait
	}
	// A staged write reads the staged copy; the operator's status is on the
	// API server's.
	if st, ok := cl.(*Stage); ok {
		cl = st.Client
	}
	deadline := time.Now().Add(wait)
	// a's own read decides when nothing is in flight; a stale one errs
	// toward waiting, and each wait re-reads the App.
	live := a
	for {
		if live.Status.ReleaseGeneration == 0 {
			return nil
		}
		behind, err := ahead(ctx, lister, live)
		if err != nil || !behind {
			return ctx.Err()
		}
		if !time.Now().Before(deadline) {
			log.Printf("rollout: the deploy ahead of %s/%s was not adopted within %s; writing through", a.Namespace, a.Name, wait)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(adoptionPoll):
		}
		live = &appv1alpha1.App{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(a), live); err != nil {
			return ctx.Err()
		}
	}
}
