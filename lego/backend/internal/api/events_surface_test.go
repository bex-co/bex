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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
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
	rows   []store.ServiceEventRow
	gotFil store.ServiceEventFilter
	gotApp string
	gotTgt string
}

func (f *fakeEventStore) ListServiceEvents(_ context.Context, appID, target, _ string, fil store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	f.gotApp, f.gotTgt, f.gotFil = appID, target, fil
	out := f.rows
	if fil.Limit > 0 && len(out) > fil.Limit {
		out = out[:fil.Limit]
	}
	return out, nil
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

type restEvent struct {
	Event struct {
		ID        string         `json:"id"`
		Timestamp string         `json:"timestamp"`
		ServiceID string         `json:"serviceId"`
		Type      string         `json:"type"`
		Details   map[string]any `json:"details"`
	} `json:"event"`
	Cursor string `json:"cursor"`
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
	// The HTTP transports get their caller Identity from the auth middleware; the
	// in-memory transport has no middleware, so the session context carries it —
	// the same gated Service.List runs either way.
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-x", Method: "session"})
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.MCPServer().Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("mcp client connect: %v", err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_service_events", Arguments: args})
	if err != nil {
		t.Fatalf("call list_service_events: %v", err)
	}
	if res.IsError {
		b, _ := json.Marshal(res.Content)
		t.Fatalf("list_service_events errored: %s", b)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Events []restEvent `json:"events"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("mcp decode: %v (%s)", err, raw)
	}
	return out.Events
}
