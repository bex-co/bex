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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nativeTranscript is BuildKit's plain progress for a native Node build of
// command, abridged from the live round-84 probe (w4/m324): bex's recipe
// steps, then the build command's RUN with its output, then failure (if any).
func nativeTranscript(command, failure string) string {
	run := fmt.Sprintf(`#23 [stage-0 11/12] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\n%s"]`, command)
	out := `#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 2.10kB done
#1 DONE 0.0s
#4 [context bex-init] load .dockerignore
#4 DONE 0.0s
#6 [internal] load metadata for docker.io/mwader/static-ffmpeg:9.0.2-1@sha256:f5d1aa
#6 DONE 0.4s
#11 [stage-0  4/12] ADD --link --checksum=sha256:c729 https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz
#11 DONE 0.2s
#22 [stage-0 10/12] COPY <<BEX_LOAD_ENV /opt/bex/load-env
#22 DONE 0.0s
` + run + `
#23 2.411 added 80 packages, and audited 81 packages in 2s
#23 2.411 
#23 2.424   run ` + "`npm fund`" + ` for details
`
	if failure == "" {
		return out + `#23 DONE 2.6s
#24 exporting to oci image format
#24 exporting layers 1.2s done
#24 DONE 1.4s
`
	}
	process := strconv.Quote("/bin/bash -c . /opt/bex/load-env\n" + command)
	return out + `#23 2.413 ` + failure + `
#23 ERROR: process ` + process + ` did not complete successfully: exit code: 3
------
 > ` + strings.TrimPrefix(run, "#23 ") + `:
2.413 ` + failure + `
------
error: failed to solve: process ` + process + ` did not complete successfully: exit code: 3
`
}

// nativeCommandOutput is the command's output in nativeTranscript, as the
// build log shows it.
const nativeCommandOutput = "added 80 packages, and audited 81 packages in 2s\n\n  run `npm fund` for details\n"

// runBuildkit runs the buildkit phase BuildJob gives o, with its env as
// the kubelet resolves it and a stand-in buildctl that prints transcript and
// exits code. It returns the container's log (stdout and stderr), the
// termination message the phase left, and its exit status.
func runBuildkit(t *testing.T, o Options, transcript string, code int) (log, message string, exit int) {
	t.Helper()
	return runBuildkitWithOutput(t, o, transcript, code, t.TempDir())
}

// runBuildkitWithOutput is runBuildkit with the build's output volume at
// output, where the phase leaves uploadStartedFile.
func runBuildkitWithOutput(t *testing.T, o Options, transcript string, code int, output string) (log, message string, exit int) {
	t.Helper()
	fake := filepath.Join(t.TempDir(), "transcript")
	if err := os.WriteFile(fake, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	return runBuildkitFrom(t, o, fake, code, output)
}

// runBuildkitFrom is runBuildkitWithOutput with the stand-in buildctl
// printing what it reads from the file (or FIFO) at transcript.
func runBuildkitFrom(t *testing.T, o Options, transcript string, code int, output string) (log, message string, exit int) {
	t.Helper()
	dir := t.TempDir()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	c := cachePhaseIn(&spec, "buildkit")
	messagePath := filepath.Join(dir, "termination-log")
	script := strings.NewReplacer("/tmp/bex-", dir+"/bex-", "/dev/termination-log", messagePath, outputMount+"/", output+"/",
		"bex_run buildctl-daemonless.sh", "bex_run bex_fake_buildctl").Replace(scriptOf(t, c))
	script = "bex_fake_buildctl() { cat \"$BEX_TRANSCRIPT\"; return $BEX_CODE; }\n" + script
	cmd := exec.Command("sh", append([]string{"-eu", "-c", script, c.Name}, c.Args...)...)
	cmd.Env = append(os.Environ(), "BEX_TRANSCRIPT="+transcript, "BEX_CODE="+strconv.Itoa(code))
	values := expandedBuildValues(c.Env)
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			cmd.Env = append(cmd.Env, e.Name+"="+values[e.Name])
		}
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		ee, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			t.Fatal(err)
		}
		exit = ee.ExitCode()
	}
	raw, err := os.ReadFile(messagePath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return out.String(), string(raw), exit
}

