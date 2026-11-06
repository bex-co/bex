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

// Package agent is `bex agent`: Cloud Agents from the terminal and from CI, a
// thin client of bex-api's agent-session REST contract (ADR094, ADR095). It
// starts sessions, waits for them, streams their transcript and fetches what
// they delivered; every decision stays with the API.
package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dashboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bex-co/bex/lego/cli/internal/desktop"
	"github.com/bex-co/bex/lego/cli/internal/telemetry"
)

// Exit codes. A command that waits (run/send --wait or --follow, wait, logs
// --follow) exits by the state the turn ended in, so a CI step can branch on
// it without parsing output.
// Any other error, from the CLI or the API (network, auth, a refused request),
// exits 1.
const (
	exitUsage      = 2 // the command line is wrong
	exitFailed     = 3 // the turn ended failed (see failureReasonCode)
	exitCanceled   = 4 // the session was canceled
	exitNeedsInput = 5 // the agent is waiting for a reply (`bex agent send`)
	exitTimeout    = 124
	// exitInterrupted is SIGINT or SIGTERM ending a wait, the shell's 128+2.
	exitInterrupted = 130
)

// defaultAgent is the dashboard composer's default agent.
const defaultAgent = "claude"

// deps is the process boundary, injectable for tests.
type deps struct {
	api         func() (*api, error)
	workspace   func() (string, error)
	stdin       io.Reader
	interactive func() bool
	dashboard   func() string
	openURL     func(string) error
	canDesktop  func() bool
	openDesktop func(ctx context.Context, id, sshAddress string, beforeExec func()) error
	newKey      func() string
	lookupEnv   func(string) (string, bool)
}

func osDeps() deps {
	return deps{
		api:         defaultAPI,
		workspace:   config.WorkspaceID,
		stdin:       os.Stdin,
		interactive: func() bool { return term.IsTerminal(int(os.Stdout.Fd())) },
		dashboard:   config.DashboardURL,
		openURL:     dashboard.Open,
		canDesktop:  desktop.Available,
		openDesktop: desktop.OpenSession,
		newKey:      newIdempotencyKey,
		lookupEnv:   os.LookupEnv,
	}
}

// Command is `bex agent`.
func Command() *cobra.Command {
	return newCommand(osDeps())
}

func newCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:     "agent",
		Aliases: []string{"agents"},
		Short:   "Run Cloud Agents: start, wait for and collect agent sessions",
		Long: "Cloud Agents are bex's cloud coding agents: each session runs in its own\n" +
			"sandbox, can open a pull request, share files and return a structured\n" +
			"result. `bex agent` drives them from a terminal or a CI job.\n\n" +
			"Exit codes (commands that wait: run/send with --wait or --follow, wait,\n" +
			"logs --follow):\n" +
			"  0    the turn succeeded\n" +
			"  1    the CLI or the API failed\n" +
			"  2    usage error\n" +
			"  3    the turn failed (failureReasonCode says why)\n" +
			"  4    the session was canceled\n" +
			"  5    the agent needs input — reply with `bex agent send`\n" +
			"  124  --timeout elapsed while the session was still working\n" +
			"  130  interrupted (Ctrl-C or SIGTERM)\n\n" +
			"Use -o json (or yaml) for the API's own answer on stdout; streamed\n" +
			"transcript output then goes to stderr.",
		Example: "  bex agent run --repo acme/api --attach crash.log --wait -o json \"Fix the crash in crash.log\"\n" +
			"  bex agent send ags-… --follow \"Also add a regression test\"\n" +
			"  bex agent files ags-… --download ./out",
		SilenceUsage: true,
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return command.NewExitError(exitUsage, err)
	})
	for _, sub := range []*cobra.Command{
		runCommand(d), sendCommand(d), waitCommand(d), getCommand(d), listCommand(d),
		logsCommand(d), filesCommand(d), answerCommand(d), cancelCommand(d), openCommand(d),
	} {
		// Cobra reads SilenceUsage from the executed command and the root
		// only, and a failed turn is not a usage mistake.
		sub.SilenceUsage = true
		root.AddCommand(sub)
	}
	return root
}

func usageError(format string, args ...any) error {
	return command.NewExitError(exitUsage, fmt.Errorf(format, args...))
}

