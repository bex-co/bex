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
	"time"
)

func TestPGClaimBlueprintResource(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "claim")
	a := lifecycleBlueprint(t, st, tenant, "claim-a")
	b := lifecycleBlueprint(t, st, tenant, "claim-b")
	ctx := context.Background()

	if err := st.ClaimBlueprintResource(ctx, tenant.ID, "service", "web", a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.ClaimBlueprintResource(ctx, tenant.ID, "service", "web", b.ID, ""); !errors.Is(err, ErrBlueprintResourceConflict) {
		t.Fatalf("second claim = %v, want ErrBlueprintResourceConflict", err)
	}
	if err := st.ClaimBlueprintResource(ctx, tenant.ID, "service", "web", b.ID, a.ID); err != nil {
		t.Fatalf("takeover: %v", err)
	}
	owner, err := st.GetBlueprintResourceOwner(ctx, tenant.ID, "service", "web")
	if err != nil || owner != b.ID {
		t.Fatalf("owner = %q (%v), want %q", owner, err, b.ID)
	}
	// Stale expected owner after intervening takeover.
	if err := st.ClaimBlueprintResource(ctx, tenant.ID, "service", "web", a.ID, a.ID); !errors.Is(err, ErrBlueprintResourceConflict) {
		t.Fatalf("stale takeover = %v, want conflict", err)
	}
	if err := st.ReleaseBlueprintResourceClaims(ctx, tenant.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	owner, err = st.GetBlueprintResourceOwner(ctx, tenant.ID, "service", "web")
	if err != nil || owner != "" {
		t.Fatalf("after release owner = %q (%v)", owner, err)
	}
	_ = time.Now()
}
