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
	"net/http"
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
	"github.com/spf13/pflag"
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
	exitNotStarted = 6 // an automation run started no session
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
	// localZone is this machine's IANA time zone, "" when unknown: an
	// automation schedule defaults to it, as the dashboard's to the browser's.
	localZone func() string
	// readHidden prompts on the terminal and reads a secret without echoing
	// it. It answers nil when stdin is not a terminal; the value is then read
	// from stdin.
	readHidden func(prompt string) ([]byte, error)
	// workdir is where handoff and pickup find the git checkout.
	workdir func() (string, error)
	// execSSH replaces the process with the system ssh and args, after
	// beforeExec (the launch telemetry).
	execSSH func(args []string, beforeExec func()) error
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
		localZone:   machineZone,
		readHidden:  terminalSecret,
		workdir:     os.Getwd,
		execSSH:     execSystemSSH,
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
			"  5    reserved: needs_input, which sessions never enter\n" +
			"  6    an automation run started no session (automations run)\n" +
			"  124  --timeout elapsed while the session was still working\n" +
			"  130  interrupted (Ctrl-C or SIGTERM)\n\n" +
			"On a terminal, a command whose one argument is a session may leave it\n" +
			"out and pick one of your recent sessions.\n\n" +
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
	pick := func(c *cobra.Command) *cobra.Command { return d.withPicker(sessionsKind, c) }
	completeFirst := func(c *cobra.Command) *cobra.Command { return d.withCompletion(sessionsKind, 1, c) }
	completeAll := func(c *cobra.Command) *cobra.Command { return d.withCompletion(sessionsKind, -1, c) }
	for _, sub := range []*cobra.Command{
		runCommand(d), completeFirst(sendCommand(d)), completeAll(waitCommand(d)), pick(getCommand(d)), listCommand(d),
		pick(logsCommand(d)), pick(filesCommand(d)), pick(cancelCommand(d)), pick(openCommand(d)),
		completeAll(archiveCommand(d, "archive", "Archived", "Archive agent sessions (hidden from the default list, read-only)")),
		completeAll(archiveCommand(d, "unarchive", "Unarchived", "Unarchive agent sessions")),
		pick(deleteCommand(d)), completeFirst(tagsCommand(d)), pick(treeCommand(d)), pick(costCommand(d)),
		completeFirst(chatCommand(d)), handoffCommand(d), pick(pickupCommand(d)), completeFirst(sshCommand(d)),
		automationsCommand(d), secretsCommand(d), skillsCommand(d), envCommand(d), modelsCommand(d),
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

// sessionArgs requires a session first, then up to more further arguments
// (-1: any number, such as a message). Like sessionIDs, it rewrites a
// dashboard URL to its id in place (cobra hands RunE the same slice), so
// RunE always sees an ags- id.
func sessionArgs(more int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) (err error) {
		if len(args) == 0 {
			return usageError("expected an agent session id (ags-…) or its dashboard URL")
		}
		if more >= 0 && len(args) > 1+more {
			return usageError("unexpected arguments after %s: %s", args[0], strings.Join(args[1:], " "))
		}
		args[0], err = sessionRef(args[0])
		return err
	}
}

// sessionIDs requires 1 to max arguments, each a session.
func sessionIDs(max int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) (err error) {
		if len(args) == 0 || len(args) > max {
			return usageError("expected 1 to %d agent sessions (ags-… ids or dashboard URLs)", max)
		}
		for i := range args {
			if args[i], err = sessionRef(args[i]); err != nil {
				return err
			}
		}
		return nil
	}
}

// refArgs requires one reference to what (an id or a name) first, then up to
// more further arguments (-1: any number, such as a new task).
func refArgs(what string, more int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
			return usageError("expected %s", what)
		}
		if more >= 0 && len(args) > 1+more {
			return usageError("unexpected arguments after %s: %s", args[0], strings.Join(args[1:], " "))
		}
		return nil
	}
}

// dashboardAgents is the dashboard's sessions path segment: /agents/ags-….
const dashboardAgents = "agents"

// isSessionID is an ags- id: the prefix, then the lowercase letters and
// digits of an xid (ADR020).
func isSessionID(s string) bool {
	rest, ok := strings.CutPrefix(s, "ags-")
	return ok && rest != "" && strings.Trim(rest, "abcdefghijklmnopqrstuvwxyz0123456789") == ""
}

