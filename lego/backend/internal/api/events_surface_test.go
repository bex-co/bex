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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// events_surface_test.go is w3/m7's t005/t007: the three adapters over one
// events.Service.List must return the SAME events (ids, types, timestamps,
// details, cursors), and no env-var value may appear on any of them.

// plantedSecret is written into the fixture wherever a value COULD leak if the
// feed ever re-joined a verb's arguments — the audit row's caller, its verb, the
// deploy image. Every surface's raw bytes are then grepped for it. It is not a
// tautology: the fixture's env-var write (secrets.SetEnvVar) is exactly the verb
// whose value this asserts can never surface.
const plantedSecret = "s3cr3t-postgres-password"

// fakeEventStore returns canned composed rows — all three sources' shapes.
type fakeEventStore struct {
	rows              []store.ServiceEventRow
	lookups           map[string]store.ServiceEventLookup
	gotFil            store.ServiceEventFilter
	gotApp            string
	gotTgt            string
	gotEventWorkspace string
}

func (f *fakeEventStore) ListServiceEvents(_ context.Context, appID, target, _ string, fil store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	f.gotApp, f.gotTgt, f.gotFil = appID, target, fil
	out := f.rows
	if fil.Limit > 0 && len(out) > fil.Limit {
		out = out[:fil.Limit]
	}
	return out, nil
}

func (f *fakeEventStore) GetServiceEvent(_ context.Context, workspaceID, eventID string) (store.ServiceEventLookup, error) {
	f.gotEventWorkspace = workspaceID
	if lookup, ok := f.lookups[workspaceID+"\x00"+eventID]; ok {
		return lookup, nil
	}
	for _, row := range f.rows {
		if ids.Derive(ids.Event, row.Key) == eventID && workspaceID == core.DefaultTenant {
			return store.ServiceEventLookup{Event: row, ServiceID: "web"}, nil
		}
	}
	return store.ServiceEventLookup{}, store.ErrNotFound
}

func eventFixture(at time.Time) *fakeEventStore {
	from, to := int32(1), int32(3)
	return &fakeEventStore{rows: []store.ServiceEventRow{
		// An env-var write — the redaction case. The caller is a subject, never a value.
		{Key: "aud-3:", At: at, Source: store.EventSourceAudit, Verb: "secrets.SetEnvVar", Caller: "user-x"},
		{Key: "aud-2:", At: at.Add(-time.Minute), Source: store.EventSourceAudit, Verb: "apps.Suspend", Caller: "user-x"},
		{Key: "dep-1:ended", At: at.Add(-2 * time.Minute), Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: "dep-1", Trigger: store.TriggerAPI, Status: store.DeployLive},
		{Key: "dep-1:started", At: at.Add(-3 * time.Minute), Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerAPI},
		{Key: "fact:autoscaling-1", At: at.Add(-4 * time.Minute), Source: store.EventSourceFact, FactType: string(store.EventFactAutoscalingStarted), FromCount: &from, ToCount: &to},
	}}
}

// eventsApp is a store-managed App (the app-id label the reconciler stamps).
func eventsApp() *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default",
			Labels: map[string]string{store.LabelAppID: "srv-1", core.LabelTenant: "tea-a"},
		},
		Spec: appv1alpha1.AppSpec{Image: "web:v1"},
	}
}

type wireEvent struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	ServiceID string         `json:"serviceId"`
	Type      string         `json:"type"`
	Details   map[string]any `json:"details"`
}

type restEvent struct {
	Event  wireEvent `json:"event"`
	Cursor string    `json:"cursor"`
}

