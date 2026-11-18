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

package tenancy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// tenancy_fault_test.go pins how onboarding answers a transient store or
// OpenFGA fault (w32/m446): which faults a returning person is served through,
// which still fail closed, and what a failure names.

var (
	errStoreReset = errors.New("read tcp 10.244.0.9:41552->10.96.0.20:5432: read: connection reset by peer")
	errFGAReset   = errors.New(`Post "http://openfga.auth.svc:8080/stores/s/check": read: connection reset by peer`)
)

const faultPerson = "identity-a"

// onboardThenExpire resolves faultPerson's personal workspace on ts and drops
// the 30 s cache entry, as its expiry would, so the next call takes the
// cache-miss path with the workspace still recently known.
func onboardThenExpire(t *testing.T, ts *Service) string {
	t.Helper()
	tid, err := ts.EnsureTenant(context.Background(), faultPerson, "", false)
	if err != nil || tid == "" {
		t.Fatalf("first onboarding: %q %v", tid, err)
	}
	ts.cache.Delete(cacheKey(methodSession, faultPerson))
	return tid
}

// failedStep is the step an onboarding error names, or "" when it names none.
func failedStep(err error) string {
	var step *onboardingStepError
	if errors.As(err, &step) {
		return step.step
	}
	return ""
}

// t002: an existing admin whose admin Check errors is still served. The grant
// that follows is a write of a tuple that is already there, which OpenFGA
// accepts as a no-op since w21/099, so nothing is revoked and no 503 comes back.
// A new service (no recent workspace) proves the grant path serves it, not the
// t003 fallback.
func TestEnsureTenantServesAnAdminWhoseCheckErrors(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	tid, err := NewTenantService(st, granter).EnsureTenant(context.Background(), faultPerson, "", false)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	granter.checkErr = errFGAReset

	got, err := NewTenantService(st, granter).EnsureTenant(context.Background(), faultPerson, "", false)
	if err != nil || got != tid {
		t.Fatalf("returning admin with an erroring Check = %q %v, want %q served", got, err, tid)
	}
	if len(granter.revoked) != 0 {
		t.Errorf("revoked %v; an existing admin's grant must not be taken back", granter.revoked)
	}
	if !granter.tuples["admin:"+tid+":user:"+faultPerson] {
		t.Error("the admin tuple is gone")
	}
}

// t002: when OpenFGA can answer neither the Check nor the grant, a caller with
// no recently known workspace still fails closed, naming the step.
func TestEnsureTenantCheckAndGrantBothFailingFailsClosed(t *testing.T) {
	st := newFakeTenantStore()
	granter := fgWithFail(1)
	granter.checkErr = errFGAReset

	_, err := NewTenantService(st, granter).EnsureTenant(context.Background(), faultPerson, "", false)
	if err == nil {
		t.Fatal("a Check and a grant that both fail served the request; want an error, never a silent skip")
	}
	if failedStep(err) != StepGrant {
		t.Errorf("error %v names step %q, want %q", err, failedStep(err), StepGrant)
	}
	if got, want := UnavailableMessage(err), "tenant onboarding unavailable: workspace admin grant failed (authorization service)"; got != want {
		t.Errorf("503 message = %q, want %q", got, want)
	}
	if len(granter.revoked) != 0 {
		t.Errorf("revoked %v after a failed grant", granter.revoked)
	}
}

