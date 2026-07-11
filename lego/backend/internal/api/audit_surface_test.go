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
	rows []store.AuditRow
}

func (f *fakeAuditStore) ListAuditEvents(_ context.Context, workspaceID string, _ store.AuditFilter) ([]store.AuditRow, error) {
	var out []store.AuditRow
	for _, r := range f.rows {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAuditStore) PurgeAuditEvents(context.Context, time.Time) (int64, error) { return 0, nil }

// TestAuditSurfaceParity is w4/m10's t007: REST's GET
// /v1/owners/{ownerId}/audit-logs and GraphQL's auditLogs(ownerId: …) both
// delegate to audit.Service.List, so the same store data must render
// identically on both — this asserts that rather than assuming it from the
// shared-verb architecture.
func TestAuditSurfaceParity(t *testing.T) {
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fakeStore := &fakeAuditStore{rows: []store.AuditRow{
		{ID: "aud-1", WorkspaceID: "tea-a", Caller: "user-x", CallerMethod: "session", Verb: "apps.Suspend", Resource: "workspace:tea-a", Outcome: string(core.AuditAllowed), At: at},
		{ID: "aud-2", WorkspaceID: "tea-a", Caller: "user-y", CallerMethod: "oauth2", Verb: "workspaces.Delete", Resource: "workspace:tea-other", Outcome: string(core.AuditDenied), At: at.Add(-time.Minute)},
	}}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	auditSvc := &audit.Service{Base: base, Store: fakeStore}
	h, _ := serverWith(t, base, Deps{Audit: auditSvc})

	restBody := do(t, h, "GET", "/v1/owners/tea-a/audit-logs", testToken, "")
	if restBody.Code != 200 {
		t.Fatalf("REST audit-logs: %d %s", restBody.Code, restBody.Body.String())
	}
	var restList []struct {
		AuditLog struct {
			ID          string `json:"id"`
			Timestamp   string `json:"timestamp"`
			Actor       string `json:"actor"`
			ActorMethod string `json:"actorMethod"`
			Action      string `json:"action"`
			Status      string `json:"status"`
			Resource    string `json:"resource"`
		} `json:"auditLog"`
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal(restBody.Body.Bytes(), &restList); err != nil {
		t.Fatalf("decode REST: %v", err)
	}
	if len(restList) != 2 {
		t.Fatalf("REST list = %d events, want 2", len(restList))
	}

	gqlData := gql(t, h, `{ auditLogs(ownerId: "tea-a") { id timestamp actor actorMethod action status resource } }`)
	gqlList, ok := gqlData["auditLogs"].([]any)
	if !ok || len(gqlList) != 2 {
		t.Fatalf("GraphQL auditLogs = %v, want 2 events", gqlData["auditLogs"])
	}

	for i, rest := range restList {
		g := gqlList[i].(map[string]any)
		if rest.AuditLog.ID != g["id"] || rest.AuditLog.Timestamp != g["timestamp"] ||
			rest.AuditLog.Actor != g["actor"] || rest.AuditLog.ActorMethod != g["actorMethod"] ||
			rest.AuditLog.Action != g["action"] || rest.AuditLog.Status != g["status"] ||
			rest.AuditLog.Resource != g["resource"] {
			t.Errorf("event %d diverges: REST=%+v GraphQL=%+v", i, rest.AuditLog, g)
		}
	}
	// Newest first on both surfaces.
	if restList[0].AuditLog.ID != "aud-1" || restList[1].AuditLog.ID != "aud-2" {
		t.Errorf("REST not newest-first: %+v", restList)
	}
	// A denial renders as "denied", not Render's binary success/error.
	if restList[1].AuditLog.Status != "denied" {
		t.Errorf("denied event Status = %q, want %q", restList[1].AuditLog.Status, "denied")
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
