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
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// recordedTuples records the org tuples a verb writes and deletes.
type recordedTuples struct{ writes, deletes []core.Tuple }

func (r *recordedTuples) WriteOrganizationTuples(_ context.Context, writes, deletes []core.Tuple) error {
	r.writes = append(r.writes, writes...)
	r.deletes = append(r.deletes, deletes...)
	return nil
}

// TestOrganizationTransferPG is Render support's transfer (w7/m464 t006,
// ADR115 D3: "Existing workspaces can be transferred into an org by contacting
// Render support"): without --apply it changes nothing; applied, the workspace
// takes the org's plan, its people become Members with their tuples, and
// AddOrgMemberEvent lands in the org's log; a workspace already in an org is
// refused, and so is a primary.
func TestOrganizationTransferPG(t *testing.T) {
	pg := settlePG(t)
	ctx := context.Background()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, other := "move-owner-"+stamp, "move-other-"+stamp
	primary, err := pg.CreateWorkspace(ctx, "move-primary-"+stamp, store.PlanPro, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), primary.ID) })
	_, org, _, err := pg.UpgradeToOrganization(ctx, primary.ID, store.PlanScale, []string{owner})
	if err != nil {
		t.Fatal(err)
	}
	solo, err := pg.CreateWorkspace(ctx, "move-solo-"+stamp, store.PlanPro, other)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), solo.ID) })

	tuples := &recordedTuples{}
	sink := &orgAuditSink{}
	svc := &Service{Base: &core.Base{Audit: sink}, Store: pg, OrgTuples: tuples}
	move := OrganizationTransfer{Service: svc}

	plan, err := move.Move(ctx, solo.ID, org.ID, false)
	if err != nil || plan.Applied || plan.PlanTo != store.PlanScale || !slices.Equal(plan.NewMembers, []string{other}) {
		t.Fatalf("dry run = %+v %v", plan, err)
	}
	if got, _ := pg.GetTenant(ctx, solo.ID); got.OrganizationID != "" || got.Plan != store.PlanPro {
		t.Fatalf("a dry run changed the workspace: %+v", got)
	}
	plan, err = move.Move(ctx, solo.ID, org.ID, true)
	if err != nil || !plan.Applied || !slices.Equal(plan.NewMembers, []string{other}) {
		t.Fatalf("applied move = %+v %v", plan, err)
	}
	if got, _ := pg.GetTenant(ctx, solo.ID); got.OrganizationID != org.ID || got.Plan != store.PlanScale || got.Privacy != store.PrivacyInviteOnly {
		t.Fatalf("moved workspace = %+v", got)
	}
	if !slices.Contains(tuples.writes, core.OrgWorkspaceTuple(org.ID, solo.ID)) || !slices.Contains(tuples.writes, core.OrgRoleTuple(org.ID, other, store.OrgRoleMember)) {
		t.Fatalf("tuples written = %+v", tuples.writes)
	}
	if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbOrgMemberAdded || sink.events[0].Resource != core.OrganizationObject(org.ID) {
		t.Fatalf("audit = %+v, want one AddOrgMemberEvent in the org's log", sink.events)
	}
	for _, ws := range []string{solo.ID, primary.ID} {
		var coded *core.CodedError
		if _, err := move.Move(ctx, ws, org.ID, true); !errors.As(err, &coded) || coded.Code != "WORKSPACE_IN_ORGANIZATION" {
			t.Fatalf("moving %s, already in an org = %v", ws, err)
		}
	}
}

type orgAuditSink struct{ events []core.AuditEvent }

func (s *orgAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.events = append(s.events, ev)
	return nil
}