// sessionArgs requires a session id first, then up to more further
// arguments (-1: any number, such as a message).
func sessionArgs(more int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return usageError("expected an agent session id (ags-…)")
		}
		if more >= 0 && len(args) > 1+more {
			return usageError("unexpected arguments after %s: %s", args[0], strings.Join(args[1:], " "))
		}
		return checkSessionID(args[0])
	}
}

// sessionIDs requires 1 to max arguments, each a session id.
func sessionIDs(max int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 0 || len(args) > max {
			return usageError("expected 1 to %d agent session ids (ags-…)", max)
		}
		for _, id := range args {
			if err := checkSessionID(id); err != nil {
				return err
			}
		}
		return nil
	}
}

func checkSessionID(id string) error {
	if !strings.HasPrefix(id, "ags-") || len(id) == len("ags-") {
		return usageError("%q is not an agent session id (ags-…)", id)
	}
	return nil
}

// invocation is what every subcommand needs once flags are parsed.
type invocation struct {
	ctx    context.Context
	api    *api
	format format
	out    io.Writer
	errOut io.Writer
}

// resolveFormat is the command's --output. The root pre-run resolves it with
// RENDER_OUTPUT and the terminal check, like every other command; the raw
// flag is the fallback when no pre-run ran.
func resolveFormat(c *cobra.Command) (format, error) {
	value, _ := c.Flags().GetString("output")
	if resolved := command.GetFormatFromContext(c.Context()); resolved != nil {
		value = string(*resolved)
	}
	return parseFormat(value)
}

func (d deps) start(c *cobra.Command) (*invocation, context.CancelFunc, error) {
	f, err := resolveFormat(c)
	if err != nil {
		return nil, nil, err
	}
	a, err := d.api()
	if err != nil {
		return nil, nil, err
	}
	parent := c.Context()
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	return &invocation{ctx: ctx, api: a, format: f, out: c.OutOrStdout(), errOut: c.ErrOrStderr()}, cancel, nil
}

// owner is the active workspace, which create, list and upload address.
func (d deps) owner() (string, error) {
	id, err := d.workspace()
	if err != nil {
		if errors.Is(err, config.ErrNoWorkspace) {
			return "", errors.New("no active workspace: run `bex workspace set`, or set BEX_WORKSPACE")
		}
		return "", err
	}
	return id, nil
}

// waitFlags are the shared --wait/--follow/--timeout/--reasoning flags.
type waitFlags struct {
	wait      bool
	follow    bool
	timeout   time.Duration
	reasoning bool
}

// bindTimeout binds --timeout, for every command that waits.
func (w *waitFlags) bindTimeout(c *cobra.Command) {
	c.Flags().DurationVar(&w.timeout, "timeout", 0, "stop waiting after this long (exit 124), e.g. 30m; 0 waits indefinitely")
}

// bindFollow binds --follow and what streaming takes, for logs.
func (w *waitFlags) bindFollow(c *cobra.Command, usage string) {
	c.Flags().BoolVar(&w.follow, "follow", false, usage)
	c.Flags().BoolVar(&w.reasoning, "reasoning", false, "also show the agent's reasoning when streaming")
	w.bindTimeout(c)
}

// bindTurn binds --wait and --follow, for the commands that start a turn.
func (w *waitFlags) bindTurn(c *cobra.Command) {
	c.Flags().BoolVar(&w.wait, "wait", false, "wait until the turn ends and exit by its state")
	w.bindFollow(c, "stream the transcript until the turn ends, then exit by its state")
}

func (w waitFlags) validate() error {
	if w.wait && w.follow {
		return usageError("use --wait or --follow, not both")
	}
	return nil
}

func (w waitFlags) blocking() bool { return w.wait || w.follow }

func (w waitFlags) context(ctx context.Context) (context.Context, context.CancelFunc) {
	if w.timeout > 0 {
		return context.WithTimeout(ctx, w.timeout)
	}
	return context.WithCancel(ctx)
}

// promptFlags read a task or message: the arguments, or --file (- is stdin).
type promptFlags struct{ file string }

func (p *promptFlags) bind(c *cobra.Command) {
	c.Flags().StringVarP(&p.file, "file", "f", "", "read the prompt from this file (- for stdin)")
}

