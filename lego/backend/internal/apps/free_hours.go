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
	"fmt"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// FreeHoursMonth is the UTC calendar month ("2006-01") the Free instance-hours
// budget accounts in (w8/m139).
func FreeHoursMonth(t time.Time) string { return t.UTC().Format("2006-01") }

// FreeHoursErrorCode is the refusal for resuming a service the monthly Free
// instance hours suspended.
const FreeHoursErrorCode = "FREE_INSTANCE_HOURS_EXHAUSTED"

// suspendForBilling suspends a for a spent monthly allowance, recording month
// under annotation (core.AnnotFreeHoursSuspended or
// core.AnnotBandwidthSuspended). It is an internal worker verb: no caller
// identity, no audit row. A service already suspended, by its owner or another
// allowance, is left alone, so the owner's suspension survives the month
// rollover.
func (s *Service) suspendForBilling(ctx context.Context, a *appv1alpha1.App, annotation, month string) error {
	if a.Spec.Suspended || a.Annotations[annotation] != "" {
		return nil
	}
	_, err := s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, true) },
		func(a *appv1alpha1.App) {
			a.Spec.Suspended = true
			if a.Annotations == nil {
				a.Annotations = map[string]string{}
			}
			a.Annotations[annotation] = month
		})
	return err
}

// resumeFromBilling lifts a suspension suspendForBilling recorded under
// annotation. A service not suspended by that allowance is untouched.
func (s *Service) resumeFromBilling(ctx context.Context, a *appv1alpha1.App, annotation string) error {
	if a.Annotations[annotation] == "" {
		return nil
	}
	_, err := s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, false) },
		func(a *appv1alpha1.App) {
			a.Spec.Suspended = false
			delete(a.Annotations, annotation)
		})
	return err
}

// requireFreeHoursAvailable refuses a manual resume of a service the budget
// suspended while it is still the same month on the free plan.
func (s *Service) requireFreeHoursAvailable(a *appv1alpha1.App) error {
	month := a.Annotations[core.AnnotFreeHoursSuspended]
	if month == "" || month != FreeHoursMonth(s.Now()) || core.PaidPlan(a.Spec.Tier) {
		return nil
	}
	return core.NewBadRequestError(FreeHoursErrorCode,
		fmt.Sprintf("this workspace has used its monthly Free instance hours; Free web services resume at the start of next month, or upgrade %q to a paid plan to run it now", core.AppPublicName(a)),
		map[string]any{"month": month})
}

// BillingSuspensionWorker exposes one spent-allowance suspension to its
// sweeper: Annotation is core.AnnotFreeHoursSuspended (Free instance hours) or
// core.AnnotBandwidthSuspended (included bandwidth, w6/m319). They are worker
// operations with no caller identity, so they live on their own type rather
// than as verbs of Service.
type BillingSuspensionWorker struct {
	Svc        *Service
	Annotation string
}

// Now is the service clock.
func (w BillingSuspensionWorker) Now() time.Time { return w.Svc.Now() }

// Suspend suspends a for the month's spent allowance.
func (w BillingSuspensionWorker) Suspend(ctx context.Context, a *appv1alpha1.App, month string) error {
	return w.Svc.suspendForBilling(ctx, a, w.Annotation, month)
}

// Resume lifts the allowance's suspension.
func (w BillingSuspensionWorker) Resume(ctx context.Context, a *appv1alpha1.App) error {
	return w.Svc.resumeFromBilling(ctx, a, w.Annotation)
}

// List returns every service App in the cluster.
func (w BillingSuspensionWorker) List(ctx context.Context) ([]appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := w.Svc.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	return core.ServiceApps(list.Items), nil
}
