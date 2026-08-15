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

const (
	// defaultInterval paces the tick; defaultLease is how long a claimed
	// lifecycle row or notification stays hidden from the other replica while
	// this worker settles it.
	defaultInterval = 5 * time.Second
	defaultLease    = 2 * time.Minute
	// drainBatch caps both queue drains in one tick, bounding how long a
	// backlog can hold the loop before the next poll.
	drainBatch = 20
	// purgeInterval and eventRetention are the Stripe-event sweep's cadence and
	// horizon.
	purgeInterval  = 24 * time.Hour
	eventRetention = 90 * 24 * time.Hour
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
		interval = defaultInterval
	}
	core.Poll(ctx, "billing lifecycle", interval, w.RunOnce)
}

// RunOnce is one tick: drain the lifecycle queue, drain the notification queue,
// then purge expired events. The three phases share only the tick's clock and
// lease; each is independent and returns its own errors.
func (w *Worker) RunOnce(ctx context.Context) error {
	if w == nil || w.Store == nil || w.Enforcer == nil {
		return fmt.Errorf("billing worker dependencies unavailable")
	}
	now := w.now()
	lease := w.Lease
	if lease <= 0 {
		lease = defaultLease
	}
	if err := w.drainLifecycle(ctx, now, lease); err != nil {
		return err
	}
	if err := w.drainNotifications(ctx, now, lease); err != nil {
		return err
	}
	return w.purgeEvents(ctx, now)
}

// drainLifecycle claims and settles up to drainBatch leased transitions. Each
// claim is one round trip (SKIP LOCKED leases a single row), so the bound caps
// how long one tick can hold the loop.
func (w *Worker) drainLifecycle(ctx context.Context, now time.Time, lease time.Duration) error {
	for i := 0; i < drainBatch; i++ {
		state, ok, err := w.Store.ClaimDueBillingLifecycle(ctx, now, lease)
		if err != nil {
			return err
		}
		if !ok {
			return nil
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
	return nil
}

// drainNotifications delivers one claimed batch, rescheduling a failed delivery
// on the shared backoff rather than failing the tick.
func (w *Worker) drainNotifications(ctx context.Context, now time.Time, lease time.Duration) error {
	if w.Notifier == nil {
		return nil
	}
	notifications, err := w.Store.ClaimBillingNotifications(ctx, now, lease, drainBatch)
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
	return nil
}

// purgeEvents drops Stripe events past the retention horizon, at most once per
// purgeInterval. The zero lastPurge deliberately purges on the first tick after
// start, so retention still runs on a process that restarts more often than the
// interval.
func (w *Worker) purgeEvents(ctx context.Context, now time.Time) error {
	if !w.lastPurge.IsZero() && now.Sub(w.lastPurge) < purgeInterval {
		return nil
	}
	if _, err := w.Store.PurgeStripeBillingEvents(ctx, now.Add(-eventRetention)); err != nil {
		return err
	}
	w.lastPurge = now
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
