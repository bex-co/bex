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
// behind a deploy the operator has not adopted waits.
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

// DeployLister is the deploy-history read ServedRelease and the Wait rule
// need; *store.PGStore provides it.
type DeployLister interface {
	ListDeploys(ctx context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error)
}

// DeployAheadUnadopted reports that the deploy executing for a carries a
// release the operator has not adopted yet. A deploy waiting in the
// latest-pending slot is not executing, so a newer write replaces it, as
// Render's newest pending deploy replaces an older one. A stale App read errs
// toward waiting.
func DeployAheadUnadopted(ctx context.Context, lister DeployLister, a *appv1alpha1.App) (bool, error) {
	appID := a.Labels[store.LabelAppID]
	if lister == nil || appID == "" {
		return false, nil
	}
	open, err := lister.ListDeploys(ctx, appID, store.DeployFilter{Statuses: store.OpenDeployStatuses()})
	if err != nil {
		return false, err
	}
	for _, o := range open {
		if !o.OverlapPending && o.Generation > a.Status.ReleaseGeneration {
			return true, nil
		}
	}
	return false, nil
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
	if wait < 0 || lister == nil || a.Labels[store.LabelAppID] == "" ||
		a.Labels[appv1alpha1.LabelOverlappingDeployPolicy] == appv1alpha1.OverlappingDeployPolicyOverride {
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
		behind, err := DeployAheadUnadopted(ctx, lister, live)
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