// failedBuildkit is what EnsureBuild reports for o's build when its buildkit
// phase ended with exit and message.
func failedBuildkit(t *testing.T, o Options, exit int, message string) Observation {
	t.Helper()
	j := completedJob(o, batchv1.JobFailed)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: j.Name + "-abcde", Namespace: o.Namespace,
			Labels: map[string]string{jobNameLabel: j.Name},
		},
		Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{{
			Name: "buildkit", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: int32(exit), Message: message,
				FinishedAt: metav1.NewTime(time.Date(2026, 11, 11, 7, 50, 0, 0, time.UTC)),
			}},
		}}},
	}
	o.Client = fakeClient(j, pod)
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	return obs
}

func nativeLogOptions(command string) Options {
	o := nativeOptions()
	o.AppUID = "uid-web"
	o.BuildCommand = command
	return o
}

// w4/m324: a native build's log is the build command, its output as the
// command printed it, and the verdict: none of bex's recipe.
func TestNativeBuildLogShowsTheCommandAndItsOutput(t *testing.T) {
	// The command as configured, past the kubelet's $(VAR) and $$ expansion.
	command := `npm install && echo "$(HOME)" $$`
	log, _, exit := runBuildkit(t, nativeLogOptions(command), nativeTranscript("npm install", ""), 0)
	if exit != 0 {
		t.Fatalf("exit %d, log:\n%s", exit, log)
	}
	want := rendersShellLine("Running build command '"+command+"'...") + "\n" + nativeCommandOutput + rendersRuntimeLine("Uploading build...") + "\n"
	if log != want {
		t.Fatalf("log:\n%q\nwant:\n%q", log, want)
	}
}

// w4/582: a build command step BuildKit served from its layer cache ran
// nothing, so the log says its result was reused after naming it.
func TestNativeBuildLogSaysACachedCommandWasReused(t *testing.T) {
	command := "npm install"
	transcript := nativeTranscript(command, "")
	head, rest, _ := strings.Cut(transcript, "#23 2.411")
	_, tail, _ := strings.Cut(rest, "#24 exporting")
	cached := head + "#23 CACHED\n#24 exporting" + tail
	log, _, exit := runBuildkit(t, nativeLogOptions(command), cached, 0)
	if exit != 0 {
		t.Fatalf("exit %d, log:\n%s", exit, log)
	}
	if want := rendersShellLine("Running build command 'npm install'...") + "\n" + "==> Reusing the cached result: nothing the build command reads has changed since an earlier build\n" +
		rendersRuntimeLine("Uploading build...") + "\n"; log != want {
		t.Fatalf("log:\n%q\nwant:\n%q", log, want)
	}
}

// nativeAfterCommand is BuildKit's plain progress after a native build's
// command step, abridged: the secret-file links, then the image's export and
// the layer cache's, which take minutes on a large image (w33/m570).
const nativeAfterCommand = `#24 [stage-0 12/12] RUN set -e; for name in .env; do [ -d "./$name" ] && [ ! -L "./$name" ] || ln -sfn "/etc/secrets/$name" "./$name"; done
#24 DONE 0.1s
#25 exporting to oci image format
#25 exporting layers
#25 exporting layers 412.6s done
#25 exporting manifest sha256:0e1f5d done
#25 exporting config sha256:9a8b7c done
#25 sending tarball
#25 sending tarball 21.4s done
#25 DONE 434.1s
#26 exporting cache to client directory
#26 preparing build cache for export
#26 writing layer sha256:77aa01 3.1s done
#26 DONE 12.3s
`

// nativeExportFailure is BuildKit's plain progress for an image export that
// fails after the command step.
const nativeExportFailure = `#25 exporting to oci image format
#25 exporting layers
#25 ERROR: failed to export layer: write /var/lib/buildkit/tmp/layer: no space left on device
------
 > exporting to oci image format:
------
error: failed to solve: failed to export layer: write /var/lib/buildkit/tmp/layer: no space left on device
`

