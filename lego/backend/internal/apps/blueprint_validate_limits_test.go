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
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// w8/026 (A): the handler's own 10 MiB file cap answers an oversized file with
// its own 413 (it used to be flattened into a bare 400), on both the CLI's
// multipart contract and the JSON one.
func TestValidateBlueprintSizeRefusalIsItsOwn413(t *testing.T) {
	svc, _ := connectionService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	huge := "# " + strings.Repeat("x", maxBlueprintValidationFileBytes) + "\nservices: []\n"

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	_ = mw.WriteField("ownerId", connOwner)
	part, _ := mw.CreateFormFile("file", "render.yaml")
	_, _ = part.Write([]byte(huge))
	_ = mw.Close()
	multipartReq := httptest.NewRequest(http.MethodPost, BlueprintValidationPath, &form)
	multipartReq.Header.Set("Content-Type", mw.FormDataContentType())

	jsonBody, _ := json.Marshal(map[string]string{"bexYaml": huge, "ownerId": connOwner})
	jsonReq := httptest.NewRequest(http.MethodPost, BlueprintValidationPath, bytes.NewReader(jsonBody))
	jsonReq.Header.Set("Content-Type", "application/json")

	for name, req := range map[string]*http.Request{"multipart": multipartReq, "json": jsonReq} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req.WithContext(ownershipCtx()))
		if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "10 MiB validation limit") {
			t.Errorf("%s: %d %s, want 413 naming the 10 MiB limit", name, rec.Code, rec.Body.String())
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
