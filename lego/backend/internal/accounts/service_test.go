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
