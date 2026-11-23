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

package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mountSecretFiles lays files out as kubelet projects a Secret volume: each
// name is a link into ..data, which links the timestamped directory holding
// the bytes.
func mountSecretFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "secrets")
	const stamp = "..2026_11_20_09_55_23.1234"
	must(t, os.MkdirAll(filepath.Join(dir, stamp), 0o755))
	must(t, os.Symlink(stamp, filepath.Join(dir, "..data")))
	for name, content := range files {
		must(t, os.WriteFile(filepath.Join(dir, stamp, name), []byte(content), 0o644))
		must(t, os.Symlink(filepath.Join("..data", name), filepath.Join(dir, name)))
	}
	return dir
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// read is what the app reads at root/name, or "" when there is no such file.
func read(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	must(t, err)
	return string(raw)
}

func exists(root, name string) bool {
	_, err := os.Lstat(filepath.Join(root, name))
	return err == nil
}

// A deploy that reruns the image's build after a group link or a file added to
// a linked group: the image linked only its own file, and every mounted file
// ends up in the root. A repository file of a secret file's name gives way;
// a directory of that name and the rest of the repository stay.
func TestLinkSecretFilesLinksEveryMountedFile(t *testing.T) {
	t.Parallel()
	secrets := mountSecretFiles(t, map[string]string{
		"own.txt": "own", "grp.txt": "grp", "grp2.txt": "grp2", ".env": "A=1", "existing.txt": "secret", "confdir": "secret",
	})
	root := t.TempDir()
	must(t, os.Symlink(filepath.Join(secrets, "own.txt"), filepath.Join(root, "own.txt"))) // the build's link
	must(t, os.WriteFile(filepath.Join(root, "app.py"), []byte("app"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "existing.txt"), []byte("repo"), 0o644))
	must(t, os.MkdirAll(filepath.Join(root, "confdir"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "confdir", "keep.txt"), []byte("kept"), 0o644))

	for range 2 { // a container restart starts it again on the same files
		var stderr bytes.Buffer
		linkSecretFiles(secrets, root, &stderr)
		if stderr.Len() != 0 {
			t.Fatalf("stderr %q", stderr.String())
		}
		for name, want := range map[string]string{
			"own.txt": "own", "grp.txt": "grp", "grp2.txt": "grp2", ".env": "A=1", "existing.txt": "secret",
			"app.py": "app", "confdir/keep.txt": "kept",
		} {
			if got := read(t, root, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		for _, name := range []string{"..data", "..2026_11_20_09_55_23.1234"} {
			if exists(root, name) {
				t.Errorf("kubelet's %s was linked into the root", name)
			}
		}
		// A link, not a copy: the root copy is as read-only as the mount.
		if target, _ := os.Readlink(filepath.Join(root, "grp.txt")); target != filepath.Join(secrets, "grp.txt") {
			t.Errorf("grp.txt links %q, want %s", target, filepath.Join(secrets, "grp.txt"))
		}
	}
}

// An unlink, or a file deleted from a linked group: the image's link to a file
// no longer mounted goes, so the name is gone from the root as from
// /etc/secrets. Links the service's repository made itself stay.
func TestLinkSecretFilesDropsAFileNoLongerMounted(t *testing.T) {
	t.Parallel()
	secrets := mountSecretFiles(t, map[string]string{"own.txt": "own"})
	root := t.TempDir()
	for name, target := range map[string]string{
		"own.txt":   filepath.Join(secrets, "own.txt"),
		"grp.txt":   filepath.Join(secrets, "grp.txt"),
		"elsewhere": "/usr/share/zoneinfo/UTC",
		"renamed":   filepath.Join(secrets, "grp.txt"),
		"relative":  "grp.txt",
	} {
		must(t, os.Symlink(target, filepath.Join(root, name)))
	}
	var stderr bytes.Buffer
	linkSecretFiles(secrets, root, &stderr)
	if stderr.Len() != 0 {
		t.Fatalf("stderr %q", stderr.String())
	}
	if exists(root, "grp.txt") {
		t.Error("grp.txt is still in the root after its file was unmounted")
	}
	if read(t, root, "own.txt") != "own" {
		t.Error("own.txt lost its link")
	}
	for _, name := range []string{"elsewhere", "renamed", "relative"} {
		if !exists(root, name) {
			t.Errorf("the repository's own link %s was removed", name)
		}
	}
}

// The last secret file removed: there is no /etc/secrets at all, and the
// image's links go with it.
func TestLinkSecretFilesWithNoneMounted(t *testing.T) {
	t.Parallel()
	secrets := filepath.Join(t.TempDir(), "secrets")
	root := t.TempDir()
	must(t, os.Symlink(filepath.Join(secrets, "grp.txt"), filepath.Join(root, "grp.txt")))
	must(t, os.WriteFile(filepath.Join(root, "app.py"), []byte("app"), 0o644))
	var stderr bytes.Buffer
	linkSecretFiles(secrets, root, &stderr)
	if stderr.Len() != 0 || exists(root, "grp.txt") || read(t, root, "app.py") != "app" {
		t.Fatalf("stderr %q, grp.txt left %v, app.py %q", stderr.String(), exists(root, "grp.txt"), read(t, root, "app.py"))
	}
}

// A root the process cannot write to is reported per file, and the files are
// still at /etc/secrets.
func TestLinkSecretFilesReportsALinkItCannotMake(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes to a read-only directory")
	}
	secrets := mountSecretFiles(t, map[string]string{"grp.txt": "grp"})
	root := t.TempDir()
	must(t, os.Chmod(root, 0o555))
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	var stderr bytes.Buffer
	linkSecretFiles(secrets, root, &stderr)
	if !strings.HasPrefix(stderr.String(), "native-init: secret file grp.txt: ") || exists(root, "grp.txt") {
		t.Fatalf("stderr %q, grp.txt made %v", stderr.String(), exists(root, "grp.txt"))
	}
}

// native-init links the files before the command starts, in the working
// directory the command starts in.
func TestNativeInitLinksSecretFilesBeforeTheCommand(t *testing.T) {
	t.Parallel()
	secrets := mountSecretFiles(t, map[string]string{"qa-grp.txt": "qa-rt9-4ca8-grp-v1"})
	root := t.TempDir()
	cmd := nativeInit("/bin/sh", "-c", "cat qa-grp.txt")
	cmd.Env = append(cmd.Env, "BEX_NATIVE_INIT_TEST_SECRETS="+secrets)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	want := running("cat qa-grp.txt") + "\nqa-rt9-4ca8-grp-v1"
	if status := exitStatus(t, cmd.Run()); status != 0 || stdout.String() != want {
		t.Fatalf("status %d stdout %q stderr %q", status, stdout.String(), stderr.String())
	}
}
