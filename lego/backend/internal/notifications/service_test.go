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
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

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
	mu         sync.Mutex
	rows       map[[2]string]store.NotificationSettings // (tenant, subject) -> row
	recipients map[string][]store.NotifyRecipient       // tenant -> recipients
	devices    map[[3]string]store.DevicePushSubscription
	browsers   map[[3]string]store.WebPushSubscription
	push       map[[2]string][]store.PushNotification
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		rows: map[[2]string]store.NotificationSettings{}, recipients: map[string][]store.NotifyRecipient{},
		devices:  map[[3]string]store.DevicePushSubscription{},
		browsers: map[[3]string]store.WebPushSubscription{},
		push:     map[[2]string][]store.PushNotification{},
	}
}

func (f *fakeStore) GetNotificationSettings(_ context.Context, tenantID, subject string) (store.NotificationSettings, error) {
	row, ok := f.rows[[2]string{tenantID, subject}]
	if !ok {
		return store.NotificationSettings{}, store.ErrNotFound
	}
	return row, nil
}

func (f *fakeStore) UpsertNotificationSettings(_ context.Context, tenantID, subject string, deployStarted, deploySucceeded, deployFailed bool) (store.NotificationSettings, error) {
	row := f.rows[[2]string{tenantID, subject}]
	row.ID, row.TenantID, row.Subject = "ntf-fake", tenantID, subject
	row.DeployStarted, row.DeploySucceeded, row.DeployFailed = deployStarted, deploySucceeded, deployFailed
	f.rows[[2]string{tenantID, subject}] = row
	return row, nil
}

func (f *fakeStore) UpsertNotificationPushPolicy(_ context.Context, tenantID, subject string, policy json.RawMessage) (store.NotificationSettings, error) {
	row, existed := f.rows[[2]string{tenantID, subject}]
	row.ID, row.TenantID, row.Subject = "ntf-fake", tenantID, subject
	if !existed {
		row.DeployFailed = true
	}
	row.PushPolicy = append(json.RawMessage(nil), policy...)
	f.rows[[2]string{tenantID, subject}] = row
	return row, nil
}

func (f *fakeStore) ListNotifyRecipients(_ context.Context, tenantID string) ([]store.NotifyRecipient, error) {
	return f.recipients[tenantID], nil
}

func (f *fakeStore) UpsertDevicePushSubscription(_ context.Context, sub store.DevicePushSubscription) (store.DevicePushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now().UTC()
	for key, other := range f.devices {
		if other.Provider == sub.Provider && other.Token == sub.Token && key != [3]string{sub.TenantID, sub.Subject, sub.DeviceID} {
			delete(f.devices, key)
		}
	}
	key := [3]string{sub.TenantID, sub.Subject, sub.DeviceID}
	if old, ok := f.devices[key]; ok {
		sub.CreatedAt = old.CreatedAt
	} else {
		sub.CreatedAt = now
	}
	sub.UpdatedAt, sub.LastRegisteredAt = now, now
	f.devices[key] = sub
	return sub, nil
}

func (f *fakeStore) ListOwnDevicePushSubscriptions(_ context.Context, tenantID, subject string) ([]store.DevicePushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.DevicePushSubscription
	for key, sub := range f.devices {
		if key[0] == tenantID && key[1] == subject {
			sub.Token = "" // the production own-list projection never reads it
			out = append(out, sub)
		}
	}
	return out, nil
}

func (f *fakeStore) RevokeDevicePushSubscription(_ context.Context, tenantID, subject, deviceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [3]string{tenantID, subject, deviceID}
	if _, ok := f.devices[key]; !ok {
		return false, nil
	}
	delete(f.devices, key)
	return true, nil
}

func (f *fakeStore) RevokeAllDevicePushSubscriptions(_ context.Context, tenantID, subject string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	for key := range f.devices {
		if key[0] == tenantID && key[1] == subject {
			delete(f.devices, key)
			count++
		}
	}
	return count, nil
}

func (f *fakeStore) UpsertWebPushSubscription(_ context.Context, sub store.WebPushSubscription) (store.WebPushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.browsers == nil {
		f.browsers = map[[3]string]store.WebPushSubscription{}
	}
	now := time.Now().UTC()
	for key, other := range f.browsers {
		if other.Endpoint == sub.Endpoint && key != [3]string{sub.TenantID, sub.Subject, sub.BrowserID} {
			delete(f.browsers, key)
		}
	}
	key := [3]string{sub.TenantID, sub.Subject, sub.BrowserID}
	if old, ok := f.browsers[key]; ok {
		sub.CreatedAt = old.CreatedAt
	} else {
		sub.CreatedAt = now
	}
	sub.UpdatedAt, sub.LastRegisteredAt = now, now
	f.browsers[key] = sub
	return sub, nil
}

