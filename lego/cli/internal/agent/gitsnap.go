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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The local half of ADR106 D6. Everything runs the user's own `git`, so their
// config, credentials and hooks apply exactly as when they push by hand; the
// snapshot never touches the worktree, the index, the stash or HEAD.

// gitRepo is a checkout the CLI reads and pushes from.
type gitRepo struct {
	dir string
	env []string // extra environment for every call (the temporary index)
}

// gitError is a failed git run: its exit code and what it said.
type gitError struct {
	args   []string
	stderr string
	code   int
}

func (e *gitError) Error() string {
	if e.stderr != "" {
		return "git " + e.args[0] + ": " + e.stderr
	}
	return fmt.Sprintf("git %s: exit status %d", e.args[0], e.code)
}

// errBranchNotPushed is a fetch of a branch the remote does not have.
var errBranchNotPushed = errors.New("the remote has no such branch")

// openRepo finds the worktree dir is in.
func openRepo(ctx context.Context, dir string) (gitRepo, error) {
	top, err := gitRepo{dir: dir}.output(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return gitRepo{}, usageError("not in a git checkout (%v)", err)
	}
	return gitRepo{dir: top}, nil
}

// repo is the checkout of the working directory.
func (d deps) repo(ctx context.Context) (gitRepo, error) {
	dir, err := d.workdir()
	if err != nil {
		return gitRepo{}, err
	}
	return openRepo(ctx, dir)
}

// exec runs git with stdout to stdout (nil: discarded). Git's messages are
// in the C locale, so the CLI can recognise the ones it handles.
func (g gitRepo) exec(ctx context.Context, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), g.env...)
	// A helper git started (ssh, a credential helper) can outlive a killed
	// git and hold its pipes; stop waiting for it.
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &gitError{args: args, stderr: strings.TrimSpace(stderr.String()), code: exitErr.ExitCode()}
	}
	return err
}

func (g gitRepo) run(ctx context.Context, args ...string) error { return g.exec(ctx, nil, args...) }

func (g gitRepo) output(ctx context.Context, args ...string) (string, error) {
	var out bytes.Buffer
	err := g.exec(ctx, &out, args...)
	return strings.TrimSpace(out.String()), err
}

// snapshot is a commit of the worktree as it is now, parented on HEAD.
type snapshot struct {
	commit    string
	head      string
	untracked []string // the first untrackedShown untracked files sent
	nUntrack  int      // how many untracked files are sent
	stat      string   // diffstat against HEAD, at most 20 files
}

const untrackedShown = 20

// snapshot commits the worktree (tracked changes, plus untracked files that
// .gitignore does not exclude unless trackedOnly) through a temporary copy of
// the index, so the real index, the worktree, the stash and HEAD stay as they
// are. A clean worktree is HEAD itself.
func (g gitRepo) snapshot(ctx context.Context, message string, trackedOnly bool) (snapshot, error) {
	refs, err := g.output(ctx, "rev-parse", "--git-path", "index", "HEAD", "HEAD^{tree}")
	lines := strings.Split(refs, "\n")
	if err != nil || len(lines) != 3 {
		return snapshot{}, usageError("the checkout has no commit to start from (%v)", err)
	}
	indexPath, head, headTree := lines[0], lines[1], lines[2]
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(g.dir, indexPath)
	}
	tmp, err := os.MkdirTemp("", "bex-handoff-")
	if err != nil {
		return snapshot{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }() // git's index.lock too, if git died
	index := filepath.Join(tmp, "index")
	// Starting from a copy keeps git's file-status cache, so `add` re-reads
	// only what changed. A missing index is an empty one; any other failure
	// must stop here: an empty index would snapshot a tree without the files.
	if err := copyFile(indexPath, index); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return snapshot{}, fmt.Errorf("copy the git index: %w", err)
	}
	staged := gitRepo{dir: g.dir, env: []string{"GIT_INDEX_FILE=" + index}}
	s := snapshot{commit: head, head: head}
	if trackedOnly {
		err = staged.run(ctx, "add", "-u")
	} else {
		list := &nameList{keep: untrackedShown}
		if err := g.exec(ctx, list, "ls-files", "--others", "--exclude-standard", "-z"); err != nil {
			return snapshot{}, err
		}
		s.untracked, s.nUntrack = list.names, list.count
		err = staged.run(ctx, "add", "-A")
	}
	if err != nil {
		return snapshot{}, err
	}
	tree, err := staged.output(ctx, "write-tree")
	if err != nil {
		return snapshot{}, err
	}
	if tree == headTree {
		return s, nil
	}
	if s.commit, err = g.output(ctx, "commit-tree", tree, "-p", head, "-m", message); err != nil {
		return snapshot{}, err
	}
	s.stat, _ = g.output(ctx, "diff", "--stat", "--stat-count="+fmt.Sprint(untrackedShown), head, s.commit)
	return s, nil
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

