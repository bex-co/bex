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

package store

import (
	"context"
	"errors"
	"testing"
)

// w6/m627: a workspace's billing email reads back with its plan, a write to a
// workspace that is gone is ErrNotFound, and every move to Hobby clears it,
// the organization paths included, since all of them write the plan through
// updateTenantPlanTx. A move between higher plans keeps it.
func TestPGBillingEmailIsClearedByAMoveToHobby(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ws, err := st.CreateTenant(ctx, "ws-billing-email", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, ws.ID) })

	if plan, email, err := st.WorkspaceBillingEmail(ctx, ws.ID); err != nil || plan != PlanPro || email != "" {
		t.Fatalf("new workspace = %q, %q, %v; want pro with none stored", plan, email, err)
	}
	if err := st.SetWorkspaceBillingEmail(ctx, ws.ID, "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if plan, email, err := st.WorkspaceBillingEmail(ctx, ws.ID); err != nil || plan != PlanScale || email != "finance@example.com" {
		t.Fatalf("after pro->scale = %q, %q, %v; want the address kept", plan, email, err)
	}
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanHobby); err != nil {
		t.Fatal(err)
	}
	if plan, email, err := st.WorkspaceBillingEmail(ctx, ws.ID); err != nil || plan != PlanHobby || email != "" {
		t.Fatalf("after scale->hobby = %q, %q, %v; want hobby with the address cleared", plan, email, err)
	}

	if err := st.SetWorkspaceBillingEmail(ctx, "tea-00000000000000000000000000", "finance@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("write to a missing workspace = %v, want ErrNotFound", err)
	}
	if _, _, err := st.WorkspaceBillingEmail(ctx, "tea-00000000000000000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("read of a missing workspace = %v, want ErrNotFound", err)
	}
}
