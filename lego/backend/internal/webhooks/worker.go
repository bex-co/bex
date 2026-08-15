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

// DefaultBackoff is the retry schedule after each failed attempt: 8 retries,
// exponential, the last ~33h after the first attempt — Render's documented
// envelope ("up to 8 times", "final retry ~33 hours after the first
// attempt"). Overridable (BEX_WEBHOOK_BACKOFF) so a live verification run
// can walk the whole path in seconds.
var DefaultBackoff = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
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
	// sendBatch caps one send pass; senderConcurrency bounds parallel POSTs
	// (the notifications fan-out size).
	sendBatch         = 50
	senderConcurrency = 8
	// requestTimeout is Render's documented per-attempt budget: respond 2xx
	// within 15s or the attempt failed.
	requestTimeout = 15 * time.Second
	// claimLease is how long ClaimDueWebhookDeliveries hides a claimed row from
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
// window before an otherwise-quiet dispatch pass persists it forward. Parking
// per-tick would be a Postgres write transaction every 2s forever; parking
// per-parkInterval makes a quiet-but-subscribed platform cost one small write
// a minute, and a restart re-read at most a minute of already-empty window.
const parkInterval = time.Minute

// WorkerStore is the Worker's seam to the control-plane store —
// *store.PGStore satisfies it; a fake backs the tests.
type WorkerStore interface {
	EnsureWebhookWatermark(ctx context.Context, at time.Time) (time.Time, string, error)
	ListWebhookEvents(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]store.WebhookEventRow, error)
	ListEnabledWebhookEndpoints(ctx context.Context) ([]store.WebhookEndpoint, error)
	EnqueueWebhookDeliveries(ctx context.Context, deliveries []store.WebhookDelivery, at time.Time, key string) error
	ClaimDueWebhookDeliveries(ctx context.Context, now, leaseUntil time.Time, limit int) ([]store.DueWebhookDelivery, error)
	SweepWebhookDeliveries(ctx context.Context, before time.Time, keepPerEndpoint, limit int) (int64, error)
	RecordWebhookAttempt(ctx context.Context, id string, status int, errMsg string, at, next time.Time, delivered, failed bool) error
	DisableWebhookEndpoint(ctx context.Context, id, reason string) error
	ClaimWebhookFailureNotice(ctx context.Context, endpointID string, now, threshold time.Time) (bool, error)
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
	// Backoff is the retry schedule (len = max retries after the first
	// attempt); nil => DefaultBackoff.
	Backoff []time.Duration
	// PollInterval paces ticks; 0 => defaultPollInterval.
	PollInterval time.Duration
	// Clock is the test seam; nil => time.Now.
	Clock func() time.Time
	// Client posts deliveries; nil => defaultClient.
	Client *http.Client
	// RetentionDays is how long a TERMINAL delivery survives before the sweep
	// purges it (BEX_WEBHOOK_RETENTION_DAYS). <1 => DefaultRetentionDays.
	RetentionDays int
	// RetentionKeepPerEndpoint caps how many terminal deliveries an endpoint may
	// retain regardless of age, so a burst inside the age window cannot evade the
	// age rule. <1 => defaultRetentionKeepPerEndpoint.
	RetentionKeepPerEndpoint int

	// wm* cache the durable watermark between ticks to skip an I/O round trip on
	// a quiet tick. It is a pure optimization under two replicas: the
	// (endpoint_id, event_id) unique index (w1/m58) dedupes any delivery a stale
	// cache re-reads, so a lagging cache converges rather than duplicates. Loaded
	// once, updated after every successful EnqueueWebhookDeliveries.
	wmLoaded bool
	wmAt     time.Time
	wmKey    string

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
	// Status is deploy_ended's outcome (succeeded|failed|canceled); omitted
	// for every other type.
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
	if !w.wmLoaded {
		wmAt, wmKey, err := w.Store.EnsureWebhookWatermark(ctx, until)
		if err != nil {
			return fmt.Errorf("watermark: %w", err)
		}
		w.wmAt, w.wmKey, w.wmLoaded = wmAt, wmKey, true
	}
	byTenant := make(map[string][]store.WebhookEndpoint)
	oldest := endpoints[0].CreatedAt
	for _, e := range endpoints {
		byTenant[e.TenantID] = append(byTenant[e.TenantID], e)
		if e.CreatedAt.Before(oldest) {
			oldest = e.CreatedAt
		}
	}
	tenants := slices.Sorted(maps.Keys(byTenant))
	// Nothing before the oldest enabled endpoint existed can be delivered
	// (the per-endpoint guard below), so the read may start there when the
	// durable watermark is older — a read-side skip, nothing persisted.
	floorAt, floorKey := w.wmAt, w.wmKey
	if oldest.After(floorAt) {
		floorAt, floorKey = oldest, ""
	}
	for {
		rows, err := w.Store.ListWebhookEvents(ctx, floorAt, floorKey, until, auditVerbs, tenants, dispatchBatch)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			// Quiet window. Persist the watermark forward only once it lags by
			// parkInterval — bounding both the per-tick write load and how much
			// empty window a restart re-reads.
			if until.Sub(w.wmAt) > parkInterval {
				if err := w.Store.EnqueueWebhookDeliveries(ctx, nil, until, ""); err != nil {
					return err
				}
				w.wmAt, w.wmKey = until, ""
			}
			return nil
		}
		var batch []store.WebhookDelivery
		now := w.now()
		for _, r := range rows {
			eventType, data, ok := project(r)
			if !ok {
				continue
			}
			var body []byte // marshaled lazily, on the first subscriber
			for _, e := range byTenant[r.TenantID] {
				// CreatedAt guard: an endpoint never receives events from before
				// it existed, however far back the watermark was when it appeared.
				if r.At.Before(e.CreatedAt) || !slices.Contains(e.EventTypes, eventType) {
					continue
				}
				if body == nil {
					if body, err = json.Marshal(payload{Type: eventType, Timestamp: r.At.UTC().Format(time.RFC3339), Data: data}); err != nil {
						return err
					}
				}
				batch = append(batch, store.WebhookDelivery{
					ID:            ids.New(ids.WebhookDelivery),
					EndpointID:    e.ID,
					EventID:       data.ID,
					EventType:     eventType,
					ServiceID:     r.ServiceID,
					Payload:       string(body),
					NextAttemptAt: now,
				})
			}
		}
		last := rows[len(rows)-1]
		if err := w.Store.EnqueueWebhookDeliveries(ctx, batch, last.CursorAt, last.Key); err != nil {
			return err
		}
		w.wmAt, w.wmKey = last.CursorAt, last.Key
		floorAt, floorKey = last.CursorAt, last.Key
		if len(rows) < dispatchBatch {
			return nil
		}
	}
}

