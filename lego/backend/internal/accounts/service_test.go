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

package accounts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type workspaceResolver struct{}

func (workspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-own", true
}
func (workspaceResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}

func accountContext(method string, human bool) context.Context {
	id := core.Identity{Subject: "identity-a", Method: method, Human: human}
	if method == "oauth2" && human {
		id.CanonicalScopes = core.ScopeRead
	}
	return core.WithIdentity(context.Background(), id)
}

type fakeStore struct {
	preview    []store.AccountWorkspaceDisposition
	claimed    []store.AccountDeletion
	advances   []string
	failed     []string
	cleaned    bool
	beginCalls int
	beginErr   error
	cleanupErr error
}

func (f *fakeStore) PreviewAccountDeletion(context.Context, string, []string) ([]store.AccountWorkspaceDisposition, error) {
	return f.preview, nil
}
func (f *fakeStore) BeginAccountDeletion(_ context.Context, subject, email string, _ []string) (store.AccountDeletion, error) {
	f.beginCalls++
	if f.beginErr != nil {
		return store.AccountDeletion{}, f.beginErr
	}
	return store.AccountDeletion{Subject: subject, State: store.AccountDeletionPending, Workspaces: f.preview}, nil
}
func (f *fakeStore) ClaimAccountDeletions(context.Context, int) ([]store.AccountDeletion, error) {
	rows := f.claimed
	f.claimed = nil
	return rows, nil
}
func (f *fakeStore) AdvanceAccountDeletion(_ context.Context, _, from, to string) error {
	f.advances = append(f.advances, from+"->"+to)
	return nil
}
func (f *fakeStore) FailAccountDeletion(_ context.Context, _, message string) error {
	f.failed = append(f.failed, message)
	return nil
}
func (f *fakeStore) CleanupAccountSubject(context.Context, string, string) error {
	f.cleaned = true
	return f.cleanupErr
}

type fakeKeys struct {
	keys       []apikeys.APIKey
	deleted    []string
	unbound    []string
	cleanupErr error
}

func (f *fakeKeys) Create(context.Context, string, string) (apikeys.APIKey, error) {
	return apikeys.APIKey{}, nil
}
func (f *fakeKeys) List(context.Context) ([]apikeys.APIKey, error) { return f.keys, nil }
func (f *fakeKeys) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeKeys) CleanupSubject(_ context.Context, subject string) error {
	if f.cleanupErr != nil {
		return f.cleanupErr
	}
	for _, key := range f.keys {
		if key.CreatedBy == subject {
			f.unbound = append(f.unbound, key.ID)
			f.deleted = append(f.deleted, key.ID)
		}
	}
	return nil
}
func (*fakeKeys) Touch(context.Context, string, time.Time) error  { return nil }
func (*fakeKeys) KeyOwner(context.Context, string) (string, bool) { return "", false }

type recorder struct {
	events []string
	failAt string
}

func (r *recorder) record(event string) error {
	r.events = append(r.events, event)
	if r.failAt == event {
		return errors.New("injected " + event + " failure")
	}
	return nil
}
func (r *recorder) CleanupSubject(context.Context, string) error { return r.record("oauth") }
func (r *recorder) DeleteSessions(context.Context, string) error { return r.record("sessions") }
func (r *recorder) DeleteIdentity(context.Context, string) error { return r.record("identity") }
func (r *recorder) Delete(_ context.Context, id string) error    { return r.record("delete:" + id) }
func (r *recorder) Remove(_ context.Context, id, _ string) error { return r.record("leave:" + id) }

func newService(st *fakeStore, rec *recorder) *Service {
	keys := &fakeKeys{keys: []apikeys.APIKey{
		{ID: "key-owned", CreatedBy: "identity-a"},
		{ID: "key-other", CreatedBy: "identity-b"},
	}}
	return &Service{
		Base:  &core.Base{Authz: allowChecker{}, Workspace: workspaceResolver{}},
		Store: st, Workspaces: rec, Members: rec, APIKeys: keys,
		OAuth: rec, Kratos: rec,
	}
}

