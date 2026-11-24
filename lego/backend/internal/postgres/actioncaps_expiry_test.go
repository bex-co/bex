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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestActionCapabilities_ResumeNamesTheExpiryRefusal pins the preview-vs-execute
// agreement for an expired free database (ADR009): the verb refuses the resume
// with 400 POSTGRES_FREE_EXPIRED, so the projection must name it — allowed
// with the expired precondition, never ready. Before the fix the projection
// read allowed with no precondition, offering a Resume the verb then refused.
func TestActionCapabilities_ResumeNamesTheExpiryRefusal(t *testing.T) {
	db := databaseForProtection("dpg-expired", "expired", false)
	db.Spec.Suspended = true
	// The sweeper's own footprint: the recorded stage for this clock, which is
	// what freeExpired judges (not wall-clock now).
	expiresAt := FreeExpiryRollout.Add(FreeLifetime)
	db.Annotations = map[string]string{
		annotationFreeExpiry: "expired@" + expiresAt.Format(time.RFC3339),
	}
	svc, _, _ := protectedPostgresService(db)
	ctx := ctxAsCaller()

	if _, err := svc.Resume(ctx, db.Name); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Resume of an expired database = %v, want a bad request", err)
	}

	acts, err := svc.ActionCapabilities(ctx, db.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	r := actionByID(t, acts, core.ActionResume)
	if r.Outcome != core.DecisionAllowed {
		t.Fatalf("expired resume = %+v, want allowed (the verb answers 400, not 403)", r)
	}
	if r.Precondition != core.PrecondExpired {
		t.Errorf("expired resume precondition = %q, want %q: the projection offers a resume the verb refuses",
			r.Precondition, core.PrecondExpired)
	}
}

// TestActionCapabilities_ResumeBillingBeatsExpiry pins the guard order the verb
// enforces (setSuspended checks dunning before expiry): under a billing lock an
// expired resume still reads billing_blocked, not expired.
func TestActionCapabilities_ResumeBillingBeatsExpiry(t *testing.T) {
	db := databaseForProtection("dpg-expired-bill", "expired-bill", false)
	db.Labels[core.LabelTenant] = "tea-a"
	db.Spec.Suspended = true
	expiresAt := FreeExpiryRollout.Add(FreeLifetime)
	db.Annotations = map[string]string{
		annotationFreeExpiry: "expired@" + expiresAt.Format(time.RFC3339),
	}
	svc, _, _ := protectedPostgresService(db)
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Billing = &refusingBillingGate{}

	acts, err := svc.ActionCapabilities(ctxAs("user-a"), db.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if r := actionByID(t, acts, core.ActionResume); r.Outcome != core.DecisionAllowed || r.Precondition != core.PrecondBillingBlocked {
		t.Errorf("billing-blocked expired resume = %+v, want allowed/%s", r, core.PrecondBillingBlocked)
	}
}
