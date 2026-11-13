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

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testRemote = "git@github.com:acme/api.git"

// gitIn runs git in dir; the author is TestMain's identity.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitFixture is a checkout whose origin is configured as acme/api on GitHub
// but reaches a local bare repository, so pushes and fetches stay on disk.
type gitFixture struct {
	dir, bare string
}

func newGitFixture(t *testing.T) gitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	f := gitFixture{dir: filepath.Join(root, "work"), bare: filepath.Join(root, "remote.git")}
	gitIn(t, root, "init", "--bare", f.bare)
	gitIn(t, root, "init", f.dir)
	gitIn(t, f.dir, "remote", "add", "origin", testRemote)
	gitIn(t, f.dir, "config", "url."+f.bare+".insteadOf", testRemote)
	writeFile(t, filepath.Join(f.dir, "app.go"), "package app\n")
	writeFile(t, filepath.Join(f.dir, ".gitignore"), "secret.env\n")
	gitIn(t, f.dir, "add", ".")
	gitIn(t, f.dir, "commit", "-m", "base")
	gitIn(t, f.dir, "push", "origin", "HEAD:refs/heads/main")
	return f
}

// state is everything handoff promises not to touch.
func (f gitFixture) state(t *testing.T) string {
	t.Helper()
	index, err := os.ReadFile(filepath.Join(f.dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(index)
	return strings.Join([]string{gitIn(t, f.dir, "rev-parse", "HEAD"), gitIn(t, f.dir, "status", "--porcelain"),
		gitIn(t, f.dir, "stash", "list"), hex.EncodeToString(sum[:])}, "|")
}

func (f gitFixture) dirtyUp(t *testing.T) {
	t.Helper()
	writeFile(t, filepath.Join(f.dir, "staged.go"), "package app // staged\n")
	gitIn(t, f.dir, "add", "staged.go")
	writeFile(t, filepath.Join(f.dir, "app.go"), "package app // edited\n")
	writeFile(t, filepath.Join(f.dir, "new.go"), "package app // untracked\n")
	writeFile(t, filepath.Join(f.dir, "secret.env"), "TOKEN=x\n")
}

func TestSnapshotLeavesTheCheckoutAlone(t *testing.T) {
	f := newGitFixture(t)
	f.dirtyUp(t)
	before := f.state(t)
	g, err := openRepo(context.Background(), f.dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := g.snapshot(context.Background(), "bex handoff: test", false)
	if err != nil {
		t.Fatal(err)
	}
	if after := f.state(t); after != before {
		t.Fatalf("the checkout changed:\nbefore %s\nafter  %s", before, after)
	}
	files := gitIn(t, f.dir, "ls-tree", "-r", "--name-only", snap.commit)
	for _, want := range []string{"app.go", "staged.go", "new.go"} {
		if !strings.Contains(files, want) {
			t.Fatalf("the snapshot lacks %s:\n%s", want, files)
		}
	}
	if strings.Contains(files, "secret.env") || gitIn(t, f.dir, "show", snap.commit+":app.go") != "package app // edited" {
		t.Fatalf("the snapshot has the ignored file or not the edit:\n%s", files)
	}
	if parent := gitIn(t, f.dir, "rev-parse", snap.commit+"^"); parent != snap.head || snap.nUntrack != 1 || snap.untracked[0] != "new.go" {
		t.Fatalf("parent %s (want %s), untracked %v", parent, snap.head, snap.untracked)
	}
	tracked, err := g.snapshot(context.Background(), "m", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gitIn(t, f.dir, "ls-tree", "-r", "--name-only", tracked.commit), "new.go") {
		t.Fatal("--tracked-only sent an untracked file")
	}
}

func TestSnapshotOfACleanCheckoutIsHEAD(t *testing.T) {
	f := newGitFixture(t)
	g, _ := openRepo(context.Background(), f.dir)
	snap, err := g.snapshot(context.Background(), "m", false)
	if err != nil || snap.commit != snap.head || snap.commit != gitIn(t, f.dir, "rev-parse", "HEAD") {
		t.Fatalf("snapshot %+v, %v", snap, err)
	}
}

func TestHandoffWithoutAGitIdentitySaysHowToSetOne(t *testing.T) {
	g := newGitFixture(t)
	t.Setenv("GIT_CONFIG_COUNT", "1") // keep useConfigOnly, drop TestMain's identity
	g.dirtyUp(t)
	before := g.state(t)
	_, d := handoffDeps(t, g)
	_, stderr, code := run(t, d, "handoff", "--confirm", "Finish the retry logic")
	if code != exitUsage || !strings.Contains(stderr, "git config --global user.email") || strings.Contains(stderr, "Please tell me who you are") {
		t.Fatalf("exit %d, want %d with the identity hint and not git's raw text:\n%s", code, exitUsage, stderr)
	}
	if after := g.state(t); after != before {
		t.Fatalf("the checkout changed:\nbefore %s\nafter  %s", before, after)
	}
	if branches := remoteBranches(t, g.bare); strings.Contains(branches, handoffPrefix) {
		t.Fatalf("a branch was pushed:\n%s", branches)
	}
}

func TestGithubRepo(t *testing.T) {
	for in, want := range map[string]string{
		"git@github.com:acme/api.git":             "acme/api",
		"https://github.com/acme/api":             "acme/api",
		"https://x-token@github.com/acme/api.git": "acme/api",
		"ssh://git@github.com/acme/my.repo.git":   "acme/my.repo",
		"https://gitlab.com/acme/api.git":         "",
	} {
		if got, _ := githubRepo(in); got != want {
			t.Errorf("githubRepo(%q) = %q, want %q", in, got, want)
		}
	}
}

// handoffDeps is the fake API with the workdir in g's checkout.
func handoffDeps(t *testing.T, g gitFixture) (*fakeAPI, deps) {
	f := newFakeAPI(t)
	d := f.deps(t).deps
	d.workdir = func() (string, error) { return g.dir, nil }
	return f, d
}

func remoteBranches(t *testing.T, bare string) string {
	return gitIn(t, bare, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads/")
}

func TestHandoffPushesTheWorktreeAndStartsTheSessionOnIt(t *testing.T) {
	g := newGitFixture(t)
	g.dirtyUp(t)
	before := g.state(t)
	f, d := handoffDeps(t, g)
	notes := filepath.Join(t.TempDir(), "NOTES.md")
	writeFile(t, notes, "context\n")
	stdout, stderr, code := run(t, d, "-o", "json", "handoff", "--confirm", "--notes", notes, "--max-spend", "2", "Finish", "the", "retry", "logic")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if after := g.state(t); after != before {
		t.Fatalf("handoff changed the checkout")
	}
	var out struct{ Branch, Commit string }
	if err := json.Unmarshal([]byte(stdout), &out); err != nil || !strings.HasPrefix(out.Branch, handoffPrefix) {
		t.Fatalf("stdout %s: %v", stdout, err)
	}
	if !strings.Contains(remoteBranches(t, g.bare), out.Branch+" "+out.Commit) {
		t.Fatalf("the remote lacks %s at %s:\n%s", out.Branch, out.Commit, remoteBranches(t, g.bare))
	}
	var body map[string]any
	_ = json.Unmarshal(f.creates[0].body, &body)
	if body["repo"] != "acme/api" || body["branch"] != out.Branch || body["maxSpendUSD"] != 2.0 ||
		asJSON(t, body["attachmentIds"]) != `["att-up1"]` || body["agentConfig"].(map[string]any)["task"] != "Finish the retry logic" {
		t.Fatalf("create body %v", body)
	}
	if key := f.creates[0].header.Get("Idempotency-Key"); !strings.HasPrefix(key, "handoff:"+out.Commit+":") {
		t.Fatalf("idempotency key %q", key)
	}
	if !strings.Contains(stderr, "+ uncommitted changes") || !strings.Contains(stderr, "Untracked files sent (1): new.go") {
		t.Fatalf("the plan was not shown:\n%s", stderr)
	}
}

func TestHandoffDeletesItsBranchWhenTheCreateIsRefused(t *testing.T) {
	g := newGitFixture(t)
	g.dirtyUp(t)
	f, d := handoffDeps(t, g)
	f.createErrs = []int{http.StatusBadRequest}
	_, stderr, code := run(t, d, "handoff", "--confirm", "Finish it")
	if code != 1 || !strings.Contains(stderr, "was deleted again") || !strings.Contains(stderr, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.Contains(remoteBranches(t, g.bare), handoffPrefix) {
		t.Fatalf("the handoff branch survived a refused create:\n%s", remoteBranches(t, g.bare))
	}
}

func TestHandoffRefusals(t *testing.T) {
	g := newGitFixture(t)
	f, d := handoffDeps(t, g)
	for _, tc := range []struct {
		args []string
		want string
		code int
	}{
		{[]string{"handoff", "Finish it"}, "pass --confirm", exitUsage},
		{[]string{"handoff", "--confirm"}, "a task, or --notes FILE", exitUsage},
		{[]string{"handoff", "--confirm", "--branch", "x", "t"}, "handoff picks the branch", exitUsage},
		// A bad flag is refused before the push, not after it.
		{[]string{"handoff", "--confirm", "--output-schema", "missing.json", "t"}, "missing.json", 1},
	} {
		if _, stderr, code := run(t, d, tc.args...); code != tc.code || !strings.Contains(stderr, tc.want) {
			t.Errorf("%v: exit %d, %s", tc.args, code, stderr)
		}
	}
	d.workdir = func() (string, error) { return t.TempDir(), nil }
	if _, stderr, code := run(t, d, "handoff", "--confirm", "t"); code != exitUsage || !strings.Contains(stderr, "not in a git checkout") {
		t.Errorf("outside a checkout: exit %d, %s", code, stderr)
	}
	if len(f.creates) != 0 || strings.Contains(remoteBranches(t, g.bare), handoffPrefix) {
		t.Fatal("a refused handoff pushed or created")
	}
}

// pickupFixture pushes an agent branch to the remote from another clone.
func pickupFixture(t *testing.T) (gitFixture, string) {
	g := newGitFixture(t)
	other := filepath.Join(t.TempDir(), "agent")
	gitIn(t, filepath.Dir(other), "clone", g.bare, other)
	gitIn(t, other, "switch", "-c", "bex-agent/ags-pick1")
	writeFile(t, filepath.Join(other, "fix.go"), "package app // the agent's fix\n")
	gitIn(t, other, "add", ".")
	gitIn(t, other, "commit", "-m", "fix")
	gitIn(t, other, "push", "origin", "bex-agent/ags-pick1")
	return g, gitIn(t, other, "rev-parse", "HEAD")
}

func pickupAPI(t *testing.T, repo, branch string) *routeFake {
	return newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			v := sessionJSON(r.PathValue("id"), stateSucceeded)
			v["repo"], v["branch"] = repo, branch
			v["outcome"] = map[string]any{"turn": 1, "pullRequest": map[string]any{"url": "https://github.com/acme/api/pull/7", "number": 7}}
			writeJSON(w, http.StatusOK, v)
		},
	})
}

func TestPickupSwitchesToTheAgentsBranch(t *testing.T) {
	g, agentCommit := pickupFixture(t)
	d := pickupAPI(t, "acme/api", "bex-agent/ags-pick1").deps()
	d.workdir = func() (string, error) { return g.dir, nil }
	stdout, stderr, code := run(t, d, "-o", "text", "pickup", "ags-pick1")
	if code != 0 || !strings.Contains(stdout, "On bex-agent/ags-pick1 at "+agentCommit[:12]) || !strings.Contains(stdout, "pull/7") {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	if branch := gitIn(t, g.dir, "branch", "--show-current"); branch != "bex-agent/ags-pick1" || gitIn(t, g.dir, "rev-parse", "HEAD") != agentCommit {
		t.Fatalf("on %s", branch)
	}
	// Again: the existing local branch fast-forwards (a no-op here).
	if _, stderr, code := run(t, d, "pickup", "ags-pick1"); code != 0 {
		t.Fatalf("second pickup: exit %d: %s", code, stderr)
	}
}

func TestPickupRefusals(t *testing.T) {
	g, _ := pickupFixture(t)
	d := pickupAPI(t, "acme/api", "bex-agent/ags-pick1").deps()
	d.workdir = func() (string, error) { return g.dir, nil }

	writeFile(t, filepath.Join(g.dir, "app.go"), "package app // local edit\n")
	if _, stderr, code := run(t, d, "pickup", "ags-pick1"); code != exitUsage || !strings.Contains(stderr, "pass --stash") {
		t.Fatalf("dirty: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "pickup", "ags-pick1", "--stash"); code != 0 || !strings.Contains(stderr, "Stashed") {
		t.Fatalf("--stash: exit %d: %s", code, stderr)
	}
	if !strings.Contains(gitIn(t, g.dir, "stash", "list"), "bex pickup ags-pick1") {
		t.Fatal("the local edit was not stashed")
	}

	// A local commit the agent's branch does not have: diverged.
	writeFile(t, filepath.Join(g.dir, "mine.go"), "package app\n")
	gitIn(t, g.dir, "add", ".")
	gitIn(t, g.dir, "commit", "-m", "mine")
	gitIn(t, g.dir, "switch", "main")
	if _, stderr, code := run(t, d, "pickup", "ags-pick1"); code != 1 || !strings.Contains(stderr, "has diverged") {
		t.Fatalf("diverged: exit %d: %s", code, stderr)
	}

	other := pickupAPI(t, "acme/web", "bex-agent/ags-pick1").deps()
	other.workdir = d.workdir
	if _, stderr, code := run(t, other, "pickup", "ags-pick1"); code != exitUsage || !strings.Contains(stderr, "works on acme/web") {
		t.Fatalf("another repo: exit %d: %s", code, stderr)
	}
	unpushed := pickupAPI(t, "acme/api", "bex-agent/ags-none").deps()
	unpushed.workdir = d.workdir
	if _, stderr, code := run(t, unpushed, "pickup", "ags-none"); code != 1 || !strings.Contains(stderr, "has not pushed its branch") {
		t.Fatalf("unpushed: exit %d: %s", code, stderr)
	}
}

func TestSSHExecsSystemSSHOnTheSandbox(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-up"], f.script["ags-down"] = []string{stateWorking}, []string{stateWorking}
	f.ssh["ags-up"] = "ags-up@ssh.bex.co"
	d := f.deps(t).deps
	var argv []string
	launched := false
	d.execSSH = func(args []string, before func()) error { argv = args; before(); launched = true; return nil }
	if _, stderr, code := run(t, d, "ssh", "ags-up", "--", "-v", "-o", "ServerAliveInterval=30", "uptime", "-p"); code != 0 || !launched {
		t.Fatalf("exit %d (launched %v): %s", code, launched, stderr)
	}
	if strings.Join(argv, " ") != "-v -o ServerAliveInterval=30 ags-up@ssh.bex.co uptime -p" {
		t.Fatalf("ssh argv %v: the destination goes after the options, before the remote command", argv)
	}
	if _, stderr, code := run(t, d, "ssh", "ags-down"); code != 1 || !strings.Contains(stderr, "no running sandbox") {
		t.Fatalf("no sandbox: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "ssh", "ags-up", "uptime"); code != exitUsage || !strings.Contains(stderr, "after --") {
		t.Fatalf("an ssh argument before --: exit %d: %s", code, stderr)
	}
	if _, _, code := run(t, d, "ssh", "ags-up", "-v"); code != exitUsage {
		t.Fatalf("an ssh flag before --: exit %d, want cobra's usage error", code)
	}
}

func TestSSHArgv(t *testing.T) {
	for in, want := range map[string]string{
		"":                   "D",
		"-v":                 "-v D",
		"-vt -i key echo hi": "-vt -i key D echo hi",
		"-o A=1 -p 22":       "-o A=1 -p 22 D",
		"ls -la":             "D ls -la",
		"-L 8080:x:80 -N":    "-L 8080:x:80 -N D",
	} {
		if got := strings.Join(sshArgv(strings.Fields(in), "D"), " "); got != want {
			t.Errorf("sshArgv(%q) = %q, want %q", in, got, want)
		}
	}
}
