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
		policy := failureNotificationPolicy(tc.toSend, tc.notifyOnFail)
		if got := wantsFailureEmail(policy, on); got != tc.wantOn {
			t.Errorf("%q/%q opted-in member = %v, want %v", tc.toSend, tc.notifyOnFail, got, tc.wantOn)
		}
		if got := wantsFailureEmail(policy, off); got != tc.wantOff {
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
