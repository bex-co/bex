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

package predeploy

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// containerDefaultPATH is what the container runtime uses when the process
// env carries no PATH at all.
const containerDefaultPATH = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// runLikeContainer executes c's argv on this host the way the container
// runtime starts it: a bare program is resolved against the container's own
// PATH (the app's env), never the operator's.
func runLikeContainer(t *testing.T, c corev1.Container) (string, error) {
	t.Helper()
	env := make([]string, 0, len(c.Env))
	path, hasPath := "", false
	for _, e := range c.Env {
		env = append(env, e.Name+"="+e.Value)
		if e.Name == "PATH" {
			path, hasPath = e.Value, true
		}
	}
	if !hasPath {
		path = containerDefaultPATH
	}
	program := c.Command[0]
	if !strings.Contains(program, "/") {
		resolved := ""
		for _, dir := range filepath.SplitList(path) {
			if dir == "" {
				continue
			}
			if info, err := os.Stat(filepath.Join(dir, program)); err == nil && !info.IsDir() {
				resolved = filepath.Join(dir, program)
				break
			}
		}
		if resolved == "" {
			return "", errors.New(program + ": executable file not found in $PATH")
		}
		program = resolved
	}
	out, err := (&exec.Cmd{Path: program, Args: c.Command, Env: env}).CombinedOutput()
	return string(out), err
}

// An empty or narrow application PATH still starts the pre-deploy shell, and
// the absolute authored command sees that exact PATH and the app's other
// values (w8/m61 t008). The original bare `sh` fails before the command runs.
func TestPreDeployShellStartsWithApplicationPATH(t *testing.T) {
	const note = "ü\nline=two"
	command := `/bin/echo "PATH=[$PATH] NOTE=[$NOTE]"`
	for _, tc := range []struct {
		name     string
		path     *string
		wantPath string
		bareFail bool // the original relative `sh` cannot start
	}{
		{"empty", new(""), "", true},
		{"narrow", new("/bex-narrow-path"), "/bex-narrow-path", true},
		{"normal", new(containerDefaultPATH), containerDefaultPATH, false},
		{"omitted", nil, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var env []corev1.EnvVar
			if tc.path != nil {
				env = append(env, corev1.EnvVar{Name: "PATH", Value: *tc.path})
			}
			env = append(env, corev1.EnvVar{Name: "NOTE", Value: note})
			c := Job(Options{Name: "api", Image: "app:1", Command: command, Env: env}).Spec.Template.Spec.Containers[0]

			out, err := runLikeContainer(t, c)
			if err != nil {
				t.Fatalf("pre-deploy did not start: %v\n%s", err, out)
			}
			// An omitted PATH reaches the shell unset; /bin/sh may then default it.
			if want := "NOTE=[" + note + "]\n"; !strings.HasSuffix(out, want) {
				t.Errorf("output = %q, want suffix %q", out, want)
			}
			if tc.path != nil && !strings.HasPrefix(out, "PATH=["+tc.wantPath+"] ") {
				t.Errorf("output = %q, want the exact application PATH %q", out, tc.wantPath)
			}

			c.Command = append([]string{"sh"}, c.Command[1:]...)
			if _, err := runLikeContainer(t, c); (err != nil) != tc.bareFail {
				t.Errorf("bare sh start error = %v, want failure %v", err, tc.bareFail)
			}
		})
	}
}
