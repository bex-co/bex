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
	"fmt"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The build strategies buildStrategy names. A native runtime is
// buildNative + ":" + its runtime.
const (
	buildDockerfile = "dockerfile"
	buildImage      = "image"
	buildBuildpack  = "buildpack"
	buildNative     = "native"
	buildPublish    = "publish"
)

// blueprintBuildpackRuntime is the runtime a Blueprint declares for a
// buildpack build. render.yaml requires one, and only x-bex.builder can spell
// buildpacks, so beside it this runtime satisfies the schema and selects
// nothing: the export writes it and the parse drops it (w5/m117).
const blueprintBuildpackRuntime = "docker"

// buildStrategy is what a spec builds with, however it spells it: a native
// toolchain, the repo's Dockerfile, buildpacks, nothing (a prebuilt image),
// or, for a static site, no build at all. A repo build with no runtime builds
// its Dockerfile, except a static site, which builds natively for a build
// command and otherwise publishes the repo as it is (the operator's
// effectiveBuilder and directStaticPublish).
func buildStrategy(spec appv1alpha1.AppSpec) string {
	runtime := strings.ToLower(strings.TrimSpace(spec.Runtime))
	switch {
	case runtime == "image" || (spec.Image != "" && spec.Repo == ""):
		return buildImage
	case runtime == "docker":
		return buildDockerfile
	case runtime != "":
		return buildNative + ":" + runtime
	case spec.Builder != "" && spec.Builder != "auto":
		return spec.Builder
	case spec.Type != appv1alpha1.TypeStaticSite, strings.TrimSpace(spec.DockerfilePath) != "":
		return buildDockerfile
	case strings.TrimSpace(spec.BuildCommand) != "":
		return buildNative
	}
	return buildPublish
}

// dockerDetails are Render's docker command and build context as a create
// sends them, beside the inputs that decide whether its build reads them.
// REST's envSpecificDetails and MCP's flat arguments resolve through it, so
// the transports agree (TestCreateWireFieldsReachTheRequest).
type dockerDetails struct {
	runtime, builder, image      string
	startCommand, command        string
	dockerCommand, dockerContext string
}

// build is what the create builds, judged from the inputs a transport
// receives.
func (d dockerDetails) build() string {
	return buildStrategy(appv1alpha1.AppSpec{Runtime: d.runtime, Builder: d.builder, Image: d.image})
}

// commandApplies reports whether the build reads a dockerCommand. A native
// runtime and buildpacks take their command elsewhere, so there it is inert,
// as on Render, and a call Render accepts keeps its native commands.
func (d dockerDetails) commandApplies() bool {
	return readsDockerCommand(d.build())
}

// readsDockerCommand reports whether a build runs a container command, the one
// render.yaml spells dockerCommand: a Dockerfile build and a prebuilt image do.
func readsDockerCommand(build string) bool {
	return build == buildDockerfile || build == buildImage
}

// resolve folds the details the build reads onto the create's commands and
// build context: a non-blank dockerCommand is the start command, and also a
// cron's run command when none is given; the context applies to a Dockerfile
// build only.
func (d dockerDetails) resolve() (startCommand, command, context string) {
	startCommand, command = d.startCommand, d.command
	if d.build() == buildDockerfile {
		context = d.dockerContext
	}
	if d.commandApplies() && strings.TrimSpace(d.dockerCommand) != "" {
		startCommand = d.dockerCommand
		if command == "" {
			command = d.dockerCommand
		}
	}
	return startCommand, command, context
}

// conflict refuses a dockerCommand the build reads sent beside another
// spelling of the same command, as the Blueprint compiler does ("cannot set
// both dockerCommand and startCommand"). MCP refuses it because its flat
// startCommand (and a cron's bex `command`) are its own arguments, and
// silently picking one would drop the caller's other value. REST resolves the
// same collision by precedence instead (dockerCommand wins, w9/m165), because
// the pinned CLI sends a cron's command as startCommand on every runtime.
func (d dockerDetails) conflict() error {
	if !d.commandApplies() || strings.TrimSpace(d.dockerCommand) == "" {
		return nil
	}
	if strings.TrimSpace(d.startCommand) != "" {
		return fmt.Errorf("%w: cannot set both dockerCommand and startCommand", core.ErrBadRequest)
	}
	if strings.TrimSpace(d.command) != "" {
		return fmt.Errorf("%w: cannot set both dockerCommand and command", core.ErrBadRequest)
	}
	return nil
}
