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

package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// NativeEnvNoneRevision is the stable BuildKit cache key when a native build
// has no effective environment. It still appears in the generated Dockerfile so
// introducing the first env value cannot reuse a no-env cached RUN (w7/m87).
const NativeEnvNoneRevision = "none"

// nativeEnvRevisionPattern rejects anything that could break out of the
// quoted no-op shell token we embed in the generated Dockerfile. The revision
// is opaque platform metadata — never a secret value.
var nativeEnvRevisionPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

// nativeSecretFilesDir is where a native build command reads the service's
// secret files — the same /etc/secrets path the runtime projection mounts, so a
// script shared by build and start finds them in both (docs/ADR013-secrets.md).
const nativeSecretFilesDir = "/etc/secrets"

// nativeFileNamePattern is the Kubernetes Secret key charset (and bex-api's
// ValidSecretFileName). A name is spliced into a Dockerfile mount flag, a
// buildctl --secret CSV value and the preparer's word-split list, so anything
// outside it — separators, quotes, whitespace, globs, slashes — is refused
// rather than escaped.
var nativeFileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validNativeFileName also refuses "." and ".." and the "..data"-style names a
// Secret volume reserves for its own atomic-swap bookkeeping.
func validNativeFileName(name string) bool {
	return nativeFileNamePattern.MatchString(name) && name != "." && !strings.HasPrefix(name, "..")
}

// nativeFileSecretID is the BuildKit secret id carrying the i-th (sorted) file.
// An index rather than the name keeps the id opaque and trivially CSV-safe.
func nativeFileSecretID(i int) string { return fmt.Sprintf("bex-file-%d", i) }

// nativePreparerImage is digest-pinned (codex round-7 F10 / the ADR055 F7
// digest-pinning deferral's highest-value pin): this initContainer mounts and
// reads the ENTIRE runtime-env secret bundle, so a retagged upstream busybox
// would exfiltrate every tenant build secret. The tag is kept readable for
// humans; the digest is the 1.37.0 multi-arch OCI index.
const (
	nativePreparerImage = "busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e"
	nativeNodeRuntime   = "node"
	nativePythonRuntime = "python"
	nativeRubyRuntime   = "ruby"
	nativeElixirRuntime = "elixir"
	// nativeRubyGems is Render's BUNDLE_PATH, beside the source tree.
	nativeRubyGems = "/opt/render/project/.gems"
)

