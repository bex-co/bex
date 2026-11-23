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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// nativeBuildNeedsStart reports whether a native build runs its own start
// command: a static site serves its build's files (w5/090), and a build
// source's linked services supply their own (Render, w2/m254).
func nativeBuildNeedsStart(svcType string, buildOnly bool) bool {
	return svcType != appv1alpha1.TypeStaticSite && !buildOnly
}

// nativeCommandGaps reports which commands a native build needs and is not
// given: the build command always, the start command when it runs one.
func nativeCommandGaps(svcType string, buildOnly bool, buildCommand, startCommand string) (build, start bool) {
	build = strings.TrimSpace(buildCommand) == ""
	start = nativeBuildNeedsStart(svcType, buildOnly) && strings.TrimSpace(startCommand) == ""
	return build, start
}

// errNativeCommands refuses a native runtime a command it needs, in create's
// words: the requirement whole, whichever part is missing.
func errNativeCommands(svcType, runtime string, buildOnly bool) error {
	required := "buildCommand"
	if nativeBuildNeedsStart(svcType, buildOnly) {
		required = "buildCommand and startCommand"
	}
	return fmt.Errorf("%w: native runtime %s requires %s", core.ErrBadRequest, runtime, required)
}

// specNativeCommandGaps is nativeCommandGaps for the native build spec
// dispatches, with its runtime: none when spec builds no repo with a native
// runtime (a prebuilt image, a build source link, a Dockerfile or buildpack
// build, a static site publishing its files as they are).
func specNativeCommandGaps(spec appv1alpha1.AppSpec) (runtime string, build, start bool) {
	runtime, native := strings.CutPrefix(buildStrategy(spec), buildNative+":")
	if spec.Repo == "" || !native || !blueprintNativeRuntime(runtime) {
		return "", false, false
	}
	build, start = nativeCommandGaps(spec.Type, spec.Type == appv1alpha1.TypeBuildSource, spec.BuildCommand, spec.StartCommand)
	return runtime, build, start
}