// t003: a returning person whose personal workspace was resolved within the
// recent window is served through one failed owner lookup, and the next call
// within the cache window is a plain hit; a first-ever caller is not.
func TestEnsureTenantServesARecentWorkspaceThroughAStoreFault(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, newFakeGranter())
	tid := onboardThenExpire(t, ts)

	reads := 0
	st.ownerFail = func() error {
		reads++
		if reads == 1 {
			return errStoreReset
		}
		return nil
	}
	got, err := ts.EnsureTenant(context.Background(), faultPerson, "", false)
	if err != nil || got != tid {
		t.Fatalf("returning person through a store fault = %q %v, want %q", got, err, tid)
	}
	if got, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); err != nil || got != tid || reads != 1 {
		t.Fatalf("next call = %q %v after %d owner reads, want %q from the cache with no new read", got, err, reads, tid)
	}

	st.ownerFail = func() error { return errStoreReset }
	_, err = NewTenantService(st, newFakeGranter()).EnsureTenant(context.Background(), faultPerson, "", false)
	if failedStep(err) != StepOwnerRead {
		t.Fatalf("caller with no recent workspace = %v, want a %q failure", err, StepOwnerRead)
	}
	if got, want := UnavailableMessage(err), "tenant onboarding unavailable: personal workspace lookup failed (workspace store)"; got != want {
		t.Errorf("503 message = %q, want %q", got, want)
	}
	if strings.Contains(UnavailableMessage(err), "10.96.0.20") {
		t.Error("the 503 message carries the store error's own text")
	}
}

// t003: OpenFGA down for both the Check and the grant on the workspace the
// person was recently resolved to: served, as the store fault is.
func TestEnsureTenantServesARecentWorkspaceThroughAnOpenFGAFault(t *testing.T) {
	st := newFakeTenantStore()
	granter := newFakeGranter()
	ts := NewTenantService(st, granter)
	tid := onboardThenExpire(t, ts)

	granter.checkErr = errFGAReset
	granter.failNext = 1
	if got, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); err != nil || got != tid {
		t.Fatalf("returning person through an OpenFGA fault = %q %v, want %q", got, err, tid)
	}
}

