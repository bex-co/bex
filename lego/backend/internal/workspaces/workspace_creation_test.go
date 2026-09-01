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

package workspaces

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type creationStoreFake struct {
	mu       sync.Mutex
	base     *fakeStore
	attempts map[string]store.WorkspaceCreationAttempt
}

func newCreationStoreFake(base *fakeStore) *creationStoreFake {
	return &creationStoreFake{base: base, attempts: map[string]store.WorkspaceCreationAttempt{}}
}

func (f *creationStoreFake) CreateWorkspaceCreationAttempt(_ context.Context, subject, name, plan, email string, required bool, expires time.Time) (store.WorkspaceCreationAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := store.WorkspaceCreationAttempt{ID: "wca-attempt", WorkspaceID: "tea-reserved", OwnerSubject: subject, Name: name, Plan: plan, BillingEmail: email, PaymentRequired: required, State: store.WorkspaceCreationPrepared, ExpiresAt: expires}
	f.attempts[a.ID] = a
	return a, nil
}

func (f *creationStoreFake) GetWorkspaceCreationAttempt(_ context.Context, id, subject string) (store.WorkspaceCreationAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.attempts[id]
	if !ok || a.OwnerSubject != subject {
		return store.WorkspaceCreationAttempt{}, store.ErrNotFound
	}
	return a, nil
}

func (f *creationStoreFake) SetWorkspaceCreationSetup(_ context.Context, id, subject, customerID, setupID string, livemode bool) (store.WorkspaceCreationAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.attempts[id]
	if a.OwnerSubject != subject {
		return store.WorkspaceCreationAttempt{}, store.ErrNotFound
	}
	a.ProviderCustomerID, a.ProviderSetupIntentID, a.State = customerID, setupID, store.WorkspaceCreationSetupPending
	a.ProviderLivemode = &livemode
	f.attempts[id] = a
	return a, nil
}

func (f *creationStoreFake) MarkWorkspaceCreationSetupSucceeded(_ context.Context, id, subject, methodID string) (store.WorkspaceCreationAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.attempts[id]
	if a.OwnerSubject != subject {
		return store.WorkspaceCreationAttempt{}, store.ErrNotFound
	}
	a.ProviderPaymentMethodID, a.State = methodID, store.WorkspaceCreationSetupSucceeded
	f.attempts[id] = a
	return a, nil
}

func (f *creationStoreFake) SetWorkspaceCreationSubscription(_ context.Context, id, subject, subscriptionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.attempts[id]
	if a.OwnerSubject != subject {
		return store.ErrNotFound
	}
	a.ProviderSubscriptionID = subscriptionID
	f.attempts[id] = a
	return nil
}

func (f *creationStoreFake) FinalizeWorkspaceCreation(_ context.Context, id, subject string, _ time.Time) (store.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.attempts[id]
	if a.OwnerSubject != subject {
		return store.Tenant{}, store.ErrNotFound
	}
	if a.State == store.WorkspaceCreationFinalized {
		return f.base.tenants[a.WorkspaceID], nil
	}
	if a.PaymentRequired && a.State != store.WorkspaceCreationSetupSucceeded {
		return store.Tenant{}, store.ErrConflict
	}
	tenant := store.Tenant{ID: a.WorkspaceID, Name: a.Name, Plan: a.Plan, CreatedAt: time.Now()}
	f.base.tenants[tenant.ID] = tenant
	f.base.members[tenant.ID] = []store.TenantMember{{TenantID: tenant.ID, Subject: subject, Role: "admin"}}
	a.State = store.WorkspaceCreationFinalized
	f.attempts[id] = a
	return tenant, nil
}

func (f *creationStoreFake) CancelWorkspaceCreationAttempt(_ context.Context, id, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.attempts[id]
	if !ok || a.OwnerSubject != subject {
		return store.ErrNotFound
	}
	a.State = store.WorkspaceCreationCleanupPending
	f.attempts[id] = a
	return nil
}

type creationBillingFake struct {
	preparedWorkspace string
	verified          int
	contracted        int
	verifyErr         error
}

func (f *creationBillingFake) PrepareWorkspaceSetup(_ context.Context, _, workspaceID, _, _, _ string) (billing.WorkspaceSetup, error) {
	f.preparedWorkspace = workspaceID
	return billing.WorkspaceSetup{CustomerID: "cus-new", SetupIntentID: "seti-new", ClientSecret: "seti_secret", PublishableKey: "pk_test", Livemode: false}, nil
}

func (f *creationBillingFake) VerifyWorkspaceSetup(context.Context, string, string, string, string) (billing.VerifiedWorkspaceSetup, error) {
	f.verified++
	if f.verifyErr != nil {
		return billing.VerifiedWorkspaceSetup{}, f.verifyErr
	}
	return billing.VerifiedWorkspaceSetup{PaymentMethodID: "pm-new"}, nil
}

