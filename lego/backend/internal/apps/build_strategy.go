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
	if staleImageRuntime(spec) {
		runtime = ""
	}
	switch {
	case runtime == "image" || prebuiltImageSource(spec):
		return buildImage
	case runtime == "docker":
		return buildDockerfile
	case runtime != "":
		return buildNative + ":" + runtime
	case spec.Type == appv1alpha1.TypeStaticSite && spec.Builder == buildNative &&
		strings.TrimSpace(spec.BuildCommand) == "" && strings.TrimSpace(spec.DockerfilePath) == "":
		// A static native builder with no runtime projects a command, so
		// without one it publishes, as the operator does (w8/m59).
		return buildPublish
	case spec.Builder != "" && spec.Builder != "auto":
		return spec.Builder
	case spec.Type != appv1alpha1.TypeStaticSite, strings.TrimSpace(spec.DockerfilePath) != "":
		return buildDockerfile
	case strings.TrimSpace(spec.BuildCommand) != "":
		return buildNative
	}
	return buildPublish
}

// commandText is the stored form of an executable command: nonblank text is
// kept byte for byte, because trailing whitespace can belong to the last shell
// argument (`printf '%s' tail\ ` passes "tail "), and whitespace-only is empty
// so it still clears or omits the command (w8/m68).
func commandText(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	return command
}

// setBuildCommand is an explicit build-command write. A static site's native
// builder with no runtime only projects its build command (effectiveBuilder
// re-derives it), so clearing the command drops it too and the site publishes
// directly, as one created without a command does (w8/m59).
func setBuildCommand(spec *appv1alpha1.AppSpec, command string) {
	spec.BuildCommand = commandText(command)
	if spec.BuildCommand == "" && spec.Type == appv1alpha1.TypeStaticSite &&
		spec.Runtime == "" && spec.Builder == buildNative {
		spec.Builder = ""
	}
}

// prebuiltImageSource reports whether spec runs a prebuilt image as its
// source, rather than building a repo (whose image is a rollback override).
func prebuiltImageSource(spec appv1alpha1.AppSpec) bool {
	return spec.Image != "" && spec.Repo == ""
}

// staleImageRuntime reports a runtime "image" left from a prebuilt-image
// source after a Git repo replaced it: it no longer describes the build, which
// the repo and builder decide (w8/088). Source saves clear it; reads of a
// record saved before that ignore it.
func staleImageRuntime(spec appv1alpha1.AppSpec) bool {
	return strings.EqualFold(strings.TrimSpace(spec.Runtime), buildImage) && spec.Repo != "" && spec.Image == ""
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

// cronStartCommandOverrides reports whether a cron's spec.startCommand is a
// runtime override: with spec.command empty, the operator's cronPodSpec runs
// it for every build except native and buildpack, which bake it into the
// image instead. A Blueprint image cron keeps its dockerCommand there.
func cronStartCommandOverrides(spec appv1alpha1.AppSpec) bool {
	return spec.Type == appv1alpha1.TypeCronJob && readsDockerCommand(buildStrategy(spec))
}

// cronCommand is the command a cron runs: spec.command, or the startCommand
// fallback the operator substitutes when it is empty (w4/m194).
func cronCommand(spec appv1alpha1.AppSpec) string {
	if spec.Command == "" && cronStartCommandOverrides(spec) {
		return spec.StartCommand
	}
	return spec.Command
}

// setCronCommand is an explicit cron command write. Clearing it also drops a
// startCommand fallback, or the operator would revive the old command instead
// of running the image's own (w4/m194); a native or buildpack cron keeps its
// baked start command.
func setCronCommand(spec *appv1alpha1.AppSpec, command string) {
	spec.Command = commandText(command)
	if spec.Command == "" && cronStartCommandOverrides(*spec) {
		spec.StartCommand = ""
	}
}

// blueprintReadsDockerCommand is readsDockerCommand for a Blueprint service,
// from the runtime and x-bex.builder it declares: runtime image, or runtime
// docker that is not the schema's spelling of a buildpack build (w5/102).
func blueprintReadsDockerCommand(runtime, builder string) bool {
	runtime = strings.ToLower(strings.TrimSpace(runtime))
	return runtime == "image" || (runtime == "docker" && !strings.EqualFold(builder, buildBuildpack))
}

// resolve folds the details the build reads onto the create's commands and
// build context: a non-blank dockerCommand is the start command, and also a
// cron's run command when none is given; the context applies to a Dockerfile
// build only.
func (d dockerDetails) resolve() (startCommand, command, context string) {
	startCommand, command = d.startCommand, d.command
	context = d.dockerfileOnly(d.dockerContext)
	if d.commandApplies() && strings.TrimSpace(d.dockerCommand) != "" {
		startCommand = d.dockerCommand
		if command == "" {
			command = d.dockerCommand
		}
	}
	return startCommand, command, context
}

// dockerfileOnly is value when the create builds a Dockerfile, else empty: a
// Dockerfile-build setting is inert elsewhere, as on Render, so the transports
// drop it rather than send the core a setting it refuses (w5/090).
func (d dockerDetails) dockerfileOnly(value string) string {
	if d.build() == buildDockerfile {
		return value
	}
	return ""
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

// nativeBuild reports whether build is a native toolchain's, which bakes the
// start command into the generated image's Bash CMD rather than running it as
// a /bin/sh override of the image (app_types.go StartCommand).
func nativeBuild(build string) bool {
	return strings.HasPrefix(build, buildNative)
}

// setStartCommand is Render's runtime-specific start command write: a
// container cron calls it its command, every other runtime-backed type,
// a native cron included, spec.startCommand. A native cron's write also drops
// any command override, which would otherwise keep running in its place; a
// blank one drops only the override, as the native build needs its start
// command (w8/m69).
func setStartCommand(spec *appv1alpha1.AppSpec, command string) {
	if spec.Type == appv1alpha1.TypeCronJob {
		if !nativeBuild(buildStrategy(*spec)) {
			setCronCommand(spec, command)
			return
		}
		spec.Command = ""
		if commandText(command) == "" {
			return
		}
	}
	spec.StartCommand = commandText(command)
}
