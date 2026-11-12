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
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func publishCheckOptions(publishPath string) Options {
	o := nativeOptions()
	o.StaticSite, o.Runtime, o.StartCommand = true, "", ""
	o.BuildCommand, o.StaticPublishPath, o.AppUID = "npm run build", publishPath, "uid-web"
	return o
}

// w8/291: a native static site's build step checks its publish directory as
// it exits, after the dependency install and before the authored command
// runs; a site publishing its root, one with no publish path and a service
// have no check.
func TestNativeStaticBuildChecksThePublishDirectory(t *testing.T) {
	o := publishCheckOptions("dist")
	run := shellJSON(nativeEnvLoaderPrefix + nativeStaticInstallStep + "bex_publish_dir='dist'\n" + nativePublishCheck + o.BuildCommand)
	if df := nativeDockerfile(o); !strings.Contains(df, run) {
		t.Fatalf("static build step lacks the publish check before its command:\n%s", df)
	}
	for name, other := range map[string]Options{
		"root": publishCheckOptions("./"), "unset": publishCheckOptions(""), "service": nativeOptions(),
	} {
		if df := nativeDockerfile(other); strings.Contains(df, nativePublishCheckVar) {
			t.Errorf("%s: build step gained the publish check:\n%s", name, df)
		}
	}
	quoted := publishCheckOptions(`it's "here" $HOME`)
	if step := nativePublishCheckStep(quoted); !strings.HasPrefix(step, `bex_publish_dir='it'"'"'s "here" $HOME'`+"\n") {
		t.Errorf("publish path is not one single-quoted word: %q", step)
	}
}

// The check sees the authored command's own outcome: a command that leaves
// the directory passes, one that does not fails with Render's sentence and
// the check's exit, and a failing command keeps its own status and says
// nothing about the directory. It looks from the image's working directory,
// wherever the command left the shell.
func TestNativePublishCheckRunsAfterTheCommand(t *testing.T) {
	missing, err := strconv.Atoi(nativePublishCheckExit)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, dir, command string
		exit               int
		said               bool
	}{
		{"left it", "out", "mkdir -p out && cd /", 0, false},
		{"did not", "out", "echo built elsewhere", missing, true},
		{"failed", "out", "exit 3", 3, false},
		{"exited cleanly without it", "out", "exit 0", missing, true},
		{"a path with a space and a quote", `my site's "out"`, `mkdir -p "my site's \"out\""`, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			script := strings.ReplaceAll(nativePublishCheckStep(publishCheckOptions(tc.dir)), nativeWorkdir, src) + tc.command
			cmd := exec.Command("/bin/bash", "-c", script)
			cmd.Dir = src
			out, err := cmd.CombinedOutput()
			exit := 0
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
				exit = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			said := strings.Contains(string(out), "==> Publish directory "+tc.dir+" does not exist!\n")
			if exit != tc.exit || said != tc.said {
				t.Errorf("exit %d (want %d), said %v (want %v):\n%s", exit, tc.exit, said, tc.said, out)
			}
		})
	}
}

// A build step that failed its publish check ends the log with Render's line,
// with no exit-code verdict about a command that succeeded, and the failure
// reason names the publish directory step with that sentence alone.
func TestNativeBuildLogNamesAMissingPublishDirectory(t *testing.T) {
	o := publishCheckOptions("nope")
	o.BuildCommand = "mkdir -p out"
	script := nativeEnvLoaderPrefix + nativeStaticInstallStep + nativePublishCheckStep(o) + o.BuildCommand
	header := `#23 [stage-0 11/12] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c","` +
		strings.ReplaceAll(script, "\n", `\n`) + `"]`
	process := strconv.Quote("/bin/bash -c " + script)
	verdict := " did not complete successfully: exit code: " + nativePublishCheckExit
	transcript := header + `
#23 1.204 ==> Publish directory nope does not exist!
#23 ERROR: process ` + process + verdict + `
------
 > ` + strings.TrimPrefix(header, "#23 ") + `:
1.204 ==> Publish directory nope does not exist!
------
error: failed to solve: process ` + process + verdict + `
`
	log, message, exit := runBuildkit(t, o, transcript, 1)
	if exit != ExitTenantError {
		t.Fatalf("exit %d, want the tenant-error class %d; log:\n%s", exit, ExitTenantError, log)
	}
	if want := "==> Running build command 'mkdir -p out'...\n==> Publish directory nope does not exist!\n"; log != want {
		t.Errorf("log:\n%q\nwant:\n%q", log, want)
	}
	obs := failedBuildkit(t, o, exit, message)
	if obs.FailedStep != nativePublishStep || obs.Tail != "Publish directory nope does not exist!" {
		t.Errorf("FailedStep, Tail = %q, %q; want %q and Render's sentence", obs.FailedStep, obs.Tail, nativePublishStep)
	}
}

