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
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// login_requirement_reminders_test.go covers w1/m647 over the fake store and
// the real twofactor.Gate: who does not meet an enforced requirement, the 2FA
// reminders an admin sends at most once per workspace per UTC day, and the
// batch turning 2FA on sends. The real-Postgres day record is
// login_requirement_reminders_pg_test.go's.

// fakeReminderDays is the day record with the store's claim rule: a day
// claims only past the last one.
type fakeReminderDays struct {
	mu   sync.Mutex
	last map[string]string
}

func (f *fakeReminderDays) ClaimTwoFactorReminderDay(_ context.Context, workspaceID string, day time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := day.UTC().Format(time.DateOnly)
	if last, ok := f.last[workspaceID]; ok && last >= d {
		return false, nil
	}
	f.last[workspaceID] = d
	return true, nil
}

func (f *fakeReminderDays) ReleaseTwoFactorReminderDay(_ context.Context, workspaceID string, day time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last[workspaceID] == day.UTC().Format(time.DateOnly) {
		delete(f.last, workspaceID)
	}
	return nil
}

// sentMail is one email the fake mailer delivered.
type sentMail struct{ to, subject, text, html string }

// fakeMailer records each email, or refuses every one while failing.
type fakeMailer struct {
	mu      sync.Mutex
	sent    []sentMail
	failing bool
}

func (m *fakeMailer) Send(_ context.Context, to, subject, text, html string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing {
		return errors.New("smtp relay down")
	}
	m.sent = append(m.sent, sentMail{to, subject, text, html})
	return nil
}

func (m *fakeMailer) recipients() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, s := range m.sent {
		out = append(out, s.to)
	}
	return out
}

type reminderFixture struct {
	*impactFixture
	mailer *fakeMailer
	days   *fakeReminderDays
	now    time.Time
}

// newReminderFixture is newImpactFixture (admin-1 and dev-1 enrolled,
// admin-2 with neither 2FA nor the matching Google account) with a mailer, the
// day record and a clock at 2026-11-26T12:00Z; 2FA is not yet required.
func newReminderFixture(t *testing.T) *reminderFixture {
	t.Helper()
	f := &reminderFixture{
		impactFixture: newImpactFixture(t),
		mailer:        &fakeMailer{},
		days:          &fakeReminderDays{last: map[string]string{}},
		now:           time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC),
	}
	f.svc.Mailer, f.svc.Reminders = f.mailer, f.days
	f.svc.DashboardURL = "https://dashboard.bex.test/"
	clock := func() time.Time { return f.now }
	f.svc.Base.Clock = clock
	// The gate's caches read the same clock, so a test can step past them.
	f.svc.LoginAssessor = twofactor.New(twofactor.Config{
		Settings: storeSettings{f.store}, Enrollment: f.identities,
		Google: googleAccounts{"admin-1": true, "dev-1": true, "admin-2": false}, Now: clock,
	})
	return f
}

// require turns 2FA on in the store directly, so no batch goes with it.
func (f *reminderFixture) require(t *testing.T) {
	t.Helper()
	if _, _, err := f.store.SetTenantRequireTwoFactor(context.Background(), f.workspace, true); err != nil {
		t.Fatal(err)
	}
}

// reminderRows are the allowed audit rows of reminders sent.
func (a *auditCapture) reminderRows() []core.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []core.AuditEvent
	for _, e := range a.events {
		if e.Verb == core.AuditVerbSendTwoFactorReminders && e.Outcome == core.AuditAllowed {
			out = append(out, e)
		}
	}
	return out
}

func TestNonCompliantMembers_TwoFactorListsTheMemberWithoutASecondFactor(t *testing.T) {
	f := newReminderFixture(t)
	got, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil {
		t.Fatal(err)
	}
	userIDs, _ := f.store.OwnerIDsForSubjects(context.Background(), []string{"admin-2"})
	want := []NonCompliantMember{{UserID: userIDs["admin-2"], Email: "two@acme.test", Name: "Two"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %+v, want %+v", got, want)
	}
}

// Each requirement is judged alone, unlike the enable confirmation's read:
// with Google login also required, the 2FA list still names only who lacks
// 2FA, and the Google list only who lacks the matching Google account.
func TestNonCompliantMembers_JudgesEachRequirementAlone(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	if _, _, err := f.store.SetTenantRequireGoogleLogin(context.Background(), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	f.identities["admin-2"] = IdentityAttrs{Email: "two@acme.test", Name: "Two", MFAEnabled: true}
	twoFactor, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil || len(twoFactor) != 0 {
		t.Fatalf("2FA list = %+v, %v; want nobody: every member has 2FA, whatever Google login says", twoFactor, err)
	}
	google, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementGoogleLogin)
	if err != nil || len(google) != 1 || google[0].Email != "two@acme.test" {
		t.Fatalf("Google list = %+v, %v; want admin-2 alone", google, err)
	}
}

// An API key's binding is a member row, not a person: never listed.
func TestNonCompliantMembers_SkipsMachineMembers(t *testing.T) {
	f := newReminderFixture(t)
	f.store.members[f.workspace] = append(f.store.members[f.workspace],
		store.TenantMember{TenantID: f.workspace, Subject: "client-ci", Role: "developer", Kind: store.MemberKindMachine})
	got, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor)
	if err != nil || len(got) != 1 || got[0].Email != "two@acme.test" {
		t.Fatalf("members = %+v, %v; want admin-2 alone", got, err)
	}
}

