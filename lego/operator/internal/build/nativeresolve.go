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
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// The resolve-native-runtime phase (w8/m51) runs bex's own image after the
// preparer: it reads the decoded env the build will see and the checked-out
// source, picks the toolchain line, rewrites the generated Dockerfile's FROM
// to that line's pin, and narrates the choice into the build log.
const (
	nativeResolveContainer = "resolve-native-runtime"
	// NativeResolveBinary is the entrypoint in bex's image (lego/Dockerfile).
	NativeResolveBinary = "/native-resolve"
	// maxVersionFileBytes bounds what a version file may make the resolver
	// read; real ones are a line, package.json/Gemfile a few KiB.
	maxVersionFileBytes = 1 << 20
)

// ResolveNativeBuild is the resolve phase: env bundle and Dockerfile paths,
// the service's root directory in the checkout, and the runtime. It returns
// the choices it applied, the primary toolchain first, then Python's Poetry
// (w8/m72) and, for a uv project, uv (w8/m81), Ruby's and Python's Node
// (w8/m77) and any opted-in companion (Bun); an *UnsupportedVersionError is
// tenant input.
func ResolveNativeBuild(runtime, bundlePath, dockerfilePath, sourceDir string) ([]NativeChoice, error) {
	env, err := readEnvBundle(bundlePath)
	if err != nil {
		return nil, err
	}
	readFile := func(name string) ([]byte, bool) { return readVersionFile(sourceDir, name) }
	choice, err := ResolveNativeVersion(runtime, env, readFile)
	if err != nil {
		return nil, err
	}
	bun, withBun, err := ResolveNativeBun(env, readFile)
	if err != nil {
		return nil, err
	}
	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("read generated Dockerfile: %w", err)
	}
	rewritten, err := rewriteNativeBase(string(dockerfile), nativeToolchains[runtime].defaultImage(), choice.Image)
	if err != nil {
		return nil, err
	}
	choices := []NativeChoice{choice}
	if runtime == "python" {
		poetry, err := ResolveNativePoetry(env)
		if err != nil {
			return nil, err
		}
		uv, withUV, err := ResolveNativeUV(env, readFile)
		if err != nil {
			return nil, err
		}
		// Each step lands right after the FROM, so uv goes in first to follow
		// Poetry's layer, which a uv version change then leaves cached.
		if withUV {
			if rewritten, err = addNativeUV(rewritten, choice.Image, uv.Line); err != nil {
				return nil, err
			}
		}
		if rewritten, err = addNativePoetry(rewritten, choice.Image, poetry.Line); err != nil {
			return nil, err
		}
		choices = append(choices, poetry)
		if withUV {
			choices = append(choices, uv)
		}
	}
	if nativeJSCompanionRuntimes[runtime] {
		node, err := ResolveNativeVersion(nativeNodeRuntime, env, readFile)
		if err != nil {
			return nil, err
		}
		if rewritten, err = addNativeJSTools(rewritten, choice.Image, node.Image); err != nil {
			return nil, err
		}
		choices = append(choices, node)
	}
	if withBun {
		if rewritten, err = addNativeBun(rewritten, choice.Image, bun.Image); err != nil {
			return nil, err
		}
		choices = append(choices, bun)
	}
	if err := os.WriteFile(dockerfilePath, []byte(rewritten), 0o600); err != nil {
		return nil, fmt.Errorf("write generated Dockerfile: %w", err)
	}
	return choices, nil
}

// addNativeBun copies the chosen Bun binary into the toolchain image right
// after its FROM, before the source and the authored build, so the build, the
// pre-deploy step and the started service all find bun and bunx on the image
// PATH. --link keeps the Bun layer cached when the primary line changes.
func addNativeBun(dockerfile, baseImage, bunImage string) (string, error) {
	return afterNativeBase(dockerfile, baseImage, "COPY --link --from="+bunImage+" /usr/local/bin/bun /usr/local/bin/bun\n"+
		"RUN ln -sf bun /usr/local/bin/bunx\n")
}

