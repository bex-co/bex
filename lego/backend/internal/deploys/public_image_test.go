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

package deploys

import (
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m236: the read boundary behind REST, GraphQL and MCP deploys strips the
// cluster-internal build registry host and leaves a user-supplied image
// byte-identical. The stored row keeps the raw ref (rollback reads it), so
// the projection must happen in view(), and REST renders that same view.
func TestDeployViewProjectsPublicImageRef(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	internal := "zot.bex-registry.svc:5000/ws/ws-svc:gen-2@" + digest
	got := view(nil, store.Deploy{ID: "dep-1", AppID: "srv-1", Image: internal})
	if want := "ws/ws-svc:gen-2@" + digest; got.Image != want {
		t.Errorf("view image = %q; want %q", got.Image, want)
	}
	if ref := toRenderDeploy(got).Image; ref == nil || ref.Ref != got.Image {
		t.Errorf("REST image ref = %+v; want the view's %q", ref, got.Image)
	}
	if strings.Contains(got.Image, ".svc") {
		t.Errorf("view image still carries a cluster-internal host: %q", got.Image)
	}

	external := "docker.io/traefik/whoami:v1.10"
	got = view(nil, store.Deploy{ID: "dep-2", AppID: "srv-1", Image: external})
	if got.Image != external {
		t.Errorf("view image = %q; want byte-identical %q", got.Image, external)
	}
}

// w4/621: Render's deploy has no image object for a Git-backed deploy, so a
// Git service's config-change or rollback deploy that reuses a built image
// reads without one, while an image service's deploy keeps it. The stored ref
// is untouched (rollback runs it).
func TestGitBackedDeployViewHasNoImage(t *testing.T) {
	built := "zot.bex-registry.svc:5000/ws/ws-svc:gen-1@sha256:" + strings.Repeat("b", 64)
	repo := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/bex"}}
	image := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v1.10"}}
	cases := []struct {
		name      string
		app       *appv1alpha1.App
		row       store.Deploy
		wantImage bool
	}{
		{"config change on a repo service, with its commit", repo, store.Deploy{ID: "dep-c", Image: built, ImageSHA: "sha256:1", Commit: "abc"}, false},
		{"rollback row of a repo service without a commit", repo, store.Deploy{ID: "dep-r", Image: built, ImageSHA: "sha256:1", Trigger: "rollback"}, false},
		{"image service deploy", image, store.Deploy{ID: "dep-i", Image: "docker.io/traefik/whoami:v1.10", ImageSHA: "sha256:2"}, true},
	}
	for _, c := range cases {
		got := view(c.app, c.row)
		if has := got.Image != "" || got.ImageSHA != ""; has != c.wantImage {
			t.Errorf("%s: image %q sha %q, want image present = %v", c.name, got.Image, got.ImageSHA, c.wantImage)
		}
		if has := toRenderDeploy(got).Image != nil; has != c.wantImage {
			t.Errorf("%s: REST image present = %v, want %v", c.name, has, c.wantImage)
		}
	}
}
