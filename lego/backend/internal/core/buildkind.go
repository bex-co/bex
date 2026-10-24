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

// IsDockerfileBuild reports whether spec builds the repo's own Dockerfile: a
// docker runtime, or no runtime with the default (auto) or dockerfile builder,
// as the operator's effectiveBuilder decides. A static site never does.
func IsDockerfileBuild(spec appv1alpha1.AppSpec) bool {
	if spec.Repo == "" || spec.Type == appv1alpha1.TypeStaticSite {
		return false
	}
	runtime := strings.ToLower(strings.TrimSpace(spec.Runtime))
	if runtime != "" {
		return runtime == "docker"
	}
	builder := strings.ToLower(strings.TrimSpace(spec.Builder))
	return builder == "" || builder == "auto" || builder == "dockerfile"
}