func TestEventSurfaceParity(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: eventFixture(at)})

	// --- REST: Render's bare array of {event, cursor} -------------------------
	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-07-01T00:00:00Z", testToken, "")
	if res.Code != 200 {
		t.Fatalf("REST events: %d %s", res.Code, res.Body.String())
	}
	raw := res.Body.String()
	var rest []restEvent
	if err := json.Unmarshal([]byte(raw), &rest); err != nil {
		t.Fatalf("decode REST (want a BARE ARRAY, Render's envelope for this endpoint): %v — %s", err, raw)
	}
	if len(rest) != 5 {
		t.Fatalf("REST events = %d, want 5", len(rest))
	}

	// Composition: each source row became the right event type, newest first.
	wantTypes := []string{events.TypeEnvVarsChanged, events.TypeSuspenderAdded, events.TypeDeployEnded, events.TypeDeployStarted, events.TypeAutoscalingStarted}
	for i, want := range wantTypes {
		if rest[i].Event.Type != want {
			t.Errorf("event %d type = %q, want %q", i, rest[i].Event.Type, want)
		}
		if rest[i].Event.ServiceID != "web" {
			t.Errorf("event %d serviceId = %q, want web", i, rest[i].Event.ServiceID)
		}
		// Render marks id/timestamp/serviceId/type/details required — all five present.
		if rest[i].Event.ID == "" || rest[i].Event.Timestamp == "" || rest[i].Event.Details == nil || rest[i].Cursor == "" {
			t.Errorf("event %d missing a required field: %+v", i, rest[i])
		}
		if !strings.HasPrefix(rest[i].Event.ID, "evt-") {
			t.Errorf("event %d id = %q, want Render's evt- prefix", i, rest[i].Event.ID)
		}
	}
	// Ids are DERIVED, not minted: the same source row yields the same id on a
	// second read (a client pages and dedupes on it).
	again := do(t, h, "GET", "/v1/services/web/events?startTime=2026-07-01T00:00:00Z", testToken, "")
	var rest2 []restEvent
	if err := json.Unmarshal(again.Body.Bytes(), &rest2); err != nil {
		t.Fatal(err)
	}
	for i := range rest {
		if rest[i].Event.ID != rest2[i].Event.ID || rest[i].Cursor != rest2[i].Cursor {
			t.Errorf("event %d id/cursor not stable across reads: %s/%s then %s/%s",
				i, rest[i].Event.ID, rest[i].Cursor, rest2[i].Event.ID, rest2[i].Cursor)
		}
	}
	// Per-type details: deploy_ended carries Render's deployStatus, deploy_started
	// its trigger object, suspender_added its actor.
	if got := rest[2].Event.Details["deployStatus"]; got != "succeeded" {
		t.Errorf("deploy_ended deployStatus = %v, want succeeded", got)
	}
	trigger, ok := rest[3].Event.Details["trigger"].(map[string]any)
	if !ok || trigger["manual"] != true || trigger["firstBuild"] != false {
		t.Errorf("deploy_started trigger = %v, want manual (an API-triggered deploy)", rest[3].Event.Details["trigger"])
	}
	if got := rest[1].Event.Details["actor"]; got != "user-x" {
		t.Errorf("suspender_added actor = %v, want the caller", got)
	}
	// An env-var change says WHAT changed and WHO, and carries no payload at all.
	if len(rest[0].Event.Details) != 0 {
		t.Errorf("env_vars_changed details = %v, want {} (no key, no value)", rest[0].Event.Details)
	}
	if rest[4].Event.Details["fromInstances"] != float64(1) || rest[4].Event.Details["toInstances"] != float64(3) {
		t.Errorf("autoscaling details = %v, want fromInstances=1 toInstances=3", rest[4].Event.Details)
	}
	if _, wrong := rest[4].Event.Details["from"]; wrong {
		t.Errorf("autoscaling details used manual-scale field names: %v", rest[4].Event.Details)
	}

	// --- GraphQL: the same events -------------------------------------------
	gqlData := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-07-01T00:00:00Z") { id type serviceId timestamp cursor details { deployId deployStatus actor fromCount toCount trigger { manual firstBuild } } } }`)
	gqlList, ok := gqlData["serviceEvents"].([]any)
	if !ok || len(gqlList) != 5 {
		t.Fatalf("GraphQL serviceEvents = %v, want 5 events", gqlData["serviceEvents"])
	}
	gqlAutoscaling := gqlList[4].(map[string]any)["details"].(map[string]any)
	if gqlAutoscaling["fromCount"] != float64(1) || gqlAutoscaling["toCount"] != float64(3) {
		t.Errorf("GraphQL autoscaling details = %v", gqlAutoscaling)
	}
	for i, r := range rest {
		g := gqlList[i].(map[string]any)
		if r.Event.ID != g["id"] || r.Event.Type != g["type"] ||
			r.Event.Timestamp != g["timestamp"] || r.Event.ServiceID != g["serviceId"] || r.Cursor != g["cursor"] {
			t.Errorf("event %d diverges REST vs GraphQL: %+v vs %+v", i, r.Event, g)
		}
	}

	// --- MCP: the same events ------------------------------------------------
	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-07-01T00:00:00Z"})
	if len(mcpEvents) != 5 {
		t.Fatalf("MCP list_service_events = %d events, want 5", len(mcpEvents))
	}
	for i, r := range rest {
		if m := mcpEvents[i]; r.Event.ID != m.Event.ID || r.Event.Type != m.Event.Type ||
			r.Event.Timestamp != m.Event.Timestamp || r.Cursor != m.Cursor {
			t.Errorf("event %d diverges REST vs MCP: %+v vs %+v", i, r.Event, m.Event)
		}
	}
}