// nativeToolchains is the reviewed set of toolchain lines a native build may
// run on (w8/m51, docs/ADR060 D7 addendum). Every image keeps a readable tag
// but pins its multi-arch manifest identity: patch upgrades are deliberate
// reviewed changes, and a registry retag cannot silently alter a privileged
// tenant build environment. Last-reviewed resolution times live in
// toolchain-freshness.json and must move with each digest. A build selects one
// line from the service's own version signal (nativeversion.go); nothing
// outside this table is ever pulled.
var nativeToolchains = map[string]nativeToolchain{
	nativeElixirRuntime: {label: "Elixir", defaultLine: "1.18", lines: []nativeLine{
		{line: "1.17", version: "1.17.3", otp: "27", image: "elixir:1.17@sha256:0ff6cbfc498e86d21d1f8939672f32acaffe9e5be8924da7942ce69fd34e2c68"},
		{line: "1.18", version: "1.18.4", otp: "28", image: "elixir:1.18@sha256:17e900a5657265d248499a1525b00c64c33e7f93f9a71c766614c8e910656351"},
		{line: "1.19", version: "1.19.6", otp: "28", image: "elixir:1.19@sha256:8f40275227054af01252a5a84c725ad2a34e971f8a5343cb132509fc37b61842"},
		{line: "1.20", version: "1.20.4", otp: "29", image: "elixir:1.20@sha256:c904ce753c35768c76a3be6f16428a83f8237ca0c402355b49bac99d74da2beb"},
	}},
	// Render offers no Go version selection for native services; neither does
	// bex. Render builds with Go's latest stable release, so this line tracks
	// it (w5/m144): the image sets GOTOOLCHAIN=local, so a go.mod declaring a
	// newer `go` than the line refuses to build. The freshness check flags the
	// next release.
	"go": {label: "Go", defaultLine: "1.27", lines: []nativeLine{
		{line: "1.27", image: "golang:1.27-bookworm@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9"},
	}},
	nativeNodeRuntime: {label: "Node", defaultLine: "24", lines: []nativeLine{
		{line: "14", version: "14.21.3", image: "node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e"},
		{line: "16", version: "16.20.2", image: "node:16-bookworm@sha256:73e677f85fbc56d22c817b4e5d7f72086c5343c0859f3bf8aaadab22aab29ce6"},
		{line: "18", version: "18.20.8", image: "node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783"},
		{line: "20", version: "20.20.2", image: "node:20-bookworm@sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5"},
		{line: "22", version: "22.23.4", image: "node:22-bookworm@sha256:18a6d90fd60542269850c80ca03b29b1d9b14d6b221ac7ee8e35b0d33ef904bd"},
		{line: "24", version: "24.22.1", image: "node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea"},
		{line: "26", version: "26.12.1", image: "node:26-bookworm@sha256:3b764117159bda01d2396391e7789b6a9106af9729cfd001ef55e98557868310"},
	}},
	nativePythonRuntime: {label: "Python", defaultLine: "3.13", lines: []nativeLine{
		{line: "3.7", version: "3.7.17", image: "python:3.7-bullseye@sha256:f36f6fe9ddb2622aab9803944cbe7e2bc50be75d624a081f7895e60e7c6b7f0a"},
		{line: "3.8", version: "3.8.20", image: "python:3.8-bookworm@sha256:d411270700143fa2683cc8264d9fa5d3279fd3b6afff62ae81ea2f9d070e390c"},
		{line: "3.9", version: "3.9.25", image: "python:3.9-bookworm@sha256:ff12c273af1e1814efcd8dfdefe16a70a4d901afe94dce721ffed8a34176f285"},
		{line: "3.10", version: "3.10.22", image: "python:3.10-bookworm@sha256:f72a86806f92fe13a49f49aeca1db0b19ea5cf4a0470b842a01b0e14314945b8"},
		{line: "3.11", version: "3.11.18", image: "python:3.11-bookworm@sha256:84fa927127b46670e768371d1ad1be271d57ba93a6fbb3479045c7c6c452432f"},
		{line: "3.12", version: "3.12.16", image: "python:3.12-bookworm@sha256:a5bab0ff300f91231c2e68bf73a8a8d60762ac39ce1c50eaf6803e4c38a9062b"},
		{line: "3.13", version: "3.13.18", image: "python:3.13-bookworm@sha256:3856e3e7bf4d0ba061501a84ed2eb0d429663307c0f54f191dc4de03c7081c3e"},
		{line: "3.14", version: "3.14.10", image: "python:3.14-bookworm@sha256:a8cf203c684b61fd2676ec5a17159cf68ed38ad471481e1f4ebc4679825250bd"},
	}},
	nativeRubyRuntime: {label: "Ruby", defaultLine: "3.4", lines: []nativeLine{
		{line: "3.0", version: "3.0.7", image: "ruby:3.0-bullseye@sha256:dcb8a8a6ee981f949da505cb9f4f15cde696e7bcb088955bc1f92f23a4cd7a72"},
		{line: "3.1", version: "3.1.7", image: "ruby:3.1-bookworm@sha256:91627f55e8969006aab67d15c92fb930500ff73948803da1330b8a853fecebb5"},
		{line: "3.2", version: "3.2.11", image: "ruby:3.2-bookworm@sha256:bf13a46a498fb4055d0e52495c351a2648c8bc8d41b473ee3f734041a74c7f56"},
		{line: "3.3", version: "3.3.13", image: "ruby:3.3-bookworm@sha256:0e4ba04b135a870aedf21340f33ce376c7fe0ed2d9a02886420c820acb3e9c50"},
		{line: "3.4", version: "3.4.12", image: "ruby:3.4-bookworm@sha256:8853c6f24866429483fce3a869e61a6a13916beb2720468c6d95c78f41c0f564"},
		{line: "4.0", version: "4.0.9", image: "ruby:4.0-bookworm@sha256:470eb587e87c42ad9b0062ba9bc362d90d94fb67b09fecd6052b381d897d0777"},
	}},
	// Rust stays one stable image: rustup inside it applies Render's own
	// RUSTUP_TOOLCHAIN / rust-toolchain signals itself.
	"rust": {label: "Rust", defaultLine: "1", lines: []nativeLine{
		{line: "1", image: "rust:1-bookworm@sha256:9181b56c2a21ab1d5e73cffe04fc762426d30aa7fac3c91141dd033f49a7fc8b"},
	}},
}

