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

package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestEdgeCacheAndPipelineEventsAgreeOnEverySurface is w9/m345's parity gate:
// the three edge-cache events and pipeline_minutes_exhausted reach REST, MCP and
// GraphQL with the same ids and types, REST and MCP carry Render's details
// (the edge-cache {manual, user, system} trigger; buildId and the deploy
// trigger), and REST's type filter follows Render's list-events enum, which
// names pipeline_minutes_exhausted but not the edge-cache trio.
func TestEdgeCacheAndPipelineEventsAgreeOnEverySurface(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	cache := func(key string, minute int, verb string, enabledTo *bool) store.ServiceEventRow {
		return store.ServiceEventRow{Key: key, At: at.Add(-time.Duration(minute) * time.Minute), Source: store.EventSourceAudit,
			Verb: verb, Caller: "user-x", AuditChanges: store.AuditChanges{EdgeCacheEnabledTo: enabledTo}}
	}
	fake := &fakeEventStore{rows: []store.ServiceEventRow{
		cache("aud-on:", 0, core.AuditVerbSetCacheProfile, new(true)),
		cache("aud-purge:", 1, core.AuditVerbPurgeCache, nil),
		cache("aud-off:", 2, core.AuditVerbSetCacheProfile, new(false)),
		{Key: "fact:deploy:dep-9:pipeline_minutes_exhausted", At: at.Add(-3 * time.Minute), Source: store.EventSourceFact,
			FactType: events.TypePipelineMinutesExhausted, DeployID: "dep-9", Trigger: store.TriggerManual, Caller: "user-x"},
	}}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	// Emails resolve through the identity reader, as in production: Render's
	// user object requires one.
	h, srv := serverWith(t, base, Deps{EventStore: fake, Identities: metadataIdentities{}})
	wantTypes := []string{events.TypeEdgeCacheEnabled, events.TypeEdgeCachePurged, events.TypeEdgeCacheDisabled, events.TypePipelineMinutesExhausted}

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-11-01T00:00:00Z", testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("REST events: %d %s", res.Code, res.Body.String())
	}
	var rest []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	if len(rest) != len(wantTypes) {
		t.Fatalf("REST events = %d, want %d", len(rest), len(wantTypes))
	}
	cacheTrigger := map[string]any{"manual": true, "system": false, "user": map[string]any{"id": "user-x", "email": "user-x@example.com"}}
	for i, want := range wantTypes {
		if rest[i].Event.Type != want {
			t.Errorf("REST event %d type = %q, want %q", i, rest[i].Event.Type, want)
		}
		if i < 3 && !reflect.DeepEqual(rest[i].Event.Details, map[string]any{"trigger": cacheTrigger}) {
			t.Errorf("%s details = %v, want Render's {trigger: {manual, user, system}} alone", want, rest[i].Event.Details)
		}
	}
	pipeline := rest[3].Event.Details
	trigger, _ := pipeline["trigger"].(map[string]any)
	if pipeline["buildId"] != "dep-9" || trigger["manual"] != true || trigger["firstBuild"] != false || trigger["user"] == nil {
		t.Errorf("pipeline_minutes_exhausted details = %v, want buildId dep-9 and the manual deploy's trigger", pipeline)
	}

	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-11-01T00:00:00Z"})
	gqlData := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-11-01T00:00:00Z") { id type details { deployId edgeCacheTrigger { manual system user { id email } } trigger { manual firstBuild user { id } } } } }`)
	gqlList, _ := gqlData["serviceEvents"].([]any)
	if len(mcpEvents) != len(rest) || len(gqlList) != len(rest) {
		t.Fatalf("MCP %d / GraphQL %d events, want %d", len(mcpEvents), len(gqlList), len(rest))
	}
	for i, r := range rest {
		g := gqlList[i].(map[string]any)
		if m := mcpEvents[i]; m.Event.ID != r.Event.ID || m.Event.Type != r.Event.Type || !reflect.DeepEqual(m.Event.Details, r.Event.Details) {
			t.Errorf("event %d: MCP %+v diverges from REST %+v", i, m.Event, r.Event)
		}
		if g["id"] != r.Event.ID || g["type"] != r.Event.Type {
			t.Errorf("event %d: GraphQL %v diverges from REST %+v", i, g, r.Event)
		}
	}
	gqlCache := gqlList[0].(map[string]any)["details"].(map[string]any)
	if !reflect.DeepEqual(gqlCache["edgeCacheTrigger"], cacheTrigger) || gqlCache["trigger"] != nil {
		t.Errorf("GraphQL edge_cache_enabled details = %v, want edgeCacheTrigger %v and no deploy trigger", gqlCache, cacheTrigger)
	}
	gqlPipeline := gqlList[3].(map[string]any)["details"].(map[string]any)
	if gqlPipeline["deployId"] != "dep-9" || gqlPipeline["edgeCacheTrigger"] != nil || gqlPipeline["trigger"] == nil {
		t.Errorf("GraphQL pipeline_minutes_exhausted details = %v", gqlPipeline)
	}

	// GET /v1/events/{id} hydrates a webhook's data.id to the same event.
	got := do(t, h, "GET", "/v1/events/"+ids.Derive(ids.Event, "aud-purge:"), testToken, "")
	var byID wireEvent
	if err := json.Unmarshal(got.Body.Bytes(), &byID); err != nil || byID.Type != events.TypeEdgeCachePurged ||
		!reflect.DeepEqual(byID.Details, map[string]any{"trigger": cacheTrigger}) {
		t.Errorf("GET /v1/events/{id} = %d %s, want the edge_cache_purged event", got.Code, got.Body.String())
	}

	// Render's list-events type filter: pipeline_minutes_exhausted is in it and
	// pushes the fact filter; the edge-cache trio is not, so REST refuses them
	// as Render does while GraphQL filters by them.
	fake.gotFil = store.ServiceEventFilter{}
	if r := do(t, h, "GET", "/v1/services/web/events?startTime=2026-11-01T00:00:00Z&type=pipeline_minutes_exhausted", testToken, ""); r.Code != http.StatusOK ||
		!slices.Equal(fake.gotFil.FactTypes, []string{events.TypePipelineMinutesExhausted}) {
		t.Errorf("REST ?type=pipeline_minutes_exhausted = %d, pushed %+v", r.Code, fake.gotFil)
	}
	for _, edge := range []string{events.TypeEdgeCacheEnabled, events.TypeEdgeCacheDisabled, events.TypeEdgeCachePurged} {
		if r := do(t, h, "GET", "/v1/services/web/events?startTime=2026-11-01T00:00:00Z&type="+edge, testToken, ""); r.Code != http.StatusBadRequest {
			t.Errorf("REST ?type=%s = %d, want 400 from Render's list-events enum", edge, r.Code)
		}
	}
	fake.gotFil = store.ServiceEventFilter{}
	gql(t, h, `{ serviceEvents(serviceId: "web", type: "edge_cache_disabled", startTime: "2026-11-01T00:00:00Z") { id } }`)
	if !slices.Equal(fake.gotFil.Verbs, []string{core.AuditVerbSetCacheProfile, core.AuditVerbSetPlanEdgeCache}) || fake.gotFil.EdgeCache != store.EdgeCacheFilterDisabled {
		t.Errorf("GraphQL type edge_cache_disabled pushed %+v", fake.gotFil)
	}
}

