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

package billing

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type workerCapture struct {
	states                []store.BillingLifecycle
	completed             []string
	failed                []time.Time
	notifications         []store.BillingNotification
	notificationsComplete int
	notificationsFailed   int
	purgeBefore           time.Time
	writeOffPurgeBefore   time.Time
	// completeFails is the workspace whose lifecycle completion errors;
	// notificationCompleteFails makes every notice's completion error.
	completeFails             string
	notificationCompleteFails bool
	completedWorkspaces       []string
}

func (s *workerCapture) ClaimDueBillingLifecycle(_ context.Context, _ time.Time, _ time.Duration) (store.BillingLifecycle, bool, error) {
	if len(s.states) == 0 {
		return store.BillingLifecycle{}, false, nil
	}
	state := s.states[0]
	s.states = s.states[1:]
	return state, true, nil
}
func (s *workerCapture) CompleteBillingLifecycleWork(_ context.Context, workspaceID string, _ int64, status string, _ time.Time) (store.BillingLifecycle, error) {
	if workspaceID == s.completeFails {
		return store.BillingLifecycle{}, errors.New("no rows in result set")
	}
	s.completed = append(s.completed, status)
	s.completedWorkspaces = append(s.completedWorkspaces, workspaceID)
	return store.BillingLifecycle{Status: status}, nil
}
func (s *workerCapture) FailBillingLifecycleWork(_ context.Context, _ string, _ int64, _ string, _ time.Time, retry time.Time) error {
	s.failed = append(s.failed, retry)
	return nil
}
func (s *workerCapture) ClaimBillingNotifications(_ context.Context, _ time.Time, _ time.Duration, _ int) ([]store.BillingNotification, error) {
	out := s.notifications
	s.notifications = nil
	return out, nil
}
func (s *workerCapture) CompleteBillingNotification(_ context.Context, _ string, _ int64, _ time.Time) error {
	if s.notificationCompleteFails {
		return errors.New("store unavailable")
	}
	s.notificationsComplete++
	return nil
}
func (s *workerCapture) FailBillingNotification(_ context.Context, _ string, _ int64, _ string, _ time.Time) error {
	s.notificationsFailed++
	return nil
}
func (s *workerCapture) PurgeStripeBillingEvents(_ context.Context, before time.Time) (int64, error) {
	s.purgeBefore = before
	return 3, nil
}
func (s *workerCapture) PurgeAgentModelWriteOffs(_ context.Context, before time.Time) (int64, error) {
	s.writeOffPurgeBefore = before
	return 0, nil
}

type enforcerCapture struct {
	enforced  []string
	recovered []string
	fail      string
}

func (e *enforcerCapture) Enforce(_ context.Context, state store.BillingLifecycle) error {
	e.enforced = append(e.enforced, state.WorkspaceID)
	if e.fail == state.WorkspaceID {
		return errors.New("temporary Kubernetes failure")
	}
	return nil
}
func (e *enforcerCapture) Recover(_ context.Context, state store.BillingLifecycle) error {
	e.recovered = append(e.recovered, state.WorkspaceID)
	return nil
}

type notifierCapture struct{ calls int }

func (n *notifierCapture) NotifyBilling(_ context.Context, _ store.BillingNotification) error {
	n.calls++
	if n.calls == 1 {
		return errors.New("temporary SMTP failure")
	}
	return nil
}

