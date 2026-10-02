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
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestPGEventsMultiTypePages drives the composed adapters over real SQL. A
// mixed-source page must exclude unrelated, out-of-window and foreign rows
// before LIMIT, and each returned cursor must resume strictly after its row.
func TestPGEventsMultiTypePages(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	tenant, err := st.CreateWorkspace(ctx, "event-pages-"+uniqueRunSuffix(), store.PlanHobby, "user-x")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	foreign, err := st.CreateWorkspace(ctx, "event-foreign-"+uniqueRunSuffix(), store.PlanHobby, "other-user")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, foreign.ID)
		_ = st.DeleteTenant(context.Background(), foreign.ID)
	})
	app, err := st.CreateApp(ctx, store.App{
		TenantID: tenant.ID, Name: "web", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `UPDATE apps SET created_at = $2 WHERE id = $1`, app.ID, at.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE deploys SET started_at = $2, finished_at = $3, status = 'live' WHERE id = $1`,
		app.FirstDeployID, at.Add(-20*time.Minute), at.Add(-6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	factKey := "test-failure:" + app.ID
	if _, err := st.InsertServiceEventFact(ctx, store.ServiceEventFact{
		SourceKey: factKey, AppID: app.ID, Type: store.EventFactServerFailed, At: at.Add(-4 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	recordAudit := func(workspace, verb string, minutes int) string {
		t.Helper()
		auditID := ids.New(ids.Audit)
		if _, err := pool.Exec(ctx, `
			INSERT INTO audit_events (id, workspace_id, resource, target, verb, caller, outcome, at)
			VALUES ($1, $2, $3, $4, $5, 'user-x', 'allowed', $6)`,
			auditID, workspace, core.WorkspaceObject(workspace), core.ServiceTarget(app.ID), verb,
			at.Add(-time.Duration(minutes)*time.Minute)); err != nil {
			t.Fatal(err)
		}
		return auditID + ":"
	}
	auditKey := recordAudit(tenant.ID, "apps.Suspend", 2)
	recordAudit(tenant.ID, "apps.Scale", 3)                   // in-window but wrong type
	recordAudit(foreign.ID, "apps.Suspend", 3)                // matching target/type, wrong owner
	recordAudit(tenant.ID, "apps.Suspend", 10)                // before startTime
	recordAudit(tenant.ID, "apps.Suspend", 1)                 // after endTime
	oldKey := recordAudit(tenant.ID, "apps.Suspend", 9*24*60) // cursor-only page crosses 7 days

	cr := eventsApp()
	cr.Labels[store.LabelAppID], cr.Labels[core.LabelTenant] = app.ID, tenant.ID
	base := &core.Base{
		Client: fakeClient(cr), Namespace: "default", Authz: &fakeChecker{allow: true},
		Clock: func() time.Time { return at },
	}
	h, srv := serverWith(t, base, Deps{EventStore: st})
	cs := mcpSessionAs(t, srv, "user-x")
	start, end := at.Add(-8*time.Minute).Format(time.RFC3339), at.Add(-90*time.Second).Format(time.RFC3339)
	types := []string{events.TypeDeployEnded, events.TypeServerFailed, events.TypeSuspenderAdded}
	wantIDs := []string{ids.Derive(ids.Event, auditKey), ids.Derive(ids.Event, "fact:"+factKey), ids.Derive(ids.Event, app.FirstDeployID+":ended")}
	wantTypes := []string{events.TypeSuspenderAdded, events.TypeServerFailed, events.TypeDeployEnded}

	unwrap := func(page []restEvent) ([]wireEvent, string) {
		out := make([]wireEvent, 0, len(page))
		cursor := ""
		for _, item := range page {
			out = append(out, item.Event)
			cursor = item.Cursor
		}
		return out, cursor
	}
	readers := []struct {
		name string
		read func(t *testing.T, cursor string) ([]wireEvent, string)
	}{
		{"REST", func(t *testing.T, cursor string) ([]wireEvent, string) {
			query := url.Values{"type": {strings.Join(types, ",")}, "startTime": {start}, "endTime": {end}, "limit": {"2"}, "cursor": {cursor}}
			res := do(t, h, http.MethodGet, "/v1/services/web/events?"+query.Encode(), testToken, "")
			if res.Code != http.StatusOK {
				t.Fatalf("REST = %d: %s", res.Code, res.Body.String())
			}
			var page []restEvent
			if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			return unwrap(page)
		}},
		{"GraphQL", func(t *testing.T, cursor string) ([]wireEvent, string) {
			result := gql(t, h, fmt.Sprintf(`{serviceEvents(serviceId:"web",type:%q,startTime:%q,endTime:%q,limit:2,cursor:%q){id type cursor}}`, strings.Join(types, ","), start, end, cursor))
			var page []wireEvent
			last := ""
			for _, raw := range result["serviceEvents"].([]any) {
				row := raw.(map[string]any)
				page = append(page, wireEvent{ID: row["id"].(string), Type: row["type"].(string)})
				last = row["cursor"].(string)
			}
			return page, last
		}},
		{"list_events", func(t *testing.T, cursor string) ([]wireEvent, string) {
			return callRenderEvents(t, cs, map[string]any{"serviceId": "web", "eventTypes": types, "startTime": start, "endTime": end, "limit": 2, "cursor": cursor})
		}},
		{"list_service_events", func(t *testing.T, cursor string) ([]wireEvent, string) {
			out := callTool[struct{ Events []restEvent }](t, cs, "list_service_events", map[string]any{
				"serviceId": "web", "type": strings.Join(types, ","), "startTime": start, "endTime": end, "limit": 2, "cursor": cursor,
			})
			return unwrap(out.Events)
		}},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			cursor := ""
			var gotIDs, gotTypes []string
			for _, wantCount := range []int{2, 1, 0} {
				page, next := reader.read(t, cursor)
				if len(page) != wantCount {
					t.Fatalf("page after %q has %d events, want %d: %+v", cursor, len(page), wantCount, page)
				}
				for _, event := range page {
					gotIDs, gotTypes = append(gotIDs, event.ID), append(gotTypes, event.Type)
				}
				if (wantCount == 0) != (next == "") || (next != "" && next == cursor) {
					t.Fatalf("page cursor did not advance/terminate: %q -> %q", cursor, next)
				}
				cursor = next
			}
			if !slices.Equal(gotIDs, wantIDs) || !slices.Equal(gotTypes, wantTypes) {
				t.Fatalf("pages = %v %v, want %v %v", gotIDs, gotTypes, wantIDs, wantTypes)
			}
		})
	}
	for _, eventTypes := range [][]string{{"unknown_event"}, {events.TypeDeployStarted}} {
		page, _ := callRenderEvents(t, cs, map[string]any{"serviceId": "web", "eventTypes": eventTypes, "startTime": start, "endTime": end})
		if len(page) != 0 {
			t.Fatalf("filter %v unexpectedly returned %+v", eventTypes, page)
		}
	}
	// A supplied cursor must not silently reinstate the first page's 7-day
	// default and hide older matching events.
	page, _ := callRenderEvents(t, cs, map[string]any{
		"serviceId": "web", "eventTypes": []string{events.TypeSuspenderAdded},
		"cursor": core.EncodeKeysetCursor(at.Add(-8*24*time.Hour), "older-page:"),
	})
	if len(page) != 1 || page[0].ID != ids.Derive(ids.Event, oldKey) {
		t.Fatalf("cursor-only page = %+v, want the 9-day-old event", page)
	}
}