func (f *creationBillingFake) PrepareWorkspaceContract(context.Context, string, string, string, string) (string, error) {
	f.contracted++
	return "sub-new", nil
}

func TestWorkspaceCreationAllModeOwnsBillingBeforeTenantExists(t *testing.T) {
	baseStore := newFakeStore()
	creationStore := newCreationStoreFake(baseStore)
	provider := &creationBillingFake{}
	svc := allowSvc(baseStore, &fakeGranter{}, &fakeRevoker{}, nil)
	svc.CreationStore = creationStore
	svc.CreationBilling = provider
	svc.Payment = rejectingPaymentGate{}
	svc.PaymentAllPlans = true

	attempt, err := svc.PrepareWorkspaceCreation(ctxAs("user-a"), "acme", "hobby", " BILLING@EXAMPLE.COM ", "", true)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !attempt.PaymentRequired || attempt.BillingEmail != "billing@example.com" || provider.preparedWorkspace != "tea-reserved" {
		t.Fatalf("attempt = %+v provider workspace=%q", attempt, provider.preparedWorkspace)
	}
	if len(baseStore.tenants) != 0 {
		t.Fatalf("prepare exposed tenant: %v", baseStore.tenants)
	}
	if _, err := svc.ResumeWorkspaceCreation(ctxAs("user-b"), attempt.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("foreign resume = %v, want not found", err)
	}
}

func TestWorkspaceCreationFinalizationIsIdempotentAndNeverUsesCurrentWorkspace(t *testing.T) {
	baseStore := newFakeStore()
	// An existing workspace has no bearing on the new attempt.
	_, _ = baseStore.CreateWorkspace(context.Background(), "old", "hobby", "user-a")
	creationStore := newCreationStoreFake(baseStore)
	provider := &creationBillingFake{}
	granter, revoker := &fakeGranter{}, &fakeRevoker{}
	svc := allowSvc(baseStore, granter, revoker, nil)
	svc.CreationStore = creationStore
	svc.CreationBilling = provider
	svc.Payment = rejectingPaymentGate{}

	attempt, err := svc.PrepareWorkspaceCreation(ctxAs("user-a"), "new-paid", "pro", "billing@example.com", "", true)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	first, err := svc.FinalizeWorkspaceCreation(ctxAs("user-a"), attempt.ID)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	second, err := svc.FinalizeWorkspaceCreation(ctxAs("user-a"), attempt.ID)
	if err != nil {
		t.Fatalf("finalize replay: %v", err)
	}
	if first.ID != "tea-reserved" || second.ID != first.ID || provider.verified != 1 || provider.contracted != 1 {
		t.Fatalf("first=%+v second=%+v verify=%d contract=%d", first, second, provider.verified, provider.contracted)
	}
	if len(granter.granted) != 1 || granter.granted[0] != "tea-reserved/user:user-a" || len(revoker.revoked) != 0 {
		t.Fatalf("grants=%v revokes=%v", granter.granted, revoker.revoked)
	}
}

func TestLegacyWorkspaceCreateNeverBorrowsAnotherWorkspacePayment(t *testing.T) {
	baseStore := newFakeStore()
	_, _ = baseStore.CreateWorkspace(context.Background(), "old", "hobby", "user-a")
	svc := allowSvc(baseStore, &fakeGranter{}, nil, nil)
	svc.Payment = rejectingPaymentGate{}
	if _, err := svc.Create(ctxAs("user-a"), "new-paid", "pro"); !errors.Is(err, core.ErrPaymentRequired) {
		t.Fatalf("legacy create = %v, want payment required", err)
	}
	if len(baseStore.tenants) != 1 {
		t.Fatalf("legacy create wrote a tenant: %v", baseStore.tenants)
	}
}

func TestWorkspaceCreationProviderFailureStaysResumable(t *testing.T) {
	baseStore := newFakeStore()
	creationStore := newCreationStoreFake(baseStore)
	provider := &creationBillingFake{verifyErr: errors.New("stripe unavailable")}
	svc := allowSvc(baseStore, &fakeGranter{}, &fakeRevoker{}, nil)
	svc.CreationStore = creationStore
	svc.CreationBilling = provider
	svc.Payment = rejectingPaymentGate{}

	attempt, err := svc.PrepareWorkspaceCreation(ctxAs("user-a"), "retry-paid", "pro", "billing@example.com", "", true)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := svc.FinalizeWorkspaceCreation(ctxAs("user-a"), attempt.ID); !errors.Is(err, core.ErrBillingUnavailable) {
		t.Fatalf("finalize provider failure = %v, want billing unavailable", err)
	}
	if len(baseStore.tenants) != 0 || creationStore.attempts[attempt.ID].State != store.WorkspaceCreationSetupPending {
		t.Fatalf("provider failure exposed tenant or lost retry state: tenants=%v attempt=%+v", baseStore.tenants, creationStore.attempts[attempt.ID])
	}
}
