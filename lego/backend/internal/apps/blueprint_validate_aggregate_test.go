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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Two independent semantic errors of the same kind (w8/019): past the schema
// stage, validation used to stop at the first service, drop its location, and
// leak "bad request:" mid-message.
const twoFreeWorkers = `services:
  - type: worker
    name: qa5-wrk
    runtime: image
    image:
      url: docker.io/library/busybox:1.36
    plan: free
  - type: worker
    name: qa5-wrk2
    runtime: image
    image:
      url: docker.io/library/busybox:1.36
    plan: free
`

func TestValidateBlueprintAggregatesPerServiceErrorsWithLocations(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", twoFreeWorkers, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) != 2 {
		t.Fatalf("want two errors, got %+v", v)
	}
	for i, want := range []struct {
		path string
		line int
	}{{"services[0].plan", 7}, {"services[1].plan", 13}} {
		e := v.Errors[i]
		if e.Path == nil || *e.Path != want.path {
			t.Errorf("error %d path = %v, want %s", i, e.Path, want.path)
		}
		if e.Line == nil || *e.Line != want.line {
			t.Errorf("error %d line = %v, want %d", i, e.Line, want.line)
		}
		if e.Column == nil || *e.Column <= 0 {
			t.Errorf("error %d column = %v, want set", i, e.Column)
		}
		if strings.Contains(e.Error, "bad request") {
			t.Errorf("error %d leaks the sentinel: %q", i, e.Error)
		}
		if !strings.Contains(e.Error, "requires a paid plan") {
			t.Errorf("error %d = %q, want the paid-plan refusal", i, e.Error)
		}
	}

	// Every surface returns the same two entries.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q}`, twoFreeWorkers))))
	var rest BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST unmarshal: %v (%s)", err, rec.Body.String())
	}
	if len(rest.Errors) != 2 || *rest.Errors[1].Path != "services[1].plan" || *rest.Errors[1].Line != 13 {
		t.Errorf("REST errors = %+v", rest.Errors)
	}
	res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(),
		RequestString: fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errorDetails { path line } } }`, twoFreeWorkers)})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	details, _ := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)["errorDetails"].([]any)
	if len(details) != 2 || details[1].(map[string]any)["path"] != "services[1].plan" {
		t.Errorf("GraphQL errorDetails = %+v", details)
	}
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpErrors, _ := call("validate_bex_yml", map[string]any{"bexYaml": twoFreeWorkers})["errors"].([]any)
	if len(mcpErrors) != 2 || mcpErrors[1].(map[string]any)["path"] != "services[1].plan" {
		t.Errorf("MCP errors = %+v", mcpErrors)
	}
}

// A parse-stage refusal (reserved PORT) and a later service's own check (free
// worker) are independent: both are reported.
func TestValidateBlueprintReportsParseAndServiceErrorsTogether(t *testing.T) {
	const manifest = `services:
  - type: web
    name: qa5-web
    runtime: image
    image:
      url: nginx:1
    envVars:
      - key: PORT
        value: "8080"
  - type: worker
    name: qa5-wrk
    runtime: image
    image:
      url: docker.io/library/busybox:1.36
    plan: free
`
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) != 2 {
		t.Fatalf("want the PORT and the plan errors, got %+v", v.Errors)
	}
	if e := v.Errors[0]; !strings.Contains(e.Error, "PORT") || e.Path == nil || *e.Path != "services[0].envVars[0]" || e.Line == nil || *e.Line != 8 {
		t.Errorf("PORT entry = %q path=%v line=%v", e.Error, ptrValue(e.Path), ptrValue(e.Line))
	}
	if e := v.Errors[1]; e.Path == nil || *e.Path != "services[1].plan" || e.Line == nil || *e.Line != 15 {
		t.Errorf("plan entry = %+v", e)
	}
}

// A dangling workspace reference points at the envVars entry that made it,
// not at nothing (w8/019 sweep 8).
func TestValidateBlueprintLocatesDanglingWorkspaceReference(t *testing.T) {
	svc, _ := connectionService(t)
	v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, m118DanglingDatabase, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if len(v.Errors) != 1 {
		t.Fatalf("errors = %+v", v.Errors)
	}
	e := v.Errors[0]
	if e.Path == nil || !strings.Contains(*e.Path, ".envVars[") || e.Line == nil || *e.Line <= 0 {
		t.Errorf("dangling reference entry = %+v, want an envVars[i] path with a line", e)
	}
}

func ptrValue[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
