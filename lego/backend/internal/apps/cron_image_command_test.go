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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m167. The pinned client clones a cron's command only from
// serviceDetails.envSpecificDetails.startCommand, and an image-runtime service
// projected no envSpecificDetails at all — so `services create --from` cloned
// an image cron with no command, and every run silently executed the image
// entrypoint and reported success.

// imageCronCloneSource is the read the pinned client clones from, shared with
// lego/cli/image_cron_clone_contract_test.go, which drives the PINNED clone
// path (decode → ServiceFromAPI → normalize → BuildCreateRequest) against it.
// Regenerate with BEX_UPDATE_GOLDEN=1 when the read shape intentionally moves.
var imageCronCloneSource = filepath.Join("..", "..", "..", "cli", "testdata", "image-cron-clone-source.json")

func imageCron(name, command string) *appv1alpha1.App {
	app := sampleApp(name)
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Image = "docker.io/library/busybox:1.37"
	app.Spec.Schedule = "*/2 * * * *"
	app.Spec.Command = command
	app.Spec.Replicas = 1
	return app
}

func getServiceBody(t *testing.T, app *appv1alpha1.App) []byte {
	t.Helper()
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/"+app.Name, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", app.Name, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

// withoutClock drops the fields computed from the wall clock, so the golden
// stays stable.
func withoutClock(t *testing.T, raw []byte) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "nextRunAt")
	if details, ok := m["serviceDetails"].(map[string]any); ok {
		delete(details, "nextRunAt")
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestImageCronReadIsTheCloneContract(t *testing.T) {
	body := withoutClock(t, getServiceBody(t, imageCron("qa-image-cron", "echo QA_CLONE_MARKER")))
	if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(imageCronCloneSource, append(pretty.Bytes(), '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(imageCronCloneSource)
	if err != nil {
		t.Fatalf("read the shared golden: %v", err)
	}
	var got, want any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the image cron read drifted from the clone contract the pinned CLI is tested against\n got: %s\nwant: %s", body, raw)
	}
	if start := nativeStartCommandFromRead(t, body); start != "echo QA_CLONE_MARKER" {
		t.Fatalf("envSpecificDetails.startCommand = %q, want the cron's command", start)
	}
}

// The command is the only thing projected: runtime stays image, and nothing
// build-shaped is invented for a service that has no build.
func TestImageCronProjectsOnlyTheCommand(t *testing.T) {
	var read struct {
		ServiceDetails struct {
			Runtime            string         `json:"runtime"`
			EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(getServiceBody(t, imageCron("qa-image-cron", "echo hi")), &read); err != nil {
		t.Fatal(err)
	}
	if read.ServiceDetails.Runtime != "image" {
		t.Errorf("runtime = %q, want image", read.ServiceDetails.Runtime)
	}
	if want := map[string]any{"startCommand": "echo hi"}; !reflect.DeepEqual(read.ServiceDetails.EnvSpecificDetails, want) {
		t.Errorf("envSpecificDetails = %v, want only %v", read.ServiceDetails.EnvSpecificDetails, want)
	}
}

// An image cron with no command runs its entrypoint on purpose, and an image
// web service has no cron command: neither grows an envSpecificDetails block.
func TestImageServicesWithoutACronCommandProjectNoEnvDetails(t *testing.T) {
	web := sampleApp("qa-image-web")
	web.Spec.Image = "docker.io/library/nginx:1"
	web.Spec.Command = "nginx -g 'daemon off;'"
	for name, app := range map[string]*appv1alpha1.App{
		"entrypoint cron": imageCron("qa-image-cron", ""),
		"image web":       web,
	} {
		var read struct {
			ServiceDetails map[string]any `json:"serviceDetails"`
		}
		body := getServiceBody(t, app)
		if err := json.Unmarshal(body, &read); err != nil {
			t.Fatal(err)
		}
		if _, ok := read.ServiceDetails["envSpecificDetails"]; ok {
			t.Errorf("%s: envSpecificDetails present: %s", name, body)
		}
	}
}
