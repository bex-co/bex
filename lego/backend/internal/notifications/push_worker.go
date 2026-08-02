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
	"log"
	"sort"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	pushtransport "github.com/bex-co/bex/lego/backend/internal/notifications/push"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	pushDispatchLag         = 3 * time.Second
	pushDispatchBatch       = 200
	pushSendBatch           = 50
	pushClaimLease          = time.Minute
	pushParkInterval        = time.Minute
	pushDefaultPollInterval = 2 * time.Second
	pushMinimumPollInterval = 100 * time.Millisecond
	pushMaximumPollInterval = time.Minute
	pushSchema              = "bex.notification.v1"
	pushMaxAttempts         = 8
	pushReceiptDelay        = 15 * time.Minute
	pushReceiptWindow       = 24 * time.Hour
	pushRetention           = 90 * 24 * time.Hour
	pushSweepInterval       = time.Hour
)

// PushWorkerStore is the durable projection and queue seam. *store.PGStore
// satisfies it; tests use an in-memory transactional fake.
type PushWorkerStore interface {
	ListActivePushSubscriptions(context.Context) ([]store.ActivePushSubscription, error)
	EnsurePushWatermark(context.Context, time.Time) (time.Time, string, error)
	ListWebhookEvents(context.Context, time.Time, string, time.Time, []string, []string, int) ([]store.WebhookEventRow, error)
	ResolvePushServiceID(context.Context, string, string) (string, error)
	PushFactStatus(context.Context, string) (string, error)
	EnqueuePushNotifications(context.Context, []store.PushNotificationBatchItem, time.Time, string) error
	ClaimDuePushDeliveries(context.Context, time.Time, time.Time, int) ([]store.DuePushDelivery, error)
	AcceptPushDelivery(context.Context, store.DuePushDelivery, string, time.Time, time.Time) (bool, error)
	ReleasePushDelivery(context.Context, store.DuePushDelivery) (bool, error)
	RecordPushSendFailure(context.Context, store.DuePushDelivery, string, time.Time, time.Time, bool) (bool, error)
	ClaimDuePushReceipts(context.Context, time.Time, time.Time, int) ([]store.DuePushDelivery, error)
	RecordPushReceipt(context.Context, store.DuePushDelivery, string, time.Time, time.Time, bool, bool, bool) (bool, error)
	RevokeExactPushSubscription(context.Context, store.DuePushDelivery) (bool, error)
	PushDeliveryStats(context.Context) (store.PushQueueStats, error)
	SweepPushRetention(context.Context, time.Time, time.Time) (store.PushSweepResult, error)
}

// PushEnvelopeData is the entire provider-visible custom data envelope. Do not
// add arbitrary event facts here; mobile fetches details through authenticated
// APIs after following Route.
type PushEnvelopeData struct {
	Schema         string `json:"schema"`
	NotificationID string `json:"notificationId"`
	Event          string `json:"event"`
	Route          string `json:"route"`
}

// PushSendRequest is the narrow adapter seam for the provider transport.
// Token is a delivery capability and cannot enter JSON. Title/body and the
// four exact Data fields are the only provider-visible content.
type PushSendRequest struct {
	Provider string
	Platform string
	Token    string `json:"-"`
	Title    string
	Body     string
	Urgency  string
	Data     PushEnvelopeData
}

// PushSender submits one provider message and returns its acceptance ticket.
// Receipt polling, retry policy, and stale-token pruning belong to t006.
type PushSender interface {
	Send(context.Context, PushSendRequest) (ticketID string, err error)
}

type PushReceiptChecker interface {
	CheckReceipts(context.Context, []string) (map[string]pushtransport.Receipt, error)
}

type PushEvidence struct{ Operation, Result, ErrorCode string }
type PushEvidenceRecorder interface {
	RecordPushEvidence(context.Context, PushEvidence)
}

// ErrPushSenderFailed is deliberately detail-free so a provider error can
// never reflect a device token into worker logs.
var ErrPushSenderFailed = errors.New("push sender failed")

// PushWorker tails the existing composed event feed and sends already-committed
// queue rows. Sender nil leaves the durable queue intact and performs no send.
type PushWorker struct {
	Store        PushWorkerStore
	Sender       PushSender
	Receipts     PushReceiptChecker
	Metrics      *PushMetrics
	Evidence     PushEvidenceRecorder
	Clock        func() time.Time
	PollInterval time.Duration
	// Tick is an optional test scheduler. Production leaves it nil and Run owns
	// a bounded ticker derived from PollInterval.
	Tick <-chan time.Time

	watermarkLoaded bool
	watermarkAt     time.Time
	watermarkKey    string
	lastSweep       time.Time
}