func TestGetEventSurfaceParity(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fake := eventFixture(at)
	postgresID := ids.Derive(ids.Event, "aud-postgres-created:")
	fake.lookups = map[string]store.ServiceEventLookup{
		core.DefaultTenant + "\x00" + postgresID: {
			Event:     store.ServiceEventRow{Key: "aud-postgres-created:", At: at.Add(-time.Hour), Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresCreated, Caller: "user-x"},
			ServiceID: "dpg-1",
		},
	}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})

	tests := []struct {
		name          string
		id            string
		wantType      string
		wantServiceID string
	}{
		{name: "service deploy", id: ids.Derive(ids.Event, "dep-1:"+store.EventPhaseStarted), wantType: events.TypeDeployStarted, wantServiceID: "web"},
		{name: "typed lifecycle fact", id: ids.Derive(ids.Event, "fact:autoscaling-1"), wantType: events.TypeAutoscalingStarted, wantServiceID: "web"},
		{name: "Postgres audit event", id: postgresID, wantType: events.TypePostgresCreated, wantServiceID: "dpg-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := do(t, h, "GET", "/v1/events/"+tt.id, testToken, "")
			if res.Code != http.StatusOK {
				t.Fatalf("REST get event: %d %s", res.Code, res.Body.String())
			}
			var rest wireEvent
			if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil {
				t.Fatalf("decode REST event: %v", err)
			}
			if rest.ID != tt.id || rest.Type != tt.wantType || rest.ServiceID != tt.wantServiceID || rest.Timestamp == "" || rest.Details == nil {
				t.Errorf("REST event = %+v, want id=%s type=%s serviceId=%s", rest, tt.id, tt.wantType, tt.wantServiceID)
			}

			query := fmt.Sprintf(`{ serviceEvent(id: %q) { id type serviceId timestamp details { deployId trigger { manual } } } }`, tt.id)
			gqlEvent, ok := gql(t, h, query)["serviceEvent"].(map[string]any)
			if !ok {
				t.Fatalf("GraphQL serviceEvent did not return an object")
			}
			if gqlEvent["id"] != rest.ID || gqlEvent["type"] != rest.Type || gqlEvent["serviceId"] != rest.ServiceID || gqlEvent["timestamp"] != rest.Timestamp {
				t.Errorf("GraphQL event diverges from REST: %v vs %+v", gqlEvent, rest)
			}

			mcpEvent := callGetServiceEvent(t, srv, tt.id)
			if mcpEvent.ID != rest.ID || mcpEvent.Type != rest.Type || mcpEvent.ServiceID != rest.ServiceID || mcpEvent.Timestamp != rest.Timestamp {
				t.Errorf("MCP event diverges from REST: %+v vs %+v", mcpEvent, rest)
			}
		})
	}

	if fake.gotEventWorkspace != core.DefaultTenant {
		t.Errorf("single-event store lookup workspace = %q, want %q", fake.gotEventWorkspace, core.DefaultTenant)
	}
}

