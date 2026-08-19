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
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	pushtransport "github.com/bex-co/bex/lego/backend/internal/notifications/push"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakePushQueueDelivery struct {
	notification                 store.PushNotification
	deviceID                     string
	claimedUntil                 time.Time
	accepted                     bool
	attemptCount                 int
	acceptedAt                   *time.Time
	receiptDueAt                 *time.Time
	nextAttemptAt                time.Time
	ticketID                     string
	delivered, failed, ambiguous bool
	lastCode                     string
}

type fakePushWorkerStore struct {
	mu sync.Mutex

	destinations  []store.ActivePushSubscription
	events        []store.WebhookEventRow
	agentSessions []store.AgentSession
	serviceIDs    map[string]string
	factStatuses  map[string]string
	watermarkAt   time.Time
	watermarkKey  string

	notifications map[string]store.PushNotification
	deliveries    map[string]*fakePushQueueDelivery
	enqueueCalls  int
}

func newFakePushWorkerStore() *fakePushWorkerStore {
	return &fakePushWorkerStore{
		serviceIDs:    map[string]string{},
		factStatuses:  map[string]string{},
		notifications: map[string]store.PushNotification{},
		deliveries:    map[string]*fakePushQueueDelivery{},
	}
}

func (f *fakePushWorkerStore) ListActivePushSubscriptions(context.Context) ([]store.ActivePushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.ActivePushSubscription(nil), f.destinations...), nil
}

func (f *fakePushWorkerStore) EnsurePushWatermark(_ context.Context, at time.Time) (time.Time, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watermarkAt.IsZero() {
		f.watermarkAt = at
	}
	return f.watermarkAt, f.watermarkKey, nil
}

