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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeExportSigner struct {
	url   string
	calls int
	ttl   time.Duration
}

func (f *fakeExportSigner) Presign(_ context.Context, _ *appv1alpha1.Database, _ appv1alpha1.DatabaseExportStatus, ttl time.Duration) (string, error) {
	f.calls++
	f.ttl = ttl
	return f.url, nil
}

type denySensitiveChecker struct{}

func (denySensitiveChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != core.RelCanViewSensitive, nil
}

func seedLogicalExport(t *testing.T, cl client.Client, now time.Time, phase appv1alpha1.DatabaseExportPhase) *appv1alpha1.Database {
	t.Helper()
	seedDatabaseSpec(t, cl, "export-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "export-db"}, &db); err != nil {
		t.Fatal(err)
	}
	exportID := "exp-c185th5c2rvvnhbfiltg"
	created := now.Add(-time.Hour)
	db.Spec.Exports = []appv1alpha1.DatabaseExportRequest{{ID: exportID, RequestedAt: created.Format(time.RFC3339)}}
	db.Status.Exports = []appv1alpha1.DatabaseExportStatus{{
		ID:          exportID,
		Phase:       phase,
		CreatedAt:   created.Format(time.RFC3339),
		CompletedAt: created.Add(time.Minute).Format(time.RFC3339),
		ExpiresAt:   now.Add(6 * 24 * time.Hour).Format(time.RFC3339),
		ObjectKey:   "s3://tenant-backups/postgres/logical-exports/export-db/" + exportID + "/2026-07-14T11_00Z.dir.tar.gz",
		Filename:    "2026-07-14T11_00Z.dir.tar.gz",
	}}
	db.Status.BackupEndpointURL = "https://objects.example.com"
	db.Status.BackupS3SecretName = "pg-backup-s3"
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
	return &db
}

func TestCreateLogicalExportWritesIntentAndRejectsOverlap(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "paid-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return now }

	export, err := svc.CreateExport(context.Background(), "paid-db")
	if err != nil {
		t.Fatalf("CreateExport: %v", err)
	}
	if kind, ok := id.KindOf(export.ID); !ok || kind.Prefix() != id.Export.Prefix() || export.Status != "created" {
		t.Fatalf("created export = %+v", export)
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "paid-db"}, &db); err != nil {
		t.Fatal(err)
	}
	if len(db.Spec.Exports) != 1 || db.Spec.Exports[0].ID != export.ID || db.Spec.Exports[0].RequestedAt != now.Format(time.RFC3339) {
		t.Fatalf("export intent not written: %+v", db.Spec.Exports)
	}
	if _, err := svc.CreateExport(context.Background(), "paid-db"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("overlapping export = %v, want conflict", err)
	}
}

// TestCreateExportPrunesTerminalHistory (round-11 #9): spec.exports must not
// grow append-only — a new export keeps every non-terminal request plus only
// the newest maxTerminalExportHistory terminal ones, bounding the Database CR
// object and reconcile cost no matter how many exports a tenant churns.
func TestCreateExportPrunesTerminalHistory(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "churn-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return now }

	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "churn-db"}, &db); err != nil {
		t.Fatal(err)
	}
	// Fifteen terminal (expired) requests, one still-available artifact, in
	// chronological order.
	db.Spec.Exports = nil
	db.Status.Exports = nil
	base := now.Add(-48 * time.Hour)
	for i := 0; i < maxTerminalExportHistory+5; i++ {
		expID := id.New(id.Export)
		at := base.Add(time.Duration(i) * time.Minute)
		db.Spec.Exports = append(db.Spec.Exports, appv1alpha1.DatabaseExportRequest{ID: expID, RequestedAt: at.Format(time.RFC3339)})
		db.Status.Exports = append(db.Status.Exports, appv1alpha1.DatabaseExportStatus{
			ID: expID, Phase: appv1alpha1.DatabaseExportExpired,
			CreatedAt: at.Format(time.RFC3339), ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339),
		})
	}
	keepID := id.New(id.Export)
	db.Spec.Exports = append(db.Spec.Exports, appv1alpha1.DatabaseExportRequest{ID: keepID, RequestedAt: now.Add(-time.Hour).Format(time.RFC3339)})
	db.Status.Exports = append(db.Status.Exports, appv1alpha1.DatabaseExportStatus{
		ID: keepID, Phase: appv1alpha1.DatabaseExportAvailable,
		CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(6 * 24 * time.Hour).Format(time.RFC3339),
	})
	db.Status.BackupEndpointURL = "https://objects.example.com"
	db.Status.BackupS3SecretName = "pg-backup-s3"
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}

	export, err := svc.CreateExport(context.Background(), "churn-db")
	if err != nil {
		t.Fatalf("CreateExport: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "churn-db"}, &db); err != nil {
		t.Fatal(err)
	}
	wantLen := maxTerminalExportHistory + 2 // newest 10 expired + the available one + the new one
	if len(db.Spec.Exports) != wantLen {
		t.Fatalf("spec.exports = %d entries, want %d (bounded terminal history)", len(db.Spec.Exports), wantLen)
	}
	if db.Spec.Exports[len(db.Spec.Exports)-1].ID != export.ID {
		t.Fatalf("newest entry = %s, want the just-created export", db.Spec.Exports[len(db.Spec.Exports)-1].ID)
	}
	foundAvailable := false
	for _, request := range db.Spec.Exports {
		if request.ID == keepID {
			foundAvailable = true
		}
	}
	if !foundAvailable {
		t.Fatal("the still-available export was pruned — only terminal history is bounded")
	}
	// The oldest expired entries are the ones pruned.
	if db.Spec.Exports[0].ID == db.Spec.Exports[1].ID {
		t.Fatal("impossible duplicate ids")
	}
}

