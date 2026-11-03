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

package store_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type alertMail struct{ to, subject, text string }

type alertMailer struct {
	mu   sync.Mutex
	sent []alertMail
}

func (m *alertMailer) Send(_ context.Context, to, subject, text, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, alertMail{to, subject, text})
	return nil
}

func (m *alertMailer) take() []alertMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.sent
	m.sent = nil
	return out
}

type alertEmails map[string]string

func (e alertEmails) LookupEmails(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if addr, ok := e[s]; ok {
			out[s] = addr
		}
	}
	return out, nil
}

// TestAlertEmailWorkerMailsFailures drives w2/m252 end to end on real Postgres:
// observed failure facts and a failed Blueprint sync become one email per
// member and resource, later failures inside the window become one digest, a
// member who turned failure email off and a service whose policy is `none`
// get nothing, and a replayed feed page counts nothing twice.
func TestAlertEmailWorkerMailsFailures(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	if _, err := pool.Exec(ctx, `DELETE FROM alert_email_watermarks`); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	alice, bob, carol := "alert-alice-"+stamp, "alert-bob-"+stamp, "alert-carol-"+stamp
	loud, err := st.CreateWorkspace(ctx, "alert-loud-"+stamp, store.PlanHobby, alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), loud.ID) })
	if err := st.AddMember(ctx, bob, loud.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertNotificationSettings(ctx, loud.ID, bob, false, false, false); err != nil {
		t.Fatal(err)
	}
	quiet, err := st.CreateWorkspace(ctx, "alert-quiet-"+stamp, store.PlanHobby, carol)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), quiet.ID) })
	web, err := st.CreateApp(ctx, store.App{TenantID: loud.ID, Name: "web-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	muted, err := st.CreateApp(ctx, store.App{TenantID: quiet.ID, Name: "muted-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}

	mailer := &alertMailer{}
	clock := time.Now()
	worker := &notifications.AlertEmailWorker{
		Store: st, Mailer: mailer,
		Identities:   alertEmails{alice: "alice@example.com", bob: "bob@example.com", carol: "carol@example.com"},
		Clock:        func() time.Time { return clock },
		Window:       10 * time.Minute,
		BlueprintLag: store.FeedCommitLag,
		Policy: func(_ context.Context, tenantID, _ string) (string, error) {
			if tenantID == quiet.ID {
				return appv1alpha1.NotificationsToSendNone, nil
			}
			return appv1alpha1.NotificationsToSendDefault, nil
		},
	}
	// Seed the cursors before any failure exists.
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fact := func(app store.App, key string, typ store.ServiceEventFactType, status string) {
		t.Helper()
		if _, err := st.InsertServiceEventFact(ctx, store.ServiceEventFact{
			SourceKey: key + "-" + stamp, AppID: app.ID, Type: typ, At: time.Now(), Status: status,
		}); err != nil {
			t.Fatal(err)
		}
	}
	advance := func() {
		t.Helper()
		clock = time.Now().Add(store.FeedCommitLag + time.Second)
		if err := worker.RunOnce(ctx); err != nil {
			t.Fatalf("run: %v", err)
		}
	}

	fact(web, "fail-1", store.EventFactServerFailed, "")
	fact(web, "cron-1", store.EventFactCronRunEnded, store.EventStatusFailed)
	fact(web, "cron-ok", store.EventFactCronRunEnded, store.EventStatusSucceeded)
	fact(web, "pull-1", store.EventFactImagePullFailed, "")
	fact(web, "up-1", store.EventFactServerAvailable, "")
	fact(muted, "muted-1", store.EventFactServerFailed, "")
	blueprintID := "blp-alert" + stamp[len(stamp)-12:]
	if _, err := pool.Exec(ctx, `INSERT INTO blueprints (id, tenant_id, name, repo, branch) VALUES ($1, $2, 'stack', $3, 'main')`,
		blueprintID, loud.ID, "https://github.com/acme/"+stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO blueprint_syncs (id, blueprint_id, state, completed_at, error_message) VALUES ($1, $2, 'error', now(), 'render.yaml: services[0].plan: unknown plan')`,
		"exs-alert"+stamp[len(stamp)-11:], blueprintID); err != nil {
		t.Fatal(err)
	}

	advance()
	subjects := map[string]alertMail{}
	for _, m := range mailer.take() {
		if m.to != "alice@example.com" {
			t.Errorf("mailed %s (%q); only alice wants failure email", m.to, m.subject)
		}
		subjects[m.subject] = m
	}
	for _, want := range []string{
		"Service unavailable: " + web.Name,
		"Cron job failed: " + web.Name,
		"Image pull failed: " + web.Name,
		"Blueprint sync failed: stack",
	} {
		if _, ok := subjects[want]; !ok {
			t.Errorf("missing %q in %v", want, subjects)
		}
	}
	if len(subjects) != 4 {
		t.Errorf("sent %d distinct mails, want 4 (no recovery or success mail): %v", len(subjects), subjects)
	}
	if m := subjects["Blueprint sync failed: stack"]; !strings.Contains(m.text, "unknown plan") {
		t.Errorf("blueprint mail lacks the sync error: %q", m.text)
	}

	// Two more failures inside the window: nothing now, one digest after it.
	fact(web, "fail-2", store.EventFactServerFailed, "")
	fact(web, "fail-3", store.EventFactServerFailed, "")
	advance()
	if got := mailer.take(); len(got) != 0 {
		t.Fatalf("mailed inside the digest window: %v", got)
	}

	// A replica with a stale cursor re-reads the same page; nothing is counted twice.
	if _, err := pool.Exec(ctx, `UPDATE alert_email_watermarks SET at = at - interval '1 hour' WHERE name = 'feed'`); err != nil {
		t.Fatal(err)
	}
	replica := &notifications.AlertEmailWorker{Store: st, Policy: worker.Policy, Clock: func() time.Time { return clock }, Window: worker.Window}
	if err := replica.RunOnce(ctx); err != nil {
		t.Fatalf("replica: %v", err)
	}

	clock = clock.Add(11 * time.Minute)
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	digest := mailer.take()
	if len(digest) != 1 || digest[0].subject != fmt.Sprintf("Service unavailable: %s (2 times)", web.Name) {
		t.Fatalf("digest = %v, want one 'Service unavailable … (2 times)'", digest)
	}
	if !strings.Contains(digest[0].text, "2 times since") {
		t.Errorf("digest text does not count the failures: %q", digest[0].text)
	}

	// Membership removal cascades the alert state away.
	if _, err := pool.Exec(ctx, `DELETE FROM tenant_members WHERE tenant_id = $1 AND subject = $2`, loud.ID, alice); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM alert_email_streams WHERE tenant_id = $1 AND subject = $2)
		+ (SELECT count(*) FROM alert_email_events WHERE tenant_id = $1 AND subject = $2)`, loud.ID, alice).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d alert rows survived alice's membership", left)
	}
}

