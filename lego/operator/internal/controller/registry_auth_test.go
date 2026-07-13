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

import "testing"

// TestImagePullSecretsGatedOnRegistryHostedImage pins w7/m8: the pull
// imagePullSecret is attached ONLY to registry-hosted images (so a build-from-git
// deploy authenticates its pulls), never to an external/prebuilt image (so the
// registry credential is not sent to a foreign registry), and omitted entirely
// when no pull secret is configured (byte-identical default).
func TestImagePullSecretsGatedOnRegistryHostedImage(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000", RegistryPullSecret: "bex-registry-pull"}

	// Registry-hosted image (a build-from-git deploy) → pull secret attached.
	got := r.imagePullSecrets("zot.bex-registry.svc:5000/hello:gen-1")
	if len(got) != 1 || got[0].Name != "bex-registry-pull" {
		t.Errorf("registry-hosted image = %+v; want [bex-registry-pull]", got)
	}

	// External/prebuilt image → no pull secret (cred never sent to a foreign registry).
	if got := r.imagePullSecrets("docker.io/nginx:1.25"); got != nil {
		t.Errorf("external image = %+v; want nil", got)
	}
	// A same-name-but-different-host image is also external (prefix match is exact).
	if got := r.imagePullSecrets("notzot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("foreign host masquerading = %+v; want nil", got)
	}

	// Unset pull secret → nil (byte-identical default; no imagePullSecret on the pod).
	r2 := &AppReconciler{Registry: "zot.bex-registry.svc:5000"}
	if got := r2.imagePullSecrets("zot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("unset pull secret = %+v; want nil", got)
	}
}
