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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeWorkerStore is an in-memory WorkerStore: a fixed event feed, an
// endpoint table, and the delivery queue the worker mutates.
type fakeWorkerStore struct {
	mu        sync.Mutex
	events    []store.WebhookEventRow
	endpoints []store.WebhookEndpoint

	wmSeeded   bool
	wmAt       time.Time
	wmKey      string
	queue      map[string]*store.WebhookDelivery
	disabled   map[string]string // endpoint id -> reason
	queueOrder []string
}

func newFakeWorkerStore() *fakeWorkerStore {
	return &fakeWorkerStore{queue: map[string]*store.WebhookDelivery{}, disabled: map[string]string{}}
}

func (f *fakeWorkerStore) EnsureWebhookWatermark(_ context.Context, at time.Time) (time.Time, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.wmSeeded {
		f.wmSeeded, f.wmAt, f.wmKey = true, at, ""
	}
	return f.wmAt, f.wmKey, nil
}

func (f *fakeWorkerStore) ListWebhookEvents(_ context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]store.WebhookEventRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.WebhookEventRow
	for _, r := range f.events {
		afterWM := r.At.After(afterAt) || (r.At.Equal(afterAt) && r.Key > afterKey)
		if !afterWM || r.At.After(until) || !slices.Contains(tenants, r.TenantID) {
			continue
		}
		if r.Source == store.EventSourceAudit && !slices.Contains(verbs, r.Verb) {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeWorkerStore) ListEnabledWebhookEndpoints(context.Context) ([]store.WebhookEndpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.WebhookEndpoint
	for _, e := range f.endpoints {
		if _, off := f.disabled[e.ID]; e.Enabled && !off {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeWorkerStore) EnqueueWebhookDeliveries(_ context.Context, deliveries []store.WebhookDelivery, at time.Time, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range deliveries {
		cp := d
		f.queue[d.ID] = &cp
		f.queueOrder = append(f.queueOrder, d.ID)
	}
	f.wmAt, f.wmKey = at, key
	return nil
}

func (f *fakeWorkerStore) DueWebhookDeliveries(_ context.Context, now time.Time, limit int) ([]store.DueWebhookDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.DueWebhookDelivery
	for _, id := range f.queueOrder {
		d := f.queue[id]
		if d.DeliveredAt != nil || d.FailedAt != nil || d.NextAttemptAt.After(now) {
			continue
		}
		ep, ok := f.endpointByID(d.EndpointID)
		if !ok {
			continue
		}
		if _, off := f.disabled[ep.ID]; off || !ep.Enabled {
			continue
		}
		out = append(out, store.DueWebhookDelivery{
			WebhookDelivery: *d,
			URL:             ep.URL, Secret: ep.Secret, TenantID: ep.TenantID,
			EndpointName: ep.Name, CreatedBy: ep.CreatedBy,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeWorkerStore) endpointByID(id string) (store.WebhookEndpoint, bool) {
	for _, e := range f.endpoints {
		if e.ID == id {
			return e, true
		}
	}
	return store.WebhookEndpoint{}, false
}

func (f *fakeWorkerStore) RecordWebhookAttempt(_ context.Context, id string, status int, errMsg string, at, next time.Time, delivered, failed bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.queue[id]
	d.AttemptCount++
	d.LastStatus = status
	d.LastError = errMsg
	attemptedAt := at
	d.LastAttemptedAt = &attemptedAt
	d.NextAttemptAt = next
	if delivered {
		deliveredAt := at
		d.DeliveredAt = &deliveredAt
	}
	if failed {
		failedAt := at
		d.FailedAt = &failedAt
	}
	return nil
}

func (f *fakeWorkerStore) DisableWebhookEndpoint(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled[id] = reason
	return nil
}

// fakeMailer records sends.
type fakeMailer struct {
	mu    sync.Mutex
	sends []string // "to: subject"
}

func (f *fakeMailer) Send(_ context.Context, to, subject, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, to+": "+subject)
	return nil
}

func (f *fakeMailer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sends)
}

type fakeEmails map[string]string

func (f fakeEmails) LookupEmail(_ context.Context, subject string) (string, bool) {
	e, ok := f[subject]
	return e, ok
}

func endpoint(id, tenant, url, secret string, types ...string) store.WebhookEndpoint {
	return store.WebhookEndpoint{
		ID: id, TenantID: tenant, Name: id, URL: url, Secret: secret,
		EventTypes: types, Enabled: true, CreatedBy: "user-1",
	}
}

// receiver is a mock destination recording each request's headers + body and
// answering a configurable status.
type receiver struct {
	mu     sync.Mutex
	status int
	got    []receivedRequest
}

type receivedRequest struct {
	id, timestamp, signature string
	body                     []byte
}

func (rc *receiver) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		rc.mu.Lock()
		rc.got = append(rc.got, receivedRequest{
			id:        r.Header.Get("webhook-id"),
			timestamp: r.Header.Get("webhook-timestamp"),
			signature: r.Header.Get("webhook-signature"),
			body:      body,
		})
		status := rc.status
		rc.mu.Unlock()
		w.WriteHeader(status)
	}
}

func (rc *receiver) requests() []receivedRequest {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return slices.Clone(rc.got)
}

func TestDispatchFansOutOnlyToSubscribedEndpointsOfTheEventsWorkspace(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	// Watermark predates the events (they happened while the worker was up).
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{
		{Key: "dep-1:started", At: eventAt, TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1"},
		{Key: "aud-1:", At: eventAt.Add(time.Second), TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceAudit, Verb: "apps.Suspend"},
		{Key: "aud-2:", At: eventAt.Add(2 * time.Second), TenantID: "tea-b", ServiceID: "beta-worker", ServiceName: "worker", Source: store.EventSourceAudit, Verb: "apps.Suspend"},
	}
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-deploys", "tea-a", "https://a.example/hook", "whsec_x", TypeDeployStarted),
		endpoint("whk-lifecycle", "tea-a", "https://a.example/hook2", "whsec_y", TypeServiceSuspended),
		endpoint("whk-other-tenant", "tea-b", "https://b.example/hook", "whsec_z", TypeDeployStarted, TypeServiceSuspended),
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	byEndpoint := map[string][]string{}
	for _, d := range st.queue {
		byEndpoint[d.EndpointID] = append(byEndpoint[d.EndpointID], d.EventType)
	}
	if got := byEndpoint["whk-deploys"]; !slices.Equal(got, []string{TypeDeployStarted}) {
		t.Errorf("whk-deploys got %v, want [deploy_started]", got)
	}
	if got := byEndpoint["whk-lifecycle"]; !slices.Equal(got, []string{TypeServiceSuspended}) {
		t.Errorf("whk-lifecycle got %v, want [service_suspended]", got)
	}
	// tea-b's endpoint must see only tea-b's event, not tea-a's.
	if got := byEndpoint["whk-other-tenant"]; !slices.Equal(got, []string{TypeServiceSuspended}) {
		t.Errorf("whk-other-tenant got %v, want [service_suspended]", got)
	}
	if st.wmKey != "aud-2:" {
		t.Errorf("watermark key = %q, want the last processed row", st.wmKey)
	}
	for _, d := range st.queue {
		var p payload
		if err := json.Unmarshal([]byte(d.Payload), &p); err != nil {
			t.Fatalf("payload not JSON: %v", err)
		}
		if p.Data.ID != d.EventID || p.Data.ServiceID == "" || p.Timestamp == "" {
			t.Errorf("payload = %+v, delivery = %+v", p, d)
		}
	}
}

func TestDispatchSkipsEventsInsideTheSafetyLag(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st.events = []store.WebhookEventRow{
		{Key: "dep-1:started", At: now.Add(-time.Second), TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted},
	}
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", "https://a.example/hook", "whsec_x", TypeDeployStarted)}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 0 {
		t.Fatalf("an event younger than the lag must not dispatch yet, queued %d", len(st.queue))
	}
	// Once the clock passes the lag, the same event dispatches.
	w.Clock = func() time.Time { return now.Add(dispatchLag) }
	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 1 {
		t.Fatalf("event should dispatch after the lag, queued %d", len(st.queue))
	}
}

func TestSendDeliversWithAVerifiableSignature(t *testing.T) {
	rc := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rc.handler())
	defer srv.Close()

	secret, _ := NewSecret()
	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, secret, TypeDeployStarted)}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	body := `{"type":"deploy_started","timestamp":"2026-07-14T11:59:00Z","data":{"id":"evt-abc","serviceId":"api","serviceName":"api"}}`
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-abc", EventType: TypeDeployStarted,
		Payload: body, NextAttemptAt: now.Add(-time.Second),
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.send(context.Background()); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := rc.requests()
	if len(got) != 1 {
		t.Fatalf("receiver got %d requests, want 1", len(got))
	}
	r := got[0]
	if r.id != "evt-abc" {
		t.Errorf("webhook-id = %q, want evt-abc", r.id)
	}
	if !Verify(secret, r.id, r.timestamp, r.body, r.signature) {
		t.Errorf("delivered signature does not verify: id=%q ts=%q sig=%q body=%s", r.id, r.timestamp, r.signature, r.body)
	}
	d := st.queue["whd-1"]
	if d.DeliveredAt == nil || d.LastStatus != http.StatusOK || d.AttemptCount != 1 {
		t.Errorf("delivery not booked as delivered: %+v", d)
	}
}