func TestNonCompliantMembers_OnlyAnAdminMayAsk(t *testing.T) {
	f := newReminderFixture(t)
	for _, subject := range []string{"dev-1", "viewer-1", "stranger"} {
		if _, err := f.svc.NonCompliantMembers(ctxAs(subject), f.workspace, LoginRequirementTwoFactor); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s = %v, want ErrForbidden", subject, err)
		}
	}
}

// A lookup that cannot answer fails the read: never an empty list, which the
// card would show as "All members ... have enabled two-factor authentication!".
func TestNonCompliantMembers_AnOutageFailsTheRead(t *testing.T) {
	f := newReminderFixture(t)
	f.svc.LoginAssessor = twofactor.New(twofactor.Config{Settings: storeSettings{f.store}, Enrollment: failingEnrollment{}})
	if got, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("read = (%+v, %v), want ErrAuthzUnavailable", got, err)
	}
	f.svc.LoginAssessor = nil
	if _, err := f.svc.NonCompliantMembers(ctxAs("admin-1"), f.workspace, LoginRequirementTwoFactor); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("read with no gate = %v, want ErrAuthzUnavailable", err)
	}
}

func TestSendTwoFactorReminders_EmailsEachMemberWithout2FAOnceADay(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	ctx := ctxAs("admin-1")

	n, err := f.svc.SendTwoFactorReminders(ctx, f.workspace)
	if err != nil || n != 1 {
		t.Fatalf("send = (%d, %v), want 1 reminder", n, err)
	}
	if got := f.mailer.recipients(); !reflect.DeepEqual(got, []string{"two@acme.test"}) {
		t.Fatalf("recipients = %v, want admin-2 alone", got)
	}
	rows := f.audit.reminderRows()
	if len(rows) != 1 || rows[0].Target != core.MemberTarget("admin-2") || rows[0].TargetName != "two@acme.test" ||
		rows[0].Resource != core.WorkspaceObject(f.workspace) || rows[0].Caller != "admin-1" {
		t.Fatalf("audit rows = %+v, want one naming admin-2 by admin-1", rows)
	}

	// The same UTC day: Render's "Remind limit has been reached!", nothing sent.
	f.now = time.Date(2026, 11, 26, 23, 59, 0, 0, time.UTC)
	_, err = f.svc.SendTwoFactorReminders(ctx, f.workspace)
	var coded *core.CodedError
	if !isCoded(err, core.ErrTooManyRequests, core.CodeWorkspaceTwoFactorReminderLimit) || !errors.As(err, &coded) ||
		coded.Params["nextReminderAt"] != "2026-11-27T00:00:00Z" {
		t.Fatalf("second send = %v, want the coded 429 with nextReminderAt 2026-11-27T00:00:00Z", err)
	}
	if len(f.mailer.recipients()) != 1 || len(f.audit.reminderRows()) != 1 {
		t.Fatalf("a refused batch sent %v and recorded %d rows", f.mailer.recipients(), len(f.audit.reminderRows()))
	}

	// The next UTC day sends again.
	f.now = time.Date(2026, 11, 27, 0, 1, 0, 0, time.UTC)
	if n, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil || n != 1 {
		t.Fatalf("next day = (%d, %v), want 1 reminder", n, err)
	}
	if len(f.mailer.recipients()) != 2 {
		t.Fatalf("recipients = %v, want the second day's reminder", f.mailer.recipients())
	}
}

