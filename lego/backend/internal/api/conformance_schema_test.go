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
	"fmt"
	"os"
	"strings"
	"testing"
)

// conformance_schema_test.go — minimal JSON Schema validator for the Render
// OpenAPI conformance suite. Supports the subset used in render-openapi.json:
// object (required/properties), array (items), string/number/integer/boolean
// types, nullable, oneOf, anyOf, and $ref into components/schemas. Enum and
// format constraints are deliberately not checked — this suite validates
// structural shape (required fields present, correct JSON types), not value
// constraints; enum drift is handled per-field in the allowlist.

// schemaNode is a single JSON Schema node as it appears in an OpenAPI 3.0 spec.
type schemaNode struct {
	Type       string                 `json:"type"`
	Properties map[string]*schemaNode `json:"properties"`
	Required   []string               `json:"required"`
	Items      *schemaNode            `json:"items"`
	Ref        string                 `json:"$ref"`
	Nullable   bool                   `json:"nullable"`
	OneOf      []*schemaNode          `json:"oneOf"`
	AnyOf      []*schemaNode          `json:"anyOf"`
}

type renderAPISpec struct {
	Paths      map[string]renderPathItem `json:"paths"`
	Components struct {
		Schemas map[string]*schemaNode `json:"schemas"`
	} `json:"components"`
}

type renderPathItem struct {
	Get  *renderOp `json:"get"`
	Post *renderOp `json:"post"`
}

type renderOp struct {
	OperationID string                 `json:"operationId"`
	Responses   map[string]*renderResp `json:"responses"`
}

type renderResp struct {
	Content map[string]*renderMedia `json:"content"`
}

type renderMedia struct {
	Schema *schemaNode `json:"schema"`
}

// renderSpec is the parsed, ready-to-validate form of the pinned Render OpenAPI
// spec. Load once via loadRenderSpec; validate responses via validate.
type renderSpec struct {
	ops  map[string]*schemaNode // operationId -> 200 response schema
	defs map[string]*schemaNode // components/schemas for $ref resolution
}

// loadRenderSpec parses testdata/render-openapi.json and indexes it by operationId.
func loadRenderSpec(t *testing.T) *renderSpec {
	t.Helper()
	data, err := os.ReadFile("testdata/render-openapi.json")
	if err != nil {
		t.Fatalf("conformance: read testdata/render-openapi.json: %v", err)
	}
	var raw renderAPISpec
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("conformance: parse testdata/render-openapi.json: %v", err)
	}
	ops := make(map[string]*schemaNode)
	for _, pi := range raw.Paths {
		for _, op := range []*renderOp{pi.Get, pi.Post} {
			if op == nil || op.OperationID == "" {
				continue
			}
			resp, ok := op.Responses["200"]
			if !ok {
				continue
			}
			ct, ok := resp.Content["application/json"]
			if !ok {
				continue
			}
			ops[op.OperationID] = ct.Schema
		}
	}
	return &renderSpec{ops: ops, defs: raw.Components.Schemas}
}

// resolve dereferences a $ref of the form "#/components/schemas/<name>" using
// the spec's components/schemas map. Returns n unchanged when no $ref is set.
func (rs *renderSpec) resolve(n *schemaNode) *schemaNode {
	if n == nil || n.Ref == "" {
		return n
	}
	const pref = "#/components/schemas/"
	if !strings.HasPrefix(n.Ref, pref) {
		return n // non-local ref — leave as-is (not used in the pinned spec)
	}
	if def, ok := rs.defs[n.Ref[len(pref):]]; ok {
		return def
	}
	return n
}

// validate checks body against the 200 response schema for operationID.
// Returns a slice of human-readable error strings; empty means fully conformant.
func (rs *renderSpec) validate(operationID string, body []byte) []string {
	schema, ok := rs.ops[operationID]
	if !ok {
		return []string{fmt.Sprintf("unknown operationId %q in pinned spec", operationID)}
	}
	var val any
	if err := json.Unmarshal(body, &val); err != nil {
		return []string{fmt.Sprintf("invalid JSON: %v", err)}
	}
	return rs.walk(val, schema, "$")
}

// walk recursively validates val against schema, returning all violations with
// their JSON-path prefix (e.g. "$[0].service.id: expected string, got <nil>").
func (rs *renderSpec) walk(val any, schema *schemaNode, path string) []string {
	schema = rs.resolve(schema)
	if schema == nil {
		return nil
	}
	// JSON null: valid only when the schema is nullable or has no concrete type.
	if val == nil {
		if schema.Nullable || schema.Type == "" {
			return nil
		}
		return []string{fmt.Sprintf("%s: got null but schema type=%q is not nullable", path, schema.Type)}
	}
	// oneOf: accept if exactly-one branch validates clean (we accept any-one for simplicity).
	if len(schema.OneOf) > 0 {
		for _, b := range schema.OneOf {
			if errs := rs.walk(val, b, path); len(errs) == 0 {
				return nil
			}
		}
		return []string{fmt.Sprintf("%s: value does not match any oneOf branch", path)}
	}
	// anyOf: accept if at least one branch validates clean.
	if len(schema.AnyOf) > 0 {
		for _, b := range schema.AnyOf {
			if errs := rs.walk(val, b, path); len(errs) == 0 {
				return nil
			}
		}
		return []string{fmt.Sprintf("%s: value does not match any anyOf branch", path)}
	}
	switch schema.Type {
	case "object", "":
		obj, ok := val.(map[string]any)
		if schema.Type == "object" && !ok {
			return []string{fmt.Sprintf("%s: expected object, got %T", path, val)}
		}
		if !ok {
			return nil // untyped schema — skip structural checks
		}
		var errs []string
		for _, req := range schema.Required {
			if _, present := obj[req]; !present {
				errs = append(errs, fmt.Sprintf("%s: missing required field %q", path, req))
			}
		}
		for k, propSchema := range schema.Properties {
			v, present := obj[k]
			if !present {
				continue // optional absent field is fine
			}
			errs = append(errs, rs.walk(v, propSchema, path+"."+k)...)
		}
		return errs
	case "array":
		arr, ok := val.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: expected array, got %T", path, val)}
		}
		var errs []string
		for i, item := range arr {
			errs = append(errs, rs.walk(item, schema.Items, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return errs
	case "string":
		if _, ok := val.(string); !ok {
			return []string{fmt.Sprintf("%s: expected string, got %T", path, val)}
		}
	case "number", "integer":
		if _, ok := val.(float64); !ok {
			return []string{fmt.Sprintf("%s: expected number, got %T", path, val)}
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return []string{fmt.Sprintf("%s: expected boolean, got %T", path, val)}
		}
	}
	return nil
}