func TestGetEventRESTHonorsNamedWorkspace(t *testing.T) {
	at := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	eventID := ids.Derive(ids.Event, "aud-named-workspace:")
	fake := &fakeEventStore{lookups: map[string]store.ServiceEventLookup{
		"tea-b\x00" + eventID: {
			Event: store.ServiceEventRow{
				Key: "aud-named-workspace:", At: at, Source: store.EventSourceAudit,
				Verb: core.AuditVerbPostgresCreated, Caller: "user-x",
			},
			ServiceID: "dpg-bravo",
		},
	}}
	base := &core.Base{
		Client: fakeClient(eventsApp()), Namespace: "default",
		Authz: &fakeChecker{allow: true}, Workspace: twoWorkspaceResolver{},
	}
	h, _ := serverWith(t, base, Deps{EventStore: fake})

	res := do(t, h, http.MethodGet, "/v1/events/"+eventID+"?ownerId=tea-b", testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("named-workspace event: %d %s", res.Code, res.Body.String())
	}
	if fake.gotEventWorkspace != "tea-b" {
		t.Fatalf("event lookup workspace = %q, want tea-b", fake.gotEventWorkspace)
	}
}

// TestLifecycleFactStatusAcrossSurfaces proves a deploy-lifecycle fact's status
// (w7/m66: build_ended / pre_deploy_ended / job_run_ended) surfaces identically
// on all three adapters as details.status.
func TestLifecycleFactStatusAcrossSurfaces(t *testing.T) {
	at := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	fake := &fakeEventStore{rows: []store.ServiceEventRow{{
		Key: "fact:deploy:dep-1:build_ended", At: at, Source: store.EventSourceFact,
		FactType: string(store.EventFactBuildEnded), FactStatus: "failed", DeployID: "dep-1",
	}}}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-07-01T00:00:00Z", testToken, "")
	var rest []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil || len(rest) != 1 {
		t.Fatalf("REST events = %s (err %v)", res.Body.String(), err)
	}
	if rest[0].Event.Type != events.TypeBuildEnded || rest[0].Event.Details["status"] != "failed" {
		t.Errorf("REST build_ended details = %+v, want type=build_ended status=failed", rest[0].Event)
	}

	gqlData := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-07-01T00:00:00Z") { type details { status } } }`)
	gqlList, ok := gqlData["serviceEvents"].([]any)
	if !ok || len(gqlList) != 1 {
		t.Fatalf("GraphQL serviceEvents = %v", gqlData["serviceEvents"])
	}
	if g := gqlList[0].(map[string]any)["details"].(map[string]any); g["status"] != "failed" {
		t.Errorf("GraphQL status = %v, want failed", g["status"])
	}

	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-07-01T00:00:00Z"})
	if len(mcpEvents) != 1 || mcpEvents[0].Event.Details["status"] != "failed" {
		t.Errorf("MCP build_ended details = %+v, want status=failed", mcpEvents)
	}
}