func (p promptFlags) read(d deps, args []string) (string, error) {
	if p.file != "" {
		if len(args) > 0 {
			return "", usageError("give the prompt as arguments or with --file, not both")
		}
		var raw []byte
		var err error
		if p.file == "-" {
			raw, err = io.ReadAll(io.LimitReader(d.stdin, 1<<20))
		} else {
			raw, err = os.ReadFile(p.file)
		}
		if err != nil {
			return "", err
		}
		args = []string{string(raw)}
	}
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if prompt == "" {
		return "", usageError("the prompt is empty: pass it as arguments or with --file")
	}
	return prompt, nil
}

// readSchema loads --output-schema: a JSON object (ADR094 D5).
func readSchema(path string) (json.RawMessage, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, usageError("--output-schema %s is not a JSON object: %v", path, err)
	}
	return compactJSON(raw), nil
}

func uploadAll(inv *invocation, owner string, paths []string) ([]string, error) {
	ids := make([]string, 0, len(paths))
	for _, path := range paths {
		att, err := inv.api.upload(inv.ctx, owner, path)
		if err != nil {
			return nil, err
		}
		ids = append(ids, att.ID)
	}
	return ids, nil
}

// secretFlags attach ADR098 secrets to a turn. Values never come from the
// command line, where shell history and the process list would keep them:
// --secret names a stored workspace secret, and --secret-from-env reads an
// inline value from this process's environment.
type secretFlags struct {
	names   []string
	fromEnv []string
}

func (s *secretFlags) bind(c *cobra.Command) {
	c.Flags().StringArrayVar(&s.names, "secret", nil, "workspace agent secret to expose to the agent as an env var; repeatable")
	c.Flags().StringArrayVar(&s.fromEnv, "secret-from-env", nil, "NAME or NAME=VAR: send the local env var VAR (default NAME) as this session's inline secret NAME; repeatable")
}

func (s *secretFlags) resolve(d deps) (sessionSecrets, error) {
	out := sessionSecrets{SecretNames: s.names}
	for _, spec := range s.fromEnv {
		name, from, ok := strings.Cut(spec, "=")
		if !ok {
			from = name
		}
		if name == "" || from == "" {
			return sessionSecrets{}, usageError("--secret-from-env takes NAME or NAME=VAR, not %q", spec)
		}
		value, set := d.lookupEnv(from)
		if !set || value == "" {
			return sessionSecrets{}, usageError("--secret-from-env %s: the environment variable %s is not set", name, from)
		}
		out.Secrets = append(out.Secrets, inlineSecret{Name: name, Value: value})
	}
	return out, nil
}

// turnFlags are what a new turn carries, on run and send: the prompt, files
// to attach, an output schema and secrets.
type turnFlags struct {
	prompt  promptFlags
	attach  []string
	schema  string
	skills  []string
	secrets secretFlags
}

func (t *turnFlags) bind(c *cobra.Command) {
	c.Flags().StringArrayVar(&t.attach, "attach", nil, "file to attach to this turn; repeatable")
	c.Flags().StringArrayVar(&t.skills, "skill", nil, "NAME or NAME:ARGUMENTS: a workspace skill to use this turn; repeatable, up to 5")
	c.Flags().StringVar(&t.schema, "output-schema", "", "JSON Schema file this turn's result must satisfy")
	t.secrets.bind(c)
	t.prompt.bind(c)
}

// turnInput is a turn's parsed flags; the files are uploaded once the API
// client exists.
type turnInput struct {
	prompt  string
	schema  json.RawMessage
	skills  []skillInvocation
	secrets sessionSecrets
}

func (t *turnFlags) resolve(d deps, args []string) (turnInput, error) {
	var (
		in  turnInput
		err error
	)
	if in.prompt, err = t.prompt.read(d, args); err != nil {
		return turnInput{}, err
	}
	if in.schema, err = readSchema(t.schema); err != nil {
		return turnInput{}, err
	}
	if in.secrets, err = t.secrets.resolve(d); err != nil {
		return turnInput{}, err
	}
	for _, spec := range t.skills {
		name, arguments, _ := strings.Cut(spec, ":")
		if name = strings.TrimSpace(name); name == "" {
			return turnInput{}, usageError("--skill takes NAME or NAME:ARGUMENTS, not %q", spec)
		}
		in.skills = append(in.skills, skillInvocation{Name: name, Arguments: strings.TrimSpace(arguments)})
	}
	return in, nil
}

