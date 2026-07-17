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
	"strings"
	"sync"
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

func (f *fakeStore) UpsertNotificationSettings(_ context.Context, tenantID, subject string, deployStarted, deploySucceeded, deployFailed bool) (store.NotificationSettings, error) {
	row := store.NotificationSettings{ID: "ntf-fake", TenantID: tenantID, Subject: subject, DeployStarted: deployStarted, DeploySucceeded: deploySucceeded, DeployFailed: deployFailed}
	f.rows[[2]string{tenantID, subject}] = row
	return row, nil
}

func (f *fakeStore) ListNotifyRecipients(_ context.Context, tenantID string) ([]store.NotifyRecipient, error) {
	return f.recipients[tenantID], nil
}

// fakeMailer records every send.
type fakeMailer struct {
	mu   sync.Mutex
	sent []struct{ to, subject, body string }
	err  error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
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

	updated, err := svc.UpdateSettings(ctx, false, false, true)
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	want := SettingsView{DeployStarted: false, DeploySucceeded: false, DeployFailed: true}
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
	if _, err := svc.UpdateSettings(ctx, true, true, true); !errors.Is(err, core.ErrNotificationsUnavailable) {
		t.Errorf("UpdateSettings with nil store: want ErrNotificationsUnavailable, got %v", err)
	}
}

func TestNotifyDeployStartedRespectsStartedPreference(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{
		{Subject: "alice", DeployStarted: true},
		{Subject: "bob", DeployStarted: false},
	}
	mailer := &fakeMailer{}
	svc := newTestService(st, nil, mailer, fakeIdentities{
		"alice": "alice@example.com",
		"bob":   "bob@example.com",
	})

	svc.NotifyDeployStarted(context.Background(), "tea-a", "web", "default")

	if len(mailer.sent) != 1 || mailer.sent[0].to != "alice@example.com" {
		t.Fatalf("sent = %+v, want exactly one deploy-start mail to opted-in alice", mailer.sent)
	}
	if mailer.sent[0].subject != "Deploy started: web" ||
		mailer.sent[0].body != "A deploy of \"web\" has started. We'll email you when it finishes.\n" {
		t.Errorf("deploy-start mail = %+v", mailer.sent[0])
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

	svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployLive, NotifyOnFail: "default"})

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

	svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployUpdateFailed, NotifyOnFail: "default"})

	if len(mailer.sent) != 1 || mailer.sent[0].to != "bob@example.com" {
		t.Fatalf("sent = %+v, want exactly one mail to bob (alice opted out of failure mail)", mailer.sent)
	}
}

// TestNotifyDeployFailedHonorsNotifyOnFailOverride (w4/m21) is the decision
// table the DoD calls for: notifyOnFail (ignore/default/notify) crossed with
// each recipient's own deployFailed opt-in/opt-out. "default" is covered by
// TestNotifyDeployFailedRespectsDeployFailedPreference above; this covers the
// two override values, each against both an opted-in and an opted-out member,
// so the override is proven to win over — not merely coexist with — the
// member's own preference.
func TestNotifyDeployFailedHonorsNotifyOnFailOverride(t *testing.T) {
	cases := []struct {
		name         string
		notifyOnFail string
		wantSent     []string // emails expected to receive the failure mail
	}{
		{
			name:         "ignore mutes everyone regardless of their own opt-in",
			notifyOnFail: "ignore",
			wantSent:     nil, // alice opted in, bob opted out — neither gets mail
		},
		{
			name:         "notify forces everyone regardless of their own opt-out",
			notifyOnFail: "notify",
			wantSent:     []string{"alice@example.com", "bob@example.com"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := newFakeStore()
			st.recipients["tea-a"] = []store.NotifyRecipient{
				{Subject: "alice", DeploySucceeded: true, DeployFailed: true}, // opted IN to failure mail
				{Subject: "bob", DeploySucceeded: true, DeployFailed: false},  // opted OUT of failure mail
			}
			mailer := &fakeMailer{}
			identities := fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"}
			svc := newTestService(st, nil, mailer, identities)

			svc.NotifyDeploy(context.Background(), store.DeployNotification{
				TenantID: "tea-a", AppName: "web", Status: store.DeployUpdateFailed, NotifyOnFail: c.notifyOnFail,
			})

			var got []string
			for _, m := range mailer.sent {
				got = append(got, m.to)
			}
			if !equalUnordered(got, c.wantSent) {
				t.Fatalf("notifyOnFail=%q sent = %v, want %v", c.notifyOnFail, got, c.wantSent)
			}
		})
	}
}

