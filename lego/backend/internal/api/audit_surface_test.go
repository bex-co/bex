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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeAuditStore is a minimal audit.AuditStore returning canned rows —
// t007's parity check: REST and GraphQL must render the same store data
// identically, so the fixture is shared between both requests in one test.
type fakeAuditStore struct {
	rows      []store.AuditRow
	gotFilter store.AuditFilter
}

func (f *fakeAuditStore) ListAuditEvents(_ context.Context, workspaceID string, filter store.AuditFilter) ([]store.AuditRow, error) {
	f.gotFilter = filter
	var out []store.AuditRow
	for _, r := range f.rows {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAuditStore) PurgeAuditEvents(context.Context, time.Time) (int64, error) { return 0, nil }
func (f *fakeAuditStore) PurgeSSHSessions(context.Context, time.Time) (int64, error) {
	return 0, nil
}

// TestAuditSurfaceParity is w4/m10's t007: REST's GET
// /v1/owners/{ownerId}/audit-logs and GraphQL's auditLogs(ownerId: …) both
// delegate to audit.Service.List, so the same store data must render
// identically on both — this asserts that rather than assuming it from the
// shared-verb architecture.
func TestAuditSurfaceParity(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	maintenanceDisabled := false
	fakeStore := &fakeAuditStore{rows: []store.AuditRow{
		{ID: "aud-1", WorkspaceID: "tea-a", Caller: "user-x", CallerMethod: "session", Verb: "apps.SetMaintenanceMode", Resource: "workspace:tea-a", Outcome: string(core.AuditAllowed), At: at, MaintenanceModeTo: &maintenanceDisabled},
		{ID: "aud-2", WorkspaceID: "tea-a", Caller: "user-y", CallerMethod: "oauth2", Verb: "workspaces.Delete", Resource: "workspace:tea-other", Outcome: string(core.AuditDenied), At: at.Add(-time.Minute)},
	}}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	auditSvc := &audit.Service{Base: base, Store: fakeStore}
	h, _ := serverWith(t, base, Deps{Audit: auditSvc})

	restBody := do(t, h, "GET", "/v1/owners/tea-a/audit-logs", testToken, "")
	if restBody.Code != 200 {
		t.Fatalf("REST audit-logs: %d %s", restBody.Code, restBody.Body.String())
	}
	// REST speaks Render's captured auditLog schema (w4/m26): event (not
	// action), actor an object, metadata a required string map, status the
	// closed success|error enum. GraphQL keeps bex's own dialect (flat actor,
	// action, denied) — the parity assertion below is the documented mapping,
	// not field-for-field equality.
	var restList []struct {
		AuditLog struct {
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			Event     string `json:"event"`
			Status    string `json:"status"`
			Actor     struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"actor"`
			Metadata map[string]string `json:"metadata"`
			Resource string            `json:"resource"`
		} `json:"auditLog"`
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal(restBody.Body.Bytes(), &restList); err != nil {
		t.Fatalf("decode REST: %v", err)
	}
	if len(restList) != 2 {
		t.Fatalf("REST list = %d events, want 2", len(restList))
	}

	gqlData := gql(t, h, `{ auditLogs(ownerId: "tea-a") { id timestamp actor actorMethod action status resource metadata { to } } }`)
	gqlList, ok := gqlData["auditLogs"].([]any)
	if !ok || len(gqlList) != 2 {
		t.Fatalf("GraphQL auditLogs = %v, want 2 events", gqlData["auditLogs"])
	}

	wantActorType := map[string]string{"session": "user", "oauth2": "rest_api"}
	wantRESTStatus := map[string]string{"success": "success", "denied": "error"}
	for i, rest := range restList {
		g := gqlList[i].(map[string]any)
		if rest.AuditLog.ID != g["id"] || rest.AuditLog.Timestamp != g["timestamp"] ||
			rest.AuditLog.Event != g["action"] ||
			rest.AuditLog.Resource != g["resource"] {
			t.Errorf("event %d diverges: REST=%+v GraphQL=%+v", i, rest.AuditLog, g)
		}
		if rest.AuditLog.Actor.ID != g["actor"] || rest.AuditLog.Actor.Type != wantActorType[g["actorMethod"].(string)] {
			t.Errorf("event %d actor mapping: REST=%+v GraphQL actor=%v method=%v", i, rest.AuditLog.Actor, g["actor"], g["actorMethod"])
		}
		if rest.AuditLog.Status != wantRESTStatus[g["status"].(string)] {
			t.Errorf("event %d status mapping: REST=%q GraphQL=%q", i, rest.AuditLog.Status, g["status"])
		}
	}
	if restList[0].AuditLog.Metadata["to"] != "false" ||
		gqlList[0].(map[string]any)["metadata"].(map[string]any)["to"] != false {
		t.Fatalf("maintenance metadata.to=false diverges: REST=%+v GraphQL=%+v", restList[0].AuditLog.Metadata, gqlList[0])
	}
	if restList[0].AuditLog.Event != "MaintenanceModeEnabledEvent" {
		t.Fatalf("maintenance audit event = %q, want Render event name", restList[0].AuditLog.Event)
	}
	if restList[1].AuditLog.Metadata == nil {
		t.Fatalf("REST metadata is required by Render's schema — must be {} not null: %+v", restList[1].AuditLog)
	}
	// Newest first on both surfaces.
	if restList[0].AuditLog.ID != "aud-1" || restList[1].AuditLog.ID != "aud-2" {
		t.Errorf("REST not newest-first: %+v", restList)
	}
	// A denial is Render's "error" on REST, with the denial preserved in
	// metadata; GraphQL keeps bex's richer "denied" (asserted in the loop).
	if restList[1].AuditLog.Status != "error" || restList[1].AuditLog.Metadata["outcome"] != "denied" {
		t.Errorf("denied event = status %q metadata %v, want error + outcome:denied", restList[1].AuditLog.Status, restList[1].AuditLog.Metadata)
	}
	if restList[1].AuditLog.Event != "DeleteWorkspaceEvent" {
		t.Errorf("workspaces.Delete = %q, want DeleteWorkspaceEvent", restList[1].AuditLog.Event)
	}
	// Cross-workspace: tea-a's data must never answer for another owner.
	otherWorkspace := do(t, h, "GET", "/v1/owners/tea-other/audit-logs", testToken, "")
	var otherList []any
	if err := json.Unmarshal(otherWorkspace.Body.Bytes(), &otherList); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(otherList) != 0 {
		t.Errorf("tea-other's list = %d events, want 0 (tea-a's rows must not leak)", len(otherList))
	}
}

// TestAuditDirectionSurface is w4/013: Render's direction param is honored,
// not silently ignored — forward reaches the store as oldest-first on both
// surfaces, and an unknown value is a named 400 on REST / a named GraphQL
// error, per w3/m8's "nothing accepted is ignored" principle.
func TestAuditDirectionSurface(t *testing.T) {
	fakeStore := &fakeAuditStore{}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: fakeStore}})

	// REST: forward => the store's oldest-first flag.
	if code := do(t, h, "GET", "/v1/owners/tea-a/audit-logs?direction=forward", testToken, "").Code; code != 200 {
		t.Fatalf("REST direction=forward: %d", code)
	}
	if !fakeStore.gotFilter.OldestFirst {
		t.Errorf("REST direction=forward did not reach the store as OldestFirst")
	}

	// REST: backward (and unset — covered by every other test here) stays newest-first.
	if code := do(t, h, "GET", "/v1/owners/tea-a/audit-logs?direction=backward", testToken, "").Code; code != 200 {
		t.Fatalf("REST direction=backward: %d", code)
	}
	if fakeStore.gotFilter.OldestFirst {
		t.Errorf("REST direction=backward must stay newest-first")
	}

	// REST: unknown value is a named 400.
	bad := do(t, h, "GET", "/v1/owners/tea-a/audit-logs?direction=sideways", testToken, "")
	if bad.Code != 400 {
		t.Fatalf("REST direction=sideways: %d, want 400 — an accepted param must never be ignored", bad.Code)
	}
	if !strings.Contains(bad.Body.String(), "sideways") {
		t.Errorf("400 body %q should name the offending value", bad.Body.String())
	}

	// GraphQL: same verb, same behavior on both sides of the enum.
	fakeStore.gotFilter = store.AuditFilter{}
	gql(t, h, `{ auditLogs(ownerId: "tea-a", direction: "forward") { id } }`)
	if !fakeStore.gotFilter.OldestFirst {
		t.Errorf("GraphQL direction=forward did not reach the store as OldestFirst")
	}
	body, _ := json.Marshal(map[string]string{"query": `{ auditLogs(ownerId: "tea-a", direction: "sideways") { id } }`})
	w := do(t, h, "POST", "/graphql", testToken, string(body))
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Errors) == 0 || !strings.Contains(out.Errors[0].Message, "sideways") {
		t.Errorf("GraphQL direction=sideways: errors = %+v, want a named error", out.Errors)
	}
}

// TestAuditReadSurfaceAuthMatrix is t006's read-surface authorization matrix:
// admin 200, non-admin 403, store-less 503 — the three states the DoD names.
func TestAuditReadSurfaceAuthMatrix(t *testing.T) {
	fakeStore := &fakeAuditStore{}

	t.Run("admin sees 200", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: fakeStore}})
		if code := do(t, h, "GET", "/v1/owners/tea-a/audit-logs", testToken, "").Code; code != 200 {
			t.Errorf("admin => 200, got %d", code)
		}
	})

	t.Run("non-admin sees 403", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: false}}
		h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: fakeStore}})
		if code := do(t, h, "GET", "/v1/owners/tea-a/audit-logs", testToken, "").Code; code != 403 {
			t.Errorf("non-admin => 403, got %d", code)
		}
	})

	t.Run("store-less sees 503", func(t *testing.T) {
		base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
		h, _ := serverWith(t, base, Deps{Audit: &audit.Service{Base: base, Store: nil}})
		if code := do(t, h, "GET", "/v1/owners/tea-a/audit-logs", testToken, "").Code; code != 503 {
			t.Errorf("store-less => 503, got %d", code)
		}
	})
}
