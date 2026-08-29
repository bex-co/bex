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
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
	attempts   map[string]*fakeAttempt
	disabled   map[string]string    // endpoint id -> reason
	notifiedAt map[string]time.Time // endpoint id -> last failure-notice CAS
	queueOrder []string
	// seen dedupes inserts on (endpoint_id, event_id), mirroring the store's
	// unique index so a re-dispatch of the same event enqueues no duplicate.
	seen map[string]bool

	// sweeps records each retention pass's (before, keepPerEndpoint, limit) so a
	// test can assert the policy the worker applies (w1/m67 F3).
	sweeps []sweepCall

	// nonAdmins marks subjects the admin gate (round-14 #6) must refuse: a
	// subject absent from the set is a current workspace admin (the fixtures'
	// default "user-1"), so the pre-existing email tests keep flowing.
	nonAdmins map[string]bool
}

// SubjectIsWorkspaceAdmin fakes the failure-notice recipient gate: everyone is
// a current admin except subjects listed in nonAdmins.
func (f *fakeWorkerStore) SubjectIsWorkspaceAdmin(_ context.Context, _ string, subject string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.nonAdmins[subject], nil
}

type fakeAttempt struct {
	parentID  string
	origin    string
	available time.Time
	lease     time.Time
	resume    *time.Time
	done      bool
}

type sweepCall struct {
	before          time.Time
	keepPerEndpoint int
	limit           int
}

// SweepWebhookDeliveries mirrors the store's contract: only TERMINAL rows are
// eligible, and it never touches a delivery still awaiting or retrying delivery.
func (f *fakeWorkerStore) SweepWebhookDeliveries(_ context.Context, before time.Time, keepPerEndpoint, limit int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sweeps = append(f.sweeps, sweepCall{before: before, keepPerEndpoint: keepPerEndpoint, limit: limit})

	type terminal struct {
		id string
		at time.Time
	}
	perEndpoint := map[string][]terminal{}
	for id, d := range f.queue {
		if d.DeliveredAt == nil && d.FailedAt == nil {
			continue // pending or retryable: never eligible
		}
		perEndpoint[d.EndpointID] = append(perEndpoint[d.EndpointID], terminal{id: id, at: d.CreatedAt})
	}
	var deleted int64
	for _, rows := range perEndpoint {
		sort.Slice(rows, func(i, j int) bool { return rows[i].at.After(rows[j].at) })
		for i, r := range rows {
			if deleted >= int64(limit) {
				return deleted, nil
			}
			if i >= keepPerEndpoint || r.at.Before(before) {
				delete(f.queue, r.id)
				deleted++
			}
		}
	}
	return deleted, nil
}

