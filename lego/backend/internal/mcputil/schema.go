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

package mcputil

import (
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

// SchemaOption adjusts a property on an inferred MCP input schema.
type SchemaOption func(*jsonschema.Schema)

// InputSchema infers the JSON Schema for In using google/jsonschema-go (the
// same engine mcp.AddTool uses) and applies each SchemaOption. It panics on
// inference failure or unknown property names, matching mcp.AddTool's
// registration-time fail-fast behavior.
func InputSchema[In any](opts ...SchemaOption) *jsonschema.Schema {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(fmt.Errorf("mcputil.InputSchema: %w", err))
	}
	for _, opt := range opts {
		opt(schema)
	}
	return schema
}

// PropEnum sets a JSON Schema "enum" constraint on property name. When the
// field was inferred from a nullable *string pointer (["null", "string"]), the
// type is narrowed to "string" so the advertised type agrees with the enum.
func PropEnum(name string, values ...string) SchemaOption {
	return func(s *jsonschema.Schema) {
		if len(values) == 0 {
			panic(fmt.Sprintf("mcputil.PropEnum(%q): empty enum", name))
		}
		prop := mustProperty(s, name)
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = v
		}
		prop.Enum = enum
		if len(prop.Types) == 2 && slices.Contains(prop.Types, "null") && slices.Contains(prop.Types, "string") {
			prop.Type = "string"
			prop.Types = nil
		}
	}
}

// PropTypes sets an explicit JSON Schema "type" (or union type list) on
// property name, replacing the untyped {} schema jsonschema-go infers for an
// any-typed field.
func PropTypes(name string, types ...string) SchemaOption {
	return func(s *jsonschema.Schema) {
		if len(types) == 0 {
			panic(fmt.Sprintf("mcputil.PropTypes(%q): empty types", name))
		}
		prop := mustProperty(s, name)
		if len(types) == 1 {
			prop.Type = types[0]
			prop.Types = nil
			return
		}
		prop.Type = ""
		prop.Types = append([]string(nil), types...)
	}
}

// MergeEnums merges one or more string slices into a sorted, deduplicated slice.
func MergeEnums(sets ...[]string) []string {
	var merged []string
	for _, set := range sets {
		merged = append(merged, set...)
	}
	slices.Sort(merged)
	return slices.Compact(merged)
}

func mustProperty(s *jsonschema.Schema, name string) *jsonschema.Schema {
	if s == nil || s.Properties == nil {
		panic(fmt.Sprintf("mcputil: schema has no properties (looking for %q)", name))
	}
	prop, ok := s.Properties[name]
	if !ok || prop == nil {
		panic(fmt.Sprintf("mcputil: schema has no property %q", name))
	}
	return prop
}
