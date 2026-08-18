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

package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/netutil"
)

// worker.go is the delivery side (w3/m11/t002+t003), two phases per tick:
//
//   - dispatch: tail the composed event feed (deploys + audit_events + typed
//     service_event_facts — the same rows the service feed derives from) through a durable
//     watermark, fan each event out to the workspace's subscribed endpoints
//     as webhook_deliveries rows. Insert + watermark advance are one
//     transaction, so a crash re-reads rather than drops.
//   - send: POST each due delivery (Standard-Webhooks signed, 15s timeout,
//     2xx = delivered), rescheduling failures on the backoff below. The 3rd
//     consecutive failure emails the endpoint's creator (once per endpoint
//     per outage window); exhausting all attempts disables the endpoint until
//     it is manually re-enabled — Render's documented semantics.
//
// Durability lives in Postgres (rows + watermark), so retries survive
// restarts and a deploy that happens while bex-api is down is still delivered
// once it is back. Multi-replica-safe (w1/m58): dispatch dedupes on the
// (endpoint_id, event_id) unique index (ON CONFLICT DO NOTHING) and send leases
// each row with FOR UPDATE SKIP LOCKED, so the two bex-api replicas (w1/m52)
// deliver each event exactly once and a crash mid-send re-delivers at-least-once
// (receivers dedupe on webhook-id) rather than dropping it.

// DefaultBackoff is the retry schedule between Render's eight total attempts:
// the initial attempt plus seven retries, with the last at 32h40m30s (Render
// documents it as approximately 33 hours after the first). Overridable
// (BEX_WEBHOOK_BACKOFF) so a live verification run can walk the whole path in
// seconds. An override of N delays likewise permits N+1 total attempts.
var DefaultBackoff = []time.Duration{
	30 * time.Second,
	10 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	5 * time.Hour,
	10 * time.Hour,
	15 * time.Hour,
}

const (
	// defaultPollInterval paces both phases — the "within seconds" delivery
	// budget, spent mostly here.
	defaultPollInterval = 2 * time.Second
	// dispatchLag is a property of the feed itself, shared with every other
	// tailer of it — see store.FeedCommitLag for why it exists.
	dispatchLag = store.FeedCommitLag
	// dispatchBatch caps one dispatch pass's read; a full batch loops
	// immediately rather than waiting a tick.
	dispatchBatch = 200
	// DefaultMaxDeliveriesPerWorkspace bounds one tenant's open logical
	// notifications. It is deliberately well above ordinary endpoint/event
	// volume; the ceiling is an abuse/failure containment boundary, not a
	// customer-facing product quota. BEX_MAX_WEBHOOK_DELIVERIES_PER_WORKSPACE=0
	// disables it explicitly.
	DefaultMaxDeliveriesPerWorkspace = 10000
	// sendBatch caps one send pass; senderConcurrency bounds parallel POSTs
	// (the notifications fan-out size).
	sendBatch         = 50
	senderConcurrency = 8
	// requestTimeout is Render's documented per-attempt budget: respond 2xx
	// within 15s or the attempt failed.
	requestTimeout = 15 * time.Second
	// claimLease is how long ClaimDueWebhookAttempts hides a claimed row from
	// other workers while this one POSTs it. It must exceed the WORST-CASE time a
	// row can wait inside one send pass before its own POST completes, not just a
	// single requestTimeout: a row claimed in the first wave may not be POSTed
	// until the last of ceil(sendBatch/senderConcurrency) concurrency waves, each
	// up to requestTimeout. A lease shorter than that whole window lets the other
	// replica re-claim a still-in-flight row and double-POST it — double-counting
	// the attempt and burning the retry schedule roughly twice as fast (scan
	// finding #3, adjacent bug). One wave of headroom above the worst case; a
	// crashed worker still releases the row after this window (at-least-once).
	sendWaves  = (sendBatch + senderConcurrency - 1) / senderConcurrency
	claimLease = (sendWaves + 1) * requestTimeout
	// emailAfterFailures is when the failure notice goes out (Render: "after 3
	// consecutive failures"); emailSuppression stops a burst of failing
	// deliveries to one endpoint from mailing per-delivery.
	emailAfterFailures = 3
	emailSuppression   = time.Hour
	// disabledReason marks the auto-disable path apart from a manual toggle.
	disabledReason = "disabled automatically after repeated delivery failures"
)

