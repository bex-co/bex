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
	"strconv"
	"strings"
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

// runBuildkit runs the buildkit phase BuildJob gives o, with its env as
// the kubelet resolves it and a stand-in buildctl that prints transcript and
// exits code. It returns the container's log (stdout and stderr), the
// termination message the phase left, and its exit status.
func runBuildkit(t *testing.T, o Options, transcript string, code int) (log, message string, exit int) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "transcript")
	if err := os.WriteFile(fake, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	c := cachePhaseIn(&spec, "buildkit")
	messagePath := filepath.Join(dir, "termination-log")
	script := strings.NewReplacer("/tmp/bex-", dir+"/bex-", "/dev/termination-log", messagePath,
		"bex_run buildctl-daemonless.sh", "bex_run bex_fake_buildctl").Replace(scriptOf(t, c))
	script = "bex_fake_buildctl() { cat \"$BEX_TRANSCRIPT\"; return $BEX_CODE; }\n" + script
	cmd := exec.Command("sh", append([]string{"-eu", "-c", script, c.Name}, c.Args...)...)
	cmd.Env = append(os.Environ(), "BEX_TRANSCRIPT="+fake, "BEX_CODE="+strconv.Itoa(code))
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
	want := "==> Running build command '" + command + "'...\n" +
		"added 80 packages, and audited 81 packages in 2s\n" +
		"\n" +
		"  run `npm fund` for details\n"
	if log != want {
		t.Fatalf("log:\n%q\nwant:\n%q", log, want)
	}
}

// w4/582: a build command step BuildKit served from its layer cache ran
// nothing, so the log says its result was reused after naming it.
func TestNativeBuildLogSaysACachedCommandWasReused(t *testing.T) {
	command := "npm install"
	transcript := nativeTranscript(command, "")
	i := strings.Index(transcript, "#23 2.411")
	j := strings.Index(transcript, "#24 exporting")
	cached := transcript[:i] + "#23 CACHED\n" + transcript[j:]
	log, _, exit := runBuildkit(t, nativeLogOptions(command), cached, 0)
	if exit != 0 {
		t.Fatalf("exit %d, log:\n%s", exit, log)
	}
	if want := "==> Running build command 'npm install'...\n==> Reusing the cached result: nothing the build command reads has changed since an earlier build\n"; log != want {
		t.Fatalf("log:\n%q\nwant:\n%q", log, want)
	}
}

// w4/582: the verdict follows the upload, as Render's does, so a push that
// fails never reads "Build successful"; skopeo's blob transcript stays out.
func TestNativeBuildPushSaysSuccessOnlyAfterTheUpload(t *testing.T) {
	spec := BuildJob(nativeLogOptions("npm install"), nativeLogOptions("npm install").ImageRef()).Spec.Template.Spec
	push := cachePhaseIn(&spec, pushContainer)
	if len(push.Command) != 4 || push.Command[0] != "sh" || push.Command[3] != pushContainer {
		t.Fatalf("native push command = %q, want the narration wrapper", push.Command)
	}
	dir := t.TempDir()
	for name, code := range map[string]int{"pushed": 0, "refused": 1} {
		bin := filepath.Join(dir, name)
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		fake := "#!/bin/sh\necho 'Copying blob sha256:abc'\necho 'denied: requested access to the resource is denied' >&2\nexit " + strconv.Itoa(code) + "\n"
		if err := os.WriteFile(filepath.Join(bin, "skopeo"), []byte(fake), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(push.Command[0], append(push.Command[1:], push.Args...)...)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		log := string(out)
		if strings.Contains(log, "Copying blob") || !strings.HasPrefix(log, "==> Uploading build...\n") {
			t.Fatalf("%s: log %q, want the narration without skopeo's transcript", name, log)
		}
		if code == 0 {
			if err != nil || !strings.Contains(log, "==> Uploaded in ") || !strings.HasSuffix(log, "==> Build successful \U0001F389\n") {
				t.Fatalf("pushed: err %v log %q", err, log)
			}
			continue
		}
		if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != code || strings.Contains(log, "Build successful") || !strings.Contains(log, "denied") {
			t.Fatalf("refused: err %v log %q, want exit %d, the error and no verdict", err, log, code)
		}
	}
}

// A failing command shows its output and its exit code, and the failure reason
// still reads BuildKit's own words through the termination message: the
// command, its unstamped output, and its exit code.
func TestNativeBuildLogNamesAFailedCommand(t *testing.T) {
	command := "npm install && echo QA84 failing on purpose && exit 3"
	o := nativeLogOptions(command)
	log, message, exit := runBuildkit(t, o, nativeTranscript(command, "QA84 failing on purpose"), 1)
	if exit != ExitTenantError {
		t.Fatalf("exit %d, want the tenant-error class %d; log:\n%s", exit, ExitTenantError, log)
	}
	if !strings.HasSuffix(log, "QA84 failing on purpose\nerror: build command '"+command+"' exited with code 3\n") {
		t.Errorf("log does not end with the command's output and exit code:\n%s", log)
	}
	for _, internal := range []string{"#", "sha256:", "static-ffmpeg", "bex-init", "/opt/bex/load-env", "--mount=type=secret", "Build successful"} {
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
