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

package controller

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestImagePullSecretsGatedOnRegistryHostedImage pins w7/m8: the internal-registry
// imagePullSecret is attached ONLY to registry-hosted images (so a build-from-git
// deploy authenticates its pulls), never to an external/prebuilt image (so the
// registry credential is not sent to a foreign registry), and omitted entirely
// when no pull secret is configured (byte-identical default).
func TestImagePullSecretsGatedOnRegistryHostedImage(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000", RegistryPullSecret: "bex-registry-pull"}
	app := &appv1alpha1.App{}

	// Registry-hosted image (a build-from-git deploy) → pull secret attached.
	got := r.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1")
	if len(got) != 1 || got[0].Name != "bex-registry-pull" {
		t.Errorf("registry-hosted image = %+v; want [bex-registry-pull]", got)
	}

	// External/prebuilt image → no pull secret (cred never sent to a foreign registry).
	if got := r.imagePullSecrets(app, "docker.io/nginx:1.25"); got != nil {
		t.Errorf("external image = %+v; want nil", got)
	}
	// A same-name-but-different-host image is also external (prefix match is exact).
	if got := r.imagePullSecrets(app, "notzot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("foreign host masquerading = %+v; want nil", got)
	}

	// Unset pull secret → nil (byte-identical default; no imagePullSecret on the pod).
	r2 := &AppReconciler{Registry: "zot.bex-registry.svc:5000"}
	if got := r2.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("unset pull secret = %+v; want nil", got)
	}
}

// TestImagePullSecretsIncludesExternalRegistryCredential pins w2/m14: an App
// whose spec.externalRegistryPullSecret is set gets that Secret referenced
// too — additive to, never replacing, the internal-registry path, and present
// even when no internal pull secret is configured at all (an external/prebuilt
// image with a stored credential, the common case).
func TestImagePullSecretsIncludesExternalRegistryCredential(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000", RegistryPullSecret: "bex-registry-pull"}
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{ExternalRegistryPullSecret: "web-registry-pull"}}

	// Registry-hosted image + an external credential on the App → both attached.
	got := r.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1")
	if len(got) != 2 || got[0].Name != "bex-registry-pull" || got[1].Name != "web-registry-pull" {
		t.Errorf("both sources = %+v; want [bex-registry-pull web-registry-pull]", got)
	}

	// External/prebuilt image (no internal pull secret attaches) + a credential on
	// the App → only the external one, the common case this milestone adds.
	got = r.imagePullSecrets(app, "ghcr.io/acme/private-app:1.0")
	if len(got) != 1 || got[0].Name != "web-registry-pull" {
		t.Errorf("external-only image = %+v; want [web-registry-pull]", got)
	}

	// No credential on the App at all → nil, byte-identical to before this milestone.
	if got := r.imagePullSecrets(&appv1alpha1.App{}, "ghcr.io/acme/private-app:1.0"); got != nil {
		t.Errorf("no external credential = %+v; want nil", got)
	}
}
