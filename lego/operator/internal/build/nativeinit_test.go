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
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func nativeInitOptions() Options {
	o := nativeOptions()
	o.NativeResolverImage = "registry.example/bex@sha256:" + strings.Repeat("0", 64)
	return o
}

// Every native service whose build has the resolve phase starts under
// native-init: the resolve phase stages it, BuildKit receives it as a named
// context, and the image runs it by absolute path in front of the unchanged
// CMD, on every line of every runtime. The image is labelled as one whose
// native-init names its command (w32/m575). Without the resolve phase, or for
// a static site, nothing about the launch changes and there is no label.
func TestNativeServiceStartsUnderNativeInit(t *testing.T) {
	for runtime, toolchain := range nativeToolchains {
		o := nativeInitOptions()
		o.Runtime = runtime
		tail := "\nLABEL co.bex.native-init.announces=\"true\"\nCOPY --link --from=bex-init /native-init /\nENTRYPOINT [\"/native-init\"]\nCMD " + shellJSON(o.StartCommand) + "\n"
		for _, line := range toolchain.lines {
			t.Run(runtime+"/"+line.line, func(t *testing.T) {
				env, files := nativeLineSignal(runtime, line.line)
				df, choices := resolveFixture(t, o, env, files)
				if choices[0].Line != line.line {
					t.Fatalf("resolved line %s, want %s", choices[0].Line, line.line)
				}
				if !strings.HasSuffix(df, tail) || strings.Count(df, nativeInitPath) != 2 || strings.Index(df, nativeBuildRun) > strings.Index(df, tail) {
					t.Errorf("service must end with native-init in front of its exact CMD:\n%s", df)
				}
			})
		}
		job := BuildJob(o, o.ImageRef()).Spec.Template.Spec
		args := strings.Join(containerByName(job.InitContainers, "buildkit").Args, " ")
		if !strings.Contains(args, "--local bex-init=/native/init --opt context:bex-init=local:bex-init") {
			t.Errorf("%s: buildctl args do not carry the bex-init context: %s", runtime, args)
		}
		resolver := containerByName(job.InitContainers, nativeResolveContainer)
		if i := slices.IndexFunc(resolver.Env, func(e corev1.EnvVar) bool { return e.Name == NativeInitEnv }); i < 0 ||
			resolver.Env[i].Value != "/native/init/native-init" {
			t.Errorf("%s: resolver env = %v, want %s=/native/init/native-init", runtime, resolver.Env, NativeInitEnv)
		}
	}

	static := nativeInitOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	for name, other := range map[string]Options{"no resolve phase": nativeOptions(), "static": static} {
		job := BuildJob(other, other.ImageRef()).Spec.Template.Spec
		if df := nativeDockerfile(other); strings.Contains(df, "native-init") || strings.Contains(df, nativeInitPath) || strings.Contains(df, NativeInitAnnouncesLabel) {
			t.Errorf("%s: Dockerfile gained native-init:\n%s", name, df)
		}
		if args := strings.Join(containerByName(job.InitContainers, "buildkit").Args, " "); strings.Contains(args, nativeInitContext) {
			t.Errorf("%s: buildctl args gained the bex-init context: %s", name, args)
		}
		if resolver := containerByName(job.InitContainers, nativeResolveContainer); resolver != nil &&
			slices.ContainsFunc(resolver.Env, func(e corev1.EnvVar) bool { return e.Name == NativeInitEnv }) {
			t.Errorf("%s: resolver stages native-init", name)
		}
	}
}

func TestStageNativeInit(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "native-init")
	must(t, os.WriteFile(src, []byte("binary"), 0o700))
	release := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	must(t, os.Chtimes(src, release, release))
	dest := filepath.Join(dir, "native", "init", "native-init")
	if err := StageNativeInit(src, dest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dest)
	got, _ := os.ReadFile(dest)
	if err != nil || info.Mode().Perm() != 0o755 || string(got) != "binary" || !info.ModTime().Equal(release) {
		t.Fatalf("staged %v %v %q, want an executable copy keeping the release's mtime", info, err, got)
	}
	if err := StageNativeInit(filepath.Join(dir, "absent"), filepath.Join(dir, "other")); err == nil {
		t.Error("a missing binary must fail the resolve phase")
	}
}