// A static site whose command fails, even with the check's own exit code, is
// a build command failure named as one: only the check's line makes it the
// publish directory's.
func TestNativeStaticBuildCommandFailureIsNotAPublishFailure(t *testing.T) {
	for _, code := range []string{"3", nativePublishCheckExit} {
		o := publishCheckOptions("out")
		o.BuildCommand = "npm run build && exit " + code
		script := nativeEnvLoaderPrefix + nativeStaticInstallStep + nativePublishCheckStep(o) + o.BuildCommand
		process := strconv.Quote("/bin/bash -c " + script)
		msg := "#23 ERROR: process " + process + " did not complete successfully: exit code: " + code + "\n"
		got, step := nativeCommandFailure(msg)
		want := "build command '" + o.BuildCommand + "' exited with code " + code
		if step != nativeBuildCommandStep || !strings.Contains(got, want) || strings.Contains(got, nativePublishCheckVar) {
			t.Errorf("exit %s: step %q, message %q; want %q naming %q", code, step, got, nativeBuildCommandStep, want)
		}
	}
}

// w8/295: the kubelet divides a pod's 12 KiB of termination messages evenly
// among its containers and keeps the head of each, so a phase writes no more
// than its share of its output's tail, and the verdict at the end survives.
// Production cut the publish failure's verdict at "exit c" before this.
func TestBuildPhaseMessageFitsThePodsShare(t *testing.T) {
	o := publishCheckOptions("nope")
	o.BuildCommand = "mkdir -p out"
	// With the resolve phase, as in production: five containers, 2457 bytes each.
	o.NativeResolverImage = "registry.example/bex@sha256:" + strings.Repeat("0", 64)
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	n := len(spec.InitContainers) + len(spec.Containers)
	share := strconv.Itoa(min(terminationMessageBytes, podTerminationMessageBytes/n))
	for _, cs := range [][]corev1.Container{spec.InitContainers, spec.Containers} {
		for _, c := range cs {
			if i := slices.IndexFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_MESSAGE_BYTES" }); i < 0 || c.Env[i].Value != share {
				t.Errorf("%s: BEX_MESSAGE_BYTES = %v, want %s (%d containers)", c.Name, c.Env, share, n)
			}
		}
	}

	script := nativeEnvLoaderPrefix + nativeStaticInstallStep + nativePublishCheckStep(o) + o.BuildCommand
	header := `#23 [stage-0 11/12] RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c","` +
		strings.ReplaceAll(script, "\n", `\n`) + `"]`
	process := strconv.Quote("/bin/bash -c " + script)
	verdict := " did not complete successfully: exit code: " + nativePublishCheckExit
	var noise strings.Builder
	for i := range 120 {
		fmt.Fprintf(&noise, "#23 0.%03d [%d/4] some build output that a long build prints\n", i, i%4+1)
	}
	transcript := header + "\n" + noise.String() + `#23 1.204 ==> Publish directory nope does not exist!
#23 ERROR: process ` + process + verdict + `
------
 > ` + strings.TrimPrefix(header, "#23 ") + `:
1.204 ==> Publish directory nope does not exist!
------
error: failed to solve: process ` + process + verdict + `
`
	_, message, exit := runBuildkit(t, o, transcript, 1)
	limit, _ := strconv.Atoi(share)
	if len(message) > limit {
		t.Fatalf("message is %d bytes, more than the phase's share %d", len(message), limit)
	}
	obs := failedBuildkit(t, o, exit, message)
	if obs.FailedStep != nativePublishStep || obs.Tail != "Publish directory nope does not exist!" {
		t.Errorf("FailedStep, Tail = %q, %q; want the publish directory step with Render's sentence", obs.FailedStep, obs.Tail)
	}
}
