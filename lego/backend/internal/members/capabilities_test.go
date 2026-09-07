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

package members

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestViewerCapabilitiesProjection is the w9/m84 gate: the projection reports
// exactly the relations the caller holds (each a Can-probe), and refuses a
// caller who cannot even view the workspace — the same 403 every other
// workspace-scoped verb returns. The dashboard reads this to disable controls,
// so a wrong boolean would either block an admin or leave a contributor a
// control that 403s on save.
func TestViewerCapabilitiesProjection(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("admin-1", "admin")
	st.seedMember("viewer-1", "viewer")

	t.Run("admin holds every capability", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "admin"})
		caps, err := s.Capabilities(ctxWith("admin-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		for name, got := range map[string]bool{
			"canView":          caps.CanView,
			"canViewLogs":      caps.CanViewLogs,
			"canOperate":       caps.CanOperate,
			"canCreate":        caps.CanCreate,
			"canViewSensitive": caps.CanViewSensitive,
			"canManageKeys":    caps.CanManageKeys,
			"canManage":        caps.CanManage,
			"canManageBilling": caps.CanManageBilling,
		} {
			if !got {
				t.Errorf("admin %s = false, want true", name)
			}
		}
	})

	t.Run("viewer holds only can_view", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
		caps, err := s.Capabilities(ctxWith("viewer-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if !caps.CanView {
			t.Errorf("viewer canView = false, want true")
		}
		// The write/reveal relations a contributor+ role would hold must all read
		// false — this is exactly what disables the create/reveal/manage controls.
		for name, got := range map[string]bool{
			"canOperate":       caps.CanOperate,
			"canCreate":        caps.CanCreate,
			"canViewSensitive": caps.CanViewSensitive,
			"canManageKeys":    caps.CanManageKeys,
			"canManage":        caps.CanManage,
			"canManageBilling": caps.CanManageBilling,
		} {
			if got {
				t.Errorf("viewer %s = true, want false", name)
			}
		}
	})

	t.Run("a non-member is refused, never served a projection", func(t *testing.T) {
		// A checker that grants nothing (not even can_view) models a caller with no
		// membership of the workspace — the cross-workspace case.
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "none"})
		_, err := s.Capabilities(ctxWith("stranger-1"), "tea-1", false)
		if !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Capabilities for a non-member = %v, want ErrForbidden", err)
		}
	})
}

// grantFor pulls one relation's tri-state grant out of the projection.
func grantFor(t *testing.T, caps Capabilities, action string) CapabilityGrant {
	t.Helper()
	for _, g := range caps.Grants {
		if g.Action == action {
			return g
		}
	}
	t.Fatalf("no grant for %s in %+v", action, caps.Grants)
	return CapabilityGrant{}
}

// membershipResolver is a map-backed core.WorkspaceResolver: each subject
// belongs to exactly the one workspace it maps to.
type membershipResolver map[string]string

func (r membershipResolver) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws, ok := r[id.Subject]
	return ws, ok
}

func (r membershipResolver) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return r[id.Subject] == tenantID, nil
}

// countingChecker allows everything and counts how often it was consulted.
type countingChecker struct{ calls int }

func (c *countingChecker) Check(context.Context, string, string, string) (bool, error) {
	c.calls++
	return true, nil
}

// TestCapabilitiesNeverFallBackToDefaultWorkspace (ADR087, w6/m136/t003):
// naming a workspace the caller is no member of is ErrForbidden — and the
// checker is never consulted, proving the refused request was not silently
// evaluated against the caller's default workspace instead (the
// confused-deputy shape: ask about B, get served an answer about A).
func TestCapabilitiesNeverFallBackToDefaultWorkspace(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("member-1", "admin")
	chk := &countingChecker{}
	s := svc(st, newFakeGranter(), nil, chk)
	s.Base.Workspace = membershipResolver{"member-1": "tea-1"}

	if _, err := s.Capabilities(ctxWith("member-1"), "tea-2", false); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("naming an unjoined workspace = %v, want ErrForbidden", err)
	}
	if chk.calls != 0 {
		t.Fatalf("checker consulted %d times for a refused workspace — the projection must not evaluate against the caller's default workspace", chk.calls)
	}

	// Control: the same caller naming their own workspace is served normally.
	caps, err := s.Capabilities(ctxWith("member-1"), "tea-1", false)
	if err != nil {
		t.Fatalf("own workspace: %v", err)
	}
	if !caps.CanView || chk.calls == 0 {
		t.Fatalf("own-workspace projection did not run (canView=%v, checks=%d)", caps.CanView, chk.calls)
	}
}

// staleCachedChecker models a replica whose positive cache is stale: the
// cached path still answers from old grants; the fresh path sees the
// revocation. Check answers cachedGrants, CheckFresh answers freshGrants.
type staleCachedChecker struct {
	cachedGrants map[string]bool
	freshGrants  map[string]bool
}

func (c staleCachedChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return c.cachedGrants[relation], nil
}

func (c staleCachedChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	return c.freshGrants[relation], nil
}

