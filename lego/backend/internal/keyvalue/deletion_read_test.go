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

package keyvalue

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

func deletingKeyValue(name string) *appv1alpha1.KeyValue {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"app.bex.co/kv-finalizer"},
			Labels:            map[string]string{core.LabelTenant: core.DefaultTenant},
		},
		Spec:   appv1alpha1.KeyValueSpec{Name: name, Plan: "starter"},
		Status: appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady},
	}
}

// TestDeletingKeyValueIsAbsentFromEveryByIDRead is the w8/m35 read contract for
// the Key Value family: the moment deletion is accepted, every by-id surface
// returns the same core.ErrNotFound that List applies by omitting the row.
func TestDeletingKeyValueIsAbsentFromEveryByIDRead(t *testing.T) {
	svc, _ := newService(deletingKeyValue("gone"))
	ctx := context.Background()

	if _, err := svc.GetKeyValue(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetKeyValue on a deleting store = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.KeyValueConnectionInfo(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("KeyValueConnectionInfo on a deleting store = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.GetIPAllowList(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetIPAllowList on a deleting store = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ActionCapabilities(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ActionCapabilities on a deleting store = %v, want core.ErrNotFound", err)
	}

	list, err := svc.ListKeyValues(ctx, "")
	if err != nil {
		t.Fatalf("ListKeyValues: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListKeyValues returned %d rows, want 0 — a deleting store must not appear", len(list))
	}
}

func TestActiveKeyValueReadsAreUnaffected(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "live",
			Namespace: "default",
			Labels:    map[string]string{core.LabelTenant: core.DefaultTenant},
		},
		Spec:   appv1alpha1.KeyValueSpec{Name: "live", Plan: "starter"},
		Status: appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady},
	}
	svc, _ := newService(kv)
	got, err := svc.GetKeyValue(context.Background(), "live")
	if err != nil {
		t.Fatalf("GetKeyValue on a live store: %v", err)
	}
	if got.ID != "live" {
		t.Fatalf("live store id = %q, want live", got.ID)
	}
	if got.Status == "deleting" {
		t.Fatalf("live store status = deleting")
	}
}

// TestDeletingKeyValueNotFoundAcrossSurfaces locks the adapter wiring: REST,
// GraphQL, and MCP all surface the same core.ErrNotFound the service verbs
// return for a deleting KeyValue (w8/m35/t004).
func TestDeletingKeyValueNotFoundAcrossSurfaces(t *testing.T) {
	svc, _ := newService(deletingKeyValue("gone"))
	ctx := context.Background()

	if code := serveREST(svc, http.MethodGet, "/v1/key-value/gone", "").Code; code != http.StatusNotFound {
		t.Errorf("REST GET deleting key-value = %d, want 404", code)
	}
	if code := serveREST(svc, http.MethodGet, "/v1/key-value/gone/connection-info", "").Code; code != http.StatusNotFound {
		t.Errorf("REST connection-info on deleting key-value = %d, want 404", code)
	}
	listRec := serveREST(svc, http.MethodGet, "/v1/key-value", "")
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
		RequestString: `{ keyValue(id: "gone") { id } }`,
	})
	if len(gql.Errors) == 0 {
		t.Fatalf("GraphQL keyValue(id) on deleting = %#v, want an error", gql.Data)
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
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_key_value", Arguments: map[string]any{"keyValueId": "gone"}})
	if err != nil {
		t.Fatalf("get_key_value transport: %v", err)
	}
	if !res.IsError {
		t.Fatalf("get_key_value on deleting = %#v, want IsError", res)
	}
}
