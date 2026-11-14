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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// schemaGoldenPath is the served GraphQL schema as SDL, which the dashboard's
// test validates every operation against (dashboard/src/graphql/__tests__/
// schema-validity.test.ts). It is SDL, not introspection JSON: the JSON is over
// the repository's 1 MiB blob limit, and a diff of SDL names the field that
// changed.
const schemaGoldenPath = "testdata/schema.graphql"

// servedSchema builds the schema the server serves. Sandbox registers fields
// only when its client is wired; an empty client includes its schema without
// live calls.
func servedSchema(t testing.TB) graphql.Schema {
	t.Helper()
	srv := NewServer(&core.Base{Namespace: "default"}, Deps{SandboxClient: &sandbox.Client{}})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return schema
}

// TestGraphQLSchemaMatchesTheGolden fails when the served schema and
// testdata/schema.graphql differ, so a change to the API's types, fields or
// arguments reaches the dashboard's operation check in the same commit. After an
// intended change, regenerate the file:
//
//	UPDATE_SCHEMA_GOLDEN=1 go test ./internal/api -run TestGraphQLSchemaMatchesTheGolden
func TestGraphQLSchemaMatchesTheGolden(t *testing.T) {
	got := printSchema(servedSchema(t))
	if os.Getenv("UPDATE_SCHEMA_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(schemaGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(schemaGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d bytes)", schemaGoldenPath, len(got))
		return
	}
	want, err := os.ReadFile(schemaGoldenPath)
	if err != nil {
		t.Fatalf("%v; create it with UPDATE_SCHEMA_GOLDEN=1", err)
	}
	if got != string(want) {
		t.Fatalf("the served GraphQL schema differs from %s (first difference: %s); regenerate it with UPDATE_SCHEMA_GOLDEN=1 go test ./internal/api -run TestGraphQLSchemaMatchesTheGolden",
			schemaGoldenPath, firstDifference(string(want), got))
	}
	if len(got) > 1<<20 {
		t.Fatalf("%s is %d bytes, over the repository's 1 MiB blob limit", schemaGoldenPath, len(got))
	}
}

// printSchema renders the types a client can query as SDL, deterministically
// (names sorted) and without descriptions: what a document is validated
// against is the shape, the names, the argument and field types, and which
// arguments are required.
func printSchema(schema graphql.Schema) string {
	var b strings.Builder
	roots := []string{}
	for _, r := range []struct {
		kind string
		obj  *graphql.Object
	}{{"query", schema.QueryType()}, {"mutation", schema.MutationType()}, {"subscription", schema.SubscriptionType()}} {
		if r.obj != nil {
			roots = append(roots, fmt.Sprintf("  %s: %s", r.kind, r.obj.Name()))
		}
	}
	fmt.Fprintf(&b, "schema {\n%s\n}\n", strings.Join(roots, "\n"))

	typeMap := schema.TypeMap()
	names := make([]string, 0, len(typeMap))
	for name := range typeMap {
		if !strings.HasPrefix(name, "__") && !builtinScalar(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString("\n")
		b.WriteString(printType(typeMap[name]))
	}
	for _, d := range schema.Directives() {
		if d.Name == "include" || d.Name == "skip" || d.Name == "deprecated" {
			continue
		}
		fmt.Fprintf(&b, "\ndirective @%s%s on %s\n", d.Name, printArgs(d.Args), strings.Join(directiveLocations(d.Locations), " | "))
	}
	return b.String()
}

func builtinScalar(name string) bool {
	switch name {
	case "String", "Int", "Float", "Boolean", "ID":
		return true
	}
	return false
}

func directiveLocations(locations []string) []string {
	out := append([]string(nil), locations...)
	sort.Strings(out)
	return out
}

func printType(t graphql.Type) string {
	switch t := t.(type) {
	case *graphql.Object:
		header := "type " + t.Name()
		if ifaces := t.Interfaces(); len(ifaces) > 0 {
			implements := make([]string, len(ifaces))
			for i, iface := range ifaces {
				implements[i] = iface.Name()
			}
			sort.Strings(implements)
			header += " implements " + strings.Join(implements, " & ")
		}
		return header + printFields(t.Fields())
	case *graphql.Interface:
		return "interface " + t.Name() + printFields(t.Fields())
	case *graphql.InputObject:
		fields := t.Fields()
		fieldNames := make([]string, 0, len(fields))
		for name := range fields {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)
		var b strings.Builder
		b.WriteString("input " + t.Name() + " {\n")
		for _, name := range fieldNames {
			f := fields[name]
			fmt.Fprintf(&b, "  %s: %s%s\n", name, f.Type, printDefault(f.DefaultValue))
		}
		b.WriteString("}\n")
		return b.String()
	case *graphql.Enum:
		values := make([]string, 0, len(t.Values()))
		for _, v := range t.Values() {
			values = append(values, v.Name)
		}
		sort.Strings(values)
		return "enum " + t.Name() + " {\n  " + strings.Join(values, "\n  ") + "\n}\n"
	case *graphql.Union:
		members := make([]string, 0, len(t.Types()))
		for _, o := range t.Types() {
			members = append(members, o.Name())
		}
		sort.Strings(members)
		return "union " + t.Name() + " = " + strings.Join(members, " | ") + "\n"
	case *graphql.Scalar:
		return "scalar " + t.Name() + "\n"
	}
	return fmt.Sprintf("# unprinted %T\n", t)
}

func printFields(fields graphql.FieldDefinitionMap) string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(" {\n")
	for _, name := range names {
		f := fields[name]
		fmt.Fprintf(&b, "  %s%s: %s\n", name, printArgs(f.Args), f.Type)
	}
	b.WriteString("}\n")
	return b.String()
}

// printArgs sorts the arguments by name: graphql-go builds them from a map, so
// the order it hands back changes from run to run, and a client names arguments
// rather than counting them. A default value is kept: a non-null argument with
// one is not required.
func printArgs(args []*graphql.Argument) string {
	if len(args) == 0 {
		return ""
	}
	sorted := append([]*graphql.Argument(nil), args...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PrivateName < sorted[j].PrivateName })
	parts := make([]string, len(sorted))
	for i, a := range sorted {
		parts[i] = fmt.Sprintf("%s: %s%s", a.PrivateName, a.Type, printDefault(a.DefaultValue))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func printDefault(v interface{}) string {
	if v == nil {
		return ""
	}
	return " = " + printLiteral(v)
}

func printLiteral(v interface{}) string {
	switch v := v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case []interface{}:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = printLiteral(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + printLiteral(v[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(v)
}

// firstDifference names the first line two renderings disagree on.
func firstDifference(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d: golden %q, served %q", i+1, w, g)
		}
	}
	return "none"
}