// nativeBun is the reviewed Bun companion a native build adds beside its
// primary toolchain when the service opts in (BUN_VERSION, .bun-version or a
// Bun lockfile; Render's Bun version contract, w8/m71). Its binary is copied
// out of the pinned image, so build, pre-deploy and start all run with it.
var nativeBun = nativeToolchain{label: "Bun", defaultLine: "1.4", lines: []nativeLine{
	{line: "1.2", version: "1.2.23", image: "oven/bun:1.2@sha256:6ebf306367da43ad75c4d5119563e24de9b66372929ad4fa31546be053a16f74"},
	{line: "1.3", version: "1.3.14", image: "oven/bun:1.3@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4"},
	{line: "1.4", version: "1.4.4", image: "oven/bun:1.4@sha256:1d22548f177f4ff38e0c15baa3c2c46c6c2060ba2d1bc573bb4720b407548f1d"},
}}

// nativeToolchain is one runtime's reviewed lines, ascending.
type nativeToolchain struct {
	label       string
	defaultLine string
	lines       []nativeLine
}

// nativeLine is one pinned toolchain line. version is the exact toolchain the
// pinned image carries — what a version range is matched against; empty for a
// runtime that offers no selection. otp is an Elixir line's Erlang/OTP major.
type nativeLine struct {
	line, version, otp, image string
	// upstream retains the reviewed official input when image is a verified
	// platform derivative. Exact-patch lookup must still use the official tag.
	upstream string
}

func (t nativeToolchain) find(line string) (nativeLine, bool) {
	for _, l := range t.lines {
		if l.line == line {
			return l, true
		}
	}
	return nativeLine{}, false
}

// defaultImage is the line a build without a version signal runs on.
func (t nativeToolchain) defaultImage() string {
	l, _ := t.find(t.defaultLine)
	return l.image
}

// nativeRuntime resolves the toolchain a native build runs in. A static
// site's build usually declares no runtime — Render's static build
// environment is Node-based, so node is the default toolchain there.
func nativeRuntime(o Options) string {
	if o.StaticSite && o.Runtime == "" {
		return nativeNodeRuntime
	}
	return o.Runtime
}

func validateNativeOptions(o Options) error {
	runtime := nativeRuntime(o)
	if _, ok := nativeToolchains[runtime]; !ok {
		return fmt.Errorf("build: unsupported native runtime %q", runtime)
	}
	if strings.TrimSpace(o.BuildCommand) == "" {
		return fmt.Errorf("build: native runtime %s requires buildCommand", runtime)
	}
	// A static site's image is only the publish Job's extract source — it is
	// never run, so no start command exists or is required.
	if !o.StaticSite && strings.TrimSpace(o.StartCommand) == "" {
		return fmt.Errorf("build: native runtime %s requires startCommand", runtime)
	}
	if rev := strings.TrimSpace(o.NativeEnvRevision); rev != "" && !nativeEnvRevisionPattern.MatchString(rev) {
		return fmt.Errorf("build: native env revision %q is not opaque cache metadata", rev)
	}
	return validateSecretFiles(o)
}

// validateSecretFiles checks the secret-file list a native or Dockerfile build
// mounts: a Secret and a list together, names in the Secret-key charset, and
// strictly ascending, which keeps the build deterministic for one file set and
// rules out duplicates.
func validateSecretFiles(o Options) error {
	if (o.NativeFilesSecret == "") != (len(o.NativeFiles) == 0) {
		return fmt.Errorf("build: secret files need both a Secret and a file list")
	}
	for i, name := range o.NativeFiles {
		if !validNativeFileName(name) {
			return fmt.Errorf("build: invalid secret file name %q", name)
		}
		if i > 0 && o.NativeFiles[i-1] >= name {
			return fmt.Errorf("build: secret files must be sorted and unique")
		}
	}
	return nil
}