// nativeThroughCommand is nativeTranscript(command, "") up to and including
// the command step's DONE line.
func nativeThroughCommand(t *testing.T, command string) string {
	t.Helper()
	head, _, ok := strings.Cut(nativeTranscript(command, ""), "#24 exporting")
	if !ok || !strings.HasSuffix(head, "#23 DONE 2.6s\n") {
		t.Fatalf("nativeTranscript changed shape:\n%s", head)
	}
	return head
}

// monotonicNow reads the same monotonic clock monotonicNowCommand uses:
// /proc/uptime seconds on Linux, falling back to Unix epoch seconds.
func monotonicNow(t *testing.T) float64 {
	t.Helper()
	if raw, err := os.ReadFile("/proc/uptime"); err == nil {
		if fields := strings.Fields(string(raw)); len(fields) > 0 {
			if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
				return secs
			}
		}
	}
	return float64(time.Now().Unix())
}

// uploadStamp reads the moment a buildkit phase left in its output volume.
func uploadStamp(t *testing.T, output string) (float64, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(output, filepath.Base(uploadStartedFile)))
	if os.IsNotExist(err) {
		return 0, false
	}
	if err != nil {
		t.Fatal(err)
	}
	at, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil {
		t.Fatalf("upload stamp %q: %v", raw, err)
	}
	return at, true
}

// uploadCompression reads the layer compression seconds a buildkit phase left
// in its output volume.
func uploadCompression(t *testing.T, output string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(output, filepath.Base(uploadCompressionFile)))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw)), true
}

// w33/m570, w32/097: once the command's step is done, the log says the build
// is uploading, once, right after the command's last line; the image's export
// and the cache's run under it, as Render's upload does. The phase leaves that
// moment and its layer compression seconds for the push phase. A build with no
// command says it too.
func TestNativeBuildLogSaysUploadingOnceTheCommandsStepEnds(t *testing.T) {
	for name, tc := range map[string]struct{ command, want string }{
		"command":    {"npm install", rendersShellLine("Running build command 'npm install'...") + "\n" + nativeCommandOutput + rendersRuntimeLine("Uploading build...") + "\n"},
		"no command": {"", nativeCommandOutput + rendersRuntimeLine("Uploading build...") + "\n"},
	} {
		volume := t.TempDir()
		before := monotonicNow(t)
		log, _, exit := runBuildkitWithOutput(t, nativeLogOptions(tc.command), nativeThroughCommand(t, tc.command)+nativeAfterCommand, 0, volume)
		after := monotonicNow(t)
		if exit != 0 || log != tc.want {
			t.Fatalf("%s: exit %d, log:\n%q\nwant:\n%q", name, exit, log, tc.want)
		}
		if at, ok := uploadStamp(t, volume); !ok || at < before-0.02 || at > after+0.02 {
			t.Errorf("%s: upload stamp %.2f (present %v), want a moment within [%.2f, %.2f]", name, at, ok, before, after)
		}
		if comp, ok := uploadCompression(t, volume); !ok || comp != "412.6" {
			t.Errorf("%s: upload compression %q (present %v), want 412.6", name, comp, ok)
		}
	}
}

// streamingAwk puts first on PATH an awk that reads a pipe as it arrives, as
// production's does (busybox, in the BuildKit image), or skips. mawk, the CI
// image's, reads a pipe in blocks unless it is told it is interactive, and
// would print nothing that could be timed.
func streamingAwk(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, candidate := range []string{"busybox", "gawk", "mawk"} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		awk := filepath.Join(bin, "awk")
		if candidate == "mawk" {
			err = os.WriteFile(awk, []byte("#!/bin/sh\nexec "+path+" -W interactive \"$@\"\n"), 0o755)
		} else {
			err = os.Symlink(path, awk)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		return
	}
	t.Skip("no awk that can read a pipe as it arrives (busybox, gawk or mawk) on PATH")
}

