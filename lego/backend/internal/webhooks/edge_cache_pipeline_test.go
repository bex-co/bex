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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w9/m345: edge-cache changes and builds refused for exhausted pipeline minutes
// are subscribable and delivered, signed, under Render's names. A profile save
// that left caching as it was is no event.
func TestEdgeCacheAndPipelineEventsReachTheirSubscribers(t *testing.T) {
	types, err := normalizeEventTypes([]string{TypePipelineMinutesExhausted, TypeEdgeCachePurged, TypeEdgeCacheDisabled, TypeEdgeCacheEnabled})
	if err != nil || len(types) != 4 {
		t.Fatalf("a subscription to the four types = %v, %v; want all accepted", types, err)
	}

	rc := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rc.handler())
	defer srv.Close()
	secret, _ := NewSecret()

	st := newFakeWorkerStore()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	st.wmSeeded, st.wmAt = true, at.Add(-time.Hour)
	row := func(key string, offset time.Duration, source string) store.WebhookEventRow {
		return store.WebhookEventRow{Key: key, At: at.Add(offset), TenantID: "tea-a", ServiceID: "tea-a-web", ServiceName: "web", Source: source}
	}
	on := row("aud-on:", 0, store.EventSourceAudit)
	on.Verb, on.EdgeCacheEnabledTo = core.AuditVerbSetCacheProfile, new(true)
	off := row("aud-off:", time.Second, store.EventSourceAudit)
	off.Verb, off.EdgeCacheEnabledTo = core.AuditVerbSetCacheProfile, new(false)
	kept := row("aud-kept:", 2*time.Second, store.EventSourceAudit)
	kept.Verb = core.AuditVerbSetCacheProfile
	purge := row("aud-purge:", 3*time.Second, store.EventSourceAudit)
	purge.Verb = core.AuditVerbPurgeCache
	refused := row("fact:deploy:dep-1:pipeline_minutes_exhausted", 4*time.Second, store.EventSourceFact)
	refused.FactType = string(store.EventFactPipelineMinutesExhausted)
	st.events = []store.WebhookEventRow{on, off, kept, purge, refused}
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-purge", "tea-a", srv.URL+"/purge", secret, TypeEdgeCachePurged),
		endpoint("whk-all", "tea-a", srv.URL+"/all", secret),
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }, Client: &http.Client{}}
	if err := dispatchStored(w); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	byEndpoint := map[string][]string{}
	for _, id := range st.queueOrder {
		d := st.queue[id]
		byEndpoint[d.EndpointID] = append(byEndpoint[d.EndpointID], d.EventType)
	}
	if got := byEndpoint["whk-purge"]; !slices.Equal(got, []string{TypeEdgeCachePurged}) {
		t.Errorf("edge_cache_purged subscriber got %v", got)
	}
	want := []string{TypeEdgeCacheEnabled, TypeEdgeCacheDisabled, TypeEdgeCachePurged, TypePipelineMinutesExhausted}
	if got := byEndpoint["whk-all"]; !slices.Equal(got, want) {
		t.Errorf("all-events subscriber got %v, want %v (a save that kept caching on is no event)", got, want)
	}

	if err := w.send(context.Background()); err != nil {
		t.Fatalf("send: %v", err)
	}
	var signed int
	for _, r := range rc.requests() {
		var p struct {
			Type string      `json:"type"`
			Data payloadData `json:"data"`
		}
		if err := json.Unmarshal(r.body, &p); err != nil {
			t.Fatalf("payload not JSON: %v", err)
		}
		if p.Type == TypeEdgeCachePurged && p.Data.ServiceID == "tea-a-web" && verifyAt(now, secret, r.id, r.timestamp, r.body, r.signature) {
			signed++
		}
	}
	if signed != 2 {
		t.Fatalf("verifiably signed edge_cache_purged deliveries = %d, want 2 (one per subscriber)", signed)
	}
}

// w9/088: a plan change's edge-cache toggle (apps.SetPlanEdgeCache) is pushed
// down into the feed query and delivered as edge_cache_disabled / _enabled
// beside the plan change's plan_changed, under the event id the Events feed
// shows, so GET /v1/events/{id} hydrates the delivery to the system-triggered
// details.
func TestPlanChangeEdgeCacheTogglesReachTheirSubscribers(t *testing.T) {
	if !slices.Contains(auditVerbs, core.AuditVerbSetPlanEdgeCache) {
		t.Fatalf("auditVerbs %v does not push %s down into the feed query", auditVerbs, core.AuditVerbSetPlanEdgeCache)
	}
	st := newFakeWorkerStore()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Minute)
	st.wmSeeded, st.wmAt = true, at.Add(-time.Hour)
	row := func(key string, offset time.Duration, verb string, enabledTo *bool) store.WebhookEventRow {
		return store.WebhookEventRow{Key: key, At: at.Add(offset), TenantID: "tea-a", ServiceID: "tea-a-web", ServiceName: "web",
			Source: store.EventSourceAudit, Verb: verb, EdgeCacheEnabledTo: enabledTo}
	}
	st.events = []store.WebhookEventRow{
		row("aud-free:", 0, core.AuditVerbSetPlan, nil),
		row("aud-off:", time.Second, core.AuditVerbSetPlanEdgeCache, new(false)),
		row("aud-paid:", 2*time.Second, core.AuditVerbSetPlan, nil),
		row("aud-on:", 3*time.Second, core.AuditVerbSetPlanEdgeCache, new(true)),
	}
	st.endpoints = []store.WebhookEndpoint{
		endpoint("whk-cache", "tea-a", "https://example.test/cache", "", TypeEdgeCacheEnabled, TypeEdgeCacheDisabled),
		endpoint("whk-all", "tea-a", "https://example.test/all", ""),
	}
	w := &Worker{Store: st, Clock: func() time.Time { return now }, Client: &http.Client{}}
	if err := dispatchStored(w); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	byEndpoint := map[string][]string{}
	for _, id := range st.queueOrder {
		d := st.queue[id]
		byEndpoint[d.EndpointID] = append(byEndpoint[d.EndpointID], d.EventType+" "+d.EventID)
	}
	off, on := TypeEdgeCacheDisabled+" "+ids.Derive(ids.Event, "aud-off:"), TypeEdgeCacheEnabled+" "+ids.Derive(ids.Event, "aud-on:")
	if got := byEndpoint["whk-cache"]; !slices.Equal(got, []string{off, on}) {
		t.Errorf("edge-cache subscriber got %v, want %v", got, []string{off, on})
	}
	want := []string{TypePlanChanged + " " + ids.Derive(ids.Event, "aud-free:"), off, TypePlanChanged + " " + ids.Derive(ids.Event, "aud-paid:"), on}
	if got := byEndpoint["whk-all"]; !slices.Equal(got, want) {
		t.Errorf("all-events subscriber got %v, want %v", got, want)
	}
}
