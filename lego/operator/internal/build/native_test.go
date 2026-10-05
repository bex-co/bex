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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func nativeOptions() Options {
	return Options{
		Repo: "https://github.com/example/app", Ref: "main", Name: "web",
		Registry: "zot:5000", Revision: "gen-1", Namespace: "bex-system",
		Builder: BuilderNative, Runtime: "node",
		BuildCommand: "npm ci && npm run build",
		StartCommand: "node dist/server.js --flag='quoted'",
		BuildEnv: []corev1.EnvVar{
			{Name: "NODE_ENV", Value: "production"},
			{Name: "PORT", Value: "9999"},
		},
		RuntimeEnvSecret: "web-env",
	}
}

func TestNativeDockerfilePreservesRenderCommands(t *testing.T) {
	o := nativeOptions()
	dockerfile := nativeDockerfile(o)
	for _, want := range []string{
		"FROM node:24-bookworm@sha256:",
		"npm ci && npm run build",
		`CMD ["/bin/bash","-c","node dist/server.js --flag='quoted'"]`,
		"--mount=type=secret,id=render-env",
		": bex-native-env-rev=none",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

// Secret mount contents are not part of BuildKit's instruction cache key, so two
// Options that differ only in BuildEnv values previously produced identical
// Dockerfiles and could reuse a stale RUN. The opaque NativeEnvRevision is the
// durable cache input that makes A→B rebuild (w7/m87).
func TestNativeDockerfileEnvRevisionBustsCacheKey(t *testing.T) {
	a := nativeOptions()
	a.BuildEnv = []corev1.EnvVar{{Name: "MESSAGE", Value: "A"}}
	b := nativeOptions()
	b.BuildEnv = []corev1.EnvVar{{Name: "MESSAGE", Value: "B"}}
	if nativeDockerfile(a) != nativeDockerfile(b) {
		t.Fatal("literal BuildEnv values must not appear in the generated Dockerfile")
	}
	a.NativeEnvRevision = "1"
	b.NativeEnvRevision = "2"
	if nativeDockerfile(a) == nativeDockerfile(b) {
		t.Fatal("distinct env revisions must change the env-dependent RUN cache key")
	}
	if strings.Contains(nativeDockerfile(a), "MESSAGE") || strings.Contains(nativeDockerfile(a), "=A") {
		t.Fatal("secret/literal values must not leak into the generated Dockerfile")
	}
	same := nativeOptions()
	same.NativeEnvRevision = "1"
	if nativeDockerfile(a) != nativeDockerfile(same) {
		t.Fatal("unchanged revision must keep a stable Dockerfile")
	}
}

// A login shell sources /etc/profile, which on Debian unconditionally rewrites
// PATH to the fixed system default — discarding the toolchain PATH the runtime
// images set via ENV (golang's /usr/local/go/bin, rust's /usr/local/cargo/bin).
// Go and Rust builds died with "command not found"; the interpreted runtimes
// only survived because their binaries sit in /usr/local/bin. Every runtime's
// RUN and CMD must therefore use a NON-login shell.
func TestNativeDockerfileKeepsToolchainPATH(t *testing.T) {
	const profileDefault = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	for runtime := range nativeToolchains {
		o := nativeOptions()
		o.Runtime = runtime
		dockerfile := nativeDockerfile(o)
		if strings.Contains(dockerfile, `"-lc"`) {
			t.Errorf("%s: login shell resets PATH to %s, dropping the toolchain PATH:\n%s", runtime, profileDefault, dockerfile)
		}
		for _, want := range []string{
			`RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",`,
			`CMD ["/bin/bash","-c",`,
		} {
			if !strings.Contains(dockerfile, want) {
				t.Errorf("%s: Dockerfile missing %q:\n%s", runtime, want, dockerfile)
			}
		}
	}
}

func TestNativeRuntimeImagesAreDigestPinned(t *testing.T) {
	for runtime, toolchain := range nativeToolchains {
		if _, ok := toolchain.find(toolchain.defaultLine); !ok {
			t.Errorf("native runtime %s default line %q is not one of its lines", runtime, toolchain.defaultLine)
		}
		for _, line := range toolchain.lines {
			parts := strings.Split(line.image, "@sha256:")
			if len(parts) != 2 || len(parts[1]) != 64 {
				t.Errorf("native runtime %s line %s image is not digest-pinned: %q", runtime, line.line, line.image)
			}
		}
	}
}

func TestNativeBuildJobPreparesDockerfileAndSecretEnv(t *testing.T) {
	o := nativeOptions()
	job := BuildJob(o, o.ImageRef())
	if len(job.Spec.Template.Spec.InitContainers) != 3 {
		t.Fatalf("initContainers = %d, want clone, preparer, buildkit", len(job.Spec.Template.Spec.InitContainers))
	}
	prep := job.Spec.Template.Spec.InitContainers[1]
	if prep.Name != "prepare-native-build" {
		t.Fatalf("preparer name = %q", prep.Name)
	}
	if hasEnv(prep.Env, "PORT") {
		t.Fatal("operator-owned PORT must not enter the native build env bundle")
	}
	if !hasEnv(prep.Env, "NODE_ENV") {
		t.Fatal("literal service env missing from native build preparer")
	}
	prepScript := prep.Args[0]
	if strings.Index(prepScript, "for path in /runtime-env/*") > strings.Index(prepScript, "for key in $BEX_NATIVE_LITERAL_KEYS") {
		t.Fatal("literal env must be appended after envFrom Secret so it wins like Kubernetes container env")
	}
	if !hasVolume(job.Spec.Template.Spec.Volumes, "runtime-env") {
		t.Fatal("OpenBao-backed runtime env Secret not mounted for native build")
	}
	buildkit := containerByName(job.Spec.Template.Spec.InitContainers, "buildkit")
	if !containsPair(buildkit.Args, "--secret", "id=render-env,src=/native/render-env") {
		t.Fatalf("buildkit args missing render env secret: %v", buildkit.Args)
	}
}

func TestValidateNativeOptions(t *testing.T) {
	o := nativeOptions()
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	o.Runtime = "java"
	if err := validateNativeOptions(o); err == nil {
		t.Fatal("unsupported runtime accepted")
	}
}

func hasEnv(env []corev1.EnvVar, name string) bool {
	for _, item := range env {
		if item.Name == name {
			return true
		}
	}
	return false
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func containsPair(items []string, first, second string) bool {
	for i := 0; i+1 < len(items); i++ {
		if items[i] == first && items[i+1] == second {
			return true
		}
	}
	return false
}

func TestNativeStaticSiteBuild(t *testing.T) {
	o := nativeOptions()
	o.StaticSite = true
	o.Runtime = ""      // static blueprints declare no toolchain — node is the default
	o.StartCommand = "" // a static site's image is never run

	if err := validateNativeOptions(o); err != nil {
		t.Fatalf("static native options must validate without startCommand/runtime: %v", err)
	}
	dockerfile := nativeDockerfile(o)
	for _, want := range []string{
		"FROM node:24-bookworm",
		"npm ci && npm run build",
		"--mount=type=secret,id=render-env",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("static Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
	for _, reject := range []string{"CMD", "ENV PORT"} {
		if strings.Contains(dockerfile, reject) {
			t.Errorf("static Dockerfile must not carry %q (extract-only image):\n%s", reject, dockerfile)
		}
	}

	// A non-static native build still requires its start command.
	o.StaticSite = false
	o.Runtime = "node"
	if err := validateNativeOptions(o); err == nil {
		t.Fatal("non-static native build without startCommand must be rejected")
	}
}

// Execute the generated shells: fragment assertions miss shell newline loss.
func TestNativeEnvironmentRoundTrip(t *testing.T) {
	for _, value := range []string{"", "a", "ab", "abc", "qa-build\n", "two\n\n", "embedded\nline", "literal $HOME quote'."} {
		for _, literal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%x/literal=%t", value, literal), func(t *testing.T) {
				dir := t.TempDir()
				secret := filepath.Join(dir, "secret")
				if err := os.Mkdir(secret, 0700); err != nil {
					t.Fatal(err)
				}
				secretValue := value
				o := nativeOptions()
				o.BuildCommand = "printf '%s' \"$MESSAGE\""
				o.BuildEnv = nil
				if literal {
					secretValue = "overridden"
					o.BuildEnv = []corev1.EnvVar{{Name: "MESSAGE", Value: value}}
				}
				if err := os.WriteFile(filepath.Join(secret, "MESSAGE"), []byte(secretValue), 0600); err != nil {
					t.Fatal(err)
				}
				prep := nativeBuildPreparer(o)
				script := strings.NewReplacer("/native", dir, "/runtime-env", secret).Replace(prep.Args[0])
				cmd := exec.Command("sh", "-eu", "-c", script)
				cmd.Env = os.Environ()
				for _, env := range prep.Env {
					cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("prepare: %v: %s", err, out)
				}
				out, err := runNativeBuildShell(t, o, filepath.Join(dir, "render-env"))
				if err != nil {
					t.Fatalf("decode: %v: %s", err, out)
				}
				if string(out) != value {
					t.Fatalf("got %q, want %q", out, value)
				}
			})
		}
	}
}

// runNativeBuildShell plays the generated Dockerfile's env loader (its COPY
// heredoc) and RUN locally, with the secret mount pointed at bundle.
func runNativeBuildShell(t *testing.T, o Options, bundle string) ([]byte, error) {
	t.Helper()
	loader := filepath.Join(t.TempDir(), "load-env")
	var heredoc []string
	inHeredoc := false
	for line := range strings.SplitSeq(nativeDockerfile(o), "\n") {
		switch {
		case line == "COPY <<'"+nativeEnvLoaderEOF+"' "+nativeEnvLoaderPath:
			inHeredoc = true
		case inHeredoc && line == nativeEnvLoaderEOF:
			inHeredoc = false
			script := strings.ReplaceAll(strings.Join(heredoc, "\n")+"\n", nativeEnvSecretPath, bundle)
			if err := os.WriteFile(loader, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
		case inHeredoc:
			heredoc = append(heredoc, line)
		case strings.HasPrefix(line, "RUN "):
			if heredoc == nil {
				t.Fatal("RUN precedes the env loader")
			}
			var args []string
			if err := json.Unmarshal([]byte(line[strings.Index(line, "["):]), &args); err != nil {
				t.Fatal(err)
			}
			args[2] = strings.Replace(args[2], nativeEnvLoaderPath, loader, 1)
			return exec.Command(args[0], args[1:]...).CombinedOutput()
		}
	}
	t.Fatal("missing native RUN")
	return nil, nil
}

// The w4/m163 regression: a native build command could not read a saved secret
// file at /etc/secrets/<name>. Each file must ride a transient, required
// BuildKit secret mount at that exact path, through the preparer's snapshot,
// with only NAMES in the Dockerfile and Job — never a COPY, ARG or ENV.
func TestNativeSecretFilesTransport(t *testing.T) {
	o := nativeOptions()
	o.NativeFilesSecret, o.NativeFiles = "bld-web-native-files", []string{".npmrc", "qa-r59.txt"}
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	dockerfile := nativeDockerfile(o)
	want := "RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env" +
		" --mount=type=secret,id=bex-file-0,target=/etc/secrets/.npmrc,required=true" +
		" --mount=type=secret,id=bex-file-1,target=/etc/secrets/qa-r59.txt,required=true ["
	if !strings.Contains(dockerfile, want) {
		t.Fatalf("Dockerfile missing file mounts %q:\n%s", want, dockerfile)
	}
	for _, reject := range []string{"COPY /native", "ARG ", "ENV QA", "/native/files"} {
		if strings.Contains(dockerfile, reject) {
			t.Fatalf("file transport must not use %q:\n%s", reject, dockerfile)
		}
	}

	job := BuildJob(o, o.ImageRef())
	spec := job.Spec.Template.Spec
	var filesVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "native-files" {
			filesVolume = &spec.Volumes[i]
		}
	}
	if filesVolume == nil || filesVolume.Secret == nil || filesVolume.Secret.SecretName != o.NativeFilesSecret {
		t.Fatalf("native-files volume = %+v, want Secret %s", filesVolume, o.NativeFilesSecret)
	}
	buildkit := containerByName(spec.InitContainers, "buildkit")
	for i, name := range o.NativeFiles {
		if !containsPair(buildkit.Args, "--secret", fmt.Sprintf("id=bex-file-%d,src=/native/files/%s", i, name)) {
			t.Fatalf("buildkit args missing %s transport: %v", name, buildkit.Args)
		}
	}
	// Only the preparer reads the Secret volume; BuildKit (which runs tenant RUN
	// steps) sees just the preparer's snapshot under /native.
	for _, mount := range buildkit.VolumeMounts {
		if mount.Name == "native-files" {
			t.Fatal("buildkit must not mount the files Secret directly")
		}
	}
	prep := containerByName(spec.InitContainers, "prepare-native-build")
	mounted := false
	for _, mount := range prep.VolumeMounts {
		mounted = mounted || (mount.Name == "native-files" && mount.ReadOnly && mount.MountPath == "/native-files")
	}
	if !mounted {
		t.Fatalf("preparer must mount the files Secret read-only: %+v", prep.VolumeMounts)
	}

	// No files: no volume, no mounts, no extra args — byte-identical to before.
	plain := nativeOptions()
	if strings.Contains(nativeDockerfile(plain), "/etc/secrets") || hasVolume(BuildJob(plain, plain.ImageRef()).Spec.Template.Spec.Volumes, "native-files") {
		t.Fatal("a build without secret files must not mount any")
	}
}

func TestValidateNativeSecretFiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret string
		files  []string
	}{
		{"traversal", "s", []string{"../x"}},
		{"slash", "s", []string{"a/b"}},
		{"space", "s", []string{"a b"}},
		{"comma breaks CSV", "s", []string{"a,b"}},
		{"equals breaks CSV", "s", []string{"a=b"}},
		{"quote", "s", []string{`a"b`}},
		{"glob", "s", []string{"a*"}},
		{"dot", "s", []string{"."}},
		{"secret volume reserved", "s", []string{"..data"}},
		{"empty", "s", []string{""}},
		{"unsorted", "s", []string{"b", "a"}},
		{"duplicate", "s", []string{"a", "a"}},
		{"files without Secret", "", []string{"a"}},
		{"Secret without files", "s", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.NativeFilesSecret, o.NativeFiles = tc.secret, tc.files
			if err := validateNativeOptions(o); err == nil {
				t.Fatalf("accepted %q / %q", tc.secret, tc.files)
			}
		})
	}
}

