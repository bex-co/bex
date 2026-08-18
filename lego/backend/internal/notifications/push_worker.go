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
	"sort"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	pushtransport "github.com/bex-co/bex/lego/backend/internal/notifications/push"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// A property of the feed, not of this worker: store.FeedCommitLag explains
	// why a tailer must read behind now, and every tailer shares it.
	pushDispatchLag   = store.FeedCommitLag
	pushDispatchBatch = 200
	pushSendBatch     = 50
	pushClaimLease    = time.Minute
	// Also a property of the feed, shared with every other tailer of it:
	// store.DefaultFeedPark explains the write-amplification trade.
	pushParkInterval        = store.DefaultFeedPark
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
	ListTerminalAgentSessionsForPush(context.Context, time.Time) ([]store.AgentSession, error)
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
	Subject        string `json:"subject"`
	WorkspaceID    string `json:"workspaceId"`
	SessionID      string `json:"sessionId"`
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

	// cursor is this worker's cached position in the composed feed, loaded once
	// and advanced by every committed page (store.FeedCursor).
	cursor    store.FeedCursor
	lastSweep time.Time
	// agentSessionCursor is the newest session UpdatedAt already projected to
	// push, and agentSessionBoundary the source keys sitting at exactly that
	// instant. Terminal sessions stay in the 6h scan window, so without them
	// every tick (2s) re-fans-out and re-enqueues the same sessions for six
	// hours — ~10k redundant write transactions per session, each a batch of
	// INSERTs that only ON CONFLICT DO NOTHING saves from being duplicates.
	//
	// The boundary set is what makes the skip exact rather than merely cheap: a
	// bare timestamp forces a choice between re-projecting the newest instant
	// forever (no saving) and skipping not-newer sessions (losing one that
	// commits after this tick's read but shares that instant). Naming the keys
	// already projected at the boundary rules out both. It holds one entry in
	// the ordinary case — only sessions sharing the exact newest timestamp.
	agentSessionCursor   time.Time
	agentSessionBoundary map[string]bool
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
	const name = "notifications: push worker"
	if w.Tick != nil {
		core.PollTicks(ctx, name, w.Tick, w.RunOnce)
		return
	}
	core.Poll(ctx, name, w.pollInterval(), w.RunOnce)
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
		// Built once per tick: both dispatchers evaluate the SAME recipients, and
		// the build recompiles every subscriber's stored policy (and evidences the
		// invalid ones), so doing it per dispatcher doubled that work and
		// double-counted the invalid-policy metric.
		byTenant, tenants := w.buildPushRecipients(ctx, destinations)
		if err := w.dispatch(ctx, byTenant, tenants); err != nil {
			return fmt.Errorf("project push events: %w", err)
		}
		if err := w.dispatchAgentSessions(ctx, byTenant, tenants); err != nil {
			return fmt.Errorf("project agent-session push events: %w", err)
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

// agentPushWindow bounds the terminal-agent-session scan: a session that goes
// terminal is pushed within this window; re-scanning within it is idempotent via
// the notification's source_event_key ON CONFLICT.
const agentPushWindow = 6 * time.Hour

// buildPushRecipients groups active subscriptions into per-tenant, per-subject
// recipients with a validated role + policy — the shared input both the feed
// dispatch and the agent-session dispatch evaluate against. Invalid role/policy
// rows are dropped (and evidenced) and never delivered.
func (w *PushWorker) buildPushRecipients(ctx context.Context, destinations []store.ActivePushSubscription) (map[string]map[string]*pushRecipient, []string) {
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
			// A recipient whose stored role or policy no longer parses is dropped
			// rather than delivered to under a guessed policy.
			markInvalid := func(code string) {
				invalidRecipients[recipientKey] = true
				delete(bySubject, destination.Subject)
				w.evidence(ctx, "policy", "invalid", code)
				w.Metrics.Operation("policy", "invalid")
			}
			role, ok := deliveryWorkspaceRole(destination.Role)
			if !ok {
				markInvalid("invalid_role")
				continue
			}
			policy, err := storedPushDeliveryPolicy(destination.PushPolicy)
			if err != nil {
				markInvalid("invalid_policy")
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
	sort.Strings(tenants)
	return byTenant, tenants
}

// projectedPush is the channel-agnostic content of one push, produced by each
// source's projector (projectPushEvent for the app feed, projectAgentSessionPush
// for terminal agent sessions) and consumed by fanOutPush.
type projectedPush struct {
	event   string
	title   string
	body    string
	urgency string
}

// pushTarget is the per-event half of a fan-out: everything that differs
// between the app-keyed feed dispatch and the workspace-keyed agent-session
// projection. The recipient evaluation and device filtering around it are
// identical, which is what fanOutPush shares.
type pushTarget struct {
	workspaceID string
	sourceKey   string
	// serviceID scopes the policy evaluation to one service; empty for agent
	// sessions, which have no App (w11/m6 t005).
	serviceID    string
	resourceKind string
	resourceID   string
	deepLink     string
	occurredAt   time.Time
	projected    projectedPush
}

// fanOutPush evaluates one event against every recipient in its workspace and
// appends the resulting batch items. A recipient contributes nothing when its
// policy drops the event or when it has no device registered before the event
// occurred — a device may not receive pushes for what happened before it existed.
func fanOutPush(
	batch []store.PushNotificationBatchItem,
	evaluator DeliveryPolicyEvaluator,
	recipients map[string]*pushRecipient,
	target pushTarget,
) ([]store.PushNotificationBatchItem, error) {
	for _, recipient := range recipients {
		decision, err := evaluator.Evaluate(recipient.policy, DeliveryInput{
			Channel: DeliveryChannelPush, Event: DeliveryEvent(target.projected.event),
			Urgency: DeliveryUrgency(target.projected.urgency), WorkspaceID: target.workspaceID,
			EventWorkspaceID: target.workspaceID, Subject: recipient.subject,
			WorkspaceRole: recipient.role, ServiceID: target.serviceID,
		})
		if err != nil {
			return nil, fmt.Errorf("evaluate stored push policy: %w", err)
		}
		if decision.Disposition == DeliveryDrop {
			continue
		}
		if decision.Disposition != DeliverySend && decision.Disposition != DeliveryDefer {
			return nil, fmt.Errorf("evaluate stored push policy: unknown disposition %q", decision.Disposition)
		}
		deviceIDs := make([]string, 0, len(recipient.devices))
		for _, device := range recipient.devices {
			if !target.occurredAt.Before(device.CreatedAt) {
				deviceIDs = append(deviceIDs, device.DeviceID)
			}
		}
		if len(deviceIDs) == 0 {
			continue
		}
		batch = append(batch, store.PushNotificationBatchItem{
			Notification: store.PushNotification{
				TenantID: target.workspaceID, Subject: recipient.subject,
				SourceEventKey: target.sourceKey,
				EventID:        ids.Derive(ids.Event, target.workspaceID, recipient.subject, target.sourceKey),
				EventType:      target.projected.event, Title: target.projected.title, Body: target.projected.body,
				Urgency: target.projected.urgency, ResourceKind: target.resourceKind, ResourceID: target.resourceID,
				DeepLink: target.deepLink, OccurredAt: target.occurredAt, DeliverAt: decision.DeliverAt,
			},
			DeviceIDs: deviceIDs,
		})
	}
	return batch, nil
}

// projectAgentSessionPush maps a terminal agent session to its push. A completed
// session that opened a draft PR is "PR ready"; a completed one without a PR and
// a failed one surface as a failure needing attention. Bodies carry no secret —
// the repo name and (for PR-ready) the PR number only.
func projectAgentSessionPush(s store.AgentSession) (projectedPush, bool) {
	switch s.Phase {
	case "completed":
		if s.PRURL != "" {
			body := "Agent opened a draft PR on " + s.Repo + "."
			if s.PRNumber > 0 {
				body = fmt.Sprintf("Agent opened draft PR #%d on %s.", s.PRNumber, s.Repo)
			}
			return projectedPush{
				event: string(DeliveryEventAgentPRReady), title: "Draft PR ready",
				body: body, urgency: string(DeliveryUrgencyImportant),
			}, true
		}
		return projectedPush{
			event: string(DeliveryEventAgentFailed), title: "Agent session ended",
			body:    "Agent session on " + s.Repo + " ended without a pull request.",
			urgency: string(DeliveryUrgencyImportant),
		}, true
	case "failed":
		return projectedPush{
			event: string(DeliveryEventAgentFailed), title: "Agent session failed",
			body: "Agent session on " + s.Repo + " failed.", urgency: string(DeliveryUrgencyImportant),
		}, true
	default:
		return projectedPush{}, false
	}
}

// dispatchAgentSessions is the agent-terminal projection (w11/m6 t005) alongside
// the app-keyed feed dispatch: agent sessions are workspace-keyed with no App,
// so they can't ride the webhook-events feed. It scans recent terminal sessions,
// evaluates the SAME recipients/policy, and enqueues a session-deep-linked push
// deduped by (session, phase). It never advances the feed watermark.
func (w *PushWorker) dispatchAgentSessions(ctx context.Context, byTenant map[string]map[string]*pushRecipient, tenants []string) error {
	if !w.cursor.Loaded() {
		return nil // the feed dispatch anchors the watermark first
	}
	if len(tenants) == 0 {
		return nil
	}
	sessions, err := w.Store.ListTerminalAgentSessionsForPush(ctx, w.now().Add(-agentPushWindow))
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}
	tenantSet := make(map[string]bool, len(tenants))
	for _, tenant := range tenants {
		tenantSet[tenant] = true
	}
	decisionTime := w.now()
	evaluator := DeliveryPolicyEvaluator{Now: func() time.Time { return decisionTime }}
	batch := make([]store.PushNotificationBatchItem, 0)
	newestSeen := w.agentSessionCursor
	boundary := map[string]bool{}
	for _, session := range sessions {
		if !tenantSet[session.WorkspaceID] {
			continue
		}
		sourceKey := "agent:" + session.ID + ":" + session.Phase
		if session.UpdatedAt.After(newestSeen) {
			newestSeen = session.UpdatedAt
			boundary = map[string]bool{}
		}
		if session.UpdatedAt.Equal(newestSeen) {
			boundary[sourceKey] = true
		}
		if w.agentSessionProjected(session.UpdatedAt, sourceKey) {
			continue
		}
		projected, ok := projectAgentSessionPush(session)
		if !ok {
			continue
		}
		batch, err = fanOutPush(batch, evaluator, byTenant[session.WorkspaceID], pushTarget{
			workspaceID:  session.WorkspaceID,
			sourceKey:    sourceKey,
			resourceKind: "agentSession", resourceID: session.ID,
			deepLink: "/sessions/" + session.ID, occurredAt: session.UpdatedAt, projected: projected,
		})
		if err != nil {
			return err
		}
	}
	if len(batch) == 0 {
		w.agentSessionCursor, w.agentSessionBoundary = newestSeen, boundary
		return nil
	}
	// Pass the CURRENT watermark: the monotonic guard makes the cursor UPDATE a
	// no-op while the notification inserts commit (deduped by source_event_key).
	watermarkAt, watermarkKey := w.cursor.Position()
	if err := w.Store.EnqueuePushNotifications(ctx, batch, watermarkAt, watermarkKey); err != nil {
		return err
	}
	// Advance only after the write commits, so a failed enqueue is retried.
	w.agentSessionCursor, w.agentSessionBoundary = newestSeen, boundary
	return nil
}

// agentSessionProjected reports whether this session has already been fanned
// out: anything strictly older than the cursor, plus the keys recorded at the
// cursor instant itself.
func (w *PushWorker) agentSessionProjected(updatedAt time.Time, sourceKey string) bool {
	if updatedAt.Before(w.agentSessionCursor) {
		return true
	}
	return updatedAt.Equal(w.agentSessionCursor) && w.agentSessionBoundary[sourceKey]
}

func (w *PushWorker) dispatch(ctx context.Context, byTenant map[string]map[string]*pushRecipient, tenants []string) error {
	until := w.now().Add(-pushDispatchLag)
	if err := w.cursor.Load(ctx, w.Store.EnsurePushWatermark, until); err != nil {
		return err
	}

	if len(tenants) == 0 {
		return nil
	}

	return store.TailFeed(ctx, &w.cursor, store.FeedPass[store.PushNotificationBatchItem]{
		Until: until,
		// No verbs => the feed's audit arms drop out entirely, which is what push
		// wants: projectPushEvent handles deploy and fact sources only.
		Verbs:   []string{},
		Tenants: tenants,
		Limit:   pushDispatchBatch,
		Park:    pushParkInterval,
		List:    w.Store.ListWebhookEvents,
		Commit:  w.Store.EnqueuePushNotifications,
		Project: func(ctx context.Context, rows []store.WebhookEventRow) ([]store.PushNotificationBatchItem, error) {
			return w.fanOutPage(ctx, rows, byTenant)
		},
	})
}

// fanOutPage turns one page of the feed into queue rows: each row is resolved to
// its service, projected onto the push vocabulary, and evaluated against every
// recipient's policy. One decision time is read per page so a policy window
// (quiet hours) cannot shift underneath a single page's evaluations.
func (w *PushWorker) fanOutPage(ctx context.Context, rows []store.WebhookEventRow, byTenant map[string]map[string]*pushRecipient) ([]store.PushNotificationBatchItem, error) {
	batch := make([]store.PushNotificationBatchItem, 0)
	decisionTime := w.now()
	evaluator := DeliveryPolicyEvaluator{Now: func() time.Time { return decisionTime }}
	var err error
	for _, row := range rows {
		// The feed query already carries the app id and the fact status, so
		// neither costs a per-row round trip. It also means an app deleted
		// between the read and a resolve can no longer poison the whole pass.
		serviceID := row.AppID
		factStatus := ""
		if row.Source == store.EventSourceFact && row.FactType == string(store.EventFactJobRunEnded) {
			factStatus = row.Status
		}
		projected, ok := projectPushEvent(row, serviceID, factStatus)
		if !ok {
			continue
		}
		batch, err = fanOutPush(batch, evaluator, byTenant[row.TenantID], pushTarget{
			workspaceID: row.TenantID, sourceKey: row.Key, serviceID: serviceID,
			resourceKind: "service", resourceID: serviceID,
			deepLink: "/services/" + serviceID, occurredAt: row.At, projected: projected,
		})
		if err != nil {
			return nil, err
		}
	}
	return batch, nil
}

func storedPushDeliveryPolicy(raw json.RawMessage) (DeliveryPolicy, error) {
	view := clonePushSettings(defaultPushSettings)
	if len(raw) != 0 {
		view = PushSettingsView{}
		if err := json.Unmarshal(raw, &view); err != nil {
			return DeliveryPolicy{}, fmt.Errorf("decode stored push policy")
		}
		dropRetiredDeliveryEvents(&view)
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

func projectPushEvent(row store.WebhookEventRow, serviceID, factStatus string) (projectedPush, bool) {
	serviceName := row.ServiceName
	if serviceName == "" {
		serviceName = serviceID
	}
	switch row.Source {
	case store.EventSourceDeploy:
		if row.Phase == store.EventPhaseStarted {
			return projectedPush{
				event: string(DeliveryEventDeployStarted), title: "Deploy started",
				body: serviceName + " deploy started.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		}
		if row.Phase != store.EventPhaseEnded {
			return projectedPush{}, false
		}
		switch row.Status {
		case store.DeployLive:
			return projectedPush{
				event: string(DeliveryEventDeploySucceeded), title: "Deploy succeeded",
				body: serviceName + " is live.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		case store.DeployBuildFailed, store.DeployPreDeployFailed, store.DeployUpdateFailed:
			return projectedPush{
				event: string(DeliveryEventDeployFailed), title: "Deploy failed",
				body: serviceName + " deploy failed.", urgency: string(DeliveryUrgencyImportant),
			}, true
		default:
			return projectedPush{}, false
		}
	case store.EventSourceFact:
		switch row.FactType {
		case string(store.EventFactServerFailed):
			return projectedPush{
				event: string(DeliveryEventServerFailed), title: "Service unavailable",
				body: serviceName + " is unavailable.", urgency: string(DeliveryUrgencyCritical),
			}, true
		case string(store.EventFactServerAvailable):
			return projectedPush{
				event: string(DeliveryEventServerAvailable), title: "Service recovered",
				body: serviceName + " recovered.", urgency: string(DeliveryUrgencyImportant),
			}, true
		case string(store.EventFactServiceSuspended):
			return projectedPush{
				event: string(DeliveryEventServiceSuspended), title: "Service suspended",
				body: serviceName + " was suspended.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		case string(store.EventFactServiceResumed):
			return projectedPush{
				event: string(DeliveryEventServiceResumed), title: "Service resumed",
				body: serviceName + " resumed.", urgency: string(DeliveryUrgencyRoutine),
			}, true
		case string(store.EventFactJobRunEnded):
			if factStatus != store.EventStatusFailed {
				return projectedPush{}, false
			}
			return projectedPush{
				event: string(DeliveryEventCronFailed), title: "Cron job failed",
				body: serviceName + " cron job failed.", urgency: string(DeliveryUrgencyImportant),
			}, true
		}
	}
	return projectedPush{}, false
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
				Subject: delivery.Subject, WorkspaceID: delivery.TenantID, SessionID: delivery.SessionID,
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

// classifyPushError maps the provider errors that mean "this destination is
// permanently unusable" onto their metric code, whether the delivery is
// terminal, and whether the subscription should be pruned. An unrecognized
// error returns an empty code so each caller applies its own transient default
// — the send path additionally distinguishes rate-limited from transient.
func classifyPushError(err error) (code string, terminal, prune bool) {
	var invalid *pushtransport.InvalidTokenError
	var payload *pushtransport.PayloadError
	var permanent *pushtransport.PermanentError
	switch {
	case errors.As(err, &invalid):
		return "invalid_token", true, true
	case errors.As(err, &payload):
		return "payload", true, false
	case errors.As(err, &permanent):
		return "permanent", true, false
	}
	return "", false, false
}

// prune revokes the exact destination behind a delivery that a provider has
// declared permanently unusable, and records the evidence + metric once.
func (w *PushWorker) prune(ctx context.Context, d store.DuePushDelivery, code string) {
	_, _ = w.Store.RevokeExactPushSubscription(ctx, d)
	w.evidence(ctx, "prune", "invalid_token", code)
	w.Metrics.Operation("prune", "invalid_token")
}

func (w *PushWorker) recordSendFailure(ctx context.Context, d store.DuePushDelivery, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		_, releaseErr := w.Store.ReleasePushDelivery(ctx, d)
		return releaseErr
	}
	delay := pushRetryDelay(d.AttemptCount + 1)
	code, terminal, prune := classifyPushError(err)
	if code == "" {
		code = "transient"
		var limited *pushtransport.RateLimitedError
		var transient *pushtransport.TransientError
		switch {
		case errors.As(err, &limited):
			code = "rate_limited"
			delay = boundedRetryAfter(limited.RetryAfter, delay)
		case errors.As(err, &transient):
			code = "transient"
		}
	}
	if d.AttemptCount+1 >= pushMaxAttempts {
		terminal = true
	}
	now := w.now()
	_, storeErr := w.Store.RecordPushSendFailure(ctx, d, code, now, now.Add(delay), terminal)
	if prune {
		w.prune(ctx, d, code)
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
		// A receipt we could not resolve this pass is rescheduled, metered as
		// "ambiguous" once its acceptance window has closed and the outcome can
		// no longer be settled, and as a plain retry until then.
		reschedule := func(code, outcome string) {
			_, e := w.Store.RecordPushReceipt(ctx, d, code, now, now.Add(pushReceiptDelay), false, false, ambiguous)
			failures = append(failures, e)
			if ambiguous {
				outcome = "ambiguous"
			}
			w.Metrics.Operation("receipt", outcome)
		}
		if checkErr != nil {
			reschedule("receipt_transient", "retry")
			continue
		}
		r, found := receipts[d.ProviderTicketID]
		if !found {
			reschedule("receipt_pending", "pending")
			continue
		}
		if r.Err == nil {
			_, e := w.Store.RecordPushReceipt(ctx, d, "", now, now, true, false, false)
			failures = append(failures, e)
			w.Metrics.Operation("receipt", "delivered")
			w.Metrics.Succeeded(now)
			continue
		}
		code, failed, prune := classifyPushError(r.Err)
		if code == "" {
			code = "receipt_transient"
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
			w.prune(ctx, d, code)
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