// parkInterval bounds how far the durable watermark may lag behind the read
// window before an otherwise-quiet dispatch pass persists it forward. Like
// dispatchLag it is a property of the shared feed, not of this worker — see
// store.DefaultFeedPark for the write-amplification trade it settles.
const parkInterval = store.DefaultFeedPark

// WorkerStore is the Worker's seam to the control-plane store —
// *store.PGStore satisfies it; a fake backs the tests.
type WorkerStore interface {
	EnsureWebhookWatermark(ctx context.Context, at time.Time) (time.Time, string, error)
	ListWebhookEvents(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]store.WebhookEventRow, error)
	ListEnabledWebhookEndpoints(ctx context.Context) ([]store.WebhookEndpoint, error)
	EnqueueWebhookDeliveries(ctx context.Context, deliveries []store.WebhookDelivery, at time.Time, key string, maxPerWorkspace int) (store.WebhookEnqueueResult, error)
	ClaimDueWebhookAttempts(ctx context.Context, now, leaseUntil time.Time, limit int) ([]store.DueWebhookAttempt, error)
	SweepWebhookDeliveries(ctx context.Context, before time.Time, keepPerEndpoint, limit int) (int64, error)
	CompleteWebhookAttempt(ctx context.Context, completion store.WebhookAttemptCompletion) (bool, error)
	ClaimWebhookFailureNotice(ctx context.Context, endpointID string, now, threshold time.Time) (bool, error)
	// SubjectIsWorkspaceAdmin answers whether subject still holds the admin
	// role in tenantID — the failure-notice recipient gate (round-14 #6).
	SubjectIsWorkspaceAdmin(ctx context.Context, tenantID, subject string) (bool, error)
}

// AttemptObserver receives only bounded attempt origin/outcome dimensions.
// A Prometheus implementation is shared with the authorized Resend service;
// endpoint, event, workspace, and caller identifiers are intentionally absent.
type AttemptObserver interface {
	ObserveWebhookAttempt(origin, result string)
}

// AdmissionObserver records only aggregate, bounded outcomes from queue
// admission. It receives no resource identifiers or tenant-controlled values.
type AdmissionObserver interface {
	ObserveWebhookAdmission(result store.WebhookEnqueueResult)
}

// Mailer sends the failure-notice email (text + optional HTML alternative) —
// the notifications feature's seam shape. nil => notices are logged, not mailed
// (BEX_SMTP_* unset; the w4/m7 graceful-skip pattern).
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// EmailLookup resolves a caller subject to a verified email address. nil (or
// a miss) => the notice for that endpoint is logged, not mailed.
type EmailLookup interface {
	LookupEmail(ctx context.Context, subject string) (string, bool)
}

// Worker runs the dispatcher + sender. Zero-value fields take the defaults
// above; only Store is required.
type Worker struct {
	Store  WorkerStore
	Mailer Mailer
	Emails EmailLookup
	// Backoff is the retry schedule (len = max retries after the initial
	// attempt, so total attempts = len+1); nil => DefaultBackoff.
	Backoff []time.Duration
	// PollInterval paces ticks; 0 => defaultPollInterval.
	PollInterval time.Duration
	// Clock is the test seam; nil => time.Now.
	Clock func() time.Time
	// Client posts deliveries; nil => defaultClient.
	Client *http.Client
	// Attempts records one result only after the store wins the terminalization
	// CAS. nil disables metrics without affecting delivery.
	Attempts AttemptObserver
	// Admissions records aggregate admitted/capped/deduplicated counts after the
	// enqueue transaction commits. nil disables metrics without affecting the
	// queue. MaxDeliveriesPerWorkspace == 0 disables the ceiling.
	Admissions                AdmissionObserver
	MaxDeliveriesPerWorkspace int
	// RetentionDays is how long a TERMINAL delivery survives before the sweep
	// purges it (BEX_WEBHOOK_RETENTION_DAYS). <1 => DefaultRetentionDays.
	RetentionDays int
	// RetentionKeepPerEndpoint caps how many terminal deliveries an endpoint may
	// retain regardless of age, so a burst inside the age window cannot evade the
	// age rule. <1 => defaultRetentionKeepPerEndpoint.
	RetentionKeepPerEndpoint int

	// cursor caches the durable watermark between ticks to skip an I/O round trip
	// on a quiet tick. It is a pure optimization under two replicas: the
	// (endpoint_id, event_id) unique index (w1/m58) dedupes any delivery a stale
	// cache re-reads, so a lagging cache converges rather than duplicates. Loaded
	// once, advanced by every successful EnqueueWebhookDeliveries.
	cursor store.FeedCursor

	// lastSweep throttles retention to one pass per sweepInterval, so the sweep
	// rides the existing tick instead of needing its own goroutine.
	lastSweep time.Time
}

