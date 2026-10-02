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
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// events_multitype_test.go is w1/m165's cross-surface contract for Render's
// multi-type event filter and its list_events MCP tool.
//
// Render's REST `type` parameter declares one enum value, but the API takes a
// comma-separated list — which is how Render's own MCP server's list_events
// drives it (render-mcp-server pkg/event/tools.go). bex's filter held one type,
// and its strict Render gate 400'd the comma form. Now every surface asks the
// store for the same union.

// assertMultiTypePushDown checks what `deploy_ended,server_failed,suspender_added`
// pushes down: one of each source kind, so a surface dropping any member shows.
func assertMultiTypePushDown(t *testing.T, surface string, got store.ServiceEventFilter) {
	t.Helper()
	if !slices.Equal(got.Phases, []string{store.EventPhaseEnded}) ||
		!slices.Equal(got.FactTypes, []string{events.TypeServerFailed}) ||
		!slices.Equal(got.Verbs, []string{"apps.Suspend"}) {
		t.Errorf("%s pushed down verbs=%v phases=%v facts=%v, want the union [apps.Suspend] [%s] [%s]",
			surface, got.Verbs, got.Phases, got.FactTypes, store.EventPhaseEnded, events.TypeServerFailed)
	}
}

func TestEventsMultiTypeFilterAgreesAcrossSurfaces(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fake := eventFixture(at)
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, srv := serverWith(t, base, Deps{EventStore: fake})
	const list = "deploy_ended,server_failed,suspender_added"

	res := do(t, h, "GET", "/v1/services/web/events?startTime=2026-07-01T00:00:00Z&type="+list, testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("REST ?type=%s = %d %s, want 200 — Render's API accepts a comma list", list, res.Code, res.Body.String())
	}
	assertMultiTypePushDown(t, "REST", fake.gotFil)

	fake.gotFil = store.ServiceEventFilter{}
	gql(t, h, `{ serviceEvents(serviceId: "web", type: "`+list+`", startTime: "2026-07-01T00:00:00Z") { id } }`)
	assertMultiTypePushDown(t, "GraphQL", fake.gotFil)

	fake.gotFil = store.ServiceEventFilter{}
	cs := mcpSessionAs(t, srv, "user-x")
	callTool[map[string]any](t, cs, "list_events", map[string]any{
		"serviceId": "web", "startTime": "2026-07-01T00:00:00Z",
		"eventTypes": []string{"deploy_ended", "server_failed", "suspender_added"},
	})
	assertMultiTypePushDown(t, "MCP list_events", fake.gotFil)

	// The alias reads its single `type` string in the same comma wire.
	fake.gotFil = store.ServiceEventFilter{}
	callTool[map[string]any](t, cs, "list_service_events", map[string]any{
		"serviceId": "web", "startTime": "2026-07-01T00:00:00Z", "type": list,
	})
	assertMultiTypePushDown(t, "MCP list_service_events", fake.gotFil)
}

// TestEventsMultiTypeRESTStillValidatesEveryType keeps the single-type rule for
// each element: the pinned Render enum refuses a bex-named or misspelled type
// with a 400 that names the allowed set, even when it hides inside a list.
func TestEventsMultiTypeRESTStillValidatesEveryType(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fake := eventFixture(at)
	base := &core.Base{Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{EventStore: fake})

	for _, bad := range []string{
		"deploy_ended," + events.TypeEnvVarsChanged, // bex-named, outside Render's enum
		"deploy_ended,deploy_endd",                  // misspelled
		events.TypeDiskRestored + ",deploy_ended",   // position does not matter
	} {
		fake.gotFil = store.ServiceEventFilter{}
		res := do(t, h, "GET", "/v1/services/web/events?type="+bad, testToken, "")
		if res.Code != http.StatusBadRequest {
			t.Errorf("?type=%s = %d, want 400 from the pinned Render enum", bad, res.Code)
			continue
		}
		if !strings.Contains(res.Body.String(), "type must be one of") {
			t.Errorf("?type=%s body = %s, want the enum named", bad, res.Body.String())
		}
		if fake.gotFil.Limit != 0 {
			t.Errorf("?type=%s reached the store: %+v", bad, fake.gotFil)
		}
	}
}