func TestPreviewRequiresDirectHumanSession(t *testing.T) {
	svc := newService(&fakeStore{}, &recorder{})
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"api key", accountContext("oauth2", false)},
		{"delegated human oauth", accountContext("oauth2", true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Preview(tc.ctx); err == nil {
				t.Fatal("preview allowed non-session caller")
			}
		})
	}
	if _, err := svc.Preview(accountContext("session", true)); err != nil {
		t.Fatalf("direct session preview: %v", err)
	}
}

func TestPreviewGroupsEveryWorkspaceDisposition(t *testing.T) {
	st := &fakeStore{preview: []store.AccountWorkspaceDisposition{
		{ID: "tea-delete", Action: store.AccountWorkspaceDelete},
		{ID: "tea-leave", Action: store.AccountWorkspaceLeave},
		{ID: "tea-blocked", Action: store.AccountWorkspaceBlocked},
	}}
	preview, err := newService(st, &recorder{}).Preview(accountContext("session", true))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Delete) != 1 || preview.Delete[0].ID != "tea-delete" ||
		len(preview.Leave) != 1 || preview.Leave[0].ID != "tea-leave" ||
		len(preview.Blocked) != 1 || preview.Blocked[0].ID != "tea-blocked" {
		t.Fatalf("grouped preview = %+v", preview)
	}
}

func TestRequestRequiresExactConfirmationAndDirectSession(t *testing.T) {
	for _, confirmation := range []string{"", "Delete my account", Confirmation + " "} {
		t.Run("confirmation="+confirmation, func(t *testing.T) {
			st := &fakeStore{}
			_, err := newService(st, &recorder{}).Request(accountContext("session", true), confirmation)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != "ACCOUNT_DELETION_CONFIRMATION" {
				t.Fatalf("request error = %v, want ACCOUNT_DELETION_CONFIRMATION", err)
			}
			if st.beginCalls != 0 {
				t.Fatalf("inexact confirmation wrote %d intent(s)", st.beginCalls)
			}
		})
	}

	for _, caller := range []context.Context{
		accountContext("oauth2", false),
		accountContext("oauth2", true),
	} {
		st := &fakeStore{}
		_, err := newService(st, &recorder{}).Request(caller, Confirmation)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "ACCOUNT_DELETION_SESSION_REQUIRED" {
			t.Fatalf("request error = %v, want ACCOUNT_DELETION_SESSION_REQUIRED", err)
		}
		if st.beginCalls != 0 {
			t.Fatalf("non-session caller wrote %d intent(s)", st.beginCalls)
		}
	}
}

func TestRequestReturnsTypedBlockersAndWritesNoIntent(t *testing.T) {
	st := &fakeStore{preview: []store.AccountWorkspaceDisposition{{ID: "tea-blocked", Name: "team", Action: "blocked"}}}
	svc := newService(st, &recorder{})
	_, err := svc.Request(accountContext("session", true), Confirmation)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeBlocked {
		t.Fatalf("blocked request = %v, want %s", err, CodeBlocked)
	}
}

type settlerFake struct {
	settled []string
	due     map[string]bool
}

func (f *settlerFake) SettleWorkspace(_ context.Context, workspaceID string) error {
	f.settled = append(f.settled, workspaceID)
	if f.due[workspaceID] {
		return core.NewBillingBalanceDueError(core.BalanceDue{WorkspaceID: workspaceID, HostedInvoiceURL: "https://invoice.stripe.test/in_1", AmountDueCents: 300, Currency: "usd"})
	}
	return nil
}

