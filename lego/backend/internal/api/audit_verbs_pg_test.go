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
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestObjectRoutesWriteAuditRowsTheAuditLogReadsPG is w21/087 on real Postgres:
// a Render put, get and delete each leave a row in audit_events, and the
// workspace's audit log (GET /v1/owners/{ownerId}/audit-logs) reads them under
// objectstorage verbs, never under a closure's name.
func TestObjectRoutesWriteAuditRowsTheAuditLogReadsPG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "objaudit-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, owner.ID)
		_ = st.DeleteTenant(context.Background(), owner.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM object_buckets WHERE workspace_id = $1`, owner.ID)
	})
	_, stores := fakeObjectStores(t)
	base := serverBase(t, st)
	base.Authz = &fakeChecker{allow: true}
	base.Audit = st
	handler, srv := serverWith(t, base, Deps{
		WorkspaceStore: st, Store: st, ObjectStorage: objectstorage.NewService(st, stores), BaseDomain: objectsTestBaseDomain,
		Audit: &audit.Service{Base: base, Store: st},
	})

	// Mint a put, upload through the presigned URL, then mint a get (which records
	// one row, w21/094) and delete.
	put := do(t, handler, http.MethodPut, "/v1/objects/"+owner.ID+"/oregon/b.txt", testToken, `{"sizeBytes":5}`)
	if put.Code != http.StatusOK {
		t.Fatalf("Render put = %d %s", put.Code, put.Body)
	}
	var minted struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &minted); err != nil {
		t.Fatal(err)
	}
	up, _ := http.NewRequest(http.MethodPut, minted.URL, strings.NewReader("hello"))
	if resp, err := http.DefaultClient.Do(up); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("upload to the presigned URL: %v %v", resp, err)
	}
	if rec := do(t, handler, http.MethodGet, "/v1/objects/"+owner.ID+"/oregon/b.txt", testToken, ""); rec.Code != http.StatusOK {
		t.Fatalf("Render get = %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, handler, http.MethodDelete, "/v1/objects/"+owner.ID+"/oregon/b.txt", testToken, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("Render delete = %d %s", rec.Code, rec.Body)
	}

	// Read through the unvalidated handler: the pinned response schema's event
	// enum has no object or bucket event, and bex reports its own verb for those
	// (docs/render-artifacts/audit-logs-api.md).
	plain, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, plain, http.MethodGet, "/v1/owners/"+owner.ID+"/audit-logs?limit=100", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("audit log = %d %s", rec.Code, rec.Body)
	}
	var rows []struct {
		AuditLog struct {
			Event    string            `json:"event"`
			Status   string            `json:"status"`
			Metadata map[string]string `json:"metadata"`
		} `json:"auditLog"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	named := map[string][]map[string]string{}
	for _, r := range rows {
		if r.AuditLog.Status != "success" {
			t.Errorf("row %q has status %q, want success", r.AuditLog.Event, r.AuditLog.Status)
		}
		got[r.AuditLog.Event]++
		named[r.AuditLog.Event] = append(named[r.AuditLog.Event], r.AuditLog.Metadata)
	}
	want := map[string]int{objectstorage.AuditVerbPutObject: 1, objectstorage.AuditVerbGetObject: 1, objectstorage.AuditVerbDeleteObject: 1}
	for verb, n := range want {
		if got[verb] != n {
			t.Errorf("audit log holds %d %s rows, want %d: %v", got[verb], verb, n, got)
		}
	}
	// Each row names its bucket and its key in REST's metadata (w21/094): the
	// target survives the store and reads back as two keys.
	object := map[string]string{"bucket": "render-oregon", "key": "b.txt"}
	wantMetadata := map[string][]map[string]string{
		objectstorage.AuditVerbGetObject:    {object},
		objectstorage.AuditVerbPutObject:    {object},
		objectstorage.AuditVerbDeleteObject: {object},
	}
	for verb, wantRows := range wantMetadata {
		for _, wantRow := range wantRows {
			if !slices.ContainsFunc(named[verb], func(md map[string]string) bool { return maps.Equal(md, wantRow) }) {
				t.Errorf("no %s row has metadata %v: %v", verb, wantRow, named[verb])
			}
		}
	}
	for event := range got {
		if p := verbProblem(event); p != "" {
			t.Errorf("audit log holds the event %q: %s", event, p)
		}
	}
}
