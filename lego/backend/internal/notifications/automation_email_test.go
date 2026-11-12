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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeNoticeStore hands out each queued notice once, as the claim does.
type fakeNoticeStore struct {
	runs   []store.AutomationRunNotice
	pauses []store.AgentAutomation
	claims int
	err    error
}

func (f *fakeNoticeStore) ClaimAutomationRunNotices(context.Context, time.Time, int) ([]store.AutomationRunNotice, error) {
	f.claims++
	if f.err != nil {
		return nil, f.err
	}
	out := f.runs
	f.runs = nil
	return out, nil
}

func (f *fakeNoticeStore) ClaimAutomationPauseNotices(context.Context, time.Time, int) ([]store.AgentAutomation, error) {
	f.claims++
	out := f.pauses
	f.pauses = nil
	return out, nil
}

func automationFixture(setting string, streak int) store.AgentAutomation {
	return store.AgentAutomation{ID: "aut-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-a", Name: "rt-fix-logic-bugs",
		CreatedBy: "alice", NotifyEmail: setting, NotifyFailureStreak: streak}
}

var noticeFiredAt = time.Date(2026, time.November, 12, 8, 0, 0, 0, time.UTC)

func runNotice(setting, result, outcome string, streak int) store.AutomationRunNotice {
	n := store.AutomationRunNotice{Automation: automationFixture(setting, streak), Result: result, EndedAt: noticeFiredAt,
		Run: store.AgentAutomationRun{FireAt: noticeFiredAt, Outcome: outcome}}
	if outcome == store.AutomationRunStarted {
		n.Run.SessionID = "ags-c185th5c2rvvnhbfiltg"
	}
	return n
}

func TestFailureBackoffMailsPowersOfThree(t *testing.T) {
	var due []int
	for streak := 0; streak <= 30; streak++ {
		if failureBackoffDue(streak) {
			due = append(due, streak)
		}
	}
	if want := []int{1, 3, 9, 27}; len(due) != len(want) || due[0] != 1 || due[1] != 3 || due[2] != 9 || due[3] != 27 {
		t.Fatalf("failures mailed at %v, want %v", due, want)
	}
}

func TestAutomationRunMailDueMatrix(t *testing.T) {
	results := []string{store.AutomationRunResultSucceeded, store.AutomationRunResultFailed, store.AutomationRunResultCanceled}
	want := map[string][]bool{ // succeeded, failed (streak 1), canceled
		store.AutomationNotifyAlways:    {true, true, true},
		store.AutomationNotifyOnFailure: {false, true, false},
		store.AutomationNotifyOnSuccess: {true, false, false},
		store.AutomationNotifyNever:     {false, false, false},
	}
	for setting, row := range want {
		for i, result := range results {
			if got := automationRunMailDue(setting, result, 1); got != row[i] {
				t.Errorf("%s / %s = %v, want %v", setting, result, got, row[i])
			}
		}
		// The backoff applies to every failure mail, Always included.
		if automationRunMailDue(setting, store.AutomationRunResultFailed, 2) {
			t.Errorf("%s mailed the 2nd consecutive failure", setting)
		}
	}
}

func TestAutomationEmailWorkerMailsTheCreatorPerSetting(t *testing.T) {
	st := &fakeNoticeStore{
		runs: []store.AutomationRunNotice{
			runNotice(store.AutomationNotifyOnFailure, store.AutomationRunResultFailed, store.AutomationRunFailedCreate, 1),
			runNotice(store.AutomationNotifyOnFailure, store.AutomationRunResultFailed, store.AutomationRunFailedCreate, 2),
			runNotice(store.AutomationNotifyOnFailure, store.AutomationRunResultSucceeded, store.AutomationRunStarted, 0),
			runNotice(store.AutomationNotifyOnSuccess, store.AutomationRunResultSucceeded, store.AutomationRunStarted, 0),
			runNotice(store.AutomationNotifyNever, store.AutomationRunResultFailed, store.AutomationRunDenied, 1),
		},
	}
	st.runs[0].Run.Error = "the repository bex-co/gone is not connected"
	mailer := &fakeMailer{}
	w := &AutomationEmailWorker{Store: st, Mailer: mailer, Identities: fakeIdentities{"alice": "alice@example.com"},
		DashboardBaseURL: "https://dashboard.bex.co", Clock: func() time.Time { return noticeFiredAt.Add(time.Hour) }}
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 {
		t.Fatalf("sent %d mails, want 2 (the 1st failure and the on_success run): %+v", len(mailer.sent), mailer.sent)
	}
	failure, success := mailer.sent[0], mailer.sent[1]
	if failure.to != "alice@example.com" || failure.subject != "[bex] Automation rt-fix-logic-bugs's run failed" ||
		!strings.Contains(failure.body, "could not start its agent session at 2026-11-12 08:00 UTC: the repository bex-co/gone is not connected") ||
		!strings.Contains(failure.body, "https://dashboard.bex.co/automations/aut-c185th5c2rvvnhbfiltg") {
		t.Fatalf("failure mail = %+v", failure)
	}
	if success.subject != "[bex] Automation rt-fix-logic-bugs finished its run" ||
		!strings.Contains(success.body, "https://dashboard.bex.co/agents/ags-c185th5c2rvvnhbfiltg") {
		t.Fatalf("success mail = %+v", success)
	}
}