// ADR091 D10.4: every workspace the deletion would delete is settled before
// the intent commits; a balance due refuses the request with
// BILLING_BALANCE_DUE and writes no intent, so the account can still pay.
// Workspaces the account only leaves are not settled.
func TestRequestSettlesDeletedWorkspacesBeforeIntent(t *testing.T) {
	preview := []store.AccountWorkspaceDisposition{
		{ID: "tea-own", Name: "mine", Action: store.AccountWorkspaceDelete},
		{ID: "tea-shared", Name: "team", Action: store.AccountWorkspaceLeave},
	}
	st := &fakeStore{preview: preview}
	settler := &settlerFake{due: map[string]bool{"tea-own": true}}
	svc := newService(st, &recorder{})
	svc.Settler = settler
	_, err := svc.Request(accountContext("session", true), Confirmation)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeBillingBalanceDue || coded.Params["hostedInvoiceUrl"] != "https://invoice.stripe.test/in_1" {
		t.Fatalf("request with a balance due = %v, want BILLING_BALANCE_DUE with the invoice link", err)
	}
	if st.beginCalls != 0 {
		t.Fatalf("a refused request wrote %d intent(s)", st.beginCalls)
	}
	settler.due = nil
	if _, err := svc.Request(accountContext("session", true), Confirmation); err != nil {
		t.Fatalf("request after paying: %v", err)
	}
	if st.beginCalls != 1 || len(settler.settled) != 2 || settler.settled[0] != "tea-own" || settler.settled[1] != "tea-own" {
		t.Fatalf("begin=%d settled=%v, want one intent and only the deleted workspace settled", st.beginCalls, settler.settled)
	}
}

func TestWorkerDeletesIdentityOnlyAfterCredentialAndWorkspaceCleanup(t *testing.T) {
	plan := []store.AccountWorkspaceDisposition{
		{ID: "tea-personal", Action: "delete"},
		{ID: "tea-shared", Action: "leave"},
	}
	st := &fakeStore{claimed: []store.AccountDeletion{{Subject: "identity-a", DeletedMarker: "deleted:own-a", State: store.AccountDeletionPending, Workspaces: plan}}}
	rec := &recorder{}
	svc := newService(st, rec)
	keys := svc.APIKeys.(*fakeKeys)
	if err := svc.processPending(context.Background(), 10); err != nil {
		t.Fatalf("process: %v", err)
	}
	wantEvents := []string{"oauth", "delete:tea-personal", "leave:tea-shared", "sessions", "identity"}
	if len(rec.events) != len(wantEvents) {
		t.Fatalf("events=%v want=%v", rec.events, wantEvents)
	}
	for i := range wantEvents {
		if rec.events[i] != wantEvents[i] {
			t.Fatalf("events=%v want=%v", rec.events, wantEvents)
		}
	}
	if len(keys.deleted) != 1 || keys.deleted[0] != "key-owned" || len(keys.unbound) != 1 || keys.unbound[0] != "key-owned" {
		t.Fatalf("owned key cleanup deleted=%v unbound=%v", keys.deleted, keys.unbound)
	}
	if !st.cleaned {
		t.Fatal("subject data was not cleaned")
	}
	wantAdvances := []string{"pending->cleaning", "cleaning->identity", "identity->done"}
	for i := range wantAdvances {
		if st.advances[i] != wantAdvances[i] {
			t.Fatalf("advances=%v want=%v", st.advances, wantAdvances)
		}
	}
}

func TestWorkerFailureLeavesIdentityAndRecordsRetry(t *testing.T) {
	st := &fakeStore{claimed: []store.AccountDeletion{{
		Subject: "identity-a", DeletedMarker: "deleted:own-a", State: store.AccountDeletionCleaning,
		Workspaces: []store.AccountWorkspaceDisposition{{ID: "tea-personal", Action: "delete"}},
	}}}
	rec := &recorder{failAt: "delete:tea-personal"}
	svc := newService(st, rec)
	if err := svc.processPending(context.Background(), 10); err != nil {
		t.Fatalf("sweep should retain row instead of failing loop: %v", err)
	}
	if len(st.failed) != 1 {
		t.Fatalf("failed retries=%v", st.failed)
	}
	for _, event := range rec.events {
		if event == "sessions" || event == "identity" {
			t.Fatalf("identity cleanup ran after workspace failure: %v", rec.events)
		}
	}
}