const (
	// DefaultRetentionDays is how long a delivered/exhausted delivery row is kept
	// for the dashboard's history view. Matches the audit log's 90-day default
	// (BEX_AUDIT_RETENTION_DAYS) — the two are read side by side.
	DefaultRetentionDays = 90
	// defaultRetentionKeepPerEndpoint bounds an endpoint's terminal history
	// independently of age.
	defaultRetentionKeepPerEndpoint = 1000
	// sweepInterval is how often retention runs; sweepBatch bounds one pass so a
	// large backlog is drained over several ticks instead of one long statement.
	sweepInterval = time.Hour
	sweepBatch    = 2000
)

func (w *Worker) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

func (w *Worker) backoff() []time.Duration {
	if len(w.Backoff) > 0 {
		return w.Backoff
	}
	return DefaultBackoff
}

// defaultClient is the production HTTP client — one shared instance (its
// transport pools connections), not a per-attempt construction. The SSRF guard
// blocks loopback/private/link-local destinations at dial time so a tenant
// cannot register a webhook endpoint that probes cloud metadata or internal
// services. Redirects are never followed (a 3xx is treated as a delivery
// failure) so a redirect-to-private chain cannot bypass the dial guard.
var defaultClient = &http.Client{
	Timeout:   requestTimeout,
	Transport: deliveryTransport(),
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// deliveryTransport builds the delivery transport. Named (rather than inlined
// into defaultClient) so a test can assert the real production construction.
func deliveryTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = netutil.SafeDialContext(requestTimeout)
	// No ambient proxy (w1/m66 F9). This client's entire destination policy is
	// implemented at dial time, and Clone() carries DefaultTransport's
	// ProxyFromEnvironment: with HTTP(S)_PROXY set, SafeDialContext would only
	// ever see the PROXY's address while the proxy resolved and fetched the
	// tenant-controlled URL — the private/link-local/metadata guard silently
	// bypassed. A dial-time guard and an ambient proxy are incompatible.
	tr.Proxy = nil
	return tr
}

func (w *Worker) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return defaultClient
}

// Run ticks the worker until ctx is done.
func (w *Worker) Run(ctx context.Context) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	core.Poll(ctx, "webhooks", interval, w.RunOnce)
}

// RunOnce drives one dispatch pass then one send pass — exported so tests and
// the verification harness can step the worker deterministically.
//
// The enabled-endpoint set is read once and shared: with NO enabled endpoints
// (every deployment that never touches webhooks) a tick costs exactly this
// one indexed SELECT — no feed read, no watermark write, no due-delivery
// query (deliveries join enabled endpoints, so none can be due).
func (w *Worker) RunOnce(ctx context.Context) error {
	// Retention runs before the early return: a workspace that deletes its last
	// endpoint must still have its terminal history reclaimed, and the sweep is
	// self-throttling (at most one pass per sweepInterval).
	sweepErr := w.sweepRetention(ctx)
	endpoints, err := w.Store.ListEnabledWebhookEndpoints(ctx)
	if err != nil {
		return errors.Join(sweepErr, fmt.Errorf("list endpoints: %w", err))
	}
	if len(endpoints) == 0 {
		return sweepErr
	}
	return errors.Join(sweepErr, w.dispatch(ctx, endpoints), w.send(ctx))
}

// sweepRetention purges terminal deliveries past the retention policy, at most
// once per sweepInterval (w1/m67 F3). The delivery table doubles as the
// dashboard's history surface, so without this a tenant's ordinary activity grew
// shared storage forever — the only thing that ever removed a row was deleting
// the endpoint or the workspace.
func (w *Worker) sweepRetention(ctx context.Context) error {
	days := w.RetentionDays
	if days <= 0 {
		days = DefaultRetentionDays
	}
	keep := w.RetentionKeepPerEndpoint
	if keep <= 0 {
		keep = defaultRetentionKeepPerEndpoint
	}
	now := w.now()
	if !w.lastSweep.IsZero() && now.Sub(w.lastSweep) < sweepInterval {
		return nil
	}
	w.lastSweep = now
	n, err := w.Store.SweepWebhookDeliveries(ctx, now.AddDate(0, 0, -days), keep, sweepBatch)
	if err != nil {
		return fmt.Errorf("sweep webhook deliveries: %w", err)
	}
	if n > 0 {
		log.Printf("webhooks: purged %d terminal deliveries older than %dd or beyond %d per endpoint", n, days, keep)
	}
	return nil
}

