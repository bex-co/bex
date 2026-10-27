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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func managedImageApp(name string) *appv1alpha1.App {
	a := managedRepoApp(name)
	a.Spec.Repo, a.Spec.Runtime, a.Spec.BuildCommand = "", "image", ""
	a.Spec.Image = "docker.io/traefik/whoami:v1.10.3"
	return a
}

// An image service refuses the build-from-git settings its own create
// refuses, on every write surface (w4/418).
func TestImageServiceRefusesGitSettingsOnUpdate(t *testing.T) {
	ctx := context.Background()
	yes, no, build := true, false, "make"
	for _, tc := range []struct {
		name, field string
		edit        func(*Service) error
	}{
		{"patch autoDeploy", "autoDeploy", func(s *Service) error {
			_, err := s.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &yes})
			return err
		}},
		{"graphql setAutoDeploy", "autoDeploy", func(s *Service) error {
			_, err := s.SetAutoDeploy(ctx, "web", true)
			return err
		}},
		{"patch buildCommand", "buildCommand", func(s *Service) error {
			_, err := s.ApplyServicePatch(ctx, "web", ServicePatch{BuildCommand: &build})
			return err
		}},
		{"graphql setBuildCommand", "buildCommand", func(s *Service) error {
			_, err := s.SetCommands(ctx, "web", &build, nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, managedImageApp("web"))
			err := tc.edit(svc)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "prebuilt image services cannot declare "+tc.field) {
				t.Fatalf("err = %v, want the create refusal for %s", err, tc.field)
			}
			var got appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &got); err != nil || got.Spec.AutoDeploy || got.Spec.BuildCommand != "" {
				t.Fatalf("refused write changed the App: %+v, %v", got.Spec, err)
			}
		})
	}

	t.Run("disabling stays accepted", func(t *testing.T) {
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &no}); err != nil {
			t.Fatal(err)
		}
	})
	// A branch is kept, hidden from reads, for a later switch to a repo: the
	// CLI sends image and branch together, and w8/088's two-step switch sets
	// branch first.
	t.Run("a staged branch stays accepted", func(t *testing.T) {
		branch := "main"
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Branch: &branch}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("switching to a repo with auto-deploy on in one patch works", func(t *testing.T) {
		repo := "https://github.com/bex-co/hello.git"
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Repo: &repo, AutoDeploy: &yes}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a repo-backed service still toggles", func(t *testing.T) {
		svc, _ := newService(nil, managedRepoApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &yes}); err != nil {
			t.Fatal(err)
		}
	})
}

// An image service stored with auto-deploy on reads back as off, as its
// branch does (w4/418).
func TestImageServiceReadsAutoDeployOff(t *testing.T) {
	a := managedImageApp("web")
	a.Spec.AutoDeploy = true
	svc, _ := newService(nil, a)
	v, err := svc.Get(context.Background(), "web")
	if err != nil || v.AutoDeploy || v.AutoDeployTrigger != "off" {
		t.Fatalf("view autoDeploy=%v trigger=%q (%v), want off", v.AutoDeploy, v.AutoDeployTrigger, err)
	}
}