func TestWorkerRetriesEveryExternalFailureWithoutSkippingAhead(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		plan       []store.AccountWorkspaceDisposition
		failAt     string
		wantEvents []string
	}{
		{name: "Hydra", state: store.AccountDeletionPending, failAt: "oauth", wantEvents: []string{"oauth"}},
		{name: "workspace purger", state: store.AccountDeletionCleaning, plan: []store.AccountWorkspaceDisposition{{ID: "tea-delete", Action: store.AccountWorkspaceDelete}}, failAt: "delete:tea-delete", wantEvents: []string{"delete:tea-delete"}},
		{name: "OpenFGA member revoke", state: store.AccountDeletionCleaning, plan: []store.AccountWorkspaceDisposition{{ID: "tea-leave", Action: store.AccountWorkspaceLeave}}, failAt: "leave:tea-leave", wantEvents: []string{"leave:tea-leave"}},
		{name: "Kratos sessions", state: store.AccountDeletionIdentity, failAt: "sessions", wantEvents: []string{"sessions"}},
		{name: "Kratos identity", state: store.AccountDeletionIdentity, failAt: "identity", wantEvents: []string{"sessions", "identity"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := &fakeStore{claimed: []store.AccountDeletion{{
				Subject: "identity-a", DeletedMarker: "deleted:own-a", State: tc.state, Workspaces: tc.plan,
			}}}
			rec := &recorder{failAt: tc.failAt}
			if err := newService(st, rec).processPending(context.Background(), 1); err != nil {
				t.Fatalf("sweep: %v", err)
			}
			if len(st.failed) != 1 {
				t.Fatalf("retry records = %v", st.failed)
			}
			if len(rec.events) != len(tc.wantEvents) {
				t.Fatalf("events = %v, want %v", rec.events, tc.wantEvents)
			}
			for i := range tc.wantEvents {
				if rec.events[i] != tc.wantEvents[i] {
					t.Fatalf("events = %v, want %v", rec.events, tc.wantEvents)
				}
			}
			if len(st.advances) != 0 {
				t.Fatalf("advanced past failed dependency: %v", st.advances)
			}
		})
	}
}

func TestWorkerRetriesAPIKeyAndSubjectStoreFailures(t *testing.T) {
	t.Run("API key cleanup", func(t *testing.T) {
		st := &fakeStore{claimed: []store.AccountDeletion{{Subject: "identity-a", State: store.AccountDeletionPending}}}
		svc := newService(st, &recorder{})
		svc.APIKeys.(*fakeKeys).cleanupErr = errors.New("injected API-key failure")
		if err := svc.processPending(context.Background(), 1); err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if len(st.failed) != 1 || len(st.advances) != 0 {
			t.Fatalf("failed=%v advances=%v", st.failed, st.advances)
		}
	})

	t.Run("Postgres subject cleanup", func(t *testing.T) {
		st := &fakeStore{
			claimed:    []store.AccountDeletion{{Subject: "identity-a", DeletedMarker: "deleted:own-a", State: store.AccountDeletionCleaning}},
			cleanupErr: errors.New("injected Postgres failure"),
		}
		svc := newService(st, &recorder{})
		if err := svc.processPending(context.Background(), 1); err != nil {
			t.Fatalf("sweep: %v", err)
		}
		if len(st.failed) != 1 || len(st.advances) != 0 {
			t.Fatalf("failed=%v advances=%v", st.failed, st.advances)
		}
	})
}

func TestCompletedDeletionIsIdempotent(t *testing.T) {
	st := &fakeStore{}
	rec := &recorder{}
	svc := newService(st, rec)
	done := store.AccountDeletion{Subject: "identity-a", State: store.AccountDeletionDone}
	if err := svc.process(context.Background(), done); err != nil {
		t.Fatalf("first completed pass: %v", err)
	}
	if err := svc.process(context.Background(), done); err != nil {
		t.Fatalf("second completed pass: %v", err)
	}
	if len(rec.events) != 0 || len(st.advances) != 0 || st.cleaned {
		t.Fatalf("completed retry performed work: events=%v advances=%v cleaned=%v", rec.events, st.advances, st.cleaned)
	}
}

type balanceDueTeardown struct {
	due     bool
	settled []bool
	waits   []time.Duration
}

