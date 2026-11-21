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
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
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

// buildkitMaxSecretSize is the most bytes buildctl hands BuildKit as one
// secret: its session store refuses a larger --secret source as "too big"
// (moby/buildkit session/secrets/secretsprovider MaxSecretSize, 500 KiB), and
// that check runs when buildctl starts, so one such file fails every build. A
// secret file may be 1 MiB (w4/m363), so a native build carries a larger file
// in parts of at most this size (w4/699).
const buildkitMaxSecretSize = 500 * 1024

// nativeFileParts is how many BuildKit secrets carry the named file: one up to
// buildkitMaxSecretSize, else as many parts of that size as its bytes need.
func nativeFileParts(o Options, name string) int {
	return max(1, (o.NativeFileSizes[name]+buildkitMaxSecretSize-1)/buildkitMaxSecretSize)
}

// nativeFileSecretIDs is the BuildKit secret ids carrying the i-th file, one
// per part in order: nativeFileSecretID for a file that fits one secret,
// bex-file-<i>-<part> for a split one.
func nativeFileSecretIDs(o Options, i int, name string) []string {
	parts := nativeFileParts(o, name)
	if parts == 1 {
		return []string{nativeFileSecretID(i)}
	}
	ids := make([]string, parts)
	for p := range ids {
		ids[p] = fmt.Sprintf("bex-file-%d-%d", i, p)
	}
	return ids
}

// nativeFilesSplit reports whether any secret file of the build is carried in
// parts. Only then does the build assemble /etc/secrets itself
// (nativeFileMounts); otherwise each file mounts there directly, as before.
func nativeFilesSplit(o Options) bool {
	for _, name := range o.NativeFiles {
		if nativeFileParts(o, name) > 1 {
			return true
		}
	}
	return false
}

// nativeFilePartsDir is where the preparer cuts a split file into parts
// (/native/file-parts/<name>/<p>), and nativeFilePartsMount where the build RUN
// mounts the parts of the i-th file (<mount>/<i>/<p>).
const (
	nativeFilePartsDir   = "/native/file-parts"
	nativeFilePartsMount = "/run/bex-files"
)

// nativeFileMounts is the build RUN's secret-file mount flags and, for a build
// that carries a file in parts, the loader lines that put each file back
// together. Without a split file, each file rides its own secret mount at
// /etc/secrets/<name>. With one, /etc/secrets is a tmpfs mount for this RUN
// only, every file's secrets mount under nativeFilePartsMount, and the loader
// concatenates each file's parts, in order, into /etc/secrets/<name> before the
// tenant's command runs: the bytes still never reach a layer, the image config
// or the build context (w4/699).
func nativeFileMounts(o Options) (mounts, assemble string) {
	var flags, lines strings.Builder
	split := nativeFilesSplit(o)
	if split {
		flags.WriteString(" --mount=type=tmpfs,target=" + nativeSecretFilesDir)
	}
	for i, name := range o.NativeFiles {
		if !split {
			fmt.Fprintf(&flags, " --mount=type=secret,id=%s,target=%s/%s,required=true",
				nativeFileSecretID(i), nativeSecretFilesDir, name)
			continue
		}
		lines.WriteString("cat")
		for p, id := range nativeFileSecretIDs(o, i, name) {
			target := fmt.Sprintf("%s/%d/%d", nativeFilePartsMount, i, p)
			fmt.Fprintf(&flags, " --mount=type=secret,id=%s,target=%s,required=true", id, target)
			lines.WriteString(" " + target)
		}
		fmt.Fprintf(&lines, " > %s/%s || exit 1\n", nativeSecretFilesDir, name)
	}
	return flags.String(), lines.String()
}