// w33/m570: the line and its moment come as the command's step ends, not
// when BuildKit's export does, minutes later on a large image. The stand-in
// buildctl holds the export back until the phase has stamped the upload.
func TestNativeBuildSaysUploadingBeforeTheExportEnds(t *testing.T) {
	streamingAwk(t)
	fifo := filepath.Join(t.TempDir(), "transcript")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	head := nativeThroughCommand(t, "npm install")
	volume := t.TempDir()
	stamped := make(chan bool, 1)
	go func() {
		f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			stamped <- false
			return
		}
		defer func() { _ = f.Close() }()
		_, _ = f.WriteString(head)
		// Not uploadStamp, which may fail the test: this is not its goroutine,
		// and the stamp can be seen created before it is written.
		ok := false
		for deadline := time.Now().Add(10 * time.Second); !ok && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			_, err := os.Stat(filepath.Join(volume, filepath.Base(uploadStartedFile)))
			ok = err == nil
		}
		stamped <- ok
		_, _ = f.WriteString(nativeAfterCommand)
	}()
	log, _, exit := runBuildkitFrom(t, nativeLogOptions("npm install"), fifo, 0, volume)
	if exit != 0 || !strings.HasSuffix(log, "  run `npm fund` for details\n"+rendersRuntimeLine("Uploading build...")+"\n") {
		t.Fatalf("exit %d, log:\n%s", exit, log)
	}
	select {
	case ok := <-stamped:
		if !ok {
			t.Fatal("the phase stamped the upload only once BuildKit's export had ended")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in buildctl never wrote the transcript")
	}
}

// w33/m570: a phase that fails after the command's step, an export that
// runs out of disk or a BuildKit that dies without a word, ends its log with
// Render's upload failure instead of BuildKit's error, and never says the
// build succeeded. The reason still quotes BuildKit.
func TestNativeBuildLogSaysAnUploadThatFailsAfterTheCommand(t *testing.T) {
	o := nativeLogOptions("npm install")
	head := nativeThroughCommand(t, "npm install")
	for name, transcript := range map[string]string{"export": head + nativeExportFailure, "silent": head} {
		log, message, exit := runBuildkit(t, o, transcript, 1)
		want := rendersShellLine("Running build command 'npm install'...") + "\n" + nativeCommandOutput + rendersRuntimeLine("Uploading build...") + "\n" + "==> Uploading build failed\n"
		if exit == 0 || log != want {
			t.Fatalf("%s: exit %d, log:\n%q\nwant:\n%q", name, exit, log, want)
		}
		if name == "export" {
			if obs := failedBuildkit(t, o, exit, message); !strings.Contains(obs.Tail, "no space left on device") {
				t.Errorf("the reason's tail %q does not quote BuildKit's error", obs.Tail)
			}
		}
	}
}

// w33/m570: a phase that cannot leave the upload's moment prints no line of
// its own, and the push phase says it then, as before.
func TestNativeBuildLogLeavesTheUploadLineToThePushWithoutItsMoment(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	log, _, exit := runBuildkitWithOutput(t, nativeLogOptions("npm install"), nativeTranscript("npm install", ""), 0, missing)
	if want := rendersShellLine("Running build command 'npm install'...") + "\n" + nativeCommandOutput; exit != 0 || log != want {
		t.Fatalf("exit %d, log:\n%q\nwant:\n%q", exit, log, want)
	}
}

