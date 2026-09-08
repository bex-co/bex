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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deletingDatabase returns a Database whose deletion Kubernetes has accepted —
// a DeletionTimestamp plus a finalizer that keeps the CR alive while the
// operator tears it down (fake client requires the finalizer alongside the
// timestamp).
func deletingDatabase(name string) *appv1alpha1.Database {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"app.bex.co/db-finalizer"},
			Labels:            map[string]string{core.LabelTenant: core.DefaultTenant},
		},
		Spec:   appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-256mb"},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
}

// TestDeletingPostgresIsAbsentFromEveryByIDRead is the w8/m35 read contract for
// the Postgres family: the moment deletion is accepted, every by-id surface
// returns the same core.ErrNotFound that List applies by omitting the row.
func TestDeletingPostgresIsAbsentFromEveryByIDRead(t *testing.T) {
	svc, _ := newService(deletingDatabase("gone"))
	ctx := context.Background()

	if _, err := svc.GetPostgres(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetPostgres on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.PostgresConnectionInfo(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("PostgresConnectionInfo on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.GetIPAllowList(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetIPAllowList on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ListUsers(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListUsers on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.RecoveryInfo(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("RecoveryInfo on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ListExports(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListExports on a deleting database = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ActionCapabilities(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ActionCapabilities on a deleting database = %v, want core.ErrNotFound", err)
	}

	list, err := svc.ListPostgres(ctx, "")
	if err != nil {
		t.Fatalf("ListPostgres: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListPostgres returned %d rows, want 0 — a deleting database must not appear", len(list))
	}
}

func TestActivePostgresReadsAreUnaffected(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "live",
			Namespace: "default",
			Labels:    map[string]string{core.LabelTenant: core.DefaultTenant},
		},
		Spec:   appv1alpha1.DatabaseSpec{Name: "live", Plan: "basic-256mb"},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	}
	svc, _ := newService(db)
	got, err := svc.GetPostgres(context.Background(), "live")
	if err != nil {
		t.Fatalf("GetPostgres on a live database: %v", err)
	}
	if got.ID != "live" {
		t.Fatalf("live database id = %q, want live", got.ID)
	}
	if got.Status == "deleting" {
		t.Fatalf("live database status = deleting")
	}
}

// TestDeletingPostgresNotFoundAcrossSurfaces locks the adapter wiring: REST,
// GraphQL, and MCP all surface the same core.ErrNotFound the service verbs
// return for a deleting Database (w8/m35/t004).
func TestDeletingPostgresNotFoundAcrossSurfaces(t *testing.T) {
	svc, _ := newService(deletingDatabase("gone"))
	ctx := context.Background()

	if code := serveREST(svc, http.MethodGet, "/v1/postgres/gone", "").Code; code != http.StatusNotFound {
		t.Errorf("REST GET deleting postgres = %d, want 404", code)
	}
	if code := serveREST(svc, http.MethodGet, "/v1/postgres/gone/connection-info", "").Code; code != http.StatusNotFound {
		t.Errorf("REST connection-info on deleting postgres = %d, want 404", code)
	}
	listRec := serveREST(svc, http.MethodGet, "/v1/postgres", "")
	if listRec.Code != http.StatusOK {
		t.Fatalf("REST list = %d", listRec.Code)
	}
	var list []any
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("REST list returned %d rows, want 0", len(list))
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	gql := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `{ database(id: "gone") { id } }`,
	})
	if len(gql.Errors) == 0 {
		t.Fatalf("GraphQL database(id) on deleting = %#v, want an error", gql.Data)
	}
	if !strings.Contains(strings.ToLower(gql.Errors[0].Message), "not found") {
		t.Errorf("GraphQL error = %q, want not found", gql.Errors[0].Message)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_postgres", Arguments: map[string]any{"postgresId": "gone"}})
	if err != nil {
		t.Fatalf("get_postgres transport: %v", err)
	}
	if !res.IsError {
		t.Fatalf("get_postgres on deleting = %#v, want IsError", res)
	}
}