// TestNotifyDeploySucceededIgnoresNotifyOnFail (w4/m21) pins the DoD's scope
// claim: notifyOnFail governs FAILURE notifications only. Even the strongest
// override value ("ignore") must not suppress a success email — it keeps
// following each member's own deploySucceeded preference, unmodified.
func TestNotifyDeploySucceededIgnoresNotifyOnFail(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{
		{Subject: "alice", DeploySucceeded: true, DeployFailed: true},
	}
	mailer := &fakeMailer{}
	svc := newTestService(st, nil, mailer, fakeIdentities{"alice": "alice@example.com"})

	svc.NotifyDeploy(context.Background(), store.DeployNotification{
		TenantID: "tea-a", AppName: "web", Status: store.DeployLive, NotifyOnFail: "ignore",
	})

	if len(mailer.sent) != 1 || mailer.sent[0].to != "alice@example.com" {
		t.Fatalf("sent = %+v, want alice still notified on success despite notifyOnFail=ignore", mailer.sent)
	}
}

// TestNotifyDeployFailedUnrecognizedNotifyOnFailDefersToPreference guards the
// fallback branch: a value that isn't ignore/notify/default (e.g. legacy ""
// from an App created before this field existed) behaves exactly like
// "default" rather than silently muting or forcing everyone.
func TestNotifyDeployFailedUnrecognizedNotifyOnFailDefersToPreference(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{
		{Subject: "alice", DeploySucceeded: true, DeployFailed: false},
		{Subject: "bob", DeploySucceeded: true, DeployFailed: true},
	}
	mailer := &fakeMailer{}
	identities := fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"}
	svc := newTestService(st, nil, mailer, identities)

	svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployUpdateFailed, NotifyOnFail: ""})

	if len(mailer.sent) != 1 || mailer.sent[0].to != "bob@example.com" {
		t.Fatalf("sent = %+v, want exactly one mail to bob (legacy empty notifyOnFail behaves as default)", mailer.sent)
	}
}

// equalUnordered compares two string slices ignoring order — mailer.sent's
// order depends on the notify goroutines' scheduling, not on input order.
func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, s := range a {
		count[s]++
	}
	for _, s := range b {
		count[s]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}

func TestNotifyDeployIgnoresNonTerminalStatus(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeploySucceeded: true, DeployFailed: true}}
	mailer := &fakeMailer{}
	svc := newTestService(st, nil, mailer, fakeIdentities{"alice": "alice@example.com"})

	svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployCanceled, NotifyOnFail: "default"})

	if len(mailer.sent) != 0 {
		t.Errorf("sent = %+v, want none (canceled is not a success/failure this feature notifies on)", mailer.sent)
	}
}

func TestNotifyDeployNoopWithoutMailerOrStore(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeploySucceeded: true, DeployFailed: true}}

	// No mailer: nothing to send to, must not panic.
	svc := newTestService(st, nil, nil, fakeIdentities{"alice": "alice@example.com"})
	svc.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployLive, NotifyOnFail: "default"})

	// No store: nothing to look recipients up from, must not panic.
	svc2 := newTestService(nil, nil, &fakeMailer{}, fakeIdentities{"alice": "alice@example.com"})
	svc2.NotifyDeploy(context.Background(), store.DeployNotification{TenantID: "tea-a", AppName: "web", Status: store.DeployLive, NotifyOnFail: "default"})
}

