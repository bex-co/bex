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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTopQueriesSuspendedUnavailable(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "suspended")
	db := &appv1alpha1.Database{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "suspended"}, db); err != nil {
		t.Fatal(err)
	}
	db.Spec.Suspended = true
	if err := cl.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TopQueries(context.Background(), db.Name); !errors.Is(err, core.ErrUnavailable) || !strings.Contains(err.Error(), "suspended") {
		t.Fatalf("error = %v", err)
	}
}

func TestMissingDatabaseInsightsAcrossAdapters(t *testing.T) {
	svc, _ := newService()
	for _, name := range []string{"databaseProcesses", "databaseTopQueries", "databaseSizes", "databaseTableScans"} {
		_, err := svc.GraphQLQuery()[name].Resolve(graphql.ResolveParams{Context: context.Background(), Args: map[string]any{"id": "missing"}})
		if !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for _, name := range []string{"list_postgres_processes", "list_postgres_top_queries", "get_postgres_sizes", "list_postgres_table_scans"} {
		out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{"postgresId": "missing"}})
		if err != nil {
			t.Fatal(err)
		}
		if !out.IsError {
			t.Fatalf("%s hid missing database: %+v", name, out)
		}
		if len(out.Content) == 0 {
			t.Fatal("no error content")
		}
		txt, ok := out.Content[0].(*mcp.TextContent)
		if !ok || !strings.Contains(txt.Text, "not found") {
			t.Fatalf("%s error content = %+v", name, out.Content)
		}
	}
}
