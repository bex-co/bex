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
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestClassifyAlertEmail(t *testing.T) {
	fact := func(typ store.ServiceEventFactType, status string) store.WebhookEventRow {
		return store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(typ), Status: status}
	}
	for _, tc := range []struct {
		name  string
		row   store.WebhookEventRow
		event string
	}{
		{"unhealthy", fact(store.EventFactServerFailed, ""), store.AlertEmailServerFailed},
		{"lost with its node", fact(store.EventFactServerHardwareFailure, ""), store.AlertEmailServerFailed},
		{"image pull", fact(store.EventFactImagePullFailed, ""), store.AlertEmailImagePullFailed},
		{"cron failed", fact(store.EventFactCronRunEnded, store.EventStatusFailed), store.AlertEmailCronFailed},
		{"cron succeeded", fact(store.EventFactCronRunEnded, store.EventStatusSucceeded), ""},
		{"cron canceled", fact(store.EventFactCronRunEnded, store.EventStatusCanceled), ""},
		{"one-off job failed (Render: Slack only)", fact(store.EventFactJobRunEnded, store.EventStatusFailed), ""},
		{"recovered (Render: Slack only)", fact(store.EventFactServerAvailable, ""), ""},
		{"suspended by a user", fact(store.EventFactServiceSuspended, ""), ""},
		{"deploy row", store.WebhookEventRow{Source: store.EventSourceDeploy, Status: store.DeployBuildFailed}, ""},
	} {
		event, _, ok := classifyAlertEmail(tc.row)
		if event != tc.event || ok != (tc.event != "") {
			t.Errorf("%s: classify = %q,%v want %q", tc.name, event, ok, tc.event)
		}
	}
}

func TestFailurePolicyDecidesWhoIsMailed(t *testing.T) {
	on := store.AlertEmailRecipient{DeployFailed: true}
	off := store.AlertEmailRecipient{DeployFailed: false}
	for _, tc := range []struct {
		toSend, notifyOnFail string
		wantOn, wantOff      bool
	}{
		{"", "", true, false},
		{appv1alpha1.NotificationsToSendDefault, "", true, false},
		{appv1alpha1.NotificationsToSendNone, "", false, false},
		{appv1alpha1.NotificationsToSendFailure, "", true, true},
		{appv1alpha1.NotificationsToSendAll, "", true, true},
		{"", "notify", true, true},
		{"", "ignore", false, false},
		// The richer policy wins over the legacy field.
		{appv1alpha1.NotificationsToSendNone, "notify", false, false},
	} {
		// Under Render's default workspace policy (email on, failure only).
		policy := emailPolicy{owner: defaultOwnerPolicy, service: failureNotificationPolicy(tc.toSend, tc.notifyOnFail)}
		if got := policy.wants(levelFailure, toggleFailed, alertPrefs(on)); got != tc.wantOn {
			t.Errorf("%q/%q opted-in member = %v, want %v", tc.toSend, tc.notifyOnFail, got, tc.wantOn)
		}
		if got := policy.wants(levelFailure, toggleFailed, alertPrefs(off)); got != tc.wantOff {
			t.Errorf("%q/%q opted-out member = %v, want %v", tc.toSend, tc.notifyOnFail, got, tc.wantOff)
		}
	}
}

