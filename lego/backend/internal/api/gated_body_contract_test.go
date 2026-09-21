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
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// gatedWriteDecoderFields is the mechanical contract for t002/w4/m104: every
// Render-gated write route bex implements that we assert against, paired with
// the JSON fields its DecodeBody destination accepts.
//
// Classification (recorded 2026-09-14):
//
//	(a) handler must cover the schema's required properties — listed here
//	(b) bex extension / native dialect outside this REST contract — GraphQL/MCP
//	    autoscaling minInstances dialect; nested service.autoscaling
//	(c) unreachable — not listed
//
// Autoscaling was the sole (a) mismatch found; the 9-route live sample in
// w4/m104/t002 was otherwise clean. A future schema re-pin or struct rename
// that drops a required field from the decoder fails this test.
var gatedWriteDecoderFields = map[string][]string{
	"PUT /services/{serviceId}/autoscaling":     {"enabled", "min", "max", "criteria"},
	"POST /projects":                            {"name", "ownerId", "environments"},
	"POST /services/{serviceId}/deploys":        {}, // no required properties
	"POST /env-groups":                          {"name", "ownerId", "envVars"},
	"PATCH /services/{serviceId}":               {},               // no required properties
	"PUT /services/{serviceId}/env-vars":        {"key", "value"}, // oneOf variant with value
	"POST /services/{serviceId}/scale":          {"numInstances"},
	"PUT /services/{serviceId}/headers":         {"path", "name", "value"},
	"PUT /services/{serviceId}/routes":          {"type", "source", "destination"},
	"POST /services/{serviceId}/custom-domains": {"name"},
}

func TestGatedWriteBodiesAgreeWithHandlers(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	for key, decoderFields := range gatedWriteDecoderFields {
		method, path, ok := splitMethodPath(key)
		if !ok {
			t.Fatalf("bad inventory key %q", key)
		}
		op := findOperation(contract.document, method, path)
		if op == nil {
			t.Fatalf("%s: operation missing from pinned OpenAPI", key)
		}
		requiredSets := requiredBodyPropertySets(op)
		if len(requiredSets) == 0 {
			continue
		}
		// A schema with oneOf (env-vars) is satisfied when the decoder covers
		// at least one variant's required set.
		covered := false
		var missing []string
		for _, required := range requiredSets {
			miss := missingRequired(decoderFields, required)
			if len(miss) == 0 {
				covered = true
				break
			}
			missing = miss
		}
		if !covered {
			t.Errorf("%s: schema requires %v (or another oneOf variant) but decoder fields are %v",
				key, missing, decoderFields)
		}
	}
}

func splitMethodPath(key string) (method, path string, ok bool) {
	method, path, found := strings.Cut(key, " ")
	return method, path, found && method != "" && path != ""
}

func findOperation(doc *openapi3.T, method, path string) *openapi3.Operation {
	item := doc.Paths.Find(path)
	if item == nil {
		return nil
	}
	return item.GetOperation(method)
}

// requiredBodyPropertySets returns the required property sets for the JSON
// request body. Ordinary objects yield one set; oneOf item schemas yield one
// set per variant (caller accepts any).
func requiredBodyPropertySets(op *openapi3.Operation) [][]string {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	media := op.RequestBody.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		return nil
	}
	return schemaRequiredSets(media.Schema.Value)
}

func schemaRequiredSets(schema *openapi3.Schema) [][]string {
	if schema == nil {
		return nil
	}
	if schema.Type != nil && schema.Type.Is("array") && schema.Items != nil {
		return schemaRequiredSets(schema.Items.Value)
	}
	if len(schema.OneOf) > 0 {
		var sets [][]string
		for _, ref := range schema.OneOf {
			if ref != nil && ref.Value != nil && len(ref.Value.Required) > 0 {
				sets = append(sets, slices.Clone(ref.Value.Required))
			}
		}
		return sets
	}
	if len(schema.Required) == 0 {
		return nil
	}
	return [][]string{slices.Clone(schema.Required)}
}

func missingRequired(decoderFields, required []string) []string {
	var miss []string
	for _, prop := range required {
		if !slices.Contains(decoderFields, prop) {
			miss = append(miss, prop)
		}
	}
	return miss
}
