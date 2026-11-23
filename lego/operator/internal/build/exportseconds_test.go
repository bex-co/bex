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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// exportReport is the termination message a successful native buildkit
// phase leaves for nativeAfterCommand followed by the wrapper's archive line
// (xdgBuildctl): every part before total, which the run itself decides.
var exportReport = regexp.MustCompile(`^export image 434\.1\nexport layers 412\.6\nexport cache 12\.3\nexport xdg 40\nexport total ([0-9]+)\n$`)

// withEachAwk runs f with each awk on the host first on PATH: busybox's, as
// production's BuildKit image has, mawk, as CI's runner has, and gawk.
func withEachAwk(t *testing.T, f func(t *testing.T)) {
	t.Helper()
	ran := false
	for _, name := range []string{"busybox", "mawk", "gawk"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		ran = true
		t.Run(name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.Symlink(path, filepath.Join(bin, "awk")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			f(t)
		})
	}
	if !ran {
		t.Skip("no busybox, mawk or gawk on PATH")
	}
}

// w33/m571: a native build's buildkit phase reports what its export cost,
// read off BuildKit's own seconds for the image (its layers' compression
// within it) and the layer cache, the wrapper's for the $XDG_CACHE_HOME
// archive, and its own clock for the whole stretch after the command. The
// operator reads the same numbers back. The log is the one the build showed
// before: nothing of the report reaches it.
func TestNativeBuildReportsWhatItsExportCost(t *testing.T) {
	withEachAwk(t, func(t *testing.T) {
		o := nativeLogOptions("npm install")
		transcript := nativeThroughCommand(t, "npm install") + nativeAfterCommand + xdgExportLine + " 40\n"
		before := time.Now().Unix()
		log, message, exit := runBuildkit(t, o, transcript, 0)
		elapsed := time.Now().Unix() - before
		if want := rendersShellLine("Running build command 'npm install'...") + "\n" + nativeCommandOutput + rendersRuntimeLine("Uploading build...") + "\n"; exit != 0 || log != want {
			t.Fatalf("exit %d, log:\n%q\nwant:\n%q", exit, log, want)
		}
		m := exportReport.FindStringSubmatch(message)
		if m == nil {
			t.Fatalf("termination message:\n%q\nwant the export's parts", message)
		}
		total, _ := strconv.ParseFloat(m[1], 64)
		if total > float64(elapsed) {
			t.Errorf("total %vs, longer than the %ds the phase ran", total, elapsed)
		}
		sig := signalsFrom(completedJob(o, batchv1.JobComplete), []corev1.Pod{buildkitPod(0, message)})
		want := []ExportPart{{ExportImage, 434.1}, {ExportLayers, 412.6}, {ExportCache, 12.3}, {ExportXDG, 40}, {ExportTotal, total}}
		if !slices.Equal(sig.Export, want) {
			t.Errorf("Signals.Export = %v, want %v", sig.Export, want)
		}
	})
}

// w33/m571: total runs from the moment `==> Uploading build...` was printed,
// not from when BuildKit's export reported: the stand-in buildctl holds the
// export back for over a second after the phase stamped the upload.
func TestNativeBuildExportTotalCountsFromTheUploadLine(t *testing.T) {
	streamingAwk(t)
	fifo := filepath.Join(t.TempDir(), "transcript")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	volume := t.TempDir()
	head := nativeThroughCommand(t, "npm install")
	go func() {
		f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		defer func() { _ = f.Close() }()
		_, _ = f.WriteString(head)
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(filepath.Join(volume, filepath.Base(uploadStartedFile))); err == nil {
				break
			}
		}
		time.Sleep(2100 * time.Millisecond)
		_, _ = f.WriteString(nativeAfterCommand + xdgExportLine + " 40\n")
	}()
	_, message, exit := runBuildkitFrom(t, nativeLogOptions("npm install"), fifo, 0, volume)
	m := exportReport.FindStringSubmatch(message)
	if exit != 0 || m == nil {
		t.Fatalf("exit %d, termination message:\n%q", exit, message)
	}
	if n, _ := strconv.Atoi(m[1]); n < 2 || n > 5 {
		t.Errorf("total %ds, want the 2 s the export was held after the upload line", n)
	}
}

// w33/m571: a native phase that fails keeps the transcript's tail as its
// message, which the failure's reason reads, and the operator reads no
// export from it. A Dockerfile build reports none either.
func TestOnlyASucceededNativeBuildReportsItsExport(t *testing.T) {
	o := nativeLogOptions("npm install")
	transcript := nativeThroughCommand(t, "npm install") + nativeAfterCommand + xdgExportLine + " 40\n"
	_, message, exit := runBuildkit(t, o, transcript, 1)
	if exit == 0 || !strings.Contains(message, "#25 exporting to oci image format") || strings.Contains(message, "export image") {
		t.Fatalf("exit %d, termination message:\n%q\nwant the transcript's tail", exit, message)
	}
	if got := exportSignals(new(buildkitPod(exit, message))); got != nil {
		t.Errorf("a failed phase's export: %v", got)
	}

	docker := nativeLogOptions("")
	docker.Builder = BuilderDockerfile
	if _, message, exit := runBuildkit(t, docker, nativeAfterCommand, 0); exit != 0 || message != "" {
		t.Errorf("a Dockerfile build: exit %d, termination message %q", exit, message)
	}
}

// w33/m571: the operator keeps only the closed set of parts, once each, in
// their order, with seconds that are a finite non-negative number, and only
// from the buildkit phase.
func TestExportSignalsReadOnlyTheClosedSet(t *testing.T) {
	message := strings.Join([]string{
		"export total 500",
		"export cache 12.3",
		"export cache 99",
		"export image 434.1",
		"export layers NaN",
		"export xdg -1",
		"export xdg +Inf",
		"export xdg 1e400",
		"export push 30",
		"export image",
		"exported image 1",
		"bex: anything else",
	}, "\n")
	want := []ExportPart{{ExportImage, 434.1}, {ExportCache, 12.3}, {ExportTotal, 500}}
	if got := exportSignals(new(buildkitPod(0, message))); !slices.Equal(got, want) {
		t.Errorf("exportSignals = %v, want %v", got, want)
	}
	other := buildkitPod(0, message)
	other.Status.InitContainerStatuses[0].Name = cacheRestorePhase
	if got := exportSignals(&other); got != nil {
		t.Errorf("another phase's lines read as the export: %v", got)
	}
}

// buildkitPod is a build pod whose buildkit phase ended with exit and message.
func buildkitPod(exit int, message string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-abcde", CreationTimestamp: metav1.NewTime(time.Date(2026, 11, 22, 22, 34, 0, 0, time.UTC))},
		Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{{
			Name: buildkitPhase, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: int32(exit), Message: message,
			}},
		}}},
	}
}