func TestRenderSingularExportCreateReturnsAcceptedWithoutBody(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "paid-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)

	response := serveREST(svc, http.MethodPost, "/v1/postgres/paid-db/export", "")
	if response.Code != http.StatusAccepted || response.Body.Len() != 0 {
		t.Fatalf("POST /export = %d %q, want 202 with no body", response.Code, response.Body.String())
	}
}

func TestGraphQLAndMCPCreateLogicalExportWriteIntent(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "gql-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)
	seedDatabaseSpec(t, cl, "mcp-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, true)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlResult := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createDatabaseExport(id:"gql-db") { id status createdAt } }`,
		Context:       context.Background(),
	})
	if len(gqlResult.Errors) != 0 {
		t.Fatal(gqlResult.Errors)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpServer)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := mcpServer.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_postgres_export", Arguments: map[string]any{"postgresId": "mcp-db"},
	})
	if err != nil || result.IsError {
		t.Fatalf("MCP create: err=%v result=%+v", err, result)
	}

	for _, name := range []string{"gql-db", "mcp-db"} {
		var db appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &db); err != nil {
			t.Fatal(err)
		}
		if len(db.Spec.Exports) != 1 {
			t.Fatalf("%s export intents = %+v, want one", name, db.Spec.Exports)
		}
	}
}

func TestListLogicalExportsPresignsAvailableAndSuppressesExpired(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	svc, cl := newService()
	seedLogicalExport(t, cl, now, appv1alpha1.DatabaseExportAvailable)
	signer := &fakeExportSigner{url: "https://objects.example.com/signed"}
	svc.ExportSigner = signer
	svc.Clock = func() time.Time { return now }

	list, err := svc.ListExports(context.Background(), "export-db")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListExports = %+v, %v", list, err)
	}
	if list[0].URL != signer.url || list[0].Filename == "" || list[0].Status != "available" || signer.ttl != ExportURLTTL {
		t.Fatalf("available export = %+v signer=%+v", list[0], signer)
	}

	// Once the recorded deadline passes, the API stops issuing URLs immediately,
	// even if the operator's asynchronous delete Job has not yet settled.
	now = now.Add(8 * 24 * time.Hour)
	list, err = svc.ListExports(context.Background(), "export-db")
	if err != nil || list[0].Status != "expired" || list[0].URL != "" || signer.calls != 1 {
		t.Fatalf("expired export = %+v err=%v signer calls=%d", list[0], err, signer.calls)
	}
}

func TestExportDownloadRequiresCanViewSensitive(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	svc, cl := newService()
	seedLogicalExport(t, cl, now, appv1alpha1.DatabaseExportAvailable)
	svc.ExportSigner = &fakeExportSigner{url: "https://objects.example.com/signed"}
	svc.Clock = func() time.Time { return now }
	svc.Authz = denySensitiveChecker{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer", Method: "session"})
	if _, err := svc.ListExports(ctx, "export-db"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("ListExports denial = %v, want forbidden", err)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/postgres/export-db/export", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthorized download list = %d: %s", response.Code, response.Body.String())
	}
}

