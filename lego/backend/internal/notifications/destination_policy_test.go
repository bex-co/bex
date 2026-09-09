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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestEveryDeliveryEventHasDestinationPolicy (ADR087, w6/m137/t001): the
// closed event vocabulary is exhaustively classified — every event is either
// deliberately all-roles (its destination read is can_view, which every role
// holds) or returns a gated role set. An event missing from BOTH tables fails
// here, so a new family cannot silently fan out to every role.
func TestEveryDeliveryEventHasDestinationPolicy(t *testing.T) {
	allRoles := map[DeliveryEvent]bool{
		// Service/deploy/cron supervision: target read is can_view.
		DeliveryEventDeployStarted:    true,
		DeliveryEventDeploySucceeded:  true,
		DeliveryEventDeployFailed:     true,
		DeliveryEventServerFailed:     true,
		DeliveryEventServerAvailable:  true,
		DeliveryEventServiceSuspended: true,
		DeliveryEventServiceResumed:   true,
		DeliveryEventCronFailed:       true,
		// Managed-datastore supervision: the target read is the datastore's own
		// can_view (type postgres in model.fga inherits it from the workspace),
		// which every member role holds — same shape as the service family.
		DeliveryEventPostgresUnavailable:   true,
		DeliveryEventPostgresAvailable:     true,
		DeliveryEventKeyValueUnhealthy:     true,
		DeliveryEventKeyValueAvailable:     true,
		DeliveryEventPostgresBackupFailed:  true,
		DeliveryEventPostgresRestoreFailed: true,
		DeliveryEventPostgresUpgradeFailed: true,
	}
	for _, event := range orderedDeliveryEvents {
		eligible := destinationEligibleRoles(event)
		if allRoles[event] {
			if eligible != nil {
				t.Errorf("%s is in the all-roles table but destinationEligibleRoles gates it: %v", event, eligible)
			}
			continue
		}
		if eligible == nil {
			t.Errorf("%s is unclassified: add it to the all-roles table above (destination read is can_view) "+
				"or gate it in destinationEligibleRoles — an unclassified event fans out to every role", event)
		}
	}

	// The agent read gate is can_operate (agentsessions/service.go, pinned by
	// api/roleladder_test.go): viewer/billing are exactly the excluded roles.
	for _, event := range []DeliveryEvent{DeliveryEventAgentPRReady, DeliveryEventAgentFailed} {
		for role, want := range map[DeliveryWorkspaceRole]bool{
			DeliveryRoleViewer: false, DeliveryRoleBilling: false,
			DeliveryRoleContributor: true, DeliveryRoleDeveloper: true, DeliveryRoleAdmin: true,
		} {
			if got := roleEligible(role, destinationEligibleRoles(event)); got != want {
				t.Errorf("%s eligibility for %s = %v, want %v", event, role, got, want)
			}
		}
	}
	// A decision request asks for work that needs create access: a contributor
	// must not be asked to approve or submit it (ADR087 notifications table).
	if roleEligible(DeliveryRoleContributor, destinationEligibleRoles(DeliveryEventAgentNeedsDecision)) {
		t.Error("agent_needs_decision must not reach a contributor — approving/steering requires create access")
	}
}

// TestAgentPushPayloadsCarryNoRepositoryMetadata (w6/m137/t004): lock-screen
// text stays generic — an OS-rendered notification cannot be recalled or
// access-checked at display time, so repository names and PR numbers never
// enter title/body. Detail is fetched after opening the deep link.
func TestAgentPushPayloadsCarryNoRepositoryMetadata(t *testing.T) {
	sessions := []store.AgentSession{
		{ID: "ags-1", Repo: "org/secret-repo", Phase: "completed", PRURL: "https://github.com/org/secret-repo/pull/7", PRNumber: 7},
		{ID: "ags-2", Repo: "org/secret-repo", Phase: "completed"},
		{ID: "ags-3", Repo: "org/secret-repo", Phase: "failed", FailureReason: "prompt: build the thing"},
	}
	for _, s := range sessions {
		projected, ok := projectAgentSessionPush(s)
		if !ok {
			t.Fatalf("session %s did not project", s.ID)
		}
		text := projected.title + " " + projected.body
		for _, leak := range []string{"secret-repo", "org/", "#7", "pull/7", "build the thing"} {
			if strings.Contains(text, leak) {
				t.Errorf("session %s push text %q leaks %q", s.ID, text, leak)
			}
		}
		if projected.title == "" || projected.body == "" {
			t.Errorf("session %s push must still carry generic text, got %q/%q", s.ID, projected.title, projected.body)
		}
	}
}