func (f *fakeStore) ListOwnWebPushSubscriptions(_ context.Context, tenantID, subject string) ([]store.WebPushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.WebPushSubscription
	for key, sub := range f.browsers {
		if key[0] == tenantID && key[1] == subject {
			sub.Endpoint, sub.P256dh, sub.Auth, sub.EndpointDigest = "", "", "", ""
			out = append(out, sub)
		}
	}
	return out, nil
}

func (f *fakeStore) RevokeWebPushSubscription(_ context.Context, tenantID, subject, browserID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [3]string{tenantID, subject, browserID}
	if _, ok := f.browsers[key]; !ok {
		return false, nil
	}
	delete(f.browsers, key)
	return true, nil
}

func (f *fakeStore) RevokeAllWebPushSubscriptions(_ context.Context, tenantID, subject string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	for key := range f.browsers {
		if key[0] == tenantID && key[1] == subject {
			delete(f.browsers, key)
			count++
		}
	}
	return count, nil
}

func (f *fakeStore) ListOwnPushNotifications(_ context.Context, tenantID, subject string, limit int) ([]store.PushNotification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rows := f.push[[2]string{tenantID, subject}]
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return append([]store.PushNotification(nil), rows...), nil
}

func (f *fakeStore) MarkOwnPushNotificationRead(_ context.Context, tenantID, subject, eventID string, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [2]string{tenantID, subject}
	for i := range f.push[key] {
		if f.push[key][i].EventID != eventID {
			continue
		}
		if f.push[key][i].ReadAt == nil {
			readAt := at
			f.push[key][i].ReadAt = &readAt
		}
		return true, nil
	}
	return false, nil
}

