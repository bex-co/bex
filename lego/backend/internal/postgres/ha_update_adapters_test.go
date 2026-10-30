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
	"fmt"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestHAUpdateGraphQLAndMCP is w2/m171's "upgrade, then enable HA" journey on
// the two adapters REST already had: GraphQL updateDatabaseHighAvailability and
// MCP update_postgres.enableHighAvailability enable HA on a ≥1-CPU plan (the
// dashboard switch rides the GraphQL mutation), refuse it below 1 CPU with
// REST's POSTGRES_HA_PLAN_UNSUPPORTED and write nothing, and turn it off again.
// GraphQL's highAvailabilityRequested reports the spec intent the observed
// highAvailabilityEnabled lags behind.
func TestHAUpdateGraphQLAndMCP(t *testing.T) {
	seed := func(name, plan string) *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.DatabaseSpec{Name: name, Plan: plan, StorageGB: 10},
		}
	}
	svc, cl := newService(seed("gql-ha", "1c-2g"), seed("mcp-ha", "pro-4gb"), seed("small-db", "basic-1gb"))
	spec := func(name string) appv1alpha1.DatabaseSpec {
		t.Helper()
		var db appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &db); err != nil {
			t.Fatal(err)
		}
		return db.Spec
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gql := func(id string, enabled bool) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(
			`mutation { updateDatabaseHighAvailability(id: %q, enabled: %t) { id highAvailabilityRequested } }`, id, enabled)})
	}
	res := gql("gql-ha", true)
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL enable: %v", res.Errors)
	}
	if got := res.Data.(map[string]any)["updateDatabaseHighAvailability"].(map[string]any); got["highAvailabilityRequested"] != true {
		t.Fatalf("GraphQL enable answered %+v, want highAvailabilityRequested", got)
	}
	if !spec("gql-ha").HighAvailability {
		t.Fatal("GraphQL enable did not persist spec.highAvailability")
	}
	res = gql("small-db", true)
	if len(res.Errors) == 0 || !strings.Contains(fmt.Sprint(res.Errors), "at least 1 CPU") {
		t.Fatalf("GraphQL enable on basic-1gb => want the 1-CPU refusal, got %+v", res)
	}
	if spec("small-db").HighAvailability {
		t.Fatal("refused GraphQL enable wrote spec.highAvailability")
	}
	if res = gql("gql-ha", false); len(res.Errors) > 0 || spec("gql-ha").HighAvailability {
		t.Fatalf("GraphQL disable: %v, HA %v", res.Errors, spec("gql-ha").HighAvailability)
	}

	cs, cleanup := pgMCPSession(t, svc)
	defer cleanup()
	mcpUpdate := func(id string, enabled bool) (*mcp.CallToolResult, error) {
		return cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_postgres",
			Arguments: map[string]any{"postgresId": id, "enableHighAvailability": enabled}})
	}
	out, err := mcpUpdate("mcp-ha", true)
	if err != nil || out.IsError {
		t.Fatalf("MCP enable: %v %+v", err, out)
	}
	view, _ := json.Marshal(out.StructuredContent)
	if !strings.Contains(string(view), `"highAvailabilityEnabled":true`) || strings.Contains(string(view), "highAvailabilityRequested") {
		t.Fatalf("MCP enable answered %s, want Render's echoed highAvailabilityEnabled and no bex-only field", view)
	}
	if !spec("mcp-ha").HighAvailability {
		t.Fatal("MCP enable did not persist spec.highAvailability")
	}
	out, err = mcpUpdate("small-db", true)
	if err != nil || !out.IsError {
		t.Fatalf("MCP enable on basic-1gb => want a tool error, got %v %+v", err, out)
	}
	if text, _ := json.Marshal(out.Content); !strings.Contains(string(text), "at least 1 CPU") {
		t.Fatalf("MCP refusal = %s", text)
	}
	if spec("small-db").HighAvailability {
		t.Fatal("refused MCP enable wrote spec.highAvailability")
	}
	if out, err = mcpUpdate("mcp-ha", false); err != nil || out.IsError || spec("mcp-ha").HighAvailability {
		t.Fatalf("MCP disable: %v %+v", err, out)
	}
}