// payload is the thin body a receiver gets — Render's documented shape:
// nothing but the event type, when, and which service; details are fetched
// back through the API, never pushed.
type payload struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      payloadData `json:"data"`
}

type payloadData struct {
	ID          string `json:"id"`
	ServiceID   string `json:"serviceId"`
	ServiceName string `json:"serviceName"`
	// Status is a checked terminal action outcome (succeeded|failed|canceled);
	// omitted for nonterminal event types.
	Status string `json:"status,omitempty"`
}

// dispatch advances the watermark through the composed feed, fanning each
// event out to its workspace's subscribed endpoints.
//
// The watermark deliberately does NOT chase the clock while nothing is
// subscribed (RunOnce returns before dispatch runs) — no-replay for a
// later-registered endpoint is enforced where it belongs, per endpoint: the
// fan-out below skips any event older than the endpoint's own CreatedAt, and
// the read floor starts at the OLDEST enabled endpoint's creation, so a long
// endpoint-less stretch is skipped outright rather than paged through.
func (w *Worker) dispatch(ctx context.Context, endpoints []store.WebhookEndpoint) error {
	until := w.now().Add(-dispatchLag)
	if err := w.cursor.Load(ctx, w.Store.EnsureWebhookWatermark, until); err != nil {
		return fmt.Errorf("watermark: %w", err)
	}
	byTenant := make(map[string][]store.WebhookEndpoint)
	oldest := endpoints[0].CreatedAt
	for _, e := range endpoints {
		byTenant[e.TenantID] = append(byTenant[e.TenantID], e)
		if e.CreatedAt.Before(oldest) {
			oldest = e.CreatedAt
		}
	}
	var capped int
	err := store.TailFeed(ctx, &w.cursor, store.FeedPass[store.WebhookDelivery]{
		Until: until,
		// Nothing from before the oldest enabled endpoint existed can be delivered
		// (the per-endpoint guard in fanOut), so the read may start there when the
		// durable watermark is older.
		Floor:   oldest,
		Verbs:   auditVerbs,
		Tenants: slices.Sorted(maps.Keys(byTenant)),
		Limit:   dispatchBatch,
		Park:    parkInterval,
		List:    w.Store.ListWebhookEvents,
		Commit: func(ctx context.Context, deliveries []store.WebhookDelivery, at time.Time, key string) error {
			result, err := w.Store.EnqueueWebhookDeliveries(
				ctx, deliveries, at, key, w.MaxDeliveriesPerWorkspace,
			)
			if err != nil {
				return err
			}
			if w.Admissions != nil {
				w.Admissions.ObserveWebhookAdmission(result)
			}
			capped += result.Capped
			return nil
		},
		Project: func(_ context.Context, rows []store.WebhookEventRow) ([]store.WebhookDelivery, error) {
			return w.fanOutPage(rows, byTenant)
		},
	})
	if capped > 0 {
		// One aggregate line per dispatch pass, even when TailFeed loops through
		// several full pages. No workspace/endpoint/event/URL/payload leaves the
		// store result, so sustained pressure cannot amplify logs or leak tenants.
		log.Printf("webhooks: capped %d deliveries at the per-workspace open-backlog limit %d; source events were committed and will not replay", capped, w.MaxDeliveriesPerWorkspace)
	}
	return err
}

