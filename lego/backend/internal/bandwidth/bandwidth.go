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

// Package bandwidth spins down the services of a workspace that has used its
// plan's included outbound bandwidth and has no payment method, as Render does
// (w6/m319, docs/ADR030 §14): "If you haven't added a payment method and you
// exceed your included bandwidth, Render spins down your workspace's services
// until the start of the next month." A workspace that can pay is billed for
// the bandwidth above its allowance instead. The suspension lifts at the month
// turn or once a payment method is bound; services their owner suspended are
// never touched.
package bandwidth

import (
	"context"
	"errors"
	"fmt"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// warnFraction is the share of the included bandwidth at which the warning
// email goes out, as for Free instance hours and build minutes.
const warnFraction = 0.8

// Notice stages, also the bandwidth_notices.stage values.
const (
	StageWarned    = "warned"
	StageExhausted = "exhausted"
)

// Store is the persistence the sweeper needs; *store.PGStore satisfies it.
type Store interface {
	EgressBytesByWorkspace(ctx context.Context, monthStart, now time.Time) ([]store.MonthUsage, error)
	ClaimBandwidthNotice(ctx context.Context, workspaceID string, month time.Time, stage string) (bool, error)
}

// Apps lists, suspends and resumes services; apps.BillingSuspensionWorker
// with core.AnnotBandwidthSuspended satisfies it.
type Apps interface {
	Now() time.Time
	List(ctx context.Context) ([]appv1alpha1.App, error)
	Suspend(ctx context.Context, a *appv1alpha1.App, month string) error
	Resume(ctx context.Context, a *appv1alpha1.App) error
}

// Notifier mails a workspace's billing owners about its included bandwidth.
type Notifier interface {
	NotifyBandwidth(ctx context.Context, workspaceID, stage string, usedGB, includedGB float64, canPay bool) error
}

// Sweeper suspends the services of every workspace out of included bandwidth
// that cannot pay for more, and resumes them when it can again. Notices are
// claimed in the database before they are sent, so concurrent bex-api
// replicas send each once; suspensions are idempotent.
type Sweeper struct {
	Apps     Apps
	Store    Store
	Notifier Notifier
	// Plans reads workspace plans for their included bandwidth, the same answer
	// the estimate and the Stripe emitter use.
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
	core.Poll(ctx, "included bandwidth", interval, w.SweepOnce)
}

// SweepOnce brings every workspace's services in line with its month-to-date
// outbound bandwidth.
func (w *Sweeper) SweepOnce(ctx context.Context) error {
	now := w.Apps.Now().UTC()
	monthStart := store.MonthStartUTC(now)
	month := apps.FreeHoursMonth(now)
	usage, err := w.Store.EgressBytesByWorkspace(ctx, monthStart, now)
	if err != nil {
		return fmt.Errorf("included bandwidth: %w", err)
	}
	spunDown := map[string]bool{}
	// unread: workspaces whose plan could not be read this sweep. Their
	// services stay as they are; resuming on a failed read would undo a
	// suspension the workspace still owes.
	unread := map[string]bool{}
	var errs []error
	for _, u := range usage {
		allowances, err := pricing.IncludedForWorkspace(ctx, w.Plans, u.WorkspaceID)
		if err != nil {
			unread[u.WorkspaceID] = true
			errs = append(errs, err)
			continue
		}
		included := allowances[store.UsageKindEgressBytes]
		if included <= 0 {
			continue // no plan applies: nothing to run out of
		}
		used, includedGB := float64(u.Quantity)/pricing.BytesPerGiB, float64(included)/pricing.BytesPerGiB
		switch {
		case u.Quantity >= included:
			spunDown[u.WorkspaceID] = !u.CanPay
			// Skip the warning a later sweep would otherwise still owe.
			if _, err := w.Store.ClaimBandwidthNotice(ctx, u.WorkspaceID, monthStart, StageWarned); err != nil {
				errs = append(errs, err)
			}
			errs = append(errs, w.notify(ctx, u, monthStart, StageExhausted, used, includedGB))
		case float64(u.Quantity) >= float64(included)*warnFraction:
			errs = append(errs, w.notify(ctx, u, monthStart, StageWarned, used, includedGB))
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
		workspaceID := a.Labels[core.LabelTenant]
		if !a.DeletionTimestamp.IsZero() || unread[workspaceID] {
			continue
		}
		var err error
		switch suspendedIn := a.Annotations[core.AnnotBandwidthSuspended]; {
		case suspendedIn != "" && (suspendedIn != month || !spunDown[workspaceID]):
			// The month ended, or the workspace can pay now.
			err = w.Apps.Resume(ctx, a)
		case suspendedIn == "" && spunDown[workspaceID]:
			err = w.Apps.Suspend(ctx, a, month)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.Name, err))
		}
	}
	return errors.Join(errs...)
}

// notify claims stage for the month and, only when this replica won the claim,
// mails the owners.
func (w *Sweeper) notify(ctx context.Context, u store.MonthUsage, monthStart time.Time, stage string, used, included float64) error {
	won, err := w.Store.ClaimBandwidthNotice(ctx, u.WorkspaceID, monthStart, stage)
	if err != nil || !won || w.Notifier == nil {
		return err
	}
	return w.Notifier.NotifyBandwidth(ctx, u.WorkspaceID, stage, used, included, u.CanPay)
}