func (f *fakeStore) CountUnreadPushNotifications(_ context.Context, tenantID, subject string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var count int64
	for _, row := range f.push[[2]string{tenantID, subject}] {
		if row.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

// fakeMailer records every send. body holds the plain-text part (the existing
// assertions pin it); html holds the branded alternative.
type fakeMailer struct {
	mu   sync.Mutex
	sent []struct{ to, subject, body, html string }
	err  error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, text, html string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, struct{ to, subject, body, html string }{to, subject, text, html})
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

	t.Run("failure carries framing, commit, and logs link — exact text parity", func(t *testing.T) {
		subject, msg := deployEmail("web", deployMailFailed, deployDetails{commitMessage: commit}, logsURL)
		if subject != "Deploy failed: web" {
			t.Errorf("subject = %q", subject)
		}
		// Byte-identical to the pre-w1/m54 plain-text body.
		want := "We encountered an error during the deploy process for \"web\". " +
			"This means your deploy didn't complete successfully and your latest changes may not be live.\n\n" +
			"Commit:\n" + commit + "\n\n" +
			"View logs:\n" + logsURL + "\n"
		if got := msg.Text(); got != want {
			t.Errorf("failure text drift:\n got %q\nwant %q", got, want)
		}
		// The HTML alternative renders the logs URL as a button href.
		if html := msg.HTML(); !strings.Contains(html, `href="`+logsURL+`"`) {
			t.Errorf("HTML missing the View logs button href:\n%s", html)
		}
	})

	t.Run("image-backed deploy (no commit) omits the commit block", func(t *testing.T) {
		_, msg := deployEmail("web", deployMailFailed, deployDetails{}, logsURL)
		body := msg.Text()
		if strings.Contains(body, "Commit:") {
			t.Errorf("body should omit the empty commit block:\n%s", body)
		}
		if !strings.Contains(body, "View logs:\n"+logsURL) {
			t.Errorf("body should still carry the logs link:\n%s", body)
		}
	})

	t.Run("no dashboard URL omits the View Logs line and CTA button", func(t *testing.T) {
		_, msg := deployEmail("web", deployMailFailed, deployDetails{commitMessage: commit}, "")
		if body := msg.Text(); strings.Contains(body, "View logs") {
			t.Errorf("body should omit the logs line when no URL is available:\n%s", body)
		}
		if html := msg.HTML(); strings.Contains(html, "Or open this link") {
			t.Errorf("HTML should render no CTA button when no logs URL is available:\n%s", html)
		}
	})

	t.Run("succeeded and started frame their own outcome — exact text parity", func(t *testing.T) {
		_, ok := deployEmail("web", deployMailSucceeded, deployDetails{}, "")
		if got, want := ok.Text(), "A new deploy of \"web\" is live. Your latest changes are now serving.\n"; got != want {
			t.Errorf("succeeded text = %q, want %q", got, want)
		}
		_, started := deployEmail("web", deployMailStarted, deployDetails{}, "")
		if got, want := started.Text(), "A deploy of \"web\" has started. We'll email you when it finishes.\n"; got != want {
			t.Errorf("started text = %q, want %q", got, want)
		}
	})

	t.Run("commit SHA + repo URL render the commit as one linked block (no duplicate)", func(t *testing.T) {
		_, msg := deployEmail("web", deployMailFailed, deployDetails{
			commitMessage: commit,
			commitSHA:     "abc1234def5678",
			repoURL:       "https://github.com/acme/web",
		}, logsURL)
		// The commit is a single "Commit <sha>" block (message + URL), before
		// the View logs CTA — not a paragraph plus a separate trailing link.
		want := "We encountered an error during the deploy process for \"web\". " +
			"This means your deploy didn't complete successfully and your latest changes may not be live.\n\n" +
			"Commit abc1234 on github.com\n" + commit + "\n" +
			"https://github.com/acme/web/commit/abc1234def5678\n\n" +
			"View logs:\n" + logsURL + "\n"
		if got := msg.Text(); got != want {
			t.Errorf("text with commit reference drift:\n got %q\nwant %q", got, want)
		}
		html := msg.HTML()
		if !strings.Contains(html, `href="https://github.com/acme/web/commit/abc1234def5678"`) || !strings.Contains(html, ">abc1234 on github.com</a>") {
			t.Errorf("HTML commit SHA not linked:\n%s", html)
		}
		// The reference is not duplicated as a "View commit" line.
		if strings.Contains(html, "View commit") {
			t.Errorf("commit link duplicated as a separate line:\n%s", html)
		}
	})

	t.Run("self-hosted forge hostname is visible in the trusted email label", func(t *testing.T) {
		_, msg := deployEmail("web", deployMailFailed, deployDetails{
			commitMessage: commit,
			commitSHA:     "abc1234def5678",
			repoURL:       "https://attacker.example/acme/web",
		}, logsURL)
		if !strings.Contains(msg.Text(), "Commit abc1234 on attacker.example") ||
			!strings.Contains(msg.HTML(), ">abc1234 on attacker.example</a>") {
			t.Fatalf("commit link hid its destination hostname:\n%s", msg.HTML())
		}
	})

	t.Run("no repo (image-backed) keeps the plain commit paragraph", func(t *testing.T) {
		_, msg := deployEmail("web", deployMailFailed, deployDetails{commitMessage: commit, repoURL: ""}, logsURL)
		body := msg.Text()
		if !strings.Contains(body, "Commit:\n"+commit) {
			t.Errorf("no repo URL should keep the plain commit paragraph:\n%s", body)
		}
		if strings.Contains(msg.HTML(), "<a href=\"https://github.com") {
			t.Errorf("no repo URL must render no commit link:\n%s", msg.HTML())
		}
	})
}

// TestCommitURL covers the clone-URL → web-commit-URL normalization across the
// shapes App.spec.repo actually takes, and the honest-omit inputs.
func TestCommitURL(t *testing.T) {
	const sha = "abc1234def"
	cases := []struct {
		name, repo, sha, want string
	}{
		{"github https", "https://github.com/acme/web", sha, "https://github.com/acme/web/commit/" + sha},
		{"github https .git", "https://github.com/acme/web.git", sha, "https://github.com/acme/web/commit/" + sha},
		{"github scp ssh", "git@github.com:acme/web.git", sha, "https://github.com/acme/web/commit/" + sha},
		{"github ssh url", "ssh://git@github.com/acme/web.git", sha, "https://github.com/acme/web/commit/" + sha},
		{"credentials stripped", "https://x-token:secret@github.com/acme/web.git", sha, "https://github.com/acme/web/commit/" + sha},
		{"gitlab path segment", "https://gitlab.com/acme/web", sha, "https://gitlab.com/acme/web/-/commit/" + sha},
		{"bitbucket path segment", "https://bitbucket.org/acme/web", sha, "https://bitbucket.org/acme/web/commits/" + sha},
		{"empty repo", "", sha, ""},
		{"empty sha", "https://github.com/acme/web", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commitURL(tc.repo, tc.sha); got != tc.want {
				t.Errorf("commitURL(%q, %q) = %q, want %q", tc.repo, tc.sha, got, tc.want)
			}
		})
	}
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
