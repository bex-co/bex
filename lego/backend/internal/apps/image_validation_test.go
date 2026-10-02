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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestImageValidationRefusesBeforeMutation(t *testing.T) {
	for _, tc := range []struct{ name, image, reason string }{
		{"malformed", "https://ghcr.io/org/repo:tag", "malformed"},
		{"uppercase repository", "ghcr.io/Org/repo:tag", "must be lowercase"},
		{"short digest", "nginx@sha256:deadbeef", "digest length"},
		{"private registry", "127.0.0.1:5000/app:tag", "private or reserved"},
		{"untrusted registry", "registry.attacker.example/app:tag", "not trusted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &recordingStore{}
			svc, cl := newService(st)
			_, err := svc.Create(context.Background(), CreateRequest{Name: "invalid", Image: tc.image})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("create error = %v, want bad request naming %q", err, tc.reason)
			}

			// A valid earlier declaration must not be applied when a later image
			// is refused; both validation and apply share create's source check.
			manifest := fmt.Sprintf("services:\n  - name: valid\n    type: web\n    runtime: image\n    image: {url: nginx:1.27}\n  - name: invalid\n    type: web\n    runtime: image\n    image: {url: %q}\n", tc.image)
			validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, tc.reason) {
				t.Fatalf("Blueprint validation = %+v, err=%v", validation, err)
			}
			problem := validation.Errors[0]
			if problem.Path == nil || *problem.Path != "services[1].image" || problem.Line == nil || *problem.Line == 0 {
				t.Fatalf("image refusal has no source location: %+v", problem)
			}
			if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Blueprint apply error = %v, want bad request naming %q", err, tc.reason)
			}
			var apps appv1alpha1.AppList
			if err := cl.List(context.Background(), &apps); err != nil {
				t.Fatal(err)
			}
			if len(apps.Items) != 0 || len(st.appCreates) != 0 || len(st.deployCalls) != 0 {
				t.Fatal("refused image created an App or deploy")
			}
		})
	}
}

func TestImageValidationPreservesValidReferences(t *testing.T) {
	for _, image := range []string{
		"nginx:1.27", "library/nginx:1.27", "GHCR.IO/org/repo:Release_1.2",
		"ghcr.io/org/repo:Release@sha256:" + strings.Repeat("a", 64),
		"ghcr.io/org/repo@sha512:" + strings.Repeat("b", 128),
	} {
		t.Run(image, func(t *testing.T) {
			spec, err := specFromCreate(CreateRequest{Name: "image", Image: image})
			if err != nil || spec.Image != image {
				t.Fatalf("create image = %q, err=%v, want %q", spec.Image, err, image)
			}
			svc, cl := newService(nil, repoApp("web", "https://github.com/acme/app", "main"))
			if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Image: &image}); err != nil {
				t.Fatalf("update: %v", err)
			}
			if got := getApp(t, cl, "web").Spec; got.Image != image || got.Repo != "" {
				t.Fatalf("updated source = image %q repo %q", got.Image, got.Repo)
			}
			manifest := fmt.Sprintf("services:\n  - name: image\n    type: web\n    runtime: image\n    image: {url: %q}\n", image)
			validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || !validation.Valid {
				t.Fatalf("valid Blueprint = %+v, err=%v", validation, err)
			}
		})
	}
}

func TestImageValidationPreservesExistingSource(t *testing.T) {
	app := managedRepoApp("web")
	app.Spec.CloneSecret = "web-clone"
	app.Spec.RegistryCredentialID = strp("registry-credential")
	st := &recordingStore{}
	svc, cl := newService(st, app)
	before := getApp(t, cl, "web")
	for _, image := range []string{"ghcr.io/Org/repo:tag", "nginx@sha256:deadbeef", "127.0.0.1:5000/app:tag"} {
		if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Image: &image}); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("image %q: %v, want bad request", image, err)
		}
		if !reflect.DeepEqual(before, getApp(t, cl, "web")) || st.sourceCalls != 0 || len(st.deployCalls) != 0 {
			t.Fatal("invalid image changed source, credentials, release marker, or deploy history")
		}
	}
}