// Execute the preparer and the generated RUN: a build command reads each file's
// exact bytes — empty, trailing newlines, NUL — and a declared file missing from
// the Secret fails the preparer before tenant code runs.
func TestNativeSecretFilesRoundTrip(t *testing.T) {
	files := map[string]string{
		".npmrc":     "",
		"cert.pem":   "-----BEGIN-----\nline\n\n",
		"qa-r59.txt": "qa-r59-build-file-marker",
		"zero.bin":   "a\x00b",
	}
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.Mkdir(secret, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(secret, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	o := nativeOptions()
	o.BuildEnv = nil
	o.NativeFilesSecret = "bld-web-native-files"
	o.NativeFiles = []string{".npmrc", "cert.pem", "qa-r59.txt", "zero.bin"}
	runPreparer := func() ([]byte, error) {
		prep := nativeBuildPreparer(o)
		script := strings.NewReplacer("/native-files", secret, "/native", dir, "/runtime-env", filepath.Join(dir, "absent")).Replace(prep.Args[0])
		cmd := exec.Command("sh", "-eu", "-c", script)
		cmd.Env = os.Environ()
		for _, env := range prep.Env {
			cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
		}
		return cmd.CombinedOutput()
	}
	if out, err := runPreparer(); err != nil {
		t.Fatalf("prepare: %v: %s", err, out)
	}
	for name, content := range files {
		// BuildKit mounts /native/files/<name> at /etc/secrets/<name>; the shell
		// stands in for that mount by reading the snapshot path directly.
		o.BuildCommand = fmt.Sprintf("cat %s/files/%s", dir, name)
		out, err := runNativeBuildShell(t, o, filepath.Join(dir, "render-env"))
		if err != nil {
			t.Fatalf("%s: %v: %s", name, err, out)
		}
		if string(out) != content {
			t.Fatalf("%s = %q, want %q", name, out, content)
		}
	}

	if err := os.Remove(filepath.Join(secret, "qa-r59.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "files")); err != nil {
		t.Fatal(err)
	}
	if out, err := runPreparer(); err == nil {
		t.Fatalf("a declared file missing from the Secret must fail the preparer: %s", out)
	}
}

// TestNativeSecretFilesPinnedBuildKit runs the generated Dockerfile through the
// pinned production BuildKit image with the exact Job buildctl args, then
// inspects the exported filesystem. It proves what generated-text assertions
// cannot: the static build command reads the marker, no secret file is left in
// the image, and — with a warm cache — changed bytes reuse the stale RUN unless
// the revision moves, which is why the operator folds file bytes into it.
// Opt-in: needs a privileged Docker daemon and pulls the node toolchain.
func TestNativeSecretFilesPinnedBuildKit(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to run against the pinned BuildKit image")
	}
	cacheVolume := fmt.Sprintf("bex-native-files-it-%d", os.Getpid())
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", "-f", cacheVolume).Run() })

	build := func(marker, revision string) string {
		t.Helper()
		dir := t.TempDir()
		for _, sub := range []string{"src", "secret", "native", "out"} {
			if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "src", "README"), []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{"qa-r59.txt": marker, "empty.txt": ""} {
			if err := os.WriteFile(filepath.Join(dir, "secret", name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		o := nativeOptions()
		o.StaticSite, o.Runtime, o.StartCommand, o.BuildEnv = true, "", "", nil
		o.BuildCommand = "cp /etc/secrets/qa-r59.txt index.html && cp /etc/secrets/empty.txt empty.out"
		o.NativeEnvRevision = revision
		o.NativeFilesSecret, o.NativeFiles = "bld-web-native-files", []string{"empty.txt", "qa-r59.txt"}
		if err := validateNativeOptions(o); err != nil {
			t.Fatal(err)
		}
		prep := nativeBuildPreparer(o)
		script := strings.NewReplacer("/native-files", filepath.Join(dir, "secret"), "/native", filepath.Join(dir, "native"),
			"/runtime-env", filepath.Join(dir, "absent")).Replace(prep.Args[0])
		cmd := exec.Command("sh", "-eu", "-c", script)
		cmd.Env = os.Environ()
		for _, env := range prep.Env {
			cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prepare: %v: %s", err, out)
		}
		// The Job's exact buildctl args, including its OCI archive output.
		args := containerByName(BuildJob(o, o.ImageRef()).Spec.Template.Spec.InitContainers, "buildkit").Args
		for i := range args {
			if strings.HasPrefix(args[i], "context=") {
				args[i] = "context=/src"
			}
		}
		run := append([]string{"run", "--rm", "--privileged", "--entrypoint", "buildctl-daemonless.sh",
			"-v", filepath.Join(dir, "src") + ":/src:ro", "-v", filepath.Join(dir, "native") + ":/native:ro",
			"-v", filepath.Join(dir, "out") + ":" + outputMount, "-v", cacheVolume + ":/var/lib/buildkit",
			defaultBuildkitImage}, args...)
		out, err := exec.Command("docker", run...).CombinedOutput()
		if err != nil {
			t.Fatalf("buildkit: %v\n%s", err, out)
		}
		if strings.Contains(string(out), marker) {
			t.Fatalf("build log carries the file contents:\n%s", out)
		}
		config, layer := lastOCILayer(t, filepath.Join(dir, "out", "image.tar"))
		if strings.Contains(string(config), marker) {
			t.Fatalf("image config carries the file contents: %s", config)
		}
		for name := range layer {
			if strings.HasPrefix(name, "etc/secrets/") || strings.HasPrefix(name, "run/secrets/") {
				t.Fatalf("secret mount committed into the RUN layer: %s", name)
			}
		}
		if empty, ok := layer["opt/render/project/src/empty.out"]; !ok || len(empty) != 0 {
			t.Fatalf("empty file = %q present=%v", empty, ok)
		}
		got := string(layer["opt/render/project/src/index.html"])
		t.Logf("marker=%q revision=%s RUN-layer=%v index.html=%q", marker, revision, slices.Sorted(maps.Keys(layer)), got)
		return got
	}

	if got := build("qa-r59-build-file-marker", "1"); got != "qa-r59-build-file-marker" {
		t.Fatalf("first build read %q", got)
	}
	// Same revision, changed bytes: BuildKit's cache key ignores secret
	// contents, so the warm RUN is reused — the hazard the revision exists for.
	if got := build("qa-r59-changed", "1"); got != "qa-r59-build-file-marker" {
		t.Fatalf("unchanged revision unexpectedly re-ran: %q", got)
	}
	if got := build("qa-r59-changed", "2"); got != "qa-r59-changed" {
		t.Fatalf("bumped revision reused the stale RUN: %q", got)
	}
}

// lastOCILayer reads an OCI image archive and returns its config blob and the
// entries (path → regular-file bytes) of its final layer — the native RUN's.
func lastOCILayer(t *testing.T, archive string) ([]byte, map[string][]byte) {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	blobs := map[string][]byte{}
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if blobs[hdr.Name], err = io.ReadAll(tr); err != nil {
			t.Fatal(err)
		}
	}
	type descriptor struct {
		Digest string `json:"digest"`
	}
	blob := func(d descriptor) []byte {
		b, ok := blobs["blobs/sha256/"+strings.TrimPrefix(d.Digest, "sha256:")]
		if !ok {
			t.Fatalf("archive missing blob %s", d.Digest)
		}
		return b
	}
	var index struct{ Manifests []descriptor }
	if err := json.Unmarshal(blobs["index.json"], &index); err != nil || len(index.Manifests) == 0 {
		t.Fatalf("index.json: %v", err)
	}
	var manifest struct {
		Config descriptor
		Layers []descriptor
	}
	if err := json.Unmarshal(blob(index.Manifests[0]), &manifest); err != nil || len(manifest.Layers) == 0 {
		t.Fatalf("manifest: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(blob(manifest.Layers[len(manifest.Layers)-1])))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string][]byte{}
	lr := tar.NewReader(gz)
	for {
		hdr, err := lr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if entries[strings.TrimPrefix(hdr.Name, "./")], err = io.ReadAll(lr); err != nil {
			t.Fatal(err)
		}
	}
	return blob(manifest.Config), entries
}

func TestNativeEnvironmentRejectsInvalidBase64(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "render-env")
	if err := os.WriteFile(bundle, []byte("MESSAGE=c2VjcmV0!\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o := nativeOptions()
	o.BuildCommand = "printf BUILD_RAN"
	out, err := runNativeBuildShell(t, o, bundle)
	if err == nil || len(out) != 0 {
		t.Fatalf("invalid bundle must fail silently before build: err=%v, output=%q", err, out)
	}
}

// w8/052: BuildKit names a failed RUN by its process string, which the failure
// summary quotes. The RUN must carry only the tenant's command behind one
// source line; the env loader lives in a COPY heredoc before it.
func TestNativeDockerfileRunNamesOnlyTheBuildCommand(t *testing.T) {
	o := nativeOptions()
	o.NativeEnvRevision = "1"
	var run string
	for line := range strings.SplitSeq(nativeDockerfile(o), "\n") {
		if strings.HasPrefix(line, "RUN ") {
			run = line
		}
	}
	want := `["/bin/bash","-c",". /opt/bex/load-env\nnpm ci && npm run build"]`
	if !strings.HasSuffix(run, want) {
		t.Fatalf("RUN = %s, want it to end %s", run, want)
	}
	for _, internal := range []string{"bex-native-env-rev", "while IFS", "base64 -d", "/run/secrets/render-env ["} {
		if strings.Contains(run[strings.Index(run, "["):], internal) {
			t.Errorf("RUN process string carries %q: %s", internal, run)
		}
	}
}

func TestNativeCommandFailureNamesTheCommand(t *testing.T) {
	// Captured from a real BuildKit build of the generated shape.
	tail := `#12 [stage-0 5/5] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\necho qa7-marker; exit 3"]
0.101 qa7-marker
------
ERROR: failed to build: failed to solve: process "/bin/bash -c . /opt/bex/load-env\necho qa7-marker; exit 3" did not complete successfully: exit code: 3`
	got := nativeCommandFailure(tail)
	if !strings.HasSuffix(got, "ERROR: failed to build: build command 'echo qa7-marker; exit 3' exited with code 3") ||
		!strings.Contains(got, "0.101 qa7-marker") ||
		!strings.HasPrefix(got, "#12 [stage-0 5/5] RUN build command 'echo qa7-marker; exit 3'\n") {
		t.Fatalf("rewritten tail =\n%s", got)
	}
	for _, internal := range []string{"bex-native-env-rev", "/run/secrets/render-env", "base64 -d", "while IFS= read", "load-env"} {
		if strings.Contains(got, internal) {
			t.Errorf("rewritten tail still carries %q:\n%s", internal, got)
		}
	}
	// With secret-file mounts, and BuildKit's "> [step] RUN …:" error header.
	header := `> [stage-0 5/5] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env --mount=type=secret,id=bex-file-0,target=/etc/secrets/.npmrc,required=true ["/bin/bash","-c",". /opt/bex/load-env\nnpm ci"]:`
	if got := nativeCommandFailure(header); got != "> [stage-0 5/5] RUN build command 'npm ci':" {
		t.Errorf("error header = %q", got)
	}

	// The pre-w8/052 inline loader, as production reported it.
	legacy := `error: failed to solve: process "/bin/bash -c : bex-native-env-rev=1\nwhile IFS= read -r record; do\n  [ -n \"$record\" ] || continue\n  export \"$key=${value%.}\"\ndone < /run/secrets/render-env\necho qa7-build-445879; exit 3" did not complete successfully: exit code: 3`
	if got := nativeCommandFailure(legacy); got != "error: build command 'echo qa7-build-445879; exit 3' exited with code 3" {
		t.Fatalf("legacy rewrite = %q", got)
	}

	// A Dockerfile build's own RUN, and anything unparseable, are untouched.
	for _, keep := range []string{
		`ERROR: failed to solve: process "/bin/sh -c make" did not complete successfully: exit code: 2`,
		`process "/bin/bash -c npm test" did not complete successfully: exit code: 1`,
		"plain output",
	} {
		if got := nativeCommandFailure(keep); got != keep {
			t.Errorf("nativeCommandFailure(%q) = %q, want unchanged", keep, got)
		}
	}
}
