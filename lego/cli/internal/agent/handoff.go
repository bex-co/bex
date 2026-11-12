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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ADR106 D6: moving work between a laptop and a Cloud Agent. A session
// started on an existing bex-agent/* branch begins from it (the driver's
// ensureRepo checks out the remote branch when it exists), so handoff pushes
// the worktree to such a branch with the developer's own git and creates the
// session there.

const handoffPrefix = "bex-agent/handoff-"

func handoffCommand(d deps) *cobra.Command {
	var (
		flags       createFlags
		remote      string
		notes       string
		trackedOnly bool
		chat        bool
		waiting     waitFlags
	)
	c := &cobra.Command{
		Use:   "handoff [flags] [<task…>]",
		Short: "Hand this checkout's work, uncommitted changes included, to a new Cloud Agent session",
		Long: "Commits the worktree as it is (tracked changes and untracked files .gitignore\n" +
			"does not exclude) through a temporary index, so your index, worktree, stash\n" +
			"and HEAD stay as they are; pushes that commit with your own git to\n" +
			"bex-agent/handoff-<commit>; and starts a session on that branch, which\n" +
			"begins from it. It shows what will be sent and asks first (--confirm skips).\n" +
			"--notes uploads a file the agent finds under /attachments. The pull\n" +
			"request targets the repository's default branch.",
		Example: "  bex agent handoff \"Finish the retry logic and make the tests pass\"\n" +
			"  bex agent handoff --notes NOTES.md --chat\n" +
			"  bex agent handoff --tracked-only --max-spend 5 --wait -o json \"Fix the build\"",
		RunE: func(c *cobra.Command, args []string) error {
			if err := waiting.validate(); err != nil {
				return err
			}
			if err := validateChat(d, chat, waiting); err != nil {
				return err
			}
			if c.Flags().Changed("branch") {
				return usageError("handoff picks the branch (%s<commit>); --branch does not apply", handoffPrefix)
			}
			if len(args) == 0 && flags.turn.prompt.file == "" {
				if notes == "" {
					return usageError("say what the agent should do (a task, or --notes FILE)")
				}
				args = []string{"Continue the work on this branch. My notes are in /attachments/" + filepath.Base(notes) + "."}
			}
			// Everything is validated before anything is pushed.
			in, err := flags.request(d, args)
			if err != nil {
				return err
			}
			attach := flags.turn.attach
			if notes != "" {
				if err := checkAttachable(notes); err != nil {
					return err
				}
				attach = append(attach, notes)
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			g, err := d.repo(a.ctx)
			if err != nil {
				return err
			}
			if in.Repo == "" {
				if in.Repo, err = g.remoteRepo(a.ctx, remote); err != nil {
					return err
				}
			}
			task := in.AgentConfig.Task
			snap, err := g.snapshot(a.ctx, "bex handoff: "+clip(firstLine(task), 60), trackedOnly)
			if err != nil {
				return err
			}
			in.Branch = handoffPrefix + snap.commit[:8]
			writeHandoffPlan(a.errOut, in.Repo, remote, in.Branch, snap)
			if err := d.confirm(c, fmt.Sprintf("Push %s to %s and start the session?", in.Branch, remote)); err != nil {
				return err
			}
			if err := g.pushBranch(a.ctx, remote, snap.commit, in.Branch); err != nil {
				return err
			}
			if flags.key == "" {
				sum := sha256.Sum256([]byte(task))
				in.IdempotencyKey = "handoff:" + snap.commit + ":" + hex.EncodeToString(sum[:])[:16]
			}
			created, raw, err := a.createSession(in, attach)
			if err != nil {
				if derr := g.deleteRemoteBranch(a.ctx, remote, in.Branch); derr != nil {
					return fmt.Errorf("%w\nThe pushed branch %s could not be deleted either: %v", err, in.Branch, derr)
				}
				return fmt.Errorf("%w\nThe pushed branch %s was deleted again", err, in.Branch)
			}
			switch {
			case chat:
				return d.chat(a, created.ID, chatOptions{reasoning: waiting.reasoning})
			case waiting.blocking():
				_, _ = fmt.Fprintf(a.errOut, "Handed off to %s on %s\n", created.ID, in.Branch)
				return a.await(created, 1, waiting)
			case a.format != formatText:
				return writeValue(a.out, a.format, map[string]any{"branch": in.Branch, "commit": snap.commit, "session": json.RawMessage(raw)})
			}
			_, _ = fmt.Fprintf(a.out, "Handed off to %s on %s.\n`bex agent chat %s` watches it; `bex agent pickup %s` brings its branch back.\n",
				created.ID, in.Branch, created.ID, created.ID)
			return nil
		},
	}
	flags.bind(c)
	_ = c.Flags().MarkHidden("branch")
	c.Flags().Lookup("repo").Usage = "the GitHub repository (owner/name); default: the one --remote names"
	c.Flags().Lookup("idempotency-key").Usage = "reuse a key to make re-running this command safe (default: the commit and the task)"
	waiting.bindTurn(c)
	c.Flags().StringVar(&remote, "remote", "origin", "the git remote to push to")
	c.Flags().StringVar(&notes, "notes", "", "a file of notes for the agent, uploaded as an attachment")
	c.Flags().BoolVar(&trackedOnly, "tracked-only", false, "send changes to tracked files only, no untracked files")
	c.Flags().BoolVar(&chat, "chat", false, "attach interactively after starting (a terminal only)")
	return c
}

// writeHandoffPlan says what handoff is about to push, before it asks.
func writeHandoffPlan(w io.Writer, repo, remote, branch string, s snapshot) {
	tw := newTable(w)
	tw.row("Repository", repo+" (remote "+remote+")")
	tw.row("Branch", branch)
	from := s.head[:12]
	if s.commit != s.head {
		from += " + uncommitted changes"
	}
	tw.row("From", from)
	_ = tw.Flush()
	if s.stat != "" {
		_, _ = fmt.Fprintln(w, indent(s.stat))
	}
	if s.nUntrack > 0 {
		more := ""
		if s.nUntrack > len(s.untracked) {
			more = fmt.Sprintf(", and %d more", s.nUntrack-len(s.untracked))
		}
		_, _ = fmt.Fprintf(w, "Untracked files sent (%d): %s%s\n", s.nUntrack, strings.Join(s.untracked, ", "), more)
	}
}

func pickupCommand(d deps) *cobra.Command {
	var (
		remote  string
		stash   bool
		waiting waitFlags
	)
	c := &cobra.Command{
		Use:   "pickup <ags-id>",
		Short: "Check out an agent session's branch here, to continue its work locally",
		Long: "Fetches the session's branch from the checkout's remote and switches to it,\n" +
			"fast-forward only: a local branch of that name that has diverged is refused.\n" +
			"Uncommitted changes are refused too, unless --stash stashes them first.\n" +
			"The cloud session is untouched and still takes messages.",
		Args: sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			g, err := d.repo(inv.ctx)
			if err != nil {
				return err
			}
			s, _, err := inv.api.session(inv.ctx, id)
			if err != nil {
				return err
			}
			if s.Branch == "" || s.Repo == "" {
				return fmt.Errorf("agent session %s has no repository branch to pick up", id)
			}
			local, err := g.remoteRepo(inv.ctx, remote)
			if err != nil {
				return err
			}
			if !sameRepo(local, s.Repo) {
				return usageError("this checkout's %s is %s, but %s works on %s", remote, local, id, s.Repo)
			}
			if waiting.wait {
				ctx, stop := waiting.context(inv.ctx)
				defer stop()
				res, err := inv.api.wait(ctx, []string{id}, false)
				if err != nil {
					return err
				}
				if res.timedOut {
					return stateExit(res, waiting.timeout)
				}
				s = res.sessions[0]
			} else if !settled(s) {
				_, _ = fmt.Fprintf(inv.errOut, "Note: %s is still working and may push more commits.\n", id)
			}
			if dirty, err := g.dirty(inv.ctx); err != nil {
				return err
			} else if dirty {
				if !stash {
					return usageError("the checkout has uncommitted changes: commit them, or pass --stash")
				}
				if err := g.run(inv.ctx, "stash", "push", "-u", "-m", "bex pickup "+id); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(inv.errOut, "Stashed your changes (`git stash pop` brings them back).")
			}
			tracking, err := g.fetchBranch(inv.ctx, remote, s.Branch)
			if errors.Is(err, errBranchNotPushed) {
				return fmt.Errorf("%s has not pushed its branch %s yet", id, s.Branch)
			}
			if err != nil {
				return err
			}
			commit, err := g.checkoutTracking(inv.ctx, s.Branch, tracking)
			if err != nil {
				return err
			}
			if inv.format != formatText {
				var pr any
				if url := prURL(s); url != "" {
					pr = url
				}
				return writeValue(inv.out, inv.format, map[string]any{"branch": s.Branch, "commit": commit, "pullRequest": pr})
			}
			_, _ = fmt.Fprintf(inv.out, "On %s at %s.\n", s.Branch, commit[:12])
			if pr := prURL(s); pr != "" {
				_, _ = fmt.Fprintf(inv.out, "Pull request: %s\n", pr)
			}
			return nil
		},
	}
	c.Flags().StringVar(&remote, "remote", "origin", "the git remote to fetch from")
	c.Flags().BoolVar(&stash, "stash", false, "stash uncommitted changes (untracked files too) before switching")
	c.Flags().BoolVar(&waiting.wait, "wait", false, "wait until the session's turn ends first")
	waiting.bindTimeout(c)
	return c
}