func TestFailingEndpointRetriesOnScheduleThenDisablesAndEmails(t *testing.T) {
	rc := &receiver{status: http.StatusInternalServerError}
	srv := httptest.NewServer(rc.handler())
	defer srv.Close()

	secret, _ := NewSecret()
	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, secret, TypeDeployStarted)}
	mailer := &fakeMailer{}
	backoff := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := &now
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-abc", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now.Add(-time.Second),
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{
		Store: st, Mailer: mailer, Emails: fakeEmails{"user-1": "user-1@example.com"},
		Backoff: backoff, Clock: func() time.Time { return *clock },
	}
	ctx := context.Background()

	// Attempt 1: fails, rescheduled +10s, no email yet.
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	d := st.queue["whd-1"]
	if d.AttemptCount != 1 || d.FailedAt != nil || d.LastStatus != http.StatusInternalServerError {
		t.Fatalf("after attempt 1: %+v", d)
	}
	if want := now.Add(10 * time.Second); !d.NextAttemptAt.Equal(want) {
		t.Errorf("next attempt = %v, want %v (the schedule's first delay)", d.NextAttemptAt, want)
	}
	if mailer.count() != 0 {
		t.Fatalf("no email expected before the 3rd failure, got %d", mailer.count())
	}

	// Not due yet: send is a no-op.
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	if d.AttemptCount != 1 {
		t.Fatalf("delivery attempted before its next_attempt_at: %+v", d)
	}

	// Attempt 2: advance past the first delay.
	*clock = now.Add(11 * time.Second)
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	if d.AttemptCount != 2 || mailer.count() != 0 {
		t.Fatalf("after attempt 2: %+v, emails %d", d, mailer.count())
	}

	// Attempt 3: the failure-notice email fires exactly once.
	*clock = now.Add(40 * time.Second)
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	if d.AttemptCount != 3 || mailer.count() != 1 {
		t.Fatalf("after attempt 3: %+v, emails %d (want the 3rd-failure notice)", d, mailer.count())
	}

	// Attempt 4 (= initial + 3 retries) exhausts the schedule: the delivery
	// fails terminally, the endpoint is disabled, and the disable notice goes
	// out even within the suppression window.
	*clock = now.Add(2 * time.Minute)
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	if d.AttemptCount != 4 || d.FailedAt == nil {
		t.Fatalf("after exhausting retries: %+v", d)
	}
	if reason := st.disabled["whk-1"]; reason != disabledReason {
		t.Errorf("endpoint disabled reason = %q, want %q", reason, disabledReason)
	}
	if mailer.count() != 2 {
		t.Errorf("emails = %d, want 2 (3rd-failure notice + disable notice)", mailer.count())
	}

	// Disabled endpoint: nothing further is attempted.
	*clock = now.Add(3 * time.Minute)
	if err := w.send(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	if d.AttemptCount != 4 {
		t.Errorf("a disabled endpoint's queue must be parked, attempts = %d", d.AttemptCount)
	}
	if got := len(rc.requests()); got != 4 {
		t.Errorf("receiver saw %d requests, want 4", got)
	}
}

