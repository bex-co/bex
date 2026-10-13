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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/088: an image-born service switched to a Git repo kept runtime "image",
// so every read said image and the pinned CLI's same-type clone refused with
// "runtime is required". Through the real create and source fold, the saved
// Git source now reads as the default Docker build on every image-born kind,
// while a genuine image, a branch-only save and an explicit native runtime
// keep theirs.
func TestImageToGitSourceSaveReadsDockerRuntime(t *testing.T) {
	ctx := context.Background()
	repo := "https://github.com/traefik/whoami"
	for _, req := range []CreateRequest{
		{Name: "web", Type: appv1alpha1.TypeWebService},
		{Name: "private", Type: appv1alpha1.TypePrivateService},
		{Name: "worker", Type: appv1alpha1.TypeBackgroundWorker},
		{Name: "cron", Type: appv1alpha1.TypeCronJob, Schedule: "*/15 * * * *", Command: "/echo"},
	} {
		t.Run(req.Type, func(t *testing.T) {
			svc, cl := newService(nil)
			req.Runtime, req.Image = "image", "docker.io/mendhak/http-https-echo:39"
			if _, err := svc.Create(ctx, req); err != nil {
				t.Fatalf("Create: %v", err)
			}
			branch := "main"
			if v, err := svc.ApplyServicePatch(ctx, req.Name, ServicePatch{Branch: &branch}); err != nil || v.Runtime != "image" {
				t.Fatalf("branch-only save on the image = %q, %v; want image kept", v.Runtime, err)
			}
			v, err := svc.ApplyServicePatch(ctx, req.Name, ServicePatch{Repo: &repo})
			if err != nil {
				t.Fatalf("source save: %v", err)
			}
			details := renderServiceDetails(v, req.Type, "")
			if v.Runtime != "docker" || details["runtime"] != "docker" || details["env"] != "docker" {
				t.Fatalf("after Git save: view %q, render runtime %v env %v; want docker", v.Runtime, details["runtime"], details["env"])
			}
			if spec := getApp(t, cl, req.Name).Spec; spec.Runtime != "" || spec.Image != "" || spec.Repo != repo || buildStrategy(spec) != buildDockerfile {
				t.Fatalf("spec = runtime %q image %q repo %q strategy %q", spec.Runtime, spec.Image, spec.Repo, buildStrategy(spec))
			}
		})
	}

	// A record saved before the fix still reads as its Docker build.
	legacy := appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: repo, Runtime: "image", Builder: "auto"}
	if got := effectiveRuntime(legacy, legacy.Type); got != "docker" || buildStrategy(legacy) != buildDockerfile {
		t.Fatalf("legacy runtime = %q strategy %q, want docker", got, buildStrategy(legacy))
	}
	// A Dockerfile path is a Dockerfile-build setting, which the legacy record is.
	svc, _ := newService(nil, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: "default"}, Spec: legacy})
	if _, err := svc.checkDockerfilePath(ctx, getApp(t, svc.Client, "legacy"), "legacy", "Dockerfile"); err != nil {
		t.Fatalf("dockerfilePath on the legacy record = %v, want accepted", err)
	}
	// Controls: a genuine image and an explicit native Git runtime keep theirs.
	if got := effectiveRuntime(appv1alpha1.AppSpec{Image: "nginx:1", Runtime: "image"}, appv1alpha1.TypeWebService); got != "image" {
		t.Fatalf("genuine image runtime = %q", got)
	}
	if got := effectiveRuntime(appv1alpha1.AppSpec{Repo: repo, Runtime: "go", Builder: "native"}, appv1alpha1.TypeWebService); got != "go" {
		t.Fatalf("native runtime = %q", got)
	}
}
