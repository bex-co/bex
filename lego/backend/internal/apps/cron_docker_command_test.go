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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m165. The pinned CLI's cron builder emits the NATIVE
// envSpecificDetails.startCommand shape for every runtime — it has no docker
// branch — so a docker cron's command used to be discarded on create and
// projected as an empty dockerCommand on read. Both halves are pinned here: a
// cron that silently runs its image entrypoint instead of the command it was
// given is worse than one that refuses.

// cronCommandFromRead pulls the docker projection's dockerCommand out of a
// service read, which is exactly where the pinned client looks for it
// (pkg/service/clone.go startCommandFromEnvDetails).
func cronCommandFromRead(t *testing.T, body []byte) (dockerCommand string, topLevel string) {
	t.Helper()
	// A create answers Render's {service, deployId} envelope; a get/patch
	// answers the bare service. Accept either so one helper reads both.
	type serviceShape struct {
		ServiceDetails struct {
			Command            string `json:"command"`
			EnvSpecificDetails struct {
				DockerCommand string `json:"dockerCommand"`
				StartCommand  string `json:"startCommand"`
			} `json:"envSpecificDetails"`
		} `json:"serviceDetails"`
	}
	var read struct {
		serviceShape
		Service *serviceShape `json:"service"`
	}
	if err := json.Unmarshal(body, &read); err != nil {
		t.Fatalf("decode service read: %v (%s)", err, body)
	}
	svc := read.serviceShape
	if read.Service != nil {
		svc = *read.Service
	}
	return svc.ServiceDetails.EnvSpecificDetails.DockerCommand, svc.ServiceDetails.Command
}

// nativeStartCommandFromRead is the same unwrap for the native projection.
func nativeStartCommandFromRead(t *testing.T, body []byte) string {
	t.Helper()
	type serviceShape struct {
		ServiceDetails struct {
			EnvSpecificDetails struct {
				StartCommand string `json:"startCommand"`
			} `json:"envSpecificDetails"`
		} `json:"serviceDetails"`
	}
	var read struct {
		serviceShape
		Service *serviceShape `json:"service"`
	}
	if err := json.Unmarshal(body, &read); err != nil {
		t.Fatalf("decode service read: %v (%s)", err, body)
	}
	if read.Service != nil {
		return read.Service.ServiceDetails.EnvSpecificDetails.StartCommand
	}
	return read.ServiceDetails.EnvSpecificDetails.StartCommand
}

