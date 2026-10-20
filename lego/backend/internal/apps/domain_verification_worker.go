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
	"log"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// domain_verification_worker.go (w5/m137) is Render's automatic domain
// verification: a pending claim promotes by itself once its traffic DNS
// record propagates, without the tenant clicking Verify. It runs the exact
// promote path the manual verb runs (checkAndPromoteClaim), so the two can
// never disagree about what "verified" means.

const domainVerificationBatch = 100

// DomainVerificationWorker periodically re-checks unverified custom-domain
// claims. Run is a no-op when the claim store is unwired (storeless mode has
// no pending rows — its synchronous gate already ran at add time).
type DomainVerificationWorker struct {
	Svc *Service
	// Interval between sweep ticks; non-positive means 30s.
	Interval time.Duration
}

// Run drives the loop until ctx is canceled.
func (w *DomainVerificationWorker) Run(ctx context.Context) {
	if w == nil || w.Svc == nil {
		return
	}
	if _, ok := w.Svc.Store.(domainVerificationStore); !ok {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	core.Poll(ctx, "domain verification", interval, w.tick)
}

func (w *DomainVerificationWorker) tick(ctx context.Context) error {
	claims0, ok := w.Svc.Store.(domainVerificationStore)
	if !ok {
		return nil
	}
	pending, err := claims0.ListPendingDomainClaims(ctx, domainVerificationBatch)
	if err != nil {
		return err
	}
	// Resolve each App CR once per tick: an auto-paired apex+www are two
	// claims on one App, and the client is uncached.
	appsByID := map[string]*appv1alpha1.App{}
	for _, claim := range pending {
		if ctx.Err() != nil {
			return nil
		}
		app, seen := appsByID[claim.AppID]
		if !seen {
			app = w.resolveApp(ctx, claim.AppID)
			appsByID[claim.AppID] = app
		}
		if app == nil {
			continue
		}
		w.checkOne(ctx, claims0, app, claim)
	}
	return nil
}

// resolveApp finds the App CR for a claim's app id. The sweep is a system
// actor: it resolves by the stable app-id label, not through any caller
// authorization (there is no caller). Nil means not projected yet — the
// claim is retried next tick.
func (w *DomainVerificationWorker) resolveApp(ctx context.Context, appID string) *appv1alpha1.App {
	var list appv1alpha1.AppList
	if err := w.Svc.Client.List(ctx, &list, client.MatchingLabels{core.LabelAppID: appID}); err != nil {
		log.Printf("domain verification: list app %s: %v", appID, err)
		return nil
	}
	if len(list.Items) == 0 {
		return nil
	}
	return &list.Items[0]
}

func (w *DomainVerificationWorker) checkOne(ctx context.Context, claims domainClaimStore, app *appv1alpha1.App, claim store.Domain) {
	if _, err := w.Svc.checkAndPromoteClaim(ctx, app, claims, claim.AppID, claim); err != nil {
		log.Printf("domain verification: %s on %s: %v", claim.Host, claim.AppID, err)
	}
}
