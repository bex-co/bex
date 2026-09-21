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