func TestAlertEmailCopy(t *testing.T) {
	at := time.Date(2026, 11, 3, 1, 2, 3, 0, time.UTC)
	single, msg := alertEmail(store.AlertEmailStream{
		Event: store.AlertEmailServerFailed, ResourceID: "srv-1", ResourceName: "api",
		Occurrences: []time.Time{at}, PendingCount: 1,
	}, "https://dashboard.example/services/srv-1/events")
	if single != "Service unavailable: api" || !strings.Contains(msg.Text(), "2026-11-03 01:02:03 UTC") ||
		!strings.Contains(msg.Text(), "https://dashboard.example/services/srv-1/events") {
		t.Errorf("single = %q / %q", single, msg.Text())
	}

	last := at.Add(-time.Hour)
	times := make([]time.Time, 0, store.AlertEmailOccurrenceCap+5)
	for i := store.AlertEmailOccurrenceCap + 4; i >= 0; i-- { // unsorted on purpose
		times = append(times, at.Add(time.Duration(i)*time.Second))
	}
	digest, msg := alertEmail(store.AlertEmailStream{
		Event: store.AlertEmailCronFailed, ResourceName: "nightly",
		Occurrences: times, PendingCount: 40, LastMailedAt: &last,
	}, "")
	text := msg.Text()
	if digest != "Cron job failed: nightly (40 times)" {
		t.Errorf("digest subject = %q", digest)
	}
	newest := at.Add(time.Duration(store.AlertEmailOccurrenceCap+4) * time.Second)
	for _, want := range []string{"40 times since 2026-11-03 00:02 UTC", "and 20 earlier", newest.Format("15:04:05")} {
		if !strings.Contains(text, want) {
			t.Errorf("digest text lacks %q: %q", want, text)
		}
	}
	if strings.Contains(text, at.Format("15:04:05 UTC")) {
		t.Errorf("digest listed more than the newest %d times", store.AlertEmailOccurrenceCap)
	}

	blueprint, msg := alertEmail(store.AlertEmailStream{
		Event: store.AlertEmailBlueprintSyncFailed, ResourceName: "stack", Detail: "unknown plan",
		Occurrences: []time.Time{at}, PendingCount: 1,
	}, "")
	if blueprint != "Blueprint sync failed: stack" || !strings.Contains(msg.Text(), "unknown plan") {
		t.Errorf("blueprint = %q / %q", blueprint, msg.Text())
	}
}

type fakeAlertStore struct {
	AlertEmailStore
	streams   []store.AlertEmailStream
	completed []store.AlertEmailStream
	dropped   []store.AlertEmailStream
	retried   map[string]time.Time
}

func (f *fakeAlertStore) DropAlertEmailStream(_ context.Context, st store.AlertEmailStream) (bool, error) {
	f.dropped = append(f.dropped, st)
	return true, nil
}

func (f *fakeAlertStore) ClaimDueAlertEmailStreams(context.Context, time.Time, time.Time, int) ([]store.AlertEmailStream, error) {
	out := f.streams
	f.streams = nil
	return out, nil
}

func (f *fakeAlertStore) CompleteAlertEmailStream(_ context.Context, st store.AlertEmailStream, _ time.Time, _ time.Duration) (bool, error) {
	f.completed = append(f.completed, st)
	return true, nil
}

func (f *fakeAlertStore) RetryAlertEmailStream(_ context.Context, st store.AlertEmailStream, next time.Time) (bool, error) {
	if f.retried == nil {
		f.retried = map[string]time.Time{}
	}
	f.retried[st.Subject] = next
	return true, nil
}

type scriptedMailer map[string]error

func (m scriptedMailer) Send(_ context.Context, to, _, _, _ string) error { return m[to] }

type staticEmails struct {
	addrs map[string]string
	err   error
}

func (e staticEmails) LookupEmails(context.Context, []string) (map[string]string, error) {
	return e.addrs, e.err
}

