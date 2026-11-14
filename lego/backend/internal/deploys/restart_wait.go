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

package deploys

import (
	"context"
	"errors"
	"log"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Wait overlap policy for the deploys bex-api opens without the
// operator's pin. Render lets an in-progress deploy finish before the most
// recently triggered one runs — the default for every workspace created on or
// after 2025-07-14, which is every bex workspace — and that holds for a restart
// ("a special form of manual deploy") and a rollback ("a new deploy using the
// target deploy's build artifact") as for any other deploy
// (render.com/docs/deploys, render.com/docs/rollbacks).
//
// The operator pins a source build or a saved prebuilt image in flight
// (w8/m119, w4/635), but a release that selects its own runtime (Restart,
// Rollback, an image override) is neither, so such a deploy waits here, at the
// row level: it holds the latest-pending slot with no release and no App CR
// change, and runs once nothing is in flight. A Restart waits behind any deploy
// (w5/m284), restarting what serves once that deploy settles; a Rollback or an
// image override waits behind any deploy, and any deploy this service opens
// waits behind one of those (w4/652).

// DeployDeferrer is the store capability a waiting deploy needs;
// *store.PGStore provides it. Without it the deploy replaces the one in
// flight, as it did before w5/m284.
type DeployDeferrer interface {
	CreateDeferredDeploy(ctx context.Context, d store.Deploy, req store.DeferredRequest) (store.Deploy, bool, error)
}

// DeferredDeployQueue is what the deferred-deploy worker and Cancel read and
// close: a close lands only on a row still waiting.
type DeferredDeployQueue interface {
	ListDeferredDeploys(ctx context.Context, limit int) ([]store.DeferredDeploy, error)
	CancelDeferredDeploy(ctx context.Context, id, reason string) (bool, error)
}

// deferredDeployChecker is openRelease's pre-check for a deferred deploy.
type deferredDeployChecker interface {
	CheckDeferredDeploy(ctx context.Context, id, appID string) error
}

var (
	_ DeployDeferrer        = (*store.PGStore)(nil)
	_ DeferredDeployQueue   = (*store.PGStore)(nil)
	_ deferredDeployChecker = (*store.PGStore)(nil)
)

// deferDeploy records d, a deploy of a, as waiting for the deploy in flight
// when waits says it must, judged on a as re-read under the trigger lock.
// deferred=false means nothing is in flight, waits declined, or the deploy is
// a deferred one already running (the worker's own call), and the caller
// deploys now.
func (s *Service) deferDeploy(ctx context.Context, a *appv1alpha1.App, d store.Deploy, req store.DeferredRequest, waits func(*appv1alpha1.App) bool) (DeployView, bool, error) {
	deferrer, ok := s.Store.(DeployDeferrer)
	d.AppID = appStoreID(a)
	if !ok || d.AppID == "" || store.DeferredDeployID(ctx) != "" {
		return DeployView{}, false, nil
	}
	var deferred bool
	err := s.withTriggerLock(ctx, d.AppID, func() error {
		if err := core.RereadApp(ctx, s.Client, a); err != nil {
			return err
		}
		if err := core.NotFoundIfDeleting(a); err != nil || !waits(a) {
			return err
		}
		var err error
		d, deferred, err = deferrer.CreateDeferredDeploy(ctx, d, req)
		return err
	})
	if err != nil || !deferred {
		return DeployView{}, false, err
	}
	return view(a, d), true, nil
}

// alwaysWaits is deferDeploy's rule for a deploy that selects its own release
// (Restart, Rollback): it waits behind whatever is in flight.
func alwaysWaits(*appv1alpha1.App) bool { return true }

// deferRestart records a Restart that waits for the deploy in flight.
// deferred=false means nothing is in flight (or the service has never been
// live, which Restart's own refusal covers), and the caller restarts now.
func (s *Service) deferRestart(ctx context.Context, service string, a *appv1alpha1.App) (DeployView, bool, error) {
	if _, ok := s.Store.(DeployDeferrer); !ok || appStoreID(a) == "" {
		return DeployView{}, false, nil
	}
	if err := s.validateTrigger(service, a, TriggerParams{restart: true}); err != nil {
		return DeployView{}, false, err
	}
	live, err := s.Store.ListDeploys(ctx, appStoreID(a), store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
	if err != nil || len(live) == 0 {
		return DeployView{}, false, err
	}
	return s.deferDeploy(ctx, a, store.Deploy{Trigger: store.TriggerRestart, TriggeredBy: core.SubjectFrom(ctx)}, store.DeferredRequest{}, alwaysWaits)
}

// DeferredDeployWorker runs each waiting deploy once its app has no deploy in
// flight, and cancels one that has waited longer than any deploy can run.
type DeferredDeployWorker struct {
	Svc      *Service
	Interval time.Duration
	// MaxWait bounds a wait; past it the deploy ahead has outlived every gate
	// timeout (build 125m + rollout 18m), so something is stuck.
	MaxWait time.Duration
	Now     func() time.Time
}

const deferredDeployBatch = 20

func (w *DeferredDeployWorker) Run(ctx context.Context) {
	if w == nil || w.Svc == nil {
		return
	}
	if _, ok := w.Svc.Store.(DeferredDeployQueue); !ok {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	core.Poll(ctx, "deferred deploy", interval, w.tick)
}

func (w *DeferredDeployWorker) tick(ctx context.Context) error {
	queue := w.Svc.Store.(DeferredDeployQueue)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	waiting, err := queue.ListDeferredDeploys(ctx, deferredDeployBatch)
	if err != nil {
		log.Printf("deferred deploy: list: %v", err)
		return nil
	}
	maxWait := w.MaxWait
	if maxWait <= 0 {
		maxWait = 3 * time.Hour
	}
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	for _, r := range waiting {
		if ctx.Err() != nil {
			return nil
		}
		switch {
		case r.Ready:
			w.Svc.runDeferredDeploy(ctx, queue, r)
		case now().Sub(r.CreatedAt) > maxWait:
			code := appv1alpha1.DeployReasonDeployWaitTimedOut
			if r.Trigger == store.TriggerRestart {
				code = appv1alpha1.DeployReasonRestartWaitTimedOut
			}
			closeDeferredDeploy(ctx, queue, r.ID, appv1alpha1.FormatDeployReason(code,
				appv1alpha1.DeployReasonParam{Name: "wait", Value: maxWait.String()}))
		}
	}
	return nil
}

// runDeferredDeploy runs the waiting deploy's verb through its row, so the
// deploy the caller was answered with is the one that runs. A Restart restarts
// what serves now; a Rollback and any other deploy run as asked.
func (s *Service) runDeferredDeploy(ctx context.Context, queue DeferredDeployQueue, r store.DeferredDeploy) {
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.MatchingLabels{core.LabelAppID: r.AppID}); err != nil {
		log.Printf("deferred deploy %s: find service: %v", r.ID, err)
		return
	}
	if len(list.Items) != 1 {
		closeDeferredDeploy(ctx, queue, r.ID, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonRestartServiceGone))
		return
	}
	a := &list.Items[0]
	ctx = store.WithDeferredDeploy(ctx, r.ID)
	service := core.AppPublicName(a)
	var err error
	switch r.Trigger {
	case store.TriggerRestart:
		_, err = s.restartNow(ctx, service, a)
	case store.TriggerRollback:
		_, err = s.rollbackFetched(ctx, service, a, r.RollbackOf, RollbackOptions{DisableAutoDeploy: r.Request.DisableAutoDeploy})
	default:
		_, err = s.triggerFetched(ctx, service, a, TriggerParams{
			ImageURL: r.Request.ImageURL, CommitID: r.Request.CommitID, BuildID: r.Request.BuildID,
			DeployMode: r.Request.DeployMode, ClearCache: r.Request.ClearCache,
		}, r.Trigger)
	}
	switch {
	case err == nil, errors.Is(err, store.ErrDeferredDeployGone):
	case errors.Is(err, store.ErrDeferredDeployNotReady):
		// Another deploy started since the list; the next tick retries.
	case errors.Is(err, core.ErrConflict), errors.Is(err, core.ErrBadRequest), errors.Is(err, core.ErrNotFound):
		closeDeferredDeploy(ctx, queue, r.ID, "the "+deferredVerb(r.Trigger)+" could not run: "+err.Error())
	default:
		// Transient (the cluster or the database); the next tick retries.
		log.Printf("deferred deploy %s: %v", r.ID, err)
	}
}

// deferredVerb names a waiting deploy's verb in its cancel reason.
func deferredVerb(trigger string) string {
	switch trigger {
	case store.TriggerRestart:
		return "restart"
	case store.TriggerRollback:
		return "rollback"
	}
	return "deploy"
}

// closeDeferredDeploy ends a waiting deploy with reason as its cancelReason.
// The fixed reasons are worded from the shared templates, which the dashboard
// words in its own language (w4/591).
func closeDeferredDeploy(ctx context.Context, queue DeferredDeployQueue, id, reason string) {
	if _, err := queue.CancelDeferredDeploy(ctx, id, reason); err != nil {
		log.Printf("deferred deploy %s: cancel: %v", id, err)
	}
}
