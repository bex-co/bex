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
	"encoding/json"
	"os"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestDumpGraphQLSchema is a developer tool, not an assertion: set
// SCHEMA_DUMP_PATH to write the server's GraphQL introspection JSON there, so
// the dashboard's graphql-codegen can regenerate src/graphql/definitions.ts
// offline (no live bex-api needed). Skipped in normal runs.
func TestDumpGraphQLSchema(t *testing.T) {
	path := os.Getenv("SCHEMA_DUMP_PATH")
	if path == "" {
		t.Skip("set SCHEMA_DUMP_PATH to dump the introspection JSON")
	}
	srv := NewServer(&core.Base{Namespace: "default"}, Deps{})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: testutil.IntrospectionQuery})
	if len(res.Errors) > 0 {
		t.Fatalf("introspection: %v", res.Errors)
	}
	out, err := json.MarshalIndent(res.Data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote introspection JSON to %s (%d bytes)", path, len(out))
}
