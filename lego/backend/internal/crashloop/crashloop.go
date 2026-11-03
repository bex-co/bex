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

// Package crashloop suspends services that keep failing to start, as Render
// does: "Render suspends a service that repeatedly fails to start and become
// healthy over a 24-hour period" (render.com/docs/notifications), reported
// with the suspender stuck_crashlooping (w2/m253). The owners are emailed; the
// suspension itself is observed as service_suspended for the Events feed,
// push and webhooks. Resuming by hand clears it.
package crashloop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// DefaultWindow is Render's 24 hours.
const DefaultWindow = 24 * time.Hour

// Notifier tells a service's workspace it is being suspended and why.
type Notifier interface {
	NotifyCrashLoopSuspended(ctx context.Context, tenantID, serviceID, name string, failingSince, at time.Time) error
}

// Sweeper finds crash-looping services and suspends them. Suspension is
// idempotent, so concurrent bex-api replicas converge.
type Sweeper struct {
	Apps     apps.CrashLoopWorker
	Notifier Notifier
	// Window is how long a service must fail before it is suspended;
	// non-positive means DefaultWindow.
	Window time.Duration
	// Interval between sweeps; non-positive means five minutes.
	Interval time.Duration
}

// Run sweeps until ctx is canceled. A nil sweeper or unwired Service is a no-op.
func (w *Sweeper) Run(ctx context.Context) {
	if w == nil || w.Apps.Svc == nil || w.Apps.Svc.Base == nil || w.Apps.Svc.Client == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	core.Poll(ctx, "crash-loop suspension", interval, w.SweepOnce)
}

func (w *Sweeper) window() time.Duration {
	if w.Window > 0 {
		return w.Window
	}
	return DefaultWindow
}

// SweepOnce suspends every service that has been failing for the window. The
// window starts at the later of Ready's transition and the sweeper's first
// sighting at the current spec generation, so a resume or a new deploy starts
// it again. The owners are notified before the suspension is written: the
// alert is idempotent, so a failed write retries without a second email, and a
// failed notification is retried before anything is suspended.
func (w *Sweeper) SweepOnce(ctx context.Context) error {
	list, err := w.Apps.List(ctx)
	if err != nil {
		return fmt.Errorf("list apps: %w", err)
	}
	now := w.Apps.Now()
	var errs []error
	for i := range list {
		a := &list[i]
		since, failing := apps.CrashLoopFailingSince(a)
		if !failing {
			if a.Annotations[core.AnnotCrashLoopObserved] != "" {
				errs = append(errs, w.Apps.Observe(ctx, a, ""))
			}
			continue
		}
		observed, ok := apps.CrashLoopObservedAt(a)
		if !ok {
			errs = append(errs, w.Apps.Observe(ctx, a, apps.CrashLoopObservation(a.Generation, now)))
			continue
		}
		if observed.After(since) {
			since = observed
		}
		if now.Sub(since) < w.window() {
			continue
		}
		if w.Notifier != nil {
			if err := w.Notifier.NotifyCrashLoopSuspended(ctx, a.Labels[core.LabelTenant], core.AppPublicID(a), core.AppPublicName(a), since, now); err != nil {
				errs = append(errs, fmt.Errorf("notify %s: %w", a.Name, err))
				continue
			}
		}
		if err := w.Apps.Suspend(ctx, a, now); err != nil {
			errs = append(errs, fmt.Errorf("suspend %s: %w", a.Name, err))
			continue
		}
		log.Printf("crashloop: suspended %s (failing to start since %s)", core.AppPublicName(a), since.UTC().Format(time.RFC3339))
	}
	return errors.Join(errs...)
}