// fanOutPage turns one page of the feed into deliveries: every subscribed
// endpoint of the event's workspace that both wants this event type and already
// existed when it happened.
func (w *Worker) fanOutPage(rows []store.WebhookEventRow, byTenant map[string][]store.WebhookEndpoint) ([]store.WebhookDelivery, error) {
	var batch []store.WebhookDelivery
	now := w.now()
	for _, r := range rows {
		eventType, data, ok := project(r)
		if !ok {
			continue
		}
		// Marshaled lazily, on the first subscriber, and kept as the string the
		// delivery row wants so N endpoints share one copy rather than N.
		var body string
		for _, e := range byTenant[r.TenantID] {
			// CreatedAt guard: an endpoint never receives events from before
			// it existed, however far back the watermark was when it appeared.
			if r.At.Before(e.CreatedAt) || (len(e.EventTypes) > 0 && !slices.Contains(e.EventTypes, eventType)) {
				continue
			}
			if body == "" {
				raw, err := json.Marshal(payload{Type: eventType, Timestamp: r.At.UTC().Format(time.RFC3339), Data: data})
				if err != nil {
					return nil, err
				}
				body = string(raw)
			}
			batch = append(batch, store.WebhookDelivery{
				ID:            ids.New(ids.WebhookDelivery),
				EndpointID:    e.ID,
				EventID:       data.ID,
				EventType:     eventType,
				ServiceID:     data.ServiceID,
				Payload:       body,
				NextAttemptAt: now,
			})
		}
	}
	return batch, nil
}

// project maps one composed feed row onto the webhook vocabulary. The event
// id is DERIVED from the row key (ids.Derive) — the identical id the
// service-events feed shows for the same transition, so a receiver can
// correlate a webhook with GET /v1/services/{id}/events, and every retry
// carries the same webhook-id for the receiver to dedupe on.
func project(r store.WebhookEventRow) (string, payloadData, bool) {
	serviceID := r.ServiceID
	if r.AppID != "" {
		serviceID = r.AppID
	}
	data := payloadData{
		ID:          ids.Derive(ids.Event, r.Key),
		ServiceID:   serviceID,
		ServiceName: r.ServiceName,
	}
	switch r.Source {
	case store.EventSourceDeploy:
		if r.Phase == store.EventPhaseStarted {
			return TypeDeployStarted, data, true
		}
		data.Status = store.RenderDeployStatus(r.Status)
		return TypeDeployEnded, data, true
	case store.EventSourceAudit:
		if r.Verb == autoDeployVerb {
			if r.AutoDeployEnabled == nil {
				return "", payloadData{}, false
			}
			if *r.AutoDeployEnabled {
				return TypeAutoDeployEnabled, data, true
			}
			return TypeAutoDeployDisabled, data, true
		}
		t, ok := verbEvents[r.Verb]
		return t, data, ok
	case store.EventSourceFact:
		t, ok := factEvents[r.FactType]
		if !ok {
			return "", payloadData{}, false
		}
		switch t {
		case TypeBuildEnded, TypePreDeployEnded, TypeJobRunEnded, TypeCronJobRunEnded:
			data.Status = r.Status
		}
		return t, data, ok
	}
	return "", payloadData{}, false
}

// send claims every due delivery and POSTs it, senderConcurrency at a time. The
// claim (SKIP LOCKED + lease) is what makes two replicas safe: each send pass
// leases a disjoint batch, so no event is POSTed twice concurrently.
func (w *Worker) send(ctx context.Context) error {
	now := w.now()
	due, err := w.Store.ClaimDueWebhookAttempts(ctx, now, now.Add(claimLease), sendBatch)
	if err != nil {
		return fmt.Errorf("claim due deliveries: %w", err)
	}
	sem := make(chan struct{}, senderConcurrency)
	var wg sync.WaitGroup
	for _, d := range due {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			w.attempt(ctx, d)
		}()
	}
	wg.Wait()
	return nil
}

