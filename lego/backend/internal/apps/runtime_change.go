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
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// checkRuntime is Render's PATCH serviceDetails.runtime (w5/m152) under the
// same rules create applies (resolveBuildStrategy): a repo-backed service may
// move between Render's native runtimes and docker, a native runtime needs
// its build and start commands, and an image service stays image (a source
// change, not a runtime change, moves it to a repo). It returns the runtime
// and the builder it selects.
func checkRuntime(a *appv1alpha1.App, runtime string) (string, string, error) {
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		return "", "", fmt.Errorf("%w: a static site has no runtime to change", core.ErrBadRequest)
	}
	return resolveBuildStrategy(a.Spec.Type, CreateRequest{
		Repo: a.Spec.Repo, Image: a.Spec.Image, Runtime: runtime,
		BuildCommand: a.Spec.BuildCommand, StartCommand: a.Spec.StartCommand,
	})
}

// applyRuntimeChange applies a validated runtime/builder change to spec.
// Moving from a non-Docker build (native or buildpack) to docker clears the
// stale native buildCommand and, unless the caller explicitly supplied a
// command in the same patch, clears startCommand so the old native command
// does not become a /bin/sh Docker Command override on the new Dockerfile.
func applyRuntimeChange(spec *appv1alpha1.AppSpec, runtime, builder string, dropNativeStart bool) {
	wasDocker := readsDockerCommand(buildStrategy(*spec))
	spec.Runtime, spec.Builder = runtime, builder
	if runtime == "docker" && !wasDocker {
		spec.BuildCommand = ""
		if dropNativeStart {
			spec.StartCommand = ""
		}
	}
}

// SetRuntime changes a service's runtime and rebuilds it with the builder the
// runtime selects, as Render does when a service's runtime changes.
func (s *Service) SetRuntime(ctx context.Context, name, runtime string) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	runtime, builder, err := checkRuntime(a, runtime)
	if err != nil {
		return AppView{}, err
	}
	// Echoing the read runtime ("image") of a prebuilt-image service changes
	// nothing: the retained runtime and builder stay for a switch back to Git
	// (w4/m202, w32/103).
	if prebuiltImageSource(a.Spec) && runtime == "image" {
		return s.view(a), nil
	}
	dropNativeStart := !patchHasExplicitStart(ctx)
	return s.recordedPatch(ctx, core.AuditVerbSetRuntime, a, func(a *appv1alpha1.App) {
		applyRuntimeChange(&a.Spec, runtime, builder, dropNativeStart)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}