// nativeJSCompanionRuntimes are the primary runtimes whose native builds also
// carry Render's documented JavaScript tools — Node, npm, Yarn and pnpm, at
// build and runtime: a Rails, Django, Phoenix or Go/Rust web app builds its
// frontend assets with them (w8/m77 Ruby/Python, w4/m213 Go/Rust/Elixir).
// Primary Node already has them.
var nativeJSCompanionRuntimes = map[string]bool{"ruby": true, "python": true, "go": true, "rust": true, "elixir": true}

// nativeYarnDir is Yarn 1 as the official Node 22/24 images ship it.
const nativeYarnDir = "/opt/yarn-v1.22.22"

// nativeYarnSourceLine is the reviewed Node line whose pinned image Yarn 1 is
// copied from. Yarn 1 is plain JavaScript, so one source serves every Node
// line and every companion; it is named rather than "the default image" so a
// later default of 26 — whose image ships no Yarn — cannot silently drop it.
const nativeYarnSourceLine = "24"

// nativeYarnInstall copies Yarn 1 out of the reviewed source image and links
// yarn and yarnpkg onto the image PATH, the way the official node images do.
// Shared by primary Node (w4/320) and the JavaScript companion (w8/m77).
func nativeYarnInstall() string {
	source, _ := nativeToolchains[nativeNodeRuntime].find(nativeYarnSourceLine)
	return "COPY --link --from=" + source.image + " " + nativeYarnDir + " " + nativeYarnDir + "\n" +
		"RUN ln -sf " + nativeYarnDir + "/bin/yarn /usr/local/bin/yarn && ln -sf " + nativeYarnDir + "/bin/yarnpkg /usr/local/bin/yarnpkg\n"
}

// addNativeJSTools copies the selected Node line's node binary and its global
// node_modules (npm, plus Corepack where the line has it) out of that line's
// pinned image, then adds Yarn and the pinned pnpm — right after the primary
// FROM, before the source and the authored build, with the links the official
// node images use. The Node images are Debian bookworm; every primary image is
// bookworm or (Elixir 1.18/1.19) trixie, whose glibc and libstdc++ are newer
// and backward compatible, so the copied binary runs on the primary image's own
// libraries — the real-image gate executes it on every line. --link keeps these
// layers cached when the primary line changes.
func addNativeJSTools(dockerfile, baseImage, nodeImage string) (string, error) {
	return afterNativeBase(dockerfile, baseImage, nativeJSToolLines(nodeImage))
}

func nativeJSToolLines(nodeImage string) string {
	return "COPY --link --from=" + nodeImage + " /usr/local/bin/node /usr/local/bin/node\n" +
		"COPY --link --from=" + nodeImage + " /usr/local/lib/node_modules /usr/local/lib/node_modules\n" +
		"RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm" +
		" && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx\n" +
		nativeYarnInstall() + nativePnpmInstall()
}

// addNativePoetry installs the chosen Poetry (w8/m72); see
// nativePythonToolInstall.
func addNativePoetry(dockerfile, baseImage, version string) (string, error) {
	return afterNativeBase(dockerfile, baseImage, nativePythonToolInstall("poetry", version, "poetry"))
}

// addNativeUV installs the chosen uv, with uvx (w8/m81); see
// nativePythonToolInstall. uv discovers interpreters itself: the image's
// selected Python on PATH serves a project whose pin it satisfies, and a
// project pinning another patch (bex builds a line's reviewed patch) gets the
// CPython uv downloads for it, as uv does anywhere.
func addNativeUV(dockerfile, baseImage, version string) (string, error) {
	return afterNativeBase(dockerfile, baseImage, nativePythonToolInstall("uv", version, "uv", "uvx"))
}