// nativePreparerImage is digest-pinned (codex round-7 F10 / the ADR055 F7
// digest-pinning deferral's highest-value pin): this initContainer mounts and
// reads the ENTIRE runtime-env secret bundle, so a retagged upstream busybox
// would exfiltrate every tenant build secret. The tag is kept readable for
// humans; the digest is the 1.37.0 multi-arch OCI index.
const (
	nativePreparerImage = "busybox:1.37.0@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec"
	nativeNodeRuntime   = "node"
	nativePythonRuntime = "python"
	nativeRubyRuntime   = "ruby"
	nativeElixirRuntime = "elixir"
	nativeRustRuntime   = "rust"
	// nativeRubyGems is Render's BUNDLE_PATH and GEM_PATH, beside the source
	// tree.
	nativeRubyGems = "/opt/render/project/.gems"
	// nativeRubyGemHome is the selected Ruby's Gem.default_dir, resolved by
	// the Dockerfile frontend from the image's RUBY_VERSION (nativeBuildDefaults).
	nativeRubyGemHome = "/usr/local/lib/ruby/gems/${RUBY_VERSION%.*}.0"
	// Render's GOPATH, CARGO_HOME and RUSTUP_HOME (nativeBuildDefaults).
	nativeGoPath     = "/opt/render/project/go"
	nativeCargoHome  = "/opt/render/project/.cargo"
	nativeRustupHome = "/opt/render/project/.rustup"
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
		{line: "1.17", version: "1.17.3", otp: "27", image: "elixir:1.17@sha256:08879d1eff5207a26ecab32ac7a7c5df610fce532b791eccbae7288a4e4b27b3", debian: "bookworm"},
		{line: "1.18", version: "1.18.6", otp: "28", image: "elixir:1.18@sha256:16382709b3765b946748714575d8748ea7dcb7cb9a4849730bcdc56533503691", debian: "trixie"},
		{line: "1.19", version: "1.19.7", otp: "28", image: "elixir:1.19@sha256:7e27df3e4a684163e4a18e618eede53badb70e97e0c92a1845ab22e2e1d96da2", debian: "trixie"},
		{line: "1.20", version: "1.20.5", otp: "29", image: "elixir:1.20@sha256:1b850191faeaae55678b782cd8b34d391f793672f3175b1956d024f794ca321b", debian: "trixie"},
	}},
	// Render offers no Go version selection for native services; neither does
	// bex. Render builds with Go's latest stable release, so this line tracks
	// it (w5/m144): the image sets GOTOOLCHAIN=local, so a go.mod declaring a
	// newer `go` than the line refuses to build. The freshness check flags the
	// next release.
	"go": {label: "Go", defaultLine: "1.27", lines: []nativeLine{
		{line: "1.27", image: "golang:1.27-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea"},
	}},
	nativeNodeRuntime: {label: "Node", defaultLine: "24", lines: []nativeLine{
		{line: "14", version: "14.21.3", image: "node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e"},
		{line: "16", version: "16.20.2", image: "node:16-bookworm@sha256:73e677f85fbc56d22c817b4e5d7f72086c5343c0859f3bf8aaadab22aab29ce6"},
		{line: "18", version: "18.20.8", image: "node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783"},
		{line: "20", version: "20.20.2", image: "node:20-bookworm@sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5"},
		{line: "22", version: "22.23.5", image: "node:22-bookworm@sha256:e13726ca604184d7a91fcacab1fc09eefd642dc28e79ac34e85540dd93b78cfb"},
		{line: "24", version: "24.22.3", image: "node:24-bookworm@sha256:9d266751ca3d7fdbe651c878ffd5ad0a42fed49a18c283509267c97f025b7496"},
		{line: "26", version: "26.13.1", image: "node:26-bookworm@sha256:b7087bc4d973034b30854b378ba107f9fab622b47514cb7e2592e41b8bab8d49"},
	}},
	nativePythonRuntime: {label: "Python", defaultLine: "3.13", lines: []nativeLine{
		{line: "3.7", version: "3.7.17", image: "python:3.7-bullseye@sha256:f36f6fe9ddb2622aab9803944cbe7e2bc50be75d624a081f7895e60e7c6b7f0a"},
		{line: "3.8", version: "3.8.20", image: "python:3.8-bookworm@sha256:d411270700143fa2683cc8264d9fa5d3279fd3b6afff62ae81ea2f9d070e390c"},
		{line: "3.9", version: "3.9.25", image: "python:3.9-bookworm@sha256:ff12c273af1e1814efcd8dfdefe16a70a4d901afe94dce721ffed8a34176f285"},
		{line: "3.10", version: "3.10.22", image: "python:3.10-bookworm@sha256:f72a86806f92fe13a49f49aeca1db0b19ea5cf4a0470b842a01b0e14314945b8"},
		{line: "3.11", version: "3.11.18", image: "python:3.11-bookworm@sha256:726d304906fa5eb468e5531e366548d1903286352f5adb54ef508ac01fbd80c6"},
		{line: "3.12", version: "3.12.16", image: "python:3.12-bookworm@sha256:4209fae09a3e20afd863e528bfddc235da5fd5977983e1996cd54cc9a72e6267"},
		{line: "3.13", version: "3.13.18", image: "python:3.13-bookworm@sha256:cc55e4ded9c95f6ce2133b3d096db43f214d280c2a8af65d83e246c8fa7d168c"},
		{line: "3.14", version: "3.14.10", image: "python:3.14-bookworm@sha256:3fe825da0a80fa8e2cd79baadeb76b03c0ddf3d6cdbc8346778a0208b9d6038c"},
		{line: "3.15", version: "3.15.2", image: "python:3.15-bookworm@sha256:6f060cab421d9509c6c6a6a7762c651baf03c3f1d0145befe16b688762b7fe81"},
	}},
	nativeRubyRuntime: {label: "Ruby", defaultLine: "3.4", lines: []nativeLine{
		{line: "3.0", version: "3.0.7", image: "ruby:3.0-bullseye@sha256:dcb8a8a6ee981f949da505cb9f4f15cde696e7bcb088955bc1f92f23a4cd7a72"},
		{line: "3.1", version: "3.1.7", image: "ruby:3.1-bookworm@sha256:91627f55e8969006aab67d15c92fb930500ff73948803da1330b8a853fecebb5"},
		{line: "3.2", version: "3.2.11", image: "ruby:3.2-bookworm@sha256:bf13a46a498fb4055d0e52495c351a2648c8bc8d41b473ee3f734041a74c7f56"},
		{line: "3.3", version: "3.3.13", image: "ruby:3.3-bookworm@sha256:ae098d019fd2995e6cb82e16a87529348ba26a7c83ad6189937decc2846b0730"},
		{line: "3.4", version: "3.4.12", image: "ruby:3.4-bookworm@sha256:d7c312a48736dd291d7ce043c847f04dfde9bdf7334491ca6a545a4a184395be"},
		{line: "4.0", version: "4.0.9", image: "ruby:4.0-bookworm@sha256:862aacfe5415f2dcb399677d9139963ef4cd989140096cf723a91256ae8a9de3"},
	}},
	// Rust stays one stable image: rustup inside it applies Render's own
	// RUSTUP_TOOLCHAIN / rust-toolchain signals itself. version is the image's
	// RUST_VERSION, which the default's log line names (w8/327); refresh it
	// with the digest.
	nativeRustRuntime: {label: "Rust", defaultLine: "1", lines: []nativeLine{
		{line: "1", version: "1.100.0", image: "rust:1-bookworm@sha256:ade39a710ef85f3506cffcfec3e8a24ba090badef1dff2fb798f71d878833fea"},
	}},
}

