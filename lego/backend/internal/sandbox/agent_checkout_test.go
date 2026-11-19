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

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m491: bex-api reads what a session sandbox's checkout holds undelivered
// before it decides to keep the disk. The script runs as written against a
// real Git checkout, with only its working directory moved.
func TestTheCheckoutScriptReadsUndeliveredWork(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	run := func(t *testing.T, dir string) store.TurnCheckout {
		t.Helper()
		script := strings.Replace(agentCheckoutScript, "cd /workspace", "cd "+dir, 1)
		out, err := exec.Command("/bin/sh", "-c", script).Output()
		if err != nil {
			t.Fatalf("script: %v", err)
		}
		got, err := parseSessionCheckout(string(out))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	git := func(t *testing.T, dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=a@bex.co", "-c", "user.name=a"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	root := t.TempDir()
	if got := run(t, filepath.Join(root, "missing")); got != (store.TurnCheckout{}) {
		t.Fatalf("a missing workspace = %+v", got)
	}
	plain := filepath.Join(root, "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := run(t, plain); got != (store.TurnCheckout{}) {
		t.Fatalf("an empty workspace = %+v", got)
	}
	if err := os.WriteFile(filepath.Join(plain, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(t, plain); !got.Dirty {
		t.Fatalf("a workspace with a file that is no checkout = %+v, want dirty", got)
	}

	remote := filepath.Join(root, "remote.git")
	git(t, root, "init", "--bare", "--initial-branch=main", remote)
	repo := filepath.Join(root, "repo")
	git(t, root, "clone", remote, repo)
	git(t, repo, "commit", "--allow-empty", "-m", "seed")
	git(t, repo, "push", "origin", "HEAD:main")
	if got := run(t, repo); got != (store.TurnCheckout{}) {
		t.Fatalf("a pushed, clean checkout = %+v", got)
	}
	git(t, repo, "commit", "--allow-empty", "-m", "one")
	git(t, repo, "commit", "--allow-empty", "-m", "two")
	if err := os.WriteFile(filepath.Join(repo, "half.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(t, repo); !got.Dirty || got.UnpushedCommits != 2 {
		t.Fatalf("two unpushed commits and an untracked file = %+v", got)
	}
	git(t, repo, "push", "origin", "HEAD:refs/heads/bex-agent/ags-x")
	if got := run(t, repo); !got.Dirty || got.UnpushedCommits != 0 {
		t.Fatalf("after the push = %+v, want only the dirty tree", got)
	}
}

func TestParseSessionCheckout(t *testing.T) {
	for in, want := range map[string]store.TurnCheckout{
		"empty\n":     {},
		"files\n":     {Dirty: true},
		"git 0 0\n":   {},
		"git 1 3\n":   {Dirty: true, UnpushedCommits: 3},
		"  git 0 1  ": {UnpushedCommits: 1},
	} {
		if got, err := parseSessionCheckout(in); err != nil || got != want {
			t.Errorf("%q = %+v / %v, want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "git", "git x 1", "git 1 -2", "something else", "git 1 2 3"} {
		if _, err := parseSessionCheckout(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