func (w *PushWorker) pollInterval() time.Duration {
	interval := w.PollInterval
	if interval == 0 {
		return pushDefaultPollInterval
	}
	if interval < pushMinimumPollInterval {
		return pushMinimumPollInterval
	}
	if interval > pushMaximumPollInterval {
		return pushMaximumPollInterval
	}
	return interval
}

// Run polls until cancellation. Delivery errors are deliberately swallowed:
// rows remain durable/retryable and the next tick resumes. Provider details
// are already redacted before they reach this boundary.
func (w *PushWorker) Run(ctx context.Context) {
	var ticks <-chan time.Time
	var ticker *time.Ticker
	if w.Tick != nil {
		ticks = w.Tick
	} else {
		ticker = time.NewTicker(w.pollInterval())
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("notifications: push worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}

func (w *PushWorker) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

// RunOnce projects first and sends second, so no provider call can occur until
// logical notifications, device deliveries, and the feed cursor are committed.
func (w *PushWorker) RunOnce(ctx context.Context) error {
	if w.Store == nil {
		return errors.New("push worker store is required")
	}
	destinations, err := w.Store.ListActivePushSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("list active push destinations: %w", err)
	}
	if len(destinations) > 0 {
		if err := w.dispatch(ctx, destinations); err != nil {
			return fmt.Errorf("project push events: %w", err)
		}
	}
	if w.Sender == nil && w.Receipts == nil {
		w.Metrics.SetEnabled(false)
		return nil
	}
	w.Metrics.SetEnabled(true)
	var sendErr error
	if w.Sender != nil {
		sendErr = w.send(ctx)
	}
	err = errors.Join(sendErr, w.checkReceipts(ctx), w.sweep(ctx))
	if stats, statsErr := w.Store.PushDeliveryStats(ctx); statsErr == nil {
		w.Metrics.SetQueue(stats)
	}
	return err
}

type pushRecipient struct {
	subject string
	role    DeliveryWorkspaceRole
	policy  DeliveryPolicy
	devices []store.ActivePushSubscription
}

func (w *PushWorker) dispatch(ctx context.Context, destinations []store.ActivePushSubscription) error {
	until := w.now().Add(-pushDispatchLag)
	if !w.watermarkLoaded {
		at, key, err := w.Store.EnsurePushWatermark(ctx, until)
		if err != nil {
			return err
		}
		w.watermarkAt, w.watermarkKey, w.watermarkLoaded = at, key, true
	}

	byTenant := make(map[string]map[string]*pushRecipient)
	invalidRecipients := make(map[string]bool)
	for _, destination := range destinations {
		recipientKey := destination.TenantID + "\x00" + destination.Subject
		if invalidRecipients[recipientKey] {
			continue
		}
		bySubject := byTenant[destination.TenantID]
		if bySubject == nil {
			bySubject = make(map[string]*pushRecipient)
			byTenant[destination.TenantID] = bySubject
		}
		recipient := bySubject[destination.Subject]
		if recipient == nil {
			role, ok := deliveryWorkspaceRole(destination.Role)
			if !ok {
				invalidRecipients[recipientKey] = true
				delete(bySubject, destination.Subject)
				w.evidence(ctx, "policy", "invalid", "invalid_role")
				w.Metrics.Operation("policy", "invalid")
				continue
			}
			policy, err := storedPushDeliveryPolicy(destination.PushPolicy)
			if err != nil {
				invalidRecipients[recipientKey] = true
				delete(bySubject, destination.Subject)
				w.evidence(ctx, "policy", "invalid", "invalid_policy")
				w.Metrics.Operation("policy", "invalid")
				continue
			}
			recipient = &pushRecipient{subject: destination.Subject, role: role, policy: policy}
			bySubject[destination.Subject] = recipient
		}
		recipient.devices = append(recipient.devices, destination)
	}
	tenants := make([]string, 0, len(byTenant))
	for tenantID, recipients := range byTenant {
		if len(recipients) == 0 {
			continue
		}
		tenants = append(tenants, tenantID)
	}
	if len(tenants) == 0 {
		return nil
	}
	sort.Strings(tenants)

	for {
		rows, err := w.Store.ListWebhookEvents(
			ctx, w.watermarkAt, w.watermarkKey, until, []string{}, tenants, pushDispatchBatch,
		)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			if until.Sub(w.watermarkAt) > pushParkInterval {
				if err := w.Store.EnqueuePushNotifications(ctx, nil, until, ""); err != nil {
					return err
				}
				w.watermarkAt, w.watermarkKey = until, ""
			}
			return nil
		}

		batch := make([]store.PushNotificationBatchItem, 0)
		decisionTime := w.now()
		evaluator := DeliveryPolicyEvaluator{Now: func() time.Time { return decisionTime }}
		for _, row := range rows {
			serviceID, err := w.Store.ResolvePushServiceID(ctx, row.TenantID, row.ServiceName)
			if err != nil {
				return err
			}
			factStatus := ""
			if row.Source == store.EventSourceFact && row.FactType == string(store.EventFactJobRunEnded) {
				factStatus, err = w.Store.PushFactStatus(ctx, row.Key)
				if err != nil {
					return err
				}
			}
			projected, ok := projectPushEvent(row, serviceID, factStatus)
			if !ok {
				continue
			}
			for _, recipient := range byTenant[row.TenantID] {
				decision, err := evaluator.Evaluate(recipient.policy, DeliveryInput{
					Channel: DeliveryChannelPush, Event: DeliveryEvent(projected.event),
					Urgency: DeliveryUrgency(projected.urgency), WorkspaceID: row.TenantID,
					EventWorkspaceID: row.TenantID, Subject: recipient.subject,
					WorkspaceRole: recipient.role, ServiceID: serviceID,
				})
				if err != nil {
					return fmt.Errorf("evaluate stored push policy: %w", err)
				}
				if decision.Disposition == DeliveryDrop {
					continue
				}
				if decision.Disposition != DeliverySend && decision.Disposition != DeliveryDefer {
					return fmt.Errorf("evaluate stored push policy: unknown disposition %q", decision.Disposition)
				}
				deviceIDs := make([]string, 0, len(recipient.devices))
				for _, device := range recipient.devices {
					if !row.At.Before(device.CreatedAt) {
						deviceIDs = append(deviceIDs, device.DeviceID)
					}
				}
				if len(deviceIDs) == 0 {
					continue
				}
				notificationID := ids.Derive(ids.Event, row.TenantID, recipient.subject, row.Key)
				batch = append(batch, store.PushNotificationBatchItem{
					Notification: store.PushNotification{
						TenantID: row.TenantID, Subject: recipient.subject,
						SourceEventKey: row.Key, EventID: notificationID,
						EventType: projected.event, Title: projected.title, Body: projected.body,
						Urgency: projected.urgency, ResourceKind: "service", ResourceID: serviceID,
						DeepLink: "/services/" + serviceID, OccurredAt: row.At, DeliverAt: decision.DeliverAt,
					},
					DeviceIDs: deviceIDs,
				})
			}
		}
		last := rows[len(rows)-1]
		if err := w.Store.EnqueuePushNotifications(ctx, batch, last.CursorAt, last.Key); err != nil {
			return err
		}
		w.watermarkAt, w.watermarkKey = last.CursorAt, last.Key
		if len(rows) < pushDispatchBatch {
			return nil
		}
	}
}

func storedPushDeliveryPolicy(raw json.RawMessage) (DeliveryPolicy, error) {
	view := clonePushSettings(defaultPushSettings)
	if len(raw) != 0 {
		view = PushSettingsView{}
		if err := json.Unmarshal(raw, &view); err != nil {
			return DeliveryPolicy{}, fmt.Errorf("decode stored push policy")
		}
	}
	normalized, err := normalizePushSettings(view)
	if err != nil {
		return DeliveryPolicy{}, fmt.Errorf("normalize stored push policy: %w", err)
	}
	policy, err := normalized.deliveryPolicy()
	if err != nil {
		return DeliveryPolicy{}, fmt.Errorf("compile stored push policy: %w", err)
	}
	return policy, nil
}

func deliveryWorkspaceRole(role string) (DeliveryWorkspaceRole, bool) {
	switch role {
	case string(DeliveryRoleViewer):
		return DeliveryRoleViewer, true
	case string(DeliveryRoleContributor):
		return DeliveryRoleContributor, true
	case string(DeliveryRoleDeveloper):
		return DeliveryRoleDeveloper, true
	case string(DeliveryRoleAdmin):
		return DeliveryRoleAdmin, true
	case string(DeliveryRoleBilling):
		return DeliveryRoleBilling, true
	default:
		return "", false
	}
}

type projectedPushEvent struct {
	event   string
	title   string
	body    string
	urgency string
}

func projectPushEvent(row store.WebhookEventRow, serviceID, factStatus string) (projectedPushEvent, bool) {
	serviceName := row.ServiceName
	if serviceName == "" {
		serviceName = serviceID
	}
	switch row.Source {
	case store.EventSourceDeploy:
		if row.Phase == store.EventPhaseStarted {
			return projectedPushEvent{
				event: string(DeliveryEventDeployStarted), title: "Deploy started",
				body: serviceName + " deploy started.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		}
		if row.Phase != store.EventPhaseEnded {
			return projectedPushEvent{}, false
		}
		switch row.Status {
		case store.DeployLive:
			return projectedPushEvent{
				event: string(DeliveryEventDeploySucceeded), title: "Deploy succeeded",
				body: serviceName + " is live.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		case store.DeployBuildFailed, store.DeployPreDeployFailed, store.DeployUpdateFailed:
			return projectedPushEvent{
				event: string(DeliveryEventDeployFailed), title: "Deploy failed",
				body: serviceName + " deploy failed.", urgency: string(DeliveryUrgencyImportant),
			}, true
		default:
			return projectedPushEvent{}, false
		}
	case store.EventSourceFact:
		switch row.FactType {
		case string(store.EventFactServerFailed):
			return projectedPushEvent{
				event: string(DeliveryEventServerFailed), title: "Service unavailable",
				body: serviceName + " is unavailable.", urgency: string(DeliveryUrgencyCritical),
			}, true
		case string(store.EventFactJobRunEnded):
			if factStatus != store.EventStatusFailed {
				return projectedPushEvent{}, false
			}
			return projectedPushEvent{
				event: string(DeliveryEventCronFailed), title: "Cron job failed",
				body: serviceName + " cron job failed.", urgency: string(DeliveryUrgencyImportant),
			}, true
		}
	}
	return projectedPushEvent{}, false
}

func (w *PushWorker) send(ctx context.Context) error {
	now := w.now()
	deliveries, err := w.Store.ClaimDuePushDeliveries(ctx, now, now.Add(pushClaimLease), pushSendBatch)
	if err != nil {
		return err
	}
	var failures []error
	for _, delivery := range deliveries {
		ticketID, sendErr := w.Sender.Send(ctx, PushSendRequest{
			Provider: delivery.Provider, Platform: delivery.Platform, Token: delivery.Token,
			Title: delivery.Title, Body: delivery.Body, Urgency: delivery.Urgency,
			Data: PushEnvelopeData{
				Schema: pushSchema, NotificationID: delivery.EventID,
				Event: delivery.EventType, Route: delivery.DeepLink,
			},
		})
		if sendErr != nil {
			failures = append(failures, w.recordSendFailure(ctx, delivery, sendErr))
			failures = append(failures, ErrPushSenderFailed)
			continue
		}
		acceptedAt := w.now()
		accepted, acceptErr := w.Store.AcceptPushDelivery(ctx, delivery, ticketID, acceptedAt, acceptedAt.Add(pushReceiptDelay))
		if acceptErr != nil {
			failures = append(failures, acceptErr)
			continue
		}
		if !accepted {
			failures = append(failures, errors.New("push delivery lease changed before acceptance"))
		}
		w.Metrics.Operation("send", "accepted")
		w.Metrics.Succeeded(acceptedAt)
	}
	return errors.Join(failures...)
}

func pushRetryDelay(attempt int) time.Duration {
	d := 30 * time.Second
	for i := 1; i < attempt && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		return time.Hour
	}
	return d
}
func boundedRetryAfter(v, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	if v > time.Hour {
		return time.Hour
	}
	return v
}

func (w *PushWorker) recordSendFailure(ctx context.Context, d store.DuePushDelivery, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		_, releaseErr := w.Store.ReleasePushDelivery(ctx, d)
		return releaseErr
	}
	code, terminal, prune, delay := "transient", false, false, pushRetryDelay(d.AttemptCount+1)
	var invalid *pushtransport.InvalidTokenError
	var payload *pushtransport.PayloadError
	var permanent *pushtransport.PermanentError
	var limited *pushtransport.RateLimitedError
	var transient *pushtransport.TransientError
	switch {
	case errors.As(err, &invalid):
		code, terminal, prune = "invalid_token", true, true
	case errors.As(err, &payload):
		code, terminal = "payload", true
	case errors.As(err, &permanent):
		code, terminal = "permanent", true
	case errors.As(err, &limited):
		code = "rate_limited"
		delay = boundedRetryAfter(limited.RetryAfter, delay)
	case errors.As(err, &transient):
		code = "transient"
	}
	if d.AttemptCount+1 >= pushMaxAttempts {
		terminal = true
	}
	now := w.now()
	_, storeErr := w.Store.RecordPushSendFailure(ctx, d, code, now, now.Add(delay), terminal)
	if prune {
		_, _ = w.Store.RevokeExactPushSubscription(ctx, d)
		w.evidence(ctx, "prune", "invalid_token", code)
		w.Metrics.Operation("prune", "invalid_token")
	}
	result := "retry"
	if terminal {
		result = "terminal"
		w.evidence(ctx, "send", result, code)
	}
	w.Metrics.Operation("send", result)
	return storeErr
}

func (w *PushWorker) checkReceipts(ctx context.Context) error {
	if w.Receipts == nil {
		return nil
	}
	now := w.now()
	due, err := w.Store.ClaimDuePushReceipts(ctx, now, now.Add(pushClaimLease), pushSendBatch)
	if err != nil {
		return err
	}
	if len(due) == 0 {
		return nil
	}
	ids := make([]string, 0, len(due))
	for _, d := range due {
		ids = append(ids, d.ProviderTicketID)
	}
	receipts, checkErr := w.Receipts.CheckReceipts(ctx, ids)
	var failures []error
	if errors.Is(checkErr, context.Canceled) || errors.Is(checkErr, context.DeadlineExceeded) {
		// The bounded claim lease recovers these receipt rows. Avoid converting
		// cancellation into a provider failure or ambiguous outcome.
		return checkErr
	}
	for _, d := range due {
		ambiguous := d.AcceptedAt != nil && !now.Before(d.AcceptedAt.Add(pushReceiptWindow))
		if checkErr != nil {
			_, e := w.Store.RecordPushReceipt(ctx, d, "receipt_transient", now, now.Add(pushReceiptDelay), false, false, ambiguous)
			failures = append(failures, e)
			if ambiguous {
				w.Metrics.Operation("receipt", "ambiguous")
			} else {
				w.Metrics.Operation("receipt", "retry")
			}
			continue
		}
		r, found := receipts[d.ProviderTicketID]
		if !found {
			_, e := w.Store.RecordPushReceipt(ctx, d, "receipt_pending", now, now.Add(pushReceiptDelay), false, false, ambiguous)
			failures = append(failures, e)
			if ambiguous {
				w.Metrics.Operation("receipt", "ambiguous")
			} else {
				w.Metrics.Operation("receipt", "pending")
			}
			continue
		}
		if r.Err == nil {
			_, e := w.Store.RecordPushReceipt(ctx, d, "", now, now, true, false, false)
			failures = append(failures, e)
			w.Metrics.Operation("receipt", "delivered")
			w.Metrics.Succeeded(now)
			continue
		}
		code, failed, prune := "receipt_transient", false, false
		var invalid *pushtransport.InvalidTokenError
		var payload *pushtransport.PayloadError
		var permanent *pushtransport.PermanentError
		switch {
		case errors.As(r.Err, &invalid):
			code, failed, prune = "invalid_token", true, true
		case errors.As(r.Err, &payload):
			code, failed = "payload", true
		case errors.As(r.Err, &permanent):
			code, failed = "permanent", true
		}
		_, e := w.Store.RecordPushReceipt(ctx, d, code, now, now.Add(pushReceiptDelay), false, failed, !failed && ambiguous)
		failures = append(failures, e)
		result := "retry"
		if failed {
			result = "failed"
		} else if ambiguous {
			result = "ambiguous"
		}
		w.Metrics.Operation("receipt", result)
		if prune {
			_, _ = w.Store.RevokeExactPushSubscription(ctx, d)
			w.evidence(ctx, "prune", "invalid_token", code)
			w.Metrics.Operation("prune", "invalid_token")
		}
	}
	return errors.Join(failures...)
}

func (w *PushWorker) sweep(ctx context.Context) error {
	now := w.now()
	if !w.lastSweep.IsZero() && now.Sub(w.lastSweep) < pushSweepInterval {
		return nil
	}
	_, err := w.Store.SweepPushRetention(ctx, now.Add(-pushRetention), now.Add(-pushRetention))
	if err == nil {
		w.lastSweep = now
		w.Metrics.Operation("sweep", "success")
	}
	return err
}
func (w *PushWorker) evidence(ctx context.Context, op, result, code string) {
	if w.Evidence != nil {
		w.Evidence.RecordPushEvidence(ctx, PushEvidence{Operation: op, Result: result, ErrorCode: code})
	}
}