// nativeBun is the reviewed Bun companion a native build adds beside its
// primary toolchain when the service opts in (BUN_VERSION, .bun-version or a
// Bun lockfile; Render's Bun version contract, w8/m71). Its binary is copied
// out of the pinned image, so build, pre-deploy and start all run with it.
var nativeBun = nativeToolchain{label: "Bun", defaultLine: "1.4", floating: nativeBunCanary, lines: []nativeLine{
	{line: "1.2", version: "1.2.23", image: "oven/bun:1.2@sha256:6ebf306367da43ad75c4d5119563e24de9b66372929ad4fa31546be053a16f74"},
	{line: "1.3", version: "1.3.14", image: "oven/bun:1.3@sha256:e10577f0db68676a7024391c6e5cb4b879ebd17188ab750cf10024a6d700e5c4"},
	{line: "1.4", version: "1.4.4", image: "oven/bun:1.4@sha256:1d22548f177f4ff38e0c15baa3c2c46c6c2060ba2d1bc573bb4720b407548f1d"},
}}

// nativeToolchain is one runtime's reviewed lines, ascending.
type nativeToolchain struct {
	label       string
	defaultLine string
	// floating is an upstream tag a service may request that is no reviewed
	// line (Bun's canary): it resolves to the default line's pinned image with
	// Exact set to the tag, which the build then pins to that tag's digest.
	floating string
	lines    []nativeLine
}

// nativeLine is one pinned toolchain line. version is the exact toolchain the
// pinned image carries — what a version range is matched against; empty for a
// runtime that offers no selection, except Rust's, which only the build log
// names (w8/327). otp is an Elixir line's Erlang/OTP major.
type nativeLine struct {
	line, version, otp, image, debian string
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
	// never run, so no start command exists or is required. A build source's
	// image runs only under each linked service's own start command.
	if !o.StaticSite && !o.BuildOnly && strings.TrimSpace(o.StartCommand) == "" {
		return fmt.Errorf("build: native runtime %s requires startCommand", runtime)
	}
	if rev := strings.TrimSpace(o.NativeEnvRevision); rev != "" && !nativeEnvRevisionPattern.MatchString(rev) {
		return fmt.Errorf("build: native env revision %q is not opaque cache metadata", rev)
	}
	if !nativeEnvRevisionPattern.MatchString(nativeReleaseRevision(o)) {
		return fmt.Errorf("build: release revision %q is not opaque cache metadata", o.Revision)
	}
	if err := validateNativeManagedOIDC(o); err != nil {
		return err
	}
	return validateSecretFiles(o)
}

