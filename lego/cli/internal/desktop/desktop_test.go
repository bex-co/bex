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

package desktop

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeInfo struct{ dir bool }

func (f fakeInfo) Name() string       { return "f" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return 0o755 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

type execCall struct {
	path string
	argv []string
}

// fakeSystem has the given files (path → is directory) and records exec.
func fakeSystem(goos string, files map[string]bool, links map[string]string) (system, *[]execCall) {
	var calls []execCall
	return system{
		goos: goos,
		lookPath: func(name string) (string, error) {
			p := "/usr/local/bin/" + name
			if _, ok := files[p]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		stat: func(p string) (os.FileInfo, error) {
			dir, ok := files[p]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return fakeInfo{dir: dir}, nil
		},
		glob: func(pattern string) ([]string, error) {
			var out []string
			for p := range files {
				for dir := filepath.Dir(p); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
					if ok, _ := filepath.Match(pattern, dir); ok && !slices.Contains(out, dir) {
						out = append(out, dir)
					}
				}
			}
			slices.Sort(out)
			return out, nil
		},
		evalLinks: func(p string) (string, error) {
			if target, ok := links[p]; ok {
				return target, nil
			}
			return p, nil
		},
		executable:  func() (string, error) { return "/usr/local/bin/bex", nil },
		home:        func() (string, error) { return "/home/u", nil },
		interactive: func() bool { return true },
		exec: func(path string, argv []string) error {
			calls = append(calls, execCall{path, argv})
			return nil
		},
		session: func(context.Context, string) (string, error) { return "", errors.New("unexpected session lookup") },
	}, &calls
}

func TestRunPassesArgumentsThroughToTheLauncherOnPath(t *testing.T) {
	sys, calls := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false}, nil)
	var execsBeforeEmit = -1
	args := []string{"a", "b:1:2", "-n", "--wait", "-"}
	if err := run(context.Background(), sys, args, func() { execsBeforeEmit = len(*calls) }); err != nil {
		t.Fatal(err)
	}
	want := execCall{"/usr/local/bin/bex-desktop", append([]string{"bex-desktop"}, args...)}
	if len(*calls) != 1 || (*calls)[0].path != want.path || !slices.Equal((*calls)[0].argv, want.argv) {
		t.Fatalf("exec = %+v, want %+v", *calls, want)
	}
	if execsBeforeEmit != 0 {
		t.Fatalf("launch telemetry must be emitted once, before exec (execs seen at emit: %d)", execsBeforeEmit)
	}
}

func TestLocateFallsBackToTheAppBundleLauncher(t *testing.T) {
	cases := map[string]struct {
		goos  string
		files map[string]bool
		want  string
	}{
		"macOS stable app": {"darwin", map[string]bool{
			"/Applications/Bex.app/Contents/MacOS/cli": false,
			"/Applications/Bex.app/Contents/MacOS/bex": false,
		}, "/Applications/Bex.app/Contents/MacOS/cli"},
		"macOS user Applications preview channel": {"darwin", map[string]bool{
			"/home/u/Applications/Bex Preview.app/Contents/MacOS/cli": false,
		}, "/home/u/Applications/Bex Preview.app/Contents/MacOS/cli"},
		"linux stable before channels": {"linux", map[string]bool{
			"/home/u/.local/bex-preview.app/bin/bex": false,
			"/home/u/.local/bex.app/bin/bex":         false,
		}, "/home/u/.local/bex.app/bin/bex"},
		"linux pre-0.139 cli": {"linux", map[string]bool{
			"/home/u/.local/bex-nightly.app/bin/cli": false,
		}, "/home/u/.local/bex-nightly.app/bin/cli"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sys, _ := fakeSystem(tc.goos, tc.files, nil)
			got, err := locate(sys)
			if err != nil || got != tc.want {
				t.Fatalf("locate = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestLocateNeverExecsItself(t *testing.T) {
	// A pre-rename desktop install still links bex-desktop's old name, and a
	// user may point bex-desktop at the platform CLI; neither may recurse.
	sys, _ := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false},
		map[string]string{"/usr/local/bin/bex-desktop": "/opt/bex/bex", "/usr/local/bin/bex": "/opt/bex/bex"})
	if got, err := locate(sys); err == nil {
		t.Fatalf("locate = %q, want the install hint", got)
	}
}

func TestLocateIgnoresTheEditorBinaryAndPlainBex(t *testing.T) {
	// Contents/MacOS/bex is the editor, and `bex` on PATH is this CLI: only
	// the launcher names count.
	sys, _ := fakeSystem("darwin", map[string]bool{
		"/Applications/Bex.app/Contents/MacOS/bex": false,
		"/usr/local/bin/bex":                       false,
	}, nil)
	_, err := locate(sys)
	if err == nil || !strings.Contains(err.Error(), "bex.co/download") {
		t.Fatalf("err = %v, want the install hint", err)
	}
}

func TestRunRefusesNonInteractiveSessions(t *testing.T) {
	sys, calls := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false}, nil)
	sys.interactive = func() bool { return false }
	err := run(context.Background(), sys, []string{"."}, func() { t.Fatal("telemetry emitted") })
	if err == nil || !strings.Contains(err.Error(), "run `bex-desktop` directly") {
		t.Fatalf("err = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("exec ran: %+v", *calls)
	}
}

func TestRunOpensAnAgentSessionSandboxOverSSH(t *testing.T) {
	sys, calls := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false}, nil)
	sys.session = func(_ context.Context, id string) (string, error) {
		if id != "ags-d0abc123" {
			t.Fatalf("looked up %q", id)
		}
		return "ags-d0abc123@ssh.bex.co", nil
	}
	if err := run(context.Background(), sys, []string{"-n", "ags-d0abc123"}, func() {}); err != nil {
		t.Fatal(err)
	}
	want := []string{"bex-desktop", "-n", "ssh://ags-d0abc123@ssh.bex.co/workspace"}
	if len(*calls) != 1 || !slices.Equal((*calls)[0].argv, want) {
		t.Fatalf("exec = %+v, want argv %q", *calls, want)
	}
}

func TestRunSessionLookupFailureExecsNothing(t *testing.T) {
	sys, calls := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false}, nil)
	sys.session = func(context.Context, string) (string, error) { return "", errors.New("403 forbidden") }
	if err := run(context.Background(), sys, []string{"ags-d0abc123"}, func() { t.Fatal("telemetry emitted") }); err == nil {
		t.Fatal("want the lookup error")
	}
	if len(*calls) != 0 {
		t.Fatalf("exec ran: %+v", *calls)
	}
}

func TestAnExistingPathNamedLikeASessionStaysAPath(t *testing.T) {
	sys, calls := fakeSystem("darwin", map[string]bool{"/usr/local/bin/bex-desktop": false, "ags-notes": true}, nil)
	if err := run(context.Background(), sys, []string{"ags-notes"}, func() {}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || !slices.Equal((*calls)[0].argv, []string{"bex-desktop", "ags-notes"}) {
		t.Fatalf("exec = %+v", *calls)
	}
}

func TestIsSessionID(t *testing.T) {
	for arg, want := range map[string]bool{
		"ags-d0abc123": true, "ags-": false, "ags-ABC": false, "ags-a/b": false, "srv-abc": false, "./ags-a": false,
	} {
		if got := isSessionID(arg); got != want {
			t.Errorf("isSessionID(%q) = %v, want %v", arg, got, want)
		}
	}
}