// attempt makes one POST and books its outcome: delivered, rescheduled on
// the backoff, or — after the schedule is exhausted — failed + endpoint
// disabled. Booking errors are logged, never returned: the next due-poll
// simply retries the row.
func (w *Worker) attempt(ctx context.Context, d store.DueWebhookAttempt) {
	at := w.now()
	status, responseBody, err := w.post(ctx, d, at)
	completion := store.WebhookAttemptCompletion{
		AttemptID: d.ID, StatusCode: status, ResponseBody: responseBody,
		CompletedAt: at, NextAttemptAt: at,
	}
	if err == nil {
		completion.Delivered = true
		completed, bookErr := w.Store.CompleteWebhookAttempt(ctx, completion)
		if bookErr != nil {
			log.Printf("webhooks: complete attempt %s: %v", d.ID, bookErr)
		} else if completed {
			w.observeAttempt(d.Origin, store.WebhookAttemptDelivered)
		}
		return
	}
	errMsg := SanitizeDeliveryError(err)
	completion.TransportError = errMsg
	if d.Origin == store.WebhookAttemptManual {
		if d.ResumeAutomaticAt != nil {
			completion.NextAttemptID = ids.New(ids.WebhookDelivery)
		}
		completed, bookErr := w.Store.CompleteWebhookAttempt(ctx, completion)
		if bookErr != nil {
			log.Printf("webhooks: complete manual attempt %s: %v", d.ID, bookErr)
		} else if completed {
			w.observeAttempt(d.Origin, store.WebhookAttemptFailed)
		}
		return
	}

	attempt := d.AutomaticAttemptCount + 1
	backoff := w.backoff()
	if attempt > len(backoff) {
		// Out of retries: close the row and disable the endpoint until a human
		// re-enables it (Render's documented endgame).
		completion.Exhausted = true
		completion.DisableReason = disabledReason
		completed, bookErr := w.Store.CompleteWebhookAttempt(ctx, completion)
		if bookErr != nil {
			log.Printf("webhooks: complete attempt %s: %v", d.ID, bookErr)
			return
		}
		if completed {
			w.observeAttempt(d.Origin, store.WebhookAttemptFailed)
			d.TransportError = errMsg
			w.notifyFailure(ctx, d, true)
		}
		return
	}
	next := at.Add(backoff[attempt-1])
	completion.NextAttemptID = ids.New(ids.WebhookDelivery)
	completion.NextAttemptAt = next
	completed, bookErr := w.Store.CompleteWebhookAttempt(ctx, completion)
	if bookErr != nil {
		log.Printf("webhooks: complete attempt %s: %v", d.ID, bookErr)
		return
	}
	if completed {
		w.observeAttempt(d.Origin, store.WebhookAttemptFailed)
	}
	if completed && attempt == emailAfterFailures {
		d.TransportError = errMsg
		w.notifyFailure(ctx, d, false)
	}
}

func (w *Worker) observeAttempt(origin, result string) {
	if w.Attempts != nil {
		w.Attempts.ObserveWebhookAttempt(origin, result)
	}
}

// post makes the signed POST. A non-2xx response or transport error is the
// attempt's failure; the returned status is 0 on a transport error.
func (w *Worker) post(ctx context.Context, d store.DueWebhookAttempt, at time.Time) (int, string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	payload := []byte(d.Payload) // one copy, shared by the body and the signature
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", d.EventID)
	req.Header.Set("webhook-timestamp", fmt.Sprintf("%d", at.Unix()))
	req.Header.Set("webhook-signature", Sign(d.Secret, d.EventID, at, payload))
	resp, err := w.client().Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	responseBody, readErr := readResponseEvidence(resp.Body)
	if readErr != nil {
		return resp.StatusCode, responseBody, fmt.Errorf("read endpoint response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, responseBody, fmt.Errorf("endpoint answered %d", resp.StatusCode)
	}
	return resp.StatusCode, responseBody, nil
}

func boundUTF8(value string, limit int) string {
	clean := bytes.ToValidUTF8([]byte(value), []byte("\uFFFD"))
	if len(clean) <= limit {
		return string(clean)
	}
	clean = clean[:limit]
	for len(clean) > 0 && !utf8.Valid(clean) {
		clean = clean[:len(clean)-1]
	}
	return string(clean)
}

const (
	maxWebhookResponseBytes = 4096
	responseTruncatedSuffix = "\n[bex: response truncated]"
)

// readResponseEvidence retains a UTF-8-safe prefix of the endpoint response.
// It reads one byte past the cap to detect truncation, and the marker itself is
// included inside the 4096-byte storage/wire bound.
func readResponseEvidence(r io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxWebhookResponseBytes+1))
	truncated := len(raw) > maxWebhookResponseBytes
	budget := maxWebhookResponseBytes
	if truncated {
		budget -= len(responseTruncatedSuffix)
	}
	clean := bytes.ToValidUTF8(raw, []byte("\uFFFD"))
	if len(clean) > budget {
		clean = clean[:budget]
		for len(clean) > 0 && !utf8.Valid(clean) {
			clean = clean[:len(clean)-1]
		}
	}
	if truncated {
		clean = append(clean, responseTruncatedSuffix...)
	}
	return string(clean), err
}

