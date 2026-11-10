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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w6/m319: a service suspended because its card-less workspace used its
// included bandwidth reports the billing suspender, cannot be resumed by hand
// that month without a payment method, and resumes once one is bound.
func TestBandwidthSuspensionRules(t *testing.T) {
	ctx := context.Background()
	month := FreeHoursMonth(time.Now())
	worker := func(svc *Service) BillingSuspensionWorker {
		return BillingSuspensionWorker{Svc: svc, Annotation: core.AnnotBandwidthSuspended}
	}

	svc, cl := newService(nil, sampleApp("web"))
	svc.Payment = &rejectingPaymentGate{}
	if err := worker(svc).Suspend(ctx, getApp(t, cl, "web"), month); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	v, err := svc.Get(ctx, "web")
	if err != nil || !v.Suspended || strings.Join(core.SuspendersFor(v.Suspended, v.BillingSuspended, false), ",") != "billing" {
		t.Fatalf("view = %+v err=%v, want suspended by billing", v, err)
	}
	var coded *core.CodedError
	if _, err := svc.Resume(ctx, "web"); !errors.As(err, &coded) || coded.Code != BandwidthErrorCode {
		t.Fatalf("manual resume without a card: want %s, got %v", BandwidthErrorCode, err)
	}
	if stored := getApp(t, cl, "web"); !stored.Spec.Suspended || stored.Annotations[core.AnnotBandwidthSuspended] != month {
		t.Fatal("a refused resume changed the suspension")
	}

	// A card bound: the owner may resume at once.
	svc.Payment = &acceptingPaymentGate{}
	if _, err := svc.Resume(ctx, "web"); err != nil {
		t.Fatalf("resume with a card: %v", err)
	}
	if stored := getApp(t, cl, "web"); stored.Spec.Suspended || stored.Annotations[core.AnnotBandwidthSuspended] != "" {
		t.Fatalf("resume left %+v / %v", stored.Spec.Suspended, stored.Annotations)
	}

	// A billing read that fails is not a card: the refusal fails closed.
	stuck, stuckCl := newService(nil, sampleApp("api"))
	stuck.Payment = erroringPaymentGate{}
	if err := worker(stuck).Suspend(ctx, getApp(t, stuckCl, "api"), month); err != nil {
		t.Fatal(err)
	}
	if _, err := stuck.Resume(ctx, "api"); !errors.Is(err, core.ErrBillingUnavailable) {
		t.Fatalf("resume on a failed billing read = %v, want ErrBillingUnavailable", err)
	}

	// Last month's suspension never blocks a resume.
	stale := sampleApp("old")
	stale.Spec.Suspended = true
	stale.Annotations = map[string]string{core.AnnotBandwidthSuspended: "2020-01"}
	old, _ := newService(nil, stale)
	old.Payment = &rejectingPaymentGate{}
	if _, err := old.Resume(ctx, "old"); err != nil {
		t.Fatalf("a previous month's suspension must not block resume: %v", err)
	}
}

// The worker never takes over an owner's suspension, so the owner's survives
// the month turn.
func TestBandwidthWorkerLeavesAnOwnersSuspension(t *testing.T) {
	ctx := context.Background()
	owner := sampleApp("paused")
	owner.Spec.Suspended = true
	svc, cl := newService(nil, owner)
	w := BillingSuspensionWorker{Svc: svc, Annotation: core.AnnotBandwidthSuspended}
	if err := w.Suspend(ctx, getApp(t, cl, "paused"), "2026-11"); err != nil {
		t.Fatal(err)
	}
	if err := w.Resume(ctx, getApp(t, cl, "paused")); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "paused"); !got.Spec.Suspended || got.Annotations[core.AnnotBandwidthSuspended] != "" {
		t.Fatalf("owner's suspension = %v / %v, want untouched", got.Spec.Suspended, got.Annotations)
	}
}

type erroringPaymentGate struct{}

func (erroringPaymentGate) RequirePaymentMethod(context.Context, string) error {
	return core.ErrBillingUnavailable
}
