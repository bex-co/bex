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

package controller

import (
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Docker Command runs as Render runs it (w8/164).
func TestShellCommand(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		want          []string
	}{
		{"plain program with flags execs directly", "/whoami --port 8080 --name qa1281", []string{"/whoami", "--port", "8080", "--name", "qa1281"}},
		{"quoted argument stays one word", `bin/server --msg "hello world" --x='a b'`, []string{"bin/server", "--msg", "hello world", "--x=a b"}},
		{"literal dollars survive direct exec", "bin/server 'a$$b $(PORT)'", []string{"bin/server", "a$$b $(PORT)"}},
		{"command substitution reaches the shell", `sh -c 'echo "$(PORT)" "$$"'`, []string{"sh", "-c", `echo "$(PORT)" "$$"`}},
		{"escaped space", `bin/server a\ b`, []string{"bin/server", "a b"}},
		{"shell operator keeps the shell", "npm run build && npm start", []string{"/bin/sh", "-c", "npm run build && npm start"}},
		{"variable expansion keeps the shell", "./server --port $PORT", []string{"/bin/sh", "-c", "./server --port $PORT"}},
		{"expansion inside double quotes keeps the shell", `echo "$HOME"`, []string{"/bin/sh", "-c", `echo "$HOME"`}},
		{"quoted metacharacters are data", `echo '$HOME && x'`, []string{"echo", "$HOME && x"}},
		{"glob keeps the shell", "ls *.txt", []string{"/bin/sh", "-c", "ls *.txt"}},
		{"leading assignment keeps the shell", "FOO=1 ./server", []string{"/bin/sh", "-c", "FOO=1 ./server"}},
		{"unterminated quote keeps the shell", `echo "oops`, []string{"/bin/sh", "-c", `echo "oops`}},
		// Render's documented form passes everything after -c as one script.
		{"documented unquoted multi-command form", "/bin/sh -c python manage.py migrate && gunicorn app.wsgi:application --bind 0.0.0.0:10000", []string{"/bin/sh", "-c", "python manage.py migrate && gunicorn app.wsgi:application --bind 0.0.0.0:10000"}},
		{"minio example", "/bin/sh -c mkdir -p /data && exec minio server --address $HOST:$PORT /data", []string{"/bin/sh", "-c", "mkdir -p /data && exec minio server --address $HOST:$PORT /data"}},
		{"quoted script is that word", `sh -c 'a && b'`, []string{"sh", "-c", "a && b"}},
		{"bash with a double-quoted script", `/bin/bash -c "echo hi; echo bye"`, []string{"/bin/bash", "-c", "echo hi; echo bye"}},
		{"unquoted simple script stays verbatim", "sh -c echo hi", []string{"sh", "-c", "echo hi"}},
		// w4/412: a quoted script whose first word is an assignment, or whose
		// double quotes hold an expansion, is still that one script — the
		// shell runs it, so $PORT must reach it unquoted, not as the script's
		// literal name.
		{"single-quoted script starting with an assignment", `sh -c 'i=0; echo $i'`, []string{"sh", "-c", "i=0; echo $i"}},
		{"single-quoted assignment prefix", `sh -c 'FOO=bar exec app'`, []string{"sh", "-c", "FOO=bar exec app"}},
		{"two assignments through /bin/sh", `/bin/sh -c 'X=1 Y=2 cmd'`, []string{"/bin/sh", "-c", "X=1 Y=2 cmd"}},
		{"double-quoted script expanding PORT", `sh -c "node server.js --port $PORT"`, []string{"sh", "-c", "node server.js --port $PORT"}},
		{"bash double-quoted exec with PORT", `bash -c "exec gunicorn -b 0.0.0.0:$PORT app:app"`, []string{"bash", "-c", "exec gunicorn -b 0.0.0.0:$PORT app:app"}},
		{"double-quoted script from the live crash", `sh -c "echo q2d6-port-$PORT; httpd -f -p $PORT -h /"`, []string{"sh", "-c", "echo q2d6-port-$PORT; httpd -f -p $PORT -h /"}},
		{"escaped quote inside a double-quoted script", `sh -c "echo \"hi\"; sleep 1"`, []string{"sh", "-c", `echo "hi"; sleep 1`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := shellCommand(tc.command)
			for i := range got {
				got[i] = executiontest.Expand(got[i], map[string]string{"PORT": "10000", "HOME": "/home/platform"})
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("shellCommand(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

// A pre-deploy command launches like its service's own container: a Docker
// Command as Render runs it (w8/172), a native image's command as arguments to
// its native-init entrypoint (w8/350), a buildpack image's through /bin/sh -c.
func TestPreDeployLaunch(t *testing.T) {
	docker := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "r", Builder: build.BuilderDockerfile}}
	native := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "r", Builder: build.BuilderNative, StartCommand: "npm start"}}
	buildpack := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "r", Builder: build.BuilderBuildpack, StartCommand: "npm start"}}
	// A rollback across a runtime change: the selected release is a Docker
	// image with no start command, which a native build never is.
	commandless := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "r", Builder: build.BuilderNative}}
	prebuilt := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Image: "img:1", Runtime: "node", StartCommand: "npm start"}}
	shell := []string{"/bin/sh", "-c", "/whoami --help"}
	for _, tc := range []struct {
		name     string
		app      *appv1alpha1.App
		launch   launchMode
		cmd      string
		wantCmd  []string
		wantArgs []string
	}{
		{"docker sh -c runs its script", docker, launchUnknown, "/bin/sh -c echo x", []string{"/bin/sh", "-c", "echo x"}, nil},
		{"docker plain command is exec'd", docker, launchUnknown, "/whoami --help", []string{"/whoami", "--help"}, nil},
		{"docker shell syntax keeps the shell", docker, launchUnknown, "a && b", []string{"/bin/sh", "-c", "a && b"}, nil},
		{"native runs under its entrypoint", native, launchUnknown, "/whoami --help", nil, shell},
		{"native rerun of the served build", native, launchNativeArgs, "/whoami --help", nil, shell},
		{"a linked native build source", docker, launchNativeArgs, "/whoami --help", nil, shell},
		{"a recorded shell launch under a native runtime", native, launchShell, "/whoami --help", []string{"/whoami", "--help"}, nil},
		{"a release with no start command is not native", commandless, launchUnknown, "/whoami --help", shell, nil},
		{"buildpack keeps the shell in place of its launcher", buildpack, launchUnknown, "/whoami --help", shell, nil},
		{"the saved prebuilt image keeps the Docker Command", prebuilt, launchUnknown, "/whoami --help", []string{"/whoami", "--help"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args := preDeployLaunch(tc.app, "img:1", tc.launch, tc.cmd)
			if !slices.Equal(cmd, tc.wantCmd) || !slices.Equal(args, tc.wantArgs) {
				t.Errorf("command %q args %q, want command %q args %q", cmd, args, tc.wantCmd, tc.wantArgs)
			}
		})
	}
}