func (f *fakePushWorkerStore) ListWebhookEvents(
	_ context.Context, afterAt time.Time, afterKey string, until time.Time,
	_ []string, tenants []string, limit int,
) ([]store.WebhookEventRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tenantSet := map[string]bool{}
	for _, tenant := range tenants {
		tenantSet[tenant] = true
	}
	var rows []store.WebhookEventRow
	for _, row := range f.events {
		after := row.CursorAt.After(afterAt) || row.CursorAt.Equal(afterAt) && row.Key > afterKey
		if after && !row.CursorAt.After(until) && tenantSet[row.TenantID] {
			// Mirror the real query's joins: the feed carries the app id, and a
			// fact row carries its own status, so the dispatcher needs no
			// per-row lookups.
			if row.AppID == "" {
				row.AppID = f.serviceIDs[row.TenantID+"\x00"+row.ServiceName]
			}
			if row.Source == store.EventSourceFact && row.Status == "" {
				row.Status = f.factStatuses[row.Key]
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CursorAt.Equal(rows[j].CursorAt) {
			return rows[i].Key < rows[j].Key
		}
		return rows[i].CursorAt.Before(rows[j].CursorAt)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (f *fakePushWorkerStore) ListTerminalAgentSessionsForPush(_ context.Context, since time.Time) ([]store.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.AgentSession
	for _, session := range f.agentSessions {
		if (session.Phase == "completed" || session.Phase == "failed") && !session.UpdatedAt.Before(since) {
			out = append(out, session)
		}
	}
	return out, nil
}

func pushNotificationKey(n store.PushNotification) string {
	return n.TenantID + "\x00" + n.Subject + "\x00" + n.SourceEventKey
}

func pushDeliveryKey(tenantID, subject, deviceID, sourceKey string) string {
	return tenantID + "\x00" + subject + "\x00" + deviceID + "\x00" + sourceKey
}

func (f *fakePushWorkerStore) EnqueuePushNotifications(
	_ context.Context, items []store.PushNotificationBatchItem, at time.Time, key string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueueCalls++
	for _, item := range items {
		notificationKey := pushNotificationKey(item.Notification)
		if _, exists := f.notifications[notificationKey]; !exists {
			f.notifications[notificationKey] = item.Notification
		}
		for _, deviceID := range item.DeviceIDs {
			deliveryKey := pushDeliveryKey(
				item.Notification.TenantID, item.Notification.Subject, deviceID, item.Notification.SourceEventKey,
			)
			if _, exists := f.deliveries[deliveryKey]; !exists {
				f.deliveries[deliveryKey] = &fakePushQueueDelivery{notification: item.Notification, deviceID: deviceID}
			}
		}
	}
	if at.After(f.watermarkAt) || at.Equal(f.watermarkAt) && key > f.watermarkKey {
		f.watermarkAt, f.watermarkKey = at, key
	}
	return nil
}

func (f *fakePushWorkerStore) ClaimDuePushDeliveries(
	_ context.Context, now, leaseUntil time.Time, limit int,
) ([]store.DuePushDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.deliveries))
	for key := range f.deliveries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []store.DuePushDelivery
	for _, key := range keys {
		delivery := f.deliveries[key]
		if delivery.accepted || delivery.failed || delivery.notification.DeliverAt.After(now) ||
			delivery.nextAttemptAt.After(now) || delivery.claimedUntil.After(now) {
			continue
		}
		var destination store.ActivePushSubscription
		for _, candidate := range f.destinations {
			if candidate.TenantID == delivery.notification.TenantID &&
				candidate.Subject == delivery.notification.Subject && candidate.DeviceID == delivery.deviceID {
				destination = candidate
				break
			}
		}
		if destination.Token == "" {
			continue
		}
		delivery.claimedUntil = leaseUntil
		out = append(out, store.DuePushDelivery{
			PushNotification: delivery.notification,
			DeviceID:         delivery.deviceID, Provider: destination.Provider,
			Platform: destination.Platform, Token: destination.Token,
			P256dh: destination.P256dh, Auth: destination.Auth,
			ClaimedUntil: leaseUntil, AttemptCount: delivery.attemptCount,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakePushWorkerStore) AcceptPushDelivery(
	_ context.Context, due store.DuePushDelivery, ticketID string, at, receiptDue time.Time,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delivery := f.deliveries[pushDeliveryKey(due.TenantID, due.Subject, due.DeviceID, due.SourceEventKey)]
	if delivery == nil || delivery.accepted || !delivery.claimedUntil.Equal(due.ClaimedUntil) || ticketID == "" {
		return false, nil
	}
	delivery.accepted = true
	delivery.attemptCount++
	delivery.acceptedAt = &at
	delivery.receiptDueAt = &receiptDue
	delivery.ticketID = ticketID
	delivery.claimedUntil = time.Time{}
	return true, nil
}

func (f *fakePushWorkerStore) CompletePushDelivery(_ context.Context, due store.DuePushDelivery, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delivery := f.deliveries[pushDeliveryKey(due.TenantID, due.Subject, due.DeviceID, due.SourceEventKey)]
	if delivery == nil || delivery.accepted || delivery.failed || delivery.delivered || !delivery.claimedUntil.Equal(due.ClaimedUntil) {
		return false, nil
	}
	delivery.delivered = true
	delivery.attemptCount++
	delivery.claimedUntil = time.Time{}
	_ = at
	return true, nil
}

func (f *fakePushWorkerStore) RecordPushSendFailure(_ context.Context, due store.DuePushDelivery, code string, _ time.Time, next time.Time, terminal bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.deliveries[pushDeliveryKey(due.TenantID, due.Subject, due.DeviceID, due.SourceEventKey)]
	if d == nil {
		return false, nil
	}
	d.attemptCount++
	d.lastCode = code
	d.failed = terminal
	d.nextAttemptAt = next
	d.claimedUntil = time.Time{}
	return true, nil
}
func (f *fakePushWorkerStore) ClaimDuePushReceipts(_ context.Context, now, lease time.Time, limit int) ([]store.DuePushDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.DuePushDelivery
	for _, d := range f.deliveries {
		if !d.accepted || d.delivered || d.failed || d.ambiguous || d.receiptDueAt == nil || d.receiptDueAt.After(now) || d.claimedUntil.After(now) {
			continue
		}
		d.claimedUntil = lease
		var dest store.ActivePushSubscription
		for _, x := range f.destinations {
			if x.DeviceID == d.deviceID && x.Subject == d.notification.Subject {
				dest = x
				break
			}
		}
		out = append(out, store.DuePushDelivery{PushNotification: d.notification, DeviceID: d.deviceID, Provider: dest.Provider, Token: dest.Token, ClaimedUntil: lease, AttemptCount: d.attemptCount, AcceptedAt: d.acceptedAt, ReceiptDueAt: d.receiptDueAt, ProviderTicketID: d.ticketID})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}
func (f *fakePushWorkerStore) RecordPushReceipt(_ context.Context, due store.DuePushDelivery, code string, _ time.Time, next time.Time, delivered, failed, ambiguous bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.deliveries[pushDeliveryKey(due.TenantID, due.Subject, due.DeviceID, due.SourceEventKey)]
	if d == nil {
		return false, nil
	}
	d.lastCode = code
	d.delivered = delivered
	d.failed = failed
	d.ambiguous = ambiguous
	d.receiptDueAt = &next
	d.claimedUntil = time.Time{}
	return true, nil
}
func (f *fakePushWorkerStore) RevokeExactPushSubscription(_ context.Context, due store.DuePushDelivery) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.destinations {
		if f.destinations[i].DeviceID == due.DeviceID && f.destinations[i].Token == due.Token {
			f.destinations = append(f.destinations[:i], f.destinations[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakePushWorkerStore) PushDeliveryStats(context.Context) (store.PushQueueStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var s store.PushQueueStats
	for _, d := range f.deliveries {
		if d.delivered || d.failed || d.ambiguous {
			s.Terminal++
		} else if d.accepted {
			s.ReceiptPending++
		} else {
			s.Pending++
		}
	}
	return s, nil
}
func (f *fakePushWorkerStore) SweepPushRetention(context.Context, time.Time, time.Time) (store.PushSweepResult, error) {
	return store.PushSweepResult{}, nil
}

func (f *fakePushWorkerStore) ReleasePushDelivery(
	_ context.Context, due store.DuePushDelivery,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delivery := f.deliveries[pushDeliveryKey(due.TenantID, due.Subject, due.DeviceID, due.SourceEventKey)]
	if delivery == nil || delivery.accepted || !delivery.claimedUntil.Equal(due.ClaimedUntil) {
		return false, nil
	}
	delivery.claimedUntil = time.Time{}
	return true, nil
}

type fakePushSender struct {
	mu       sync.Mutex
	messages []PushSendRequest
	fail     bool
	err      error
	support  map[string]bool
}

func (f *fakePushSender) Supports(provider string) bool {
	if f.support == nil {
		return true
	}
	return f.support[provider]
}

func (f *fakePushSender) Send(_ context.Context, request PushSendRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, request)
	if f.err != nil {
		return "", f.err
	}
	if f.fail {
		return "", fmt.Errorf("provider reflected %s", request.Token)
	}
	return fmt.Sprintf("ticket-%d", len(f.messages)), nil
}

type fakePushReceiptChecker struct {
	mu       sync.Mutex
	receipts map[string]pushtransport.Receipt
	err      error
	calls    [][]string
}

func (f *fakePushReceiptChecker) CheckReceipts(_ context.Context, ticketIDs []string) (map[string]pushtransport.Receipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), ticketIDs...))
	return f.receipts, f.err
}

func seedAcceptedPushDelivery(queue *fakePushWorkerStore, now time.Time) *fakePushQueueDelivery {
	n := validWorkerNotification(now.Add(-time.Hour))
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: n.TenantID, Subject: n.Subject, DeviceID: "ios", Provider: "expo",
		Platform: "ios", Token: "exact-secret-token",
	}}
	d := &fakePushQueueDelivery{
		notification: n, deviceID: "ios", accepted: true, attemptCount: 1,
		acceptedAt: timePointer(now.Add(-time.Hour)), receiptDueAt: timePointer(now.Add(-time.Minute)),
		ticketID: "ticket-one",
	}
	queue.notifications[pushNotificationKey(n)] = n
	queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, d.deviceID, n.SourceEventKey)] = d
	return d
}

func validWorkerNotification(at time.Time) store.PushNotification {
	return store.PushNotification{
		TenantID: "tea-one", Subject: "alice", SourceEventKey: "dep-receipt:ended",
		EventID: "evt-c185th5c2rvvnhbfiltg", EventType: "deploy_failed",
		Title: "Deploy failed", Body: "api deploy failed.", Urgency: "important",
		ResourceKind: "service", ResourceID: "srv-c185th5c2rvvnhbfiltg",
		DeepLink: "/services/srv-c185th5c2rvvnhbfiltg", OccurredAt: at, DeliverAt: at,
	}
}

func timePointer(v time.Time) *time.Time { return &v }

func (f *fakePushSender) snapshot() []PushSendRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]PushSendRequest(nil), f.messages...)
}