func runCommand(d deps) *cobra.Command {
	var (
		in      createRequest
		turn    turnFlags
		spend   float64
		key     string
		bypass  bool
		waiting waitFlags
	)
	c := &cobra.Command{
		Use:   "run [flags] <task…>",
		Short: "Start an agent session on a task",
		Long: "Starts a Cloud Agent session in the active workspace. Without --repo it\n" +
			"runs in an empty sandbox. --attach uploads files the agent finds under\n" +
			"/attachments; --output-schema asks for a result matching a JSON Schema\n" +
			"(read it from outcome.result). The create is idempotent: a retry after a\n" +
			"network error returns the same session, never a second one.",
		Example: "  bex agent run --repo acme/api \"Upgrade the Go toolchain to 1.26\"\n" +
			"  bex agent run --attach screenshot.png --wait -o json \"Fix the layout bug in the screenshot\"\n" +
			"  bex agent run --output-schema schema.json --max-spend 2 --follow -f task.md\n" +
			"  bex agent run --approval-mode risky --wait \"Deploy the fix to staging\"",
		RunE: func(c *cobra.Command, args []string) error {
			if err := waiting.validate(); err != nil {
				return err
			}
			t, err := turn.resolve(d, args)
			if err != nil {
				return err
			}
			in.AgentConfig.Task, in.OutputSchema, in.Skills, in.sessionSecrets = t.prompt, t.schema, t.skills, t.secrets
			if c.Flags().Changed("max-spend") {
				if spend <= 0 {
					return usageError("--max-spend must be more than 0 (USD)")
				}
				in.MaxSpendUSD = &spend
			}
			switch in.ApprovalMode {
			case "", approvalAutonomous, approvalRisky, approvalAll:
			default:
				return usageError("--approval-mode must be autonomous, risky or all, not %q", in.ApprovalMode)
			}
			if bypass && in.ApprovalMode != "" && in.ApprovalMode != approvalAutonomous {
				return usageError("--bypass-approval runs the session autonomous; drop --approval-mode %s", in.ApprovalMode)
			}
			in.BypassApproval = bypass
			if key == "" {
				key = d.newKey()
			}
			in.IdempotencyKey = key
			owner, err := d.owner()
			if err != nil {
				return err
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			if in.AttachmentIDs, err = uploadAll(inv, owner, turn.attach); err != nil {
				return err
			}
			created, raw, err := inv.api.create(inv.ctx, owner, in)
			if err != nil {
				return err
			}
			if !waiting.blocking() {
				return inv.show(created, raw)
			}
			_, _ = fmt.Fprintf(inv.errOut, "Started %s\n", created.ID)
			return inv.await(created, 1, waiting)
		},
	}
	f := c.Flags()
	f.StringVar(&in.Repo, "repo", "", "repository to work on (owner/name or URL); empty runs without one")
	f.StringVar(&in.Branch, "branch", "", "branch to start from (default: the repository's default branch)")
	f.StringVar(&in.AgentConfig.Agent, "agent", defaultAgent, "agent to run (see the dashboard composer for the workspace's agents)")
	f.StringVar(&in.AgentConfig.Model, "model", "", "model id (default: the workspace default)")
	f.StringVar(&in.Title, "title", "", "display title (default: the task's first line)")
	f.StringArrayVar(&in.Tags, "tag", nil, "label for filtering and cost attribution; repeatable")
	f.Float64Var(&spend, "max-spend", 0, "ceiling on this session's platform model spend, in USD")
	f.StringVar(&key, "idempotency-key", "", "reuse a key to make re-running this command safe (default: one per invocation)")
	f.StringVar(&in.ApprovalMode, "approval-mode", "", "when the agent pauses for approval: autonomous, risky or all (default: the workspace's mode)")
	f.BoolVar(&bypass, "bypass-approval", false, "run autonomous even if the workspace asks for approvals (when it allows bypass)")
	turn.bind(c)
	waiting.bindTurn(c)
	return c
}

func sendCommand(d deps) *cobra.Command {
	var (
		turn    turnFlags
		waiting waitFlags
	)
	c := &cobra.Command{
		Use:   "send <ags-id> [flags] <message…>",
		Short: "Send a follow-up message to an agent session",
		Long: "Starts the session's next turn with a message, the way a reply works in\n" +
			"the dashboard. The session must be steerable (its last turn ended).",
		Args: sessionArgs(-1),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			if err := waiting.validate(); err != nil {
				return err
			}
			t, err := turn.resolve(d, args[1:])
			if err != nil {
				return err
			}
			in := steerRequest{Prompt: t.prompt, OutputSchema: t.schema, Skills: t.skills, sessionSecrets: t.secrets}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			if len(turn.attach) > 0 {
				current, _, err := inv.api.session(inv.ctx, id)
				if err != nil {
					return err
				}
				if in.AttachmentIDs, err = uploadAll(inv, current.OwnerID, turn.attach); err != nil {
					return err
				}
			}
			steered, raw, err := inv.api.steer(inv.ctx, id, in)
			if err != nil {
				return err
			}
			if !waiting.blocking() {
				return inv.show(steered, raw)
			}
			return inv.await(steered, steered.Turns, waiting)
		},
	}
	turn.bind(c)
	waiting.bindTurn(c)
	return c
}