func newFakeWorkerStore() *fakeWorkerStore {
	return &fakeWorkerStore{
		queue:      map[string]*store.WebhookDelivery{},
		attempts:   map[string]*fakeAttempt{},
		disabled:   map[string]string{},
		notifiedAt: map[string]time.Time{},
		seen:       map[string]bool{},
		nonAdmins:  map[string]bool{},
	}
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
		cursorAt := r.CursorAt
		if cursorAt.IsZero() {
			cursorAt = r.At
			r.CursorAt = cursorAt
		}
		afterWM := cursorAt.After(afterAt) || (cursorAt.Equal(afterAt) && r.Key > afterKey)
		if !afterWM || cursorAt.After(until) || !slices.Contains(tenants, r.TenantID) {
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

func (f *fakeWorkerStore) EnqueueWebhookDeliveries(_ context.Context, deliveries []store.WebhookDelivery, at time.Time, key string, maxPerWorkspace int) (store.WebhookEnqueueResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := store.WebhookEnqueueResult{}
	openByWorkspace := map[string]int{}
	for _, d := range f.queue {
		if d.DeliveredAt != nil || d.FailedAt != nil {
			continue
		}
		if ep, ok := f.endpointByID(d.EndpointID); ok {
			openByWorkspace[ep.TenantID]++
		}
	}
	for _, d := range deliveries {
		dedupKey := d.EndpointID + "\x00" + d.EventID
		if f.seen[dedupKey] { // mirrors the (endpoint_id, event_id) unique index
			result.Deduplicated++
			continue
		}
		ep, ok := f.endpointByID(d.EndpointID)
		if !ok {
			result.Deduplicated++
			continue
		}
		if maxPerWorkspace > 0 && openByWorkspace[ep.TenantID] >= maxPerWorkspace {
			result.Capped++
			continue
		}
		f.seen[dedupKey] = true
		cp := d
		f.queue[d.ID] = &cp
		f.attempts[d.ID] = &fakeAttempt{parentID: d.ID, origin: store.WebhookAttemptAutomatic, available: d.NextAttemptAt}
		f.queueOrder = append(f.queueOrder, d.ID)
		openByWorkspace[ep.TenantID]++
		result.Admitted++
	}
	f.wmAt, f.wmKey = at, key
	return result, nil
}

func (f *fakeWorkerStore) ClaimDueWebhookAttempts(_ context.Context, now, leaseUntil time.Time, limit int) ([]store.DueWebhookAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Older tests seed the parent map directly. Materialize their initial
	// reservation lazily so the fake follows the production two-level model.
	for _, id := range f.queueOrder {
		if _, ok := f.attempts[id]; !ok {
			d := f.queue[id]
			f.attempts[id] = &fakeAttempt{parentID: id, origin: store.WebhookAttemptAutomatic, available: d.NextAttemptAt}
		}
	}
	var out []store.DueWebhookAttempt
	for _, id := range f.queueOrder {
		a := f.attempts[id]
		if a == nil || a.done || a.available.After(now) || (!a.lease.IsZero() && a.lease.After(now)) {
			continue
		}
		d := f.queue[a.parentID]
		ep, ok := f.endpointByID(d.EndpointID)
		if !ok {
			continue
		}
		if _, off := f.disabled[ep.ID]; off || !ep.Enabled {
			continue
		}
		a.lease = leaseUntil
		nextAt := a.available
		out = append(out, store.DueWebhookAttempt{
			WebhookAttempt: store.WebhookAttempt{
				ID: id, NotificationID: a.parentID, EndpointID: d.EndpointID,
				EventID: d.EventID, EventType: d.EventType, ServiceID: d.ServiceID,
				Payload: d.Payload, Status: store.WebhookAttemptPending,
				Origin: a.origin, NextAttemptAt: &nextAt, ResumeAutomaticAt: a.resume,
			},
			URL: ep.URL, Secret: ep.Secret, TenantID: ep.TenantID,
			EndpointName: ep.Name, CreatedBy: ep.CreatedBy, AutomaticAttemptCount: d.AttemptCount,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeWorkerStore) ClaimWebhookFailureNotice(_ context.Context, endpointID string, now, threshold time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if last, ok := f.notifiedAt[endpointID]; ok && last.After(threshold) {
		return false, nil
	}
	f.notifiedAt[endpointID] = now
	return true, nil
}

func (f *fakeWorkerStore) endpointByID(id string) (store.WebhookEndpoint, bool) {
	for _, e := range f.endpoints {
		if e.ID == id {
			return e, true
		}
	}
	return store.WebhookEndpoint{}, false
}

func (f *fakeWorkerStore) CompleteWebhookAttempt(_ context.Context, completion store.WebhookAttemptCompletion) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.attempts[completion.AttemptID]
	if a == nil || a.done {
		return false, nil
	}
	a.done = true
	d := f.queue[a.parentID]
	if a.origin == store.WebhookAttemptAutomatic {
		d.AttemptCount++
	}
	d.LastStatus = completion.StatusCode
	d.LastError = completion.TransportError
	d.ResponseBody = completion.ResponseBody
	if d.SentAt == nil {
		sentAt := completion.CompletedAt
		d.SentAt = &sentAt
	}
	attemptedAt := completion.CompletedAt
	d.LastAttemptedAt = &attemptedAt
	if a.origin == store.WebhookAttemptAutomatic {
		d.NextAttemptAt = completion.NextAttemptAt
	}
	if completion.Delivered {
		deliveredAt := completion.CompletedAt
		d.DeliveredAt = &deliveredAt
		d.FailedAt = nil
	}
	if completion.Exhausted {
		failedAt := completion.CompletedAt
		d.FailedAt = &failedAt
		f.disabled[d.EndpointID] = completion.DisableReason
	}
	if !completion.Delivered && a.origin == store.WebhookAttemptAutomatic && !completion.Exhausted {
		f.attempts[completion.NextAttemptID] = &fakeAttempt{parentID: a.parentID, origin: store.WebhookAttemptAutomatic, available: completion.NextAttemptAt}
		f.queueOrder = append(f.queueOrder, completion.NextAttemptID)
	}
	if !completion.Delivered && a.origin == store.WebhookAttemptManual && a.resume != nil {
		f.attempts[completion.NextAttemptID] = &fakeAttempt{parentID: a.parentID, origin: store.WebhookAttemptAutomatic, available: *a.resume}
		f.queueOrder = append(f.queueOrder, completion.NextAttemptID)
		d.NextAttemptAt = *a.resume
		d.DeliveredAt, d.FailedAt = nil, nil
	}
	return true, nil
}

// fakeMailer records sends, keeping the last text body for content assertions.
type fakeMailer struct {
	mu       sync.Mutex
	sends    []string // "to: subject"
	lastText string
	lastHTML string
}

type fakeAttemptObserver struct {
	mu     sync.Mutex
	values []string
}

type fakeAdmissionObserver struct {
	mu    sync.Mutex
	value store.WebhookEnqueueResult
}

func (o *fakeAttemptObserver) ObserveWebhookAttempt(origin, result string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.values = append(o.values, origin+":"+result)
}

func (o *fakeAttemptObserver) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.values)
}

func (o *fakeAdmissionObserver) ObserveWebhookAdmission(result store.WebhookEnqueueResult) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.value.Admitted += result.Admitted
	o.value.Capped += result.Capped
	o.value.Deduplicated += result.Deduplicated
}

func (o *fakeAdmissionObserver) snapshot() store.WebhookEnqueueResult {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.value
}

func (f *fakeMailer) Send(_ context.Context, to, subject, text, html string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, to+": "+subject)
	f.lastText, f.lastHTML = text, html
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
		{Key: "fact:suspend-1", At: eventAt.Add(time.Second), TenantID: "tea-a", ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceFact, FactType: string(store.EventFactServiceSuspended)},
		{Key: "fact:suspend-2", At: eventAt.Add(2 * time.Second), TenantID: "tea-b", ServiceID: "beta-worker", ServiceName: "worker", Source: store.EventSourceFact, FactType: string(store.EventFactServiceSuspended)},
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
	if st.wmKey != "fact:suspend-2" {
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

func TestEmptyFilterSubscribesToNewlyIntroducedEvents(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{{
		Key: "fact:new-auto-deploy", At: eventAt, TenantID: "tea-a",
		ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceAudit,
		Verb: autoDeployVerb, AutoDeployEnabled: func() *bool { value := true; return &value }(),
	}}
	// The persisted empty slice is Render's all-events subscription. It predates
	// auto_deploy_enabled yet must receive it without an endpoint update.
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-all", "tea-a", "https://a.example/hook", "whsec_x"),
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}
	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatal(err)
	}
	if len(st.queue) != 1 {
		t.Fatalf("all-events deliveries = %d, want 1", len(st.queue))
	}
	for _, delivery := range st.queue {
		if delivery.EventType != TypeAutoDeployEnabled {
			t.Fatalf("event type = %q, want %q", delivery.EventType, TypeAutoDeployEnabled)
		}
	}
}

