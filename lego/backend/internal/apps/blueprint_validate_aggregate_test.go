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
	"errors"
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

func TestValidateBlueprintAggregatesAcrossResourceKinds(t *testing.T) {
	const manifest = `services:
  - type: web
    name: bad-service
    runtime: image
    image:
      url: nginx:1
    envVars:
      - key: BAD-KEY
        value: bad
  - type: keyvalue
    name: bad-kv
    ipAllowList:
      - source: not-a-cidr
databases:
  - name: bad-db
    databaseName: BadName
envVarGroups:
  - name: bad-group
    envVars:
      - key: 1BAD
        value: bad
`
	// Apply still sees the first refusal in parse order (groups precede
	// databases and services), even though validate presents source order.
	_, applyErr := parseStack(DeployRequest{Manifest: manifest})
	if !errors.Is(applyErr, core.ErrBadRequest) || !strings.Contains(applyErr.Error(), "1BAD") {
		t.Fatalf("apply error = %v, want the first env-group refusal", applyErr)
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid || len(v.Errors) != 4 {
		t.Fatalf("want all four resource refusals, got %+v", v)
	}
	for i, want := range []struct {
		path string
		line int
	}{
		{"services[0].envVars[0]", 8},
		{"services[1].ipAllowList", 13},
		{"databases[0].databaseName", 16},
		{"envVarGroups[0].envVars[0]", 20},
	} {
		e := v.Errors[i]
		if e.Path == nil || *e.Path != want.path || e.Line == nil || *e.Line != want.line || e.Column == nil || *e.Column <= 0 {
			t.Errorf("error %d: path=%v line=%v column=%v, want %s line %d: %s", i, ptrValue(e.Path), ptrValue(e.Line), ptrValue(e.Column), want.path, want.line, e.Error)
		}
	}
}

func TestValidateBlueprintRefusedGroupDoesNotCascade(t *testing.T) {
	const manifest = `envVarGroups:
  - name: bad-group
    envVars:
      - key: 1BAD
        value: bad
services:
  - type: web
    name: web
    runtime: image
    image:
      url: nginx:1
    envVars:
      - fromGroup: bad-group
      - key: OTHER
        fromService:
          type: web
          name: absent
          property: host
`
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid || len(v.Errors) != 1 {
		t.Fatalf("want only the group refusal, got %+v", v)
	}
	if e := v.Errors[0]; e.Path == nil || *e.Path != "envVarGroups[0].envVars[0]" || !strings.Contains(e.Error, "1BAD") {
		t.Fatalf("want located group refusal, got %+v", e)
	}
}

// w8/051: every dangling workspace reference gets its own located entry —
// grouped by sorted target name, each in document order — identically on every run — the resolver used to stop at the
// first name drawn from a Go map. Apply's single message is the sorted first.
func TestValidateBlueprintReportsEveryDanglingWorkspaceReference(t *testing.T) {
	const manifest = `services:
  - type: web
    name: qa-bp4-web
    runtime: image
    image: { url: docker.io/mendhak/http-https-echo:35 }
    plan: free
    envVars:
      - key: B_URL
        fromDatabase: { name: qa-bp4-missing-b, property: connectionString }
      - key: A_URL
        fromDatabase: { name: qa-bp4-missing-a, property: connectionString }
      - key: KV_URL
        fromService: { type: keyvalue, name: qa-bp4-missing-kv, property: connectionString }
  - type: web
    name: qa-bp4-web2
    runtime: image
    image: { url: docker.io/mendhak/http-https-echo:35 }
    plan: free
    envVars:
      - key: C_URL
        fromDatabase: { name: qa-bp4-missing-c, property: host }
      - key: A_HOST
        fromDatabase: { name: qa-bp4-missing-a, property: host }
      - key: PEER
        fromService: { type: web, name: qa-bp4-missing-svc, property: host }
`
	want := []string{
		"services[0].envVars[1] " + `fromDatabase references unknown database "qa-bp4-missing-a" in this workspace`,
		"services[1].envVars[1] " + `fromDatabase references unknown database "qa-bp4-missing-a" in this workspace`,
		"services[0].envVars[0] " + `fromDatabase references unknown database "qa-bp4-missing-b" in this workspace`,
		"services[1].envVars[0] " + `fromDatabase references unknown database "qa-bp4-missing-c" in this workspace`,
		"services[0].envVars[2] " + `fromService references unknown Key Value "qa-bp4-missing-kv" in this workspace`,
		"services[1].envVars[2] " + `service "qa-bp4-web2": fromService references unknown service "qa-bp4-missing-svc" (declare it under services: or create it in this workspace first)`,
	}
	svc, _ := connectionService(t)
	for run := 0; run < 10; run++ {
		v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
		if err != nil {
			t.Fatalf("ValidateBlueprint: %v", err)
		}
		got := make([]string, 0, len(v.Errors))
		for _, e := range v.Errors {
			if e.Line == nil || *e.Line <= 0 {
				t.Errorf("entry %q has no line", e.Error)
			}
			got = append(got, fmt.Sprint(ptrValue(e.Path))+" "+e.Error)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("run %d errors =\n%s\nwant\n%s", run, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	st, err := parseStack(DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	for run := 0; run < 10; run++ {
		_, _, err := svc.resolveExistingBlueprintReferences(ownershipCtx(), st, nil, nil)
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), `"qa-bp4-missing-a"`) {
			t.Fatalf("apply resolver run %d = %v, want the sorted-first missing database", run, err)
		}
	}
}
