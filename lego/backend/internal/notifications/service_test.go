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

package notifications

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeWorkspace is a map-backed core.WorkspaceResolver — the same shape
// apps_test.go uses. Identities not in the map resolve ok=false.
type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

// IsMember: a map-backed caller belongs to exactly the one workspace it
// resolves to — the single-membership case, same as apps_test.go's fake.
func (f fakeWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	tid, ok := f[id.Subject]
	return ok && tid == tenantID, nil
}

// fakeStore is an in-memory NotificationsStore.
type fakeStore struct {
	rows       map[[2]string]store.NotificationSettings // (tenant, subject) -> row
	recipients map[string][]store.NotifyRecipient       // tenant -> recipients
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[[2]string]store.NotificationSettings{}, recipients: map[string][]store.NotifyRecipient{}}
}

func (f *fakeStore) GetNotificationSettings(_ context.Context, tenantID, subject string) (store.NotificationSettings, error) {
	row, ok := f.rows[[2]string{tenantID, subject}]
	if !ok {
		return store.NotificationSettings{}, store.ErrNotFound
	}
	return row, nil
}

func (f *fakeStore) UpsertNotificationSettings(_ context.Context, tenantID, subject string, deploySucceeded, deployFailed bool) (store.NotificationSettings, error) {
	row := store.NotificationSettings{ID: "ntf-fake", TenantID: tenantID, Subject: subject, DeploySucceeded: deploySucceeded, DeployFailed: deployFailed}
	f.rows[[2]string{tenantID, subject}] = row
	return row, nil
}

func (f *fakeStore) ListNotifyRecipients(_ context.Context, tenantID string) ([]store.NotifyRecipient, error) {
	return f.recipients[tenantID], nil
}

// fakeMailer records every send.
type fakeMailer struct {
	sent []struct{ to, subject, body string }
	err  error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, struct{ to, subject, body string }{to, subject, body})
	return nil
}

// fakeIdentities is a map-backed EmailLookup.
type fakeIdentities map[string]string

func (f fakeIdentities) LookupEmail(_ context.Context, subject string) (string, bool) {
	email, ok := f[subject]
	return email, ok
}

func newTestService(st NotificationsStore, ws core.WorkspaceResolver, mailer Mailer, identities EmailLookup) *Service {
	return &Service{
		Base:       &core.Base{Workspace: ws},
		Store:      st,
		Mailer:     mailer,
		Identities: identities,
	}
}

func TestGetSettingsDefaultsWhenNoRow(t *testing.T) {
	st := newFakeStore()
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a"}, nil, nil)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})

	got, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got != defaultSettings {
		t.Errorf("GetSettings with no row = %+v, want the default %+v", got, defaultSettings)
	}
}

func TestUpdateSettingsThenGetReflectsIt(t *testing.T) {
	st := newFakeStore()
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a"}, nil, nil)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})

	updated, err := svc.UpdateSettings(ctx, false, true)
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	want := SettingsView{DeploySucceeded: false, DeployFailed: true}
	if updated != want {
		t.Fatalf("UpdateSettings returned %+v, want %+v", updated, want)
	}
	got, err := svc.GetSettings(ctx)
	if err != nil || got != want {
		t.Errorf("GetSettings after update = %+v (%v), want %+v", got, err, want)
	}

	// A second caller in the same workspace is untouched.
	other := core.WithIdentity(context.Background(), core.Identity{Subject: "bob"})
	if got, err := svc.GetSettings(other); err != nil || got != defaultSettings {
		t.Errorf("bob's settings = %+v (%v), want unaffected default %+v", got, err, defaultSettings)
	}
}

func TestSettingsUnavailableWhenStoreNil(t *testing.T) {
	svc := newTestService(nil, fakeWorkspace{"alice": "tea-a"}, nil, nil)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})

	if _, err := svc.GetSettings(ctx); !errors.Is(err, core.ErrNotificationsUnavailable) {
		t.Errorf("GetSettings with nil store: want ErrNotificationsUnavailable, got %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, true, true); !errors.Is(err, core.ErrNotificationsUnavailable) {
		t.Errorf("UpdateSettings with nil store: want ErrNotificationsUnavailable, got %v", err)
	}
}

func TestNotifyDeploySendsOnlyToRecipientsWhoWantIt(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{
		{Subject: "alice", DeploySucceeded: true, DeployFailed: true},
		{Subject: "bob", DeploySucceeded: false, DeployFailed: true}, // opted out of success mail
		{Subject: "carol", DeploySucceeded: true, DeployFailed: true},
	}
	mailer := &fakeMailer{}
	identities := fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"} // carol has no known email
	svc := newTestService(st, nil, mailer, identities)

	svc.NotifyDeploy(context.Background(), "tea-a", "web", store.DeployLive)

	if len(mailer.sent) != 1 || mailer.sent[0].to != "alice@example.com" {
		t.Fatalf("sent = %+v, want exactly one mail to alice (bob opted out, carol has no email)", mailer.sent)
	}
}

func TestNotifyDeployFailedRespectsDeployFailedPreference(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{
		{Subject: "alice", DeploySucceeded: true, DeployFailed: false}, // opted out of failure mail
		{Subject: "bob", DeploySucceeded: true, DeployFailed: true},
	}
	mailer := &fakeMailer{}
	identities := fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"}
	svc := newTestService(st, nil, mailer, identities)

	svc.NotifyDeploy(context.Background(), "tea-a", "web", store.DeployUpdateFailed)

	if len(mailer.sent) != 1 || mailer.sent[0].to != "bob@example.com" {
		t.Fatalf("sent = %+v, want exactly one mail to bob (alice opted out of failure mail)", mailer.sent)
	}
}

func TestNotifyDeployIgnoresNonTerminalStatus(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeploySucceeded: true, DeployFailed: true}}
	mailer := &fakeMailer{}
	svc := newTestService(st, nil, mailer, fakeIdentities{"alice": "alice@example.com"})

	svc.NotifyDeploy(context.Background(), "tea-a", "web", store.DeployCanceled)

	if len(mailer.sent) != 0 {
		t.Errorf("sent = %+v, want none (canceled is not a success/failure this feature notifies on)", mailer.sent)
	}
}

func TestNotifyDeployNoopWithoutMailerOrStore(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeploySucceeded: true, DeployFailed: true}}

	// No mailer: nothing to send to, must not panic.
	svc := newTestService(st, nil, nil, fakeIdentities{"alice": "alice@example.com"})
	svc.NotifyDeploy(context.Background(), "tea-a", "web", store.DeployLive)

	// No store: nothing to look recipients up from, must not panic.
	svc2 := newTestService(nil, nil, &fakeMailer{}, fakeIdentities{"alice": "alice@example.com"})
	svc2.NotifyDeploy(context.Background(), "tea-a", "web", store.DeployLive)
}