func TestRESTCreateKeepsDockerCronCommandFromNativeSpelling(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Byte-for-byte the shape the pinned CLI builds for
	// `--type cron_job --runtime docker --cron-command 'echo qa'`.
	body := `{"name":"nightly","type":"cron_job","repo":"https://github.com/bex-co/example","serviceDetails":{"runtime":"docker","plan":"free",` +
		`"schedule":"*/10 * * * *","envSpecificDetails":{"buildCommand":"","startCommand":"echo qa"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST docker cron = %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, "nightly").Spec.Command; got != "echo qa" {
		t.Errorf("stored cron command = %q, want %q (the docker branch discarded it)", got, "echo qa")
	}
	if dockerCommand, _ := cronCommandFromRead(t, rec.Body.Bytes()); dockerCommand != "echo qa" {
		t.Errorf("create response dockerCommand = %q, want %q", dockerCommand, "echo qa")
	}
}

func TestRESTCreateDockerCronPrefersDockerCommandWhenBothSent(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"nightly","type":"cron_job","repo":"https://github.com/bex-co/example","serviceDetails":{"runtime":"docker","plan":"free",` +
		`"schedule":"*/10 * * * *","envSpecificDetails":{"startCommand":"native","dockerCommand":"docker"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST docker cron = %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, "nightly").Spec.Command; got != "docker" {
		t.Errorf("stored cron command = %q, want the docker spelling to win", got)
	}
}

// A docker WEB service keeps the exact prior rule: its command is the docker
// spelling, and a stray native startCommand is not promoted into one.
func TestRESTCreateDockerWebServiceIgnoresNativeStartCommand(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","type":"web_service","repo":"https://github.com/bex-co/example","serviceDetails":{"runtime":"docker","plan":"free",` +
		`"envSpecificDetails":{"startCommand":"echo native"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST docker web = %d: %s", rec.Code, rec.Body.String())
	}
	app := getApp(t, cl, "web")
	if app.Spec.StartCommand != "" || app.Spec.Command != "" {
		t.Errorf("docker web service commands = start %q / command %q, want both empty (unchanged behavior)",
			app.Spec.StartCommand, app.Spec.Command)
	}
}

// The read half, independent of how the command got there: a stored docker
// cron command must be visible where the client looks for it, so
// `services update --cron-command` cannot report a no-op it did not perform
// and `create --from` cannot clone an empty command.
func TestReadProjectsStoredDockerCronCommand(t *testing.T) {
	app := sampleApp("nightly")
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Runtime = "docker"
	app.Spec.Schedule = "*/10 * * * *"
	app.Spec.Command = "echo stored"
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/nightly", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET docker cron = %d: %s", rec.Code, rec.Body.String())
	}
	dockerCommand, topLevel := cronCommandFromRead(t, rec.Body.Bytes())
	if dockerCommand != "echo stored" {
		t.Errorf("read dockerCommand = %q, want %q (the projection read StartCommand)", dockerCommand, "echo stored")
	}
	if topLevel != "echo stored" {
		t.Errorf("read serviceDetails.command = %q, want %q", topLevel, "echo stored")
	}
}

// The update round trip end to end, which is the journey the sweep saw lie:
// PATCH the docker spelling, read it back in the same place.
func TestUpdateDockerCronCommandRoundTripsThroughTheRead(t *testing.T) {
	app := sampleApp("nightly")
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Runtime = "docker"
	app.Spec.Schedule = "*/10 * * * *"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	patch := `{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"echo patched"}}}`
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/nightly", strings.NewReader(patch)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH docker cron command = %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, "nightly").Spec.Command; got != "echo patched" {
		t.Fatalf("stored cron command after PATCH = %q", got)
	}
	if dockerCommand, _ := cronCommandFromRead(t, rec.Body.Bytes()); dockerCommand != "echo patched" {
		t.Errorf("PATCH response dockerCommand = %q, want %q — the CLI prints this value", dockerCommand, "echo patched")
	}
}

// A NATIVE cron is the control that was already green and must stay green.
func TestNativeCronCommandUnchanged(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"ncron","type":"cron_job","repo":"https://github.com/bex-co/example","serviceDetails":{"runtime":"go","plan":"free",` +
		`"schedule":"*/10 * * * *","envSpecificDetails":{"buildCommand":"go build","startCommand":"echo native"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST native cron = %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, "ncron").Spec.Command; got != "echo native" {
		t.Errorf("native cron command = %q, want %q", got, "echo native")
	}
	if got := nativeStartCommandFromRead(t, rec.Body.Bytes()); got != "echo native" {
		t.Errorf("native cron read startCommand = %q, want %q", got, "echo native")
	}
}

// MCP and GraphQL must report the same value as REST — one stored fact, one
// reading, on every surface (t005).
func TestDockerCronCommandAgreesAcrossRESTAndGraphQL(t *testing.T) {
	app := sampleApp("nightly")
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Runtime = "docker"
	app.Spec.Schedule = "*/10 * * * *"
	app.Spec.Command = "echo stored"
	svc, _ := newService(nil, app)

	view, err := svc.Get(context.Background(), "nightly")
	if err != nil {
		t.Fatal(err)
	}
	if view.Command != "echo stored" {
		t.Fatalf("service view command = %q", view.Command)
	}
	details, ok := envSpecificDetails(view, appv1alpha1.TypeCronJob)
	if !ok {
		t.Fatal("docker cron has no envSpecificDetails")
	}
	if got := details["dockerCommand"]; got != "echo stored" {
		t.Errorf("projection dockerCommand = %v, want %q", got, "echo stored")
	}
}

// The pinned client resolves a cron's command through
// AsNativeEnvironmentDetails().StartCommand for EVERY runtime — its clone path
// has no docker branch, exactly as its create path has none — so a docker cron
// must carry the command in both spellings or `create --from` clones an empty
// one (w9/m165, caught live on dev-9 after the first two fixes).
func TestDockerCronProjectsCommandInBothSpellings(t *testing.T) {
	app := sampleApp("nightly")
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Runtime = "docker"
	app.Spec.Schedule = "*/10 * * * *"
	app.Spec.Command = "echo stored"
	svc, _ := newService(nil, app)
	view, err := svc.Get(context.Background(), "nightly")
	if err != nil {
		t.Fatal(err)
	}
	details, ok := envSpecificDetails(view, appv1alpha1.TypeCronJob)
	if !ok {
		t.Fatal("docker cron has no envSpecificDetails")
	}
	if got := details["dockerCommand"]; got != "echo stored" {
		t.Errorf("dockerCommand = %v, want %q", got, "echo stored")
	}
	if got := details["startCommand"]; got != "echo stored" {
		t.Errorf("startCommand = %v, want %q — the pinned client's clone reads only this one", got, "echo stored")
	}
}

// A docker WEB service keeps the plain docker shape: the extra spelling exists
// only because a cron's command is runtime-independent on the wire.
func TestDockerWebServiceProjectionHasNoStartCommand(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Runtime = "docker"
	app.Spec.StartCommand = "./server"
	svc, _ := newService(nil, app)
	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	details, ok := envSpecificDetails(view, appv1alpha1.TypeWebService)
	if !ok {
		t.Fatal("docker web service has no envSpecificDetails")
	}
	if got := details["dockerCommand"]; got != "./server" {
		t.Errorf("dockerCommand = %v, want %q", got, "./server")
	}
	if _, present := details["startCommand"]; present {
		t.Errorf("docker web projection gained a startCommand key: %#v", details)
	}
}

// w9/068. The generalized guard: whatever a service READS BACK must be
// re-sendable as a create. The defect it replaces was one plan-gated field
// (`maintenanceMode` was refused on a free plan merely for being PRESENT, even
// disabled — which is exactly what every read emits, since Render's schema
// requires it on webServiceDetails), so `create --from <free web service>` was
// impossible. Asserting the whole read shape rather than that one field is what
// stops the next plan-gated field from reintroducing the class: w9/done/m93's
// readback guard covers build-strategy shapes and does not reach this one.
func TestFreeWebServiceReadShapeIsReSendableAsCreate(t *testing.T) {
	app := sampleApp("freeweb")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Tier = "free"
	app.Spec.Runtime = "image"
	app.Spec.Replicas = 1 // the real shape of a free service; sampleApp defaults to 2
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/freeweb", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET free web service = %d: %s", rec.Code, rec.Body.String())
	}
	var read map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	details, ok := read["serviceDetails"].(map[string]any)
	if !ok {
		t.Fatalf("read has no serviceDetails: %s", rec.Body.Bytes())
	}
	if _, present := details["maintenanceMode"]; !present {
		t.Fatalf("free web service read omits maintenanceMode; w4/125 requires it present: %v", details)
	}

	// Echo the read's serviceDetails back, exactly as `create --from` does.
	clone := map[string]any{
		"name":           "freeweb-clone",
		"type":           read["type"],
		"image":          map[string]any{"imagePath": "nginx:alpine", "ownerId": ""},
		"serviceDetails": details,
		"dryRun":         true,
	}
	body, err := json.Marshal(clone)
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("echoing a free web service's own read back as a create = %d: %s", rec.Code, rec.Body.String())
	}
}

// Enabling maintenance mode on a free plan is still refused, by the same named
// error — only the inert value stopped being a refusal.
func TestEnablingMaintenanceModeOnFreePlanStillRefused(t *testing.T) {
	app := sampleApp("freeweb")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Tier = "free"
	svc, _ := newService(nil, app)

	if err := validateMaintenanceEligibility(appv1alpha1.TypeWebService, "free",
		&MaintenanceModeView{Enabled: false}); err != nil {
		t.Errorf("disabled maintenanceMode on free = %v, want accepted", err)
	}
	err := validateMaintenanceEligibility(appv1alpha1.TypeWebService, "free",
		&MaintenanceModeView{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "paid web service plan") {
		t.Errorf("enabling maintenanceMode on free = %v, want the paid-plan refusal", err)
	}
	// The sibling branch has the same shape: a disabled mode on a non-web type
	// is inert (GraphQL returns it for every type, w4/125), enabling is not.
	if err := validateMaintenanceEligibility(appv1alpha1.TypeCronJob, "starter",
		&MaintenanceModeView{Enabled: false}); err != nil {
		t.Errorf("disabled maintenanceMode on a cron = %v, want accepted", err)
	}
	if err := validateMaintenanceEligibility(appv1alpha1.TypeCronJob, "starter",
		&MaintenanceModeView{Enabled: true}); err == nil {
		t.Error("enabling maintenanceMode on a cron was accepted")
	}
	_ = svc
}

// w9/069 (found while verifying w9/068 live): the same "refuses what it
// emitted" class, one field over. A prebuilt-image service read back
// branch:"main" — the CRD's default, meaningless without a repo — and bex's own
// create refuses a branch on an image service (the deliberate
// prebuiltImageSourceFields policy), so `create --from <image service>` could
// never succeed. The read is what was wrong: AppView.Branch already documents
// itself as "empty for an image-backed App".
func TestImageBackedServiceReadsNoBranch(t *testing.T) {
	image := sampleApp("imgweb")
	image.Spec.Type = appv1alpha1.TypeWebService
	image.Spec.Image = "nginx:alpine"
	image.Spec.Branch = "main" // the CRD default, present even with no repo
	repo := sampleApp("repoweb")
	repo.Spec.Type = appv1alpha1.TypeWebService
	repo.Spec.Repo = "https://github.com/bex-co/example"
	repo.Spec.Branch = "release"
	svc, _ := newService(nil, image, repo)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, tc := range []struct{ name, want string }{
		{"imgweb", ""},
		{"repoweb", "release"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+tc.name, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", tc.name, rec.Code, rec.Body.String())
		}
		var read struct {
			Branch string `json:"branch"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil {
			t.Fatal(err)
		}
		if read.Branch != tc.want {
			t.Errorf("%s read branch = %q, want %q", tc.name, read.Branch, tc.want)
		}
	}
}