func TestAlertEmailSendOutcomes(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	stream := func(subject string, attempts int) store.AlertEmailStream {
		return store.AlertEmailStream{Subject: subject, Event: store.AlertEmailServerFailed, ResourceID: "srv-1",
			ResourceName: "api", PendingCount: 1, Occurrences: []time.Time{now}, Attempts: attempts}
	}
	st := &fakeAlertStore{streams: []store.AlertEmailStream{
		stream("ok", 0), stream("noaddr", 0), stream("rejected", 0), stream("flaky", 2), stream("exhausted", alertEmailMaxAttempts-1),
	}}
	w := &AlertEmailWorker{
		Store: st, Clock: func() time.Time { return now },
		Identities: staticEmails{addrs: map[string]string{"ok": "ok@x", "rejected": "rejected@x", "flaky": "flaky@x", "exhausted": "exhausted@x"}},
		Mailer: scriptedMailer{
			"rejected@x":  &textproto.Error{Code: 550, Msg: "no such mailbox"},
			"flaky@x":     errors.New("connection reset"),
			"exhausted@x": errors.New("connection reset"),
		},
	}
	if err := w.send(t.Context()); err == nil {
		t.Fatal("a transient failure must surface as a tick error")
	}
	completed := map[string]bool{}
	for _, c := range st.completed {
		completed[c.Subject] = true
	}
	if len(st.dropped) != 1 || st.dropped[0].Subject != "exhausted" {
		t.Errorf("dropped = %+v, want only the exhausted stream (not recorded as mailed)", st.dropped)
	}
	for _, s := range []string{"ok", "noaddr", "rejected"} {
		if !completed[s] {
			t.Errorf("%s not completed (completed=%v)", s, completed)
		}
	}
	if completed["flaky"] {
		t.Error("a transient failure was completed instead of retried")
	}
	if next, ok := st.retried["flaky"]; !ok || next != now.Add(pushRetryDelay(3)) {
		t.Errorf("flaky retried at %v, want %v (attempt 3 backoff)", next, now.Add(pushRetryDelay(3)))
	}

	// An identity lookup that failed leaves an unresolved stream for retry.
	st = &fakeAlertStore{streams: []store.AlertEmailStream{stream("unread", 0)}}
	w.Store, w.Identities = st, staticEmails{err: errors.New("kratos down")}
	_ = w.send(t.Context())
	if _, ok := st.retried["unread"]; !ok || len(st.completed) != 0 {
		t.Errorf("unresolved address: retried=%v completed=%v", st.retried, st.completed)
	}
}

type enqueueRecorder struct {
	recipients []store.AlertEmailRecipient
	watermark  string
	items      []store.AlertEmailItem
}

