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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// `bex agent chat` is ADR106 D2: a session live in the terminal. Line mode,
// not a full-screen UI, so scrollback, tmux and pipes to tee keep working.
// Public live turns are disabled (ADR047 D9), so a message typed while a turn
// runs waits in the client and goes out by steer when the turn ends. Leaving
// never cancels the session (ADR047: a client disconnect is not a cancel);
// there are no permission prompts (DO_NOT_DO #APPROVAL-MODE).

type chatOptions struct {
	readOnly  bool
	reasoning bool
}

// errDetached ends a chat the user left (/quit, end of input); the session
// keeps running.
var errDetached = errors.New("detached")

const chatHelp = `Type a message and press Enter to send it; while a turn runs it waits
and goes out when the turn ends. Commands:
  /attach FILE        attach a file to the next message
  /skill NAME[:ARGS]  use a workspace skill in the next message
  /files [flags]      list (or --download DIR) the files the agent shared
  /open [--dashboard] open the sandbox in bex desktop, or the dashboard page
  /tags TAG…|--clear  replace the session's tags
  /tree               show the session's child sessions
  /cost               show the session's model cost
  /archive            archive the session (it stops taking messages)
  /cancel             cancel the session, after a confirmation
  /quit, /exit        leave; the session keeps running (also Ctrl-D, Ctrl-C)`

// chatVerbs are the `bex agent` commands a chat runs on its session.
var chatVerbs = []string{"files", "open", "tags", "tree", "cost", "archive"}

