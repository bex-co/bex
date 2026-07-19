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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildJobPullSecret pins the credential the static-site publish Job uses to
// pull the just-built tenant image from Zot. The publish Job runs in the BUILD
// namespace; when that differs from the App namespace (BEX_BUILD_NAMESPACE), the
// apps-namespace tenant pull secret (per-App reg-pull-<name> or shared) is NOT
// reachable, so the Job must use the build-namespace credential. w9/m44: pulling
// with the apps-ns secret was the prod static-site "Deploy failed" bug (build
// succeeded, publish/extract could not pull → the deploy failed post-build).
func TestBuildJobPullSecret(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "default"}}

	tests := []struct {
		name string
		r    *AppReconciler
		want string
	}{
		{
			// The prod case (BEX_BUILD_NAMESPACE=bex-system, per-App creds): the
			// apps-ns reg-pull-myapp is mirrored into the build namespace, so the
			// publisher keeps its own-repository scope instead of a shared credential.
			name: "separate build namespace uses the build-ns credential (the fix)",
			r: &AppReconciler{
				BuildNamespace:          "bex-system",
				RegistryBuildPullSecret: "bex-registry-pull",
				PerAppRegistry:          &registry.Creds{},
				RegistryPullSecret:      "must-not-be-used-apps-ns",
			},
			want: "reg-pull-myapp",
		},
		{
			name: "separate build namespace, no build credential => anonymous (dev)",
			r:    &AppReconciler{BuildNamespace: "bex-system"},
			want: "",
		},
		{
			name: "same namespace, per-App registry => reg-pull-<name>",
			r:    &AppReconciler{PerAppRegistry: &registry.Creds{}},
			want: "reg-pull-myapp",
		},
		{
			name: "same namespace, shared pull secret",
			r:    &AppReconciler{RegistryPullSecret: "shared-pull"},
			want: "shared-pull",
		},
		{
			name: "same namespace, unauthenticated (dev) => anonymous",
			r:    &AppReconciler{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.buildJobPullSecret(app); got != tt.want {
				t.Errorf("buildJobPullSecret = %q, want %q", got, tt.want)
			}
		})
	}
}