// TestAgentPushExcludesViewerAndBillingRecipients (w6/m137/t002, proven red
// against the pre-fix fan-out): an agent event enqueues nothing for a
// viewer/billing recipient — no notification row, no badge contribution — no
// matter their device registrations or preferences, while a deploy event
// still reaches every role (the control).
func TestAgentPushExcludesViewerAndBillingRecipients(t *testing.T) {
	now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	queue := newFakePushWorkerStore()
	queue.watermarkAt = now.Add(-time.Hour)
	queue.destinations = []store.ActivePushSubscription{
		{TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios", Provider: "expo", Platform: "ios", Token: "t1", CreatedAt: now.Add(-time.Hour)},
		{TenantID: "tea-one", Subject: "dan", Role: "contributor", DeviceID: "dan-ios", Provider: "expo", Platform: "ios", Token: "t2", CreatedAt: now.Add(-time.Hour)},
		{TenantID: "tea-one", Subject: "bob", Role: "viewer", DeviceID: "bob-ios", Provider: "expo", Platform: "ios", Token: "t3", CreatedAt: now.Add(-time.Hour)},
		{TenantID: "tea-one", Subject: "carol", Role: "billing", DeviceID: "carol-ios", Provider: "expo", Platform: "ios", Token: "t4", CreatedAt: now.Add(-time.Hour)},
	}
	queue.agentSessions = []store.AgentSession{{
		ID: "ags-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-one", Repo: "org/app",
		Phase: "failed", UpdatedAt: at,
	}}
	worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()
	got := map[string]bool{}
	for _, n := range queue.notifications {
		got[n.Subject] = true
	}
	for subject, want := range map[string]bool{"alice": true, "dan": true, "bob": false, "carol": false} {
		if got[subject] != want {
			t.Errorf("agent push for %s = %v, want %v (destination access, not preference, decides)", subject, got[subject], want)
		}
	}
}

// TestPreSendEligibilityRecheck (w6/m137/t002): a delivery enqueued while the
// recipient could read its destination is re-checked at send time against the
// CURRENT membership role the claim query joins — a downgrade or membership
// removal drops it without a send; an unchanged member still receives it. (A
// membership-store outage fails the claim itself, so nothing sends — the
// never-widen property holds structurally.)
func TestPreSendEligibilityRecheck(t *testing.T) {
	run := func(t *testing.T, mutate func(*fakePushWorkerStore)) (*fakePushSender, *fakePushWorkerStore, error) {
		t.Helper()
		now := time.Date(2026, time.September, 7, 12, 0, 0, 0, time.UTC)
		queue := newFakePushWorkerStore()
		queue.watermarkAt = now.Add(-time.Hour)
		queue.destinations = []store.ActivePushSubscription{{
			TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
			Provider: "expo", Platform: "ios", Token: "tok", CreatedAt: now.Add(-time.Hour),
		}}
		queue.agentSessions = []store.AgentSession{{
			ID: "ags-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-one", Repo: "org/app",
			Phase: "failed", UpdatedAt: now.Add(-time.Minute),
		}}
		// First pass enqueues only (no sender wired).
		worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		// The access change happens between enqueue and delivery.
		mutate(queue)
		sender := &fakePushSender{}
		worker.Sender = sender
		err := worker.RunOnce(context.Background())
		return sender, queue, err
	}

	t.Run("downgraded to viewer: dropped without a send", func(t *testing.T) {
		sender, _, err := run(t, func(q *fakePushWorkerStore) { q.members["tea-one\x00alice"] = "viewer" })
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if n := len(sender.snapshot()); n != 0 {
			t.Fatalf("downgraded recipient still received %d sends", n)
		}
	})

	t.Run("membership removed: dropped without a send", func(t *testing.T) {
		sender, _, err := run(t, func(q *fakePushWorkerStore) { q.members["tea-one\x00alice"] = "" })
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if n := len(sender.snapshot()); n != 0 {
			t.Fatalf("removed member still received %d sends", n)
		}
	})

	t.Run("unchanged member: delivered", func(t *testing.T) {
		sender, _, err := run(t, func(*fakePushWorkerStore) {})
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if n := len(sender.snapshot()); n != 1 {
			t.Fatalf("eligible recipient got %d sends, want 1", n)
		}
	})
}

// grantSetChecker models the workspace relations: can_view always holds,
// everything else answers from the set.
type grantSetChecker map[string]bool

func (c grantSetChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return c[relation], nil
}

// erroringOperateChecker answers can_view and fails on everything else.
type erroringOperateChecker struct{}

func (erroringOperateChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return false, context.DeadlineExceeded
}

// TestInboxFollowsDestinationAccess (w6/m137/t003): historic agent-session
// inbox rows and their badge contribution vanish for a caller without the
// session-read relation — including after a downgrade, and fail-closed when
// the relation cannot be checked — while ungated service rows survive.
func TestInboxFollowsDestinationAccess(t *testing.T) {
	seed := func() *fakeStore {
		st := newFakeStore()
		st.push[[2]string{"tea-a", "alice"}] = []store.PushNotification{
			{TenantID: "tea-a", Subject: "alice", EventID: "evt-service00000000000", EventType: "deploy_failed", ResourceKind: "service", ResourceID: "srv-1"},
			{TenantID: "tea-a", Subject: "alice", EventID: "evt-agent0000000000000", EventType: "agent_failed", ResourceKind: resourceKindAgentSession, ResourceID: "ags-1"},
		}
		return st
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})

	t.Run("operator sees both, viewer only the service row", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			grants grantSetChecker
			want   int
		}{
			{"operator", grantSetChecker{core.RelCanOperate: true}, 2},
			{"viewer", grantSetChecker{}, 1},
		} {
			svc := newTestService(seed(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
			svc.Base.Authz = tc.grants
			rows, err := svc.ListNotificationInbox(ctx, 50)
			if err != nil {
				t.Fatalf("%s list: %v", tc.name, err)
			}
			if len(rows) != tc.want {
				t.Fatalf("%s sees %d rows, want %d: %+v", tc.name, len(rows), tc.want, rows)
			}
			count, err := svc.UnreadPushNotificationCount(ctx)
			if err != nil {
				t.Fatalf("%s count: %v", tc.name, err)
			}
			if count != tc.want {
				t.Fatalf("%s badge = %d, want %d — the badge must agree with the list", tc.name, count, tc.want)
			}
		}
	})

	t.Run("tiers are independent: operate does not unlock create-gated rows", func(t *testing.T) {
		st := seed()
		st.push[[2]string{"tea-a", "alice"}] = append(st.push[[2]string{"tea-a", "alice"}], store.PushNotification{
			TenantID: "tea-a", Subject: "alice", EventID: "evt-decision0000000000",
			EventType: string(DeliveryEventAgentNeedsDecision), ResourceKind: resourceKindAgentSession, ResourceID: "ags-2",
		})
		svc := newTestService(st, fakeWorkspace{"alice": "tea-a"}, nil, nil)
		svc.Base.Authz = grantSetChecker{core.RelCanOperate: true} // contributor: no can_create
		rows, err := svc.ListNotificationInbox(ctx, 50)
		if err != nil {
			t.Fatalf("contributor list: %v", err)
		}
		for _, row := range rows {
			if row.Event == string(DeliveryEventAgentNeedsDecision) {
				t.Fatal("a contributor must not see a decision request their create access could not act on")
			}
		}
		if len(rows) != 2 {
			t.Fatalf("contributor sees %d rows, want service + agent_failed", len(rows))
		}
	})

	t.Run("checker outage suppresses gated rows (fail closed)", func(t *testing.T) {
		svc := newTestService(seed(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
		svc.Base.Authz = erroringOperateChecker{}
		rows, err := svc.ListNotificationInbox(ctx, 50)
		if err != nil {
			t.Fatalf("list under outage: %v", err)
		}
		for _, row := range rows {
			if row.ResourceKind == resourceKindAgentSession {
				t.Fatal("an unanswerable relation check disclosed a gated row")
			}
		}
	})
}
