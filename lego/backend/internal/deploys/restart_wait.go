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

// Render's Wait overlap policy for Restart (w5/m284). Render calls a restart
// "a special form of manual deploy" and lets an in-progress deploy finish
// before the most recently triggered one runs — the default for every
// workspace created on or after 2025-07-14, which is every bex workspace. A
// Restart therefore waits for the deploy in flight rather than ending it, and
// restarts what is serving once that deploy settles: the new release if it
// went live, the previous one if it failed.

// RestartDeferrer is the store capability a waiting Restart needs;
// *store.PGStore provides it. Without it Restart ends the deploy in flight, as
// it did before w5/m284.
type RestartDeferrer interface {
	CreateDeferredRestart(ctx context.Context, appID, triggeredBy string) (store.Deploy, bool, error)
}

// DeferredRestartQueue is what the deferred-restart worker and Cancel read
// and close: a close lands only on a row still waiting.
type DeferredRestartQueue interface {
	ListDeferredRestarts(ctx context.Context, limit int) ([]store.DeferredRestart, error)
	CancelDeferredRestart(ctx context.Context, id, reason string) (bool, error)
}

// deferredRestartChecker is openRelease's pre-check for a deferred Restart.
type deferredRestartChecker interface {
	CheckDeferredRestart(ctx context.Context, id, appID string) error
}

var (
	_ RestartDeferrer        = (*store.PGStore)(nil)
	_ DeferredRestartQueue   = (*store.PGStore)(nil)
	_ deferredRestartChecker = (*store.PGStore)(nil)
)

// deferRestart records a Restart that waits for the deploy in flight.
// deferred=false means nothing is in flight (or the service has never been
// live, which Restart's own refusal covers), and the caller restarts now.
func (s *Service) deferRestart(ctx context.Context, service string, a *appv1alpha1.App) (DeployView, bool, error) {
	deferrer, ok := s.Store.(RestartDeferrer)
	appID := appStoreID(a)
	if !ok || appID == "" {
		return DeployView{}, false, nil
	}
	if err := s.validateTrigger(service, a, TriggerParams{restart: true}); err != nil {
		return DeployView{}, false, err
	}
	live, err := s.Store.ListDeploys(ctx, appID, store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
	if err != nil || len(live) == 0 {
		return DeployView{}, false, err
	}
	var d store.Deploy
	var deferred bool
	err = s.withTriggerLock(ctx, appID, func() error {
		var err error
		d, deferred, err = deferrer.CreateDeferredRestart(ctx, appID, core.SubjectFrom(ctx))
		return err
	})
	if err != nil || !deferred {
		return DeployView{}, false, err
	}
	return view(d), true, nil
}

// DeferredRestartWorker runs each waiting Restart once its app has no deploy
// in flight, and cancels one that has waited longer than any deploy can run.
type DeferredRestartWorker struct {
	Svc      *Service
	Interval time.Duration
	// MaxWait bounds a wait; past it the deploy ahead has outlived every gate
	// timeout (build 125m + rollout 18m), so something is stuck.
	MaxWait time.Duration
	Now     func() time.Time
}

const deferredRestartBatch = 20

func (w *DeferredRestartWorker) Run(ctx context.Context) {
	if w == nil || w.Svc == nil {
		return
	}
	if _, ok := w.Svc.Store.(DeferredRestartQueue); !ok {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	core.Poll(ctx, "deferred restart", interval, w.tick)
}

func (w *DeferredRestartWorker) tick(ctx context.Context) error {
	queue := w.Svc.Store.(DeferredRestartQueue)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	waiting, err := queue.ListDeferredRestarts(ctx, deferredRestartBatch)
	if err != nil {
		log.Printf("deferred restart: list: %v", err)
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
			w.Svc.runDeferredRestart(ctx, queue, r.Deploy)
		case now().Sub(r.CreatedAt) > maxWait:
			closeDeferredRestart(ctx, queue, r.ID, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonRestartWaitTimedOut,
				appv1alpha1.DeployReasonParam{Name: "wait", Value: maxWait.String()}))
		}
	}
	return nil
}

// runDeferredRestart restarts what serves now through the waiting row, so the
// deploy the caller was answered with is the one that runs.
func (s *Service) runDeferredRestart(ctx context.Context, queue DeferredRestartQueue, d store.Deploy) {
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.MatchingLabels{core.LabelAppID: d.AppID}); err != nil {
		log.Printf("deferred restart %s: find service: %v", d.ID, err)
		return
	}
	if len(list.Items) != 1 {
		closeDeferredRestart(ctx, queue, d.ID, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonRestartServiceGone))
		return
	}
	a := &list.Items[0]
	_, err := s.restartNow(store.WithDeferredRestart(ctx, d.ID), d.AppID, a)
	switch {
	case err == nil, errors.Is(err, store.ErrDeferredRestartGone):
	case errors.Is(err, store.ErrDeferredRestartNotReady):
		// Another deploy started since the list; the next tick retries.
	case errors.Is(err, core.ErrConflict), errors.Is(err, core.ErrBadRequest), errors.Is(err, core.ErrNotFound):
		closeDeferredRestart(ctx, queue, d.ID, "the restart could not run: "+err.Error())
	default:
		// Transient (the cluster or the database); the next tick retries.
		log.Printf("deferred restart %s: %v", d.ID, err)
	}
}

// closeDeferredRestart ends a waiting Restart with reason as its cancelReason.
// The fixed reasons are worded from the shared templates, which the dashboard
// words in its own language (w4/591).
func closeDeferredRestart(ctx context.Context, queue DeferredRestartQueue, id, reason string) {
	if _, err := queue.CancelDeferredRestart(ctx, id, reason); err != nil {
		log.Printf("deferred restart %s: cancel: %v", id, err)
	}
}
