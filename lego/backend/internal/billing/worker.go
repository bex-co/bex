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

package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type WorkerStore interface {
	ClaimDueBillingLifecycle(context.Context, time.Time, time.Duration) (store.BillingLifecycle, bool, error)
	CompleteBillingLifecycleWork(context.Context, string, int64, string, time.Time) (store.BillingLifecycle, error)
	FailBillingLifecycleWork(context.Context, string, int64, string, time.Time, time.Time) error
	ClaimBillingNotifications(context.Context, time.Time, time.Duration, int) ([]store.BillingNotification, error)
	CompleteBillingNotification(context.Context, string, int64, time.Time) error
	FailBillingNotification(context.Context, string, int64, string, time.Time) error
	PurgeStripeBillingEvents(context.Context, time.Time) (int64, error)
}

type BillingNotifier interface {
	NotifyBilling(context.Context, store.BillingNotification) error
}

type Worker struct {
	Store     WorkerStore
	Enforcer  ResourceEnforcer
	Notifier  BillingNotifier
	Interval  time.Duration
	Lease     time.Duration
	Clock     func() time.Time
	lastPurge time.Time
}

func (w *Worker) now() time.Time {
	if w.Clock != nil {
		return w.Clock().UTC()
	}
	return time.Now().UTC()
}

func (w *Worker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	core.Poll(ctx, "billing lifecycle", interval, w.RunOnce)
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if w == nil || w.Store == nil || w.Enforcer == nil {
		return fmt.Errorf("billing worker dependencies unavailable")
	}
	now := w.now()
	lease := w.Lease
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	for i := 0; i < 20; i++ {
		state, ok, err := w.Store.ClaimDueBillingLifecycle(ctx, now, lease)
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		var actionErr error
		completed := store.BillingEnforced
		if state.Status == store.BillingRecovering {
			actionErr = w.Enforcer.Recover(ctx, state)
			completed = state.RecoveryTarget
			if completed == "" {
				completed = store.BillingHealthy
			}
		} else {
			actionErr = w.Enforcer.Enforce(ctx, state)
		}
		if actionErr != nil {
			retry := now.Add(workerBackoff(state.AttemptCount))
			if err := w.Store.FailBillingLifecycleWork(ctx, state.WorkspaceID, state.TransitionVersion, actionErr.Error(), now, retry); err != nil {
				return err
			}
			continue
		}
		if _, err := w.Store.CompleteBillingLifecycleWork(ctx, state.WorkspaceID, state.TransitionVersion, completed, now); err != nil {
			return err
		}
	}
	if w.Notifier != nil {
		notifications, err := w.Store.ClaimBillingNotifications(ctx, now, lease, 20)
		if err != nil {
			return err
		}
		for _, n := range notifications {
			if err := w.Notifier.NotifyBilling(ctx, n); err != nil {
				if ferr := w.Store.FailBillingNotification(ctx, n.WorkspaceID, n.TransitionVersion, err.Error(), now.Add(workerBackoff(n.AttemptCount))); ferr != nil {
					return ferr
				}
				continue
			}
			if err := w.Store.CompleteBillingNotification(ctx, n.WorkspaceID, n.TransitionVersion, now); err != nil {
				return err
			}
		}
	}
	if w.lastPurge.IsZero() || now.Sub(w.lastPurge) >= 24*time.Hour {
		if _, err := w.Store.PurgeStripeBillingEvents(ctx, now.Add(-90*24*time.Hour)); err != nil {
			return err
		}
		w.lastPurge = now
	}
	return nil
}

func workerBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<(attempt-1)) * time.Minute
}