// nativePythonToolInstall is the step that installs an exact PyPI release of a
// Python-packaged tool right after the FROM, before the source and the
// authored build, into its own virtualenv /opt/bex/<pkg> on the selected
// Python, and links its commands onto the image PATH so the build, the
// pre-deploy step and the started service all find them. The application's
// packages, virtualenvs and PATH never share that virtualenv, and the tool's
// dependencies never land in the image's site-packages. The step runs under
// the image's own environment, not the service's, and sits before COPY so a
// source change reuses it; a changed version is a changed instruction.
func nativePythonToolInstall(pkg, version string, commands ...string) string {
	home := "/opt/bex/" + pkg
	var links strings.Builder
	for _, command := range commands {
		fmt.Fprintf(&links, " && ln -s %s/bin/%s /usr/local/bin/%s", home, command, command)
	}
	return fmt.Sprintf("RUN /usr/local/bin/python -m venv %[1]s && %[1]s/bin/pip install --no-cache-dir --disable-pip-version-check --no-input %[2]s==%[3]s%[4]s\n",
		home, pkg, version, links.String())
}

// afterNativeBase inserts lines right after the generated Dockerfile's FROM.
func afterNativeBase(dockerfile, baseImage, lines string) (string, error) {
	from := "\nFROM " + baseImage + "\n"
	if !strings.Contains(dockerfile, from) {
		return "", fmt.Errorf("generated Dockerfile does not carry the base %s", baseImage)
	}
	return strings.Replace(dockerfile, from, from+lines, 1), nil
}

// readEnvBundle decodes the preparer's KEY=base64 bundle. A later line wins,
// exactly as the in-image env loader exports them (literals after secrets).
func readEnvBundle(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read env bundle: %w", err)
	}
	defer func() { _ = f.Close() }()
	env := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		key, encoded, ok := strings.Cut(scanner.Text(), "=")
		if !ok || key == "" {
			continue
		}
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode env bundle entry %s: %w", key, err)
		}
		env[key] = string(value)
	}
	return env, scanner.Err()
}

// readVersionFile reads name from the service's root directory. Only a
// regular file is read: a symlink is ignored, because the refusal message
// quotes what it read and a link could point at the env bundle beside it.
func readVersionFile(sourceDir, name string) ([]byte, bool) {
	path := filepath.Join(sourceDir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxVersionFileBytes))
	return raw, err == nil
}

// rewriteNativeBase swaps the generated Dockerfile's single FROM (the default
// line) for the chosen line, refusing anything it did not generate.
func rewriteNativeBase(dockerfile, defaultImage, image string) (string, error) {
	want := "FROM " + defaultImage + "\n"
	if strings.Count(dockerfile, "\nFROM ") != 1 || !strings.Contains(dockerfile, "\n"+want) {
		return "", fmt.Errorf("generated Dockerfile does not carry the expected base %s", defaultImage)
	}
	return strings.Replace(dockerfile, "\n"+want, "\nFROM "+image+"\n", 1), nil
}

// nativeVersionResolver is the resolve phase's container: bex's own image,
// the preparer's non-root identity (it rewrites the 0600 Dockerfile the
// preparer wrote), the checkout read-only, no service account token.
func nativeVersionResolver(o Options) corev1.Container {
	return corev1.Container{
		Name:    nativeResolveContainer,
		Image:   o.NativeResolverImage,
		Command: []string{NativeResolveBinary},
		Env: []corev1.EnvVar{
			{Name: "BEX_NATIVE_RUNTIME", Value: nativeRuntime(o)},
			{Name: "BEX_NATIVE_SOURCE_DIR", Value: boundedSourceDir(o.RootDir)},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "native-build", MountPath: "/native"},
			{Name: "source", MountPath: sourceMount, ReadOnly: true},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                new(int64(65532)),
			RunAsGroup:               new(int64(65532)),
			RunAsNonRoot:             new(true),
			AllowPrivilegeEscalation: new(false),
			ReadOnlyRootFilesystem:   new(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}
