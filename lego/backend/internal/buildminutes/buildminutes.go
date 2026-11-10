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

// Package buildminutes stops a workspace's builds when it has used its plan's
// included pipeline minutes and cannot buy more, as Render does (w6/m308,
// docs/ADR030 §13): "If you run out of pipeline minutes during a given month,
// you automatically purchase a supplementary amount of minutes for your current
// tier, unless ... you haven't added a payment method. In the above cases,
// Render stops running pipeline tasks for the remainder of the current month."
package buildminutes

import (
	"context"
	"errors"
	"fmt"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// warnFraction is the share of the included minutes at which the warning email
// goes out, as for Free instance hours.
const warnFraction = 0.8

// Notice stages, also the build_minutes_notices.stage values.
const (
	StageWarned    = "warned"
	StageExhausted = "exhausted"
	// StageLimited: a workspace that can pay reached its pipeline spend limit
	// (w6/m309), so its builds stop too.
	StageLimited = "limited"
)

// Store is the persistence the sweeper needs; *store.PGStore satisfies it.
type Store interface {
	BuildSecondsByWorkspace(ctx context.Context, monthStart, now time.Time) ([]store.BuildMinutesUsage, error)
	ClaimBuildMinutesNotice(ctx context.Context, workspaceID string, month time.Time, stage string) (bool, error)
}

// Apps lists and marks Apps; apps.BuildMinutesWorker satisfies it.
type Apps interface {
	Now() time.Time
	List(ctx context.Context) ([]appv1alpha1.App, error)
	Mark(ctx context.Context, a *appv1alpha1.App, month string) error
}

// Notifier mails a workspace's billing owners about its build minutes.
type Notifier interface {
	NotifyBuildMinutes(ctx context.Context, workspaceID, stage string, usedMinutes, includedMinutes float64, canPay bool) error
}

// Sweeper marks the Apps of every workspace that is out of build minutes and
// cannot pay for more, and lifts the mark when it can again (a payment method,
// or a new month). Notices are claimed in the database before they are sent,
// so concurrent bex-api replicas send each once; marks are idempotent.
type Sweeper struct {
	Apps     Apps
	Store    Store
	Notifier Notifier
	// Plans reads workspace plans for their included build minutes, the
	// same answer the estimate and the Stripe emitter use.
	Plans pricing.PlanReader
	// Interval between sweeps; non-positive means five minutes.
	Interval time.Duration
}

// Run sweeps until ctx is canceled. A nil sweeper is a no-op.
func (w *Sweeper) Run(ctx context.Context) {
	if w == nil || w.Apps == nil || w.Store == nil || w.Plans == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	core.Poll(ctx, "build minutes", interval, w.SweepOnce)
}

// SweepOnce brings every workspace's Apps in line with its month-to-date build
// minutes.
func (w *Sweeper) SweepOnce(ctx context.Context) error {
	now := w.Apps.Now().UTC()
	monthStart := store.MonthStartUTC(now)
	month := appv1alpha1.BuildMinutesMonth(now)
	usage, err := w.Store.BuildSecondsByWorkspace(ctx, monthStart, now)
	if err != nil {
		return fmt.Errorf("build minutes: %w", err)
	}
	stopped := map[string]bool{}
	var errs []error
	for _, u := range usage {
		included, err := pricing.IncludedBuildSecondsFor(ctx, w.Plans, u.WorkspaceID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if included <= 0 {
			continue // no plan applies: nothing to run out of
		}
		used, includedMin := float64(u.Seconds)/60, float64(included)/60
		switch {
		case u.Seconds >= included:
			// Past its included minutes a workspace buys more, unless it has no
			// payment method or has reached its spend limit (w6/m309).
			limited := u.CanPay && u.SpendLimitCents != nil &&
				pricing.Default.BuildCentsAbove(u.Seconds, included) >= *u.SpendLimitCents
			stopped[u.WorkspaceID] = !u.CanPay || limited
			// Earlier stages a later one supersedes are claimed silently: a
			// workspace stopped by its limit is never told minutes are billed
			// on, nor told it is nearly out.
			skip := []string{StageWarned}
			send := StageExhausted
			if limited {
				skip, send = append(skip, StageExhausted), StageLimited
			}
			for _, stage := range skip {
				if _, err := w.Store.ClaimBuildMinutesNotice(ctx, u.WorkspaceID, monthStart, stage); err != nil {
					errs = append(errs, err)
				}
			}
			errs = append(errs, w.notify(ctx, u, monthStart, send, used, includedMin))
		case float64(u.Seconds) >= float64(included)*warnFraction:
			errs = append(errs, w.notify(ctx, u, monthStart, StageWarned, used, includedMin))
		}
	}

	items, err := w.Apps.List(ctx)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("list apps: %w", err))...)
	}
	for i := range items {
		if ctx.Err() != nil {
			break
		}
		a := &items[i]
		if !a.DeletionTimestamp.IsZero() {
			continue
		}
		want := ""
		if stopped[a.Labels[core.LabelTenant]] {
			want = month
		}
		if err := w.Apps.Mark(ctx, a, want); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.Name, err))
		}
	}
	return errors.Join(errs...)
}

// notify claims stage for the month and, only when this replica won the claim,
// mails the owners.
func (w *Sweeper) notify(ctx context.Context, u store.BuildMinutesUsage, monthStart time.Time, stage string, used, included float64) error {
	won, err := w.Store.ClaimBuildMinutesNotice(ctx, u.WorkspaceID, monthStart, stage)
	if err != nil || !won || w.Notifier == nil {
		return err
	}
	return w.Notifier.NotifyBuildMinutes(ctx, u.WorkspaceID, stage, used, included, u.CanPay)
}
