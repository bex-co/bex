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
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w22/m402 (migration 0260, docs/ADR012-auth.md §9a): a workspace's "require
// two-factor authentication" setting defaults to off, is written by
// SetTenantRequireTwoFactor and comes back through every tenant read and the
// enforcement point's own single-column read; the audit row of the change
// carries the value it was set to; and an agent session names its workspace.
func TestPGTenantRequireTwoFactor(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	owner := "tf-owner-" + stamp
	ws, err := st.CreateTenantWithMember(ctx, owner, PlanPro)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	if ws.RequireTwoFactor {
		t.Fatalf("a new workspace requires 2FA: %+v — the migration default is off", ws)
	}
	if on, err := st.RequireTwoFactor(ctx, ws.ID); err != nil || on {
		t.Fatalf("RequireTwoFactor on a new workspace = (%v, %v), want (false, nil)", on, err)
	}

	set, changed, err := st.SetTenantRequireTwoFactor(ctx, ws.ID, true)
	if err != nil || !changed || !set.RequireTwoFactor || set.ID != ws.ID || set.Plan != PlanPro {
		t.Fatalf("enable = (%+v, changed=%v, %v), want the updated row", set, changed, err)
	}
	// The same value again changes nothing: this is what makes two admins
	// toggling at once one change, and the loser is told so.
	if again, changed, err := st.SetTenantRequireTwoFactor(ctx, ws.ID, true); err != nil || changed || !again.RequireTwoFactor {
		t.Fatalf("enable again = (%+v, changed=%v, %v), want the row and changed=false", again, changed, err)
	}
	if on, err := st.RequireTwoFactor(ctx, ws.ID); err != nil || !on {
		t.Fatalf("RequireTwoFactor after enabling = (%v, %v)", on, err)
	}
	if got, err := st.GetTenant(ctx, ws.ID); err != nil || !got.RequireTwoFactor {
		t.Fatalf("GetTenant = (%+v, %v)", got, err)
	}
	if got, err := st.TenantForIdentity(ctx, owner); err != nil || !got.RequireTwoFactor {
		t.Fatalf("TenantForIdentity = (%+v, %v)", got, err)
	}
	if list, err := st.ListTenantsForSubject(ctx, owner); err != nil || len(list) != 1 || !list[0].RequireTwoFactor {
		t.Fatalf("ListTenantsForSubject = (%+v, %v)", list, err)
	}
	// An unrelated write does not clear it.
	if renamed, err := st.SetTenantDisplayName(ctx, ws.ID, "Acme"); err != nil || !renamed.RequireTwoFactor {
		t.Fatalf("rename = (%+v, %v), want the setting kept", renamed, err)
	}

	off, changed, err := st.SetTenantRequireTwoFactor(ctx, ws.ID, false)
	if err != nil || !changed || off.RequireTwoFactor {
		t.Fatalf("disable = (%+v, changed=%v, %v)", off, changed, err)
	}

	// A workspace that does not exist requires nothing, and cannot be set.
	if on, err := st.RequireTwoFactor(ctx, "tea-missing-"+stamp); err != nil || on {
		t.Fatalf("RequireTwoFactor of a missing workspace = (%v, %v), want (false, nil)", on, err)
	}
	if _, _, err := st.SetTenantRequireTwoFactor(ctx, "tea-missing-"+stamp, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("enabling a missing workspace = %v, want ErrNotFound", err)
	}
}

func TestPGAuditRowCarriesTheTwoFactorValue(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateTenantWithMember(ctx, "tf-audit-"+stamp, PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })

	on, off := true, false
	at := time.Now().UTC()
	for i, enabled := range []*bool{&on, &off, nil} {
		verb := core.AuditVerbSetRequireTwoFactor
		if enabled == nil {
			verb = "workspaces.Rename" // a row that is not a 2FA change
		}
		if err := st.Record(ctx, core.AuditEvent{
			Caller: "admin-" + stamp, CallerMethod: "session", Verb: verb, Resource: core.WorkspaceObject(ws.ID),
			Outcome: core.AuditAllowed, At: at.Add(time.Duration(i) * time.Second), Relation: core.RelCanManage,
			TwoFactorRequiredTo: enabled,
		}); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	rows, err := st.ListAuditEvents(ctx, ws.ID, AuditFilter{OldestFirst: true})
	if err != nil || len(rows) != 3 {
		t.Fatalf("audit rows = (%d, %v), want 3", len(rows), err)
	}
	if rows[0].TwoFactorRequiredTo == nil || !*rows[0].TwoFactorRequiredTo ||
		rows[1].TwoFactorRequiredTo == nil || *rows[1].TwoFactorRequiredTo ||
		rows[2].TwoFactorRequiredTo != nil {
		t.Fatalf("two_factor_required_to = %v, %v, %v; want true, false, NULL", rows[0].TwoFactorRequiredTo, rows[1].TwoFactorRequiredTo, rows[2].TwoFactorRequiredTo)
	}
}

func TestPGAgentSessionWorkspace(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "tf-session")
	ctx := context.Background()
	row := creatorSession(t, s, tenant.ID, "alice", "")
	if got, err := s.AgentSessionWorkspace(ctx, row.ID); err != nil || got != tenant.ID {
		t.Fatalf("AgentSessionWorkspace = (%q, %v), want %q", got, err, tenant.ID)
	}
	if got, err := s.AgentSessionWorkspace(ctx, ids.New(ids.AgentSession)); err != nil || got != "" {
		t.Fatalf("unknown session = (%q, %v), want (\"\", nil)", got, err)
	}
}