func TestProjectDatastoreAuditEventUsesRenderThinPayload(t *testing.T) {
	row := store.WebhookEventRow{
		Key:         "aud-postgres-created:",
		At:          time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
		TenantID:    "tea-acme",
		ServiceID:   "dpg-orders",
		ServiceName: "orders",
		Source:      store.EventSourceAudit,
		Verb:        core.AuditVerbPostgresCreated,
	}

	eventType, data, ok := project(row)
	if !ok || eventType != TypePostgresCreated {
		t.Fatalf("project type = (%q, %v), want (%q, true)", eventType, ok, TypePostgresCreated)
	}
	if data.ID == "" || data.ServiceID != "dpg-orders" || data.ServiceName != "orders" || data.Status != "" {
		t.Fatalf("project data = %+v", data)
	}

	row.Verb = core.AuditVerbPostgresUpdated
	if eventType, _, ok := project(row); ok {
		t.Fatalf("unrelated datastore update projected as %q", eventType)
	}
}

func TestProjectServiceEventUsesCanonicalAppID(t *testing.T) {
	eventType, data, ok := project(store.WebhookEventRow{
		Key: "dep-canonical:started", ServiceID: "acme-api", ServiceName: "api",
		AppID: "srv-00000000000000000000", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseStarted,
	})
	if !ok || eventType != TypeDeployStarted || data.ServiceID != "srv-00000000000000000000" {
		t.Fatalf("project = (%q, %+v, %v), want canonical srv id", eventType, data, ok)
	}
}