func TestProjectPushEventVocabulary(t *testing.T) {
	base := store.WebhookEventRow{ServiceName: "api"}
	tests := []struct {
		name       string
		row        store.WebhookEventRow
		factStatus string
		want       string
		ok         bool
	}{
		{"deploy started", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted}, "", "deploy_started", true},
		{"deploy live", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployLive}, "", "deploy_succeeded", true},
		{"build failed", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployBuildFailed}, "", "deploy_failed", true},
		{"predeploy failed", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployPreDeployFailed}, "", "deploy_failed", true},
		{"update failed", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed}, "", "deploy_failed", true},
		{"canceled ignored", store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, Status: store.DeployCanceled}, "", "", false},
		{"server failed", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactServerFailed)}, "", "server_failed", true},
		{"server available", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactServerAvailable)}, "", "server_available", true},
		{"service suspended", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactServiceSuspended)}, "", "service_suspended", true},
		{"service resumed", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactServiceResumed)}, "", "service_resumed", true},
		{"failed job", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactJobRunEnded)}, store.EventStatusFailed, "cron_failed", true},
		{"successful job ignored", store.WebhookEventRow{Source: store.EventSourceFact, FactType: string(store.EventFactJobRunEnded)}, store.EventStatusSucceeded, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := test.row
			row.ServiceName = base.ServiceName
			got, ok := projectPushEvent(row, "srv-c185th5c2rvvnhbfiltg", test.factStatus)
			if ok != test.ok || got.event != test.want {
				t.Fatalf("projectPushEvent() = (%+v,%v), want event=%q ok=%v", got, ok, test.want, test.ok)
			}
		})
	}
}

// The lifecycle facts carry deliberate urgencies: recovery closes a Critical
// page at Important; suspend/resume are Routine state changes.
func TestProjectPushEventLifecycleUrgencies(t *testing.T) {
	tests := []struct {
		factType string
		urgency  string
	}{
		{string(store.EventFactServerFailed), string(DeliveryUrgencyCritical)},
		{string(store.EventFactServerAvailable), string(DeliveryUrgencyImportant)},
		{string(store.EventFactServiceSuspended), string(DeliveryUrgencyRoutine)},
		{string(store.EventFactServiceResumed), string(DeliveryUrgencyRoutine)},
	}
	for _, test := range tests {
		row := store.WebhookEventRow{Source: store.EventSourceFact, FactType: test.factType, ServiceName: "api"}
		got, ok := projectPushEvent(row, "srv-c185th5c2rvvnhbfiltg", "")
		if !ok || got.urgency != test.urgency {
			t.Fatalf("projectPushEvent(%s) = (%+v,%v), want urgency=%q", test.factType, got, ok, test.urgency)
		}
	}
}

// The default policy's event set grows only by deliberate decision, never as a
// side effect of new events joining the vocabulary: the m78 lifecycle events
// (server_available/service_suspended/service_resumed) are additive opt-in and
// must stay out; the failure defaults plus w11/m6's agent terminal events are
// the chosen set.
func TestDefaultPushSettingsEventsUnchanged(t *testing.T) {
	want := []DeliveryEvent{
		DeliveryEventDeployFailed, DeliveryEventServerFailed, DeliveryEventCronFailed,
		DeliveryEventAgentPRReady, DeliveryEventAgentFailed,
	}
	if !slices.Equal(defaultPushSettings.Events, want) {
		t.Fatalf("defaultPushSettings.Events = %v, want %v", defaultPushSettings.Events, want)
	}
}

