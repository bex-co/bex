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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs git in dir for a fixture and returns its trimmed output.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@example.com",
		"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@example.com", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// runClone executes the clone container's real script against a local
// repository, in dest instead of /source: under the host's sh and git, or —
// with BEX_BUILDKIT_INTEGRATION set — in the pinned Git image's busybox sh,
// with the fixture tree mounted at its own path.
func runClone(t *testing.T, o Options, dest string) (string, int) {
	t.Helper()
	c := buildCloneContainer(o, defaultGitImage)
	script := strings.Replace(c.Args[0], "cd /source\n", "cd \"$BEX_TEST_DEST\"\n", 1)
	if script == c.Args[0] {
		t.Fatalf("clone script no longer starts in /source:\n%s", c.Args[0])
	}
	// Submodules over file:// need the opt-in Git gives every local transport;
	// production fetches over HTTPS.
	env := []string{"BEX_TEST_DEST=" + dest, "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always"}
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			env = append(env, e.Name+"="+strings.Replace(e.Value, sourceMount, dest, 1))
		}
	}
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "" {
		fixture := filepath.Dir(strings.TrimPrefix(o.Repo, "file://"))
		args := make([]string, 0, 16+2*len(env))
		args = append(args, "run", "--rm", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "--entrypoint", "sh",
			"-e", "HOME=/tmp", "-v", fixture+":"+fixture+":ro", "-v", dest+":"+dest)
		for _, e := range env {
			args = append(args, "-e", e)
		}
		cmd = exec.Command("docker", append(args, defaultGitImage, "-eu", "-c", script)...)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// submoduleFixture is a child repository with two commits and the path of a
// parent that pins the older one at lib through url, named name.
func submoduleFixture(t *testing.T, name string, url func(root string) string) string {
	t.Helper()
	root := t.TempDir()
	child := filepath.Join(root, "child")
	must(t, os.MkdirAll(child, 0o755))
	git(t, child, "init", "-q", "-b", "main")
	must(t, os.WriteFile(filepath.Join(child, "LICENSE"), []byte("pinned\n"), 0o644))
	git(t, child, "add", ".")
	git(t, child, "commit", "-q", "-m", "pinned")
	pinned := git(t, child, "rev-parse", "HEAD")
	must(t, os.WriteFile(filepath.Join(child, "LICENSE"), []byte("tip\n"), 0o644))
	git(t, child, "commit", "-q", "-am", "tip")

	parent := filepath.Join(root, "parent")
	must(t, os.MkdirAll(parent, 0o755))
	git(t, parent, "init", "-q", "-b", "main")
	must(t, os.WriteFile(filepath.Join(parent, "README"), []byte("parent\n"), 0o644))
	git(t, parent, "add", "README")
	gitlink(t, parent, name, "lib", url(root), pinned)
	git(t, parent, "commit", "-q", "-m", "parent")
	return parent
}

// gitlink stages a submodule entry: path pinned at commit, declared in
// .gitmodules under name with url.
func gitlink(t *testing.T, repo, name, path, url, commit string) {
	t.Helper()
	git(t, repo, "config", "--file", ".gitmodules", "submodule."+name+".path", path)
	git(t, repo, "config", "--file", ".gitmodules", "submodule."+name+".url", url)
	git(t, repo, "update-index", "--add", "--cacheinfo", "160000,"+commit+","+path)
	git(t, repo, "add", ".gitmodules") // not "add .": that drops the gitlink, absent on disk
}

func nativeCloneOptions(parent string) Options {
	return Options{Builder: BuilderNative, Repo: "file://" + parent, Ref: "main"}
}

// w8/m89: the clone step populates every submodule the root .gitmodules
// names, at the commit the parent pins rather than the child's tip, through
// an absolute or a parent-relative URL, before the source checks — so a root
// directory inside a submodule builds — and leaves no Git metadata behind.
func TestCloneInitializesPinnedSubmodules(t *testing.T) {
	for name, tc := range map[string]struct {
		module string
		url    func(string) string
	}{
		"absolute URL":      {"lib", func(root string) string { return "file://" + filepath.Join(root, "child") }},
		"relative URL":      {"lib", func(string) string { return "../child" }},
		"name with a space": {"my lib", func(string) string { return "../child" }},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			parent := submoduleFixture(t, tc.module, tc.url)
			o := nativeCloneOptions(parent)
			o.ExpectedCommit = git(t, parent, "rev-parse", "HEAD")
			o.RootDir = "lib"
			dest := t.TempDir()
			out, code := runClone(t, o, dest)
			license, err := os.ReadFile(filepath.Join(dest, "lib", "LICENSE"))
			if code != 0 || err != nil || string(license) != "pinned\n" {
				t.Fatalf("exit %d, lib/LICENSE %q (%v), want the pinned child file:\n%s", code, license, err, out)
			}
			if err := filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
				if err == nil && d.Name() == ".git" {
					t.Errorf("clone left Git metadata at %s", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A repository without submodules clones as before, and neither a stale
// .gitmodules entry nor a stray gitlink without one fails a build that
// succeeded before: only entries that name a real gitlink are initialized.
func TestCloneLeavesNonSubmodulesAlone(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "plain")
	must(t, os.MkdirAll(plain, 0o755))
	git(t, plain, "init", "-q", "-b", "main")
	must(t, os.WriteFile(filepath.Join(plain, "README"), []byte("plain\n"), 0o644))
	git(t, plain, "add", ".")
	git(t, plain, "commit", "-q", "-m", "plain")
	if out, code := runClone(t, nativeCloneOptions(plain), t.TempDir()); code != 0 {
		t.Fatalf("no-submodule clone exit %d:\n%s", code, out)
	}

	git(t, plain, "config", "--file", ".gitmodules", "submodule.gone.path", "gone")
	git(t, plain, "config", "--file", ".gitmodules", "submodule.gone.url", "../nowhere")
	git(t, plain, "update-index", "--add", "--cacheinfo", "160000,"+git(t, plain, "rev-parse", "HEAD")+",stray")
	must(t, os.MkdirAll(filepath.Join(plain, "vendor"), 0o755))
	must(t, os.WriteFile(filepath.Join(plain, "vendor", "kept"), []byte("kept\n"), 0o644))
	git(t, plain, "config", "--file", ".gitmodules", "submodule.vendor.path", "vendor")
	git(t, plain, "config", "--file", ".gitmodules", "submodule.vendor.url", "../nowhere")
	git(t, plain, "add", ".gitmodules", "vendor")
	git(t, plain, "commit", "-q", "-m", "stale entries and a stray gitlink")
	dest := t.TempDir()
	if out, code := runClone(t, nativeCloneOptions(plain), dest); code != 0 {
		t.Fatalf("stale/stray clone exit %d:\n%s", code, out)
	}
	if entries, _ := os.ReadDir(filepath.Join(dest, "stray")); len(entries) != 0 {
		t.Errorf("stray gitlink was populated: %v", entries)
	}
}

// A submodule the build cannot fetch fails the clone visibly, as tenant
// input, instead of building without its files.
func TestCloneFailsOnAnUnreachableSubmodule(t *testing.T) {
	parent := submoduleFixture(t, "lib", func(root string) string { return "file://" + filepath.Join(root, "missing") })
	out, code := runClone(t, nativeCloneOptions(parent), t.TempDir())
	if code != ExitTenantError || !strings.Contains(out, "lib") {
		t.Fatalf("exit %d, want ExitTenantError naming lib:\n%s", code, out)
	}
}

// A wrong parent is refused before any submodule is touched.
func TestCloneChecksTheParentBeforeSubmodules(t *testing.T) {
	parent := submoduleFixture(t, "lib", func(root string) string { return "file://" + filepath.Join(root, "missing") })
	o := nativeCloneOptions(parent)
	o.ExpectedCommit = strings.Repeat("0", 40)
	out, code := runClone(t, o, t.TempDir())
	if code != 1 || !strings.Contains(out, "fetched commit does not match EXPECTED_COMMIT") || strings.Contains(out, "missing") {
		t.Fatalf("exit %d, want the parent refusal before the unreachable submodule:\n%s", code, out)
	}
}

// A submodule path with a control character is refused before Git touches
// it: a trailing CR is CVE-2025-48384's hook-planting shape.
func TestCloneRefusesAControlCharacterInASubmodulePath(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	must(t, os.MkdirAll(parent, 0o755))
	git(t, parent, "init", "-q", "-b", "main")
	must(t, os.WriteFile(filepath.Join(parent, "README"), []byte("parent\n"), 0o644))
	git(t, parent, "add", ".")
	git(t, parent, "commit", "-q", "-m", "parent")
	must(t, os.WriteFile(filepath.Join(parent, ".gitmodules"),
		[]byte("[submodule \"lib\"]\n\tpath = \"lib\r\"\n\turl = ../child\n"), 0o644))
	git(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+git(t, parent, "rev-parse", "HEAD")+",lib\r")
	git(t, parent, "add", ".gitmodules")
	git(t, parent, "commit", "-q", "-m", "crafted path")
	out, code := runClone(t, nativeCloneOptions(parent), t.TempDir())
	if code != ExitTenantError || !strings.Contains(out, "submodule.lib.path contains a control character") {
		t.Fatalf("exit %d, want ExitTenantError refusing the path:\n%s", code, out)
	}
}

// TestCloneSubmodulesInPinnedGitImage runs the clone step in the pinned
// production Git image over HTTPS against the public fixture QA reproduced
// w8/m89 with — git-fixtures/submodule, which pins basic behind its branch
// tip — and against a local parent naming the same child by its GitHub SSH
// URL, which must go over HTTPS. Opt-in: needs Docker and network.
func TestCloneSubmodulesInPinnedGitImage(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to clone in the pinned Git image")
	}
	const basicPin = "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
	sshParent := t.TempDir()
	git(t, sshParent, "init", "-q", "-b", "main")
	gitlink(t, sshParent, "basic", "basic", "git@github.com:git-fixtures/basic.git", basicPin)
	git(t, sshParent, "commit", "-q", "-m", "ssh submodule")
	for name, o := range map[string]Options{
		"public HTTPS fixture": {Builder: BuilderNative, Repo: "https://github.com/git-fixtures/submodule", Ref: "master",
			ExpectedCommit: "b685400c1f9316f350965a5993d350bc746b0bf4"},
		"GitHub SSH URL": {Builder: BuilderNative, Repo: "file://" + sshParent, Ref: "main"},
	} {
		t.Run(name, func(t *testing.T) {
			c := buildCloneContainer(o, defaultGitImage)
			dest := t.TempDir()
			args := []string{"run", "--rm", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "--entrypoint", "sh",
				"-e", "HOME=/tmp", "-v", dest + ":" + sourceMount, "-v", sshParent + ":" + sshParent + ":ro"}
			for _, e := range c.Env {
				if e.ValueFrom == nil {
					args = append(args, "-e", e.Name+"="+e.Value)
				}
			}
			out, err := exec.Command("docker", append(args, defaultGitImage, "-eu", "-c", c.Args[0])...).CombinedOutput()
			if err != nil {
				t.Fatalf("clone: %v\n%s", err, out)
			}
			license, err := os.ReadFile(filepath.Join(dest, "basic", "LICENSE"))
			sum := sha256.Sum256(license)
			if err != nil || hex.EncodeToString(sum[:]) != "20b064910b32bce1bc04595a5b20477a28c503995ef8528929a311d6cb7a3b09" {
				t.Fatalf("basic/LICENSE = %d bytes (%v); want the parent-pinned file:\n%s", len(license), err, out)
			}
		})
	}
}
