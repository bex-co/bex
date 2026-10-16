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
	"elixir": {label: "Elixir", defaultLine: "1.18", lines: []nativeLine{
		{line: "1.17", version: "1.17.3", otp: "27", image: "elixir:1.17@sha256:c65f0bdd8827df171963e3558a1461b04efc086d0d52ee7c6900fcddb4467e08"},
		{line: "1.18", version: "1.18.4", otp: "28", image: "elixir:1.18@sha256:a68defc35f664d1d02de0dc36b12e750bd558b594eed3d0909534317796df749"},
		{line: "1.19", version: "1.19.6", otp: "28", image: "elixir:1.19@sha256:db1cbca0b49e26e65dd804c75f27ec5f01835f1d29f8b8e4cc8a51f01f986f9f"},
	}},
	// Render offers no Go version selection for native services; neither does bex.
	"go": {label: "Go", defaultLine: "1.24", lines: []nativeLine{
		{line: "1.24", image: "golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac"},
	}},
	nativeNodeRuntime: {label: "Node", defaultLine: "24", lines: []nativeLine{
		{line: "22", version: "22.23.3", image: "node:22-bookworm@sha256:0e5f906573693feaa1e21057ebdcfdb5bd5021f050b2dc7c9deceb629c7da2a8"},
		{line: "24", version: "24.22.0", image: "node:24-bookworm@sha256:14078204b6bb268c107ceff6d3c43f45a888105e5c2b18b14c32b3409d93a1b0"},
		{line: "26", version: "26.12.0", image: "node:26-bookworm@sha256:a856ae5e25d498bbe5ec85fde88f00f5ed99908fda803ba39d7735ab8574f58c"},
	}},
	"python": {label: "Python", defaultLine: "3.13", lines: []nativeLine{
		{line: "3.10", version: "3.10.22", image: "python:3.10-bookworm@sha256:f72a86806f92fe13a49f49aeca1db0b19ea5cf4a0470b842a01b0e14314945b8"},
		{line: "3.11", version: "3.11.17", image: "python:3.11-bookworm@sha256:5887f265d8d44d8b4734d3658b21d668edf5aaf92e4945bc0c984afbfab105d3"},
		{line: "3.12", version: "3.12.15", image: "python:3.12-bookworm@sha256:5560e9ab8709f459489e5b8aa696eda8a07ef821e14bb122be62d91234bfa98b"},
		{line: "3.13", version: "3.13.16", image: "python:3.13-bookworm@sha256:073ffebb96ae4d0ed73ccad59f08c47bd84af8a79f5258130f8458ac50ad9ff4"},
		{line: "3.14", version: "3.14.8", image: "python:3.14-bookworm@sha256:7ee7e4d4fb42c3ad45b8fdc473b64ec69c3cf6e80ae5d52f6fa55f77fac29027"},
	}},
	"ruby": {label: "Ruby", defaultLine: "3.4", lines: []nativeLine{
		{line: "3.3", version: "3.3.12", image: "ruby:3.3-bookworm@sha256:b899e7de6ffa6e974d947202d3ce2ac921c12e46c14cff3106214079388dcff4"},
		{line: "3.4", version: "3.4.11", image: "ruby:3.4-bookworm@sha256:80259ba83f866aaa43c689e11569dc950b2e508effca0799d50608f7c4ca6172"},
		{line: "4.0", version: "4.0.7", image: "ruby:4.0-bookworm@sha256:7671dc16cd740b9d8d653a568d8bd2df00e24a32c53a7b7a3fc39a0a194bb9ad"},
	}},
	// Rust stays one stable image: rustup inside it applies Render's own
	// RUSTUP_TOOLCHAIN / rust-toolchain signals itself.
	"rust": {label: "Rust", defaultLine: "1", lines: []nativeLine{
		{line: "1", image: "rust:1-bookworm@sha256:114c7a4425406451c2866b6aafe69fe29b1b298832db1277d411ac73c82d04d6"},
	}},
}