// checkNativeCommandsKept refuses a change that leaves a native build without
// a command it needs and had. Create refuses that state up front
// (resolveBuildStrategy) and the operator fails every build of it at dispatch,
// so saving it only turned every later deploy into a failed build (w32/052):
// clearing a command, or switching an image back to a repo its runtime builds
// natively without sending the commands the image switch dropped, when that
// switch kept none (keepSwitchBackCommands, w32/056). A gap the
// change finds and leaves, saved before this check, refuses nothing, so such a
// service still takes its other settings, and each command one save at a time
// as the dashboard sends them.
func checkNativeCommandsKept(before, after appv1alpha1.AppSpec) error {
	_, buildBefore, startBefore := specNativeCommandGaps(before)
	runtime, build, start := specNativeCommandGaps(after)
	if (build && !buildBefore) || (start && !startBefore) {
		return errNativeCommands(after.Type, runtime, after.Type == appv1alpha1.TypeBuildSource)
	}
	return nil
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

// settleSourceSwitch drops what a source switch from before left on spec that
// no longer describes its build. A Git repo replacing an image leaves a stale
// runtime "image" (w8/088). A prebuilt image replacing a build leaves the build
// command, which an image service refuses (w4/418), and a start command the
// build baked into its image, which the image would otherwise run as its
// Docker Command (w8/m461): a native or buildpack build's, or a native build
// source's (nativeLink). A Dockerfile build's, an image's or a Docker build
// source's command already is a Docker Command and stays. The runtime, builder
// and root directory stay for a switch back (w4/m202), and so do the commands
// it drops from a build of the service's own: it returns them for
// keepSwitchBackCommands. A command saved with the switch is written after it.
func settleSourceSwitch(before appv1alpha1.AppSpec, spec *appv1alpha1.AppSpec, nativeLink bool) nativeCommands {
	if staleImageRuntime(*spec) {
		spec.Runtime = ""
	}
	if !prebuiltImageSource(*spec) || prebuiltImageSource(before) {
		return nativeCommands{}
	}
	build := spec.BuildCommand
	spec.BuildCommand = ""
	if before.BuildSource != nil {
		if nativeLink {
			// A linked cron keeps its command here (LinkBuildSource).
			spec.StartCommand, spec.Command = "", ""
		}
		// The link cleared the runtime and builder a switch back would
		// build with (linkBuildSourceSpec), so nothing is kept for one.
		return nativeCommands{}
	}
	if readsDockerCommand(buildStrategy(before)) {
		return nativeCommands{}
	}
	dropped := nativeCommands{build: build, start: spec.StartCommand}
	spec.StartCommand = ""
	return dropped
}

// The native build and start commands a switch to a prebuilt image dropped
// (settleSourceSwitch), kept on the App while that image is its source, so a
// switch back to the repo builds with them again (w32/056). The image never
// runs them: spec.startCommand is its Docker Command. The pinned Render CLI
// cannot send commands with that switch, since it refuses --build-command and
// --start-command on a service whose runtime reads image, so without them
// `services update --repo` had no buildable answer. Only bex-api reads them.
const (
	switchBackBuildCommandAnnotation = "app.bex.co/switch-back-build-command"
	switchBackStartCommandAnnotation = "app.bex.co/switch-back-start-command"
)

// nativeCommands are the build and start commands a native or buildpack build
// bakes into the image it builds.
type nativeCommands struct{ build, start string }

// keepSwitchBackCommands carries a build's commands across a switch to a
// prebuilt image and back (w32/056), given the spec a's source switch started
// from and what settleSourceSwitch dropped. The switch to the image keeps what
// it dropped. The switch back to a repo puts them back where the build reads
// them, replacing an image's Docker Command, before any command the update
// sends, which is written after the source and wins. A change of image keeps
// what the first switch kept, and every other source forgets it, so a switch
// back with nothing kept and no commands sent still gets create's 400
// (checkNativeCommandsKept).
func keepSwitchBackCommands(a *appv1alpha1.App, before appv1alpha1.AppSpec, dropped nativeCommands) {
	image, wasImage := prebuiltImageSource(a.Spec), prebuiltImageSource(before)
	switch {
	case image && wasImage:
		return
	case image:
		setSwitchBackCommands(a, dropped)
		return
	case wasImage && !readsDockerCommand(buildStrategy(a.Spec)):
		kept := switchBackCommands(a)
		if kept.build != "" {
			a.Spec.BuildCommand = kept.build
		}
		if kept.start != "" {
			a.Spec.StartCommand = kept.start
		}
	}
	forgetSwitchBackCommands(a)
}

// switchBackCommands are the commands a keeps for a switch back.
func switchBackCommands(a *appv1alpha1.App) nativeCommands {
	return nativeCommands{
		build: a.Annotations[switchBackBuildCommandAnnotation],
		start: a.Annotations[switchBackStartCommandAnnotation],
	}
}

// setSwitchBackCommands keeps kept on a, each command only when it has one.
func setSwitchBackCommands(a *appv1alpha1.App, kept nativeCommands) {
	setOrDeleteAnnotation(a, switchBackBuildCommandAnnotation, kept.build)
	setOrDeleteAnnotation(a, switchBackStartCommandAnnotation, kept.start)
}

// setOrDeleteAnnotation sets key on a to value, or removes it for "".
func setOrDeleteAnnotation(a *appv1alpha1.App, key, value string) {
	if value == "" {
		delete(a.Annotations, key)
		return
	}
	metav1.SetMetaDataAnnotation(&a.ObjectMeta, key, value)
}

// forgetSwitchBackCommands drops what a keeps for a switch back: a source other
// than the image they were kept beside has its own commands, or none.
func forgetSwitchBackCommands(a *appv1alpha1.App) {
	setSwitchBackCommands(a, nativeCommands{})
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
	setCronCommandForStrategy(spec, command, buildStrategy(*spec))
}

func setCronCommandForStrategy(spec *appv1alpha1.AppSpec, command, strategy string) {
	spec.Command = commandText(command)
	if spec.Command == "" && spec.Type == appv1alpha1.TypeCronJob && readsDockerCommand(strategy) {
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

// startIsDockerCommand reports that resolve's start command is the
// dockerCommand: the build reads one and it is not blank.
func (d dockerDetails) startIsDockerCommand() bool {
	return d.commandApplies() && strings.TrimSpace(d.dockerCommand) != ""
}

// resolve folds the details the build reads onto the create's commands and
// build context: a non-blank dockerCommand is the start command, and also a
// cron's run command when none is given; the context applies to a Dockerfile
// build only.
func (d dockerDetails) resolve() (startCommand, command, context string) {
	startCommand, command = d.startCommand, d.command
	context = d.dockerfileOnly(d.dockerContext)
	if d.startIsDockerCommand() {
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

// targetBuildStrategy returns the build strategy spec will have once a
// runtime change in the same ServicePatch (targetRuntime) is applied, or
// spec's current strategy when no runtime change rides along.
func targetBuildStrategy(spec appv1alpha1.AppSpec, targetRuntime string) string {
	if targetRuntime == "" {
		return buildStrategy(spec)
	}
	next := spec
	next.Runtime = targetRuntime
	switch {
	case targetRuntime == "docker":
		next.Builder = buildDockerfile
	case blueprintNativeRuntime(targetRuntime):
		next.Builder = buildNative
	}
	return buildStrategy(next)
}

// setStartCommandForStrategy is Render's runtime-specific start command write:
// a container cron calls it its command, every other runtime-backed type, a
// native cron included, spec.startCommand. A native cron's write also drops
// any command override, which would otherwise keep running in its place; a
// blank one drops only the override, as the native build needs its start
// command (w8/m69).
func setStartCommandForStrategy(spec *appv1alpha1.AppSpec, command, strategy string) {
	if spec.Type == appv1alpha1.TypeCronJob {
		if !nativeBuild(strategy) {
			setCronCommandForStrategy(spec, command, strategy)
			return
		}
		spec.Command = ""
		if commandText(command) == "" {
			return
		}
	}
	spec.StartCommand = commandText(command)
}