// t003: the faults that say nothing about whether the workspace is still the
// person's own are served; the ones after a grant this request wrote, or a
// tombstone read, are not.
func TestEnsureTenantDoesNotServeARecentWorkspaceThroughEveryFault(t *testing.T) {
	t.Run("re-read after a grant fails", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter)
		tid := onboardThenExpire(t, ts)
		// The admin tuple is gone (a demotion's revoke), so this request writes
		// a grant and must re-read the binding before it may keep it.
		delete(granter.tuples, "admin:"+tid+":user:"+faultPerson)
		reads := 0
		st.ownerFail = func() error {
			reads++
			if reads == 2 {
				return errStoreReset
			}
			return nil
		}
		_, err := ts.EnsureTenant(context.Background(), faultPerson, "", false)
		if failedStep(err) != StepReread {
			t.Fatalf("a failed re-read after a written grant = %v, want a %q failure", err, StepReread)
		}
	})

	t.Run("tombstone read fails, even with the workspace cached", func(t *testing.T) {
		st := newFakeTenantStore()
		ts := NewTenantService(st, newFakeGranter())
		if _, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); err != nil {
			t.Fatalf("first onboarding: %v", err)
		}
		st.tombstoneErr = errStoreReset
		_, err := ts.EnsureTenant(context.Background(), faultPerson, "", false)
		if failedStep(err) != StepTombstone {
			t.Fatalf("unreadable tombstone = %v, want a %q failure (ADR086: fail closed)", err, StepTombstone)
		}
		if got, want := UnavailableMessage(err), "tenant onboarding unavailable: account deletion check failed (workspace store)"; got != want {
			t.Errorf("503 message = %q, want %q", got, want)
		}
	})

	t.Run("membership changed on this replica", func(t *testing.T) {
		st := newFakeTenantStore()
		ts := NewTenantService(st, newFakeGranter())
		onboardThenExpire(t, ts)
		ts.InvalidateTenant(faultPerson) // a removal or workspace delete here
		st.ownerFail = func() error { return errStoreReset }
		if _, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); failedStep(err) != StepOwnerRead {
			t.Fatalf("after InvalidateTenant = %v, want the store fault, not the forgotten workspace", err)
		}
	})

	t.Run("grant fails on a workspace that is not the recent one", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter)
		recent := onboardThenExpire(t, ts)
		// The binding moved after the recent workspace resolved (the old one
		// deleted, a new personal workspace minted): the owner read answers a
		// DIFFERENT workspace than the one remembered, and its grant fails.
		st.mu.Lock()
		delete(st.ownerOf, faultPerson)
		delete(st.members, faultPerson)
		st.mu.Unlock()
		minted, err := st.CreateTenantWithMember(context.Background(), faultPerson, "hobby")
		if err != nil {
			t.Fatalf("rebind: %v", err)
		}
		if minted.ID == recent {
			t.Fatalf("rebind minted %q, want a workspace other than the recent one", recent)
		}
		granter.failNext = 1
		if _, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); failedStep(err) != StepGrant {
			t.Fatalf("grant on %q with recent %q = %v, want a %q failure, never the old workspace", minted.ID, recent, err, StepGrant)
		}
	})

	t.Run("creation fails after the binding ended", func(t *testing.T) {
		st := newFakeTenantStore()
		ts := NewTenantService(st, newFakeGranter())
		onboardThenExpire(t, ts)
		// The binding ended (a removal) and the replacement mint cannot be
		// written: the fault is on the creation, not on a workspace that is
		// still the person's own.
		st.mu.Lock()
		delete(st.ownerOf, faultPerson)
		delete(st.members, faultPerson)
		st.mu.Unlock()
		st.createFail = func() error { return errStoreReset }
		if _, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); failedStep(err) != StepCreate {
			t.Fatalf("failed creation = %v, want a %q failure", err, StepCreate)
		}
	})

	t.Run("revoke fails after the binding ended", func(t *testing.T) {
		st := newFakeTenantStore()
		granter := newFakeGranter()
		ts := NewTenantService(st, granter)
		tid := onboardThenExpire(t, ts)
		// The admin tuple is gone (a demotion's revoke), so this request writes
		// a grant; the binding ends before the re-read, and taking the grant
		// back fails too. Nothing may be served: the workspace just stopped
		// being the person's own.
		delete(granter.tuples, "admin:"+tid+":user:"+faultPerson)
		granter.failRevoke = true
		reads := 0
		st.ownerFail = func() error {
			reads++
			if reads == 2 {
				return store.ErrNotFound
			}
			return nil
		}
		if _, err := ts.EnsureTenant(context.Background(), faultPerson, "", false); failedStep(err) != StepRevoke {
			t.Fatalf("a failed revoke after a written grant = %v, want a %q failure", err, StepRevoke)
		}
	})
}

// ADR086: a tombstoned account is refused while a recently resolved workspace
// is held, and the held workspace is forgotten.
func TestEnsureTenantTombstoneWinsOverARecentWorkspace(t *testing.T) {
	st := newFakeTenantStore()
	ts := NewTenantService(st, newFakeGranter())
	onboardThenExpire(t, ts)
	st.deleting = true
	st.ownerFail = func() error { return errStoreReset }

	_, err := ts.EnsureTenant(context.Background(), faultPerson, "", false)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ACCOUNT_DELETION_PENDING" {
		t.Fatalf("tombstoned account with a recent workspace = %v, want ACCOUNT_DELETION_PENDING", err)
	}
	if _, ok := ts.recent.Get(cacheKey(methodSession, faultPerson)); ok {
		t.Error("the tombstone left the recent workspace in place")
	}
}

// A coded refusal is a public answer of its own: StepFailed leaves it as is, so
// the gate writes its code, not a 503.
func TestStepFailedKeepsCodedRefusals(t *testing.T) {
	pending := core.NewAccountDeletionPendingError()
	if got := StepFailed(StepCreate, DepStore, pending); got != pending {
		t.Fatalf("StepFailed wrapped a coded refusal: %v", got)
	}
	if StepFailed(StepCreate, DepStore, nil) != nil {
		t.Fatal("StepFailed turned nil into an error")
	}
	if got := UnavailableMessage(errors.New("bare")); got != "tenant onboarding unavailable" {
		t.Fatalf("message for an untagged error = %q", got)
	}
}