// nativeBun is the reviewed Bun companion a native build adds beside its
// primary toolchain when the service opts in (BUN_VERSION, .bun-version or a
// Bun lockfile; Render's Bun version contract, w8/m71). Its binary is copied
// out of the pinned image, so build, pre-deploy and start all run with it.
var nativeBun = nativeToolchain{label: "Bun", defaultLine: "1.3", lines: []nativeLine{
	{line: "1.2", version: "1.2.23", image: "oven/bun:1.2@sha256:6ebf306367da43ad75c4d5119563e24de9b66372929ad4fa31546be053a16f74"},
	{line: "1.3", version: "1.3.14", image: "oven/bun:1.3@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4"},
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
	if (o.NativeFilesSecret == "") != (len(o.NativeFiles) == 0) {
		return fmt.Errorf("build: native secret files need both a Secret and a file list")
	}
	for i, name := range o.NativeFiles {
		if !validNativeFileName(name) {
			return fmt.Errorf("build: invalid native secret file name %q", name)
		}
		// Strictly ascending keeps the generated Dockerfile deterministic for one
		// file set, and rules out duplicates.
		if i > 0 && o.NativeFiles[i-1] >= name {
			return fmt.Errorf("build: native secret files must be sorted and unique")
		}
	}
	return nil
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
  export "$key=${value%.}"
done < ` + nativeEnvSecretPath + "\n"
	run := shellJSON(nativeEnvLoaderPrefix + o.BuildCommand)
	base := fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s
%sWORKDIR /opt/render/project/src
COPY . .
COPY <<'%s' %s
%s%s
RUN --mount=type=secret,id=render-env,target=%s%s %s
`, nativeToolchains[nativeRuntime(o)].defaultImage(), nativeNodeTools(o), nativeEnvLoaderEOF, nativeEnvLoaderPath, loader, nativeEnvLoaderEOF,
		nativeEnvSecretPath, fileMounts.String(), run)
	if o.StaticSite {
		// No PORT/CMD: the image only carries the built site for the publish
		// Job's extract initContainer (ADR029) and never runs as a workload.
		return base
	}
	// The toolchain image's own ENTRYPOINT (node's docker-entrypoint.sh) is
	// looked up on the user's PATH, which may be empty or narrow; the absolute
	// bash CMD needs none of it, so the generated image runs that alone (w8/m61).
	start := shellJSON(o.StartCommand)
	return base + fmt.Sprintf(`ENV PORT=10000
ENTRYPOINT []
CMD %s
`, start)
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
	// Literals ride private indexed names, never their own: a literal PATH (or
	// any other name the shell reads) must reach the build command, not become
	// the preparer's own environment and break its sh and codec lookup
	// (w8/m61). The stable sort keeps Kubernetes' last-duplicate-wins order.
	literals := make([]corev1.EnvVar, 0, len(o.BuildEnv))
	for _, item := range o.BuildEnv {
		if item.Name == "" || item.ValueFrom != nil || item.Name == "PORT" || strings.HasPrefix(item.Name, "BEX_NATIVE_") {
			continue
		}
		literals = append(literals, item)
	}
	sort.SliceStable(literals, func(i, j int) bool { return literals[i].Name < literals[j].Name })
	keys := make([]string, 0, len(literals))
	env := make([]corev1.EnvVar, 0, len(literals)+3)
	for i, item := range literals {
		keys = append(keys, item.Name)
		env = append(env, corev1.EnvVar{Name: nativeLiteralEnvPrefix + strconv.Itoa(i), Value: item.Value})
	}
	env = append(env,
		corev1.EnvVar{Name: "BEX_NATIVE_DOCKERFILE", Value: nativeDockerfile(o)},
		corev1.EnvVar{Name: "BEX_NATIVE_LITERAL_KEYS", Value: strings.Join(keys, "\n")},
		corev1.EnvVar{Name: "BEX_NATIVE_FILES", Value: strings.Join(o.NativeFiles, "\n")},
	)

	mounts := []corev1.VolumeMount{{Name: "native-build", MountPath: "/native"}}
	if o.RuntimeEnvSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: "runtime-env", MountPath: "/runtime-env", ReadOnly: true})
	}
	if o.NativeFilesSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: "native-files", MountPath: "/native-files", ReadOnly: true})
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
