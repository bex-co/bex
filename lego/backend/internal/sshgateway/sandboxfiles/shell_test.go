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

package sandboxfiles

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"
)

// runCode runs command, with env (and PATH) when given, and returns its exit
// status and stdout.
func runCode(t *testing.T, command []string, env ...string) (int, []byte) {
	t.Helper()
	cmd := exec.Command(command[0], command[1:]...)
	if env != nil {
		cmd.Env = append(env, "PATH="+os.Getenv("PATH"))
	}
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), out
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, out
}

// An upload never follows a symlink: through one it would write where its
// ticket did not name. A symlinked ancestor answers its own exit (74), a
// symlinked destination the existing-destination one (73). A follow variable
// in the environment, which a tenant's image or sandbox env can set, changes
// nothing.
// tarNames lists the members of a download script's directory output.
func tarNames(t *testing.T, out []byte) []string {
	t.Helper()
	var names []string
	for _, m := range members(t, bytes.TrimPrefix(out, []byte("directory\n"))) {
		names = append(names, m.name)
	}
	return names
}

func TestShellRejectsExistingSymlinkAncestorsAndTargets(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actual, alias := filepath.Join(root, "actual"), filepath.Join(root, "alias")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		command []string
		code    int
	}{
		{shell(checkScript, alias, "new-file", "file"), 74},
		{shell(stageScript, alias, "new-file", ".bex-copy-test", "file", "0"), 74},
		{shell(checkScript, root, "alias", "directory"), 73},
		{shell(stageScript, root, "alias", ".bex-copy-test", "directory", "0"), 73},
	} {
		for _, env := range [][]string{nil, {"follow=1", "HOME=" + root}} {
			if code, out := runCode(t, tc.command, env...); code != tc.code || len(out) != 0 {
				t.Errorf("%q with env %q = exit %d %q, want exit %d and no output", tc.command[4:], env, code, out, tc.code)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(actual, ".bex-copy-test")); !os.IsNotExist(err) {
		t.Fatal("upload followed symlink ancestor")
	}
}

// A download reads its source as the sandbox's filesystem resolves it
// (w32/024): a symlink to a file sends the file, one to a directory that
// directory's tree, and a path through a symlinked directory (a virtualenv's
// lib64 -> lib) its target. Only a dangling one is missing.
func TestADownloadFollowsSymlinks(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	site := filepath.Join(home, "venv", "lib", "python3.12")
	if err := os.MkdirAll(site, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "x.py"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"venv/lib64": "lib",
		"py-link":    "venv/lib/python3.12/x.py",
		"dir-link":   "venv/lib/python3.12",
		"dangling":   "missing",
	} {
		if err := os.Symlink(target, filepath.Join(home, link)); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		parent, name string
		code         int
		check        func([]byte) bool
	}{
		{".", "py-link", 0, func(out []byte) bool { return string(out) == "file\nx" }},
		{"venv/lib64/python3.12", "x.py", 0, func(out []byte) bool { return string(out) == "file\nx" }},
		{"venv/lib64", "python3.12", 0, func(out []byte) bool { return slices.Contains(tarNames(t, out), "./x.py") }},
		{".", "dir-link", 0, func(out []byte) bool { return slices.Contains(tarNames(t, out), "./x.py") }},
		{filepath.Join(home, "venv"), "lib64", 0, func(out []byte) bool { return slices.Contains(tarNames(t, out), "./python3.12/x.py") }},
		{".", "dangling", 66, func(out []byte) bool { return len(out) == 0 }},
	} {
		code, out := runCode(t, shell(downloadScript, tc.parent, tc.name), "HOME="+home)
		if code != tc.code || !tc.check(out) {
			t.Errorf("download of %s/%s = exit %d %q, want exit %d", tc.parent, tc.name, code, out, tc.code)
		}
	}
}

func TestStageUsesLiteralArgumentsAndNeverPublishes(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name := "quotes' $(touch injected); file"
	command := shell(stageScript, root, name, ".bex-copy-test", "file", "3")
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = bytes.NewReader([]byte{0, 255, 1})
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stage: %v: %s", err, output)
	}
	got, err := os.ReadFile(filepath.Join(root, ".bex-copy-test", "data"))
	if err != nil || !bytes.Equal(got, []byte{0, 255, 1}) {
		t.Fatalf("staged = %v, %v", got, err)
	}
	for _, forbidden := range []string{name, "injected"} {
		if _, err := os.Stat(filepath.Join(root, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("stage published or executed %q", forbidden)
		}
	}
}

// w8/313: a stage that refuses still reads the whole upload before it exits
// with the refusal's code, because a pods/exec stdin stream the remote stops
// reading stalls the exec instead of ending it.
func TestARefusingStageDrainsTheUpload(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0700); err != nil {
		t.Fatal(err)
	}
	// A tar that fails without reading the rest of its input, as on a full
	// sandbox disk; GNU tar reads a bad archive to its end on its own.
	failingTar := t.TempDir()
	if err := os.WriteFile(filepath.Join(failingTar, "tar"), []byte("#!/bin/sh\nexit 2\n"), 0700); err != nil {
		t.Fatal(err)
	}
	upload := bytes.Repeat([]byte("x"), 3<<20)
	size := strconv.Itoa(len(upload))
	for _, tc := range []struct {
		name, parent, entry, kind, size, path string
		code                                  int
	}{
		{"file onto a directory", root, "dir", "file", size, "", 73},
		{"directory onto an existing one", root, "dir", "directory", "0", "", 73},
		{"missing parent", filepath.Join(root, "missing"), "f", "file", size, "", 66},
		{"short upload", root, "short", "file", strconv.Itoa(len(upload) + 1), "", 65},
		{"tar fails midway", root, "new", "directory", "0", failingTar + ":", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdin := bytes.NewReader(upload)
			command := shell(stageScript, tc.parent, tc.entry, ".bex-copy-test", tc.kind, tc.size)
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Env = []string{"PATH=" + tc.path + os.Getenv("PATH")}
			cmd.Stdin = stdin
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != tc.code {
				t.Fatalf("stage = %v, want exit %d", err, tc.code)
			}
			if read := len(upload) - stdin.Len(); read != len(upload) {
				t.Fatalf("stage read %d of %d upload bytes before refusing", read, len(upload))
			}
			if _, err := os.Stat(filepath.Join(root, ".bex-copy-test")); !os.IsNotExist(err) {
				t.Fatal("a refused stage left its staging directory")
			}
		})
	}
}