// TestCapabilitiesFreshBypassesStalePositives (ADR087, w6/m136/t005): the
// default projection may ride a ≤30s cached positive; fresh=true bypasses it
// — a mid-session downgrade is observed, including on can_view itself, and
// the response labels which evaluation the caller got.
func TestCapabilitiesFreshBypassesStalePositives(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("op-1", "contributor")

	t.Run("downgraded relation: cached still allows, fresh denies", func(t *testing.T) {
		chk := staleCachedChecker{
			cachedGrants: map[string]bool{core.RelCanView: true, core.RelCanOperate: true},
			freshGrants:  map[string]bool{core.RelCanView: true}, // can_operate just revoked
		}
		s := svc(st, newFakeGranter(), nil, chk)

		cached, err := s.Capabilities(ctxWith("op-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("cached Capabilities: %v", err)
		}
		if !cached.CanOperate || cached.Fresh {
			t.Fatalf("cached projection = {canOperate:%v fresh:%v}, want stale positive with fresh=false", cached.CanOperate, cached.Fresh)
		}

		fresh, err := s.Capabilities(ctxWith("op-1"), "tea-1", true)
		if err != nil {
			t.Fatalf("fresh Capabilities: %v", err)
		}
		if fresh.CanOperate || !fresh.Fresh {
			t.Fatalf("fresh projection = {canOperate:%v fresh:%v}, want the revocation observed and fresh=true", fresh.CanOperate, fresh.Fresh)
		}
		if g := grantFor(t, fresh, core.RelCanOperate); g.Outcome != core.DecisionDenied || g.Reason != core.ReasonInsufficientPermission {
			t.Fatalf("fresh can_operate grant = %+v, want denied/insufficient_permission", g)
		}
	})

	t.Run("membership itself revoked: fresh refuses the whole projection", func(t *testing.T) {
		chk := staleCachedChecker{
			cachedGrants: map[string]bool{core.RelCanView: true},
			freshGrants:  map[string]bool{}, // even can_view gone
		}
		s := svc(st, newFakeGranter(), nil, chk)

		if _, err := s.Capabilities(ctxWith("op-1"), "tea-1", false); err != nil {
			t.Fatalf("cached Capabilities should still ride the stale positive: %v", err)
		}
		if _, err := s.Capabilities(ctxWith("op-1"), "tea-1", true); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("fresh Capabilities after membership revocation = %v, want ErrForbidden", err)
		}
	})
}

// viewOnlyErroringChecker grants can_view and ERRORS on every other relation —
// the mid-projection checker-outage shape: the caller passes the gate, then
// the probes cannot be answered.
type viewOnlyErroringChecker struct{}

func (viewOnlyErroringChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return false, errors.New("fga unreachable")
}

// TestViewerCapabilitiesTriState is the ADR087 (w6/m136/t002) contract: each
// grant distinguishes an affirmative refusal from an unanswerable check, the
// reasons stay in the bounded vocabulary, and the legacy booleans equal the
// collapsed grants — restrictive either way, but only the grant says why.
func TestViewerCapabilitiesTriState(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("admin-1", "admin")
	st.seedMember("viewer-1", "viewer")

	assertBooleansMatchGrants := func(t *testing.T, caps Capabilities) {
		t.Helper()
		for action, b := range map[string]bool{
			core.RelCanView: caps.CanView, core.RelCanViewLogs: caps.CanViewLogs,
			core.RelCanOperate: caps.CanOperate, core.RelCanCreate: caps.CanCreate,
			core.RelCanViewSensitive: caps.CanViewSensitive, core.RelCanManageKeys: caps.CanManageKeys,
			core.RelCanManage: caps.CanManage, core.RelCanManageBilling: caps.CanManageBilling,
		} {
			if g := grantFor(t, caps, action); b != (g.Outcome == core.DecisionAllowed) {
				t.Errorf("%s: boolean %v disagrees with grant outcome %q", action, b, g.Outcome)
			}
		}
	}

	t.Run("admin: every grant allowed with no reason", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "admin"})
		caps, err := s.Capabilities(ctxWith("admin-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if len(caps.Grants) != 8 {
			t.Fatalf("want 8 grants (one per relation), got %d", len(caps.Grants))
		}
		for _, g := range caps.Grants {
			if g.Outcome != core.DecisionAllowed || g.Reason != "" {
				t.Errorf("admin grant %s = {%s %s}, want allowed with no reason", g.Action, g.Outcome, g.Reason)
			}
		}
		assertBooleansMatchGrants(t, caps)
	})

	t.Run("viewer: refusals are denied/insufficient_permission", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
		caps, err := s.Capabilities(ctxWith("viewer-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if g := grantFor(t, caps, core.RelCanOperate); g.Outcome != core.DecisionDenied || g.Reason != core.ReasonInsufficientPermission {
			t.Errorf("viewer can_operate grant = {%s %s}, want denied/insufficient_permission", g.Outcome, g.Reason)
		}
		if g := grantFor(t, caps, core.RelCanView); g.Outcome != core.DecisionAllowed {
			t.Errorf("viewer can_view grant = %q, want allowed (the verb's own gate proved it)", g.Outcome)
		}
		assertBooleansMatchGrants(t, caps)
	})

	t.Run("checker outage: unavailable, never a role verdict", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, viewOnlyErroringChecker{})
		caps, err := s.Capabilities(ctxWith("viewer-1"), "tea-1", false)
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		for _, action := range []string{core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate} {
			g := grantFor(t, caps, action)
			if g.Outcome != core.DecisionUnavailable || g.Reason != core.ReasonAuthzUnavailable {
				t.Errorf("outage grant %s = {%s %s}, want unavailable/authz_unavailable — "+
					"a failed checker must not read as \"your role forbids this\"", action, g.Outcome, g.Reason)
			}
		}
		// The legacy booleans stay restrictive (false) under the same outage.
		if caps.CanOperate || caps.CanViewLogs {
			t.Error("outage booleans must stay false (fail closed)")
		}
		assertBooleansMatchGrants(t, caps)
	})
}
