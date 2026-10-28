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

// w8/210: once PORT is removed, neither a rollback to the digest-pinned
// release nor a fresh deploy of the tag may reuse a network resolved from
// the removed PORT; both return to the image's declared port.
func TestRemovedPortLeavesTheConfiguredNetwork(t *testing.T) {
	for _, tc := range []struct {
		name string
		// servedConfigured: the configured port change went live (echo 8080
		// -> 9090) rather than failing (whoami 80 -> 9999).
		servedConfigured bool
		rollback         bool
	}{
		{"rollback after a failed port change", false, true},
		{"fresh deploy after a failed port change", false, false},
		{"fresh deploy after a served port change", true, false},
		{"rollback after a served port change", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{HTTPClient: imageConfigRegistry(t, `"80/tcp":{}`)}
			app := webImageApp(1)
			pinned, port, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
			if err != nil || port != 80 {
				t.Fatalf("first release = %d, %v; want 80", port, err)
			}
			served := *app.Status.ImageNetwork
			app.Status.ServingNetwork = &served

			app.Generation, app.Status.ReleaseGeneration, app.Status.EnvPort = 2, 2, 9999
			if _, port, err = r.resolveImageNetwork(t.Context(), app, app.Spec.Image); err != nil || port != 9999 {
				t.Fatalf("PORT=9999 release = %d, %v; want 9999", port, err)
			}
			if tc.servedConfigured {
				configured := *app.Status.ImageNetwork
				app.Status.ServingNetwork = &configured
			}

			app.Generation, app.Status.ReleaseGeneration, app.Status.EnvPort = 3, 3, 0
			image := app.Spec.Image
			if tc.rollback {
				image = pinned
			}
			if _, port, err = r.resolveImageNetwork(t.Context(), app, image); err != nil || port != 80 {
				t.Fatalf("after removing PORT = %d, %v; want the image's port 80", port, err)
			}
			if got := app.Status.ImageNetwork.HTTPSelection; got == appv1alpha1.HTTPSelectionConfigured {
				t.Fatalf("HTTPSelection = %q, want an unconfigured network", got)
			}
		})
	}
}

// A configured PORT that stays set keeps its cached network (t007's
// 8080 -> 9090 control): only a configured-ness change misses the cache.
func TestConfiguredPortReusesItsNetwork(t *testing.T) {
	r := &AppReconciler{HTTPClient: imageConfigRegistry(t, `"80/tcp":{}`)}
	app := webImageApp(1)
	app.Status.EnvPort = 9090
	pinned, port, err := r.resolveImageNetwork(t.Context(), app, app.Spec.Image)
	if err != nil || port != 9090 {
		t.Fatalf("PORT=9090 = %d, %v", port, err)
	}
	app.Generation, app.Status.ReleaseGeneration = 2, 2
	if _, port, err = r.resolveImageNetwork(t.Context(), app, pinned); err != nil || port != 9090 {
		t.Fatalf("redeploy with PORT=9090 = %d, %v; want 9090", port, err)
	}
}
