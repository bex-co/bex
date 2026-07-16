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
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestDiskAutoscalingRESTCreatePatchAndReadFieldNames(t *testing.T) {
	svc, cl := newService()
	w := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"autoscale-db","plan":"free","enableDiskAutoscaling":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	var created PostgresView
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !created.DiskAutoscalingEnabled {
		t.Fatalf("create read field diskAutoscalingEnabled = false: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"enableDiskAutoscaling"`) {
		t.Fatalf("write-side field leaked into read object: %s", w.Body.String())
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: created.ID}, &db); err != nil {
		t.Fatal(err)
	}
	if !db.Spec.DiskAutoscaling {
		t.Fatal("create did not persist spec.diskAutoscaling")
	}

	w = serveREST(svc, http.MethodPatch, "/v1/postgres/"+created.ID, `{"enableDiskAutoscaling":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.DiskAutoscalingEnabled {
		t.Fatal("PATCH false did not round-trip on diskAutoscalingEnabled")
	}
	w = serveREST(svc, http.MethodGet, "/v1/postgres/"+created.ID, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"diskAutoscalingEnabled":false`) {
		t.Fatalf("GET read shape = %d: %s", w.Code, w.Body.String())
	}
}

func TestDiskAutoscalingGraphQLAndMCPAdapterParity(t *testing.T) {
	gqlDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "gql-autoscale", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "gql-autoscale", Plan: "free"},
	}
	mcpDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "mcp-autoscale", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "mcp-autoscale", Plan: "free"},
	}
	svc, cl := newService(gqlDB, mcpDB)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema:  schema,
		Context: context.Background(),
		RequestString: `mutation {
          updateDatabaseDiskAutoscaling(id: "gql-autoscale", enabled: true) {
            id
            diskAutoscalingEnabled
          }
        }`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL mutation errors: %+v", result.Errors)
	}
	gqlView := result.Data.(map[string]any)["updateDatabaseDiskAutoscaling"].(map[string]any)
	if gqlView["diskAutoscalingEnabled"] != true {
		t.Fatalf("GraphQL read field = %+v", gqlView)
	}

	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	mcpView := call("update_postgres_disk_autoscaling", map[string]any{
		"postgresId": "mcp-autoscale",
		"enabled":    true,
	})
	if mcpView["diskAutoscalingEnabled"] != true {
		t.Fatalf("MCP read field = %+v", mcpView)
	}

	for _, name := range []string{"gql-autoscale", "mcp-autoscale"} {
		var got appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &got); err != nil {
			t.Fatal(err)
		}
		if !got.Spec.DiskAutoscaling {
			t.Fatalf("%s adapter did not persist the shared CR field", name)
		}
	}
}

func TestDiskAutoscalingCreateParityGraphQLAndMCP(t *testing.T) {
	svc, _ := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `mutation { createDatabase(name:"gql-created", enableDiskAutoscaling:true) { diskAutoscalingEnabled } }`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL create errors: %+v", result.Errors)
	}
	view := result.Data.(map[string]any)["createDatabase"].(map[string]any)
	if view["diskAutoscalingEnabled"] != true {
		t.Fatalf("GraphQL create = %+v", view)
	}

	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	mcpView := call("create_postgres", map[string]any{
		"name":                  "mcp-created",
		"enableDiskAutoscaling": true,
	})
	if mcpView["diskAutoscalingEnabled"] != true {
		t.Fatalf("MCP create = %+v", mcpView)
	}
}
