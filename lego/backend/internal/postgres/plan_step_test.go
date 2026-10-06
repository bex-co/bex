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

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestSetVersionRunsTheUpdateChecks (w5/m116): GraphQL's updateDatabaseVersion
// runs UpdatePostgres's checks, so a protected database's major upgrade needs
// the "upgrade" phrase a PATCH does, and the dunning gate applies. Before, it
// skipped both.
func TestSetVersionRunsTheUpdateChecks(t *testing.T) {
	db := databaseForProtection("dpg-orders", "orders", true)
	svc, _, _ := protectedPostgresService(db)
	if _, err := svc.SetVersion(context.Background(), db.Name, "17"); !errors.Is(err, core.ErrBadRequest) ||
		!strings.Contains(err.Error(), ProtectedConfirmation("upgrade", "orders")) {
		t.Fatalf("SetVersion on a protected database = %v, want the upgrade confirmation", err)
	}

	ledger := databaseForProtection("dpg-ledger", "ledger", false)
	ledger.Labels[core.LabelTenant] = "tea-a"
	enforced, _ := newService(ledger)
	enforced.Billing = &refusingBillingGate{}
	if _, err := enforced.SetVersion(context.Background(), ledger.Name, "17"); !errors.Is(err, core.ErrBillingEnforced) {
		t.Fatalf("SetVersion while billing enforcement holds the workspace = %v, want the dunning refusal", err)
	}
}

// TestUpdatePostgresTakesThePlanGate (w5/m116): a PATCH that changes a billed
// dimension meets the same plan gate SetPlan does — with the all-plans payment
// rule, even a change to the free plan needs a payment method on file. Before,
// the PATCH checked a payment method only for a paid plan.
func TestUpdatePostgresTakesThePlanGate(t *testing.T) {
	svc, _ := newService(databaseForProtection("dpg-ledger", "ledger", false))
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	svc.PaymentAllPlans = true
	free := "free"
	for name, call := range map[string]func() error{
		"SetPlan": func() error { _, err := svc.SetPlan(context.Background(), "dpg-ledger", free); return err },
		"UpdatePostgres": func() error {
			_, err := svc.UpdatePostgres(context.Background(), "dpg-ledger", PostgresPatch{Plan: &free})
			return err
		},
		"UpdatePostgresDryRun": func() error {
			_, err := svc.UpdatePostgresDryRun(context.Background(), "dpg-ledger", PostgresPatch{Plan: &free})
			return err
		},
	} {
		if err := call(); !errors.Is(err, core.ErrPaymentRequired) {
			t.Errorf("%s to free with no payment method = %v, want the 402", name, err)
		}
	}

	// A billed dimension that is not a plan change mutates a plan already
	// chosen (ADR046): the dunning gate alone, never a payment-method check.
	paid := databaseForProtection("dpg-paid", "paid", false)
	paid.Labels[core.LabelTenant] = "tea-a"
	paid.Spec.Plan = "basic-256mb"
	sized, _ := newService(paid)
	sizedGate := &rejectingPaymentGate{}
	sized.Payment = sizedGate
	size := int32(20)
	if _, err := sized.UpdatePostgres(context.Background(), paid.Name, PostgresPatch{DiskSizeGB: &size}); errors.Is(err, core.ErrPaymentRequired) || len(sizedGate.calls) != 0 {
		t.Fatalf("a paid database's disk change = %v after %d payment checks, want no payment gate", err, len(sizedGate.calls))
	}
}

// TestPlanChangesRefuseADatabaseBeingDeleted: the real plan change and its
// dry-run answer a database being deleted alike, as services always have
// (w5/m116). Before, only the preview refused it.
func TestPlanChangesRefuseADatabaseBeingDeleted(t *testing.T) {
	db := databaseForProtection("dpg-going", "going", false)
	now := metav1.Now()
	db.DeletionTimestamp = &now
	db.Finalizers = []string{"app.bex.co/finalizer"}
	svc, _ := newService(db)
	if _, err := svc.SetPlanDryRun(context.Background(), db.Name, "free"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("SetPlanDryRun on a deleting database = %v, want not found", err)
	}
	if _, err := svc.SetPlan(context.Background(), db.Name, "free"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("SetPlan on a deleting database = %v, want not found", err)
	}
}
