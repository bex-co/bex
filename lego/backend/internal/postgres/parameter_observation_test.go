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
	"maps"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestParameterObservationsPreserveDeclarations(t *testing.T) {
	declared := map[string]string{"work_mem": "8MB", "qa_unknown": "1", "maintenance_work_mem": "not-a-size", "default_statistics_target": "200"}
	observed := []ParameterOverrideView{
		{Name: "work_mem", Setting: "8192", Unit: "kB"},
		{Name: "default_statistics_target", Setting: "100"},
		{Name: "archive_command", Setting: "platform-owned"},
	}
	rows := parameterSpecViews(declared, observed, nil)
	if len(rows) != len(declared) {
		t.Fatalf("observed platform setting became editable: %+v", rows)
	}
	for _, row := range rows {
		if row.Value != declared[row.Name] {
			t.Fatalf("declared value replaced: %+v", row)
		}
		switch row.Name {
		case "work_mem":
			if row.ObservationStatus != "observed" || row.ObservedSetting == nil || *row.ObservedSetting != "8192" || row.ObservedUnit == nil || *row.ObservedUnit != "kB" {
				t.Fatalf("normalized observation = %+v", row)
			}
		case "default_statistics_target":
			if row.ObservationStatus != "observed" || row.ObservedSetting == nil || *row.ObservedSetting != "100" || row.ObservedUnit != nil {
				t.Fatalf("prior effective value lost: %+v", row)
			}
		default:
			if row.ObservationStatus != "not_observed" || row.ObservedSetting != nil || row.ObservedUnit != nil {
				t.Fatalf("invalid declaration corroborated: %+v", row)
			}
		}
	}
	for _, row := range parameterSpecViews(declared, observed, errors.New("runtime unavailable")) {
		if row.Value != declared[row.Name] || row.ObservationStatus != "unavailable" || row.ObservedSetting != nil || row.ObservedUnit != nil {
			t.Fatalf("unavailable observation = %+v", row)
		}
	}
}

func TestParameterSpecObservationAuthorizationAndEmptyDeclarations(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "observed-db")
	ctx := ctxAs("user-a")
	svc.Authz = staleAllowChecker{}
	if rows, err := svc.ParameterSpec(ctx, "observed-db"); err != nil || len(rows) != 0 {
		t.Fatalf("empty declarations = %+v, %v", rows, err)
	}
	svc.Authz = nil
	if _, err := svc.SetParameterOverrides(context.Background(), "observed-db", map[string]string{"work_mem": "8MB"}); err != nil {
		t.Fatal(err)
	}
	svc.Authz = staleAllowChecker{}
	if _, err := svc.ParameterSpec(ctx, "observed-db"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("revoked observation = %v", err)
	}
}

func TestParameterSpecUnavailableAcrossSurfaces(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "observed-db")
	declared := map[string]string{"qa_unknown": "1", "work_mem": "not-a-size"}
	if _, err := svc.SetParameterOverrides(context.Background(), "observed-db", declared); err != nil {
		t.Fatal(err)
	}
	assertRows := func(rows []ParameterSpecView) {
		t.Helper()
		if len(rows) != 2 {
			t.Fatalf("declared rows = %+v", rows)
		}
		for _, row := range rows {
			if row.Value != declared[row.Name] || row.ObservationStatus != "unavailable" || row.ObservedSetting != nil || row.ObservedUnit != nil {
				t.Fatalf("observation = %+v", row)
			}
		}
	}
	rest := serveREST(svc, http.MethodGet, "/v1/postgres/observed-db/parameters", "")
	if rest.Code != http.StatusOK {
		t.Fatalf("REST: %d %s", rest.Code, rest.Body.String())
	}
	var rows []ParameterSpecView
	if err := json.Unmarshal(rest.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	assertRows(rows)
	var raw []map[string]any
	if err := json.Unmarshal(rest.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, row := range raw {
		for _, key := range []string{"observedSetting", "observedUnit"} {
			if v, exists := row[key]; !exists || v != nil {
				t.Fatalf("%s must be explicit null: %+v", key, row)
			}
		}
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ databaseParameterSpec(id:"observed-db") { name value observationStatus observedSetting observedUnit } }`})
	if len(gql.Errors) != 0 {
		t.Fatalf("GraphQL: %v", gql.Errors)
	}
	encoded, err := json.Marshal(gql.Data.(map[string]any)["databaseParameterSpec"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &rows); err != nil {
		t.Fatal(err)
	}
	assertRows(rows)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_postgres_parameters", Arguments: map[string]any{"postgresId": "observed-db"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("MCP: %+v", result)
	}
	encoded, err = json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out parameterSpecResult
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	assertRows(out.Parameters)
	current, err := svc.GetParameterSpec(context.Background(), "observed-db")
	if err != nil || !maps.Equal(current, declared) {
		t.Fatalf("diagnostics changed editor source: %+v, %v", current, err)
	}
}
