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
)

// w4/m236: the read boundary behind REST, GraphQL and MCP deploys strips the
// cluster-internal build registry host and leaves a user-supplied image
// byte-identical. The stored row keeps the raw ref (rollback reads it), so
// the projection must happen in view(), and REST renders that same view.
func TestDeployViewProjectsPublicImageRef(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	internal := "zot.bex-registry.svc:5000/ws/ws-svc:gen-2@" + digest
	got := view(store.Deploy{ID: "dep-1", AppID: "srv-1", Image: internal})
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
	got = view(store.Deploy{ID: "dep-2", AppID: "srv-1", Image: external})
	if got.Image != external {
		t.Errorf("view image = %q; want byte-identical %q", got.Image, external)
	}
}
