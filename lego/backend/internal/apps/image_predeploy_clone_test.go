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

// w2/042. The pinned client clones a web/private/worker pre-deploy command
// only from serviceDetails.envSpecificDetails.preDeployCommand, and an
// image-runtime service projected no envSpecificDetails — so `services create
// --from` cloned an image service that deployed without its migration step.

// imagePreDeployKinds are the service types that run a pre-deploy phase.
var imagePreDeployKinds = []string{
	appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker,
}

// imagePreDeployCloneSource is the read the pinned client clones from, shared
// with lego/cli/image_predeploy_clone_contract_test.go. Regenerate with
// BEX_UPDATE_GOLDEN=1 when the read shape intentionally moves.
func imagePreDeployCloneSource(svcType string) string {
	return filepath.Join("..", "..", "..", "cli", "testdata", "image-predeploy-clone-source-"+svcType+".json")
}

func imagePreDeployApp(svcType, preDeploy string) *appv1alpha1.App {
	app := sampleApp("qa-image-predeploy")
	app.Spec.Type = svcType
	app.Spec.Image = "docker.io/mendhak/http-https-echo:35"
	app.Spec.Tier = "starter"
	app.Spec.Replicas = 1
	app.Spec.PreDeployCommand = preDeploy
	if svcType == appv1alpha1.TypeBackgroundWorker {
		app.Status.URL = ""
	}
	return app
}

func TestImagePreDeployReadIsTheCloneContract(t *testing.T) {
	for _, svcType := range imagePreDeployKinds {
		body := getServiceBody(t, imagePreDeployApp(svcType, "echo QA_PREDEPLOY_MARKER"))
		golden := imagePreDeployCloneSource(svcType)
		if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, body, "", "  "); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, append(pretty.Bytes(), '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: read the shared golden: %v", svcType, err)
		}
		var got, want any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: the image read drifted from the clone contract the pinned CLI is tested against\n got: %s\nwant: %s", svcType, body, raw)
		}

		var read struct {
			ServiceDetails struct {
				Runtime            string         `json:"runtime"`
				PreDeployCommand   string         `json:"preDeployCommand"`
				EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
			} `json:"serviceDetails"`
		}
		if err := json.Unmarshal(body, &read); err != nil {
			t.Fatal(err)
		}
		d := read.ServiceDetails
		if d.Runtime != "image" {
			t.Errorf("%s: runtime = %q, want image", svcType, d.Runtime)
		}
		// Only the command is projected — nothing build-shaped is invented —
		// and the existing sibling field stays for its current readers.
		if want := map[string]any{"preDeployCommand": "echo QA_PREDEPLOY_MARKER"}; !reflect.DeepEqual(d.EnvSpecificDetails, want) {
			t.Errorf("%s: envSpecificDetails = %v, want only %v", svcType, d.EnvSpecificDetails, want)
		}
		if d.PreDeployCommand != "echo QA_PREDEPLOY_MARKER" {
			t.Errorf("%s: sibling serviceDetails.preDeployCommand = %q", svcType, d.PreDeployCommand)
		}
	}
}

// With no pre-deploy command an image service still has nothing to carry.
func TestImageServiceWithoutPreDeployProjectsNoEnvDetails(t *testing.T) {
	for _, svcType := range imagePreDeployKinds {
		var read struct {
			ServiceDetails map[string]any `json:"serviceDetails"`
		}
		body := getServiceBody(t, imagePreDeployApp(svcType, ""))
		if err := json.Unmarshal(body, &read); err != nil {
			t.Fatal(err)
		}
		if _, ok := read.ServiceDetails["envSpecificDetails"]; ok {
			t.Errorf("%s: envSpecificDetails present: %s", svcType, body)
		}
	}
}

// The list read shares the projection, so a client cloning from a listed
// service sees the same command.
func TestImagePreDeployListReadCarriesTheCommand(t *testing.T) {
	svc, _ := newService(nil, imagePreDeployApp(appv1alpha1.TypeWebService, "bin/migrate"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body.String())
	}
	var list []struct {
		Service struct {
			ServiceDetails struct {
				EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
			} `json:"serviceDetails"`
		} `json:"service"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v (%s)", err, rec.Body.String())
	}
	if len(list) != 1 || list[0].Service.ServiceDetails.EnvSpecificDetails["preDeployCommand"] != "bin/migrate" {
		t.Fatalf("listed envSpecificDetails missing the command: %s", rec.Body.String())
	}
}

// w4/209: an image web/private/worker service's Docker Command (stored as
// spec.startCommand; REST create spells it envSpecificDetails.dockerCommand,
// w4/188) never read back on REST, so `services get` hid it. It now reads back
// as dockerCommand beside the pre-deploy command, and alone without one.
func TestImageDockerCommandReadsBack(t *testing.T) {
	for _, svcType := range imagePreDeployKinds {
		for _, preDeploy := range []string{"", "bin/migrate"} {
			app := imagePreDeployApp(svcType, preDeploy)
			app.Spec.StartCommand = "/whoami --port 8080"
			var read struct {
				ServiceDetails struct {
					EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
				} `json:"serviceDetails"`
			}
			if err := json.Unmarshal(getServiceBody(t, app), &read); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"dockerCommand": "/whoami --port 8080"}
			if preDeploy != "" {
				want["preDeployCommand"] = preDeploy
			}
			if got := read.ServiceDetails.EnvSpecificDetails; !reflect.DeepEqual(got, want) {
				t.Errorf("%s (preDeploy %q): envSpecificDetails = %v, want %v", svcType, preDeploy, got, want)
			}
		}
	}
}