// TestEventsNeverCarryValues is the DoD's redaction clause: a planted secret
// value cannot appear on ANY surface.
//
// The plants are deliberate. Every source column an event's projection actually
// READS is either mapped through a closed vocabulary (a verb → an event type; a
// deploy status → succeeded/failed; a trigger → six booleans) or is a resource
// name. So even a row whose free-text columns hold a secret — which the real
// store cannot produce, since audit rows never carry verb arguments — cannot
// serialize one: the projection has nowhere to put it. That is the property
// under test, and it is what makes the guarantee structural rather than a filter
// somebody has to remember to apply.
func TestEventsNeverCarryValues(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fake := eventFixture(at)
	fake.rows[0].Verb = plantedSecret    // an unmapped verb: yields no type, never echoed
	fake.rows[2].Status = plantedSecret  // not "live" ⇒ renders as "failed", never echoed
	fake.rows[3].Trigger = plantedSecret // not "create"/"api" ⇒ six false booleans
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})

	rest := do(t, h, "GET", "/v1/services/web/events?startTime=2026-07-01T00:00:00Z", testToken, "").Body.String()
	if strings.Contains(rest, plantedSecret) {
		t.Errorf("REST events leaked a value:\n%s", rest)
	}
	gqlRaw, _ := json.Marshal(gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-07-01T00:00:00Z") { id type details { deployId deployStatus actor triggeredByUser trigger { manual } } } }`))
	if strings.Contains(string(gqlRaw), plantedSecret) {
		t.Errorf("GraphQL serviceEvents leaked a value:\n%s", gqlRaw)
	}
	mcpRaw, _ := json.Marshal(callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-07-01T00:00:00Z"}))
	if strings.Contains(string(mcpRaw), plantedSecret) {
		t.Errorf("MCP list_service_events leaked a value:\n%s", mcpRaw)
	}
}

// TestEventsAuthMatrix is the DoD's error surface: unknown service 404,
// non-member 403, store-less 503 — omitted, never faked.
func TestEventsAuthMatrix(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	t.Run("unknown service => 404", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		if code := do(t, h, "GET", "/v1/services/nope/events", testToken, "").Code; code != 404 {
			t.Errorf("unknown service => 404, got %d", code)
		}
	})

	t.Run("non-member => 403", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: false}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		if code := do(t, h, "GET", "/v1/services/web/events", testToken, "").Code; code != 403 {
			t.Errorf("non-member => 403, got %d", code)
		}
	})

	t.Run("store-less => 503", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: nil})
		if code := do(t, h, "GET", "/v1/services/web/events", testToken, "").Code; code != 503 {
			t.Errorf("store-less => 503, got %d", code)
		}
	})

	t.Run("malformed cursor => 400", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		if code := do(t, h, "GET", "/v1/services/web/events?cursor=not-a-cursor", testToken, "").Code; code != 400 {
			t.Errorf("malformed cursor => 400, got %d", code)
		}
	})

	t.Run("malformed event id => 400", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		if code := do(t, h, "GET", "/v1/events/not-an-event", testToken, "").Code; code != http.StatusBadRequest {
			t.Errorf("malformed event id => 400, got %d", code)
		}
	})

	t.Run("missing event => 404", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		missingID := ids.Derive(ids.Event, "missing-event:")
		if code := do(t, h, "GET", "/v1/events/"+missingID, testToken, "").Code; code != http.StatusNotFound {
			t.Errorf("missing event => 404, got %d", code)
		}
	})

	t.Run("foreign event is indistinguishable from missing => 404", func(t *testing.T) {
		foreignID := ids.Derive(ids.Event, "foreign-event:")
		fake := eventFixture(at)
		fake.lookups = map[string]store.ServiceEventLookup{
			"tea-foreign\x00" + foreignID: {
				Event:     store.ServiceEventRow{Key: "foreign-event:", At: at, Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresCreated},
				ServiceID: "dpg-foreign",
			},
		}
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: fake})
		if code := do(t, h, "GET", "/v1/events/"+foreignID, testToken, "").Code; code != http.StatusNotFound {
			t.Errorf("foreign event => 404, got %d", code)
		}
	})

	t.Run("single event non-member => 403", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: false}}
		h, _ := serverWith(t, base, Deps{EventStore: eventFixture(at)})
		eventID := ids.Derive(ids.Event, "dep-1:"+store.EventPhaseStarted)
		if code := do(t, h, "GET", "/v1/events/"+eventID, testToken, "").Code; code != http.StatusForbidden {
			t.Errorf("single event non-member => 403, got %d", code)
		}
	})

	t.Run("single event store-less => 503", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{EventStore: nil})
		eventID := ids.Derive(ids.Event, "dep-1:"+store.EventPhaseStarted)
		if code := do(t, h, "GET", "/v1/events/"+eventID, testToken, "").Code; code != http.StatusServiceUnavailable {
			t.Errorf("single event store-less => 503, got %d", code)
		}
	})
}