func TestPushWorkerReplayRestartAndConcurrentWorkersAreIdempotent(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 10, 0, time.UTC)
	eventAt := now.Add(-5 * time.Second)
	serviceID := "srv-c185th5c2rvvnhbfiltg"
	queue := newFakePushWorkerStore()
	queue.watermarkAt = eventAt.Add(-time.Second)
	queue.serviceIDs["tea-one\x00api"] = serviceID
	queue.destinations = []store.ActivePushSubscription{
		{TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios", Provider: "expo", Platform: "ios", Token: "token-alice-ios", CreatedAt: eventAt.Add(-time.Hour)},
		{TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-android", Provider: "expo", Platform: "android", Token: "token-alice-android", CreatedAt: eventAt.Add(-time.Hour)},
		{TenantID: "tea-one", Subject: "bob", Role: "viewer", DeviceID: "bob-ios", Provider: "expo", Platform: "ios", Token: "token-bob-ios", CreatedAt: eventAt.Add(-time.Hour)},
	}
	queue.events = []store.WebhookEventRow{{
		CursorAt: eventAt, Key: "dep-one:ended", At: eventAt,
		TenantID: "tea-one", ServiceName: "api", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed,
	}}
	sender := &fakePushSender{}
	clock := func() time.Time { return now }

	workers := []*PushWorker{
		{Store: queue, Sender: sender, Clock: clock},
		{Store: queue, Sender: sender, Clock: clock},
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	var errs [2]error
	for index, worker := range workers {
		wait.Add(1)
		go func(index int, worker *PushWorker) {
			defer wait.Done()
			<-start
			errs[index] = worker.RunOnce(context.Background())
		}(index, worker)
	}
	close(start)
	wait.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("concurrent workers errors = %v", errs)
	}

	queue.mu.Lock()
	logicalCount, deliveryCount := len(queue.notifications), len(queue.deliveries)
	var logicalIDs []string
	for _, notification := range queue.notifications {
		logicalIDs = append(logicalIDs, notification.EventID)
		if notification.DeepLink != "/services/"+serviceID || notification.ResourceID != serviceID {
			t.Errorf("unsafe resource projection = %+v", notification)
		}
	}
	queue.mu.Unlock()
	if logicalCount != 2 || deliveryCount != 3 {
		t.Fatalf("logical=%d deliveries=%d, want one per subject and device", logicalCount, deliveryCount)
	}
	if len(logicalIDs) != 2 || logicalIDs[0] == logicalIDs[1] {
		t.Fatalf("subject-scoped notification ids = %v", logicalIDs)
	}
	messages := sender.snapshot()
	if len(messages) != 3 {
		t.Fatalf("provider sends = %d, want one per device", len(messages))
	}
	for _, message := range messages {
		if message.Data.Schema != "bex.notification.v1" || message.Data.Event != "deploy_failed" ||
			message.Data.Route != "/services/"+serviceID || message.Data.NotificationID == "" {
			t.Errorf("provider envelope = %+v", message.Data)
		}
	}

	// A fresh process reloads the durable watermark and cannot recreate or
	// resend the accepted rows.
	restarted := &PushWorker{Store: queue, Sender: sender, Clock: clock}
	if err := restarted.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(sender.snapshot()); got != 3 {
		t.Fatalf("restart sent %d messages, want still 3", got)
	}
}

func TestPushWorkerFailureIsRedactedAndDeliveryRemainsRetryable(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 10, 0, time.UTC)
	eventAt := now.Add(-5 * time.Second)
	queue := newFakePushWorkerStore()
	queue.watermarkAt = eventAt.Add(-time.Second)
	queue.serviceIDs["tea-one\x00api"] = "srv-c185th5c2rvvnhbfiltg"
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "secret-device-token", CreatedAt: eventAt.Add(-time.Hour),
	}}
	queue.events = []store.WebhookEventRow{{
		CursorAt: eventAt, Key: "fact:server-failed-one", At: eventAt,
		TenantID: "tea-one", ServiceName: "api", Source: store.EventSourceFact,
		FactType: string(store.EventFactServerFailed),
	}}
	failing := &fakePushSender{fail: true}
	worker := &PushWorker{Store: queue, Sender: failing, Clock: func() time.Time { return now }}
	err := worker.RunOnce(context.Background())
	if !errors.Is(err, ErrPushSenderFailed) || strings.Contains(err.Error(), "secret-device-token") {
		t.Fatalf("failure = %q, want redacted ErrPushSenderFailed", err)
	}
	queue.mu.Lock()
	for _, delivery := range queue.deliveries {
		if delivery.accepted || !delivery.claimedUntil.IsZero() {
			t.Fatalf("failed delivery not retryable: %+v", delivery)
		}
	}
	queue.mu.Unlock()

	// A restarted worker claims the same durable row and accepts it exactly once.
	success := &fakePushSender{}
	retryAt := now.Add(pushRetryDelay(1))
	restarted := &PushWorker{Store: queue, Sender: success, Clock: func() time.Time { return retryAt }}
	if err := restarted.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(success.snapshot()) != 1 {
		t.Fatalf("retry sends = %d, want 1", len(success.snapshot()))
	}
}

func TestPushWorkerClassifiesSendFailuresAndBoundsRetries(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		err          error
		attempts     int
		wantCode     string
		wantTerminal bool
		wantPruned   bool
		wantDelay    time.Duration
	}{
		{name: "transient", err: &pushtransport.TransientError{Operation: "send", Detail: "network"}, wantCode: "transient", wantDelay: 30 * time.Second},
		{name: "rate limit bounds retry-after", err: &pushtransport.RateLimitedError{Operation: "send", RetryAfter: 12 * time.Hour}, wantCode: "rate_limited", wantDelay: time.Hour},
		{name: "invalid token", err: &pushtransport.InvalidTokenError{Code: "DeviceNotRegistered"}, wantCode: "invalid_token", wantTerminal: true, wantPruned: true},
		{name: "payload", err: &pushtransport.PayloadError{Field: "title", Reason: "too_large"}, wantCode: "payload", wantTerminal: true},
		{name: "permanent", err: &pushtransport.PermanentError{Operation: "send", Code: "denied"}, wantCode: "permanent", wantTerminal: true},
		{name: "attempt budget", err: &pushtransport.TransientError{Operation: "send", Detail: "network"}, attempts: pushMaxAttempts - 1, wantCode: "transient", wantTerminal: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue := newFakePushWorkerStore()
			n := validWorkerNotification(now.Add(-time.Hour))
			queue.destinations = []store.ActivePushSubscription{{
				TenantID: n.TenantID, Subject: n.Subject, DeviceID: "ios", Provider: "expo", Token: "secret-token",
			}}
			d := &fakePushQueueDelivery{notification: n, deviceID: "ios", attemptCount: test.attempts}
			queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "ios", n.SourceEventKey)] = d
			sender := &fakePushSender{err: test.err}
			worker := &PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}
			err := worker.send(context.Background())
			if !errors.Is(err, ErrPushSenderFailed) {
				t.Fatalf("send error = %v", err)
			}
			if d.lastCode != test.wantCode || d.failed != test.wantTerminal {
				t.Fatalf("delivery code=%q failed=%v, want %q/%v", d.lastCode, d.failed, test.wantCode, test.wantTerminal)
			}
			if got := len(queue.destinations) == 0; got != test.wantPruned {
				t.Fatalf("pruned=%v, want %v", got, test.wantPruned)
			}
			if !test.wantTerminal && !d.nextAttemptAt.Equal(now.Add(test.wantDelay)) {
				t.Fatalf("next attempt = %s, want %s", d.nextAttemptAt, now.Add(test.wantDelay))
			}
		})
	}
}