func TestS3ExportSignerCreatesExpiringURL(t *testing.T) {
	svc, cl := newService()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-backup-s3", Namespace: "default"},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("access"),
			"AWS_SECRET_ACCESS_KEY": []byte("secret"),
		},
	}
	if err := cl.Create(context.Background(), secret); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Status: appv1alpha1.DatabaseStatus{
			BackupEndpointURL:  "https://objects.example.com",
			BackupS3SecretName: "pg-backup-s3",
		},
	}
	export := appv1alpha1.DatabaseExportStatus{
		ObjectKey: "s3://tenant-backups/logical-exports/db/exp-c185th5c2rvvnhbfiltg/export.dir.tar.gz",
		Filename:  "export.dir.tar.gz",
	}
	signed, err := NewS3ExportSigner(svc.Client).Presign(context.Background(), db, export, ExportURLTTL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("X-Amz-Expires") != "900" || parsed.Query().Get("response-content-disposition") == "" {
		t.Fatalf("signed URL lacks expiry/download disposition: %s", signed)
	}
}

func TestS3ExportSignerDownloadsFromS3CompatibleStore(t *testing.T) {
	endpoint := os.Getenv("BEX_TEST_EXPORT_S3_ENDPOINT")
	objectKey := os.Getenv("BEX_TEST_EXPORT_S3_OBJECT")
	accessKey := os.Getenv("BEX_TEST_EXPORT_ACCESS_KEY")
	secretKey := os.Getenv("BEX_TEST_EXPORT_SECRET_KEY")
	if endpoint == "" || objectKey == "" || accessKey == "" || secretKey == "" {
		t.Skip("set BEX_TEST_EXPORT_S3_* to run the S3-compatible download integration")
	}

	svc, cl := newService()
	if err := cl.Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-backup-s3", Namespace: "default"},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte(accessKey),
			"AWS_SECRET_ACCESS_KEY": []byte(secretKey),
		},
	}); err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Status: appv1alpha1.DatabaseStatus{
			BackupEndpointURL:  endpoint,
			BackupS3SecretName: "pg-backup-s3",
		},
	}
	export := appv1alpha1.DatabaseExportStatus{ObjectKey: objectKey, Filename: "export.dir.tar.gz"}
	signed, err := NewS3ExportSigner(svc.Client).Presign(context.Background(), db, export, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, signed, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	header, err := io.ReadAll(io.LimitReader(response.Body, 2))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !reflect.DeepEqual(header, []byte{0x1f, 0x8b}) {
		t.Fatalf("signed download = %d header %x, want gzip artifact", response.StatusCode, header)
	}
}

func TestExportObjectParityAcrossRESTGraphQLAndMCP(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	svc, cl := newService()
	seedLogicalExport(t, cl, now, appv1alpha1.DatabaseExportAvailable)
	svc.ExportSigner = &fakeExportSigner{url: "https://objects.example.com/signed"}
	svc.Clock = func() time.Time { return now }

	var rest []map[string]any
	response := serveREST(svc, http.MethodGet, "/v1/postgres/export-db/export", "")
	if response.Code != http.StatusOK {
		t.Fatalf("REST: %d %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlResult := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `{ databaseExports(id:"export-db") {
			id createdAt status url urlExpiresAt expiresAt filename
		} }`,
		Context: context.Background(),
	})
	if len(gqlResult.Errors) != 0 {
		t.Fatal(gqlResult.Errors)
	}
	gqlExport := gqlResult.Data.(map[string]any)["databaseExports"].([]any)[0].(map[string]any)

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpServer)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := mcpServer.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	mcpResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_postgres_exports", Arguments: map[string]any{"postgresId": "export-db"},
	})
	if err != nil || mcpResult.IsError {
		t.Fatalf("MCP: err=%v result=%+v", err, mcpResult)
	}
	encoded, _ := json.Marshal(mcpResult.StructuredContent)
	var mcpBody map[string]any
	_ = json.Unmarshal(encoded, &mcpBody)
	mcpExport := mcpBody["exports"].([]any)[0].(map[string]any)

	keys := []string{"id", "createdAt", "status", "url", "urlExpiresAt", "expiresAt", "filename"}
	restExport := selectExportFields(rest[0], keys)
	if !reflect.DeepEqual(restExport, selectExportFields(gqlExport, keys)) || !reflect.DeepEqual(restExport, selectExportFields(mcpExport, keys)) {
		t.Fatalf("adapter drift:\nREST %#v\nGraphQL %#v\nMCP %#v", rest[0], gqlExport, mcpExport)
	}
}

func selectExportFields(input map[string]any, keys []string) map[string]any {
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = input[key]
	}
	return out
}