// The day is UTC's: a batch at 23:59Z and one two minutes later both go.
func TestSendTwoFactorReminders_TheDayTurnsAtMidnightUTC(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	ctx := ctxAs("admin-1")
	// 23:59 UTC is already the 27th in UTC+8, and still the 26th in UTC.
	f.now = time.Date(2026, 11, 27, 7, 59, 0, 0, time.FixedZone("UTC+8", 8*3600))
	if _, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil {
		t.Fatalf("23:59Z = %v", err)
	}
	f.now = time.Date(2026, 11, 27, 0, 1, 0, 0, time.UTC)
	if _, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil {
		t.Fatalf("00:01Z the next day = %v, want sent", err)
	}
	if got := f.days.last[f.workspace]; got != "2026-11-27" {
		t.Fatalf("recorded day = %q, want 2026-11-27", got)
	}
}

// A member who enables 2FA drops out of the list and of the next batch.
func TestSendTwoFactorReminders_AMemberWhoEnables2FAIsNotRemindedAgain(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	f.store.members[f.workspace] = append(f.store.members[f.workspace],
		store.TenantMember{TenantID: f.workspace, Subject: "dev-2", Role: "developer", Kind: store.MemberKindUser})
	f.identities["dev-2"] = IdentityAttrs{Email: "dev2@acme.test"}
	ctx := ctxAs("admin-1")
	if n, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil || n != 2 {
		t.Fatalf("first batch = (%d, %v), want 2", n, err)
	}

	f.identities["admin-2"] = IdentityAttrs{Email: "two@acme.test", Name: "Two", MFAEnabled: true}
	f.now = f.now.Add(twofactor.TTL + 24*time.Hour) // the next day, past the gate's cache
	listed, err := f.svc.NonCompliantMembers(ctx, f.workspace, LoginRequirementTwoFactor)
	if err != nil || len(listed) != 1 || listed[0].Email != "dev2@acme.test" {
		t.Fatalf("list = %+v, %v; want dev-2 alone", listed, err)
	}
	if n, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); err != nil || n != 1 {
		t.Fatalf("second batch = (%d, %v), want 1", n, err)
	}
	if got := f.mailer.recipients(); !reflect.DeepEqual(got, []string{"two@acme.test", "dev2@acme.test", "dev2@acme.test"}) {
		t.Fatalf("recipients = %v, want admin-2 only in the first batch", got)
	}
}

func TestSendTwoFactorReminders_OnlyAnAdminMaySendAndARefusalSendsNothing(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	for _, subject := range []string{"dev-1", "viewer-1", "stranger"} {
		if _, err := f.svc.SendTwoFactorReminders(ctxAs(subject), f.workspace); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s = %v, want ErrForbidden", subject, err)
		}
	}
	if len(f.mailer.recipients()) != 0 || len(f.days.last) != 0 {
		t.Fatalf("refused sends mailed %v and claimed %v", f.mailer.recipients(), f.days.last)
	}
	// The denial is on the record under the verb's own name.
	var denied int
	for _, e := range f.audit.events {
		if e.Verb == core.AuditVerbSendTwoFactorReminders && e.Outcome == core.AuditDenied {
			denied++
		}
	}
	if denied != 3 {
		t.Fatalf("denied rows = %d, want 3", denied)
	}
}

func TestSendTwoFactorReminders_RefusedWhere2FAIsNotRequired(t *testing.T) {
	f := newReminderFixture(t)
	if _, err := f.svc.SendTwoFactorReminders(ctxAs("admin-1"), f.workspace); !isCoded(err, core.ErrConflict, core.CodeWorkspaceTwoFactorNotRequired) {
		t.Fatalf("send = %v, want the coded 409", err)
	}
	if len(f.mailer.recipients()) != 0 || len(f.days.last) != 0 {
		t.Fatalf("mailed %v and claimed %v", f.mailer.recipients(), f.days.last)
	}
}

// With nobody to remind nothing is sent and the day stays free.
func TestSendTwoFactorReminders_NobodyToRemindLeavesTheDayFree(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	f.store.members[f.workspace] = f.store.members[f.workspace][:2] // admin-1 and dev-1, both enrolled
	if n, err := f.svc.SendTwoFactorReminders(ctxAs("admin-1"), f.workspace); err != nil || n != 0 {
		t.Fatalf("send = (%d, %v), want 0 and no error", n, err)
	}
	if len(f.days.last) != 0 {
		t.Fatalf("claimed %v with nobody reminded", f.days.last)
	}
}