func TestPushWorkerLeaseRecoveryAndAcceptedRowsAreNeverResent(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	queue := newFakePushWorkerStore()
	n := validWorkerNotification(now.Add(-time.Hour))
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: n.TenantID, Subject: n.Subject, DeviceID: "ios", Provider: "expo", Token: "token",
	}}
	d := &fakePushQueueDelivery{notification: n, deviceID: "ios", claimedUntil: now.Add(-time.Second)}
	queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "ios", n.SourceEventKey)] = d
	sender := &fakePushSender{}
	worker := &PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}
	if err := worker.send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.snapshot()) != 1 || !d.accepted {
		t.Fatalf("recovered send count=%d accepted=%v", len(sender.snapshot()), d.accepted)
	}
	if err := worker.send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.snapshot()) != 1 {
		t.Fatalf("accepted row resent: %d sends", len(sender.snapshot()))
	}
}

func TestPushWorkerReceiptsCloseWithoutActiveDestinationAndNeverResend(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	queue := newFakePushWorkerStore()
	d := seedAcceptedPushDelivery(queue, now)
	queue.destinations = nil // Token was revoked after provider acceptance.
	receipts := &fakePushReceiptChecker{receipts: map[string]pushtransport.Receipt{
		"ticket-one": {ID: "ticket-one"},
	}}
	sender := &fakePushSender{}
	worker := &PushWorker{Store: queue, Sender: sender, Receipts: receipts, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !d.delivered || d.failed || d.ambiguous {
		t.Fatalf("receipt terminal state = delivered:%v failed:%v ambiguous:%v", d.delivered, d.failed, d.ambiguous)
	}
	if len(receipts.calls) != 1 || len(sender.snapshot()) != 0 {
		t.Fatalf("receipt calls=%d sends=%d", len(receipts.calls), len(sender.snapshot()))
	}
}

func TestPushWorkerReceiptOutcomes(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		receipts      map[string]pushtransport.Receipt
		checkErr      error
		acceptedAge   time.Duration
		wantCode      string
		wantDelivered bool
		wantFailed    bool
		wantAmbiguous bool
		wantPruned    bool
	}{
		{name: "missing remains pending", receipts: map[string]pushtransport.Receipt{}, acceptedAge: time.Hour, wantCode: "receipt_pending"},
		{name: "transient recheck", checkErr: &pushtransport.TransientError{Operation: "receipt", Detail: "network"}, acceptedAge: time.Hour, wantCode: "receipt_transient"},
		{name: "missing becomes ambiguous", receipts: map[string]pushtransport.Receipt{}, acceptedAge: pushReceiptWindow, wantCode: "receipt_pending", wantAmbiguous: true},
		{name: "delivered", receipts: map[string]pushtransport.Receipt{"ticket-one": {ID: "ticket-one"}}, acceptedAge: time.Hour, wantDelivered: true},
		{name: "invalid token prunes exact destination", receipts: map[string]pushtransport.Receipt{"ticket-one": {ID: "ticket-one", Err: &pushtransport.InvalidTokenError{Code: "DeviceNotRegistered"}}}, acceptedAge: time.Hour, wantCode: "invalid_token", wantFailed: true, wantPruned: true},
		{name: "definitive failure is not ambiguous", receipts: map[string]pushtransport.Receipt{"ticket-one": {ID: "ticket-one", Err: &pushtransport.PermanentError{Operation: "receipt", Code: "rejected"}}}, acceptedAge: pushReceiptWindow, wantCode: "permanent", wantFailed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue := newFakePushWorkerStore()
			d := seedAcceptedPushDelivery(queue, now)
			d.acceptedAt = timePointer(now.Add(-test.acceptedAge))
			checker := &fakePushReceiptChecker{receipts: test.receipts, err: test.checkErr}
			worker := &PushWorker{Store: queue, Receipts: checker, Clock: func() time.Time { return now }}
			if err := worker.checkReceipts(context.Background()); err != nil {
				t.Fatal(err)
			}
			if d.lastCode != test.wantCode || d.delivered != test.wantDelivered || d.failed != test.wantFailed || d.ambiguous != test.wantAmbiguous {
				t.Fatalf("receipt state code=%q delivered=%v failed=%v ambiguous=%v", d.lastCode, d.delivered, d.failed, d.ambiguous)
			}
			if got := len(queue.destinations) == 0; got != test.wantPruned {
				t.Fatalf("pruned=%v, want %v", got, test.wantPruned)
			}
		})
	}
}

func TestPushWorkerNilSenderQueuesAndRunCancels(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 10, 0, time.UTC)
	eventAt := now.Add(-5 * time.Second)
	queue := newFakePushWorkerStore()
	queue.watermarkAt = eventAt.Add(-time.Second)
	queue.serviceIDs["tea-one\x00api"] = "srv-c185th5c2rvvnhbfiltg"
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "token", CreatedAt: eventAt.Add(-time.Hour),
	}}
	queue.events = []store.WebhookEventRow{{
		CursorAt: eventAt, Key: "dep-one:ended", At: eventAt,
		TenantID: "tea-one", ServiceName: "api", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed,
	}}
	worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue.mu.Lock()
	if len(queue.notifications) != 1 || len(queue.deliveries) != 1 {
		t.Fatalf("nil sender queue = logical %d deliveries %d", len(queue.notifications), len(queue.deliveries))
	}
	for _, delivery := range queue.deliveries {
		if delivery.accepted || !delivery.claimedUntil.IsZero() {
			t.Fatalf("nil sender touched pending delivery: %+v", delivery)
		}
	}
	queue.mu.Unlock()

	ticks := make(chan time.Time)
	worker.Tick = ticks
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
	if got := (&PushWorker{PollInterval: time.Nanosecond}).pollInterval(); got != pushMinimumPollInterval {
		t.Fatalf("minimum poll bound = %v", got)
	}
	if got := (&PushWorker{PollInterval: 2 * time.Hour}).pollInterval(); got != pushMaximumPollInterval {
		t.Fatalf("maximum poll bound = %v", got)
	}
}