func TestAutomationRunEmailNamesTheStreakAndTheSessionFailure(t *testing.T) {
	n := runNotice(store.AutomationNotifyOnFailure, store.AutomationRunResultFailed, store.AutomationRunStarted, 3)
	n.SessionFailureReason = "the turn ran out of time"
	subject, msg := automationRunEmail(n, resourcemeta.Config{})
	body := msg.Text()
	if subject != "[bex] Automation rt-fix-logic-bugs's run failed" ||
		!strings.Contains(body, "The agent session started by automation rt-fix-logic-bugs at 2026-11-12 08:00 UTC failed: the turn ran out of time") ||
		!strings.Contains(body, "It has failed 3 runs in a row") || msg.CTA != nil {
		t.Fatalf("mail = %q\n%s", subject, body)
	}
	denied := runNotice(store.AutomationNotifyAlways, store.AutomationRunResultFailed, store.AutomationRunDenied, 1)
	if _, msg := automationRunEmail(denied, resourcemeta.Config{}); !strings.Contains(msg.Text(), "its creator can no longer start agent sessions") {
		t.Fatalf("denied mail = %s", msg.Text())
	}
}

func TestAutomationEmailWorkerMailsEveryPlatformPause(t *testing.T) {
	never := automationFixture(store.AutomationNotifyNever, 3)
	never.PausedReason = store.AutomationPausedRepeatedFailures
	never.PausedAt = &noticeFiredAt
	st := &fakeNoticeStore{pauses: []store.AgentAutomation{never}}
	mailer := &fakeMailer{}
	w := &AutomationEmailWorker{Store: st, Mailer: mailer, Identities: fakeIdentities{"alice": "alice@example.com"},
		DashboardBaseURL: "https://dashboard.bex.co/", Clock: func() time.Time { return noticeFiredAt.Add(time.Minute) }}
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("sent %d pause mails, want 1 whatever the email setting", len(mailer.sent))
	}
	mail := mailer.sent[0]
	if mail.subject != "[bex] Automation rt-fix-logic-bugs was paused" ||
		!strings.Contains(mail.body, "three runs in a row could not start an agent session") ||
		!strings.Contains(mail.body, "https://dashboard.bex.co/automations/aut-c185th5c2rvvnhbfiltg") {
		t.Fatalf("pause mail = %+v", mail)
	}
	for reason, phrase := range map[string]string{
		store.AutomationPausedCreatorLostAccess: "a workspace admin can take it over",
		store.AutomationPausedTemplateInvalid:   "no longer validates",
	} {
		a := automationFixture(store.AutomationNotifyOnFailure, 0)
		a.PausedReason = reason
		if _, msg := automationPauseEmail(a, resourcemeta.Config{}); !strings.Contains(msg.Text(), phrase) {
			t.Errorf("%s pause mail = %s", reason, msg.Text())
		}
	}
}

// Without SMTP the worker still claims (and drops) notices, so nothing piles
// up to be mailed in bulk once mail is configured; a notice that ended more
// than a day before it was claimed is dropped too, and a creator with no
// address is skipped.
func TestAutomationEmailWorkerDropsWhatItCannotMailInTime(t *testing.T) {
	fresh := runNotice(store.AutomationNotifyAlways, store.AutomationRunResultSucceeded, store.AutomationRunStarted, 0)
	st := &fakeNoticeStore{runs: []store.AutomationRunNotice{fresh}}
	clock := func() time.Time { return noticeFiredAt.Add(time.Hour) }
	if err := (&AutomationEmailWorker{Store: st, Clock: clock}).RunOnce(context.Background()); err != nil || st.claims != 2 || len(st.runs) != 0 {
		t.Fatalf("no mailer: err=%v claims=%d left=%d, want the notice claimed and dropped", err, st.claims, len(st.runs))
	}

	stale := fresh
	stale.EndedAt = noticeFiredAt.Add(-25 * time.Hour)
	pausedLongAgo := automationFixture(store.AutomationNotifyOnFailure, 3)
	pausedLongAgo.PausedReason = store.AutomationPausedRepeatedFailures
	longAgo := noticeFiredAt.Add(-48 * time.Hour)
	pausedLongAgo.PausedAt = &longAgo
	noAddress := fresh
	noAddress.Automation.CreatedBy = "bob"
	st = &fakeNoticeStore{runs: []store.AutomationRunNotice{stale, noAddress}, pauses: []store.AgentAutomation{pausedLongAgo}}
	mailer := &fakeMailer{}
	w := &AutomationEmailWorker{Store: st, Mailer: mailer, Identities: fakeIdentities{"alice": "alice@example.com"}, Clock: clock}
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 0 {
		t.Fatalf("mailed a stale or unaddressable notice: %+v", mailer.sent)
	}
	st.err = errors.New("db down")
	if err := w.RunOnce(context.Background()); err == nil {
		t.Fatal("a failed claim was not reported")
	}
}