func TestEmailSuppressionCoalescesABurstOfFailingDeliveries(t *testing.T) {
	rc := &receiver{status: http.StatusBadGateway}
	srv := httptest.NewServer(rc.handler())
	defer srv.Close()

	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, "whsec_x", TypeDeployStarted)}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	// Two deliveries both on their 3rd attempt.
	for _, id := range []string{"whd-1", "whd-2"} {
		st.queue[id] = &store.WebhookDelivery{
			ID: id, EndpointID: "whk-1", EventID: "evt-" + id, EventType: TypeDeployStarted,
			Payload: `{}`, AttemptCount: 2, NextAttemptAt: now.Add(-time.Second),
		}
		st.queueOrder = append(st.queueOrder, id)
	}
	mailer := &fakeMailer{}
	w := &Worker{Store: st, Mailer: mailer, Emails: fakeEmails{"user-1": "u@example.com"}, Clock: func() time.Time { return now }}

	if err := w.send(context.Background()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if mailer.count() != 1 {
		t.Errorf("emails = %d, want 1 (suppression must coalesce the burst)", mailer.count())
	}
}

func TestNoEndpointsCostsOneQueryAndLateEndpointsGetNoReplay(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	// The watermark predates an old event that happened while nothing was
	// subscribed.
	st.wmSeeded, st.wmAt = true, now.Add(-2*time.Hour)
	st.events = []store.WebhookEventRow{
		{Key: "dep-old:started", At: now.Add(-time.Hour), TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted},
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	// No enabled endpoints: RunOnce is the one-SELECT fast path — nothing
	// queued, and the watermark is NOT chased forward.
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(st.queue) != 0 {
		t.Fatalf("no endpoints => nothing queued, got %d", len(st.queue))
	}
	if !st.wmAt.Equal(now.Add(-2 * time.Hour)) {
		t.Fatalf("no endpoints => watermark untouched, moved to %v", st.wmAt)
	}

	// An endpoint registered later must not receive the pre-registration
	// event — the per-endpoint CreatedAt guard, not a watermark race.
	late := endpoint("whk-late", "tea-a", "https://a.example/h", "whsec_x", TypeDeployStarted)
	late.CreatedAt = now.Add(-time.Minute)
	st.endpoints = []store.WebhookEndpoint{late}
	w.Clock = func() time.Time { return now.Add(time.Minute) }
	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 0 {
		t.Errorf("late-registered endpoint must not be sent pre-registration history, got %d deliveries", len(st.queue))
	}
	// A post-registration event DOES deliver. (Timestamped after the previous
	// pass's read window — within the window the dispatch lag guarantees a
	// real commit lands in.)
	st.mu.Lock()
	st.events = append(st.events, store.WebhookEventRow{
		Key: "dep-new:started", At: now.Add(90 * time.Second), TenantID: "tea-a",
		ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted,
	})
	st.mu.Unlock()
	w.Clock = func() time.Time { return now.Add(2 * time.Minute) }
	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 1 {
		t.Errorf("post-registration event should deliver, got %d deliveries", len(st.queue))
	}
}
