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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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

// strictRequest serves one request the way bex-api does behind the Render
// request gate: strict JSON decoding, response checked against Render's
// schema.
func strictRequest(t *testing.T, svc *Service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(core.WithStrictJSONDecoding(req.Context()))
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, req)
	return rec
}

// strictCreate serves one POST /v1/services behind the Render request gate.
func strictCreate(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	return strictRequest(t, svc, http.MethodPost, "/v1/services", body)
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

// terraformImageWorker is the image background worker create Render's
// Terraform provider sent in its own recording, which Render answered 201 with
// `imagePath: "docker.io/library/nginx:latest"`
// (internal/provider/backgroundworker/resource/testdata/env_var_cassette.yaml
// at 06af456). The provider reads image_url and the tag out of that form only.
const terraformImageWorker = `{"envVars":null,"image":{"imagePath":"nginx","ownerId":""},"name":"env-var-test","ownerId":"some-owner-id","rootDir":"","secretFiles":null,"serviceDetails":{"envSpecificDetails":{},"numInstances":1,"plan":"starter","previews":{},"pullRequestPreviewsEnabled":"no","region":"oregon","runtime":"image"},"type":"background_worker"}`

// TestImagePathReadsBackInRendersQualifiedForm pins w9/110: every read
// surface answers imagePath the way Render does, while the App keeps (and
// runs) the image as written.
func TestImagePathReadsBackInRendersQualifiedForm(t *testing.T) {
	digested := "ghcr.io/o/i@sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct{ image, want string }{
		{"nginx", "docker.io/library/nginx:latest"},
		{"docker.io/library/nginx:stable-perl", "docker.io/library/nginx:stable-perl"},
		{digested, digested},
	} {
		t.Run(tc.image, func(t *testing.T) {
			svc, cl := newService(nil)
			body := strings.Replace(terraformImageWorker, `"imagePath":"nginx"`, `"imagePath":"`+tc.image+`"`, 1)
			rec := strictCreate(t, svc, body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
			}
			var created serviceAndDeploy
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if created.Service.ImagePath != tc.want {
				t.Errorf("create response imagePath = %q, want %q", created.Service.ImagePath, tc.want)
			}
			if got := getApp(t, cl, created.Service.Name).Spec.Image; got != tc.image {
				t.Errorf("stored spec.image = %q, want %q as written", got, tc.image)
			}
			id := created.Service.ID

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec = httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+id, nil))
			var read renderService
			if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %v: %s", rec.Code, err, rec.Body.String())
			}
			if read.ImagePath != tc.want {
				t.Errorf("REST GET imagePath = %q, want %q", read.ImagePath, tc.want)
			}

			data := gqlDo(t, gqlSchema(t, svc), `{ service(id: "`+id+`") { imagePath } }`)
			if got := data["service"].(map[string]any)["imagePath"]; got != tc.want {
				t.Errorf("GraphQL imagePath = %v, want %q", got, tc.want)
			}

			res := callMCPTool(t, svc, "get_service", map[string]any{"serviceId": id})
			var mcpRead renderService
			if err := json.Unmarshal([]byte(mcpResultText(res)), &mcpRead); err != nil || res.IsError {
				t.Fatalf("MCP get_service: %v: %s", err, mcpResultText(res))
			}
			if mcpRead.ImagePath != tc.want {
				t.Errorf("MCP get_service imagePath = %q, want %q", mcpRead.ImagePath, tc.want)
			}
		})
	}
}

// terraformAutoscaledWorker is the autoscaled worker create Render's Terraform
// provider sent in its own recording (internal/provider/backgroundworker/
// resource/testdata/scaling_cassette.yaml): CPU on at 60, memory off at 70.
// Render answered 201 with both percentages, and its PUT …/autoscaling of the
// same criteria echoed memory 70 too.
const terraformAutoscaledWorker = `{"envVars":null,"image":{"imagePath":"nginx","ownerId":""},"name":"autoscaling-test","ownerId":"some-owner-id","rootDir":"","secretFiles":null,"serviceDetails":{"autoscaling":{"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":70}},"enabled":true,"max":3,"min":1},"envSpecificDetails":{},"numInstances":1,"plan":"starter","previews":{},"pullRequestPreviewsEnabled":"no","region":"oregon","runtime":"image"},"type":"background_worker"}`

