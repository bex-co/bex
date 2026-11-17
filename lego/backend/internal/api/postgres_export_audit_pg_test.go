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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/105 on real Postgres: a list of a database's logical exports that returns a
// download URL leaves one row in audit_events, which GET /v1/owners/{id}/audit-logs
// reports as Render's DownloadDatabaseBackupEvent, naming the database, and REST,
// GraphQL and MCP write the same row; a list that returns no URL writes none; a
// refused caller leaves its denial and no download.

const exportAuditSignature = "X-Amz-Signature=0123456789abcdef"

type exportURLSigner struct{ calls int }

func (s *exportURLSigner) Presign(context.Context, *appv1alpha1.Database, appv1alpha1.DatabaseExportStatus, time.Duration) (string, error) {
	s.calls++
	return "https://objects.example.com/logical-exports/dump.dir.tar.gz?" + exportAuditSignature, nil
}

// denySensitive allows every relation but can_view_sensitive while deny is set, so
// the same caller is first the owner who downloads and then the member who is
// refused, and can read the audit log in between.
type denySensitive struct{ deny bool }

func (d *denySensitive) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return !(d.deny && relation == core.RelCanViewSensitive), nil
}

func exportedDatabase(name, tenant string, withExport bool) *appv1alpha1.Database {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.LabelTenant: tenant, core.LabelWorkspace: tenant}},
		Spec:       appv1alpha1.DatabaseSpec{Name: "orders-" + name[len(name)-4:], Plan: "basic-1gb"},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, BackupsEnabled: true},
	}
	if withExport {
		created := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		db.Spec.Exports = []appv1alpha1.DatabaseExportRequest{{ID: ids.New(ids.Export), RequestedAt: created}}
		db.Status.Exports = []appv1alpha1.DatabaseExportStatus{{
			ID: db.Spec.Exports[0].ID, Phase: appv1alpha1.DatabaseExportAvailable, CreatedAt: created,
			ExpiresAt: time.Now().Add(6 * 24 * time.Hour).UTC().Format(time.RFC3339), Filename: "dump.dir.tar.gz",
		}}
	}
	return db
}