// validateSecretFiles checks the secret-file list a native or Dockerfile build
// mounts: a Secret and a list together, names in the Secret-key charset, and
// strictly ascending, which keeps the build deterministic for one file set and
// rules out duplicates.
func validateSecretFiles(o Options) error {
	if (len(o.NativeFilesSecrets) == 0) != (len(o.NativeFiles) == 0) {
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

// nativeReleaseRevision is the release's build revision (gen-<release
// generation>) the env loader carries beside the env revision, so every
// release runs its build command, as Render does "with each deploy" (w32/046):
// a redeploy of an unchanged commit and environment (Deploy latest commit, a
// deploy hook rebuilding a site for new CMS content) would otherwise be served
// from the layer cache and keep the previous build's output. A retry of the
// same release keeps its key.
func nativeReleaseRevision(o Options) string {
	return cmp.Or(o.Revision, defaultRevision)
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
// different values (w7/m87). The revision carries no secret material. The
// release's own revision rides beside it (nativeReleaseRevision), so the layer
// cache serves the toolchain and tool layers before the command, never the
// command itself; the command reads the build caches ($XDG_CACHE_HOME, the
// dependency directories) instead, as on Render.
//
// Each secret file rides its own transient secret mount at
// /etc/secrets/<name> (w4/m163): BuildKit mounts it only for this RUN and never
// commits it to a layer, the image config or the build context. The mount flags
// carry file NAMES only; the revision above also covers file bytes, since a
// changed file would otherwise reuse the cached RUN just like a changed env
// value. required=true makes a missing transport fail the build instead of
// silently running without the file. A file over BuildKit's 500 KiB secret
// limit rides in parts and is reassembled on a RUN-only tmpfs instead
// (nativeFileMounts, w4/699).
// The env-decoding loop lives in its own file, written by a COPY heredoc just
// before the RUN, so BuildKit's step line and its "process … did not complete"
// error name only the tenant's command behind one short source line (w8/052)
// instead of bex's plumbing. The loop's sentinel `.` keeps command substitution
// from stripping value newlines. The revision line rides in the helper: a
// changed helper misses the COPY cache and therefore the RUN after it, exactly
// as the RUN-embedded revision did.
const (
	nativeEnvSecretPath   = "/run/secrets/render-env"
	nativeEnvRunMount     = "RUN --mount=type=secret,id=render-env,target=" + nativeEnvSecretPath
	nativeEnvLoaderPath   = "/opt/bex/load-env"
	nativeEnvLoaderEOF    = "BEX_LOAD_ENV"
	nativeEnvLoaderPrefix = ". " + nativeEnvLoaderPath + "\n"
)

// nativeOneStream opens the env loader after its no-op revision lines, so the
// RUN's shell writes its stderr to its stdout before anything else runs
// (w8/356). BuildKit carries a step's two streams apart and prints each in
// batches as they reach it, so a burst of the build command's stderr came out
// ahead of the stdout written between it, and a stdout tail with no newline
// was glued to a later stderr line. One descriptor keeps both in the order the
// command wrote them, as a terminal shows them; the step's exit status is
// still the command's. Nothing reads a build step's stderr apart: the build
// pod's log is one stream. The RUN line itself stays the tenant's command
// behind one source line (w8/052).
const nativeOneStream = "exec 2>&1\n"

// nativeLiteralEnvPrefix names the preparer's carrier for literal i; the
// BEX_NATIVE_ prefix is refused as a user name, so carriers cannot collide.
const nativeLiteralEnvPrefix = "BEX_NATIVE_LITERAL_"

func nativeDockerfile(o Options) string {
	fileMounts, assembleFiles := nativeFileMounts(o)
	// The decoder is resolved once, under the image's PATH, into a function a
	// decoded variable cannot overwrite: a decoded PATH is exported mid-loop
	// and must reach the command exactly, not stop the next decode (w8/m61).
	depsMount, depsSeed, depsSave := nativeDepsRecipe(o.depsCache())
	loader := fmt.Sprintf(": bex-native-env-rev=%s\n: bex-native-release=%s\n", nativeEnvRevision(o), nativeReleaseRevision(o)) + nativeOneStream + depsSeed + assembleFiles + `decoder="$(command -v base64)" || exit 1
eval "bex_decode() { '$decoder' -d; }"
while IFS= read -r record; do
  [ -n "$record" ] || continue
  key=${record%%=*}
  encoded=${record#*=}
  value="$(printf '%s' "$encoded" | bex_decode 2>/dev/null && printf '.')" || exit 1
  case $key in ''|[0-9]*|*[!A-Za-z0-9_]*) continue ;; esac # a dotted name reaches the container, not the shell
  export "$key=${value%.}"
done < ` + nativeEnvSecretPath + "\n" + nativeOIDCExports(o) + nativeLockedBundlerStep(o)
	installLines, installStep := nativeStaticInstall(o)
	run := shellJSON(nativeEnvLoaderPrefix + installStep + nativePublishCheckStep(o) + o.BuildCommand)
	hugo := nativeStaticTools(o)
	sourceDir := nativeSourceDir(o)
	base := fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s
%sWORKDIR %s
%sCOPY . .
COPY <<'%s' %s
%s%s
%sARG RENDER_GIT_COMMIT
%s%s %s
%s`, nativeToolchains[nativeRuntime(o)].defaultImage(), nativeNodeTools(o)+nativeMediaTools+hugo+nativeBuildDefaults(o)+nativeXDGCacheDir, sourceDir, nativeSourceLink(sourceDir), nativeEnvLoaderEOF, nativeEnvLoaderPath, loader, nativeEnvLoaderEOF,
		installLines, nativeEnvRunMount, fileMounts+nativeOIDCMounts(o)+nativeXDGCacheMount+depsMount, run, depsSave)
	if o.StaticSite {
		// No PORT/CMD: the image only carries the built site for the publish
		// Job's extract initContainer (ADR029) and never runs as a workload.
		return base
	}
	base += nativeSecretFileLinks(o)
	// nativeEntrypoint replaces the toolchain image's ENTRYPOINT (w8/m61).
	runtime := fmt.Sprintf("%sENV PORT=10000\n%s", nativeRuntimeDefaults(o), nativeEntrypoint(o))
	if o.BuildOnly {
		return base + runtime
	}
	return base + runtime + "CMD " + shellJSON(o.StartCommand) + "\n"
}

// nativeSecretFileLinks is the step that puts each secret file in the service
// root as well as /etc/secrets, where Render has it: "For non-Docker services,
// the file is also available in your service's root directory" (w8/m154). The
// service root is the image's WORKDIR: the build context is the service's
// rootDir.
//
// Each file is a link to its /etc/secrets path, made after the build command.
// The image therefore carries names only, never a secret's bytes, and the
// build command cannot copy them into its output; the link resolves in every
// workload that runs this image with the files mounted (the service, its cron
// runs and its pre-deploy command). A repository file of the same name gives
// way to the secret file; a directory of that name is left alone, and the file
// is then at /etc/secrets only. Names are in the Secret-key charset
// (validNativeFileName), so they need no quoting. No files, no step.
//
// A deploy that reruns this image with other files (a group linked or
// unlinked, a file added to a linked group: w8/348) mounts names the image has
// no link for. native-init, the image's entrypoint, links what is mounted at
// every start and drops a link to a file no longer mounted (cmd/native-init).
// Every workload of the image starts under it, a cron's own command and the
// pre-deploy command included (w8/350); this step still covers an image whose
// entrypoint is not native-init (a build without the resolve phase).
func nativeSecretFileLinks(o Options) string {
	if len(o.NativeFiles) == 0 {
		return ""
	}
	// set -e: a link that cannot be made fails the build, whichever name it is.
	return fmt.Sprintf("RUN set -e; for name in %s; do [ -d \"./$name\" ] && [ ! -L \"./$name\" ] || ln -sfn \"%s/$name\" \"./$name\"; done\n",
		strings.Join(o.NativeFiles, " "), nativeSecretFilesDir)
}

// nativeXDGCacheHome is Render's build cache directory, $XDG_CACHE_HOME on
// every native runtime (w9/m493): what a build leaves there is there for the
// service's next build, a "Clear build cache & deploy" empties it, and the
// running service never carries it. pip, uv, Poetry, Go's build cache,
// Corepack, node-gyp and Playwright keep their caches under it by default.
const nativeXDGCacheHome = "/opt/render/.cache"

// nativeXDGCacheID names the BuildKit cache mount the build command writes
// $XDG_CACHE_HOME into. The id is the mount's identity across the solves of
// one build pod's daemon (xdgcache.go), and that daemon is the pod's own, so
// no two builds ever share it.
const nativeXDGCacheID = "bex-xdg-cache"

// nativeXDGCacheDir sets $XDG_CACHE_HOME for the build and the running
// service alike, as on Render, and makes the directory, so the service finds
// it writable and empty (Render keeps nothing written there at runtime).
//
// UV_LINK_MODE=copy is for the build alone (an ARG, which the image does not
// keep, and which a service's own value replaces). The cache mount is a
// filesystem of its own, so uv cannot hardlink a package from its cache into
// the virtualenv: it copies instead, as it would be told to here, but first
// prints a three-line warning into every uv build's log, which Render's
// single-disk build never shows.
const nativeXDGCacheDir = "ENV XDG_CACHE_HOME=" + nativeXDGCacheHome + "\n" +
	"ARG UV_LINK_MODE=copy\n" +
	"RUN mkdir -p " + nativeXDGCacheHome + "\n"

// nativeXDGCacheMount is the build command's mount of $XDG_CACHE_HOME: a cache
// mount, so the files it holds never reach a layer of the image, which is a
// single stage with no runtime stage to leave them out of.
const nativeXDGCacheMount = " --mount=type=cache,id=" + nativeXDGCacheID + ",target=" + nativeXDGCacheHome + ",sharing=locked"

// nativePythonVenv is where Render keeps a Python service's virtualenv
// (VENV_ROOT, render.com/docs/environment-variables).
const nativePythonVenv = nativeWorkdir + "/.venv"

// nativeWorkdir is a native image's working directory on every runtime but
// Go, Render's: it holds the service's root directory, and the publish check
// looks from it as the publish Job's extract does.
const nativeWorkdir = "/opt/render/project/src"

// nativeSourceDir is o's working directory: the source is copied into it, and
// the start command, the pre-deploy command, a cron run and a Shell or SSH
// session all start in it, since no workload of the image sets a working
// directory of its own. Render's table of source paths (render.com/docs/disks,
// read 2026-11-20) gives every native runtime nativeWorkdir but Go, whose
// source sits in its GOPATH at /opt/render/project/go/src/github.com/<owner>/<repo>,
// with the documented example disk at that directory's data (w6/m546). A
// static site keeps nativeWorkdir, whatever toolchain builds it: Render's
// static sites have no runtime.
func nativeSourceDir(o Options) string {
	if o.StaticSite || nativeRuntime(o) != "go" {
		return nativeWorkdir
	}
	if dir, ok := nativeGoSourceDir(o.Repo, o.RootDir); ok {
		return dir
	}
	return nativeWorkdir
}

// nativeSourceLink keeps nativeWorkdir resolving in an image whose source is
// in dir: a link to it, so an absolute path and a disk mounted under
// /opt/render/project/src before w6/m546 still land in the service's own tree
// (the container runtime resolves a mount path's links inside the image). The
// link is made after WORKDIR has made its target; "" when dir is nativeWorkdir.
func nativeSourceLink(dir string) string {
	if dir == nativeWorkdir {
		return ""
	}
	return "RUN ln -s " + dir + " " + nativeWorkdir + "\n"
}

// nativeSourceSegment is a part of a source directory used as is: the image's
// WORKDIR expands $ and \, and the link step is a shell line. It is the
// character set of a GitHub owner and repository name.
var nativeSourceSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// nativeGoSourceDir is Render's directory for a Go service built from repo:
// GOPATH's src/<host>/<owner>/<repo>, with the service's root directory under
// it, since Render runs every command from the root directory
// (render.com/docs/monorepo-support). The host is lowercased; the owner and
// the repository keep the URL's case, as RENDER_GIT_REPO_SLUG does, and a
// trailing .git goes. Render documents GitHub's form only: a GitLab or
// Bitbucket repository gets its own host and full path the same way, which is
// unverified against Render. ok is false when repo names no host and
// owner/repo, or when a part is ".", ".." or holds a character outside
// nativeSourceSegment; the service then keeps nativeWorkdir.
func nativeGoSourceDir(repo, rootDir string) (string, bool) {
	host, repoPath, ok := gitHostPath(repo)
	parts := strings.Split(repoPath, "/")
	if !ok || len(parts) < 2 {
		return "", false
	}
	if rel := repoRelative(rootDir); rel != "" {
		parts = append(parts, strings.Split(rel, "/")...)
	}
	for _, part := range append([]string{host}, parts...) {
		if part == "." || part == ".." || !nativeSourceSegment.MatchString(part) {
			return "", false
		}
	}
	return nativeGoPath + "/src/" + host + "/" + strings.Join(parts, "/"), true
}

// gitHostPath splits a repository URL in a form the App accepts (https://,
// ssh://, or git@host:path) into its lowercased host and its path, without
// the surrounding slashes or a trailing .git.
func gitHostPath(repo string) (host, repoPath string, ok bool) {
	s := strings.TrimSpace(repo)
	if rest, scp := strings.CutPrefix(s, "git@"); scp {
		host, repoPath, ok = strings.Cut(rest, ":")
	} else if u, err := url.Parse(s); err == nil && (u.Scheme == "https" || u.Scheme == "ssh") {
		host, repoPath, ok = u.Hostname(), u.Path, true
	}
	repoPath = strings.TrimSuffix(strings.Trim(repoPath, "/"), ".git")
	return strings.ToLower(host), repoPath, ok && host != "" && repoPath != ""
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
// Python gets Render's Gunicorn flags, preloading the app, writing access
// lines to stdout (w8/113) and binding Render's port (w32/m424): Gunicorn
// reads the variable after gunicorn.conf.py and before the command line, so
// its --bind replaces a bind in the config file, as on Render, while a -b on
// the start command still wins. It also gets
// Render's FORWARDED_ALLOW_IPS=* (w8/116): requests arrive from the ingress,
// never loopback, so without it Gunicorn (and Uvicorn, which reads the same
// variable) ignores X-Forwarded-Proto and an HTTPS request reaches the app
// as http. The ingress replaces the client's forwarding headers with its own,
// and only it and the platform's namespaces reach a pod from outside its
// workspace.
//
// Render's remaining documented per-runtime defaults (render.com/docs/
// environment-variables, read 2026-11-11; w8/256):
//   - Python: PIPENV_YES=true, and CI=true at build time only. CI is an ARG,
//     which a build RUN sees and the image does not keep.
//   - Go: GO111MODULE=on and GOPATH=/opt/render/project/go, with its bin
//     first on PATH so a `go install`ed tool still resolves.
//   - Rust: ROCKET_ENV=prod, and CARGO_HOME and RUSTUP_HOME at Render's
//     paths. Those are links to where the toolchain image keeps them: the
//     paths exist and hold the toolchain, and nothing is copied.
//
// Render pins each runtime's own port variable to its default port 10000
// (NativeServingPortEnv). Python's GUNICORN_CMD_ARGS and Ruby's
// PASSENGER_PORT are image defaults at that port, as on Render, and the
// operator replaces them on a web or private service's pod with the port it
// is routed on (servingPortDefault, w32/m424). ROCKET_PORT is set on the pod
// only, as Render sets it at runtime only (w8/273). Elixir gets
// RELEASE_DISTRIBUTION=name (w8/272): a release starts
// its node with a long name, which needs a hostname with a domain, so the
// operator gives an Elixir service's pods one (elixirNodeSubdomain). Python
// gets Render's virtualenv (w8/274): one at /opt/render/project/src/.venv,
// made with the selected interpreter before the source is copied, activated
// (VIRTUAL_ENV, its bin first on PATH) for the build, the pre-deploy step and
// the started service, and named by VENV_ROOT. pip, Pipenv, Poetry and uv
// install into an active virtualenv, so a build command's packages land there
// and its commands resolve from there. A build that keeps its dependency
// directories also records the virtualenv as made (nativeDepsRecord), so the
// next build's seed can put the kept one in its place (w9/127). One is left
// out, with its own follow-up: NODE_MODULES_CACHE (would have to turn the
// dependency cache off when false).
func nativeBuildDefaults(o Options) string {
	switch nativeRuntime(o) {
	case "elixir":
		return "ENV MIX_ENV=prod\nENV RELEASE_DISTRIBUTION=name\n"
	case "go":
		return "ENV GO111MODULE=on GOPATH=" + nativeGoPath + "\n" +
			"ENV PATH=" + nativeGoPath + "/bin:$PATH\n"
	case nativeRustRuntime:
		return `RUN mkdir -p /opt/render/project && ln -s "$CARGO_HOME" ` + nativeCargoHome + ` && ln -s "$RUSTUP_HOME" ` + nativeRustupHome + "\n" +
			"ENV CARGO_HOME=" + nativeCargoHome + " RUSTUP_HOME=" + nativeRustupHome + " ROCKET_ENV=prod\n"
	case nativePythonRuntime:
		venv := "RUN python3 -m venv " + nativePythonVenv
		if slices.Contains(o.depsCache(), nativePythonVenv) {
			venv += " && " + nativeDepsRecord(nativePythonVenv)
		}
		return venv + "\n" +
			"ENV VIRTUAL_ENV=" + nativePythonVenv + " VENV_ROOT=" + nativePythonVenv + "\n" +
			"ENV PATH=" + nativePythonVenv + "/bin:$PATH\n" +
			nativeServingPortDefault(nativePythonRuntime) +
			`ENV FORWARDED_ALLOW_IPS="*"` + "\n" +
			"ENV PIPENV_YES=true\n" +
			"ARG CI=true\n"
	case nativeRubyRuntime:
		// Render's Ruby defaults, build and runtime alike (w5/162): Rails
		// serves its own assets and logs to stdout only when told to, so
		// without these a migrated app 404s its CSS/JS and its log lines
		// never reach the Logs tab. PASSENGER_PORT makes Passenger Standalone
		// bind the platform's port instead of its own 3000 (w32/m424).
		//
		// GEM_PATH is Render's too (w32/m424). Set alone it would hide the
		// gems Ruby ships (rake, minitest): RubyGems searches GEM_PATH plus
		// GEM_HOME only, and the docker-library images set GEM_HOME to
		// /usr/local/bundle, away from Ruby's own directory. So GEM_HOME is
		// Ruby's Gem.default_dir, /usr/local/lib/ruby/gems/<major.minor>.0
		// (RUBY_VERSION is every reviewed image's full x.y.z), which is what
		// RubyGems uses when GEM_HOME is unset, as on Render. Measured on the
		// 3.0, 3.4 and 4.0 lines: `require "minitest"`, `require "rake"` and
		// the image's own rake work outside the bundle as they did before, and
		// `gem install` lands in Ruby's directory, on Gem.path. Inside
		// `bundle exec` Bundler sets both itself, from BUNDLE_PATH.
		return "ENV RAILS_ENV=production RAILS_SERVE_STATIC_FILES=true RAILS_LOG_TO_STDOUT=true\n" +
			"ENV MALLOC_ARENA_MAX=2 PIDFILE=/tmp/puma-server.pid\n" +
			"ENV PASSENGER_ENGINE=builtin PASSENGER_ENVIRONMENT=production\n" +
			nativeServingPortDefault(nativeRubyRuntime) +
			"ENV BUNDLE_DEPLOYMENT=true BUNDLE_PATH=" + nativeRubyGems + " BUNDLE_BIN=" + nativeRubyGems + "/bin" +
			" BUNDLE_APP_CONFIG=" + nativeRubyGems + "\n" +
			"ENV GEM_PATH=" + nativeRubyGems + " GEM_HOME=" + nativeRubyGemHome + "\n" +
			"ENV PATH=" + nativeRubyGems + "/bin:$PATH\n"
	}
	return ""
}

// NativeServingPortEnv is the variable through which a native runtime's usual
// server learns the port to bind, and its value for port; "" for a runtime
// with none. Render sets it to 10000 (render.com/docs/environment-variables,
// read 2026-11-16): Gunicorn's flags with --bind=0.0.0.0:10000 for Python,
// Passenger Standalone's PASSENGER_PORT for Ruby (its own default is 3000),
// and Rocket's ROCKET_PORT for Rust (w8/273). On bex the port a service is
// routed on can be a configured or a detected one, so the operator gives a
// web or private service's pod the value for that port, which is Render's
// wherever the port is the default (w32/m424).
func NativeServingPortEnv(runtime string, port int) (name, value string) {
	p := strconv.Itoa(port)
	switch runtime {
	case nativePythonRuntime:
		return "GUNICORN_CMD_ARGS", "--preload --access-logfile - --bind=0.0.0.0:" + p
	case nativeRubyRuntime:
		return "PASSENGER_PORT", p
	case nativeRustRuntime:
		return "ROCKET_PORT", p
	}
	return "", ""
}

// nativeServingPortDefault is runtime's NativeServingPortEnv at Render's
// default port as an image ENV line, its value quoted for the spaces in
// Gunicorn's flags.
func nativeServingPortDefault(runtime string) string {
	name, value := NativeServingPortEnv(runtime, appv1alpha1.DefaultPort)
	return fmt.Sprintf("ENV %s=%q\n", name, value)
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

// NativeStartArgv is how a native image runs a Render command: its CMD, and
// the arguments a service linked to a native build source passes to the same
// entrypoint (w2/m254 t017).
func NativeStartArgv(command string) []string {
	return []string{"/bin/bash", "-c", command}
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
	_ = encoder.Encode(NativeStartArgv(command))
	return strings.TrimSpace(out.String())
}

// nativeFilePartsList is "<name> <parts>" for each file the build carries in
// more than one part, space-separated for the preparer's word split (names are
// in the Secret-key charset). Empty when every file fits one secret.
func nativeFilePartsList(o Options) string {
	var fields []string
	for _, name := range o.NativeFiles {
		if parts := nativeFileParts(o, name); parts > 1 {
			fields = append(fields, name, strconv.Itoa(parts))
		}
	}
	return strings.Join(fields, " ")
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
	if parts := nativeFilePartsList(o); parts != "" {
		env = append(env, corev1.EnvVar{Name: "BEX_NATIVE_FILE_PARTS", Value: execution.KubernetesLiteral(parts)})
	}

	mounts := []corev1.VolumeMount{{Name: "native-build", MountPath: "/native"}}
	if o.RuntimeEnvSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: runtimeEnvVolume, MountPath: "/runtime-env", ReadOnly: true})
	}
	if len(o.NativeFilesSecrets) > 0 {
		mounts = append(mounts, corev1.VolumeMount{Name: secretFilesVolume, MountPath: "/native-files", ReadOnly: true})
	}
	prep := corev1.Container{
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
done
# A file over BuildKit's secret limit is cut into the parts the build mounts
# (w4/699). Its part count was fixed when the Job was made, so a file that has
# since grown past it fails here, by name, rather than inside BuildKit.
set -- ${BEX_NATIVE_FILE_PARTS:-}
while [ $# -gt 0 ]; do
  name=$1 parts=$2
  shift 2
  if [ "$(wc -c < "/native/files/$name")" -gt $((parts * ` + fmt.Sprint(buildkitMaxSecretSize) + `)) ]; then
    echo "secret file $name changed after this build started; deploy again" >&2
    exit 1
  fi
  mkdir -p "` + nativeFilePartsDir + `/$name"
  p=0
  while [ $p -lt $parts ]; do
    dd if="/native/files/$name" of="` + nativeFilePartsDir + `/$name/$p" bs=` + fmt.Sprint(buildkitMaxSecretSize) + ` skip=$p count=1 2>/dev/null
    p=$((p + 1))
  done
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
	withNativeOIDCPreparer(&prep, o)
	return prep
}