func writeUploadFiles(t *testing.T, stamp string, comp *string) string {
	t.Helper()
	output := t.TempDir()
	if err := os.WriteFile(filepath.Join(output, filepath.Base(uploadStartedFile)), []byte(stamp), 0o600); err != nil {
		t.Fatal(err)
	}
	if comp != nil {
		if err := os.WriteFile(filepath.Join(output, filepath.Base(uploadCompressionFile)), []byte(*comp), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return output
}

// w33/m570, w32/097: with the buildkit phase's moment and layer compression,
// the push says no second `Uploading build...` and emits Render's tenths-of-a-
// second `Uploaded in U.Us. Compression took C.Cs` where U + C matches the
// wall-clock stretch from `Uploading build...`.
func TestNativeBuildPushCountsTheUploadFromTheBuildkitPhase(t *testing.T) {
	uploadedWithComp := regexp.MustCompile("^" + strings.Replace(
		strings.Replace(regexp.QuoteMeta(rendersRuntimeLine("Uploaded in \x00s. Compression took \x01s")), "\x00", "([0-9]+\\.[0-9])", 1),
		"\x01", "([0-9]+\\.[0-9])", 1) +
		"\n" + regexp.QuoteMeta(rendersBuildSuccessful) + "\n$")

	withEachAwk(t, func(t *testing.T) {
		cases := []struct {
			name             string
			ago              float64
			comp             string
			minUpload, maxUp float64
			wantComp         string
		}{
			// Render's tretta shape: 12.3s total with 2.4s compression -> Uploaded in ~9.9s. Compression took 2.4s.
			{name: "measured", ago: 12.3, comp: "2.4\n", minUpload: 9.5, maxUp: 13.0, wantComp: "2.4"},
			// Zero compression (all layers cached: `#24 exporting layers done`) still reports 0.0s.
			{name: "zero_compression", ago: 400.2, comp: "0\n", minUpload: 399.5, maxUp: 405.0, wantComp: "0.0"},
			// Compression exceeding elapsed total clamps upload to 0.0s (never negative or -0.0s).
			{name: "clamped", ago: 12.3, comp: "50.0\n", minUpload: 0.0, maxUp: 0.0, wantComp: "50.0"},
		}
		for _, tc := range cases {
			stamp := fmt.Sprintf("%.2f\n", monotonicNow(t)-tc.ago)
			log, err := runNativePush(t, writeUploadFiles(t, stamp, &tc.comp), 0)
			m := uploadedWithComp.FindStringSubmatch(log)
			if err != nil || m == nil {
				t.Fatalf("%s: err %v, log %q; want Render's Uploaded + Compression line and the verdict", tc.name, err, log)
			}
			u, _ := strconv.ParseFloat(m[1], 64)
			if u < tc.minUpload || u > tc.maxUp || m[2] != tc.wantComp {
				t.Errorf("%s: Uploaded in %ss. Compression took %ss, want [%.1f, %.1f]s and %ss", tc.name, m[1], m[2], tc.minUpload, tc.maxUp, tc.wantComp)
			}
		}

		// Refused push prints no Uploading build..., no Uploaded in, and ends with Uploading build failed.
		fourHundredAgo := fmt.Sprintf("%.2f\n", monotonicNow(t)-400.2)
		comp := "2.4\n"
		log, err := runNativePush(t, writeUploadFiles(t, fourHundredAgo, &comp), 1)
		if err == nil || strings.Contains(log, "Uploading build...") || strings.Contains(log, "Uploaded in") || !strings.HasSuffix(log, "==> Uploading build failed\n") {
			t.Errorf("refused: err %v, log %q; want the failure alone", err, log)
		}
	})
}

// w33/m570, w32/097: without measured compression (or with an invalid
// compression file), the push falls back honestly to `Uploaded in T.Ts`. A
// start moment that is not a number is ignored, even if a stale compression
// file exists beside it.
func TestNativeBuildPushFallbackWithoutCompressionOrMoment(t *testing.T) {
	uploadedOnly := regexp.MustCompile("^" + strings.Replace(regexp.QuoteMeta(rendersRuntimeLine("Uploaded in \x00s")), "\x00", "([0-9]+\\.[0-9])", 1) +
		"\n" + regexp.QuoteMeta(rendersBuildSuccessful) + "\n$")

	withEachAwk(t, func(t *testing.T) {
		fourHundredAgo := fmt.Sprintf("%.2f\n", monotonicNow(t)-400.2)
		for _, badComp := range []*string{nil, new(""), new("\n"), new("soon\n"), new("12a\n"), new("$(echo 1)\n")} {
			log, err := runNativePush(t, writeUploadFiles(t, fourHundredAgo, badComp), 0)
			m := uploadedOnly.FindStringSubmatch(log)
			if err != nil || m == nil {
				t.Fatalf("badComp %v: err %v, log %q; want fallback Uploaded in N.Ns without Compression took", badComp, err, log)
			}
			if u, _ := strconv.ParseFloat(m[1], 64); u < 399.5 || u > 405.0 {
				t.Errorf("badComp %v: Uploaded in %ss, want ~400.2s", badComp, m[1])
			}
		}

		comp := "2.4\n"
		for _, stamp := range []string{"", "\n", "soon\n", "12a\n", "$(echo 1)\n"} {
			log, err := runNativePush(t, writeUploadFiles(t, stamp, &comp), 0)
			rest, ok := strings.CutPrefix(log, rendersRuntimeLine("Uploading build...")+"\n")
			m := uploadedOnly.FindStringSubmatch(rest)
			if err != nil || !ok || m == nil {
				t.Errorf("stamp %q: err %v, log %q; want the push's own Uploading build... and fallback Uploaded in", stamp, err, log)
				continue
			}
			if u, _ := strconv.ParseFloat(m[1], 64); u < 0 || u > 5.0 {
				t.Errorf("stamp %q: Uploaded in %ss, want the push's own short count", stamp, m[1])
			}
		}
	})
}

// w33/m570, w32/097: across its phases, a native build's log says it is
// uploading once, after the command's output, then the push's tenths-of-a-
// second upload + compression summary and verdict (or honest fallback when
// BuildKit reported no layer compression).
func TestANativeBuildSaysUploadingOnce(t *testing.T) {
	volume := t.TempDir()
	build, _, exit := runBuildkitWithOutput(t, nativeLogOptions("npm install"), nativeTranscript("npm install", ""), 0, volume)
	push, err := runNativePush(t, volume, 0)
	log := build + push
	whole := regexp.MustCompile(regexp.QuoteMeta("run `npm fund` for details\n"+rendersRuntimeLine("Uploading build...")+"\n") +
		strings.Replace(regexp.QuoteMeta(rendersRuntimeLine("Uploaded in \x00s. Compression took 1.2s")), "\x00", "[0-9]+\\.[0-9]", 1) + "\n" + regexp.QuoteMeta(rendersBuildSuccessful) + "\n$")
	if exit != 0 || err != nil || strings.Count(log, "Uploading build") != 1 || !whole.MatchString(log) {
		t.Fatalf("with compression: exit %d, push err %v, log:\n%s", exit, err, log)
	}

	// Honest fallback when BuildKit's transcript reports no layer compression.
	noLayers := strings.ReplaceAll(nativeTranscript("npm install", ""), "#24 exporting layers 1.2s done\n", "")
	fallbackVol := t.TempDir()
	build, _, exit = runBuildkitWithOutput(t, nativeLogOptions("npm install"), noLayers, 0, fallbackVol)
	push, err = runNativePush(t, fallbackVol, 0)
	log = build + push
	fallback := regexp.MustCompile(regexp.QuoteMeta("run `npm fund` for details\n"+rendersRuntimeLine("Uploading build...")+"\n") +
		strings.Replace(regexp.QuoteMeta(rendersRuntimeLine("Uploaded in \x00s")), "\x00", "[0-9]+\\.[0-9]", 1) + "\n" + regexp.QuoteMeta(rendersBuildSuccessful) + "\n$")
	if exit != 0 || err != nil || strings.Count(log, "Uploading build") != 1 || !fallback.MatchString(log) {
		t.Fatalf("without compression: exit %d, push err %v, log:\n%s", exit, err, log)
	}
}

// runNativePush runs a native build's push phase with the build's output
// volume at output and a stand-in skopeo that prints a blob line, then
// skopeoRegistryRefusal's records when code is not 0, and exits code.
func runNativePush(t *testing.T, output string, code int) (string, error) {
	t.Helper()
	o := nativeLogOptions("npm install")
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	push := cachePhaseIn(&spec, pushContainer)
	if len(push.Command) != 4 || push.Command[0] != "sh" || push.Command[3] != pushContainer {
		t.Fatalf("native push command = %q, want the narration wrapper", push.Command)
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\necho 'Copying blob sha256:abc'\n"
	if code != 0 {
		fake += skopeoRegistryRefusal
	}
	if err := os.WriteFile(filepath.Join(bin, "skopeo"), []byte(fake+"exit "+strconv.Itoa(code)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(push.Command[2], outputMount+"/", output+"/")
	cmd := exec.Command("sh", append([]string{"-c", script, pushContainer}, push.Args...)...)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// w4/582: the verdict follows the upload, as Render's does, so a push that
// fails never reads "Build successful"; skopeo's blob transcript stays out.
// With no moment left by the buildkit phase, the push says it is uploading.
func TestNativeBuildPushSaysSuccessOnlyAfterTheUpload(t *testing.T) {
	for name, code := range map[string]int{"pushed": 0, "refused": 1} {
		log, err := runNativePush(t, t.TempDir(), code)
		if strings.Contains(log, "Copying blob") || !strings.HasPrefix(log, rendersRuntimeLine("Uploading build...")+"\n") {
			t.Fatalf("%s: log %q, want the narration without skopeo's transcript", name, log)
		}
		if code == 0 {
			if err != nil || !strings.Contains(shown(log), "==> Uploaded in ") || !strings.HasSuffix(log, rendersBuildSuccessful+"\n") {
				t.Fatalf("pushed: err %v log %q", err, log)
			}
			continue
		}
		if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != code || strings.Contains(log, "Build successful") ||
			!strings.Contains(log, "==> The transfer to the build registry failed; retrying (1 of 3)\n") || !strings.HasSuffix(log, "==> Uploading build failed\n") {
			t.Fatalf("refused: err %v log %q, want exit %d, the retry and the failure in product words, and no verdict", err, log, code)
		}
		assertNoRegistryInternals(t, name, log)
	}
}

// skopeoRegistryRefusal is what skopeo writes to stderr when the build
// registry refuses its connections (w8/286, sweep 81af): a retry record, then
// the fatal one, naming the registry's cluster host, its address and a digest.
const skopeoRegistryRefusal = `echo 'time="2026-11-12T09:35:02Z" level=warning msg="Failed, retrying in 1s ... (1/3). Error: trying to reuse blob sha256:4f4fb700ef54 at destination: pinging container registry zot.bex-registry.svc:5000: Get \"http://zot.bex-registry.svc:5000/v2/\": dial tcp 10.96.12.34:5000: connect: connection refused"' >&2
echo 'time="2026-11-12T09:35:13Z" level=fatal msg="trying to reuse blob sha256:4f4fb700ef54 at destination: pinging container registry zot.bex-registry.svc:5000: Get \"http://zot.bex-registry.svc:5000/v2/\": dial tcp 10.96.12.34:5000: connect: connection refused"' >&2
`

func assertNoRegistryInternals(t *testing.T, name, log string) {
	t.Helper()
	for _, internal := range []string{"zot", ".svc", "10.96.12.34", "sha256:4f4f", "level=", "msg="} {
		if strings.Contains(log, internal) {
			t.Errorf("%s: the build log names %q:\n%s", name, internal, log)
		}
	}
}

// w8/286: a Dockerfile build's push keeps skopeo's transcript and loses its
// error records too, and every cache phase's skopeo does.
func TestEveryBuildSkopeoKeepsRegistryInternalsOutOfTheLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skopeo"), []byte("#!/bin/sh\necho 'Copying blob sha256:abc'\n"+skopeoRegistryRefusal+"exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(script string) (string, error) {
		cmd := exec.Command("sh", "-c", script, "phase", "copy", "a", "b")
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	log, err := run(dockerfilePush)
	if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != 1 || !strings.HasPrefix(log, "Copying blob sha256:abc\n") ||
		!strings.HasSuffix(log, "==> Uploading build failed\n") {
		t.Fatalf("Dockerfile push: err %v log %q", err, log)
	}
	assertNoRegistryInternals(t, "Dockerfile push", log)
	for name, script := range map[string]string{"cache save": cacheBestEffort, "cache restore": cacheRestoreBestEffort, "cache purge": cachePurgeBestEffort} {
		log, err := run(script)
		if err != nil || !strings.Contains(log, "bex: ") {
			t.Fatalf("%s: err %v log %q, want its own product line and a clean exit", name, err, log)
		}
		assertNoRegistryInternals(t, name, log)
	}
}

// A failing command shows its output and its exit code, and the failure reason
// still reads BuildKit's own words through the termination message: the
// command, its unstamped output, and its exit code.
func TestNativeBuildLogNamesAFailedCommand(t *testing.T) {
	command := "npm install && echo QA84 failing on purpose && exit 3"
	o := nativeLogOptions(command)
	output := t.TempDir()
	log, message, exit := runBuildkitWithOutput(t, o, nativeTranscript(command, "QA84 failing on purpose"), 1, output)
	if exit != ExitTenantError {
		t.Fatalf("exit %d, want the tenant-error class %d; log:\n%s", exit, ExitTenantError, log)
	}
	if !strings.HasSuffix(log, "QA84 failing on purpose\nerror: build command '"+command+"' exited with code 3\n") {
		t.Errorf("log does not end with the command's output and exit code:\n%s", log)
	}
	// Nothing is uploaded after a command that failed (w33/m570, w32/097).
	if _, err := os.Stat(filepath.Join(output, filepath.Base(uploadStartedFile))); !os.IsNotExist(err) {
		t.Errorf("a failed command left the upload's moment (stat: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(output, filepath.Base(uploadCompressionFile))); !os.IsNotExist(err) {
		t.Errorf("a failed command left upload compression (stat: %v)", err)
	}
	for _, internal := range []string{"#", "sha256:", "static-ffmpeg", "bex-init", "/opt/bex/load-env", "--mount=type=secret", "Build successful", "Uploading"} {
		if strings.Contains(log, internal) {
			t.Errorf("log shows %q:\n%s", internal, log)
		}
	}
	obs := failedBuildkit(t, o, exit, message)
	wantTail := "QA84 failing on purpose\nerror: build command '" + command + "' exited with code 3"
	if obs.FailedStep != nativeBuildCommandStep || obs.Tail != wantTail {
		t.Errorf("FailedStep, Tail = %q, %q; want %q, %q", obs.FailedStep, obs.Tail, nativeBuildCommandStep, wantTail)
	}
}

// A failure outside the command is named in the log, and the reason still
// recognizes a toolchain download bex supplies (w8/m148).
func TestNativeBuildLogNamesAToolchainFailure(t *testing.T) {
	o := nativeLogOptions("npm install")
	log, message, exit := runBuildkit(t, o, hugoDigestMismatch, 1)
	if !strings.Contains(log, "==> Preparing the build failed: digest mismatch sha256:bef140a1") ||
		!strings.Contains(log, "==> error: failed to solve: digest mismatch") || strings.Contains(log, "Running build command") {
		t.Fatalf("a failure outside the command is not named:\n%s", log)
	}
	obs := failedBuildkit(t, o, exit, message)
	if obs.FailedStep != nativeBuildEnvironmentStep || obs.PlatformFetch == nil || obs.PlatformFetch.Tool != "Hugo toolchain" {
		t.Errorf("FailedStep, PlatformFetch = %q, %+v; want the build environment and the Hugo download", obs.FailedStep, obs.PlatformFetch)
	}
}

// What buildctl prints before BuildKit's first step (a daemon that never came
// up) passes through, so the log never goes silent on a failure.
func TestNativeBuildLogKeepsWhatPrecedesBuildKit(t *testing.T) {
	daemon := "could not connect to unix:///run/user/1000/buildkit/buildkitd.sock after 10 trials\n"
	log, _, _ := runBuildkit(t, nativeLogOptions("npm install"), daemon, 1)
	if log != daemon {
		t.Fatalf("log = %q, want %q", log, daemon)
	}
}

// A Dockerfile build's log is BuildKit's plain output, as Render shows it.
func TestDockerfileBuildLogIsBuildKitsOutput(t *testing.T) {
	transcript := "#1 [internal] load build definition from Dockerfile\n#1 DONE 0.0s\n#9 [2/3] RUN make\n#9 0.087 compiling\n#9 DONE 0.1s\n"
	log, _, exit := runBuildkit(t, opts(), transcript, 0)
	if exit != 0 || log != transcript {
		t.Fatalf("exit %d, log:\n%q\nwant:\n%q", exit, log, transcript)
	}
}