// TestNativeNodeServiceDrainsUnderNpm is w8/m88's execution gate: the
// generated image, built by the pinned BuildKit with the Job's exact buildctl
// args, starts an app through `npm start` and is stopped mid-request. The app
// receives SIGTERM, finishes the response and closes before the container
// exits. Opt-in: needs a privileged Docker daemon.
func TestNativeNodeServiceDrainsUnderNpm(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeInitOptions()
	o.BuildCommand, o.StartCommand, o.BuildEnv = "node --version", "npm --prefix qa start", nil
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	native := filepath.Dir(dockerfile)
	if _, err := resolveNativeBuild(nativeRuntime(o), bundle, dockerfile, src); err != nil {
		t.Fatal(err)
	}
	must(t, os.MkdirAll(filepath.Join(src, "qa"), 0o755))
	must(t, os.WriteFile(filepath.Join(src, "qa", "package.json"), []byte(`{"name":"qa","scripts":{"start":"node server.js"}}`), 0o644))
	must(t, os.WriteFile(filepath.Join(src, "qa", "server.js"), []byte(`const http = require("http");
const s = http.createServer((req, res) => {
  if (req.url !== "/hold") return res.end("ok");
  console.log("hold_begin");
  setTimeout(() => { res.end("held"); console.log("hold_end"); }, 4000);
});
process.on("SIGTERM", () => { console.log("SIGTERM"); s.close(() => console.log("closed")); });
s.listen(process.env.PORT, () => console.log("listening"));
`), 0o644))

	name := fmt.Sprintf("bex-native-init-it-%d", os.Getpid())
	image := loadNativeInitImage(t, o, src, native, name)

	if out, err := exec.Command("docker", "run", "--rm", "-e", "PATH=", image, "/bin/bash", "-c", `echo "started PATH=[$PATH]"`).CombinedOutput(); err != nil ||
		!strings.Contains(string(out), "started PATH=[]") {
		t.Fatalf("native-init with PATH= = %v\n%s; want the command started with PATH still empty", err, out)
	}

	// The image's own CMD first, then the same app behind the other package
	// managers and launched directly; each must finish its in-flight response.
	for i, start := range [][]string{
		nil,
		{"/bin/bash", "-c", "pnpm --dir qa start"},
		{"/bin/bash", "-c", "yarn --cwd qa start"},
		{"/bin/bash", "-c", "node qa/server.js"},
	} {
		t.Run(cmp.Or(strings.Join(start, " "), "image CMD"), func(t *testing.T) {
			t.Parallel()
			got, order := stopMidRequest(t, image, fmt.Sprintf("%s-%d", name, i), start...)
			at := func(s string) int { return strings.Index(order, s) }
			if got != "200 held" || at("hold_begin") < 0 || at("SIGTERM") < at("hold_begin") ||
				at("hold_end") < at("SIGTERM") || at("closed") < at("hold_end") || strings.Count(order, "SIGTERM") != 1 {
				t.Errorf("in-flight response %q; logs must show hold_begin, one SIGTERM, hold_end, closed:\n%s", got, order)
			}
		})
	}
}

// loadNativeInitImage builds o's resolved native Dockerfile with native-init
// staged, as the Job does, loads the image into the local daemon and returns
// its ID.
func loadNativeInitImage(t *testing.T, o Options, src, native, name string) string {
	t.Helper()
	must(t, StageNativeInit(compileNativeInit(t), filepath.Join(native, "init", nativeInitFile)))
	out := t.TempDir()
	runJobBuildctl(t, o, src, native, out)
	return loadOCIArchive(t, out, name)
}