// TestDeployEmailContent covers the enriched deploy-email body (w7/m44): the
// per-kind impact framing, the commit block (present when the deploy has a
// commit, omitted otherwise), and the "View Logs" link (present when a logs URL
// is supplied, omitted otherwise).
func TestDeployEmailContent(t *testing.T) {
	const commit = "fix(backend): correct typo computeUnitePerDay to computeUnitPerDay\n\n- rename across all files"
	const logsURL = "https://dashboard.bex.co/services/web/deploys/dep-123"

	t.Run("failure carries framing, commit, and logs link", func(t *testing.T) {
		subject, body := deployEmail("web", deployMailFailed, deployDetails{commitMessage: commit}, logsURL)
		if subject != "Deploy failed: web" {
			t.Errorf("subject = %q", subject)
		}
		for _, want := range []string{
			"We encountered an error during the deploy process for \"web\"",
			"may not be live",
			"Commit:\n" + commit,
			"View logs:\n" + logsURL,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("failure body missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("image-backed deploy (no commit) omits the commit block", func(t *testing.T) {
		_, body := deployEmail("web", deployMailFailed, deployDetails{}, logsURL)
		if strings.Contains(body, "Commit:") {
			t.Errorf("body should omit the empty commit block:\n%s", body)
		}
		if !strings.Contains(body, "View logs:\n"+logsURL) {
			t.Errorf("body should still carry the logs link:\n%s", body)
		}
	})

	t.Run("no dashboard URL omits the View Logs line", func(t *testing.T) {
		_, body := deployEmail("web", deployMailFailed, deployDetails{commitMessage: commit}, "")
		if strings.Contains(body, "View logs") {
			t.Errorf("body should omit the logs line when no URL is available:\n%s", body)
		}
	})

	t.Run("succeeded and started frame their own outcome", func(t *testing.T) {
		_, ok := deployEmail("web", deployMailSucceeded, deployDetails{}, "")
		if !strings.Contains(ok, "live") {
			t.Errorf("succeeded body = %q", ok)
		}
		_, started := deployEmail("web", deployMailStarted, deployDetails{}, "")
		if !strings.Contains(started, "has started") {
			t.Errorf("started body = %q", started)
		}
	})
}

// TestDeployLogsURL covers the "View Logs" deep-link builder: the deploy-detail
// route when the dashboard URL is set, and honest-omit when it (or the deploy
// id) is absent.
func TestDeployLogsURL(t *testing.T) {
	set := &Service{DashboardBaseURL: "https://dashboard.bex.co"}
	if got := set.deployLogsURL("web", "dep-123"); got != "https://dashboard.bex.co/services/web/deploys/dep-123" {
		t.Errorf("deployLogsURL = %q", got)
	}
	if got := set.deployLogsURL("web", ""); got != "" {
		t.Errorf("no deploy id should omit the link, got %q", got)
	}
	unset := &Service{}
	if got := unset.deployLogsURL("web", "dep-123"); got != "" {
		t.Errorf("no dashboard URL should omit the link, got %q", got)
	}
}

// TestNotifyDeployFailedIncludesCommitAndLink is the end-to-end assertion that
// a failed-deploy notification carries the commit message and the View Logs
// deep link the reconciler now threads through DeployNotification (w7/m44).
func TestNotifyDeployFailedIncludesCommitAndLink(t *testing.T) {
	st := newFakeStore()
	st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeployFailed: true}}
	mailer := &fakeMailer{}
	svc := newTestService(st, nil, mailer, fakeIdentities{"alice": "alice@example.com"})
	svc.DashboardBaseURL = "https://dashboard.bex.co"

	svc.NotifyDeploy(context.Background(), store.DeployNotification{
		TenantID:      "tea-a",
		AppName:       "web",
		Status:        store.DeployUpdateFailed,
		DeployID:      "dep-123",
		CommitMessage: "fix: correct a typo",
		NotifyOnFail:  "default",
	})

	if len(mailer.sent) != 1 {
		t.Fatalf("sent = %+v, want one failure mail", mailer.sent)
	}
	body := mailer.sent[0].body
	if !strings.Contains(body, "fix: correct a typo") {
		t.Errorf("failure mail missing commit message:\n%s", body)
	}
	if !strings.Contains(body, "https://dashboard.bex.co/services/web/deploys/dep-123") {
		t.Errorf("failure mail missing View Logs link:\n%s", body)
	}
}