func encodedPushPolicy(t *testing.T, mutate func(*PushSettingsView)) json.RawMessage {
	t.Helper()
	settings := clonePushSettings(defaultPushSettings)
	if mutate != nil {
		mutate(&settings)
	}
	normalized, err := normalizePushSettings(settings)
	if err != nil {
		t.Fatalf("normalize test policy: %v", err)
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func policyProjectionQueue(now time.Time, policy json.RawMessage, event store.WebhookEventRow) *fakePushWorkerStore {
	eventAt := now.Add(-5 * time.Second)
	event.CursorAt, event.At = eventAt, eventAt
	event.TenantID, event.ServiceName = "tea-one", "api"
	queue := newFakePushWorkerStore()
	queue.watermarkAt = eventAt.Add(-time.Second)
	queue.serviceIDs["tea-one\x00api"] = "srv-c185th5c2rvvnhbfiltg"
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "viewer", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "token", PushPolicy: policy,
		CreatedAt: eventAt.Add(-time.Hour),
	}}
	queue.events = []store.WebhookEventRow{event}
	return queue
}

func TestPushWorkerAppliesStoredPolicyBeforeEnqueue(t *testing.T) {
	now := time.Date(2026, time.August, 3, 23, 0, 0, 0, time.UTC) // Monday
	deployFailure := store.WebhookEventRow{
		Key: "dep-policy:ended", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed,
	}
	run := func(t *testing.T, policy json.RawMessage, event store.WebhookEventRow) (*fakePushWorkerStore, error) {
		t.Helper()
		queue := policyProjectionQueue(now, policy, event)
		err := (&PushWorker{Store: queue, Clock: func() time.Time { return now }}).RunOnce(context.Background())
		return queue, err
	}
	count := func(queue *fakePushWorkerStore) int {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		return len(queue.notifications)
	}

	t.Run("disabled channel", func(t *testing.T) {
		queue, err := run(t, encodedPushPolicy(t, func(settings *PushSettingsView) {
			settings.Enabled = false
		}), deployFailure)
		if err != nil || count(queue) != 0 {
			t.Fatalf("disabled queue count=%d error=%v", count(queue), err)
		}
	})

	t.Run("event filter", func(t *testing.T) {
		queue, err := run(t, encodedPushPolicy(t, func(settings *PushSettingsView) {
			settings.Events = []DeliveryEvent{DeliveryEventServerFailed}
		}), deployFailure)
		if err != nil || count(queue) != 0 {
			t.Fatalf("filtered queue count=%d error=%v", count(queue), err)
		}
	})

	t.Run("service override", func(t *testing.T) {
		empty := []DeliveryEvent{}
		queue, err := run(t, encodedPushPolicy(t, func(settings *PushSettingsView) {
			settings.ServiceOverrides = []PushServiceOverrideView{{
				ServiceID: "srv-c185th5c2rvvnhbfiltg", Events: &empty,
			}}
		}), deployFailure)
		if err != nil || count(queue) != 0 {
			t.Fatalf("override queue count=%d error=%v", count(queue), err)
		}
	})

	quietPolicy := encodedPushPolicy(t, func(settings *PushSettingsView) {
		settings.QuietHours = []PushClockRangeView{{
			Weekdays: []string{"monday"}, Start: "22:00", End: "08:00",
		}}
		settings.MaxDeferralSeconds = 12 * 60 * 60
	})
	t.Run("quiet hours defer", func(t *testing.T) {
		queue, err := run(t, quietPolicy, deployFailure)
		if err != nil || count(queue) != 1 {
			t.Fatalf("deferred queue count=%d error=%v", count(queue), err)
		}
		queue.mu.Lock()
		defer queue.mu.Unlock()
		for _, notification := range queue.notifications {
			want := time.Date(2026, time.August, 4, 8, 0, 0, 0, time.UTC)
			if !notification.DeliverAt.Equal(want) {
				t.Fatalf("deferred until %s, want %s", notification.DeliverAt, want)
			}
		}
	})

	t.Run("critical bypass", func(t *testing.T) {
		serverFailure := store.WebhookEventRow{
			Key: "fact:server-policy", Source: store.EventSourceFact,
			FactType: string(store.EventFactServerFailed),
		}
		queue, err := run(t, quietPolicy, serverFailure)
		if err != nil || count(queue) != 1 {
			t.Fatalf("critical queue count=%d error=%v", count(queue), err)
		}
		queue.mu.Lock()
		defer queue.mu.Unlock()
		for _, notification := range queue.notifications {
			if !notification.DeliverAt.Equal(now) || notification.Urgency != "critical" {
				t.Fatalf("critical notification = %+v", notification)
			}
		}
	})

	t.Run("malformed stored policy drops only that recipient and advances", func(t *testing.T) {
		queue := policyProjectionQueue(now, json.RawMessage(`{"enabled":true}`), deployFailure)
		valid := queue.destinations[0]
		valid.Subject, valid.DeviceID, valid.PushPolicy = "bob", "bob-ios", encodedPushPolicy(t, nil)
		queue.destinations = append(queue.destinations, valid)
		err := (&PushWorker{Store: queue, Clock: func() time.Time { return now }}).RunOnce(context.Background())
		if err != nil || count(queue) != 1 || queue.watermarkKey != deployFailure.Key {
			t.Fatalf("malformed count=%d watermark=(%s,%q) error=%v", count(queue), queue.watermarkAt, queue.watermarkKey, err)
		}
	})
}