// loadOCIArchive loads the OCI archive a build wrote to out/image.tar into the
// local daemon as name and returns its ID, removed when the test ends. The
// Job pushes the archive with skopeo; here skopeo converts it for the local
// daemon instead.
func loadOCIArchive(t *testing.T, out, name string) string {
	t.Helper()
	if log, err := exec.Command("docker", "run", "--rm", "-v", out+":/out", defaultPushImage, "copy",
		"oci-archive:/out/image.tar", "docker-archive:/out/docker.tar:"+name+":latest").CombinedOutput(); err != nil {
		t.Fatalf("convert: %v\n%s", err, log)
	}
	loaded, err := exec.Command("docker", "load", "-q", "-i", filepath.Join(out, "docker.tar")).CombinedOutput()
	if err != nil {
		t.Fatalf("load: %v\n%s", err, loaded)
	}
	image := strings.TrimSpace(string(loaded))
	image = image[strings.LastIndexByte(image, ' ')+1:]
	t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", image).Run() })
	return image
}

// A Python service started by a script that runs the app without exec, or an
// app with no SIGTERM handler, stops on SIGTERM well inside the grace period
// (w8/290): native-init is PID 1, so neither bash nor the app is.
func TestNativePythonServiceStopsOnSIGTERM(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeInitOptions()
	o.Runtime = nativePythonRuntime
	o.BuildCommand, o.StartCommand, o.BuildEnv = "python3 --version", "./start.sh", nil
	bundle, dockerfile, src := writeResolveFixture(t, o, nil, nil)
	native := filepath.Dir(dockerfile)
	if _, err := resolveNativeBuild(nativeRuntime(o), bundle, dockerfile, src); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(filepath.Join(src, "start.sh"), []byte("#!/bin/bash\necho \"script pid=$$\"\npython3 app.py\n"), 0o755))
	must(t, os.WriteFile(filepath.Join(src, "app.py"), []byte(`import http.server, os, signal, sys
if os.environ.get("QA_HANDLER") == "1":
    signal.signal(signal.SIGTERM, lambda *_: (print("SIGTERM", flush=True), sys.exit(0)))
srv = http.server.HTTPServer(("0.0.0.0", int(os.environ.get("PORT", "10000"))), http.server.SimpleHTTPRequestHandler)
print("listening pid=%d" % os.getpid(), flush=True)
srv.serve_forever()
`), 0o644))
	name := fmt.Sprintf("bex-native-init-py-it-%d", os.Getpid())
	image := loadNativeInitImage(t, o, src, native, name)

	for i, tc := range []struct {
		name, handler string
		command       []string
	}{
		{"script without exec, app with a handler", "1", nil},
		{"app with no handler, launched directly", "0", []string{"/bin/bash", "-c", "python3 app.py"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := fmt.Sprintf("%s-%d", name, i)
			took, logs := stopAfterListening(t, image, container, tc.handler, tc.command...)
			status, err := exec.Command("docker", "inspect", "-f", "{{.State.ExitCode}}", container).Output()
			if err != nil {
				t.Fatal(err)
			}
			if took > 10*time.Second || strings.TrimSpace(string(status)) == "137" ||
				(tc.handler == "1" && !strings.Contains(logs, "SIGTERM")) || strings.Contains(logs, "pid=1\n") {
				t.Errorf("stop took %v (grace 20s), exit %s; the app must stop on SIGTERM, with native-init as PID 1:\n%s",
					took, strings.TrimSpace(string(status)), logs)
			}
		})
	}
}

// stopAfterListening runs image with QA_HANDLER=handler (and command in place
// of its CMD, if given), waits for it to listen, stops the container with a
// 20s grace period and returns how long the stop took and the container log.
func stopAfterListening(t *testing.T, image, name, handler string, command ...string) (time.Duration, string) {
	t.Helper()
	read := runUntilListening(t, image, name, []string{"-e", "QA_HANDLER=" + handler}, command...)
	start := time.Now()
	if out, err := exec.Command("docker", "stop", "-t", "20", name).CombinedOutput(); err != nil {
		t.Fatalf("stop: %v\n%s", err, out)
	}
	return time.Since(start), read()
}