// TestEventsWindowAndPaging pins the two params a Render client's behavior
// depends on: the default window (now-1h, Render's own) and the OpenAPI limit
// range enforced before the event store runs.
func TestEventsWindowAndPaging(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return at }
	fake := eventFixture(at)
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}, Clock: now}
	h, _ := serverWith(t, base, Deps{EventStore: fake})

	// No startTime ⇒ Render's default: the last hour, not all of history.
	do(t, h, "GET", "/v1/services/web/events", testToken, "")
	if want := at.Add(-events.DefaultWindow); !fake.gotFil.Since.Equal(want) {
		t.Errorf("default window Since = %s, want %s (Render's now-1h)", fake.gotFil.Since, want)
	}
	if fake.gotFil.Limit != core.DefaultPageLimit {
		t.Errorf("default limit = %d, want %d", fake.gotFil.Limit, core.DefaultPageLimit)
	}
	// The feed is keyed on BOTH identifiers: the store row id (deploys) and the
	// service target (audit rows).
	if fake.gotApp != "srv-1" || fake.gotTgt != core.ServiceTarget("web") {
		t.Errorf("store keyed on appID=%q target=%q", fake.gotApp, fake.gotTgt)
	}

	// An out-of-contract limit is rejected before the handler/store rather than
	// silently normalized to Render's maximum.
	badLimit := do(t, h, "GET", "/v1/services/web/events?limit=9999", testToken, "")
	if badLimit.Code != http.StatusBadRequest {
		t.Fatalf("limit=9999 = %d, want 400", badLimit.Code)
	}
	if fake.gotFil.Limit != core.DefaultPageLimit {
		t.Errorf("rejected limit reached the store: got %d, want prior %d", fake.gotFil.Limit, core.DefaultPageLimit)
	}

	// A page's last cursor round-trips into the next page's resume position.
	res := do(t, h, "GET", "/v1/services/web/events?limit=2&startTime=2026-07-01T00:00:00Z", testToken, "")
	var page []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || len(page) != 2 {
		t.Fatalf("page = %+v (err %v), want 2", page, err)
	}
	do(t, h, "GET", "/v1/services/web/events?limit=2&startTime=2026-07-01T00:00:00Z&cursor="+page[1].Cursor, testToken, "")
	if fake.gotFil.AfterKey != "aud-2:" || !fake.gotFil.AfterAt.Equal(at.Add(-time.Minute)) {
		t.Errorf("cursor resumed at key=%q at=%s, want the keyset of the last item of page 1", fake.gotFil.AfterKey, fake.gotFil.AfterAt)
	}
}

// callListServiceEvents drives the MCP tool over an in-memory transport — the
// third surface, exercised as a client would.
func callListServiceEvents(t *testing.T, srv *Server, args map[string]any) []restEvent {
	t.Helper()
	out := callTool[struct {
		Events []restEvent `json:"events"`
	}](t, mcpSessionAs(t, srv, "user-x"), "list_service_events", args)
	return out.Events
}

func callGetServiceEvent(t *testing.T, srv *Server, eventID string) wireEvent {
	t.Helper()
	out := callTool[struct {
		Event wireEvent `json:"event"`
	}](t, mcpSessionAs(t, srv, "user-x"), "get_service_event", map[string]any{"id": eventID})
	return out.Event
}

