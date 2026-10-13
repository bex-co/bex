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

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestDirectStaticPublishAfterClearedBuildCommand is w8/m59: a static site
// created with a build command carries the implicit native builder, so one
// whose command was cleared before the API dropped it must still publish its
// repo directly instead of failing a native build with no command. Every
// declared build input keeps the build path.
func TestDirectStaticPublishAfterClearedBuildCommand(t *testing.T) {
	site := func(mut func(*appv1alpha1.AppSpec)) *appv1alpha1.App {
		app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/site", PublishPath: ".",
		}}
		mut(&app.Spec)
		return app
	}
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want bool
	}{
		{"no build input", site(func(*appv1alpha1.AppSpec) {}), true},
		{"auto builder", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderAuto }), true},
		{"cleared implicit native", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderNative }), true},
		{"implicit native with command", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderNative; s.BuildCommand = "make" }), false},
		{"explicit runtime", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderNative; s.Runtime = "node" }), false},
		{"dockerfile builder", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderDockerfile }), false},
		{"buildpack builder", site(func(s *appv1alpha1.AppSpec) { s.Builder = build.BuilderBuildpack }), false},
		{"dockerfile path", site(func(s *appv1alpha1.AppSpec) { s.DockerfilePath = "Dockerfile" }), false},
		{"web service", site(func(s *appv1alpha1.AppSpec) { s.Type = appv1alpha1.TypeWebService }), false},
	} {
		if got := directStaticPublish(tc.app); got != tc.want {
			t.Errorf("%s: directStaticPublish = %v, want %v", tc.name, got, tc.want)
		}
	}
}