// TestPushWorkerProjectsTerminalAgentSessions pins w11/m6 t005: a terminal agent
// session becomes one workspace-scoped push that deep-links to the session, with
// PR-ready vs failed distinguished, deduped per (session, phase), and the feed
// watermark left untouched.
func TestPushWorkerProjectsTerminalAgentSessions(t *testing.T) {
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	queue := newFakePushWorkerStore()
	queue.watermarkAt = now.Add(-time.Hour)
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "tok", CreatedAt: now.Add(-time.Hour),
	}}
	queue.agentSessions = []store.AgentSession{
		{
			ID: "ags-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-one", Repo: "org/app",
			Phase: "completed", PRURL: "https://github.com/org/app/pull/7", PRNumber: 7, UpdatedAt: at,
		},
		{
			ID: "ags-c185th5c2rvvnhbfil00", WorkspaceID: "tea-one", Repo: "org/api",
			Phase: "failed", FailureReason: "boom", UpdatedAt: at,
		},
		// Another workspace with no recipient here — must never leak into tea-one.
		{
			ID: "ags-c185th5c2rvvnhbfil99", WorkspaceID: "tea-two", Repo: "other/x",
			Phase: "failed", UpdatedAt: at,
		},
	}
	worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()
	byResource := map[string]store.PushNotification{}
	for _, n := range queue.notifications {
		byResource[n.ResourceID] = n
	}
	if len(queue.notifications) != 2 {
		t.Fatalf("want exactly 2 agent pushes (tea-one only), got %d: %#v", len(queue.notifications), queue.notifications)
	}
	pr := byResource["ags-c185th5c2rvvnhbfiltg"]
	if pr.EventType != "agent_pr_ready" || pr.ResourceKind != "agentSession" ||
		pr.DeepLink != "/sessions/ags-c185th5c2rvvnhbfiltg" || pr.TenantID != "tea-one" {
		t.Fatalf("PR-ready push malformed: %#v", pr)
	}
	if pr.SourceEventKey != "agent:ags-c185th5c2rvvnhbfiltg:completed" {
		t.Fatalf("collapse key = %q", pr.SourceEventKey)
	}
	failed := byResource["ags-c185th5c2rvvnhbfil00"]
	if failed.EventType != "agent_failed" || failed.DeepLink != "/sessions/ags-c185th5c2rvvnhbfil00" {
		t.Fatalf("failed push malformed: %#v", failed)
	}
	if _, leaked := byResource["ags-c185th5c2rvvnhbfil99"]; leaked {
		t.Fatal("tea-two session leaked a push to tea-one's recipient")
	}
}

// TestPushWorkerDoesNotReprojectSettledAgentSessions pins the agent-session
// cursor. A terminal session stays inside the 6h scan window, so before the
// cursor every 2s tick re-fanned it out and re-enqueued it — thousands of write
// transactions per session, saved from being duplicates only by ON CONFLICT.
// The cursor must stop that WITHOUT losing a session that settles later.
func TestPushWorkerDoesNotReprojectSettledAgentSessions(t *testing.T) {
	now := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	queue := newFakePushWorkerStore()
	queue.watermarkAt = now.Add(-time.Hour)
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: "tea-one", Subject: "alice", Role: "admin", DeviceID: "alice-ios",
		Provider: "expo", Platform: "ios", Token: "tok", CreatedAt: now.Add(-time.Hour),
	}}
	queue.agentSessions = []store.AgentSession{{
		ID: "ags-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-one", Repo: "org/app",
		Phase: "failed", FailureReason: "boom", UpdatedAt: now.Add(-time.Minute),
	}}
	worker := &PushWorker{Store: queue, Clock: func() time.Time { return now }}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue.mu.Lock()
	afterFirst := queue.enqueueCalls
	queue.mu.Unlock()

	for range 5 {
		if err := worker.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	queue.mu.Lock()
	afterRepeats := queue.enqueueCalls
	queue.mu.Unlock()
	if afterRepeats != afterFirst {
		t.Errorf("settled session re-enqueued on %d further ticks; want 0", afterRepeats-afterFirst)
	}

	// A session settling later — at the SAME instant as the cursor — must still
	// be projected: the skip is strictly-older, so the boundary is re-read.
	queue.mu.Lock()
	queue.agentSessions = append(queue.agentSessions, store.AgentSession{
		ID: "ags-c185th5c2rvvnhbfil00", WorkspaceID: "tea-one", Repo: "org/api",
		Phase: "failed", FailureReason: "later", UpdatedAt: now.Add(-time.Minute),
	})
	queue.mu.Unlock()
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	found := false
	for _, n := range queue.notifications {
		if n.ResourceID == "ags-c185th5c2rvvnhbfil00" {
			found = true
		}
	}
	if !found {
		t.Error("a session settling at the cursor instant was skipped — the cursor must not lose it")
	}
}

func TestPushWorkerWebPushCompletesWithoutReceipt(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	n := validWorkerNotification(now.Add(-time.Minute))
	queue := newFakePushWorkerStore()
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: n.TenantID, Subject: n.Subject, DeviceID: "wp-browser",
		Provider: "webpush", Platform: "web", Token: "https://push.example/endpoint",
		P256dh: "p256", Auth: "auth",
	}}
	queue.notifications[pushNotificationKey(n)] = n
	queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "wp-browser", n.SourceEventKey)] =
		&fakePushQueueDelivery{notification: n, deviceID: "wp-browser"}
	sender := &fakePushSender{}
	worker := &PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "wp-browser", n.SourceEventKey)]
	if d == nil || !d.delivered || d.accepted {
		t.Fatalf("webpush delivery = %+v, want delivered without receipt acceptance", d)
	}
	if len(sender.snapshot()) != 1 {
		t.Fatalf("sends = %d, want 1", len(sender.snapshot()))
	}
}

func TestPushWorkerSharesPolicyAcrossExpoAndWebPush(t *testing.T) {
	now := time.Date(2026, time.August, 3, 23, 0, 0, 0, time.UTC)
	deployFailure := store.WebhookEventRow{
		Key: "dep-policy:ended", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed,
	}
	queue := policyProjectionQueue(now, encodedPushPolicy(t, func(settings *PushSettingsView) {
		settings.Enabled = false
	}), deployFailure)
	queue.destinations = append(queue.destinations, store.ActivePushSubscription{
		TenantID: "tea-one", Subject: "alice", Role: "viewer", DeviceID: "wp-browser",
		Provider: "webpush", Platform: "web", Token: "https://push.example/endpoint",
		CreatedAt: now.Add(-time.Hour),
	})
	if err := (&PushWorker{Store: queue, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.notifications) != 0 || len(queue.deliveries) != 0 {
		t.Fatalf("disabled policy still enqueued logical=%d deliveries=%d", len(queue.notifications), len(queue.deliveries))
	}
}

func TestPushWorkerPrunesGoneWebPushDestination(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	n := validWorkerNotification(now.Add(-time.Minute))
	queue := newFakePushWorkerStore()
	queue.destinations = []store.ActivePushSubscription{{
		TenantID: n.TenantID, Subject: n.Subject, DeviceID: "wp-browser",
		Provider: "webpush", Platform: "web", Token: "https://push.example/gone",
	}}
	queue.notifications[pushNotificationKey(n)] = n
	queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "wp-browser", n.SourceEventKey)] =
		&fakePushQueueDelivery{notification: n, deviceID: "wp-browser"}
	worker := &PushWorker{
		Store: queue, Clock: func() time.Time { return now },
		Sender: &fakePushSender{err: &pushtransport.InvalidTokenError{Code: "410"}},
	}
	_ = worker.RunOnce(context.Background())
	if len(queue.destinations) != 0 {
		t.Fatalf("gone webpush destination still active: %+v", queue.destinations)
	}
}