// TestServiceEventSurfaceCarriesDriftedTypes probes the three API surfaces with
// the exact types w6/m122 found missing from the DASHBOARD catalog. The point is
// the negative result: the API was never the defect, so this pins that down
// rather than leaving "REST already returns them" as an inherited assumption.
// custom_domain_verified was read live out of GET /v1/services/{id}/events on
// 2026-08-27; the four disk_* types were only ever traced through eventTypes
// (the QA workspace has no service with a persistent disk), so disk_attached
// stands in for that family here.
func TestServiceEventSurfaceCarriesDriftedTypes(t *testing.T) {
	at := time.Date(2026, 8, 27, 11, 54, 7, 0, time.UTC)
	fake := &fakeEventStore{rows: []store.ServiceEventRow{
		{Key: "aud-domain-verified:", At: at, Source: store.EventSourceAudit, Verb: "apps.VerifyDomain", Caller: "user-x"},
		{Key: "aud-disk-attached:", At: at.Add(-time.Minute), Source: store.EventSourceAudit, Verb: "apps.AddDisk", Caller: "user-x"},
	}}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})

	wantTypes := []string{events.TypeCustomDomainVerified, events.TypeDiskAttached}

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-08-01T00:00:00Z", testToken, "")
	if res.Code != 200 {
		t.Fatalf("REST events: %d %s", res.Code, res.Body.String())
	}
	var rest []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil {
		t.Fatalf("decode REST: %v", err)
	}
	if len(rest) != len(wantTypes) {
		t.Fatalf("REST events = %d, want %d", len(rest), len(wantTypes))
	}
	for i, want := range wantTypes {
		if rest[i].Event.Type != want {
			t.Errorf("REST event %d type = %q, want %q", i, rest[i].Event.Type, want)
		}
	}

	gqlData := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-08-01T00:00:00Z") { id type serviceId timestamp cursor } }`)
	gqlList, ok := gqlData["serviceEvents"].([]any)
	if !ok || len(gqlList) != len(wantTypes) {
		t.Fatalf("GraphQL serviceEvents = %v, want %d events", gqlData["serviceEvents"], len(wantTypes))
	}

	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-08-01T00:00:00Z"})
	if len(mcpEvents) != len(wantTypes) {
		t.Fatalf("MCP list_service_events = %d events, want %d", len(mcpEvents), len(wantTypes))
	}
	for i, r := range rest {
		g := gqlList[i].(map[string]any)
		if r.Event.Type != g["type"] || r.Event.ID != g["id"] {
			t.Errorf("event %d diverges REST vs GraphQL: %+v vs %+v", i, r.Event, g)
		}
		if m := mcpEvents[i]; r.Event.Type != m.Event.Type || r.Event.ID != m.Event.ID {
			t.Errorf("event %d diverges REST vs MCP: %+v vs %+v", i, r.Event, m.Event)
		}
	}

	// The ?type= FILTER is a different question, and its answer is Render's, not
	// bex's: the pinned Render contract declares `type` as a 39-value enum, so
	// the request validator refuses any bex-named type before the handler runs.
	// (service_moved, asserted below, is refused on this parameter for the same
	// reason — the dashboard filters it client-side like every bex-named type.)
	// disk_updated is IN Render's enum and narrows normally; custom_domain_verified
	// is not, and is refused 400. The control below proves that refusal is the
	// contract's standing behaviour for every bex-named type rather than anything
	// specific to the types w6/m122 surfaced.
	filtered := do(t, h, "GET", "/v1/services/web/events?startTime=2026-08-01T00:00:00Z&type="+events.TypeDiskUpdated, testToken, "")
	if filtered.Code != 200 {
		t.Fatalf("REST events?type=%s: %d %s", events.TypeDiskUpdated, filtered.Code, filtered.Body.String())
	}
	for _, bexNamed := range []string{events.TypeCustomDomainVerified, events.TypeCustomDomainAdded, events.TypeEnvVarsChanged, events.TypeServiceMoved} {
		refused := do(t, h, "GET", "/v1/services/web/events?startTime=2026-08-01T00:00:00Z&type="+bexNamed, testToken, "")
		if refused.Code != http.StatusBadRequest {
			t.Errorf("REST events?type=%s = %d, want 400 from the pinned Render enum", bexNamed, refused.Code)
		}
	}
}