func waitCommand(d deps) *cobra.Command {
	var (
		waiting waitFlags
		first   bool
	)
	c := &cobra.Command{
		Use:   "wait <ags-id>…",
		Short: "Wait until agent sessions finish their turn",
		Long: "Waits until every session's turn has ended (--any: the first one), then\n" +
			"prints them and exits by state: with several sessions, a failure wins\n" +
			"over a cancel, which wins over needs-input.",
		Args: sessionIDs(20),
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			ctx, stop := waiting.context(inv.ctx)
			defer stop()
			result, err := inv.api.wait(ctx, args, first)
			if err != nil {
				return err
			}
			return inv.report(result, len(args) > 1, waiting.timeout)
		},
	}
	c.Flags().BoolVar(&first, "any", false, "return when the first session settles")
	waiting.bindTimeout(c)
	return c
}

func getCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "get <ags-id>",
		Short: "Show an agent session and its outcome",
		Args:  sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			s, raw, err := inv.api.session(inv.ctx, args[0])
			if err != nil {
				return err
			}
			return inv.show(s, raw)
		},
	}
}

func listCommand(d deps) *cobra.Command {
	var (
		states   []string
		tag      string
		repo     string
		archived string
		limit    int
		cursor   string
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's agent sessions",
		Long: "Lists the active workspace's unarchived sessions, newest first. Page with\n" +
			"--cursor set to the last id of the previous page.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			for _, state := range states {
				if !slices.Contains(sessionStates, state) {
					return usageError("--state must be one of %s, not %q", strings.Join(sessionStates, ", "), state)
				}
			}
			owner, err := d.owner()
			if err != nil {
				return err
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			query := url.Values{"ownerId": {owner}, "limit": {strconv.Itoa(limit)}}
			for _, state := range states {
				query.Add("state", state)
			}
			for key, value := range map[string]string{"tag": tag, "repo": repo, "archived": archived, "cursor": cursor} {
				if value != "" {
					query.Set(key, value)
				}
			}
			raw, err := inv.api.getJSON(inv.ctx, query, "agent-sessions")
			if err != nil {
				return fmt.Errorf("list agent sessions: %w", err)
			}
			if inv.format != formatText {
				return writeRaw(inv.out, inv.format, raw)
			}
			var sessions []session
			if err := json.Unmarshal(raw, &sessions); err != nil {
				return fmt.Errorf("list agent sessions: %w", err)
			}
			writeSessionList(inv.out, sessions)
			return nil
		},
	}
	f := c.Flags()
	f.StringArrayVar(&states, "state", nil, "only sessions in this state ("+strings.Join(sessionStates, ", ")+"); repeatable")
	f.StringVar(&tag, "tag", "", "only sessions with this tag")
	f.StringVar(&repo, "repo", "", "only sessions on this repository")
	f.StringVar(&archived, "archived", "", "true for archived sessions only, all for both")
	f.IntVar(&limit, "limit", 20, "page size (1-200)")
	f.StringVar(&cursor, "cursor", "", "the last session id of the previous page")
	return c
}

func logsCommand(d deps) *cobra.Command {
	var (
		waiting waitFlags
		turn    int
	)
	c := &cobra.Command{
		Use:   "logs <ags-id>",
		Short: "Print an agent session's transcript",
		Long: "Prints the transcript: the agent's messages and a line per tool call.\n" +
			"--follow keeps streaming until the current turn ends and exits by its\n" +
			"state. -o json prints one transcript row per line.",
		Args: sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			if f, err := resolveFormat(c); err != nil || f == formatYAML {
				if err != nil {
					return err
				}
				return usageError("logs prints text or JSON lines (-o json)")
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			p := newPartPrinter(inv.out, inv.format == formatJSON, waiting.reasoning)
			if waiting.follow {
				ctx, stop := waiting.context(inv.ctx)
				defer stop()
				result, err := inv.api.follow(ctx, args[0], turn, p)
				if err != nil {
					return err
				}
				return stateExit(result, waiting.timeout)
			}
			afterSeq := int64(-1)
			if _, err := inv.api.drain(inv.ctx, args[0], &afterSeq, turn, p); err != nil {
				return err
			}
			p.flush()
			return nil
		},
	}
	c.Flags().IntVar(&turn, "turn", 0, "start at this turn")
	waiting.bindFollow(c, "keep streaming until the turn ends, then exit by its state")
	return c
}

