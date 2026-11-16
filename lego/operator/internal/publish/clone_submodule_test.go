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

// fixtureRepo makes a one-commit repository at dir on branch, holding files
// (path → content).
func fixtureRepo(t *testing.T, dir, branch string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, dir, "init", "-q", "-b", branch)
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "-q", "-m", "fixture")
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
	return runClone(t, root, o)
}

// runClone runs the direct publish's real clone script for o under root and
// returns the staged tree, the termination message and the exit status.
func runClone(t *testing.T, root string, o Options) (out, msg string, code int) {
	t.Helper()
	clone := cloneContainer(o)
	work := filepath.Join(root, "work")
	out = filepath.Join(root, "out")
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

// A submodule that cannot be fetched fails the build as the tenant's source
// (w32/028), in this operator's words, naming it, and stages nothing.
func TestDirectPublishFailsOnAnUnreachableSubmodule(t *testing.T) {
	out, msg, code := directPublish(t, func(root string) string { return "file://" + filepath.Join(root, "missing") })
	entries, _ := os.ReadDir(out)
	if code != userSourceExit || !strings.Contains(msg, `could not fetch the submodule "site/theme"`) || len(entries) != 0 {
		t.Fatalf("exit %d, message %q, staged %v; want a named failure and nothing staged", code, msg, entries)
	}
}

// w4/565: a site published from the repository root stages its files, never
// the repository's .git directory, which the static server would serve.
func TestDirectPublishOfTheRepositoryRootLeavesGitMetadataOut(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	fixtureRepo(t, repo, "master", map[string]string{"README": "Hello World!\n", ".gitignore": "x\n", "docs/index.html": "docs\n"})

	for _, publishPath := range []string{"", "."} {
		o := testOptions()
		o.Image, o.Repo, o.Ref, o.PublishPath = "", "file://"+repo, "master", publishPath
		run := filepath.Join(root, "run"+publishPath)
		if err := os.MkdirAll(run, 0o755); err != nil {
			t.Fatal(err)
		}
		out, msg, code := runClone(t, run, o)
		if code != 0 {
			t.Fatalf("publish path %q: exit %d (%q)", publishPath, code, msg)
		}
		if _, err := os.Stat(filepath.Join(out, ".git")); !os.IsNotExist(err) {
			t.Errorf("publish path %q staged the repository's .git for upload: %v", publishPath, err)
		}
		for _, name := range []string{"README", ".gitignore", "docs/index.html"} {
			if _, err := os.Stat(filepath.Join(out, name)); err != nil {
				t.Errorf("publish path %q did not stage %s: %v", publishPath, name, err)
			}
		}
	}
}

// w32/028: a direct publish of a branch, commit or repository that cannot be
// fetched is the tenant's own source, as on a repo build (w7/m82). The real
// clone script exits userSourceExit, so the observation fails the build with
// the script's own sentence; git's 128 used to close the deploy update_failed.
func TestDirectPublishOfAnUnfetchableRefIsAUserSourceError(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	fixtureRepo(t, repo, "main", map[string]string{"index.html": "site\n"})

	for _, tc := range []struct{ name, repo, ref string }{
		{"missing branch", "file://" + repo, "no-such-branch-b97f"},
		{"missing commit", "file://" + repo, "0123456789abcdef0123456789abcdef01234567"},
		{"unreadable repository", "file://" + filepath.Join(root, "gone"), "main"},
	} {
		o := testOptions()
		o.Image, o.Repo, o.Ref, o.PublishPath = "", tc.repo, tc.ref, "."
		run := filepath.Join(root, strings.ReplaceAll(tc.name, " ", "-"))
		if err := os.MkdirAll(run, 0o755); err != nil {
			t.Fatal(err)
		}
		_, msg, code := runClone(t, run, o)
		want := `could not fetch "` + tc.ref + `" from this service's repository — check that the branch or commit exists and that bex still has access to the repo`
		if code != userSourceExit || msg != want {
			t.Errorf("%s: exit %d, message %q; want exit %d with %q", tc.name, code, msg, userSourceExit, want)
			continue
		}
		if obs := failedPublishObservation(t, cloneStep, int32(code), msg); !obs.UserError || obs.Message != want {
			t.Errorf("%s: observation = %+v, want the bare sentence flagged as a user error", tc.name, obs)
		}
	}
}
