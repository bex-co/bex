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
}

func (s *workerCapture) ClaimDueBillingLifecycle(_ context.Context, _ time.Time, _ time.Duration) (store.BillingLifecycle, bool, error) {
	if len(s.states) == 0 {
		return store.BillingLifecycle{}, false, nil
	}
	state := s.states[0]
	s.states = s.states[1:]
	return state, true, nil
}
func (s *workerCapture) CompleteBillingLifecycleWork(_ context.Context, _ string, _ int64, status string, _ time.Time) (store.BillingLifecycle, error) {
	s.completed = append(s.completed, status)
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
	if !stateStore.purgeBefore.Equal(now.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("purge before = %s", stateStore.purgeBefore)
	}
}