// detailsAutoscaling is serviceDetails.autoscaling of a Render service read.
func detailsAutoscaling(t *testing.T, raw []byte) renderAutoscalingConfig {
	t.Helper()
	var svc struct {
		Service        *json.RawMessage `json:"service"`
		ServiceDetails struct {
			Autoscaling *renderAutoscalingConfig `json:"autoscaling"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(raw, &svc); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if svc.Service != nil { // the create's {service, deployId} envelope
		return detailsAutoscaling(t, *svc.Service)
	}
	if svc.ServiceDetails.Autoscaling == nil {
		t.Fatalf("no serviceDetails.autoscaling in %s", raw)
	}
	return *svc.ServiceDetails.Autoscaling
}

// TestDisabledAutoscalingCriterionKeepsItsPercentage pins w9/111: a criterion
// sent turned off keeps its percentage on every read surface, as Render's
// does, while scaling still reads only the criteria that are on.
func TestDisabledAutoscalingCriterionKeepsItsPercentage(t *testing.T) {
	svc, cl := newService(nil)
	rec := strictCreate(t, svc, terraformAutoscaledWorker)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	want := renderAutoscalingConfig{Enabled: true, Min: 1, Max: 3, Criteria: renderAutoscalingCriteria{
		CPU:    renderAutoscalingCriterion{Enabled: true, Percentage: 60},
		Memory: renderAutoscalingCriterion{Enabled: false, Percentage: 70},
	}}
	if got := detailsAutoscaling(t, rec.Body.Bytes()); got != want {
		t.Errorf("create response autoscaling = %+v, want %+v", got, want)
	}
	var created serviceAndDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, name := created.Service.ID, created.Service.Name

	// Scaling reads the targets alone: memory stays off.
	as := getApp(t, cl, name).Spec.Autoscaling
	if as == nil || as.TargetMemoryPercent != nil || as.DisabledMemoryPercent == nil || *as.DisabledMemoryPercent != 70 ||
		as.TargetCPUPercent == nil || *as.TargetCPUPercent != 60 || as.DisabledCPUPercent != nil {
		t.Fatalf("spec.autoscaling = %+v, want cpu target 60 and memory off, kept at 70", as)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(core.WithStrictJSONDecoding(req.Context()))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
		}
		return rec
	}
	if got := detailsAutoscaling(t, do(http.MethodGet, "/v1/services/"+id, "").Body.Bytes()); got != want {
		t.Errorf("REST GET serviceDetails.autoscaling = %+v, want %+v", got, want)
	}
	readConfig := func(rec *httptest.ResponseRecorder) renderAutoscalingConfig {
		t.Helper()
		var got renderAutoscalingConfig
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := readConfig(do(http.MethodGet, "/v1/services/"+id+"/autoscaling", "")); got != want {
		t.Errorf("REST GET …/autoscaling = %+v, want %+v", got, want)
	}

	// The provider's recorded PUT, which Render echoed with memory 70.
	want.Min = 2
	put := `{"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":70}},"enabled":true,"max":3,"min":2}`
	if got := readConfig(do(http.MethodPut, "/v1/services/"+id+"/autoscaling", put)); got != want {
		t.Errorf("PUT …/autoscaling = %+v, want %+v", got, want)
	}

	data := gqlDo(t, gqlSchema(t, svc), `{ service(id: "`+id+`") { autoscaling { targetMemoryPercent disabledMemoryPercent disabledCPUPercent } } }`)
	gql := data["service"].(map[string]any)["autoscaling"].(map[string]any)
	if gql["targetMemoryPercent"] != nil || gql["disabledMemoryPercent"] != 70 || gql["disabledCPUPercent"] != nil {
		t.Errorf("GraphQL autoscaling = %v, want memory off, kept at 70", gql)
	}

	res := callMCPTool(t, svc, "get_service", map[string]any{"serviceId": id})
	if res.IsError {
		t.Fatalf("MCP get_service: %s", mcpResultText(res))
	}
	if got := detailsAutoscaling(t, []byte(mcpResultText(res))); got != want {
		t.Errorf("MCP get_service autoscaling = %+v, want %+v", got, want)
	}
	res = callMCPTool(t, svc, "get_autoscaling", map[string]any{"serviceId": id})
	var mcpView AutoscalingView
	if err := json.Unmarshal([]byte(mcpResultText(res)), &mcpView); err != nil || res.IsError {
		t.Fatalf("MCP get_autoscaling: %v: %s", err, mcpResultText(res))
	}
	if mcpView.DisabledMemoryPercent == nil || *mcpView.DisabledMemoryPercent != 70 {
		t.Errorf("MCP get_autoscaling = %+v, want disabledMemoryPercent 70", mcpView)
	}

	// GraphQL and MCP cannot name the kept percentage, so their writes keep
	// it; Render's body names it, so 0 clears it.
	gqlDo(t, gqlSchema(t, svc), `mutation { setAutoscaling(id: "`+id+`", minInstances: 1, maxInstances: 2, targetCPUPercent: 50) { enabled } }`)
	res = callMCPTool(t, svc, "update_service", map[string]any{"serviceId": id,
		"autoscaling": map[string]any{"minInstances": 1, "maxInstances": 3, "targetCPUPercent": 55}})
	if res.IsError {
		t.Fatalf("MCP update_service: %s", mcpResultText(res))
	}
	if got := readConfig(do(http.MethodGet, "/v1/services/"+id+"/autoscaling", "")); got.Criteria.Memory != want.Criteria.Memory || got.Criteria.CPU.Percentage != 55 {
		t.Errorf("after GraphQL and MCP writes = %+v, want cpu 55 and memory still off at 70", got)
	}
	cleared := strings.Replace(put, `"enabled":false,"percentage":70`, `"enabled":false,"percentage":0`, 1)
	if got := readConfig(do(http.MethodPut, "/v1/services/"+id+"/autoscaling", cleared)); got.Criteria.Memory != (renderAutoscalingCriterion{}) {
		t.Errorf("PUT memory {false, 0} = %+v, want memory cleared", got.Criteria.Memory)
	}
	if as := getApp(t, cl, name).Spec.Autoscaling; as.DisabledMemoryPercent != nil {
		t.Errorf("spec.autoscaling.disabledMemoryPercent = %v after a PUT of 0, want unset", *as.DisabledMemoryPercent)
	}
}

// TestDisabledAutoscalingPercentageRules pins the rest of w9/111: a kept
// percentage is 0–100, never sits beside its criterion's target, survives a
// Blueprint's scaling block, and changes nothing about how the service scales.
func TestDisabledAutoscalingPercentageRules(t *testing.T) {
	put := func(memory string) *httptest.ResponseRecorder {
		svc, _ := newService(nil, paidWebApp("web"))
		return putAutoscaling(t, svc, `{"enabled":true,"min":1,"max":1,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":`+memory+`}}`)
	}
	for _, percentage := range []string{"-1", "101"} {
		if rec := put(`{"enabled":false,"percentage":` + percentage + `}`); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "criteria.memory.percentage must be 0–100") {
			t.Errorf("memory off at %s = %d %s, want 400 naming the range", percentage, rec.Code, rec.Body.String())
		}
	}

	// Turning memory on makes its percentage the target; nothing is kept.
	req, _, err := setAutoscalingRequestFromRender(renderAutoscalingConfig{Enabled: true, Min: 1, Max: 2, Criteria: renderAutoscalingCriteria{
		CPU: renderAutoscalingCriterion{Enabled: true, Percentage: 60}, Memory: renderAutoscalingCriterion{Enabled: true, Percentage: 70},
	}})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := autoscalingSpec(req, appv1alpha1.TypeWebService, "starter")
	if err != nil {
		t.Fatal(err)
	}
	if spec.TargetMemoryPercent == nil || *spec.TargetMemoryPercent != 70 || spec.DisabledMemoryPercent != nil || spec.DisabledCPUPercent != nil {
		t.Errorf("memory on = %+v, want target 70 and nothing kept", spec)
	}

	// A Blueprint's scaling block cannot name the kept percentage: it stays.
	kept := int32(70)
	dst := appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Tier: "starter", Autoscaling: &appv1alpha1.AutoscalingSpec{
		Enabled: true, MinReplicas: 1, MaxReplicas: 2, TargetCPUPercent: int32p(60), DisabledMemoryPercent: &kept,
	}}
	want := appv1alpha1.AppSpec{Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 3, TargetCPUPercent: int32p(60)}}
	if _, err := ApplyBlueprintServiceSpec(&dst, want, map[string]BlueprintField{"scaling": {}}); err != nil {
		t.Fatal(err)
	}
	if got := dst.Autoscaling; got.MaxReplicas != 3 || got.DisabledMemoryPercent == nil || *got.DisabledMemoryPercent != 70 {
		t.Errorf("after a Blueprint scaling block = %+v, want max 3 and memory still kept at 70", got)
	}

	// A kept percentage alone scales nothing, so it is no change.
	other := int32(80)
	moved := *dst.Autoscaling
	moved.DisabledMemoryPercent = &other
	if autoscalingChanged(dst.Autoscaling, moved) {
		t.Error("a kept percentage alone counts as an autoscaling change")
	}
	moved.MaxReplicas = 4
	if !autoscalingChanged(dst.Autoscaling, moved) {
		t.Error("a new maximum does not count as an autoscaling change")
	}
}

