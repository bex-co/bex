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
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// login_requirement_reminders_pg_test.go runs w1/m647's reminders over real
// Postgres (the day record of migration 0347 and the workspace's members) and
// the real twofactor.Gate: a non-admin is refused, turning 2FA on sends the
// day's batch once, a second batch the same UTC day is refused and the next
// day's goes.

type reminderPGFixture struct {
	pg        *store.PGStore
	svc       *Service
	mailer    *fakeMailer
	audit     *auditCapture
	workspace string
	admin     string
	member    string
	developer string
	now       time.Time
}

// newReminderPGFixture: an enrolled admin, a developer with 2FA and a member
// without it, in one Pro workspace; 2FA is not yet required.
func newReminderPGFixture(t *testing.T) *reminderPGFixture {
	t.Helper()
	pg := settlePG(t)
	ctx := context.Background()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	f := &reminderPGFixture{
		pg: pg, mailer: &fakeMailer{}, audit: &auditCapture{},
		admin: "rm-admin-" + stamp, member: "rm-member-" + stamp, developer: "rm-dev-" + stamp,
		now: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC),
	}
	ws, err := pg.CreateWorkspace(ctx, "rm-"+stamp, store.PlanPro, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	f.workspace = ws.ID
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), ws.ID) })
	for subject, role := range map[string]string{f.member: "developer", f.developer: "developer"} {
		if err := pg.AddMember(ctx, subject, f.workspace, role); err != nil {
			t.Fatal(err)
		}
	}
	identities := fakeIdentities{
		f.admin:     {Email: "admin@acme.test", MFAEnabled: true},
		f.developer: {Email: "dev@acme.test", MFAEnabled: true},
		f.member:    {Email: "member@acme.test", Name: "Member"},
	}
	clock := func() time.Time { return f.now }
	gate := twofactor.New(twofactor.Config{Settings: pg, Enrollment: identities, Now: clock})
	f.svc = &Service{
		Base: &core.Base{
			Authz: roleChecker{f.admin: "admin", f.member: "developer", f.developer: "developer"},
			Audit: f.audit, Clock: clock,
		},
		Store:            pg,
		Identities:       identities,
		LoginAssessor:    gate,
		Mailer:           f.mailer,
		Reminders:        pg,
		DashboardURL:     "https://dashboard.bex.test",
		TwoFactorChanged: gate.Invalidate,
	}
	return f
}

func TestTwoFactorRemindersPG_ANonAdminIsRefusedAndNothingIsSent(t *testing.T) {
	f := newReminderPGFixture(t)
	if _, _, err := f.pg.SetTenantRequireTwoFactor(context.Background(), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.NonCompliantMembers(ctxAs(f.developer), f.workspace, LoginRequirementTwoFactor); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer's read = %v, want ErrForbidden", err)
	}
	if _, err := f.svc.SendTwoFactorReminders(ctxAs(f.developer), f.workspace); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer's send = %v, want ErrForbidden", err)
	}
	if got := f.mailer.recipients(); len(got) != 0 {
		t.Fatalf("a refused send mailed %v", got)
	}
	// The refusal claimed no day: the admin's batch still goes.
	if n, err := f.svc.SendTwoFactorReminders(ctxAs(f.admin), f.workspace); err != nil || n != 1 {
		t.Fatalf("the admin's send = (%d, %v), want 1", n, err)
	}
}

func TestTwoFactorRemindersPG_EnforcingSendsOnceAndTheDayHoldsUntilMidnightUTC(t *testing.T) {
	f := newReminderPGFixture(t)
	ctx := ctxAs(f.admin)

	if w, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil || !w.RequireTwoFactor {
		t.Fatalf("enable = (%+v, %v)", w, err)
	}
	if got := f.mailer.recipients(); !reflect.DeepEqual(got, []string{"member@acme.test"}) {
		t.Fatalf("recipients on enforcing = %v, want the member without 2FA alone", got)
	}
	listed, err := f.svc.NonCompliantMembers(ctx, f.workspace, LoginRequirementTwoFactor)
	if err != nil || len(listed) != 1 || listed[0].Email != "member@acme.test" || listed[0].Name != "Member" {
		t.Fatalf("list = %+v, %v; want the member", listed, err)
	}

	// The enforcing batch was today's: Send reminders is refused until 00:00Z.
	f.now = time.Date(2026, 11, 26, 23, 59, 0, 0, time.UTC)
	if _, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); !isCoded(err, core.ErrTooManyRequests, core.CodeWorkspaceTwoFactorReminderLimit) {
		t.Fatalf("send the same day = %v, want the coded 429", err)
	}
	if got := f.mailer.recipients(); len(got) != 1 {
		t.Fatalf("a refused batch mailed: %v", got)
	}

	f.now = time.Date(2026, 11, 27, 0, 1, 0, 0, time.UTC)
	if n, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil || n != 1 {
		t.Fatalf("send the next day = (%d, %v), want 1", n, err)
	}
	var day time.Time
	if err := f.pg.Pool.QueryRow(context.Background(),
		`SELECT sent_on FROM workspace_two_factor_reminders WHERE workspace_id = $1`, f.workspace).Scan(&day); err != nil ||
		day.Format(time.DateOnly) != "2026-11-27" {
		t.Fatalf("recorded day = %v, %v; want 2026-11-27", day, err)
	}
	// One audit row per reminder, each naming the member.
	rows := f.audit.reminderRows()
	if len(rows) != 2 || rows[0].Target != core.MemberTarget(f.member) || rows[1].TargetName != "member@acme.test" {
		t.Fatalf("audit rows = %+v, want two naming the member", rows)
	}
}

// An org that requires 2FA in all of its workspaces is a requirement to remind
// about, though the workspace's own setting is off; read from the rows, so it
// counts the moment the org turns it on.
func TestTwoFactorRemindersPG_AnOrgsRequirementCounts(t *testing.T) {
	f := newOrgLoginFixture(t)
	mailer := &fakeMailer{}
	identities := f.svc.Identities.(fakeIdentities)
	for subject, email := range map[string]string{f.wsAdmin: "wsadmin@acme.test", f.member: "member@acme.test", f.guest: "guest@acme.test"} {
		identities[subject] = IdentityAttrs{Email: email}
	}
	f.svc.LoginAssessor, f.svc.Mailer, f.svc.Reminders = f.gate, mailer, f.pg
	ctx := ctxAs(f.wsAdmin)

	if _, err := f.svc.SendTwoFactorReminders(ctx, f.secondary); !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorNotRequired) {
		t.Fatalf("send before the org requires it = %v, want the coded 409", err)
	}
	if _, err := f.pg.SetOrganizationRequireTwoFactor(context.Background(), f.org.ID, true); err != nil {
		t.Fatal(err)
	}
	n, err := f.svc.SendTwoFactorReminders(ctx, f.secondary)
	if err != nil || n != 3 {
		t.Fatalf("send under the org's requirement = (%d, %v), want the workspace's three people without 2FA", n, err)
	}
	if got := len(mailer.recipients()); got != 3 {
		t.Fatalf("mailed %d, want 3", got)
	}
}