// A renamed service's events must project the label the dashboard shows, not
// the immutable name it was created under (w6/m101). The feed resolves the
// label in SQL; what this pins is that the projection keeps the two apart —
// serviceName carries the row's label, serviceId the immutable id a receiver
// calls the API back with.
func TestProjectRenamedServiceKeepsLabelAndIDApart(t *testing.T) {
	_, data, ok := project(store.WebhookEventRow{
		Key: "dep-renamed:started", ServiceID: "acme-block-eden-mono", ServiceName: "eden-dash-v3",
		AppID: "srv-d9ndt8hmcglc739fkp50", Source: store.EventSourceDeploy,
		Phase: store.EventPhaseStarted,
	})
	if !ok || data.ServiceName != "eden-dash-v3" || data.ServiceID != "srv-d9ndt8hmcglc739fkp50" {
		t.Fatalf("project = (%+v, %v), want the display label with the immutable id", data, ok)
	}
}

func TestProjectExistingLifecycleFactsCarriesOnlyTerminalStatus(t *testing.T) {
	tests := []struct {
		factType store.ServiceEventFactType
		status   string
		wantType string
		want     string
	}{
		{store.EventFactBranchDeleted, "", TypeBranchDeleted, ""},
		{store.EventFactBuildStarted, "", TypeBuildStarted, ""},
		{store.EventFactBuildEnded, store.EventStatusFailed, TypeBuildEnded, store.EventStatusFailed},
		// build_ended(canceled) is the fact the Cancel verb now emits on a
		// mid-build cancel (w6/m128); this pins that an outbound webhook
		// subscriber receives the closed pair's build_ended with a canceled
		// outcome, not just the reconciler-emitted failed/succeeded ones. The
		// projection reads fact_type + status, so a fact Cancel inserts is
		// indistinguishable here from one the reconciler inserts (t002).
		{store.EventFactBuildEnded, store.EventStatusCanceled, TypeBuildEnded, store.EventStatusCanceled},
		{store.EventFactPreDeployStarted, "", TypePreDeployStarted, ""},
		{store.EventFactPreDeployEnded, store.EventStatusSucceeded, TypePreDeployEnded, store.EventStatusSucceeded},
		{store.EventFactJobRunEnded, store.EventStatusCanceled, TypeJobRunEnded, store.EventStatusCanceled},
	}
	for _, tc := range tests {
		t.Run(string(tc.factType), func(t *testing.T) {
			eventType, data, ok := project(store.WebhookEventRow{
				Key:         "fact:" + string(tc.factType),
				ServiceID:   "acme-api",
				ServiceName: "api",
				Source:      store.EventSourceFact,
				FactType:    string(tc.factType),
				Status:      tc.status,
			})
			if !ok || eventType != tc.wantType || data.Status != tc.want {
				t.Fatalf("project = (%q, %+v, %v), want type %q status %q", eventType, data, ok, tc.wantType, tc.want)
			}
		})
	}
}