func TestPushWorkerQuietHoursAndOverrideApplyToWebPush(t *testing.T) {
	now := time.Date(2026, time.August, 3, 23, 0, 0, 0, time.UTC)
	deployFailure := store.WebhookEventRow{
		Key: "dep-policy-web:ended", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseEnded, Status: store.DeployUpdateFailed,
	}
	quiet := encodedPushPolicy(t, func(settings *PushSettingsView) {
		settings.QuietHours = []PushClockRangeView{{
			Weekdays: []string{"monday"}, Start: "22:00", End: "08:00",
		}}
		settings.MaxDeferralSeconds = 12 * 60 * 60
	})
	queue := policyProjectionQueue(now, quiet, deployFailure)
	queue.destinations = append(queue.destinations, store.ActivePushSubscription{
		TenantID: "tea-one", Subject: "alice", Role: "viewer", DeviceID: "wp-browser",
		Provider: "webpush", Platform: "web", Token: "https://push.example/endpoint",
		PushPolicy: quiet, CreatedAt: now.Add(-time.Hour),
	})
	if err := (&PushWorker{Store: queue, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.notifications) != 1 || len(queue.deliveries) != 2 {
		t.Fatalf("quiet-hours fan-out logical=%d deliveries=%d", len(queue.notifications), len(queue.deliveries))
	}
	want := time.Date(2026, time.August, 4, 8, 0, 0, 0, time.UTC)
	for _, n := range queue.notifications {
		if !n.DeliverAt.Equal(want) {
			t.Fatalf("shared deferral = %s, want %s", n.DeliverAt, want)
		}
	}

	empty := []DeliveryEvent{}
	blocked := encodedPushPolicy(t, func(settings *PushSettingsView) {
		settings.ServiceOverrides = []PushServiceOverrideView{{
			ServiceID: "srv-c185th5c2rvvnhbfiltg", Events: &empty,
		}}
	})
	blockedQueue := policyProjectionQueue(now, blocked, deployFailure)
	blockedQueue.destinations = append(blockedQueue.destinations, store.ActivePushSubscription{
		TenantID: "tea-one", Subject: "alice", Role: "viewer", DeviceID: "wp-browser",
		Provider: "webpush", Platform: "web", Token: "https://push.example/endpoint",
		PushPolicy: blocked, CreatedAt: now.Add(-time.Hour),
	})
	if err := (&PushWorker{Store: blockedQueue, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	blockedQueue.mu.Lock()
	defer blockedQueue.mu.Unlock()
	if len(blockedQueue.notifications) != 0 || len(blockedQueue.deliveries) != 0 {
		t.Fatalf("override still enqueued logical=%d deliveries=%d", len(blockedQueue.notifications), len(blockedQueue.deliveries))
	}
}

func TestPushWorkerTransportSelectionMatrix(t *testing.T) {
	now := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	n := validWorkerNotification(now.Add(-time.Minute))
	seed := func() *fakePushWorkerStore {
		queue := newFakePushWorkerStore()
		queue.destinations = []store.ActivePushSubscription{
			{TenantID: n.TenantID, Subject: n.Subject, DeviceID: "ios", Provider: "expo", Platform: "ios", Token: "expo-token"},
			{TenantID: n.TenantID, Subject: n.Subject, DeviceID: "wp-browser", Provider: "webpush", Platform: "web", Token: "https://push.example/endpoint", P256dh: "p256", Auth: "auth"},
		}
		queue.notifications[pushNotificationKey(n)] = n
		queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "ios", n.SourceEventKey)] =
			&fakePushQueueDelivery{notification: n, deviceID: "ios"}
		queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "wp-browser", n.SourceEventKey)] =
			&fakePushQueueDelivery{notification: n, deviceID: "wp-browser"}
		return queue
	}
	providers := func(messages []PushSendRequest) []string {
		out := make([]string, 0, len(messages))
		for _, m := range messages {
			out = append(out, m.Provider)
		}
		return out
	}

	t.Run("expo only", func(t *testing.T) {
		queue, sender := seed(), &fakePushSender{support: map[string]bool{"expo": true}}
		if err := (&PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := providers(sender.snapshot()); len(got) != 1 || got[0] != "expo" {
			t.Fatalf("sends = %v, want [expo]", got)
		}
	})
	t.Run("webpush only", func(t *testing.T) {
		queue, sender := seed(), &fakePushSender{support: map[string]bool{"webpush": true}}
		if err := (&PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := providers(sender.snapshot()); len(got) != 1 || got[0] != "webpush" {
			t.Fatalf("sends = %v, want [webpush]", got)
		}
		d := queue.deliveries[pushDeliveryKey(n.TenantID, n.Subject, "wp-browser", n.SourceEventKey)]
		if d == nil || !d.delivered {
			t.Fatalf("webpush delivery = %+v, want delivered", d)
		}
	})
	t.Run("both", func(t *testing.T) {
		queue, sender := seed(), &fakePushSender{}
		if err := (&PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := providers(sender.snapshot()); len(got) != 2 {
			t.Fatalf("sends = %v, want expo and webpush", got)
		}
	})
	t.Run("neither", func(t *testing.T) {
		queue, sender := seed(), &fakePushSender{support: map[string]bool{}}
		if err := (&PushWorker{Store: queue, Sender: sender, Clock: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(sender.snapshot()) != 0 {
			t.Fatalf("neither-configured still sent %v", providers(sender.snapshot()))
		}
		for _, d := range queue.deliveries {
			if d.accepted || d.delivered || !d.claimedUntil.IsZero() {
				t.Fatalf("unsupported provider mutated delivery: %+v", d)
			}
		}
	})
}