// terraformScalingUpdate is the update the provider sent right after its
// DELETE …/autoscaling in TestBackgroundWorkerScalingResource's "turn off
// autoscaling" step (scaling_cassette.yaml at 06af456). Render answered 200
// with no serviceDetails.autoscaling, as every later read did. The image
// owner is the test service's (unlabeled) workspace.
const terraformScalingUpdate = `{"image":{"imagePath":"nginx","ownerId":""},"name":"autoscaling-test","rootDir":"","serviceDetails":{"envSpecificDetails":{"dockerCommand":""},"plan":"starter","preDeployCommand":"","previews":{},"pullRequestPreviewsEnabled":"no"}}`

// hasDetailsAutoscaling reports whether a Render service read carries
// serviceDetails.autoscaling at all.
func hasDetailsAutoscaling(t *testing.T, raw []byte) bool {
	t.Helper()
	var svc struct {
		ServiceDetails map[string]json.RawMessage `json:"serviceDetails"`
	}
	if err := json.Unmarshal(raw, &svc); err != nil || svc.ServiceDetails == nil {
		t.Fatalf("no serviceDetails in %s (%v)", raw, err)
	}
	_, present := svc.ServiceDetails["autoscaling"]
	return present
}

// TestTurnedOffAutoscalingLeavesTheServiceRead pins w9/123: once autoscaling
// is turned off, no Render-shaped service read carries
// serviceDetails.autoscaling, as none of Render's does, so the provider reads
// the removed block as null. The settings the disable keeps (w4/597) still
// read on GET …/autoscaling, GraphQL and MCP get_autoscaling, and turning it
// back on brings the key back.
func TestTurnedOffAutoscalingLeavesTheServiceRead(t *testing.T) {
	const policy = `{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":70}}}`
	rest := func(t *testing.T, svc *Service, method, path, body string) []byte {
		t.Helper()
		rec := strictRequest(t, svc, method, path, body)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	for _, off := range []struct {
		name    string
		disable func(t *testing.T, svc *Service, id string)
	}{
		{"DELETE", func(t *testing.T, svc *Service, id string) {
			rest(t, svc, http.MethodDelete, "/v1/services/"+id+"/autoscaling", "")
		}},
		{"PUT enabled false", func(t *testing.T, svc *Service, id string) {
			rest(t, svc, http.MethodPut, "/v1/services/"+id+"/autoscaling", strings.Replace(policy, `"enabled":true,"min"`, `"enabled":false,"min"`, 1))
		}},
		{"GraphQL disableAutoscaling", func(t *testing.T, svc *Service, id string) {
			gqlDo(t, gqlSchema(t, svc), `mutation { disableAutoscaling(id: "`+id+`") }`)
		}},
		{"MCP disable_autoscaling", func(t *testing.T, svc *Service, id string) {
			if res := callMCPTool(t, svc, "disable_autoscaling", map[string]any{"serviceId": id}); res.IsError {
				t.Fatalf("MCP disable_autoscaling: %s", mcpResultText(res))
			}
		}},
	} {
		t.Run(off.name, func(t *testing.T) {
			svc, _ := newService(nil)
			rec := strictCreate(t, svc, terraformAutoscaledWorker)
			var created serviceAndDeploy
			if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || rec.Code != http.StatusCreated {
				t.Fatalf("POST = %d %v: %s", rec.Code, err, rec.Body.String())
			}
			if _, present := created.Service.ServiceDetails["autoscaling"]; !present {
				t.Fatalf("create answer has no serviceDetails.autoscaling: %s", rec.Body.String())
			}
			id := created.Service.ID
			off.disable(t, svc, id)

			if hasDetailsAutoscaling(t, rest(t, svc, http.MethodPatch, "/v1/services/"+id, terraformScalingUpdate)) {
				t.Error("PATCH answer carries serviceDetails.autoscaling after autoscaling was turned off")
			}
			if hasDetailsAutoscaling(t, rest(t, svc, http.MethodGet, "/v1/services/"+id, "")) {
				t.Error("REST GET carries serviceDetails.autoscaling after autoscaling was turned off")
			}
			var list []serviceWithCursor
			if err := json.Unmarshal(rest(t, svc, http.MethodGet, "/v1/services", ""), &list); err != nil || len(list) != 1 {
				t.Fatalf("REST list = %v (%v), want the one service", list, err)
			}
			if _, present := list[0].Service.ServiceDetails["autoscaling"]; present {
				t.Error("REST list carries serviceDetails.autoscaling after autoscaling was turned off")
			}
			res := callMCPTool(t, svc, "get_service", map[string]any{"serviceId": id})
			if res.IsError || hasDetailsAutoscaling(t, []byte(mcpResultText(res))) {
				t.Errorf("MCP get_service = %s, want no serviceDetails.autoscaling", mcpResultText(res))
			}

			// The kept settings, turned off.
			kept := renderAutoscalingConfig{Enabled: false, Min: 1, Max: 3, Criteria: renderAutoscalingCriteria{
				CPU:    renderAutoscalingCriterion{Enabled: true, Percentage: 60},
				Memory: renderAutoscalingCriterion{Enabled: false, Percentage: 70},
			}}
			var got renderAutoscalingConfig
			if err := json.Unmarshal(rest(t, svc, http.MethodGet, "/v1/services/"+id+"/autoscaling", ""), &got); err != nil || got != kept {
				t.Errorf("GET …/autoscaling = %+v (%v), want the kept %+v", got, err, kept)
			}
			data := gqlDo(t, gqlSchema(t, svc), `{ service(id: "`+id+`") { autoscaling { enabled minInstances maxInstances targetCPUPercent } } }`)
			gql := data["service"].(map[string]any)["autoscaling"].(map[string]any)
			if gql["enabled"] != false || gql["minInstances"] != 1 || gql["maxInstances"] != 3 || gql["targetCPUPercent"] != 60 {
				t.Errorf("GraphQL autoscaling = %v, want 1–3 at 60%% CPU, turned off", gql)
			}
			res = callMCPTool(t, svc, "get_autoscaling", map[string]any{"serviceId": id})
			var mcpView AutoscalingView
			if err := json.Unmarshal([]byte(mcpResultText(res)), &mcpView); err != nil || res.IsError ||
				mcpView.Enabled || mcpView.MinInstances != 1 || mcpView.MaxInstances != 3 {
				t.Errorf("MCP get_autoscaling = %s, want 1–3, turned off", mcpResultText(res))
			}

			// Turned back on, the policy is in the service read again.
			rest(t, svc, http.MethodPut, "/v1/services/"+id+"/autoscaling", policy)
			kept.Enabled = true
			if got := detailsAutoscaling(t, rest(t, svc, http.MethodGet, "/v1/services/"+id, "")); got != kept {
				t.Errorf("REST GET serviceDetails.autoscaling after re-enabling = %+v, want %+v", got, kept)
			}
		})
	}
}

// terraformImageUpdate is the update the provider sent in
// TestWebServiceResource_Image's second step, which Render answered 200
// (internal/provider/webservice/resource/testdata/web_service_image_cassette.yaml
// at 06af456): every update carries the service's rootDir, "" for an image
// service. The image owner is the test service's (unlabeled) workspace.
const terraformImageUpdate = `{"image":{"imagePath":"docker.io/library/nginx:stable-perl","ownerId":""},"name":"web-service-image-tf","rootDir":"","serviceDetails":{"envSpecificDetails":{"dockerCommand":"echo hello"},"healthCheckPath":"","maintenanceMode":{"enabled":false,"uri":""},"plan":"starter","preDeployCommand":"","previews":{},"pullRequestPreviewsEnabled":"no","runtime":"image"}}`

// terraformRepoToImageUpdate is TestWebServiceResource_RuntimeUpdate's last
// step (runtime_update_cassette.yaml): a Docker service from a repo becomes
// an image service, still with rootDir "".
const terraformRepoToImageUpdate = `{"image":{"imagePath":"docker.io/library/nginx:latest","ownerId":""},"name":"web-service-runtime-tf","rootDir":"","serviceDetails":{"envSpecificDetails":{"dockerCommand":""},"healthCheckPath":"","maintenanceMode":{"enabled":false,"uri":""},"plan":"starter","preDeployCommand":"","previews":{},"pullRequestPreviewsEnabled":"no","runtime":"image"}}`

// TestPatchKeepsTheServicesOwnRootDir pins w9/113: a rootDir equal to the
// service's own is no change, so an image service's "" is not refused for
// having no repo, while a different one still is, by name.
func TestPatchKeepsTheServicesOwnRootDir(t *testing.T) {
	patch := func(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
		t.Helper()
		return strictRequest(t, svc, http.MethodPatch, "/v1/services/web", body)
	}
	t.Run("image service update", func(t *testing.T) {
		svc, cl := newService(nil, paidWebApp("web"))
		rec := patch(t, svc, terraformImageUpdate)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body.String())
		}
		var out renderService
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.ImagePath != "docker.io/library/nginx:stable-perl" || out.RootDir != "" {
			t.Errorf("answer imagePath %q rootDir %q, want the new image and rootDir \"\"", out.ImagePath, out.RootDir)
		}
		if a := getApp(t, cl, "web"); a.Spec.Image != "docker.io/library/nginx:stable-perl" || a.Spec.StartCommand != "echo hello" || a.Spec.RootDir != "" {
			t.Errorf("spec image %q startCommand %q rootDir %q, want the update applied", a.Spec.Image, a.Spec.StartCommand, a.Spec.RootDir)
		}
	})

	t.Run("repo service becomes an image service", func(t *testing.T) {
		a := repoApp("web", "https://github.com/render-examples/bun-docker", "main")
		a.Spec.Type, a.Spec.Tier, a.Spec.Runtime = appv1alpha1.TypeWebService, "starter", "docker"
		svc, cl := newService(nil, a)
		if rec := patch(t, svc, terraformRepoToImageUpdate); rec.Code != http.StatusOK {
			t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body.String())
		}
		if got := getApp(t, cl, "web").Spec; got.Repo != "" || got.Image != "docker.io/library/nginx:latest" {
			t.Errorf("spec repo %q image %q, want the image service", got.Repo, got.Image)
		}
	})

	t.Run("another root directory on an image service", func(t *testing.T) {
		svc, cl := newService(nil, paidWebApp("web"))
		body := strings.Replace(terraformImageUpdate, `"rootDir":""`, `"rootDir":"api"`, 1)
		rec := patch(t, svc, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "root directory only applies to build-from-git") {
			t.Fatalf("PATCH rootDir api = %d %s, want 400 naming the root directory", rec.Code, rec.Body.String())
		}
		if a := getApp(t, cl, "web"); a.Spec.Image != "web:v1" || a.Spec.RootDir != "" {
			t.Errorf("a refused patch wrote image %q rootDir %q", a.Spec.Image, a.Spec.RootDir)
		}
	})

	t.Run("a repo service's own root directory starts no build", func(t *testing.T) {
		a := repoApp("web", "https://github.com/x/mono", "main")
		a.Spec.RootDir = "services/api"
		svc, cl := newService(nil, a)
		for _, rootDir := range []string{"services/api", "./services/api/"} {
			if rec := patch(t, svc, `{"rootDir":"`+rootDir+`"}`); rec.Code != http.StatusOK {
				t.Fatalf("PATCH rootDir %q = %d: %s", rootDir, rec.Code, rec.Body.String())
			}
		}
		if got := getApp(t, cl, "web").Spec; got.RootDir != "services/api" || got.RestartedAt != "" {
			t.Errorf("spec rootDir %q restartedAt %q, want unchanged and no fresh build", got.RootDir, got.RestartedAt)
		}
	})

	t.Run("GraphQL and MCP", func(t *testing.T) {
		svc, cl := newService(nil, paidWebApp("web"))
		data := gqlDo(t, gqlSchema(t, svc), `mutation { setRootDir(id: "web", rootDir: "") { rootDir } }`)
		if got := data["setRootDir"].(map[string]any)["rootDir"]; got != "" {
			t.Errorf("GraphQL setRootDir rootDir = %v, want \"\"", got)
		}
		if res := callMCPTool(t, svc, "update_service", map[string]any{"serviceId": "web", "rootDir": ""}); res.IsError {
			t.Errorf("MCP update_service rootDir \"\": %s", mcpResultText(res))
		}
		res := callMCPTool(t, svc, "update_service", map[string]any{"serviceId": "web", "rootDir": "api"})
		if !res.IsError || !strings.Contains(mcpResultText(res), "root directory only applies to build-from-git") {
			t.Errorf("MCP update_service rootDir api = %s, want the refusal", mcpResultText(res))
		}
		if a := getApp(t, cl, "web"); a.Spec.RootDir != "" || a.Spec.RestartedAt != "" {
			t.Errorf("spec rootDir %q restartedAt %q, want untouched", a.Spec.RootDir, a.Spec.RestartedAt)
		}
	})
}