func TestProjectAutoDeployAuditDiscriminatesResult(t *testing.T) {
	enabled, disabled := true, false
	for _, tc := range []struct {
		name   string
		value  *bool
		want   string
		wantOK bool
	}{
		{name: "enabled", value: &enabled, want: TypeAutoDeployEnabled, wantOK: true},
		{name: "disabled", value: &disabled, want: TypeAutoDeployDisabled, wantOK: true},
		{name: "legacy missing discriminator", value: nil, wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eventType, data, ok := project(store.WebhookEventRow{
				Key:               "audit:auto-deploy",
				ServiceID:         "acme-api",
				ServiceName:       "api",
				Source:            store.EventSourceAudit,
				Verb:              autoDeployVerb,
				AutoDeployEnabled: tc.value,
			})
			if ok != tc.wantOK || eventType != tc.want {
				t.Fatalf("project = (%q, %+v, %v), want (%q, _, %v)", eventType, data, ok, tc.want, tc.wantOK)
			}
		})
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

func TestDispatchUsesFactRecordingTimeWithoutRewritingOccurrenceTime(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st.wmSeeded, st.wmAt = true, now.Add(-time.Minute)
	st.events = []store.WebhookEventRow{{
		CursorAt: now.Add(-10 * time.Second),
		Key:      "fact:late-observation", At: now.Add(-time.Hour), TenantID: "tea-a",
		ServiceID: "acme-api", ServiceName: "api", Source: store.EventSourceFact,
		FactType: string(store.EventFactServerAvailable),
	}}
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-1", "tea-a", "https://a.example/hook", "whsec_x", TypeServerAvailable),
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }}

	if err := w.dispatch(context.Background(), st.endpoints); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(st.queue) != 1 {
		t.Fatalf("late-recorded fact deliveries = %d, want 1", len(st.queue))
	}
	for _, delivery := range st.queue {
		var got payload
		if err := json.Unmarshal([]byte(delivery.Payload), &got); err != nil {
			t.Fatal(err)
		}
		if got.Timestamp != now.Add(-time.Hour).Format(time.RFC3339) {
			t.Fatalf("payload timestamp = %q, want original occurrence time", got.Timestamp)
		}
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
	w := &Worker{Store: st, Clock: func() time.Time { return now }, Client: &http.Client{}}

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

func TestResponseEvidenceIsUTF8AndByteBounded(t *testing.T) {
	// Include invalid UTF-8 before an oversized tail: Postgres text and JSON must
	// still receive valid text, and the truncation marker lives inside the cap.
	body := append([]byte{'o', 'k', 0xff}, bytes.Repeat([]byte("界"), 2000)...)
	got, err := readResponseEvidence(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > maxWebhookResponseBytes {
		t.Fatalf("evidence bytes = %d, cap %d", len(got), maxWebhookResponseBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatal("response evidence is not valid UTF-8")
	}
	if !strings.HasSuffix(got, responseTruncatedSuffix) {
		t.Fatalf("truncated evidence missing marker: %q", got[len(got)-64:])
	}
}

func TestSendStoresBoundedResponseEvidenceAndFirstSentAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxWebhookResponseBytes+100))
	}))
	defer srv.Close()

	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, "whsec_x", TypeDeployStarted)}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now.Add(-time.Second),
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{Store: st, Backoff: []time.Duration{time.Second}, Clock: func() time.Time { return now }, Client: &http.Client{}}
	if err := w.send(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := st.queue["whd-1"]
	if d.SentAt == nil || !d.SentAt.Equal(now) || len(d.ResponseBody) > maxWebhookResponseBytes ||
		!strings.HasSuffix(d.ResponseBody, responseTruncatedSuffix) {
		t.Fatalf("recorded evidence = %+v", d)
	}
}

