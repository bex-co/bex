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

package publish

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureGit(t *testing.T, dir string, args ...string) string {
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

// directPublish runs the direct publish's real clone script for a parent
// whose site/theme is a submodule pinned behind the child's tip at url, and
// returns the staged tree, the termination message and the exit status.
func directPublish(t *testing.T, url func(root string) string) (out, msg string, code int) {
	t.Helper()
	root := t.TempDir()
	child, parent := filepath.Join(root, "child"), filepath.Join(root, "parent")
	for _, d := range []string{child, filepath.Join(parent, "site")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, child, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(child, "theme.css"), []byte("pinned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, child, "add", ".")
	fixtureGit(t, child, "commit", "-q", "-m", "pinned")
	pinned := fixtureGit(t, child, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(child, "theme.css"), []byte("tip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, child, "commit", "-q", "-am", "tip")
	fixtureGit(t, parent, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(parent, "site", "index.html"), []byte("site\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, parent, "config", "--file", ".gitmodules", "submodule.theme.path", "site/theme")
	fixtureGit(t, parent, "config", "--file", ".gitmodules", "submodule.theme.url", url(root))
	fixtureGit(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+pinned+",site/theme")
	fixtureGit(t, parent, "add", ".gitmodules", "site/index.html")
	fixtureGit(t, parent, "commit", "-q", "-m", "parent")

	o := testOptions()
	o.Image, o.Repo, o.Ref, o.PublishPath = "", "file://"+parent, "main", "site"
	clone := cloneContainer(o)
	work, out := filepath.Join(root, "work"), filepath.Join(root, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	term := filepath.Join(root, "termination-log")
	script := strings.NewReplacer("/work", work, outMount, out, "/dev/termination-log", term).Replace(clone.Command[2])
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always")
	for _, e := range clone.Env {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	raw, _ := os.ReadFile(term)
	return out, strings.TrimSpace(string(raw)), code
}

// w8/m89: a static site published straight from its repository gets the
// submodules its publish directory holds, at the commit the parent pins.
func TestDirectPublishIncludesPinnedSubmodules(t *testing.T) {
	out, msg, code := directPublish(t, func(string) string { return "../child" })
	theme, err := os.ReadFile(filepath.Join(out, "theme", "theme.css"))
	if code != 0 || err != nil || string(theme) != "pinned\n" {
		t.Fatalf("exit %d (%q), theme.css %q (%v); want the pinned child file staged", code, msg, theme, err)
	}
	if _, err := os.Stat(filepath.Join(out, "theme", ".git")); !os.IsNotExist(err) {
		t.Errorf("the submodule's gitfile was staged for upload: %v", err)
	}
}

// A submodule that cannot be fetched fails the publish in this operator's
// words, naming it, and stages nothing.
func TestDirectPublishFailsOnAnUnreachableSubmodule(t *testing.T) {
	out, msg, code := directPublish(t, func(root string) string { return "file://" + filepath.Join(root, "missing") })
	entries, _ := os.ReadDir(out)
	if code == 0 || !strings.Contains(msg, `could not fetch the submodule "site/theme"`) || len(entries) != 0 {
		t.Fatalf("exit %d, message %q, staged %v; want a named failure and nothing staged", code, msg, entries)
	}
}
