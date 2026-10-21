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
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// renderFieldProbe is one served operation and a body the composed stack
// already accepts; the guard adds each property Render declares, one at a time.
type renderFieldProbe struct {
	op, method, path string
	base             map[string]any
	// nested names an object property whose oneOf variant (by schema name) is
	// probed field by field inside base[nested].
	nested, variant string
}

// declaredFieldAllowlist records Render-declared request fields bex answers
// with something other than "handled" or a named refusal, with the reason.
// A generic `unknown field` not listed here fails the guard (w5/m146).
var declaredFieldAllowlist = map[string]string{}

// TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName is w5/m146's guard:
// every request property the pinned OpenAPI declares on a served body
// operation, sent alongside a body the stack accepts, either reaches the
// handler or answers a named refusal — never the strict decoder's generic
// `unknown field`, which tells a Render client nothing. It closes the class
// w1/m86, w6/m106 and w6/m130 fixed one field at a time.
func TestRenderDeclaredRequestFieldsAreHandledOrRefusedByName(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(renderOpenAPISource)
	if err != nil {
		t.Fatal(err)
	}
	const srv = "srv-00000000000000000000"
	image := map[string]any{"imagePath": "docker.io/library/nginx:alpine", "ownerId": ""}
	repo := "https://github.com/render-examples/express-hello-world"
	probes := []renderFieldProbe{
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "web_service", "name": "web2", "image": image}, "", ""},
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "web_service", "name": "web3", "image": image, "serviceDetails": map[string]any{"runtime": "image"}}, "serviceDetails", "webServiceDetailsPOST"},
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "private_service", "name": "pserv", "image": image, "serviceDetails": map[string]any{"runtime": "image", "plan": "starter"}}, "serviceDetails", "privateServiceDetailsPOST"},
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "background_worker", "name": "worker", "image": image, "serviceDetails": map[string]any{"runtime": "image"}}, "serviceDetails", "backgroundWorkerDetailsPOST"},
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "static_site", "name": "site", "repo": repo, "serviceDetails": map[string]any{"publishPath": "public"}}, "serviceDetails", "staticSiteDetailsPOST"},
		{"create-service", http.MethodPost, "/v1/services", map[string]any{"type": "cron_job", "name": "cron", "image": image, "serviceDetails": map[string]any{"runtime": "image", "schedule": "0 * * * *"}}, "serviceDetails", "cronJobDetailsPOST"},
		{"update-service", http.MethodPatch, "/v1/services/" + srv, map[string]any{"name": "web4"}, "", ""},
		{"update-service", http.MethodPatch, "/v1/services/" + srv, map[string]any{"serviceDetails": map[string]any{}}, "serviceDetails", "webServiceDetailsPATCH"},
		{"create-postgres", http.MethodPost, "/v1/postgres", map[string]any{"name": "pg", "plan": "free", "version": "16", "ownerId": "tea-a"}, "", ""},
		{"update-postgres", http.MethodPatch, "/v1/postgres/dpg-00000000000000000000", map[string]any{"name": "pg2"}, "", ""},
		{"create-key-value", http.MethodPost, "/v1/key-value", map[string]any{"name": "kv", "plan": "free", "ownerId": "tea-a"}, "", ""},
		{"update-key-value", http.MethodPatch, "/v1/key-value/red-00000000000000000000", map[string]any{"name": "kv2"}, "", ""},
	}
	unprobed := 0
	for _, p := range probes {
		schema := renderRequestSchema(t, doc, p.op)
		if p.nested != "" {
			schema = oneOfVariant(t, doc, schema.Properties[p.nested].Value, p.variant)
		}
		for _, name := range slices.Sorted(maps.Keys(schema.Properties)) {
			target := p.base
			if p.nested != "" {
				target, _ = p.base[p.nested].(map[string]any)
			}
			if _, inBase := target[name]; inBase {
				continue
			}
			key := p.op + "." + name
			if p.nested != "" {
				key = p.op + "." + p.nested + "(" + p.variant + ")." + name
			}
			t.Run(key, func(t *testing.T) {
				body := cloneJSON(p.base)
				dest := body
				if p.nested != "" {
					dest = body[p.nested].(map[string]any)
				}
				dest[name] = exampleFor(schema.Properties[name].Value)
				raw, _ := json.Marshal(body)
				h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
				w := do(t, h, p.method, p.path, testToken, string(raw))
				got := w.Body.String()
				switch {
				case strings.Contains(got, "does not match the Render API schema") || strings.Contains(got, "invalid request body at"):
					// The generated example did not satisfy the validator, so the
					// handler was never reached; counted, not silently passed.
					unprobed++
					t.Logf("unprobed (validator refused the example %s): %s", raw, got)
				case strings.Contains(got, "unknown field"):
					if reason, ok := declaredFieldAllowlist[key]; ok {
						t.Logf("allowlisted: %s", reason)
						return
					}
					t.Errorf("Render declares %s but bex answers a generic %d %s — handle it or refuse it by name", key, w.Code, got)
				}
			})
		}
	}
	// An example the validator refuses never reaches the handler, so a rising
	// count would let the guard pass while checking nothing.
	if unprobed > 0 {
		t.Fatalf("%d declared properties were never probed; fix exampleFor or allowlist them", unprobed)
	}
}

func renderRequestSchema(t *testing.T, doc *openapi3.T, opID string) *openapi3.Schema {
	t.Helper()
	_, op := findRenderOperation(t, doc, opID)
	if op.RequestBody == nil || op.RequestBody.Value.Content.Get("application/json") == nil {
		t.Fatalf("no JSON request body for %s", opID)
	}
	return op.RequestBody.Value.Content.Get("application/json").Schema.Value
}

func oneOfVariant(t *testing.T, doc *openapi3.T, s *openapi3.Schema, name string) *openapi3.Schema {
	t.Helper()
	for _, ref := range s.OneOf {
		if strings.HasSuffix(ref.Ref, "/"+name) {
			return ref.Value
		}
	}
	if len(s.OneOf) == 0 && len(s.Properties) > 0 {
		return s
	}
	t.Fatalf("no oneOf variant %s", name)
	return nil
}

// exampleFor builds a value the pinned schema accepts: its example, its first
// enum value, or a typed placeholder, recursing into objects and arrays.
func exampleFor(s *openapi3.Schema) any {
	if s == nil {
		return "x"
	}
	if s.Example != nil {
		return s.Example
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	if len(s.OneOf) > 0 {
		return exampleFor(s.OneOf[0].Value)
	}
	if len(s.AllOf) > 0 {
		return exampleFor(s.AllOf[0].Value)
	}
	switch {
	case s.Type.Is("boolean"):
		return true
	case s.Type.Is("integer"), s.Type.Is("number"):
		if s.Min != nil {
			return *s.Min
		}
		return 1
	case s.Type.Is("array"):
		if s.Items == nil {
			return []any{}
		}
		return []any{exampleFor(s.Items.Value)}
	case s.Type.Is("object") || len(s.Properties) > 0:
		out := map[string]any{}
		for _, r := range s.Required {
			if prop, ok := s.Properties[r]; ok {
				out[r] = exampleFor(prop.Value)
			}
		}
		return out
	case s.Format == "date-time":
		return "2026-10-20T00:00:00Z"
	case s.Format == "uri":
		return "https://example.com"
	default:
		return "x"
	}
}

func cloneJSON(m map[string]any) map[string]any {
	raw, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