func TestSendStoresBoundedUTF8TransportError(t *testing.T) {
	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", "https://hooks.example.test/error", "whsec_x", TypeDeployStarted)}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now,
	}
	st.queueOrder = []string{"whd-1"}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(strings.Repeat("界", 1000))
	})}
	w := &Worker{Store: st, Backoff: []time.Duration{time.Second}, Clock: func() time.Time { return now }, Client: client}
	if err := w.send(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := st.queue["whd-1"].LastError
	if len(got) > 2048 || !utf8.ValidString(got) {
		t.Fatalf("transport error bytes=%d valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestAttemptObserverDistinguishesAutomaticAndManualTerminalizations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, "whsec_x", TypeDeployStarted)}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.queue["whd-auto"] = &store.WebhookDelivery{
		ID: "whd-auto", EndpointID: "whk-1", EventID: "evt-auto", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now,
	}
	st.queue["whd-manual-parent"] = &store.WebhookDelivery{
		ID: "whd-manual-parent", EndpointID: "whk-1", EventID: "evt-manual", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now,
	}
	st.attempts["whd-manual"] = &fakeAttempt{parentID: "whd-manual-parent", origin: store.WebhookAttemptManual, available: now}
	st.queueOrder = []string{"whd-auto", "whd-manual"}
	observer := &fakeAttemptObserver{}
	w := &Worker{
		Store: st, Backoff: []time.Duration{time.Hour}, Clock: func() time.Time { return now },
		Client: &http.Client{}, Attempts: observer,
	}
	if err := w.send(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := observer.snapshot()
	slices.Sort(got)
	want := []string{
		store.WebhookAttemptAutomatic + ":" + store.WebhookAttemptFailed,
		store.WebhookAttemptManual + ":" + store.WebhookAttemptFailed,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("attempt observations = %v, want %v", got, want)
	}
	// A losing/duplicate completion does not increment metrics a second time.
	w.attempt(t.Context(), store.DueWebhookAttempt{
		WebhookAttempt: store.WebhookAttempt{
			ID: "whd-manual", NotificationID: "whd-manual-parent", EndpointID: "whk-1",
			EventID: "evt-manual", EventType: TypeDeployStarted, Payload: `{}`,
			Origin: store.WebhookAttemptManual,
		},
		URL: srv.URL,
	})
	if got := observer.snapshot(); len(got) != 2 {
		t.Fatalf("duplicate completion observations = %v", got)
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
		Backoff: backoff, Clock: func() time.Time { return *clock }, Client: &http.Client{},
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
	// The failing notice carries the same sentences as before, with "Last error:"
	// as its own paragraph, and a branded HTML alternative.
	if !strings.Contains(mailer.lastText, "times in a row.") {
		t.Errorf("failing notice text missing the failure-count sentence:\n%s", mailer.lastText)
	}
	if !strings.Contains(mailer.lastText, "\n\nLast error: ") {
		t.Errorf("failing notice text should carry Last error on its own paragraph:\n%s", mailer.lastText)
	}
	if !strings.Contains(mailer.lastHTML, "Last error:") || !strings.Contains(mailer.lastHTML, email.BrandPrimary) {
		t.Errorf("failing notice HTML should be branded and carry Last error:\n%s", mailer.lastHTML)
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
	// The disable notice's sentences are intact across both bodies.
	if !strings.Contains(mailer.lastText, "the endpoint has been disabled.") {
		t.Errorf("disable notice text missing the disabled sentence:\n%s", mailer.lastText)
	}
	if !strings.Contains(mailer.lastHTML, "No further events will be sent") {
		t.Errorf("disable notice HTML missing the re-enable guidance:\n%s", mailer.lastHTML)
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

func TestDefaultRetryTimelineStopsAfterEightAttempts(t *testing.T) {
	rc := &receiver{status: http.StatusInternalServerError}
	srv := httptest.NewServer(rc.handler())
	defer srv.Close()

	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, "whsec_x", TypeDeployStarted)}
	mailer := &fakeMailer{}
	first := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	now := first
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: first,
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{
		Store: st, Mailer: mailer, Emails: fakeEmails{"user-1": "user-1@example.com"},
		Clock: func() time.Time { return now }, Client: &http.Client{},
	}

	wantOffsets := []time.Duration{
		0,
		30 * time.Second,
		10*time.Minute + 30*time.Second,
		40*time.Minute + 30*time.Second,
		2*time.Hour + 40*time.Minute + 30*time.Second,
		7*time.Hour + 40*time.Minute + 30*time.Second,
		17*time.Hour + 40*time.Minute + 30*time.Second,
		32*time.Hour + 40*time.Minute + 30*time.Second,
	}
	for attempt, offset := range wantOffsets {
		now = first.Add(offset)
		if err := w.send(t.Context()); err != nil {
			t.Fatalf("attempt %d: %v", attempt+1, err)
		}
		if got := st.queue["whd-1"].AttemptCount; got != attempt+1 {
			t.Fatalf("after attempt %d count = %d", attempt+1, got)
		}
	}
	d := st.queue["whd-1"]
	if d.FailedAt == nil || !d.FailedAt.Equal(first.Add(wantOffsets[7])) {
		t.Fatalf("terminal failure = %+v", d.FailedAt)
	}
	if got := st.disabled["whk-1"]; got != disabledReason {
		t.Fatalf("disabled reason = %q", got)
	}
	if got := mailer.count(); got != 2 {
		t.Fatalf("notices = %d, want third-failure plus disable", got)
	}

	// Even if the clock moves far beyond the final schedule, the disabled
	// endpoint cannot produce a ninth POST or a second disable notice.
	now = first.Add(7 * 24 * time.Hour)
	if err := w.send(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := len(rc.requests()); got != 8 || d.AttemptCount != 8 || mailer.count() != 2 {
		t.Fatalf("after exhaustion: posts=%d attempts=%d notices=%d", got, d.AttemptCount, mailer.count())
	}
}

func TestDefaultRetryTimelineCanSucceedOnEighthAttempt(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts++
		if posts < 8 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	st := newFakeWorkerStore()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, "whsec_x", TypeDeployStarted)}
	first := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	now := first
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: first,
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{Store: st, Clock: func() time.Time { return now }, Client: &http.Client{}}

	for attempt := 0; attempt < 8; attempt++ {
		if err := w.send(t.Context()); err != nil {
			t.Fatal(err)
		}
		if attempt < 7 {
			now = st.queue["whd-1"].NextAttemptAt
		}
	}
	d := st.queue["whd-1"]
	if posts != 8 || d.AttemptCount != 8 || d.DeliveredAt == nil || d.FailedAt != nil || len(st.disabled) != 0 {
		t.Fatalf("last-attempt recovery: posts=%d delivery=%+v disabled=%v", posts, d, st.disabled)
	}
}

func TestResponseBudgetAndBackoffOverrideContract(t *testing.T) {
	if requestTimeout != 15*time.Second || defaultClient.Timeout != requestTimeout {
		t.Fatalf("response budget = %v, client timeout = %v", requestTimeout, defaultClient.Timeout)
	}
	var remaining time.Duration
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatal("delivery request has no context deadline")
		}
		remaining = time.Until(deadline)
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	w := &Worker{Client: client}
	if _, _, err := w.post(t.Context(), store.DueWebhookAttempt{WebhookAttempt: store.WebhookAttempt{EventID: "evt-1", Payload: `{}`}, URL: "https://hooks.example.com", Secret: "whsec_x"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if remaining < 14*time.Second || remaining > requestTimeout {
		t.Fatalf("request deadline remaining = %v", remaining)
	}

	got, err := ParseBackoff("5s, 10s,1m")
	if err != nil || !slices.Equal(got, []time.Duration{5 * time.Second, 10 * time.Second, time.Minute}) {
		t.Fatalf("ParseBackoff = %v, %v", got, err)
	}
	if got, err := ParseBackoff("  "); err != nil || got != nil {
		t.Fatalf("empty ParseBackoff = %v, %v", got, err)
	}
	if _, err := ParseBackoff("5s,nope"); err == nil || !strings.Contains(err.Error(), "BEX_WEBHOOK_BACKOFF") {
		t.Fatalf("malformed ParseBackoff error = %v", err)
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
	w := &Worker{Store: st, Mailer: mailer, Emails: fakeEmails{"user-1": "u@example.com"}, Clock: func() time.Time { return now }, Client: &http.Client{}}

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

// TestDefaultClientSSRFGuard verifies that the production webhook client
// blocks loopback/private/link-local destinations at dial time and never
// follows redirects (so a redirect-to-private chain cannot bypass the guard).
func TestDefaultClientSSRFGuard(t *testing.T) {
	// Redirects must not be followed — a 3xx is a delivery failure, not a hop.
	if err := defaultClient.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("defaultClient.CheckRedirect = %v; want http.ErrUseLastResponse", err)
	}

	tr, ok := defaultClient.Transport.(*http.Transport)
	if !ok || tr.DialContext == nil {
		t.Fatal("defaultClient.Transport must be *http.Transport with a DialContext SSRF guard")
	}

	// Literal IP addresses are resolved locally (no external DNS needed).
	for _, addr := range []string{
		"127.0.0.1:80",       // loopback
		"10.0.0.1:80",        // RFC 1918 private
		"169.254.169.254:80", // cloud metadata (AWS, GCP)
		"192.168.1.1:80",     // RFC 1918 private
	} {
		_, err := tr.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			t.Errorf("dial %s: expected SSRF block, got nil error", addr)
			continue
		}
		if !strings.Contains(err.Error(), "blocked address") {
			t.Errorf("dial %s: error %q; want to contain \"blocked address\"", addr, err.Error())
		}
	}
}
