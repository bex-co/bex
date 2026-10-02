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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestPersistenceTransitionStatusAcrossAdapters(t *testing.T) {
	for _, tc := range []struct {
		name     string
		phase    appv1alpha1.KeyValuePhase
		observed int64
		ready    metav1.ConditionStatus
		want     string
	}{
		{"accepted before reconcile", appv1alpha1.KVPhaseReady, 1, metav1.ConditionTrue, "config_restart"},
		{"conversion pending", appv1alpha1.KVPhaseProvisioning, 2, metav1.ConditionFalse, "config_restart"},
		{"conversion failed", appv1alpha1.KVPhaseFailed, 2, metav1.ConditionFalse, "unavailable"},
		{"current revision ready", appv1alpha1.KVPhaseReady, 2, metav1.ConditionTrue, "available"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resourceID := id.New(id.KeyValue)
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: resourceID, Namespace: "default", Generation: 2},
				Spec:       appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: "journal-snapshot"},
				Status: appv1alpha1.KeyValueStatus{
					Phase: tc.phase, CredentialRevision: "served-before",
					Conditions: []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: tc.ready, ObservedGeneration: tc.observed}},
				},
			}
			svc, _ := newService(kv)
			response := serveREST(svc, "GET", "/v1/key-value/"+resourceID, "")
			if response.Code != 200 {
				t.Fatalf("REST status %d: %s", response.Code, response.Body)
			}
			var rest renderKeyValue
			if err := json.Unmarshal(response.Body.Bytes(), &rest); err != nil {
				t.Fatal(err)
			}
			if rest.Status != tc.want || rest.Options.PersistenceMode != "journal_snapshot" {
				t.Fatalf("REST status/mode = %s/%s", rest.Status, rest.Options.PersistenceMode)
			}
			schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
			if err != nil {
				t.Fatal(err)
			}
			ctx := ctxAs("user-a")
			result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: fmt.Sprintf(`{ keyValue(id:%q) { status persistenceMode } }`, resourceID)})
			if len(result.Errors) != 0 {
				t.Fatal(result.Errors)
			}
			gql := result.Data.(map[string]any)["keyValue"].(map[string]any)
			if gql["status"] != tc.want || gql["persistenceMode"] != "journal_snapshot" {
				t.Fatalf("GraphQL = %v", gql)
			}

			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
			svc.RegisterMCP(server)
			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			session, err := server.Connect(ctx, serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			connection, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			tool, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: "get_key_value", Arguments: map[string]any{"keyValueId": resourceID}})
			if err != nil {
				t.Fatal(err)
			}
			if tool.IsError {
				t.Fatalf("MCP error: %v", tool.Content)
			}
			data, err := json.Marshal(tool.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var view KeyValueView
			if err := json.Unmarshal(data, &view); err != nil {
				t.Fatal(err)
			}
			if view.Status != tc.want || view.PersistenceMode != "journal_snapshot" {
				t.Fatalf("MCP status/mode = %s/%s", view.Status, view.PersistenceMode)
			}
		})
	}
}