// TestServiceMovedEventCarriesPlacementDetails is w6/m134's cross-surface
// contract: both move verbs project onto ONE service_moved type, the typed
// placement pair reaches REST/GraphQL/MCP under the same spelling, and an
// absent placement side is OMITTED (assign shows only *To, unassign only
// *From) rather than serialized as an empty string.
func TestServiceMovedEventCarriesPlacementDetails(t *testing.T) {
	at := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	prjFrom, prjTo := "prj-old", "prj-new"
	envFrom, envTo := "env-old", "env-new"
	fake := &fakeEventStore{rows: []store.ServiceEventRow{
		// A full move through the environments funnel: both dimensions change.
		{Key: "aud-move:", At: at, Source: store.EventSourceAudit, Verb: core.AuditVerbEnvironmentServiceMoved, Caller: "user-x",
			ProjectFrom: &prjFrom, ProjectTo: &prjTo, EnvironmentFrom: &envFrom, EnvironmentTo: &envTo},
		// An unassign through the projects funnel: only the from side exists.
		{Key: "aud-unassign:", At: at.Add(-time.Minute), Source: store.EventSourceAudit, Verb: core.AuditVerbProjectServiceMoved, Caller: "user-x",
			ProjectFrom: &prjFrom, EnvironmentFrom: &envFrom},
	}}
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-08-01T00:00:00Z", testToken, "")
	if res.Code != 200 {
		t.Fatalf("REST events: %d %s", res.Code, res.Body.String())
	}
	var rest []restEvent
	if err := json.Unmarshal(res.Body.Bytes(), &rest); err != nil {
		t.Fatalf("decode REST: %v", err)
	}
	if len(rest) != 2 {
		t.Fatalf("REST events = %d, want 2", len(rest))
	}
	for i, r := range rest {
		if r.Event.Type != events.TypeServiceMoved {
			t.Fatalf("REST event %d type = %q, want %q (both funnels' verbs map to one type)", i, r.Event.Type, events.TypeServiceMoved)
		}
	}
	move, unassign := rest[0].Event.Details, rest[1].Event.Details
	if move["projectFrom"] != prjFrom || move["projectTo"] != prjTo ||
		move["environmentFrom"] != envFrom || move["environmentTo"] != envTo {
		t.Errorf("move details = %v, want the full placement pair", move)
	}
	if unassign["projectFrom"] != prjFrom || unassign["environmentFrom"] != envFrom {
		t.Errorf("unassign details = %v, want the from side", unassign)
	}
	for _, absent := range []string{"projectTo", "environmentTo"} {
		if _, ok := unassign[absent]; ok {
			t.Errorf("unassign details carry %q = %v, want the absent side omitted", absent, unassign[absent])
		}
	}

	gqlData := gql(t, h, `{ serviceEvents(serviceId: "web", startTime: "2026-08-01T00:00:00Z") { id type details { projectFrom projectTo environmentFrom environmentTo } } }`)
	gqlList, ok := gqlData["serviceEvents"].([]any)
	if !ok || len(gqlList) != 2 {
		t.Fatalf("GraphQL serviceEvents = %v, want 2 events", gqlData["serviceEvents"])
	}
	gqlMove := gqlList[0].(map[string]any)["details"].(map[string]any)
	if gqlMove["projectFrom"] != prjFrom || gqlMove["projectTo"] != prjTo ||
		gqlMove["environmentFrom"] != envFrom || gqlMove["environmentTo"] != envTo {
		t.Errorf("GraphQL move details = %v, want the full placement pair", gqlMove)
	}
	gqlUnassign := gqlList[1].(map[string]any)["details"].(map[string]any)
	if gqlUnassign["projectFrom"] != prjFrom || gqlUnassign["projectTo"] != nil {
		t.Errorf("GraphQL unassign details = %v, want from side only (absent = null)", gqlUnassign)
	}

	mcpEvents := callListServiceEvents(t, srv, map[string]any{"serviceId": "web", "startTime": "2026-08-01T00:00:00Z"})
	if len(mcpEvents) != 2 {
		t.Fatalf("MCP list_service_events = %d events, want 2", len(mcpEvents))
	}
	for i, r := range rest {
		m := mcpEvents[i]
		if m.Event.ID != r.Event.ID || m.Event.Type != r.Event.Type ||
			m.Event.Details["projectFrom"] != r.Event.Details["projectFrom"] ||
			m.Event.Details["environmentTo"] != r.Event.Details["environmentTo"] {
			t.Errorf("event %d diverges REST vs MCP: %+v vs %+v", i, r.Event, m.Event)
		}
	}
}