// runUntilListening starts image as container name, with runFlags before the
// image and command in place of its CMD, waits for it to log "listening" and
// returns a reader of its log.
func runUntilListening(t *testing.T, image, name string, runFlags []string, command ...string) func() string {
	t.Helper()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	args := append(append([]string{"run", "-d", "--name", name}, runFlags...), image)
	if out, err := exec.Command("docker", append(args, command...)...).CombinedOutput(); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	read := func() string { out, _ := exec.Command("docker", "logs", name).CombinedOutput(); return string(out) }
	waitForLog(t, read, "listening", 30*time.Second)
	return read
}

// waitForLog polls read until it contains marker, failing after within.
func waitForLog(t *testing.T, read func() string, marker string, within time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(within); !strings.Contains(read(), marker); time.Sleep(250 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %s within %v:\n%s", marker, within, read())
		}
	}
}

// stopMidRequest runs image (with command in place of its CMD, if given),
// starts a request the app holds for 4s, stops the container with a 20s grace
// period and returns that response and the container log.
func stopMidRequest(t *testing.T, image, name string, command ...string) (response, logs string) {
	t.Helper()
	read := runUntilListening(t, image, name, []string{"-p", "127.0.0.1::10000"}, command...)
	port, err := exec.Command("docker", "port", name, "10000").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + strings.TrimSpace(strings.Split(string(port), "\n")[0])
	held := make(chan string, 1)
	go func() {
		res, err := (&http.Client{Timeout: 30 * time.Second}).Get(base + "/hold")
		if err != nil {
			held <- err.Error()
			return
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		held <- fmt.Sprintf("%d %s", res.StatusCode, body)
	}()
	waitForLog(t, read, "hold_begin", 10*time.Second)
	if out, err := exec.Command("docker", "stop", "-t", "20", name).CombinedOutput(); err != nil {
		t.Fatalf("stop: %v\n%s", err, out)
	}
	return <-held, read()
}

// compileNativeInit builds native-init for the Docker daemon's platform the
// way lego/Dockerfile does and returns the binary's path.
func compileNativeInit(t *testing.T) string {
	t.Helper()
	arch, err := exec.Command("docker", "version", "-f", "{{.Server.Arch}}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return compileNativeInitForArch(t, strings.TrimSpace(string(arch)))
}

// Explicit qualification platforms use the same compiler as daemon-default tests.
func compileNativeInitForArch(t *testing.T, arch string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), nativeInitFile)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	compile := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", bin, "../../cmd/native-init")
	compile.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native-init: %v\n%s", err, out)
	}
	return bin
}

// Two builds that stage the same native-init get the same layer, so a release
// adds one blob to the registry, not one per build. Opt-in: needs Docker.
func TestNativeInitLayerIsReproducible(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build the layer")
	}
	bin := compileNativeInit(t)
	copyLine := strings.SplitAfter(nativeEntrypoint(nativeInitOptions()), "\n")[0]
	digests := make([]string, 0, 2)
	for i := range 2 {
		dir := t.TempDir()
		must(t, StageNativeInit(bin, filepath.Join(dir, "init", nativeInitFile)))
		must(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"+copyLine), 0o644))
		tag := fmt.Sprintf("bex-native-init-layer-%d:%d", i, os.Getpid())
		t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
		time.Sleep(1100 * time.Millisecond) // a later staging gets a later clock
		build := exec.Command("docker", "build", "-q", "--no-cache", "--build-context", nativeInitContext+"="+filepath.Join(dir, "init"), "-t", tag, dir)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		out, err := exec.Command("docker", "image", "inspect", "--format", "{{range .RootFS.Layers}}{{println .}}{{end}}", tag).Output()
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, strings.TrimSpace(string(out)))
	}
	if digests[0] != digests[1] || strings.Contains(digests[0], "\n") {
		t.Fatalf("native-init layers %q, want one identical layer", digests)
	}
}