func (r *enqueueRecorder) ListAlertEmailRecipientsFor(_ context.Context, tenantID string) ([]store.AlertEmailRecipient, error) {
	var out []store.AlertEmailRecipient
	for _, rec := range r.recipients {
		if rec.TenantID == tenantID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *enqueueRecorder) EnqueueAlertEmails(_ context.Context, watermark string, items []store.AlertEmailItem, _ time.Duration, _ time.Time, _ string) error {
	r.watermark = watermark
	r.items = append(r.items, items...)
	return nil
}

// w2/m253: a crash-loop suspension is a failure notification for the
// suspended service's workspace only, keyed so replicas mail it once.
func TestCrashLoopNotifierEnqueuesTheSuspension(t *testing.T) {
	since := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	rec := &enqueueRecorder{recipients: []store.AlertEmailRecipient{
		{TenantID: "tea-a", Subject: "alice", DeployFailed: true},
		{TenantID: "tea-a", Subject: "bob", DeployFailed: false},
		{TenantID: "tea-b", Subject: "carol", DeployFailed: true},
	}}
	n := CrashLoopNotifier{Store: rec, Window: 24 * time.Hour,
		Policy: func(context.Context, string, string) (string, error) {
			return appv1alpha1.NotificationsToSendDefault, nil
		}}
	if err := n.NotifyCrashLoopSuspended(t.Context(), "tea-a", "srv-1", "api", since, since.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if rec.watermark != "" || len(rec.items) != 1 {
		t.Fatalf("watermark=%q items=%+v, want one cursor-less item for alice", rec.watermark, rec.items)
	}
	item := rec.items[0]
	if item.Subject != "alice" || item.Event != store.AlertEmailServiceSuspended || item.SourceKey != "crashloop:srv-1:2026-11-02T12:00:00Z" {
		t.Errorf("item = %+v", item)
	}
	subject, msg := alertEmail(store.AlertEmailStream{Event: item.Event, ResourceName: item.ResourceName, Detail: item.Detail,
		Occurrences: []time.Time{item.OccurredAt}, PendingCount: 1}, "")
	if subject != "Service suspended: api" || !strings.Contains(msg.Text(), "over 24 hours") || strings.Contains(msg.Text(), "24h0m0s") {
		t.Errorf("suspension mail = %q / %q", subject, msg.Text())
	}

	rec.items = nil
	n.Policy = func(context.Context, string, string) (string, error) { return appv1alpha1.NotificationsToSendNone, nil }
	if err := n.NotifyCrashLoopSuspended(t.Context(), "tea-a", "srv-1", "api", since, since); err != nil || len(rec.items) != 0 {
		t.Errorf("policy none: err=%v items=%+v", err, rec.items)
	}
}

// w2/m255: a disk over 80% is a failure notification; back below 80% is sent
// only to "all notifications", as Render documents.
func TestDiskUsageNotifierLevels(t *testing.T) {
	d := store.DiskUsageDetails{DiskID: "dsk-1", DiskName: "data", MountPath: "/data", UsagePercent: 85.5, ThresholdPercent: 80}
	for _, tc := range []struct {
		policy    string
		workspace string // the workspace policy's level; "" = no row (Render's default)
		high      bool
		wantItems int
	}{
		{appv1alpha1.NotificationsToSendDefault, "", true, 1},
		{appv1alpha1.NotificationsToSendDefault, "", false, 0},
		{appv1alpha1.NotificationsToSendFailure, "", false, 0},
		{appv1alpha1.NotificationsToSendAll, "", false, 2},
		{appv1alpha1.NotificationsToSendNone, "", true, 0},
		// w3/m271: a service on default follows the workspace policy. "all"
		// sends the recovery to every member (no member preference speaks
		// about it); "none" suppresses even the failure.
		{appv1alpha1.NotificationsToSendDefault, appv1alpha1.NotificationsToSendAll, false, 2},
		{appv1alpha1.NotificationsToSendDefault, appv1alpha1.NotificationsToSendNone, true, 0},
		// An explicit service override still wins over the workspace.
		{appv1alpha1.NotificationsToSendFailure, appv1alpha1.NotificationsToSendAll, false, 0},
		{appv1alpha1.NotificationsToSendAll, appv1alpha1.NotificationsToSendNone, false, 2},
	} {
		rec := &enqueueRecorder{recipients: []store.AlertEmailRecipient{
			{TenantID: "tea-a", Subject: "alice", DeployFailed: true},
			{TenantID: "tea-a", Subject: "bob", DeployFailed: false},
		}}
		policy := tc.policy
		n := DiskUsageNotifier{Store: rec, Policy: func(context.Context, string, string) (string, error) { return policy, nil }}
		if tc.workspace != "" {
			n.Owners = staticOwners(OwnerPolicy{EmailEnabled: true, NotificationsToSend: tc.workspace})
		}
		if err := n.NotifyDiskUsage(t.Context(), "tea-a", "srv-1", "db", tc.high, d, "disk_usage:dsk-1:x:1", time.Now()); err != nil {
			t.Fatal(err)
		}
		if len(rec.items) != tc.wantItems {
			t.Errorf("policy %s workspace %q high=%v: %d items, want %d", tc.policy, tc.workspace, tc.high, len(rec.items), tc.wantItems)
		}
	}
	subject, msg := alertEmail(store.AlertEmailStream{Event: store.AlertEmailDiskUsageHigh, ResourceName: "db",
		Detail: diskUsageDetail(d), Occurrences: []time.Time{time.Now()}, PendingCount: 1}, "")
	if subject != "Disk usage high: db" || !strings.Contains(msg.Text(), `"data" mounted at /data is 85.5% full (threshold 80%)`) {
		t.Errorf("mail = %q / %q", subject, msg.Text())
	}
}