// TestListEventsWindowFollowsUpstream: list_events defaults to Render's MCP
// lookback (7 days), while the list_service_events alias keeps REST's now-1h —
// retiring or changing the alias would break its existing callers.
func TestListEventsWindowFollowsUpstream(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fake := eventFixture(at)
	base := &core.Base{
		Client: fakeClient(eventsApp()), Namespace: "default", Authz: &fakeChecker{allow: true},
		Clock: func() time.Time { return at },
	}
	_, srv := serverWith(t, base, Deps{EventStore: fake})
	cs := mcpSessionAs(t, srv, "user-x")

	callTool[map[string]any](t, cs, "list_events", map[string]any{"serviceId": "web"})
	if want := at.Add(-7 * 24 * time.Hour); !fake.gotFil.Since.Equal(want) {
		t.Errorf("list_events default Since = %s, want %s (upstream's 7-day lookback)", fake.gotFil.Since, want)
	}

	callTool[map[string]any](t, cs, "list_events", map[string]any{"serviceId": "web", "startTime": "2026-07-01T00:00:00Z"})
	if want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC); !fake.gotFil.Since.Equal(want) {
		t.Errorf("list_events explicit Since = %s, want %s", fake.gotFil.Since, want)
	}

	callTool[map[string]any](t, cs, "list_service_events", map[string]any{"serviceId": "web"})
	if want := at.Add(-events.DefaultWindow); !fake.gotFil.Since.Equal(want) {
		t.Errorf("list_service_events default Since = %s, want %s (unchanged alias)", fake.gotFil.Since, want)
	}

	// serviceId is required, as upstream declares it.
	callToolError(t, cs, "list_events", map[string]any{"eventTypes": []string{"deploy_ended"}})
}

// TestListEventsCommaListIsOperationScoped: the comma-list concession is made
// on list-events' own copy of the parameter. The pinned component stays the
// single-value schema Render publishes, so no other operation inherits it.
func TestListEventsCommaListIsOperationScoped(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(renderCommaListQueryCompatibility["list-events"], "type") {
		t.Fatal("list-events `type` is not declared a comma list — Render's multi-type wire would 400 at the gate")
	}
	for operationID, names := range renderCommaListQueryCompatibility {
		item, operation := findRenderOperation(t, contract.document, operationID)
		for _, name := range names {
			found := false
			for _, ref := range append(append(openapi3.Parameters{}, item.Parameters...), operation.Parameters...) {
				if ref.Value == nil || ref.Value.Name != name {
					continue
				}
				found = true
				p := ref.Value
				if p.Style != "form" || p.Explode == nil || *p.Explode || p.Schema == nil ||
					p.Schema.Value == nil || !p.Schema.Value.Type.Is("array") || p.Schema.Value.Items == nil ||
					len(p.Schema.Value.Items.Value.Enum) == 0 {
					t.Errorf("%s %s is not a form/explode=false array of the enum: %+v", operationID, name, p)
				}
				if ref.Ref != "" {
					t.Errorf("%s %s still points at the shared component %s", operationID, name, ref.Ref)
				}
			}
			if !found {
				t.Errorf("%s compatibility parameter %s is absent", operationID, name)
			}
		}
	}
	shared := contract.document.Components.Parameters["eventTypeParam"]
	if shared == nil || shared.Value.Style != "" || shared.Value.Explode != nil ||
		shared.Value.Schema.Value.Items != nil || len(shared.Value.Schema.Value.AnyOf) != 1 {
		t.Errorf("the shared eventTypeParam component was mutated: %+v", shared)
	}
}
