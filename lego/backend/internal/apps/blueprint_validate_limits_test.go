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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/026 (B): `fromService … property: host` may name a service outside the
// file, and validate skipped it — a target that existed nowhere validated as
// `valid: true` and failed only at apply.
const hostToNowhere = `services:
  - type: web
    name: qa33-web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: PEER
        fromService:
          type: pserv
          name: qa33-nope-anywhere
          property: host
`

func TestValidateBlueprintChecksOutOfFileServiceHosts(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	v, err := svc.ValidateBlueprint(ctx, connOwner, hostToNowhere, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) != 1 || !strings.Contains(v.Errors[0].Error, `fromService references unknown service "qa33-nope-anywhere"`) {
		t.Fatalf("host to a service that exists nowhere = %+v, want the unknown-service error", v)
	}
	if e := v.Errors[0]; e.Path == nil || !strings.Contains(*e.Path, ".envVars[0]") || e.Line == nil {
		t.Errorf("entry = %+v, want it located at the envVars entry", e)
	}

	// An existing workspace service is still a valid target.
	peer := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "tea-a-qa33-nope-anywhere", Namespace: "tea-a",
			Labels: map[string]string{core.LabelTenant: "tea-a", core.LabelServiceName: "qa33-nope-anywhere"}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, Image: "nginx:1", Port: 8080},
	}
	if err := svc.Client.Create(ctx, peer); err != nil {
		t.Fatal(err)
	}
	if v, err := svc.ValidateBlueprint(ctx, connOwner, hostToNowhere, ""); err != nil || !v.Valid {
		t.Fatalf("host to an existing workspace service = %+v, %v; want valid", v, err)
	}
}

func blueprintValidationRequest(t *testing.T, format, manifest string) *http.Request {
	t.Helper()
	if format == "multipart" {
		return multipartBlueprintRequest(t, connOwner, "render.yaml", manifest)
	}
	encoded, err := json.Marshal(map[string]string{"bexYaml": manifest, "ownerId": connOwner})
	if err != nil {
		t.Fatal(err)
	}
	if format == "json-escaped" {
		encoded = bytes.ReplaceAll(encoded, []byte("x"), []byte(`\u0078`))
	}
	req := httptest.NewRequest(http.MethodPost, BlueprintValidationPath, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestValidateBlueprintManifestSizeBoundary(t *testing.T) {
	svc, _ := connectionService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	const valid = "\nservices:\n  - type: web\n    name: web\n    runtime: image\n    image: {url: nginx:1}\n"
	for _, format := range []string{"json", "json-escaped", "multipart"} {
		for _, size := range []int{blueprintMaxManifestBytes, blueprintMaxManifestBytes + 1, 600 << 10, 3 << 20} {
			if format == "json-escaped" && size > blueprintMaxManifestBytes+1 {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", format, size), func(t *testing.T) {
				manifest := "#" + strings.Repeat("x", size-len(valid)-1) + valid
				req := blueprintValidationRequest(t, format, manifest)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req.WithContext(ownershipCtx()))
				if size == blueprintMaxManifestBytes {
					var result BlueprintValidation
					if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || !result.Valid {
						t.Fatalf("at-limit manifest: %d %s", rec.Code, rec.Body.String())
					}
					return
				}
				var refusal struct{ Message string }
				if rec.Code != http.StatusRequestEntityTooLarge || json.Unmarshal(rec.Body.Bytes(), &refusal) != nil || refusal.Message != "Blueprint manifests are limited to 512 KiB" {
					t.Fatalf("over-limit manifest: %d %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestValidateBlueprintSizeDiagnosticAcrossGraphQLAndMCP(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := blueprintSchema(t, svc)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	for _, size := range []int{600 << 10, 3 << 20} {
		manifest := strings.Repeat("x", size)
		result := graphql.Do(graphql.Params{
			Schema: schema, Context: context.Background(),
			RequestString:  `query($manifest:String!) { validateBlueprint(bexYaml:$manifest) { valid errorDetails { code error } } }`,
			VariableValues: map[string]any{"manifest": manifest},
		})
		if len(result.Errors) > 0 {
			t.Fatalf("GraphQL: %v", result.Errors)
		}
		gqlValidation := result.Data.(map[string]any)["validateBlueprint"].(map[string]any)
		mcpValidation := call("validate_bex_yml", map[string]any{"bexYaml": manifest})
		for surface, validation := range map[string]map[string]any{"GraphQL": gqlValidation, "MCP": mcpValidation} {
			errorsKey := "errors"
			if surface == "GraphQL" {
				errorsKey = "errorDetails"
			}
			details := validation[errorsKey].([]any)
			if validation["valid"] != false || len(details) != 1 {
				t.Fatalf("%s size %d: %+v", surface, size, validation)
			}
			problem := details[0].(map[string]any)
			if problem["code"] != "BLUEPRINT_YAML_TOO_LARGE" || problem["error"] != "Blueprint manifests are limited to 512 KiB" {
				t.Fatalf("%s size %d: %+v", surface, size, problem)
			}
		}
	}
}

// w8/027: `blueprints validate` passed envVars with names every env write
// refuses; each is now a located validation error.
func TestValidateBlueprintRefusesInvalidEnvVarNames(t *testing.T) {
	svc, _ := connectionService(t)
	manifest := `services:
  - type: worker
    name: qa35-wrk
    runtime: image
    image: {url: busybox:1.36}
    envVars:
      - key: OK_NAME
        value: "1"
      - key: 1BAD
        value: x
envVarGroups:
  - name: qa35-group
    envVars:
      - key: "BAD KEY"
        value: y
`
	v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) == 0 {
		t.Fatalf("invalid env var names validated: %+v", v)
	}
	for _, e := range v.Errors {
		if !strings.Contains(e.Error, "invalid environment variable name") {
			t.Errorf("entry %+v, want the invalid-name refusal", e)
		}
	}
	if e := v.Errors[0]; e.Line == nil {
		t.Errorf("first entry %+v has no location", e)
	}
}