// sessionRef is the session an argument names: an ags- id, or a dashboard
// session URL (…/agents/ags-…, on any dashboard origin) as Devin's CLI
// takes a session URL (ADR106 D3).
func sessionRef(arg string) (string, error) {
	if isSessionID(arg) {
		return arg, nil
	}
	if u, err := url.Parse(arg); err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == dashboardAgents && isSessionID(parts[i+1]) {
				return parts[i+1], nil
			}
		}
		return "", usageError("%q is not an agent session URL (…/agents/ags-…)", arg)
	}
	return "", usageError("%q is not an agent session id (ags-…) or URL", arg)
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

func (p *promptFlags) bind(f *pflag.FlagSet) {
	f.StringVarP(&p.file, "file", "f", "", "read the prompt from this file (- for stdin)")
}

// maxPromptBytes bounds a prompt read from a file or stdin.
const maxPromptBytes = 1 << 20

func (p promptFlags) read(d deps, args []string) (string, error) {
	if p.file != "" {
		if len(args) > 0 {
			return "", usageError("give the prompt as arguments or with --file, not both")
		}
		raw, err := readPathOrStdin(d, p.file, maxPromptBytes)
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

// readPathOrStdin reads path ("-": stdin), refusing more than limit bytes
// rather than acting on a silently truncated copy.
func readPathOrStdin(d deps, path string, limit int64) ([]byte, error) {
	r := d.stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, usageError("%s is larger than %d KiB", firstNonEmpty(strings.TrimPrefix(path, "-"), "stdin"), limit>>10)
	}
	return raw, nil
}

// terminalSecret prompts on stderr and reads a line from a terminal stdin
// without echo; nil when stdin is not a terminal.
func terminalSecret(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s (input hidden): ", prompt)
	defer func() { _, _ = fmt.Fprintln(os.Stderr) }()
	return term.ReadPassword(fd)
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

func (s *secretFlags) bind(f *pflag.FlagSet) {
	f.StringArrayVar(&s.names, "secret", nil, "workspace agent secret to expose to the agent as an env var; repeatable")
	f.StringArrayVar(&s.fromEnv, "secret-from-env", nil, "NAME or NAME=VAR: send the local env var VAR (default NAME) as this session's inline secret NAME; repeatable")
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
		value, err := envValue(d, "--secret-from-env "+name, from)
		if err != nil {
			return sessionSecrets{}, err
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

func (t *turnFlags) bind(f *pflag.FlagSet) {
	f.StringArrayVar(&t.attach, "attach", nil, "file to attach to this turn; repeatable")
	f.StringArrayVar(&t.skills, "skill", nil, "NAME or NAME:ARGUMENTS: a workspace skill to use this turn; repeatable, up to 5")
	f.StringVar(&t.schema, "output-schema", "", "JSON Schema file this turn's result must satisfy")
	t.secrets.bind(f)
	t.prompt.bind(f)
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
	if in.skills, err = parseSkills(t.skills); err != nil {
		return turnInput{}, err
	}
	return in, nil
}

// parseSkills reads --skill NAME or NAME:ARGUMENTS values.
func parseSkills(specs []string) ([]skillInvocation, error) {
	var out []skillInvocation
	for _, spec := range specs {
		name, arguments, _ := strings.Cut(spec, ":")
		if name = strings.TrimSpace(name); name == "" {
			return nil, usageError("--skill takes NAME or NAME:ARGUMENTS, not %q", spec)
		}
		out = append(out, skillInvocation{Name: name, Arguments: strings.TrimSpace(arguments)})
	}
	return out, nil
}

// sessionFlags are the session settings `run` and an automation's template
// share, with the same names and help.
type sessionFlags struct {
	repo, branch, agent, model, title string
	tags                              []string
	spend                             float64
}

func (s *sessionFlags) bind(f *pflag.FlagSet) {
	f.StringVar(&s.repo, "repo", "", "repository to work on (owner/name or URL); empty runs without one")
	f.StringVar(&s.branch, "branch", "", "branch to start from (default: the repository's default branch)")
	f.StringVar(&s.agent, "agent", defaultAgent, "agent to run (see the dashboard composer for the workspace's agents)")
	f.StringVar(&s.model, "model", "", "model id (default: the workspace default)")
	f.StringVar(&s.title, "title", "", "display title (default: the task's first line)")
	f.StringArrayVar(&s.tags, "tag", nil, "label for filtering and cost attribution; repeatable")
	f.Float64Var(&s.spend, "max-spend", 0, "ceiling on this session's platform model spend, in USD")
}

// maxSpend is --max-spend when it was given (fs holds the bound flags).
func (s *sessionFlags) maxSpend(fs *pflag.FlagSet) (*float64, error) {
	if !fs.Changed("max-spend") {
		return nil, nil
	}
	if s.spend <= 0 {
		return nil, usageError("--max-spend must be more than 0 (USD)")
	}
	spend := s.spend
	return &spend, nil
}

// createFlags are what starting a session takes, on run and chat.
type createFlags struct {
	session sessionFlags
	turn    turnFlags
	key     string
	parent  string
	fs      *pflag.FlagSet
}

func (f *createFlags) bind(c *cobra.Command) {
	f.fs = pflag.NewFlagSet("create", pflag.ContinueOnError)
	f.session.bind(f.fs)
	f.fs.StringVar(&f.key, "idempotency-key", "", "reuse a key to make re-running this command safe (default: one per invocation)")
	f.fs.StringVar(&f.parent, "parent", "", "start the session as a child of this one (ags- id or URL); its spend counts against the parent's ceiling")
	f.turn.bind(f.fs)
	c.Flags().AddFlagSet(f.fs)
}

// given reports whether any create flag was set.
func (f *createFlags) given() bool { return anyChanged(f.fs) }

// request is the session the flags and task describe, checked and with the
// prompt read, before anything is sent; files are uploaded by createSession.
func (f *createFlags) request(d deps, task []string) (createRequest, error) {
	t, err := f.turn.resolve(d, task)
	if err != nil {
		return createRequest{}, err
	}
	in := createRequest{
		Repo: f.session.repo, Branch: f.session.branch, Title: f.session.title, Tags: f.session.tags,
		AgentConfig:  agentConfig{Agent: f.session.agent, Model: f.session.model, Task: t.prompt},
		OutputSchema: t.schema, Skills: t.skills, sessionSecrets: t.secrets,
		IdempotencyKey: firstNonEmpty(f.key, d.newKey()),
	}
	if in.MaxSpendUSD, err = f.session.maxSpend(f.fs); err != nil {
		return createRequest{}, err
	}
	if f.parent != "" {
		if in.ParentSessionID, err = sessionRef(f.parent); err != nil {
			return createRequest{}, err
		}
	}
	return in, nil
}

// create starts the session the flags and task describe.
func (f *createFlags) create(d deps, a workspaceAPI, task []string) (session, []byte, error) {
	in, err := f.request(d, task)
	if err != nil {
		return session{}, nil, err
	}
	return a.createSession(in, f.turn.attach)
}

// createSession uploads attach, then starts in; the create is idempotent
// across retries (ADR094 D4).
func (a workspaceAPI) createSession(in createRequest, attach []string) (session, []byte, error) {
	var err error
	if in.AttachmentIDs, err = uploadAll(a.invocation, a.owner, attach); err != nil {
		return session{}, nil, err
	}
	return a.api.create(a.ctx, a.owner, in)
}

// validateChat refuses --chat where it cannot work.
func validateChat(d deps, chat bool, w waitFlags) error {
	if chat && (w.blocking() || !d.interactive()) {
		return usageError("--chat needs a terminal and no --wait or --follow")
	}
	return nil
}

func runCommand(d deps) *cobra.Command {
	var (
		flags   createFlags
		waiting waitFlags
		chat    bool
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
			"  bex agent run --output-schema schema.json --max-spend 2 --follow -f task.md",
		RunE: func(c *cobra.Command, args []string) error {
			if err := waiting.validate(); err != nil {
				return err
			}
			if err := validateChat(d, chat, waiting); err != nil {
				return err
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			created, raw, err := flags.create(d, a, args)
			if err != nil {
				return err
			}
			switch {
			case chat:
				return d.chat(a, created.ID, chatOptions{reasoning: waiting.reasoning})
			case !waiting.blocking():
				return a.show(created, raw)
			}
			_, _ = fmt.Fprintf(a.errOut, "Started %s\n", created.ID)
			return a.await(created, 1, waiting)
		},
	}
	flags.bind(c)
	waiting.bindTurn(c)
	c.Flags().BoolVar(&chat, "chat", false, "attach interactively after starting (a terminal only)")
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
			"the dashboard. While a turn is running the server queues the message\n" +
			"instead (at most 10 per session) and runs it when the turn ends; --wait and\n" +
			"--follow then return at once, since the message's own turn has not started.",
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
			steered, raw, err := inv.api.steer(inv.ctx, id, in, d.newKey())
			if err != nil {
				return err
			}
			if steered.Queued != nil {
				note := fmt.Sprintf("Queued (position %d): it runs when the session's running turn ends.", steered.Queued.Position)
				if waiting.blocking() {
					note += " Not waiting: a queued message runs as its own turn."
				}
				_, _ = fmt.Fprintln(inv.errOut, note)
				return inv.show(steered, raw)
			}
			if !waiting.blocking() {
				return inv.show(steered, raw)
			}
			return inv.await(steered, steered.Turns, waiting)
		},
	}
	turn.bind(c.Flags())
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

// defaultListLimit is `bex agent list`'s page size.
const defaultListLimit = 20

// sessionFilters are `bex agent list`'s filters, each one of the API's.
type sessionFilters struct {
	states, phases, createdBy               []string
	tag, repo, archived, automation, parent string
	since, until, cursor                    string
	mine                                    bool
	limit                                   int
}

func (f *sessionFilters) bind(c *cobra.Command) {
	fs := c.Flags()
	fs.StringArrayVar(&f.states, "state", nil, "only sessions in this state ("+strings.Join(sessionStates, ", ")+"); repeatable")
	fs.StringArrayVar(&f.phases, "phase", nil, "only sessions in this lifecycle phase (finer than --state); repeatable")
	fs.StringVar(&f.tag, "tag", "", "only sessions with this tag")
	fs.StringVar(&f.repo, "repo", "", "only sessions on this repository")
	fs.StringVar(&f.archived, "archived", "", "true for archived sessions only, all for both")
	fs.BoolVar(&f.mine, "mine", false, "only sessions I started")
	fs.StringArrayVar(&f.createdBy, "created-by", nil, "only sessions this member started; repeatable, up to 20")
	fs.StringVar(&f.automation, "automation", "", "only sessions this automation started (an aut- id or a name)")
	fs.StringVar(&f.parent, "parent", "", "only the direct children of this session")
	fs.StringVar(&f.since, "since", "", "only sessions created at or after this time (RFC 3339, or relative: 5m, 24h, 7d)")
	fs.StringVar(&f.until, "until", "", "only sessions created before this time (RFC 3339, or relative)")
	fs.IntVar(&f.limit, "limit", defaultListLimit, "page size (1-200)")
	fs.StringVar(&f.cursor, "cursor", "", "the last session id of the previous page")
}

// query is the API query the filters ask for, but the automation, which a
// name makes a lookup (see workspaceAPI.automationID).
func (f *sessionFilters) query(now time.Time) (url.Values, error) {
	for _, state := range f.states {
		if !slices.Contains(sessionStates, state) {
			return nil, usageError("--state must be one of %s, not %q", strings.Join(sessionStates, ", "), state)
		}
	}
	if len(f.createdBy) > 20 {
		return nil, usageError("--created-by takes at most 20 members")
	}
	query := url.Values{"limit": {strconv.Itoa(f.limit)}, "state": f.states, "phase": f.phases, "createdBy": f.createdBy}
	if f.mine {
		query.Set("mine", "true")
	}
	if f.parent != "" {
		id, err := sessionRef(f.parent)
		if err != nil {
			return nil, err
		}
		query.Set("parentSessionId", id)
	}
	for key, value := range map[string]string{"createdAfter": f.since, "createdBefore": f.until} {
		if value == "" {
			continue
		}
		at, err := parseTimeOrAge(value, now)
		if err != nil {
			return nil, err
		}
		query.Set(key, at.UTC().Format(time.RFC3339))
	}
	for key, value := range map[string]string{"tag": f.tag, "repo": f.repo, "archived": f.archived, "cursor": f.cursor} {
		if value != "" {
			query.Set(key, value)
		}
	}
	return query, nil
}

func listCommand(d deps) *cobra.Command {
	var filters sessionFilters
	c := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's agent sessions",
		Long: "Lists the active workspace's unarchived sessions, newest first. Page with\n" +
			"--cursor set to the last id of the previous page. --since and --until take\n" +
			"RFC 3339 or a relative time such as 24h or 7d, as `bex logs` does.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			query, err := filters.query(time.Now())
			if err != nil {
				return err
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			if filters.automation != "" {
				id, err := a.automationID(filters.automation)
				if err != nil {
					return err
				}
				query.Set("automationId", id)
			}
			raw, err := a.call(http.MethodGet, query, nil, "agent-sessions")
			if err != nil {
				return fmt.Errorf("list agent sessions: %w", err)
			}
			return showAs(a.invocation, raw, "list agent sessions", func(w io.Writer, sessions []session) error {
				writeSessionList(w, sessions)
				return nil
			})
		},
	}
	filters.bind(c)
	return c
}

// parseTimeOrAge reads a time the way upstream's `logs --start` does: RFC
// 3339, or relative to now (5m, 24h, 7d).
func parseTimeOrAge(value string, now time.Time) (time.Time, error) {
	t, err := command.ParseTime(now, &value)
	if err != nil || t == nil || t.T == nil {
		return time.Time{}, usageError("%q is not a time: use RFC 3339 or a relative time such as 24h or 7d", value)
	}
	return *t.T, nil
}

// automationID is ref as an aut- id: a name is looked up in the workspace,
// since the session filter takes ids.
func (a workspaceAPI) automationID(ref string) (string, error) {
	if strings.HasPrefix(ref, "aut-") {
		return ref, nil
	}
	raw, err := a.getAutomation(ref)
	if err != nil {
		return "", err
	}
	var found automation
	if err := json.Unmarshal(raw, &found); err != nil {
		return "", fmt.Errorf("get automation %s: %w", ref, err)
	}
	return found.ID, nil
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
				result, err := inv.api.followLive(ctx, args[0], turn, p)
				if err != nil {
					return err
				}
				inv.warnTail(args[0], result)
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
			page := strings.TrimSuffix(d.dashboard(), "/") + "/" + dashboardAgents + "/" + url.PathEscape(id)
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
		result, err = inv.api.followLive(ctx, s.ID, fromTurn, newPartPrinter(stream, false, w.reasoning))
		if err == nil {
			inv.warnTail(s.ID, result)
			if inv.format == formatText && len(result.sessions) > 0 {
				_, _ = fmt.Fprintln(inv.out)
			}
		}
	} else {
		result, err = inv.api.wait(ctx, []string{s.ID}, false)
	}
	if err != nil {
		return err
	}
	return inv.report(result, false, w.timeout)
}

// warnTail says on stderr that a follow could not read the end of id's
// transcript.
func (inv *invocation) warnTail(id string, result waitResult) {
	if result.tailErr != nil {
		_, _ = fmt.Fprintf(inv.errOut, "Warning: the end of the transcript may be missing: %v. `bex agent logs %s` prints the whole transcript.\n", result.tailErr, id)
	}
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
		return timeoutExit(result.sessions, timeout)
	}
	worst := worstSession(result.sessions)
	switch {
	case worst == nil:
		return nil
	case worst.State == stateFailed:
		return failedSessionExit(worst)
	case worst.State == stateCanceled:
		return command.NewExitError(exitCanceled, fmt.Errorf("agent session %s was canceled", worst.ID))
	default:
		return command.NewExitError(exitNeedsInput, fmt.Errorf("agent session %s needs input: reply with `bex agent send %s …`", worst.ID, worst.ID))
	}
}

// worstSession is the highest-severity unsettled outcome among the sessions,
// or nil when every one ended cleanly. A failed turn outranks a canceled one,
// which outranks one waiting for input; ties keep the first session seen.
func worstSession(sessions []session) *session {
	rank := map[string]int{stateFailed: 3, stateCanceled: 2, stateNeedsInput: 1}
	var worst *session
	for i := range sessions {
		s := &sessions[i]
		if rank[s.State] > 0 && (worst == nil || rank[s.State] > rank[worst.State]) {
			worst = s
		}
	}
	return worst
}

func timeoutExit(sessions []session, timeout time.Duration) error {
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
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

func failedSessionExit(s *session) error {
	reason := firstNonEmpty(s.FailureReasonCode, "failed")
	if s.FailureReason != "" {
		reason += ": " + s.FailureReason
	}
	return command.NewExitError(exitFailed, fmt.Errorf("agent session %s failed: %s", s.ID, reason))
}

// newIdempotencyKey is one key per `bex agent run` (ADR094 D4: 1-128
// printable ASCII).
func newIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "bex-cli-" + hex.EncodeToString(b)
}
