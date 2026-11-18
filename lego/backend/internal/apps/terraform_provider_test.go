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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// terraformDockerCron is the Docker cron job create Render's Terraform
// provider sent in its own recording, which Render answered 201
// (render-oss/terraform-provider-render 06af456,
// internal/provider/cronjob/resource/testdata/docker_cassette.yaml). The
// provider builds envSpecificDetails from Render's response type, so it
// carries a zero-valued registryCredential object when none is configured.
const terraformDockerCron = `{"autoDeployTrigger":"commit","branch":"main","buildFilter":{"ignoredPaths":null,"paths":null},"envVars":null,"name":"some-name","ownerId":"some-owner-id","repo":"https://github.com/render-examples/bun-docker","secretFiles":null,"serviceDetails":{"envSpecificDetails":{"dockerCommand":"","dockerContext":".","dockerfilePath":"./Dockerfile","registryCredential":{"id":"","name":"","registry":"","updatedAt":"0001-01-01T00:00:00Z","username":""}},"plan":"starter","region":"oregon","runtime":"docker","schedule":"0 0 * * *"},"type":"cron_job"}`

// terraformImageCron is the provider's image cron create (create.go's image
// case): the credential id travels on image and on the registryCredential
// object alike.
const terraformImageCron = `{"envVars":null,"image":{"imagePath":"ghcr.io/acme/private:1","ownerId":"","registryCredentialId":"rgc-one"},"name":"nightly","ownerId":"some-owner-id","secretFiles":null,"serviceDetails":{"envSpecificDetails":{"dockerCommand":"","dockerContext":"","dockerfilePath":"","registryCredential":{"id":"rgc-one","name":"","registry":"","updatedAt":"0001-01-01T00:00:00Z","username":""}},"plan":"starter","region":"oregon","runtime":"image","schedule":"0 0 * * *"},"type":"cron_job"}`

// strictCreate serves one POST /v1/services the way bex-api does behind the
// Render request gate: strict JSON decoding, response checked against
// Render's schema.
func strictCreate(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body))
	req = req.WithContext(core.WithStrictJSONDecoding(req.Context()))
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, req)
	return rec
}

func TestRESTCreateAcceptsTerraformRegistryCredentialObject(t *testing.T) {
	withID := strings.Replace(terraformDockerCron, `"registryCredential":{"id":""`, `"registryCredential":{"id":"rgc-one"`, 1)
	for _, tc := range []struct {
		name, body, want string
	}{
		{"docker cron with no credential", terraformDockerCron, ""},
		{"docker cron naming a credential", withID, "rgc-one"},
		{"image cron naming a credential", terraformImageCron, "rgc-one"},
		{"object id matching registryCredentialId", strings.Replace(withID, `"dockerCommand":""`, `"registryCredentialId":"rgc-one","dockerCommand":""`, 1), "rgc-one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := &fakePullSecrets{name: "cron-registry-pull", ok: true}
			svc, cl := rcService(rc)
			rec := strictCreate(t, svc, tc.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
			}
			var created serviceAndDeploy
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if created.Service.RegistryCredentialID != tc.want {
				t.Errorf("response registryCredentialId = %q, want %q", created.Service.RegistryCredentialID, tc.want)
			}
			got := getApp(t, cl, created.Service.Name)
			switch {
			case tc.want == "" && (got.Spec.RegistryCredentialID != nil || rc.calls != 0):
				t.Errorf("an empty registryCredential.id bound %v (materializations %d), want none", got.Spec.RegistryCredentialID, rc.calls)
			case tc.want != "" && (got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != tc.want || got.Spec.ExternalRegistryPullSecret != "cron-registry-pull"):
				t.Errorf("spec binding = %v pull secret %q, want %s", got.Spec.RegistryCredentialID, got.Spec.ExternalRegistryPullSecret, tc.want)
			}
		})
	}
}

func TestRESTCreateRefusesConflictingTerraformRegistryCredential(t *testing.T) {
	withID := strings.Replace(terraformDockerCron, `"registryCredential":{"id":""`, `"registryCredential":{"id":"rgc-one"`, 1)
	for _, tc := range []struct {
		name, body, want string
	}{
		{"object id against registryCredentialId",
			strings.Replace(withID, `"dockerCommand":""`, `"registryCredentialId":"rgc-two","dockerCommand":""`, 1),
			"registryCredential.id conflicts with registryCredentialId"},
		{"object id against image.registryCredentialId",
			strings.Replace(terraformImageCron, `"registryCredentialId":"rgc-one"`, `"registryCredentialId":"rgc-two"`, 1),
			"conflicting values"},
		{"undeclared object key", strings.Replace(terraformDockerCron, `"username":""`, `"username":"","password":"x"`, 1),
			"password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := &fakePullSecrets{name: "cron-registry-pull", ok: true}
			svc, _ := rcService(rc)
			rec := strictCreate(t, svc, tc.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("POST = %d: %s, want 400 naming %q", rec.Code, rec.Body.String(), tc.want)
			}
			if rc.calls != 0 {
				t.Errorf("a refused create materialized %d pull secrets", rc.calls)
			}
		})
	}
}