// TestAlertEmailStreamLifecycle pins the stream arithmetic: a failure arriving
// while a mail is in flight stays pending and falls due when the window after
// that mail closes; a replica that lost its lease cannot complete; a retry
// moves the due time; a sweep keeps pending streams.
func TestAlertEmailStreamLifecycle(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	owner := "alert-life-" + stamp
	ws, err := st.CreateWorkspace(ctx, "alert-life-"+stamp, store.PlanHobby, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	if _, err := pool.Exec(ctx, `INSERT INTO alert_email_watermarks (name, at) VALUES ('feed', now()) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	var wmAt time.Time
	var wmKey string
	if err := pool.QueryRow(ctx, `SELECT at, key FROM alert_email_watermarks WHERE name = 'feed'`).Scan(&wmAt, &wmKey); err != nil {
		t.Fatal(err)
	}
	window := 10 * time.Minute
	t0 := time.Now().UTC().Truncate(time.Second)
	item := func(key string, at time.Time) store.AlertEmailItem {
		return store.AlertEmailItem{TenantID: ws.ID, Subject: owner, SourceKey: key + stamp, ResourceID: "srv-life",
			Event: store.AlertEmailServerFailed, ResourceName: "life", OccurredAt: at}
	}
	enqueue := func(items ...store.AlertEmailItem) {
		t.Helper()
		if err := st.EnqueueAlertEmails(ctx, store.AlertEmailFeedWatermark, items, window, wmAt, wmKey); err != nil {
			t.Fatal(err)
		}
	}
	claim := func(now time.Time) []store.AlertEmailStream {
		t.Helper()
		all, err := st.ClaimDueAlertEmailStreams(ctx, now, now.Add(time.Minute), 100)
		if err != nil {
			t.Fatal(err)
		}
		var out []store.AlertEmailStream
		for _, s := range all {
			if s.TenantID == ws.ID {
				out = append(out, s)
			}
		}
		return out
	}

	enqueue(item("a", t0), item("a", t0)) // the same source twice counts once
	claimed := claim(t0)
	if len(claimed) != 1 || claimed[0].PendingCount != 1 {
		t.Fatalf("first claim = %+v", claimed)
	}
	if again := claim(t0); len(again) != 0 {
		t.Fatalf("a leased stream was claimed twice: %+v", again)
	}
	enqueue(item("b", t0.Add(time.Second))) // arrives while the mail is in flight
	lost := claimed[0]
	lost.ClaimedUntil = lost.ClaimedUntil.Add(time.Second)
	if ok, err := st.CompleteAlertEmailStream(ctx, lost, t0, window); err != nil || ok {
		t.Fatalf("a lost lease completed: ok=%v err=%v", ok, err)
	}
	if ok, err := st.CompleteAlertEmailStream(ctx, claimed[0], t0, window); err != nil || !ok {
		t.Fatalf("complete: ok=%v err=%v", ok, err)
	}
	if early := claim(t0.Add(window - time.Second)); len(early) != 0 {
		t.Fatalf("the in-flight failure fell due inside the window: %+v", early)
	}
	due := claim(t0.Add(window))
	if len(due) != 1 || due[0].PendingCount != 1 || len(due[0].Occurrences) != 1 || due[0].LastMailedAt == nil || !due[0].Occurrences[0].Equal(t0.Add(time.Second)) {
		t.Fatalf("digest claim = %+v", due)
	}
	next := t0.Add(window + 5*time.Minute)
	if ok, err := st.RetryAlertEmailStream(ctx, due[0], next); err != nil || !ok {
		t.Fatalf("retry: ok=%v err=%v", ok, err)
	}
	if got := claim(next.Add(-time.Second)); len(got) != 0 {
		t.Fatal("a retried stream fell due before its retry time")
	}
	if err := st.SweepAlertEmails(ctx, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := claim(next); len(got) != 1 || got[0].Attempts != 1 {
		t.Fatalf("after sweep the pending stream = %+v, want it kept with attempts=1", got)
	}
}
