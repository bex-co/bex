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

package core

import (
	"strings"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// EffectiveRuntime resolves the Render-facing source runtime a read surface
// reports (serviceDetails.runtime/env, the runtime-keyed envSpecificDetails, and
// the GraphQL/MCP runtime field). Every Render service HAS a runtime, but bex's
// App CR leaves spec.runtime empty for the common Dockerfile build it expresses
// through spec.builder instead — a hand-applied App, a Blueprint, or a dashboard
// create that never set Render's `runtime` alias. The official CLI reads
// serviceDetails.runtime to round-trip a partial `services update`: with none
// returned it rejects the empty value client-side ("unsupported runtime") or
// treats an explicit --runtime as a forbidden switch ("cannot switch runtimes
// via the CLI"), so a Dockerfile web service could not repoint its own
// healthCheckPath (w4/052). Deriving it here — the one place the read runtime is
// decided — keeps that string consistent across every surface.
//
// The mapping mirrors the operator's effectiveBuilder in reverse and names only
// a runtime bex can determine unambiguously: an explicit spec.runtime wins; a
// prebuilt image (image set, no repo build) is "image"; and a repo build under
// the default/auto/dockerfile builder is "docker" (all three resolve to a
// Dockerfile build). A static site has no App runtime (bex's runtime enum has no
// "static"), and a bare "buildpack"/"native" builder without an explicit runtime
// names no runtime bex can pin here, so each reads back empty as it did before.
func EffectiveRuntime(spec appv1alpha1.AppSpec, svcType string) string {
	if spec.Runtime != "" && !(strings.EqualFold(strings.TrimSpace(spec.Runtime), "image") && spec.Repo != "" && spec.Image == "") {
		return spec.Runtime
	}
	if svcType == appv1alpha1.TypeStaticSite {
		return ""
	}
	if spec.Image != "" && spec.Repo == "" {
		return "image"
	}
	if spec.Repo != "" {
		switch spec.Builder {
		case "", "auto", "dockerfile":
			return "docker"
		}
	}
	return ""
}