// project maps one composed feed row onto the webhook vocabulary. The event
// id is DERIVED from the row key (ids.Derive) — the identical id the
// service-events feed shows for the same transition, so a receiver can
// correlate a webhook with GET /v1/services/{id}/events, and every retry
// carries the same webhook-id for the receiver to dedupe on.
func project(r store.WebhookEventRow) (string, payloadData, bool) {
	data := payloadData{
		ID:          ids.Derive(ids.Event, r.Key),
		ServiceID:   r.ServiceID,
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
		t, ok := verbEvents[r.Verb]
		return t, data, ok
	case store.EventSourceFact:
		t, ok := factEvents[r.FactType]
		return t, data, ok
	}
	return "", payloadData{}, false
}

// send claims every due delivery and POSTs it, senderConcurrency at a time. The
// claim (SKIP LOCKED + lease) is what makes two replicas safe: each send pass
// leases a disjoint batch, so no event is POSTed twice concurrently.
func (w *Worker) send(ctx context.Context) error {
	now := w.now()
	due, err := w.Store.ClaimDueWebhookDeliveries(ctx, now, now.Add(claimLease), sendBatch)
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
func (w *Worker) attempt(ctx context.Context, d store.DueWebhookDelivery) {
	at := w.now()
	status, err := w.post(ctx, d, at)
	if err == nil {
		if bookErr := w.Store.RecordWebhookAttempt(ctx, d.ID, status, "", at, d.NextAttemptAt, true, false); bookErr != nil {
			log.Printf("webhooks: record delivery %s: %v", d.ID, bookErr)
		}
		return
	}
	errMsg := err.Error()
	attempt := d.AttemptCount + 1
	backoff := w.backoff()
	if attempt > len(backoff) {
		// Out of retries: close the row and disable the endpoint until a human
		// re-enables it (Render's documented endgame).
		if bookErr := w.Store.RecordWebhookAttempt(ctx, d.ID, status, errMsg, at, at, false, true); bookErr != nil {
			log.Printf("webhooks: record delivery %s: %v", d.ID, bookErr)
		}
		if disErr := w.Store.DisableWebhookEndpoint(ctx, d.EndpointID, disabledReason); disErr != nil {
			log.Printf("webhooks: disable endpoint %s: %v", d.EndpointID, disErr)
		}
		w.notifyFailure(ctx, d, true)
		return
	}
	next := at.Add(backoff[attempt-1])
	if bookErr := w.Store.RecordWebhookAttempt(ctx, d.ID, status, errMsg, at, next, false, false); bookErr != nil {
		log.Printf("webhooks: record delivery %s: %v", d.ID, bookErr)
	}
	if attempt == emailAfterFailures {
		w.notifyFailure(ctx, d, false)
	}
}

// post makes the signed POST. A non-2xx response or transport error is the
// attempt's failure; the returned status is 0 on a transport error.
func (w *Worker) post(ctx context.Context, d store.DueWebhookDelivery, at time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewReader([]byte(d.Payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", d.EventID)
	req.Header.Set("webhook-timestamp", fmt.Sprintf("%d", at.Unix()))
	req.Header.Set("webhook-signature", Sign(d.Secret, d.EventID, at, []byte(d.Payload)))
	resp, err := w.client().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("endpoint answered %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// notifyFailure emails the endpoint's creator that deliveries are failing
// (final=false, the 3rd consecutive failure) or that the endpoint was
// disabled (final=true). Best-effort and suppressed per endpoint for
// emailSuppression, except that the final disable notice always goes out.
// With no mailer, no lookup, or no resolvable address it logs instead — the
// notifications feature's degrade-quietly convention.
func (w *Worker) notifyFailure(ctx context.Context, d store.DueWebhookDelivery, final bool) {
	// The 3rd-failure notice is suppressed to one per endpoint per window via a
	// durable compare-and-set, so a restart or a second replica cannot re-send it
	// (w1/m58). The final disable notice is not suppressed here — only one replica
	// drives a delivery to exhaustion (it holds the SKIP LOCKED claim), so it is
	// already sent once.
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
	msg := email.Message{
		Title: "Webhook delivery failing",
		Paragraphs: []string{
			fmt.Sprintf("Deliveries to your webhook %q (%s) have failed %d times in a row.", d.EndpointName, d.URL, emailAfterFailures),
			fmt.Sprintf("Last error: %s", d.LastError),
			"bex will keep retrying on an exponential backoff.",
		},
	}
	if final {
		subject = fmt.Sprintf("[bex] webhook %q was disabled after repeated failures", d.EndpointName)
		msg = email.Message{
			Title: "Webhook disabled",
			Paragraphs: []string{
				fmt.Sprintf("Deliveries to your webhook %q (%s) kept failing after every retry, and the endpoint has been disabled.", d.EndpointName, d.URL),
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