func chatCommand(d deps) *cobra.Command {
	var (
		flags createFlags
		opts  chatOptions
	)
	c := &cobra.Command{
		Use:     "chat [<ags-id> | [flags] <task…>]",
		Aliases: []string{"attach"},
		Short:   "Watch and steer an agent session live in the terminal",
		Long: "Attaches to a session: its transcript so far, then the agent's work as it\n" +
			"happens, with a line to send follow-ups. With no session and the run\n" +
			"flags or a task, starts a new session first; with neither, picks one of\n" +
			"your recent sessions. Ctrl-D, Ctrl-C and /quit leave without cancelling.\n\n" + chatHelp,
		Example: "  bex agent chat ags-…\n" +
			"  bex agent chat --repo acme/api \"Fix the flaky test in ci.log\"\n" +
			"  bex agent chat --read-only https://dashboard.bex.co/agents/ags-…",
		RunE: func(c *cobra.Command, args []string) error {
			if !d.interactive() {
				return usageError("chat needs a terminal; in a script use `bex agent run --follow` or `bex agent logs --follow`")
			}
			var id string
			if len(args) > 0 {
				if ref, err := sessionRef(args[0]); err == nil {
					if len(args) > 1 {
						return usageError("give a session to attach to, or a task for a new one, not both")
					}
					id = ref
				}
			}
			if id == "" && len(args) == 0 && !flags.given() {
				picked, err := d.pick(c, sessionsKind)
				if err != nil {
					return err
				}
				id = picked
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			if id == "" {
				if len(args) == 0 && flags.turn.prompt.file == "" {
					args = []string{d.ask(a.errOut, "Task for the new session: ")}
				}
				created, _, err := flags.create(d, a, args)
				if err != nil {
					return err
				}
				id = created.ID
			}
			return d.chat(a, id, opts)
		},
	}
	flags.bind(c)
	c.Flags().BoolVar(&opts.readOnly, "read-only", false, "watch without an input line (for a session you can read but not steer)")
	c.Flags().BoolVar(&opts.reasoning, "reasoning", false, "also show the agent's reasoning")
	return c
}

// chat is one attached terminal: it shows each turn as it runs, then sends
// the next message, until the user leaves or the session closes.
func (d deps) chat(a workspaceAPI, id string, opts chatOptions) error {
	out := &lockedWriter{w: a.out}
	ch := &chatState{
		d: d, a: a, id: id, readOnly: opts.readOnly, out: out,
		lines:    readLines(d.stdin),
		renderer: newChunkRenderer(newPartPrinter(out, false, opts.reasoning), 0),
	}
	err := ch.run()
	if errors.Is(err, errDetached) || (err != nil && a.ctx.Err() != nil) {
		_, _ = fmt.Fprintf(a.errOut, "\nLeft %s; it keeps running. `bex agent chat %s` attaches again.\n", id, id)
		return nil
	}
	return err
}

type chatState struct {
	d        deps
	a        workspaceAPI
	id       string
	readOnly bool
	out      *lockedWriter
	lines    <-chan string
	renderer *chunkRenderer

	pending   pendingMessage
	summaries int // the last turn whose summary was printed
}

// pendingMessage is the next message and what goes with it. Its files are
// uploaded once, even when the send has to wait for a turn in flight.
type pendingMessage struct {
	prompt string
	files  []string
	ids    []string
	skills []skillInvocation
}

func (ch *chatState) say(format string, args ...any) {
	_, _ = fmt.Fprintf(ch.out, format+"\n", args...)
}

func (ch *chatState) run() error {
	s, _, err := ch.a.api.session(ch.a.ctx, ch.id)
	if err != nil {
		return err
	}
	ch.say("%s", chatHeader(s))
	if !ch.readOnly {
		ch.say("(/help for commands; Ctrl-D or /quit leaves without cancelling)")
	}
	for {
		s, err := ch.watchTurn()
		if err != nil {
			return err
		}
		if s.Outcome != nil && s.Outcome.Turn > ch.summaries {
			ch.summaries = s.Outcome.Turn
			ch.say("%s", turnSummary(s))
		}
		if !s.Steerable {
			ch.say("The session is %s and takes no more messages.", s.State)
			return nil
		}
		if ch.readOnly {
			err = ch.awaitNextTurn(s.Turns)
		} else {
			err = ch.sendNext()
		}
		if err != nil {
			return err
		}
	}
}

// next waits for a typed line, done, a timer, or the user leaving; a nil
// channel never fires.
func (ch *chatState) next(done <-chan struct{}, timer <-chan time.Time) (line string, fired bool, err error) {
	select {
	case line, ok := <-ch.lines:
		if !ok {
			return "", false, errDetached
		}
		return line, false, nil
	case <-done:
		return "", true, nil
	case <-timer:
		return "", true, nil
	case <-ch.a.ctx.Done():
		return "", false, ch.a.ctx.Err()
	}
}

// watchTurn shows the running turn until the session settles while taking
// typed lines: commands run at once, a message waits for the turn's end.
func (ch *chatState) watchTurn() (session, error) {
	ctx, cancel := context.WithCancel(ch.a.ctx)
	defer cancel()
	var (
		res waitResult
		err error
	)
	finished := make(chan struct{})
	go func() {
		res, err = ch.a.api.followRendered(ctx, ch.id, ch.renderer)
		close(finished)
	}()
	for {
		line, fired, lerr := ch.next(finished, nil)
		if fired {
			break
		}
		if lerr == nil {
			lerr = ch.input(line, true)
		}
		if lerr != nil {
			cancel()
			<-finished // nothing prints after the chat has returned
			return session{}, lerr
		}
	}
	if err != nil {
		return session{}, err
	}
	if len(res.sessions) == 0 {
		return session{}, errInterrupted
	}
	return res.sessions[0], nil
}

// sendNext sends the pending message, or the next one typed.
func (ch *chatState) sendNext() error {
	for ch.pending.prompt == "" {
		_, _ = fmt.Fprint(ch.out, "> ")
		line, _, err := ch.next(nil, nil)
		if err == nil {
			err = ch.input(line, false)
		}
		if err != nil {
			return err
		}
	}
	p := &ch.pending
	if p.ids == nil {
		ids, err := uploadAll(ch.a.invocation, ch.a.owner, p.files)
		if err != nil {
			return err
		}
		p.ids = ids
	}
	_, _, err := ch.a.api.steer(ch.a.ctx, ch.id, steerRequest{Prompt: p.prompt, AttachmentIDs: p.ids, Skills: p.skills})
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Code == codeSessionTurnInFlight {
		return nil // a turn someone else started is running; send after it
	}
	if err != nil {
		return err
	}
	ch.pending = pendingMessage{}
	return nil
}

// awaitNextTurn is read-only mode between turns: wait, backing off, for
// someone else's next message to start a turn, still taking commands.
func (ch *chatState) awaitNextTurn(turns int) error {
	var poll poller
	delay := pollFirst
	for {
		s, _, ok, err := ch.a.api.readSession(ch.a.ctx, ch.id, &poll)
		if err != nil {
			return err
		}
		if ok && (s.Turns > turns || !settled(s)) {
			return nil
		}
		line, fired, err := ch.next(nil, time.After(delay))
		if err != nil {
			return err
		}
		if !fired {
			if err := ch.input(line, true); err != nil {
				return err
			}
			continue
		}
		delay = nextPoll(delay)
	}
}

// input handles one typed line: a command, or a message (held while a turn
// runs, sent at once between turns).
func (ch *chatState) input(line string, running bool) error {
	line = strings.TrimSpace(line)
	switch {
	case line == "":
	case strings.HasPrefix(line, "/"):
		return ch.command(strings.Fields(line))
	case ch.readOnly:
		ch.say("read-only: messages are not sent (attach without --read-only to steer)")
	default:
		ch.pending.prompt = line
		if running {
			ch.say("queued — sends when this turn ends")
		}
	}
	return nil
}

func (ch *chatState) command(words []string) error {
	verb, rest := strings.TrimPrefix(words[0], "/"), words[1:]
	switch verb {
	case "quit", "exit":
		return errDetached
	case "help":
		ch.say("%s", chatHelp)
	case "attach":
		for _, path := range rest {
			if err := checkAttachable(path); err != nil {
				ch.say("%v", err)
				return nil
			}
		}
		ch.pending.files, ch.pending.ids = append(ch.pending.files, rest...), nil
		ch.say("%d file(s) go with the next message", len(ch.pending.files))
	case "skill":
		skills, err := parseSkills(rest)
		if err != nil || len(skills) == 0 {
			ch.say("usage: /skill NAME[:ARGUMENTS]")
			return nil
		}
		ch.pending.skills = append(ch.pending.skills, skills...)
		ch.say("the next message uses %d skill(s)", len(ch.pending.skills))
	case "cancel":
		ch.say("Cancel %s? It stops and takes no more messages. [y/N]", ch.id)
		answer, _, err := ch.next(nil, nil)
		if err != nil {
			return err
		}
		if !isYes(answer) {
			ch.say("not canceled")
			return nil
		}
		s, _, err := ch.a.api.cancel(ch.a.ctx, ch.id)
		if err != nil {
			ch.say("cancel: %v", err)
			return nil
		}
		ch.say("%s is %s.", s.ID, s.State)
	default:
		if !slices.Contains(chatVerbs, verb) {
			ch.say("unknown command /%s (/help lists them)", verb)
			return nil
		}
		ch.sub(verb, rest...)
	}
	return nil
}

// sub runs `bex agent <verb> <id> args…` in place, so a chat command is the
// shipped command, output and all; its error is shown, not fatal.
func (ch *chatState) sub(verb string, args ...string) {
	root := agentRoot(ch.d, string(formatText))
	root.SetArgs(append([]string{"agent", verb, ch.id}, args...))
	root.SetOut(ch.out)
	root.SetErr(ch.out)
	if err := root.ExecuteContext(ch.a.ctx); err != nil {
		ch.say("%s: %v", verb, err)
	}
}

// agentRoot is `bex agent` under a root with upstream's persistent --output
// and --confirm, for running a command in place.
func agentRoot(d deps, output string) *cobra.Command {
	root := &cobra.Command{Use: "bex", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().StringP("output", "o", output, "")
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "")
	root.AddCommand(newCommand(d))
	return root
}

func chatHeader(s session) string {
	parts := []string{s.ID}
	if s.Repo != "" {
		parts = append(parts, repoBranch(s.Repo, s.Branch))
	}
	parts = append(parts, firstNonEmpty(s.AgentConfig.Model, s.AgentConfig.Agent), s.State)
	if cost := costUSD(s); cost != "" {
		parts = append(parts, "$"+cost)
	}
	return strings.Join(parts, " · ")
}

func turnSummary(s session) string {
	parts := []string{fmt.Sprintf("── turn %d %s", s.Outcome.Turn, s.State)}
	if pr := prURL(s); pr != "" {
		parts = append(parts, pr)
	}
	if cost := costUSD(s); cost != "" {
		parts = append(parts, "$"+cost)
	}
	if why := firstNonEmpty(failureCode(s), clip(firstLine(s.FailureReason), 80)); why != "" {
		parts = append(parts, why)
	}
	return strings.Join(parts, " · ") + " ──"
}

// readLines delivers r's lines on a channel, closed at the end of input.
func readLines(r io.Reader) <-chan string {
	lines := make(chan string)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return lines
}

// lockedWriter keeps the stream's lines and the input's replies whole.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