// w32/093: a native Node service built with a Bun lockfile, .bun-version or
// BUN_VERSION writes the resolved Bun version pair into the image for
// native-init to print before `==> Running '<command>'` at every start (without
// any build-time lookup note). A Node service with no Bun choice, a static
// site, and every other native runtime write no start-lines file.
func TestNativeNodeBunWritesStartLinesForNativeInit(t *testing.T) {
	const bunDocs = "==> Docs on specifying a Bun version: https://render.com/docs/bun-version"
	at := func(s string) time.Time {
		tm, err := time.Parse(time.RFC3339, s)
		must(t, err)
		return tm
	}
	wantStep := func(lines ...string) string {
		return "COPY --chmod=0644 <<'" + nativeInitStartLinesEOF + "' " + nativeInitStartLinesPath + "\n" +
			strings.Join(lines, "\n") + "\n" + nativeInitStartLinesEOF + "\n"
	}

	// Pin constant agreement with cmd/native-init/main.go, which cannot import
	// internal/build without bloating the static binary.
	initSrc, err := os.ReadFile("../../cmd/native-init/main.go")
	must(t, err)
	if !strings.Contains(string(initSrc), fmt.Sprintf("startLinesFile = %q", nativeInitStartLinesPath)) {
		t.Fatalf("cmd/native-init/main.go startLinesFile drifted from %q", nativeInitStartLinesPath)
	}

	for _, tc := range []struct {
		name      string
		created   time.Time
		env       map[string]string
		files     map[string]string
		digests   map[string]string
		exactErr  error
		buildOnly bool
		wantLines []string
	}{
		{
			name:      "bun.lock dated default resolved",
			created:   at("2026-04-19T00:00:00Z"),
			files:     map[string]string{"bun.lock": "{}"},
			digests:   map[string]string{bunRepo + ":1.3.4": exactDigest},
			wantLines: []string{"==> Using Bun version 1.3.4 (default)", bunDocs},
		},
		{
			name:      "bun.lock dated default fallback omits build note",
			created:   at("2026-11-21T00:00:00Z"),
			files:     map[string]string{"bun.lock": "{}"},
			wantLines: []string{"==> Using Bun version 1.4.4 (default)", bunDocs},
		},
		{
			name:      "bun.lockb dated default resolved",
			created:   at("2026-11-21T00:00:00Z"),
			files:     map[string]string{"bun.lockb": "\x00"},
			digests:   map[string]string{bunRepo + ":1.4.5": exactDigest},
			wantLines: []string{"==> Using Bun version 1.4.5 (default)", bunDocs},
		},
		{
			name:      ".bun-version reviewed line",
			files:     map[string]string{".bun-version": "1.3\n"},
			wantLines: []string{"==> Using Bun 1.3.14 (from .bun-version)", bunDocs},
		},
		{
			name:      ".bun-version unreviewed exact release",
			files:     map[string]string{".bun-version": "1.1.38\n"},
			digests:   map[string]string{bunRepo + ":1.1.38": exactDigest},
			wantLines: []string{"==> Using Bun 1.1.38 (from .bun-version)", bunDocs},
		},
		{
			name:      "BUN_VERSION exact release",
			env:       map[string]string{"BUN_VERSION": "1.3.0"},
			digests:   map[string]string{bunRepo + ":1.3.0": exactDigest},
			wantLines: []string{"==> Using Bun 1.3.0 (from BUN_VERSION=1.3.0)", bunDocs},
		},
		{
			name:      "BUN_VERSION canary omits digest note",
			env:       map[string]string{"BUN_VERSION": "canary"},
			digests:   map[string]string{bunRepo + ":canary": exactDigest},
			wantLines: []string{"==> Using Bun canary (from BUN_VERSION=canary)", bunDocs},
		},
		{
			name:      "BUN_VERSION multiline range collapses safely inside heredoc",
			env:       map[string]string{"BUN_VERSION": ">=1.2.0\n<=1.3.14"},
			wantLines: []string{"==> Using Bun 1.3.14 (from BUN_VERSION=>=1.2.0 <=1.3.14)", bunDocs},
		},
		{
			name:      "build source Node image carries start lines for linked services",
			created:   at("2026-04-19T00:00:00Z"),
			files:     map[string]string{"bun.lock": "{}"},
			digests:   map[string]string{bunRepo + ":1.3.4": exactDigest},
			buildOnly: true,
			wantLines: []string{"==> Using Bun version 1.3.4 (default)", bunDocs},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeInitOptions()
			o.Runtime = nativeNodeRuntime
			o.BuildOnly = tc.buildOnly
			if tc.buildOnly {
				o.StartCommand = ""
			}
			bundle, dockerfile, src := writeResolveFixture(t, o, tc.env, tc.files)
			fake := &fakeExact{digests: tc.digests, err: tc.exactErr}
			choices, err := ResolveNativeBuildExact(t.Context(), fake, tc.created, o.Runtime, bundle, dockerfile, src)
			must(t, err)
			if len(choices) != 2 || choices[1].Label != "Bun" {
				t.Fatalf("choices = %+v, want Node + Bun", choices)
			}
			raw, err := os.ReadFile(dockerfile)
			must(t, err)
			df := string(raw)
			want := wantStep(tc.wantLines...)
			if !strings.Contains(df, want) {
				t.Fatalf("Dockerfile missing start lines step:\nwant:\n%s\ngot:\n%s", want, df)
			}
			// Must sit after Bun binary copy and before COPY . . so source edits keep it cached.
			bunAt := strings.Index(df, "/usr/local/bin/bunx\n")
			stepAt := strings.Index(df, want)
			copyAt := strings.Index(df, "\nCOPY . .\n")
			if bunAt < 0 || stepAt < bunAt || copyAt < stepAt {
				t.Fatalf("start lines step out of order (bun=%d step=%d copy=%d):\n%s", bunAt, stepAt, copyAt, df)
			}
		})
	}

	// Node without Bun writes no start-lines file.
	t.Run("node without bun", func(t *testing.T) {
		o := nativeInitOptions()
		o.Runtime = nativeNodeRuntime
		df, choices := resolveFixture(t, o, nil, nil)
		if len(choices) != 1 || strings.Contains(df, nativeInitStartLinesPath) {
			t.Fatalf("node without bun wrote start lines (choices=%d):\n%s", len(choices), df)
		}
	})

	// Static site with bun.lock never runs native-init, so writes no start-lines file.
	t.Run("static site with bun.lock", func(t *testing.T) {
		o := nativeInitOptions()
		o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
		bundle, dockerfile, src := writeResolveFixture(t, o, nil, map[string]string{"bun.lock": "{}"})
		_, err := ResolveNativeBuildExact(t.Context(), &fakeExact{}, time.Time{}, nativeRuntime(o), bundle, dockerfile, src)
		must(t, err)
		raw, err := os.ReadFile(dockerfile)
		must(t, err)
		if strings.Contains(string(raw), nativeInitStartLinesPath) {
			t.Fatalf("static site wrote native-init start lines:\n%s", raw)
		}
	})

	// Every non-Node native runtime with Bun still installs Bun, but writes no start-lines file.
	for runtime := range nativeToolchains {
		if runtime == nativeNodeRuntime {
			continue
		}
		t.Run(runtime+" with bun", func(t *testing.T) {
			o := nativeInitOptions()
			o.Runtime = runtime
			df, _ := resolveFixture(t, o, map[string]string{"BUN_VERSION": "1.3"}, map[string]string{"bun.lock": "{}"})
			if !strings.Contains(df, "/usr/local/bin/bun\n") {
				t.Fatalf("%s lost Bun binary:\n%s", runtime, df)
			}
			if strings.Contains(df, nativeInitStartLinesPath) {
				t.Fatalf("%s wrote native-init start lines:\n%s", runtime, df)
			}
		})
	}
}