// Approval modes (ADR100 D3).
const (
	approvalAutonomous = "autonomous"
	approvalRisky      = "risky"
	approvalAll        = "all"
)

func answerCommand(d deps) *cobra.Command {
	var (
		allow, deny bool
		scope, text string
	)
	c := &cobra.Command{
		Use:   "answer <ags-id> (--allow [--scope session] | --deny | --text <answer>)",
		Short: "Answer the approval or question an agent session is waiting on",
		Long: "A session in state needs_input waits for a person (approval mode). Allow or\n" +
			"deny the action it asked to run, or answer its question; the turn then\n" +
			"continues. --scope session also allows the same kind of action for the rest\n" +
			"of the session.",
		Example: "  bex agent answer ags-… --allow\n" +
			"  bex agent answer ags-… --text \"Use the eu region\"",
		Args: sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			given := 0
			for _, set := range []bool{allow, deny, c.Flags().Changed("text")} {
				if set {
					given++
				}
			}
			if given != 1 {
				return usageError("pass exactly one of --allow, --deny or --text")
			}
			if scope != "" && !allow {
				return usageError("--scope applies only to --allow")
			}
			if scope != "" && scope != "once" && scope != "session" {
				return usageError("--scope must be once or session, not %q", scope)
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			s, _, err := inv.api.session(inv.ctx, args[0])
			if err != nil {
				return err
			}
			if s.Outcome == nil || s.Outcome.PendingDecision == nil {
				return fmt.Errorf("agent session %s is not waiting on a decision (state %s)", s.ID, s.State)
			}
			pending := s.Outcome.PendingDecision
			in := answerRequest{Scope: scope}
			switch {
			case pending.Kind == "question" && !c.Flags().Changed("text"):
				return usageError("agent session %s asked a question: answer it with --text", s.ID)
			case pending.Kind != "question" && c.Flags().Changed("text"):
				return usageError("agent session %s asks for approval: use --allow or --deny", s.ID)
			case c.Flags().Changed("text"):
				in.Answer = &text
			default:
				in.Allow = &allow
			}
			answered, raw, err := inv.api.answer(inv.ctx, s.ID, pending.ID, in)
			if err != nil {
				return err
			}
			if inv.format != formatText {
				return writeRaw(inv.out, inv.format, raw)
			}
			_, err = fmt.Fprintf(inv.out, "%s %s: %s\n", s.ID, answered.ID, answered.State)
			return err
		},
	}
	f := c.Flags()
	f.BoolVar(&allow, "allow", false, "allow the action the agent asked to run")
	f.BoolVar(&deny, "deny", false, "refuse the action")
	f.StringVar(&scope, "scope", "", "with --allow: once (default) or session")
	f.StringVar(&text, "text", "", "the answer to the agent's question")
	return c
}

func cancelCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <ags-id>",
		Short: "Cancel an agent session",
		Long:  "Stops the running turn and closes the session; it accepts no further messages.",
		Args:  sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			s, raw, err := inv.api.cancel(inv.ctx, args[0])
			if err != nil {
				return err
			}
			return inv.show(s, raw)
		},
	}
}