func TestWorkerRetriesIndependentlyAndCompletesRecoveryTarget(t *testing.T) {
	now := time.Date(2026, 7, 28, 2, 0, 0, 0, time.UTC)
	stateStore := &workerCapture{
		states: []store.BillingLifecycle{
			{WorkspaceID: "tea-enforce", Status: store.BillingEnforcing, AttemptCount: 1},
			{WorkspaceID: "tea-retry", Status: store.BillingEnforcing, AttemptCount: 2},
			{WorkspaceID: "tea-recover", Status: store.BillingRecovering, RecoveryTarget: store.BillingComped},
		},
		notifications: []store.BillingNotification{
			{WorkspaceID: "tea-first", TransitionVersion: 1},
			{WorkspaceID: "tea-second", TransitionVersion: 2},
		},
	}
	enforcer := &enforcerCapture{fail: "tea-retry"}
	notifier := &notifierCapture{}
	w := &Worker{Store: stateStore, Enforcer: enforcer, Notifier: notifier, Clock: func() time.Time { return now }}
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(stateStore.completed) != 2 || stateStore.completed[0] != store.BillingEnforced || stateStore.completed[1] != store.BillingComped {
		t.Fatalf("completed = %v", stateStore.completed)
	}
	if len(stateStore.failed) != 1 || !stateStore.failed[0].Equal(now.Add(2*time.Minute)) {
		t.Fatalf("retry = %v, want %s", stateStore.failed, now.Add(2*time.Minute))
	}
	if stateStore.notificationsFailed != 1 || stateStore.notificationsComplete != 1 {
		t.Fatalf("notification outcomes failed=%d complete=%d", stateStore.notificationsFailed, stateStore.notificationsComplete)
	}
	if !stateStore.writeOffPurgeBefore.Equal(now.Add(-store.AgentModelWriteOffRetention)) {
		t.Fatalf("write-off purge before = %s", stateStore.writeOffPurgeBefore)
	}
	if !stateStore.purgeBefore.Equal(now.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("purge before = %s", stateStore.purgeBefore)
	}
}

// One failing lifecycle row must not stop the next row, the notice drain or
// the purge, and its error names the workspace (w21/m336).
func TestWorkerLifecycleRowFailureDoesNotStarveTheTick(t *testing.T) {
	now := time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC)
	stateStore := &workerCapture{
		states: []store.BillingLifecycle{
			{WorkspaceID: "tea-poison", Status: store.BillingEnforcing},
			{WorkspaceID: "tea-next", Status: store.BillingEnforcing},
		},
		notifications: []store.BillingNotification{{WorkspaceID: "tea-notice", TransitionVersion: 1}},
		completeFails: "tea-poison",
	}
	notifier := &notifierCapture{calls: 1} // the first-call failure is spent
	w := &Worker{Store: stateStore, Enforcer: &enforcerCapture{}, Notifier: notifier, Clock: func() time.Time { return now }}
	err := w.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "tea-poison") || !strings.Contains(err.Error(), "no rows in result set") {
		t.Fatalf("RunOnce error = %v, want the poison row's error naming its workspace", err)
	}
	if !slices.Equal(stateStore.completedWorkspaces, []string{"tea-next"}) {
		t.Fatalf("completed workspaces = %v, want the row after the poison one", stateStore.completedWorkspaces)
	}
	if stateStore.notificationsComplete != 1 {
		t.Fatalf("notices completed = %d, want the notification drain to still run", stateStore.notificationsComplete)
	}
	if stateStore.purgeBefore.IsZero() {
		t.Fatal("the event purge did not run after the lifecycle drain failed")
	}
}

// A notice whose completion fails does not stop the notices after it or the
// purge.
func TestWorkerNotificationCompletionFailureDoesNotStarveThePurge(t *testing.T) {
	now := time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC)
	stateStore := &workerCapture{
		notifications: []store.BillingNotification{
			{WorkspaceID: "tea-first", TransitionVersion: 1},
			{WorkspaceID: "tea-second", TransitionVersion: 2},
		},
		notificationCompleteFails: true,
	}
	notifier := &notifierCapture{calls: 1}
	w := &Worker{Store: stateStore, Enforcer: &enforcerCapture{}, Notifier: notifier, Clock: func() time.Time { return now }}
	err := w.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "tea-first") || !strings.Contains(err.Error(), "tea-second") {
		t.Fatalf("RunOnce error = %v, want both notices' errors", err)
	}
	if delivered := notifier.calls - 1; delivered != 2 {
		t.Fatalf("notices delivered = %d, want both attempted after the first completion failed", delivered)
	}
	if stateStore.purgeBefore.IsZero() {
		t.Fatal("the event purge did not run after the notification drain failed")
	}
}
