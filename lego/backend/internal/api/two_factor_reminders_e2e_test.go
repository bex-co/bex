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

package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// two_factor_reminders_e2e_test.go walks w1/m647 the way bex-api serves it:
// over GraphQL, on real Postgres and OpenFGA, through the composition root's
// wiring of the mailer and the day record. An admin turns 2FA on and the
// member without it is emailed; the dashboard's read names them; a developer
// may neither read nor send; and Send reminders the same day answers the
// daily limit.

// reminderMailbox is the SMTP relay: every email it was handed.
type reminderMailbox struct {
	mu   sync.Mutex
	sent []string // "to|subject"
}

func (m *reminderMailbox) Send(_ context.Context, to, subject, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, to+"|"+subject)
	return nil
}

func (m *reminderMailbox) all() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.sent...)
}

func TestTwoFactorRemindersE2E(t *testing.T) {
	identities := impactIdentities{unenrolled: map[string]bool{}}
	mailbox := &reminderMailbox{}
	e := newOrgE2E(t, func(base *core.Base, d *Deps) {
		d.Identities = identities
		d.Mailer = mailbox
		d.DashboardURL = "https://dashboard.bex.test"
		d.TwoFactorReminders = d.WorkspaceStore.(*store.PGStore)
		d.LoginAssessor = twofactor.New(twofactor.Config{Settings: d.WorkspaceStore.(*store.PGStore), Enrollment: identities, Google: identities})
	})
	ctx := context.Background()
	alice, bob := e.person("alice"), e.person("bob")
	identities.unenrolled[bob.Subject] = true

	acme, err := e.st.CreateWorkspace(ctx, "remind-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	if err := e.st.AddMember(ctx, bob.Subject, acme.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := e.fga.(members.RoleGranter).GrantWorkspaceRole(ctx, acme.ID, "user:"+bob.Subject, "developer"); err != nil {
		t.Fatal(err)
	}
	read := fmt.Sprintf(`{ workspaceNonCompliantMembers(workspaceId: %q, requirement: TWO_FACTOR) { userId email name } }`, acme.ID)
	send := fmt.Sprintf(`mutation { sendWorkspaceTwoFactorReminders(workspaceId: %q) }`, acme.ID)

	// Not required yet: there is nobody to remind.
	e.refused(alice, send, core.CodeWorkspaceTwoFactorNotRequired)

	e.ok(alice, fmt.Sprintf(`mutation { setWorkspaceRequireTwoFactor(workspaceId: %q, enabled: true) { id requireTwoFactor } }`, acme.ID))
	bobEmail := bob.Subject + "@example.com"
	if got := mailbox.all(); len(got) != 1 || !strings.HasPrefix(got[0], bobEmail+"|") || !strings.Contains(got[0], acme.Name) {
		t.Fatalf("mail on enforcing = %v, want one reminder to bob naming the workspace", got)
	}

	listed, _ := e.ok(alice, read)["workspaceNonCompliantMembers"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["email"] != bobEmail {
		t.Fatalf("non-compliant members = %v, want bob", listed)
	}
	e.refused(bob, read, "FORBIDDEN")
	e.refused(bob, send, "FORBIDDEN")

	// Turning it on was today's batch.
	e.refused(alice, send, core.CodeWorkspaceTwoFactorReminderLimit)
	if got := mailbox.all(); len(got) != 1 {
		t.Fatalf("mail after the refused sends = %v, want only the enforcing batch", got)
	}

	// The reminder is on the workspace's audit log, naming bob.
	logs, _ := e.ok(alice, fmt.Sprintf(`{ auditLogs(ownerId: %q) { action targetName } }`, acme.ID))["auditLogs"].([]any)
	var reminded int
	for _, l := range logs {
		row := l.(map[string]any)
		if row["action"] == core.AuditVerbSendTwoFactorReminders && row["targetName"] == bobEmail {
			reminded++
		}
	}
	if reminded != 1 {
		t.Fatalf("audit log = %v, want one reminder row naming bob", logs)
	}
}