// nameList counts NUL-separated names, keeping the first keep of them.
type nameList struct {
	keep    int
	names   []string
	count   int
	partial []byte
}

func (l *nameList) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != 0 {
			if len(l.names) < l.keep {
				l.partial = append(l.partial, b)
			}
			continue
		}
		if len(l.names) < l.keep {
			l.names = append(l.names, string(l.partial))
		}
		l.partial, l.count = l.partial[:0], l.count+1
	}
	return len(p), nil
}

// dirty reports uncommitted changes, untracked files included.
func (g gitRepo) dirty(ctx context.Context) (bool, error) {
	status, err := g.output(ctx, "status", "--porcelain")
	return status != "", err
}

// pushBranch makes remote's branch the commit, with the user's git.
func (g gitRepo) pushBranch(ctx context.Context, remote, commit, branch string) error {
	return g.run(ctx, "push", remote, commit+":refs/heads/"+branch)
}

func (g gitRepo) deleteRemoteBranch(ctx context.Context, remote, branch string) error {
	return g.run(ctx, "push", remote, "--delete", branch)
}

// fetchBranch fetches remote's branch into its remote-tracking ref and
// returns that ref; errBranchNotPushed when the remote has no such branch.
func (g gitRepo) fetchBranch(ctx context.Context, remote, branch string) (string, error) {
	tracking := "refs/remotes/" + remote + "/" + branch
	err := g.run(ctx, "fetch", remote, "+refs/heads/"+branch+":"+tracking)
	var gitErr *gitError
	if errors.As(err, &gitErr) && strings.Contains(gitErr.stderr, "couldn't find remote ref") {
		return "", errBranchNotPushed
	}
	return tracking, err
}

// checkoutTracking switches to branch at tracking: a new local branch tracks
// it, an existing one fast-forwards to it. It returns the commit, or an
// error naming how to reconcile a local branch that has diverged.
func (g gitRepo) checkoutTracking(ctx context.Context, branch, tracking string) (string, error) {
	refs, err := g.output(ctx, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/"+branch, tracking)
	if err != nil {
		return "", err
	}
	commits := map[string]string{}
	for _, line := range strings.Split(refs, "\n") {
		if name, sha, ok := strings.Cut(line, " "); ok {
			commits[name] = sha
		}
	}
	target, local := commits[tracking], commits["refs/heads/"+branch]
	switch local {
	case "":
		return target, g.run(ctx, "switch", "-c", branch, "--track", strings.TrimPrefix(tracking, "refs/remotes/"))
	case target:
		return target, g.run(ctx, "switch", branch)
	}
	err = g.run(ctx, "merge-base", "--is-ancestor", local, target)
	var gitErr *gitError
	if errors.As(err, &gitErr) && gitErr.code == 1 {
		return "", fmt.Errorf("your local %s has diverged from the agent's; reconcile it by hand:\n  git switch %s && git rebase %s", branch, branch, strings.TrimPrefix(tracking, "refs/remotes/"))
	}
	if err != nil {
		return "", err
	}
	if err := g.run(ctx, "switch", branch); err != nil {
		return "", err
	}
	return target, g.run(ctx, "merge", "--ff-only", target)
}

// githubRemote matches a github.com remote URL: https, ssh or scp-like.
var githubRemote = regexp.MustCompile(`^(?:https?://(?:[^@/]+@)?|ssh://git@|git@)github\.com[:/]([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

// githubRepo is the owner/name a GitHub remote URL names.
func githubRepo(remoteURL string) (string, bool) {
	m := githubRemote.FindStringSubmatch(strings.TrimSpace(remoteURL))
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}

// remoteRepo is the GitHub owner/name of remote, from its URL as configured
// (before any url.*.insteadOf rewrite).
func (g gitRepo) remoteRepo(ctx context.Context, remote string) (string, error) {
	u, err := g.output(ctx, "config", "--get", "remote."+remote+".url")
	if err != nil {
		return "", usageError("no git remote %q", remote)
	}
	repo, ok := githubRepo(u)
	if !ok {
		return "", usageError("remote %q (%s) is not a github.com repository; pass --repo owner/name", remote, redactURL(u))
	}
	return repo, nil
}

// redactURL drops any credentials a remote URL embeds before it is shown.
func redactURL(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		u.User = nil
		return u.String()
	}
	return raw
}

// sameRepo compares owner/name case-insensitively, as GitHub does.
func sameRepo(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, ".git"), strings.TrimSuffix(b, ".git"))
}