func (b *balanceDueTeardown) Delete(ctx context.Context, id string) error {
	b.settled = append(b.settled, core.SettledBalance(ctx))
	wait, _ := core.SettleWait(ctx)
	b.waits = append(b.waits, wait)
	if b.due {
		return core.NewBillingBalanceDueError(core.BalanceDue{WorkspaceID: id, InvoiceID: "in_1",
			HostedInvoiceURL: "https://invoice.stripe.com/i/acct_1/in_1", AmountDueCents: 312, Currency: "usd"})
	}
	return nil
}

type mailFake struct{ to, text, html []string }

func (m *mailFake) Send(_ context.Context, to, _, text, html string) error {
	m.to, m.text, m.html = append(m.to, to), append(m.text, text), append(m.html, html)
	return nil
}

type emailsFake map[string]string

func (e emailsFake) LookupEmails(context.Context, []string) (map[string]string, error) { return e, nil }

// ADR091 D10.4: a worker teardown refused for an unpaid balance stores the
// error code only (never the payment link), and emails the hosted invoice
// link to the account once, since the account can no longer sign in.
func TestWorkerBalanceDueEmailsInvoiceLinkAndStoresCode(t *testing.T) {
	row := store.AccountDeletion{Subject: "identity-a", DeletedMarker: "deleted:own-a", State: store.AccountDeletionCleaning,
		CreatedAt: time.Now().Add(-time.Hour), Workspaces: []store.AccountWorkspaceDisposition{{ID: "tea-own", Action: store.AccountWorkspaceDelete}}}
	st := &fakeStore{claimed: []store.AccountDeletion{row}}
	svc := newService(st, &recorder{})
	teardown := &balanceDueTeardown{due: true}
	mail := &mailFake{}
	svc.Workspaces, svc.Mailer, svc.Emails = teardown, mail, emailsFake{"identity-a": "a@example.com"}
	if err := svc.processPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(st.failed) != 1 || st.failed[0] != core.CodeBillingBalanceDue {
		t.Fatalf("last_error = %v, want only the code", st.failed)
	}
	if len(mail.to) != 1 || mail.to[0] != "a@example.com" || !strings.Contains(mail.text[0], "https://invoice.stripe.com/i/acct_1/in_1") ||
		!strings.Contains(mail.text[0], "3.12 USD") || !strings.Contains(mail.html[0], `href="https://invoice.stripe.com/i/acct_1/in_1"`) {
		t.Fatalf("mail = %+v, want the invoice link sent to the account", mail)
	}
	if len(teardown.settled) != 1 || teardown.settled[0] {
		t.Fatalf("an hour-old intent skipped the teardown settle: %v", teardown.settled)
	}
	// The worker is bound by no request: its settle may outwait Stripe's
	// minute of meter aggregation instead of refusing BILLING_SETTLING.
	if teardown.waits[0] != workerSettleWait || workerSettleWait < 2*time.Minute {
		t.Fatalf("teardown settle wait = %v, want workerSettleWait (%v)", teardown.waits, workerSettleWait)
	}
	// The retry with the code already stored does not email again.
	row.LastError = core.CodeBillingBalanceDue
	st.claimed = []store.AccountDeletion{row}
	if err := svc.processPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(mail.to) != 1 || len(st.failed) != 2 {
		t.Fatalf("retry mailed %d times, failures %v", len(mail.to), st.failed)
	}
}

// The teardown right after a request does not settle a second time: Request
// settled every deleted workspace moments before the intent committed.
func TestWorkerSkipsSettleRightAfterRequest(t *testing.T) {
	row := store.AccountDeletion{Subject: "identity-a", DeletedMarker: "deleted:own-a", State: store.AccountDeletionCleaning,
		CreatedAt: time.Now(), Workspaces: []store.AccountWorkspaceDisposition{{ID: "tea-own", Action: store.AccountWorkspaceDelete}}}
	st := &fakeStore{claimed: []store.AccountDeletion{row}}
	svc := newService(st, &recorder{})
	teardown := &balanceDueTeardown{}
	svc.Workspaces = teardown
	if err := svc.processPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if len(teardown.settled) != 1 || !teardown.settled[0] {
		t.Fatalf("teardown settled flags = %v, want the request's settle reused", teardown.settled)
	}
}