// TestPlanChangeEdgeCacheEventsAreSystemTriggeredOnEverySurface is w9/088's
// parity gate: a plan change to Free that stopped a service's edge cache, and
// the move back that restarted it, are edge_cache_disabled / _enabled on REST,
// MCP, GraphQL and GET /v1/events/{id} (a webhook's data.id), with Render's
// trigger marked system and naming the member who changed the plan.
func TestPlanChangeEdgeCacheEventsAreSystemTriggeredOnEverySurface(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	toggle := func(key string, minute int, enabledTo bool) store.ServiceEventRow {
		return store.ServiceEventRow{Key: key, At: at.Add(-time.Duration(minute) * time.Minute), Source: store.EventSourceAudit,
			Verb: core.AuditVerbSetPlanEdgeCache, Caller: "user-x", AuditChanges: store.AuditChanges{EdgeCacheEnabledTo: &enabledTo}}
	}
	fake := &fakeEventStore{rows: []store.ServiceEventRow{toggle("aud-paid:", 0, true), toggle("aud-free:", 1, false)}}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake, Identities: metadataIdentities{}})
	wantTypes := []string{events.TypeEdgeCacheEnabled, events.TypeEdgeCacheDisabled}
	systemTrigger := map[string]any{"manual": false, "system": true, "user": map[string]any{"id": "user-x", "email": "user-x@example.com"}}

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-11-01T00:00:00Z", testToken, "")
	var rest []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil || len(rest) != len(wantTypes) {
		t.Fatalf("REST events = %d %s (%v), want %d", res.Code, res.Body.String(), err, len(wantTypes))
	}
	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-11-01T00:00:00Z"})
	gqlList, _ := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-11-01T00:00:00Z") { id type details { edgeCacheTrigger { manual system user { id email } } } } }`)["serviceEvents"].([]any)
	if len(mcpEvents) != len(rest) || len(gqlList) != len(rest) {
		t.Fatalf("MCP %d / GraphQL %d events, want %d", len(mcpEvents), len(gqlList), len(rest))
	}
	for i, want := range wantTypes {
		r := rest[i].Event
		if r.Type != want || !reflect.DeepEqual(r.Details, map[string]any{"trigger": systemTrigger}) {
			t.Errorf("REST event %d = %s %v, want %s with trigger %v", i, r.Type, r.Details, want, systemTrigger)
		}
		if m := mcpEvents[i].Event; m.ID != r.ID || m.Type != r.Type || !reflect.DeepEqual(m.Details, r.Details) {
			t.Errorf("event %d: MCP %+v diverges from REST %+v", i, m, r)
		}
		g := gqlList[i].(map[string]any)
		if g["id"] != r.ID || g["type"] != want || !reflect.DeepEqual(g["details"].(map[string]any)["edgeCacheTrigger"], systemTrigger) {
			t.Errorf("event %d: GraphQL %v, want %s with trigger %v", i, g, want, systemTrigger)
		}
	}

	got := do(t, h, "GET", "/v1/events/"+ids.Derive(ids.Event, "aud-free:"), testToken, "")
	var byID wireEvent
	if err := json.Unmarshal(got.Body.Bytes(), &byID); err != nil || byID.Type != events.TypeEdgeCacheDisabled ||
		!reflect.DeepEqual(byID.Details, map[string]any{"trigger": systemTrigger}) {
		t.Errorf("GET /v1/events/{id} = %d %s, want the system-triggered edge_cache_disabled", got.Code, got.Body.String())
	}
}