func TestAPostgresExportDownloadIsOneAuditRowOnEverySurfacePG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "exportaudit-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, owner.ID)
		_ = st.DeleteTenant(context.Background(), owner.ID)
	})

	withExport := exportedDatabase("dpg-"+uniqueRunSuffix(), owner.ID, true)
	withoutExport := exportedDatabase("dpg-"+uniqueRunSuffix(), owner.ID, false)
	signer := &exportURLSigner{}
	checker := &denySensitive{}
	base := serverBase(t, st)
	base.Client = fakeClient(withExport, withoutExport)
	base.Authz = checker
	base.Audit = st
	srv := NewServer(base, Deps{WorkspaceStore: st, Store: st, Audit: &audit.Service{Base: base, Store: st}})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.Postgres.ExportSigner = signer // the production signer reads the object store's credentials from the cluster
	handler := buildHandler(t, srv)
	// The audit log is read through the handler that does not validate the event
	// against Render's pinned enum: a denial reports bex's own verb, as the object
	// routes' rows do (docs/render-artifacts/audit-logs-api.md).
	plain, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	identity := core.Identity{Subject: "client-1", Method: core.MethodOAuth2}

	// auditRows is the workspace's audit log as REST reports it.
	type row struct {
		Event, Status string
		Metadata      map[string]string
	}
	auditRows := func() (rows []row, raw string) {
		t.Helper()
		was := checker.deny
		checker.deny = false
		defer func() { checker.deny = was }()
		rec := do(t, plain, http.MethodGet, "/v1/owners/"+owner.ID+"/audit-logs?limit=1000", testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("audit log = %d %s", rec.Code, rec.Body)
		}
		var body []struct {
			AuditLog struct {
				Event    string            `json:"event"`
				Status   string            `json:"status"`
				Metadata map[string]string `json:"metadata"`
			} `json:"auditLog"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, b := range body {
			rows = append(rows, row{b.AuditLog.Event, b.AuditLog.Status, b.AuditLog.Metadata})
		}
		return rows, rec.Body.String()
	}
	count := func(event string) (n int) {
		rows, _ := auditRows()
		for _, r := range rows {
			if r.Event == event {
				n++
			}
		}
		return n
	}

	listRESTStatus := func(name string) int {
		return do(t, handler, http.MethodGet, "/v1/postgres/"+name+"/export", testToken, "").Code
	}
	gqlList := func(name, fields string) []any {
		t.Helper()
		out := gql(t, handler, `{ databaseExports(id: "`+name+`") { `+fields+` } }`)
		list, _ := out["databaseExports"].([]any)
		return list
	}
	mcpList := func(name string) bool {
		return mcpCall(t, srv, identity, "list_postgres_exports", map[string]any{"postgresId": name}).IsError
	}

	const download = "DownloadDatabaseBackupEvent"

	// A list that returned a URL records one row, on each surface in turn.
	if got := listRESTStatus(withExport.Name); got != http.StatusOK {
		t.Fatalf("REST list = %d", got)
	}
	if got := count(download); got != 1 {
		t.Fatalf("after one REST list the log holds %d %s rows, want 1", got, download)
	}
	if list := gqlList(withExport.Name, "id status url"); len(list) != 1 || list[0].(map[string]any)["url"] == nil {
		t.Fatalf("GraphQL list = %v", list)
	}
	if got := count(download); got != 2 {
		t.Fatalf("after a GraphQL list that selects url the log holds %d rows, want 2", got)
	}
	if mcpList(withExport.Name) {
		t.Fatal("MCP list failed")
	}
	if got := count(download); got != 3 {
		t.Fatalf("after an MCP list the log holds %d rows, want 3", got)
	}
	if signer.calls != 3 {
		t.Fatalf("the three lists minted %d URLs, want 3", signer.calls)
	}

	rows, raw := auditRows()
	var downloads []row
	for _, r := range rows {
		if r.Event == download {
			downloads = append(downloads, r)
		}
	}
	for i, r := range downloads {
		if r.Status != "success" || r.Metadata["database"] != withExport.Name || r.Metadata["targetName"] != withExport.Spec.Name ||
			r.Metadata["outcome"] != "" || r.Metadata["relation"] != "" {
			t.Errorf("download row %d = %+v, want a success naming %s", i, r, withExport.Name)
		}
		if i > 0 && !equalMetadata(r.Metadata, downloads[0].Metadata) {
			t.Errorf("surfaces disagree: row %d has %v, row 0 has %v", i, r.Metadata, downloads[0].Metadata)
		}
	}
	// The URL, its signature and the object key never reach the log.
	for _, secret := range []string{"objects.example.com", exportAuditSignature, "X-Amz", "logical-exports", "dump.dir.tar.gz"} {
		if strings.Contains(raw, secret) {
			t.Errorf("the audit log carries %q", secret)
		}
	}

	// The dashboard's status poll selects no url: it mints none and records none.
	if list := gqlList(withExport.Name, "id status createdAt expiresAt filename"); len(list) != 1 {
		t.Fatalf("GraphQL status poll = %v", list)
	}
	if signer.calls != 3 || count(download) != 3 {
		t.Fatalf("a status poll minted URLs (%d) or recorded a download (%d)", signer.calls, count(download))
	}

	// A database with no export to download returns no URL on any surface and
	// records nothing.
	if got := listRESTStatus(withoutExport.Name); got != http.StatusOK {
		t.Fatalf("REST list of a database with no export = %d", got)
	}
	if list := gqlList(withoutExport.Name, "id url"); len(list) != 0 {
		t.Fatalf("GraphQL list of a database with no export = %v", list)
	}
	if mcpList(withoutExport.Name) {
		t.Fatal("MCP list of a database with no export failed")
	}
	if signer.calls != 3 || count(download) != 3 {
		t.Fatalf("a list with no export minted URLs (%d) or recorded a download (%d)", signer.calls, count(download))
	}

	// A caller refused can_view_sensitive is refused on every surface, mints no URL
	// and leaves its denial under the verb the list is known by.
	checker.deny = true
	if got := listRESTStatus(withExport.Name); got != http.StatusForbidden {
		t.Fatalf("a refused REST list = %d, want 403", got)
	}
	body, _ := json.Marshal(map[string]string{"query": `{ databaseExports(id: "` + withExport.Name + `") { id url } }`})
	if rec := do(t, handler, http.MethodPost, "/graphql", testToken, string(body)); !strings.Contains(rec.Body.String(), "errors") {
		t.Fatalf("a refused GraphQL list = %s", rec.Body)
	}
	if !mcpList(withExport.Name) {
		t.Fatal("a refused MCP list answered")
	}
	checker.deny = false

	if signer.calls != 3 || count(download) != 3 {
		t.Fatalf("a refused list minted URLs (%d) or recorded a download (%d)", signer.calls, count(download))
	}
	rows, _ = auditRows()
	denials := 0
	for _, r := range rows {
		if r.Event != "postgres.ListExports" {
			continue
		}
		denials++
		if r.Status != "error" || r.Metadata["outcome"] != "denied" || r.Metadata["relation"] != core.RelCanViewSensitive ||
			r.Metadata["database"] != withExport.Name {
			t.Errorf("denial = %+v", r)
		}
	}
	if denials != 3 {
		t.Fatalf("the log holds %d denials of the list, want one per surface: %+v", denials, rows)
	}
}

func equalMetadata(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