// w8/313: the stdin-less destination check refuses what the stage would.
func TestCheckRefusesTheStagesDestinations(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "file"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		parent, entry, kind string
		code                int
	}{
		{root, "dir", "directory", 73},
		{root, "dir", "file", 73},
		{root, "file", "directory", 73},
		{root, "link", "file", 73},
		{root, "file", "file", 0},
		{root, "new", "directory", 0},
		{root, "new", "file", 0},
		{filepath.Join(root, "missing"), "new", "file", 66},
	} {
		if code, _ := runCode(t, shell(checkScript, tc.parent, tc.entry, tc.kind)); code != tc.code {
			t.Errorf("check %s %s as %s = exit %d, want %d", tc.parent, tc.entry, tc.kind, code, tc.code)
		}
	}
}

// w32/m299: a relative path is under the sandbox user's home, as Render
// resolves it, even when the home itself is reached through a symlink; a
// symlinked directory under home is still refused for an upload.
func TestShellResolvesRelativePathsUnderHome(t *testing.T) {
	// The publish script runs in the sandbox's Linux userland and uses GNU
	// `mv -T`; BSD mv (a macOS ship-gate host) answers usage exit 64. CI's
	// Linux runners execute it.
	if runtime.GOOS != "linux" {
		t.Skip("publishScript needs GNU coreutils (mv -T); runs on Linux CI")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, homeLink := filepath.Join(root, "home"), filepath.Join(root, "home-link")
	if err := os.MkdirAll(filepath.Join(home, "rel"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(home, homeLink); err != nil {
		t.Fatal(err)
	}
	run := func(stdin []byte, command []string) ([]byte, error) {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = []string{"HOME=" + homeLink, "PATH=" + os.Getenv("PATH")}
		cmd.Dir = root // the exec's cwd is not the home
		cmd.Stdin = bytes.NewReader(stdin)
		return cmd.Output()
	}
	for _, parent := range []string{"rel", "."} {
		if out, err := run([]byte("abc"), shell(stageScript, parent, "f.txt", ".bex-copy-test", "file", "3")); err != nil {
			t.Fatalf("stage under %q: %v %s", parent, err, out)
		}
		if out, err := run(nil, shell(publishScript, parent, "f.txt", ".bex-copy-test", "file", "644")); err != nil {
			t.Fatalf("publish under %q: %v %s", parent, err, out)
		}
		want := filepath.Join(home, parent, "f.txt")
		if got, err := os.ReadFile(want); err != nil || string(got) != "abc" {
			t.Fatalf("%s = %q, %v", want, got, err)
		}
		out, err := run(nil, shell(downloadScript, parent, "f.txt"))
		if err != nil || string(out) != "file\nabc" {
			t.Fatalf("download of %s/f.txt = %q, %v", parent, out, err)
		}
	}
	// "." is the home directory itself (w32/022): path.Dir and path.Base of
	// "." are both ".", so a download archives home, and an upload to it is
	// refused like any existing directory destination.
	out, err := run(nil, shell(downloadScript, ".", "."))
	if err != nil || !bytes.HasPrefix(out, []byte("directory\n")) {
		t.Fatalf("download of home = %q, %v", out, err)
	}
	if names := tarNames(t, out); !slices.Contains(names, "./f.txt") || !slices.Contains(names, "./rel/f.txt") {
		t.Fatalf("home archive = %v, want its files", names)
	}
	for _, kind := range []string{"directory", "file"} {
		if _, err := run(nil, shell(checkScript, ".", ".", kind)); err == nil {
			t.Fatalf("an upload of a %s to home itself was let through", kind)
		}
	}
	if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(home, "sneaky")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "elsewhere"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := run([]byte("x"), shell(stageScript, "sneaky", "f.txt", ".bex-copy-test", "file", "1")); err == nil {
		t.Fatal("a symlinked directory under home was entered")
	}
	if _, err := run(nil, shell(downloadScript, "missing", "f.txt")); err == nil {
		t.Fatal("a missing relative parent read as success")
	}
}

// w32/018: a directory upload keeps each file's modification time and
// publishes its root with the archive's own mode,
// not the 0700 staging leaves; a single uploaded file is published 0644, a
// plain create's mode. The published file modes inside the tree are tar's own
// and are not asserted: they depend on the extracting user and tar.
func TestUploadKeepsModTimesAndPublishesPlainModes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("publishScript needs GNU coreutils (mv -T); runs on Linux CI")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := func(stdin []byte, command []string) {
		t.Helper()
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = []string{"HOME=" + root, "PATH=" + os.Getenv("PATH")}
		cmd.Stdin = bytes.NewReader(stdin)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%q: %v %s", command[4:], err, out)
		}
	}
	mtime := time.Unix(1767323045, 0)
	src := archive(t,
		&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o750, ModTime: mtime},
		&tar.Header{Name: "x.sh", Typeflag: tar.TypeReg, Mode: 0o755, Size: 3, ModTime: mtime},
		&tar.Header{Name: "sub/", Typeflag: tar.TypeDir, Mode: 0o750, ModTime: mtime},
		&tar.Header{Name: "sub/y.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2, ModTime: mtime},
	)
	var clean bytes.Buffer
	mode, err := copyArchive(&clean, bytes.NewReader(src))
	if err != nil || mode != 0o750 {
		t.Fatalf("copyArchive = %o, %v; want the root's 750", mode, err)
	}
	run(clean.Bytes(), shell(stageScript, root, "m", ".bex-copy-dir", "directory", "0"))
	run(nil, shell(publishScript, root, "m", ".bex-copy-dir", "directory", strconv.FormatInt(mode, 8)))
	if info, err := os.Stat(filepath.Join(root, "m")); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("published root = %v, %v; want the archive root's 0750", info, err)
	}
	for _, name := range []string{"x.sh", "sub/y.txt"} {
		if info, err := os.Stat(filepath.Join(root, "m", name)); err != nil || !info.ModTime().Equal(mtime) {
			t.Fatalf("%s = %v, %v; want modified at %s", name, info, err, mtime)
		}
	}

	run([]byte("abc"), shell(stageScript, root, "single.sh", ".bex-copy-file", "file", "3"))
	run(nil, shell(publishScript, root, "single.sh", ".bex-copy-file", "file", strconv.FormatInt(plainFileMode, 8)))
	if info, err := os.Stat(filepath.Join(root, "single.sh")); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("published file = %v, %v; want 0644", info, err)
	}
}

// An archive with no root entry publishes its directory 0755, and an entry
// time outside what USTAR holds is clamped rather than failing the upload.
func TestCopyArchiveDefaultsTheRootModeAndClampsTimes(t *testing.T) {
	var clean bytes.Buffer
	mode, err := copyArchive(&clean, bytes.NewReader(archive(t,
		&tar.Header{Name: "early", Typeflag: tar.TypeReg, Mode: 0o600, ModTime: time.Unix(-5, 0)},
		&tar.Header{Name: "late", Typeflag: tar.TypeReg, Mode: 0o600, ModTime: time.Unix(1<<40, 0), Format: tar.FormatPAX},
	)))
	if err != nil || mode != plainDirMode {
		t.Fatalf("copyArchive = %o, %v; want the default %o", mode, err, plainDirMode)
	}
	tr := tar.NewReader(&clean)
	for _, want := range []int64{0, 1<<33 - 1} {
		h, err := tr.Next()
		if err != nil || h.ModTime.Unix() != want {
			t.Fatalf("entry %v (err %v), want mtime %d", h, err, want)
		}
	}
}
