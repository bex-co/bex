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
	"net/http"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func nodeApp() *appv1alpha1.App {
	a := sampleApp("web")
	a.Spec.Image = ""
	a.Spec.Repo, a.Spec.Branch = "https://github.com/acme/web", "main"
	a.Spec.Runtime, a.Spec.Builder = "node", "native"
	a.Spec.BuildCommand, a.Spec.StartCommand = "npm ci", "npm start"
	return a
}

// Render lets PATCH serviceDetails.runtime move a service between runtimes
// (w5/m152); bex rebuilds it with the builder the new runtime selects.
func TestRESTRuntimeChangeRebuildsWithTheRuntimesBuilder(t *testing.T) {
	for _, tc := range []struct{ runtime, builder string }{
		{"python", "native"},
		{"docker", "dockerfile"},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			svc, cl := newService(nil, nodeApp())
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := restDo(t, mux, http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"runtime":"`+tc.runtime+`"}}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH runtime => %d: %s", rec.Code, rec.Body)
			}
			a := getApp(t, cl, "web")
			if a.Spec.Runtime != tc.runtime || a.Spec.Builder != tc.builder || a.Spec.RestartedAt == "" {
				t.Fatalf("spec = runtime %q builder %q restartedAt %q, want %s/%s and a rebuild", a.Spec.Runtime, a.Spec.Builder, a.Spec.RestartedAt, tc.runtime, tc.builder)
			}
		})
	}
}

func TestRESTRuntimeChangeRefusesWhatCreateRefuses(t *testing.T) {
	docker := nodeApp()
	docker.Spec.Runtime, docker.Spec.Builder, docker.Spec.BuildCommand, docker.Spec.StartCommand = "docker", "dockerfile", "", ""
	image := sampleApp("web")
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		body string
		want string
	}{
		{"docker to node without commands", docker, `{"serviceDetails":{"runtime":"node"}}`, "requires buildCommand and startCommand"},
		{"an image service to docker", image, `{"serviceDetails":{"runtime":"docker"}}`, "requires repo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, tc.app)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := restDo(t, mux, http.MethodPatch, "/v1/services/web", tc.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("PATCH => %d %s, want a 400 saying %q", rec.Code, rec.Body, tc.want)
			}
		})
	}
	// The same move succeeds when the PATCH carries the commands.
	svc, cl := newService(nil, docker.DeepCopy())
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := restDo(t, mux, http.MethodPatch, "/v1/services/web",
		`{"serviceDetails":{"runtime":"node","envSpecificDetails":{"buildCommand":"npm ci","startCommand":"npm start"}}}`)
	if rec.Code != http.StatusOK || getApp(t, cl, "web").Spec.Runtime != "node" {
		t.Fatalf("docker to node with commands => %d %s", rec.Code, rec.Body)
	}
}