func openCommand(d deps) *cobra.Command {
	var inDashboard bool
	c := &cobra.Command{
		Use:   "open <ags-id>",
		Short: "Open an agent session in bex desktop or the dashboard",
		Long: "Opens the session's running sandbox in bex desktop over SSH (like\n" +
			"`bex edit ags-…`) when the desktop is installed and the sandbox is up;\n" +
			"otherwise opens, and prints, the session's dashboard page.",
		Args: sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			id := args[0]
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			s, _, err := inv.api.session(inv.ctx, id)
			if err != nil {
				return err
			}
			if !inDashboard && s.SSHAddress != "" && d.canDesktop() {
				startedAt := time.Now()
				return d.openDesktop(inv.ctx, id, s.SSHAddress, func() {
					telemetry.EmitLaunch(c.CommandPath(), startedAt, c.ErrOrStderr())
				})
			}
			page := strings.TrimSuffix(d.dashboard(), "/") + "/agents/" + url.PathEscape(id)
			_, _ = fmt.Fprintln(inv.out, page)
			if d.interactive() {
				_ = d.openURL(page)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&inDashboard, "dashboard", false, "open the dashboard page even when bex desktop could open the sandbox")
	return c
}

func (inv *invocation) show(s session, raw []byte) error {
	if inv.format != formatText {
		return writeRaw(inv.out, inv.format, raw)
	}
	writeSession(inv.out, s)
	return nil
}

// await waits for (or follows) the turn s just started, prints where it
// ended and exits by its state.
func (inv *invocation) await(s session, fromTurn int, w waitFlags) error {
	ctx, stop := w.context(inv.ctx)
	defer stop()
	var (
		result waitResult
		err    error
	)
	if w.follow {
		// Structured output keeps stdout to the API's answer.
		stream := inv.out
		if inv.format != formatText {
			stream = inv.errOut
		}
		result, err = inv.api.follow(ctx, s.ID, fromTurn, newPartPrinter(stream, false, w.reasoning))
		if err == nil && inv.format == formatText && len(result.sessions) > 0 {
			_, _ = fmt.Fprintln(inv.out)
		}
	} else {
		result, err = inv.api.wait(ctx, []string{s.ID}, false)
	}
	if err != nil {
		return err
	}
	return inv.report(result, false, w.timeout)
}

// report prints a wait's sessions and returns the exit their states call for.
func (inv *invocation) report(result waitResult, many bool, timeout time.Duration) error {
	switch {
	case inv.format != formatText && many:
		if err := writeRaw(inv.out, inv.format, rawArray(result.raws)); err != nil {
			return err
		}
	case inv.format != formatText:
		if len(result.raws) > 0 {
			if err := writeRaw(inv.out, inv.format, result.raws[0]); err != nil {
				return err
			}
		}
	default:
		for i, s := range result.sessions {
			if i > 0 {
				_, _ = fmt.Fprintln(inv.out)
			}
			writeSession(inv.out, s)
		}
	}
	return stateExit(result, timeout)
}

// stateExit maps where the waited sessions stand to the exit code, with a
// one-line reason on stderr through the returned error.
func stateExit(result waitResult, timeout time.Duration) error {
	if result.timedOut {
		ids := make([]string, 0, len(result.sessions))
		for _, s := range result.sessions {
			if !settled(s) {
				ids = append(ids, s.ID)
			}
		}
		what := "the session is"
		if len(ids) > 0 {
			what = strings.Join(ids, ", ") + " still"
		}
		return command.NewExitError(exitTimeout, fmt.Errorf("%s working after %s; `bex agent wait` resumes waiting", what, timeout))
	}
	var worst *session
	rank := map[string]int{stateFailed: 3, stateCanceled: 2, stateNeedsInput: 1}
	for i := range result.sessions {
		s := &result.sessions[i]
		if rank[s.State] > 0 && (worst == nil || rank[s.State] > rank[worst.State]) {
			worst = s
		}
	}
	if worst == nil {
		return nil
	}
	switch worst.State {
	case stateFailed:
		reason := firstNonEmpty(worst.FailureReasonCode, "failed")
		if worst.FailureReason != "" {
			reason += ": " + worst.FailureReason
		}
		return command.NewExitError(exitFailed, fmt.Errorf("agent session %s failed: %s", worst.ID, reason))
	case stateCanceled:
		return command.NewExitError(exitCanceled, fmt.Errorf("agent session %s was canceled", worst.ID))
	default:
		what := "is waiting on a decision"
		if worst.Outcome != nil && worst.Outcome.PendingDecision != nil {
			what = "asks: " + worst.Outcome.PendingDecision.Title
		}
		return command.NewExitError(exitNeedsInput, fmt.Errorf("agent session %s %s; answer with `bex agent answer %s …`", worst.ID, what, worst.ID))
	}
}

// newIdempotencyKey is one key per `bex agent run` (ADR094 D4: 1-128
// printable ASCII).
func newIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "bex-cli-" + hex.EncodeToString(b)
}