// BuildLiterals is the literal spec.env a native or Dockerfile build receives,
// sorted by name with Kubernetes' last-duplicate-wins order kept. PORT is
// operator-owned at runtime (appEnv drops a user PORT) and BEX_NATIVE_* names
// are the native preparer's own, so neither is a build input.
func BuildLiterals(env []corev1.EnvVar) []corev1.EnvVar {
	out := make([]corev1.EnvVar, 0, len(env))
	for _, item := range env {
		if item.Name == "" || item.ValueFrom != nil || item.Name == "PORT" || strings.HasPrefix(item.Name, "BEX_NATIVE_") {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// indexedLiteralEnv carries literals under private indexed names, never their
// own: a literal PATH (or any other name the shell reads) must reach the build,
// not become the build container's own environment and break its sh (w8/m61).
// It returns those carriers and the newline-joined key list a script walks.
func indexedLiteralEnv(literals []corev1.EnvVar, prefix string) ([]corev1.EnvVar, string) {
	keys := make([]string, 0, len(literals))
	env := make([]corev1.EnvVar, 0, len(literals))
	for i, item := range literals {
		keys = append(keys, item.Name)
		env = append(env, corev1.EnvVar{Name: prefix + strconv.Itoa(i), Value: execution.KubernetesLiteral(item.Value)})
	}
	return env, execution.KubernetesLiteral(strings.Join(keys, "\n"))
}

// nativeEnvRevision returns the opaque BuildKit cache key embedded in the
// generated Dockerfile. Empty Options default to NativeEnvNoneRevision.
func nativeEnvRevision(o Options) string {
	rev := strings.TrimSpace(o.NativeEnvRevision)
	if rev == "" {
		return NativeEnvNoneRevision
	}
	return rev
}

// nativeDockerfile translates Render's native runtime contract into a
// reproducible OCI build: select the language toolchain, run the caller's exact
// build command with service env available, and retain the exact start command
// as the image CMD. The generated file is written by an init container and is
// never committed to the tenant repository.
//
// BuildKit secret mounts do not participate in the instruction cache key
// (https://docs.docker.com/build/cache/invalidation/#build-secrets). The opaque
// NativeEnvRevision is therefore written into the env loader the RUN sources,
// so a changed effective environment cannot reuse a layer baked under
// different values (w7/m87). The revision carries no secret material.
//
// Each secret file rides its own transient secret mount at
// /etc/secrets/<name> (w4/m163): BuildKit mounts it only for this RUN and never
// commits it to a layer, the image config or the build context. The mount flags
// carry file NAMES only; the revision above also covers file bytes, since a
// changed file would otherwise reuse the cached RUN just like a changed env
// value. required=true makes a missing transport fail the build instead of
// silently running without the file.
// The env-decoding loop lives in its own file, written by a COPY heredoc just
// before the RUN, so BuildKit's step line and its "process … did not complete"
// error name only the tenant's command behind one short source line (w8/052)
// instead of bex's plumbing. The loop's sentinel `.` keeps command substitution
// from stripping value newlines. The revision line rides in the helper: a
// changed helper misses the COPY cache and therefore the RUN after it, exactly
// as the RUN-embedded revision did.
const (
	nativeEnvSecretPath   = "/run/secrets/render-env"
	nativeEnvLoaderPath   = "/opt/bex/load-env"
	nativeEnvLoaderEOF    = "BEX_LOAD_ENV"
	nativeEnvLoaderPrefix = ". " + nativeEnvLoaderPath + "\n"
)

// nativeLiteralEnvPrefix names the preparer's carrier for literal i; the
// BEX_NATIVE_ prefix is refused as a user name, so carriers cannot collide.
const nativeLiteralEnvPrefix = "BEX_NATIVE_LITERAL_"

func nativeDockerfile(o Options) string {
	var fileMounts strings.Builder
	for i, name := range o.NativeFiles {
		fmt.Fprintf(&fileMounts, " --mount=type=secret,id=%s,target=%s/%s,required=true",
			nativeFileSecretID(i), nativeSecretFilesDir, name)
	}
	// The decoder is resolved once, under the image's PATH, into a function a
	// decoded variable cannot overwrite: a decoded PATH is exported mid-loop
	// and must reach the command exactly, not stop the next decode (w8/m61).
	loader := fmt.Sprintf(": bex-native-env-rev=%s\n", nativeEnvRevision(o)) + `decoder="$(command -v base64)" || exit 1
eval "bex_decode() { '$decoder' -d; }"
while IFS= read -r record; do
  [ -n "$record" ] || continue
  key=${record%%=*}
  encoded=${record#*=}
  value="$(printf '%s' "$encoded" | bex_decode 2>/dev/null && printf '.')" || exit 1
  case $key in ''|[0-9]*|*[!A-Za-z0-9_]*) continue ;; esac # a dotted name reaches the container, not the shell
  export "$key=${value%.}"
done < ` + nativeEnvSecretPath + "\n"
	installLines, installStep := nativeStaticInstall(o)
	run := shellJSON(nativeEnvLoaderPrefix + installStep + o.BuildCommand)
	stages, hugo := nativeStaticTools(o)
	base := fmt.Sprintf(`# syntax=docker/dockerfile:1.7
%sFROM %s
%sWORKDIR /opt/render/project/src
COPY . .
COPY <<'%s' %s
%s%s
%sARG RENDER_GIT_COMMIT
RUN --mount=type=secret,id=render-env,target=%s%s %s
`, stages, nativeToolchains[nativeRuntime(o)].defaultImage(), nativeNodeTools(o)+nativeMediaTools+hugo+nativeBuildDefaults(o), nativeEnvLoaderEOF, nativeEnvLoaderPath, loader, nativeEnvLoaderEOF,
		installLines, nativeEnvSecretPath, fileMounts.String(), run)
	if o.StaticSite {
		// No PORT/CMD: the image only carries the built site for the publish
		// Job's extract initContainer (ADR029) and never runs as a workload.
		return base
	}
	// nativeEntrypoint replaces the toolchain image's ENTRYPOINT (w8/m61).
	start := shellJSON(o.StartCommand)
	return base + fmt.Sprintf(`%sENV PORT=10000
%sCMD %s
`, nativeRuntimeDefaults(o), nativeEntrypoint(o), start)
}

// nativeBuildDefaults is the environment Render gives a native runtime at
// build and runtime alike, as image ENV before the build RUN: the env loader
// exports the service's own values over it for the build, and the container's
// configured values override it at runtime, an explicit empty one included in
// either phase. An explicit Elixir, Python or Ruby static build gets it for
// its build.
//
// Elixir builds and runs with MIX_ENV=prod (w8/112), so a Phoenix app's
// production dependencies and config are the ones compiled and started.
// Ruby does the same with RAILS_ENV=production (w8/114): in development
// `rails server` binds localhost only, which no readiness probe or request
// reaches; in production it binds every interface on PORT. It also gets
// Render's Bundler settings (w8/117): deployment mode, so a missing lock or
// one that no longer matches its Gemfile fails the build instead of being
// re-resolved, and Render's gem directory outside the source tree with
// binstubs first on PATH — deployment mode alone would install into the
// app's vendor/bundle, where a static site publishing its root or a Jekyll
// build would pick the gems up, and a bare `puma` would no longer resolve.
// Python gets Render's Gunicorn flags, preloading the app and writing access
// lines to stdout (w8/113). Render's --bind is left out: Gunicorn binds the
// platform's PORT by default, which a bind in gunicorn.conf.py or on the
// command line still replaces, as it would without these flags. It also gets
// Render's FORWARDED_ALLOW_IPS=* (w8/116): requests arrive from the ingress,
// never loopback, so without it Gunicorn (and Uvicorn, which reads the same
// variable) ignores X-Forwarded-Proto and an HTTPS request reaches the app
// as http. The ingress replaces the client's forwarding headers with its own,
// and only it and the platform's namespaces reach a pod from outside its
// workspace.
func nativeBuildDefaults(o Options) string {
	switch nativeRuntime(o) {
	case "elixir":
		return "ENV MIX_ENV=prod\n"
	case nativePythonRuntime:
		return `ENV GUNICORN_CMD_ARGS="--preload --access-logfile -"` + "\n" +
			`ENV FORWARDED_ALLOW_IPS="*"` + "\n"
	case nativeRubyRuntime:
		// Render's Ruby defaults, build and runtime alike (w5/162): Rails
		// serves its own assets and logs to stdout only when told to, so
		// without these a migrated app 404s its CSS/JS and its log lines
		// never reach the Logs tab. PASSENGER_PORT is left out: Render pins it
		// to its own default port, while a bex service's port is PORT. So is
		// GEM_PATH: setting it drops RubyGems' default directory, hiding the
		// gems Ruby ships (rake, minitest), and BUNDLE_PATH already decides
		// where Bundler installs.
		return "ENV RAILS_ENV=production RAILS_SERVE_STATIC_FILES=true RAILS_LOG_TO_STDOUT=true\n" +
			"ENV MALLOC_ARENA_MAX=2 PIDFILE=/tmp/puma-server.pid\n" +
			"ENV PASSENGER_ENGINE=builtin PASSENGER_ENVIRONMENT=production\n" +
			"ENV BUNDLE_DEPLOYMENT=true BUNDLE_PATH=" + nativeRubyGems + " BUNDLE_BIN=" + nativeRubyGems + "/bin" +
			" BUNDLE_APP_CONFIG=" + nativeRubyGems + "\n" +
			"ENV PATH=" + nativeRubyGems + "/bin:$PATH\n"
	}
	return ""
}

// nativeRuntimeDefaults is the environment Render gives a native runtime's
// running service by default, as image ENV after the build RUN: the build does
// not see it, and as image configuration it yields to every value the service
// configures — a literal, a linked group, an explicit empty one. Node runs
// with NODE_ENV=production at runtime only (w8/111), so Express and its peers
// leave development mode.
func nativeRuntimeDefaults(o Options) string {
	if nativeRuntime(o) == nativeNodeRuntime {
		return "ENV NODE_ENV=production\n"
	}
	return ""
}

// shellJSON wraps a Render build/start command in a NON-login bash so the
// toolchain image's own PATH survives. A login shell (`-lc`) sources Debian's
// /etc/profile, which unconditionally overwrites PATH with the fixed system
// default — dropping the toolchain directories the official images add via ENV
// (golang's /usr/local/go/bin, rust's /usr/local/cargo/bin), so `go`/`cargo`
// resolved to "command not found". Interpreted runtimes only appeared to work
// because their binaries happen to live in /usr/local/bin, which is on that
// default. `-c` keeps the image ENV PATH exactly as the runtime image declares.
func shellJSON(command string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode([]string{"/bin/bash", "-c", command})
	return strings.TrimSpace(out.String())
}

// nativeBuildPreparer materializes the generated Dockerfile and a base64 env
// bundle into an EmptyDir shared with buildkit. The env bundle enters BuildKit
// through a secret mount, so neither literal nor OpenBao-backed values appear in
// image metadata or the generated Dockerfile. Secret files are copied byte for
// byte from the App-owned files Secret into /native/files at the same moment, so
// one build reads one consistent snapshot of both inputs even if a later
// reconcile rewrites the projection mid-build; a declared file that is absent
// fails the preparer rather than the tenant's command.
func nativeBuildPreparer(o Options) corev1.Container {
	env, keys := indexedLiteralEnv(BuildLiterals(o.BuildEnv), nativeLiteralEnvPrefix)
	env = append(env,
		corev1.EnvVar{Name: "BEX_NATIVE_DOCKERFILE", Value: execution.KubernetesLiteral(nativeDockerfile(o))},
		corev1.EnvVar{Name: "BEX_NATIVE_LITERAL_KEYS", Value: keys},
		corev1.EnvVar{Name: "BEX_NATIVE_FILES", Value: execution.KubernetesLiteral(strings.Join(o.NativeFiles, "\n"))},
	)

	mounts := []corev1.VolumeMount{{Name: "native-build", MountPath: "/native"}}
	if o.RuntimeEnvSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: runtimeEnvVolume, MountPath: "/runtime-env", ReadOnly: true})
	}
	if o.NativeFilesSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: secretFilesVolume, MountPath: "/native-files", ReadOnly: true})
	}
	return corev1.Container{
		Name:    "prepare-native-build",
		Image:   nativePreparerImage,
		Command: []string{"sh", "-eu", "-c"},
		Args: []string{`umask 077
printf '%s' "$BEX_NATIVE_DOCKERFILE" > /native/Dockerfile
: > /native/render-env
if [ -d /runtime-env ]; then
  for path in /runtime-env/*; do
    [ -f "$path" ] || continue
    key="$(basename "$path")"
    encoded="$(base64 < "$path" | tr -d '\n')"
    printf '%s=%s\n' "$key" "$encoded" >> /native/render-env
  done
fi
i=0
for key in $BEX_NATIVE_LITERAL_KEYS; do
  # The sentinel guards value newlines through command substitution; %?. then
  # drops it together with printenv's own final terminator, nothing else.
  value="$(printenv "` + nativeLiteralEnvPrefix + `$i"; printf '.')"
  i=$((i + 1))
  encoded="$(printf '%s' "${value%?.}" | base64 | tr -d '\n')"
  printf '%s=%s\n' "$key" "$encoded" >> /native/render-env
done
mkdir /native/files
# Names are validated to the Secret-key charset, so word splitting is exact.
for name in $BEX_NATIVE_FILES; do
  cp "/native-files/$name" "/native/files/$name"
done`},
		Env:          env,
		VolumeMounts: mounts,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                new(int64(65532)),
			RunAsGroup:               new(int64(65532)),
			RunAsNonRoot:             new(true),
			AllowPrivilegeEscalation: new(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
}
