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

// Package freehours enforces Render's monthly Free instance-hours budget
// (w8/m139): each workspace's Free web services share 750 instance hours per
// UTC calendar month, counted from the running-pod meter. At 80% the owners
// get a warning email; when the budget is spent every Free web service of the
// workspace is suspended (suspender "billing") and the owners are told. At the
// start of the next month, or when a service leaves the free plan, the
// suspension lifts. Services their owner suspended are never touched.
package freehours

import (
	"context"
	"errors"
	"fmt"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// DefaultBudgetHours is Render's allowance per workspace per month
// (render.com/docs/free).
const DefaultBudgetHours = 750

// warnFraction is the share of the budget at which the warning email goes out.
const warnFraction = 0.8

// Notice stages, also the free_hours_notices.stage values.
const (
	StageWarned    = "warned"
	StageExhausted = "exhausted"
)

// Store is the persistence the sweeper needs; *store.PGStore satisfies it.
type Store interface {
	FreeInstanceSecondsByWorkspace(ctx context.Context, monthStart, now time.Time) (map[string]int64, error)
	ClaimFreeHoursNotice(ctx context.Context, workspaceID string, month time.Time, stage string) (bool, error)
}

// Notifier mails a workspace's owners about its Free instance hours.
type Notifier interface {
	NotifyFreeHours(ctx context.Context, workspaceID, stage string, usedHours, budgetHours float64) error
}

// Sweeper accounts Free instance hours and suspends or resumes Free web
// services. Notices are claimed in the database before they are sent, so
// concurrent bex-api replicas send each once; suspensions are idempotent.
type Sweeper struct {
	Apps     apps.FreeHoursWorker
	Store    Store
	Notifier Notifier
	// BudgetHours is the monthly allowance; non-positive means 750.
	BudgetHours float64
	// Interval between sweeps; non-positive means five minutes.
	Interval time.Duration
}

func (w *Sweeper) budgetHours() float64 {
	if w.BudgetHours > 0 {
		return w.BudgetHours
	}
	return DefaultBudgetHours
}

// Run sweeps until ctx is canceled. A nil sweeper or unwired Service is a no-op.
func (w *Sweeper) Run(ctx context.Context) {
	if w == nil || w.Apps.Svc == nil || w.Apps.Svc.Base == nil || w.Apps.Svc.Client == nil || w.Store == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	core.Poll(ctx, "free instance hours", interval, w.SweepOnce)
}

// SweepOnce brings every workspace's Free web services in line with its
// month-to-date Free instance hours.
func (w *Sweeper) SweepOnce(ctx context.Context) error {
	now := w.Apps.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	month := apps.FreeHoursMonth(now)
	totals, err := w.Store.FreeInstanceSecondsByWorkspace(ctx, monthStart, now)
	if err != nil {
		return fmt.Errorf("free instance hours: %w", err)
	}
	budget := w.budgetHours()
	// Keep thresholds in the meter's unit: 0.05 hours warns at exactly 144s.
	budgetSeconds := budget * time.Hour.Seconds()
	exhausted := map[string]bool{}
	var errs []error
	for workspaceID, seconds := range totals {
		used := float64(seconds) / 3600
		switch {
		case float64(seconds) >= budgetSeconds:
			exhausted[workspaceID] = true
			// Skip the warning a later sweep would otherwise still owe.
			if _, err := w.Store.ClaimFreeHoursNotice(ctx, workspaceID, monthStart, StageWarned); err != nil {
				errs = append(errs, err)
			}
			errs = append(errs, w.notify(ctx, workspaceID, monthStart, StageExhausted, used, budget))
		case float64(seconds) >= budgetSeconds*warnFraction:
			errs = append(errs, w.notify(ctx, workspaceID, monthStart, StageWarned, used, budget))
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
		if err := w.reconcileApp(ctx, &items[i], month, exhausted); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", items[i].Name, err))
		}
	}
	return errors.Join(errs...)
}

// notify claims stage for the month and, only when this replica won the claim,
// mails the owners.
func (w *Sweeper) notify(ctx context.Context, workspaceID string, monthStart time.Time, stage string, used, budget float64) error {
	won, err := w.Store.ClaimFreeHoursNotice(ctx, workspaceID, monthStart, stage)
	if err != nil || !won || w.Notifier == nil {
		return err
	}
	return w.Notifier.NotifyFreeHours(ctx, workspaceID, stage, used, budget)
}

func (w *Sweeper) reconcileApp(ctx context.Context, a *appv1alpha1.App, month string, exhausted map[string]bool) error {
	if !a.DeletionTimestamp.IsZero() {
		return nil
	}
	suspendedIn := a.Annotations[core.AnnotFreeHoursSuspended]
	if suspendedIn != "" {
		// The month ended, or the service is no longer on the free plan.
		if suspendedIn != month || core.PaidPlan(a.Spec.Tier) {
			return w.Apps.Resume(ctx, a)
		}
		return nil
	}
	if core.PaidPlan(a.Spec.Tier) || (a.Spec.Type != "" && a.Spec.Type != appv1alpha1.TypeWebService) {
		return nil
	}
	if !exhausted[a.Labels[core.LabelTenant]] {
		return nil
	}
	return w.Apps.Suspend(ctx, a, month)
}