// notifyFailure emails the endpoint's creator that deliveries are failing
// (final=false, the 3rd consecutive failure) or that the endpoint was
// disabled (final=true). Best-effort and suppressed per endpoint for
// emailSuppression, except that the final disable notice always goes out.
// With no mailer, no lookup, or no resolvable address it logs instead — the
// notifications feature's degrade-quietly convention.
//
// Round-14 #6 tightened two properties of the channel: the body never carries
// the exact destination (email is an unauthenticated channel, and the URL may
// be a capability another admin configured AFTER this creator's involvement —
// it degrades to the same redacted origin non-admin reads see), and the
// recipient is resolved from CURRENT authorization state: created_by is
// immutable provenance, so a removed or demoted creator must not receive a
// later administrator's capability. The creator must still be a workspace
// admin at send time; anything else (or an unanswerable check) skips the
// notice, fail-closed.
func (w *Worker) notifyFailure(ctx context.Context, d store.DueWebhookAttempt, final bool) {
	if admin, err := w.Store.SubjectIsWorkspaceAdmin(ctx, d.TenantID, d.CreatedBy); err != nil {
		log.Printf("webhooks: failure notice for %s skipped: creator membership check failed: %v", d.EndpointID, err)
		return
	} else if !admin {
		log.Printf("webhooks: failure notice for %s skipped: creator %s is no longer a workspace admin", d.EndpointID, d.CreatedBy)
		return
	}
	// The 3rd-failure notice is suppressed to one per endpoint per window via a
	// durable compare-and-set, so a restart or a second replica cannot re-send it
	// (w1/m58). The final disable notice is not suppressed here — only one replica
	// drives a delivery to exhaustion (it holds the SKIP LOCKED claim), so it is
	// already sent once. Checked AFTER the membership gate so a skipped notice
	// does not consume the suppression window.
	if !final {
		now := w.now()
		claimed, err := w.Store.ClaimWebhookFailureNotice(ctx, d.EndpointID, now, now.Add(-emailSuppression))
		if err != nil {
			log.Printf("webhooks: claim failure notice for %s: %v", d.EndpointID, err)
			return
		}
		if !claimed {
			return
		}
	}
	subject := fmt.Sprintf("[bex] webhook %q is failing to deliver", d.EndpointName)
	// Never the exact destination in the body (round-14 #6): the redacted
	// origin + the endpoint's name/id is enough to act on from an email, and
	// the exact URL stays behind the authenticated admin surface.
	dest := RedactedURL(d.URL)
	msg := email.Message{
		Title: "Webhook delivery failing",
		Paragraphs: []string{
			fmt.Sprintf("Deliveries to your webhook %q (%s) have failed %d times in a row.", d.EndpointName, dest, emailAfterFailures),
			fmt.Sprintf("Last error: %s", scrubDeliveryEvidence(d.TransportError, d.URL)),
			"bex will keep retrying on an exponential backoff.",
		},
	}
	if final {
		subject = fmt.Sprintf("[bex] webhook %q was disabled after repeated failures", d.EndpointName)
		msg = email.Message{
			Title: "Webhook disabled",
			Paragraphs: []string{
				fmt.Sprintf("Deliveries to your webhook %q (%s) kept failing after every retry, and the endpoint has been disabled.", d.EndpointName, dest),
				"No further events will be sent until you re-enable it from the dashboard or API.",
			},
		}
	}
	if w.Mailer == nil || w.Emails == nil {
		log.Printf("webhooks: %s (no SMTP relay configured; notice not emailed)", subject)
		return
	}
	to, ok := w.Emails.LookupEmail(ctx, d.CreatedBy)
	if !ok || to == "" {
		log.Printf("webhooks: %s (no email address for %s; notice not emailed)", subject, d.CreatedBy)
		return
	}
	if err := w.Mailer.Send(ctx, to, subject, msg.Text(), msg.HTML()); err != nil {
		log.Printf("webhooks: email %s: %v", to, err)
	}
}

// ParseBackoff parses BEX_WEBHOOK_BACKOFF — a comma-separated list of Go
// durations ("5s,10s,1m") overriding DefaultBackoff, which is how the live
// verification harness (w3/m11/t008) walks the retry/auto-disable path in
// seconds instead of 33 hours. Empty => nil (use the default); malformed =>
// an error, never a silently shortened schedule.
func ParseBackoff(s string) ([]time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]time.Duration, 0, len(parts))
	for _, p := range parts {
		d, err := time.ParseDuration(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("BEX_WEBHOOK_BACKOFF: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}