// A batch the relay refuses entirely is an error the admin can retry the
// same day: the day is given back.
func TestSendTwoFactorReminders_ABatchThatSentNothingGivesTheDayBack(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	f.mailer.failing = true
	if _, err := f.svc.SendTwoFactorReminders(ctxAs("admin-1"), f.workspace); !isCoded(err, core.ErrUnavailable, core.CodeWorkspaceTwoFactorRemindersUnavailable) {
		t.Fatalf("send = %v, want the coded 503", err)
	}
	if len(f.days.last) != 0 || len(f.audit.reminderRows()) != 0 {
		t.Fatalf("a failed batch kept the day %v and recorded %d rows", f.days.last, len(f.audit.reminderRows()))
	}
	f.mailer.failing = false
	if n, err := f.svc.SendTwoFactorReminders(ctxAs("admin-1"), f.workspace); err != nil || n != 1 {
		t.Fatalf("retry = (%d, %v), want 1", n, err)
	}
}

func TestSendTwoFactorReminders_ADeploymentWithoutEmailRefuses(t *testing.T) {
	f := newReminderFixture(t)
	f.require(t)
	f.svc.Mailer = nil
	if _, err := f.svc.SendTwoFactorReminders(ctxAs("admin-1"), f.workspace); !isCoded(err, core.ErrUnavailable, core.CodeWorkspaceTwoFactorRemindersUnavailable) {
		t.Fatalf("send = %v, want the coded 503", err)
	}
}

// Turning 2FA on sends the day's batch, which then counts toward the limit;
// turning it on again, or Google login on, sends nothing.
func TestSetRequireTwoFactor_EnablingSendsTheDaysRemindersOnce(t *testing.T) {
	f := newReminderFixture(t)
	ctx := ctxAs("admin-1")
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if got := f.mailer.recipients(); !reflect.DeepEqual(got, []string{"two@acme.test"}) {
		t.Fatalf("recipients = %v, want admin-2's reminder", got)
	}
	if rows := f.audit.reminderRows(); len(rows) != 1 || rows[0].Caller != "admin-1" {
		t.Fatalf("audit rows = %+v, want one by the enabling admin", rows)
	}
	if _, err := f.svc.SetRequireTwoFactor(ctx, f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SendTwoFactorReminders(ctx, f.workspace); !isCoded(err, core.ErrTooManyRequests, core.CodeWorkspaceTwoFactorReminderLimit) {
		t.Fatalf("send after enabling = %v, want the day's limit", err)
	}
	f.svc.GoogleLogin = f.svc.LoginAssessor.(*twofactor.Gate)
	if _, err := f.svc.SetRequireGoogleLogin(asGoogle("admin-1"), f.workspace, true); err != nil {
		t.Fatal(err)
	}
	if got := f.mailer.recipients(); len(got) != 1 {
		t.Fatalf("recipients = %v, want only the 2FA batch", got)
	}
}

// Turning it on never fails on the mail: a relay that refuses every reminder,
// or a deployment that sends none, still leaves 2FA required.
func TestSetRequireTwoFactor_EnablingSucceedsWhateverTheMailDoes(t *testing.T) {
	for name, setup := range map[string]func(*reminderFixture){
		"relay down": func(f *reminderFixture) { f.mailer.failing = true },
		"no mailer":  func(f *reminderFixture) { f.svc.Mailer = nil },
	} {
		t.Run(name, func(t *testing.T) {
			f := newReminderFixture(t)
			setup(f)
			if w, err := f.svc.SetRequireTwoFactor(ctxAs("admin-1"), f.workspace, true); err != nil || !w.RequireTwoFactor {
				t.Fatalf("enable = (%+v, %v), want required", w, err)
			}
			if len(f.days.last) != 0 {
				t.Fatalf("claimed %v with nothing sent", f.days.last)
			}
		})
	}
}

func TestTwoFactorReminder_NamesTheWorkspaceAndLinksToAccountSecurity(t *testing.T) {
	subject, msg := twoFactorReminder(store.Tenant{ID: "tea-1", Name: "acme", DisplayName: "Acme Corp"}, "https://dashboard.bex.test/")
	if subject != `Enable two-factor authentication to access the "Acme Corp" workspace on bex` {
		t.Fatalf("subject = %q", subject)
	}
	text := msg.Text()
	for _, want := range []string{
		`The "Acme Corp" workspace on bex requires two-factor authentication (2FA).`,
		"You can't access the workspace or its resources until you enable it",
		"https://dashboard.bex.test/settings#security",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if html := msg.HTML(); !strings.Contains(html, `href="https://dashboard.bex.test/settings#security"`) {
		t.Errorf("html lacks the account security link:\n%s", html)
	}
	if _, bare := twoFactorReminder(store.Tenant{ID: "tea-1", Name: "acme"}, ""); bare.CTA != nil ||
		!strings.Contains(bare.Text(), "Enable it under Security in your bex account settings.") {
		t.Errorf("without a dashboard URL = %+v, want the way in words", bare)
	}
}
