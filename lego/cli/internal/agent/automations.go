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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// `bex agent automations` is ADR106 D7: a thin client of the ADR101 D8
// automation routes. Every rule (schedule math, template validation, the
// creator check, spend) stays in bex-api; the commands shape requests and
// render answers. Writes work on the API's raw JSON, so a field this CLI
// does not model survives an update.

// runStarted is the only run outcome with a session.
const runStarted = "started"

// The automation error codes the CLI adds a hint to (ADR101 D8).
const (
	codeAutomationConflict   = "AGENT_AUTOMATION_CONFLICT"
	codeAutomationNotCreator = "AGENT_AUTOMATION_NOT_CREATOR"
)

// automation is an automation as the API shows it, decoded for text output.
type automation struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Enabled             bool                `json:"enabled"`
	CreatedBy           string              `json:"createdBy"`
	Triggers            []automationTrigger `json:"triggers"`
	Template            automationTemplate  `json:"template"`
	NextFireAt          *time.Time          `json:"nextFireAt"`
	LastFireAt          *time.Time          `json:"lastFireAt"`
	LastSessionID       string              `json:"lastSessionId"`
	PausedReason        string              `json:"pausedReason"`
	ConsecutiveFailures int                 `json:"consecutiveFailures"`
	Version             int                 `json:"version"`
}

type automationTrigger struct {
	Type     string              `json:"type"`
	Schedule *automationSchedule `json:"schedule,omitempty"`
}

type automationSchedule struct {
	Frequency string     `json:"frequency"`
	Minute    *int       `json:"minute,omitempty"`
	At        string     `json:"at,omitempty"`
	Weekdays  []string   `json:"weekdays,omitempty"`
	RunAt     *time.Time `json:"runAt,omitempty"`
	Cron      string     `json:"cron,omitempty"`
	Timezone  string     `json:"timezone"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
}

type automationTemplate struct {
	Task        string      `json:"task"`
	Repo        string      `json:"repo"`
	Branch      string      `json:"branch"`
	AgentConfig agentConfig `json:"agentConfig"`
	MaxSpendUSD *float64    `json:"maxSpendUSD"`
	SecretNames []string    `json:"secretNames"`
}

type automationList struct {
	Automations []automation `json:"automations"`
	NextCursor  string       `json:"nextCursor"`
}

type automationRun struct {
	AutomationID string    `json:"automationId"`
	FireAt       time.Time `json:"fireAt"`
	Outcome      string    `json:"outcome"`
	SessionID    string    `json:"sessionId"`
	MissedTicks  int       `json:"missedTicks"`
	Error        string    `json:"error"`
}

type automationRunList struct {
	Runs       []automationRun `json:"runs"`
	NextCursor string          `json:"nextCursor"`
}

func automationsCommand(d deps) *cobra.Command {
	c := &cobra.Command{
		Use:     "automations",
		Aliases: []string{"automation", "aut"},
		Short:   "Manage Agent Automations: agent sessions started on a schedule",
		Long: "An automation is a schedule plus the session it starts (ADR101). Each\n" +
			"fire is an ordinary agent session in the active workspace, run as the\n" +
			"member who created the automation. <automation> is an aut- id or a name.\n\n" +
			"`apply -f` keeps automations in files (the REST body as YAML or JSON) and\n" +
			"`export` prints them in that shape.",
		Example: "  bex agent automations create --name error-triage --cron \"0 9 * * 1-5\" --timezone Europe/Berlin \\\n" +
			"    --repo acme/api --max-spend 3 \"Triage yesterday's new errors\"\n" +
			"  bex agent automations apply -f .bex/automations/ --dry-run\n" +
			"  bex agent automations run error-triage --wait --timeout 45m",
	}
	for _, sub := range []*cobra.Command{
		automationListCommand(d), automationGetCommand(d), automationCreateCommand(d),
		automationUpdateCommand(d), automationDeleteCommand(d),
		automationVerbCommand(d, "pause", "Stop an automation firing"),
		automationVerbCommand(d, "resume", "Let a paused automation fire again (re-validates it)"),
		automationRunCommand(d), automationTakeOverCommand(d), automationRunsCommand(d),
		automationApplyCommand(d), automationExportCommand(d),
	} {
		sub.SilenceUsage = true
		c.AddCommand(sub)
	}
	return c
}

// automationArgs requires an automation reference first, then up to more
// further arguments (-1: any number, such as a new task).
func automationArgs(more int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
			return usageError("expected an automation (an aut- id or a name)")
		}
		if more >= 0 && len(args) > 1+more {
			return usageError("unexpected arguments after %s: %s", args[0], strings.Join(args[1:], " "))
		}
		return nil
	}
}

// automationPath is the collection, one automation, or a verb on it.
func automationPath(parts ...string) []string {
	return append([]string{"agent-automations"}, parts...)
}

// automationsAPI is an invocation bound to the active workspace, the one an
// automation name resolves in.
type automationsAPI struct {
	*invocation
	owner string
}

func (d deps) startAutomations(c *cobra.Command) (automationsAPI, context.CancelFunc, error) {
	owner, err := d.owner()
	if err != nil {
		return automationsAPI{}, nil, err
	}
	inv, cancel, err := d.start(c)
	if err != nil {
		return automationsAPI{}, nil, err
	}
	return automationsAPI{inv, owner}, cancel, nil
}

func (a automationsAPI) call(method string, query url.Values, payload any, path ...string) ([]byte, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("ownerId", a.owner)
	return a.api.send(a.ctx, method, request{path: path, query: query}, payload)
}

func (a automationsAPI) get(ref string) ([]byte, error) {
	raw, err := a.call(http.MethodGet, nil, nil, automationPath(ref)...)
	if err != nil {
		return nil, fmt.Errorf("get automation %s: %w", ref, err)
	}
	return raw, nil
}

// patch updates ref if it is still at version, with the way out of the two
// refusals a stale or foreign write meets.
func (a automationsAPI) patch(ref string, version any, patch map[string]any) ([]byte, error) {
	patch["ifVersion"] = version
	raw, err := a.call(http.MethodPatch, nil, patch, automationPath(ref)...)
	var apiErr *apiError
	switch {
	case err == nil:
		return raw, nil
	case errors.As(err, &apiErr) && apiErr.Code == codeAutomationConflict:
		return nil, fmt.Errorf("update automation %s: %w\nSomeone changed it since it was read: re-run to apply on top of the newer version", ref, err)
	case errors.As(err, &apiErr) && apiErr.Code == codeAutomationNotCreator:
		return nil, fmt.Errorf("update automation %s: %w\nOnly its creator can change it; a workspace admin can run `bex agent automations take-over %s` first", ref, err, ref)
	}
	return nil, fmt.Errorf("update automation %s: %w", ref, err)
}

// pageFlags are a list's --limit and --cursor.
type pageFlags struct {
	limit  int
	cursor string
}

func (p *pageFlags) bind(c *cobra.Command, limit int) {
	c.Flags().IntVar(&p.limit, "limit", limit, "page size")
	c.Flags().StringVar(&p.cursor, "cursor", "", "the cursor the previous page printed")
}

func (p pageFlags) query() url.Values {
	query := url.Values{"limit": {strconv.Itoa(p.limit)}}
	if p.cursor != "" {
		query.Set("cursor", p.cursor)
	}
	return query
}

func writeMore(w io.Writer, cursor string) {
	if cursor != "" {
		_, _ = fmt.Fprintf(w, "\nMore: --cursor %s\n", cursor)
	}
}

// listPage reads one page of path and prints it: the API's answer for -o
// json/yaml, else page decoded and written by text.
func listPage[T any](d deps, c *cobra.Command, page pageFlags, extra url.Values, what string, text func(io.Writer, T) string, path ...string) error {
	a, cancel, err := d.startAutomations(c)
	if err != nil {
		return err
	}
	defer cancel()
	query := page.query()
	for key, values := range extra {
		query[key] = values
	}
	raw, err := a.call(http.MethodGet, query, nil, path...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if a.format != formatText {
		return writeRaw(a.out, a.format, raw)
	}
	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	writeMore(a.out, text(a.out, decoded))
	return nil
}

func automationListCommand(d deps) *cobra.Command {
	var (
		mine bool
		page pageFlags
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List the workspace's automations",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			extra := url.Values{}
			if mine {
				extra.Set("mine", "true")
			}
			return listPage(d, c, page, extra, "list automations", func(w io.Writer, l automationList) string {
				writeAutomationList(w, l.Automations)
				return l.NextCursor
			}, automationPath()...)
		},
	}
	c.Flags().BoolVar(&mine, "mine", false, "only automations I created")
	page.bind(c, 50)
	return c
}

func automationRunsCommand(d deps) *cobra.Command {
	var page pageFlags
	c := &cobra.Command{
		Use:   "runs <automation>",
		Short: "List an automation's runs, newest first",
		Args:  automationArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			return listPage(d, c, page, nil, "list runs of automation "+args[0], func(w io.Writer, l automationRunList) string {
				writeRunList(w, l.Runs)
				return l.NextCursor
			}, automationPath(args[0], "runs")...)
		},
	}
	page.bind(c, 20)
	return c
}

// automationCommand is a command that sends one request about args[0] and
// shows the automation the API answers, after confirm when it asks a question.
func automationCommand(d deps, use, short, method, verb, question string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  automationArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			if question != "" {
				if err := d.confirm(c, fmt.Sprintf(question, args[0])); err != nil {
					return err
				}
			}
			a, cancel, err := d.startAutomations(c)
			if err != nil {
				return err
			}
			defer cancel()
			path := automationPath(args[0])
			if verb != "" {
				path = automationPath(args[0], verb)
			}
			raw, err := a.call(method, nil, nil, path...)
			if err != nil {
				return fmt.Errorf("%s automation %s: %w", firstNonEmpty(verb, strings.ToLower(method)), args[0], err)
			}
			return a.showAutomation(raw)
		},
	}
}

func automationGetCommand(d deps) *cobra.Command {
	return automationCommand(d, "get <automation>", "Show an automation: its schedule in words, next fire and last run",
		http.MethodGet, "", "")
}

// automationVerbCommand is pause or resume.
func automationVerbCommand(d deps, verb, short string) *cobra.Command {
	return automationCommand(d, verb+" <automation>", short, http.MethodPost, verb, "")
}

func automationTakeOverCommand(d deps) *cobra.Command {
	c := automationCommand(d, "take-over <automation>", "Make yourself an automation's creator (workspace admins)",
		http.MethodPost, "take-over", "Take over automation %s? It will run as you.")
	c.Long = "An automation runs as the member who created it. A workspace admin can\n" +
		"take one over; it then runs as the admin, and its template is\n" +
		"re-validated under the admin's access."
	return c
}

func automationDeleteCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <automation>",
		Short: "Delete an automation and its run history (its sessions stay)",
		Args:  automationArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			if err := d.confirm(c, fmt.Sprintf("Delete automation %s and its run history?", args[0])); err != nil {
				return err
			}
			a, cancel, err := d.startAutomations(c)
			if err != nil {
				return err
			}
			defer cancel()
			if _, err := a.call(http.MethodDelete, nil, nil, automationPath(args[0])...); err != nil {
				return fmt.Errorf("delete automation %s: %w", args[0], err)
			}
			if a.format == formatText {
				_, _ = fmt.Fprintf(a.out, "Deleted automation %s.\n", args[0])
			}
			return nil
		},
	}
}

func automationCreateCommand(d deps) *cobra.Command {
	var (
		name     string
		disabled bool
		sched    scheduleFlags
		tpl      templateFlags
	)
	c := &cobra.Command{
		Use:   "create --name NAME <schedule flags> [flags] <task…>",
		Short: "Create an automation",
		Long: "Creates an automation with one schedule trigger and a session template.\n" +
			"Give exactly one of --every, --once or --cron. --timezone defaults to\n" +
			"this machine's time zone. A template on the platform model needs\n" +
			"--max-spend; the API refuses one without.",
		Example: "  bex agent automations create --name nightly-deps --every week --on mon --at 03:00 \\\n" +
			"    --repo acme/api --max-spend 5 --secret NPM_TOKEN \"Update dependencies and open a draft PR\"",
		RunE: func(c *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return usageError("--name is required")
			}
			if err := refuseSessionOnly(c); err != nil {
				return err
			}
			schedule, err := sched.build(c, d, nil)
			if err != nil {
				return err
			}
			template, err := tpl.template(c, d, args, nil)
			if err != nil {
				return err
			}
			body := map[string]any{"name": name, "triggers": scheduleTriggers(schedule), "template": template}
			if disabled {
				body["enabled"] = false
			}
			a, cancel, err := d.startAutomations(c)
			if err != nil {
				return err
			}
			defer cancel()
			raw, err := a.call(http.MethodPost, nil, body, automationPath()...)
			if err != nil {
				return fmt.Errorf("create automation %s: %w", name, err)
			}
			return a.showAutomation(raw)
		},
	}
	c.Flags().StringVar(&name, "name", "", "automation name: lowercase letters, digits and hyphens, at most 64")
	c.Flags().BoolVar(&disabled, "disabled", false, "create it paused")
	sched.bind(c)
	tpl.bind(c)
	return c
}

func automationUpdateCommand(d deps) *cobra.Command {
	var (
		name  string
		sched scheduleFlags
		tpl   templateFlags
	)
	c := &cobra.Command{
		Use:   "update <automation> [flags] [<task…>]",
		Short: "Change an automation's name, schedule or template",
		Long: "Changes only what the flags (and a new task) name; everything else keeps\n" +
			"its stored value. The update is refused if someone else changed the\n" +
			"automation since it was read: re-run the command to apply on top.",
		Args: automationArgs(-1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := refuseSessionOnly(c); err != nil {
				return err
			}
			ref, task := args[0], args[1:]
			renaming := c.Flags().Changed("name")
			if !renaming && !sched.changed() && !tpl.changed() && len(task) == 0 {
				return usageError("nothing to update: give a flag or a new task")
			}
			a, cancel, err := d.startAutomations(c)
			if err != nil {
				return err
			}
			defer cancel()
			raw, err := a.get(ref)
			if err != nil {
				return err
			}
			var stored map[string]any
			if err := decodeExact(raw, &stored); err != nil {
				return fmt.Errorf("get automation %s: %w", ref, err)
			}
			patch := map[string]any{}
			if renaming {
				patch["name"] = name
			}
			if sched.changed() {
				schedule, err := sched.build(c, d, storedSchedule(stored))
				if err != nil {
					return err
				}
				patch["triggers"] = scheduleTriggers(schedule)
			}
			if tpl.changed() || len(task) > 0 {
				storedTemplate, _ := stored["template"].(map[string]any)
				template, err := tpl.template(c, d, task, storedTemplate)
				if err != nil {
					return err
				}
				patch["template"] = template
			}
			raw, err = a.patch(ref, stored["version"], patch)
			if err != nil {
				return err
			}
			return a.showAutomation(raw)
		},
	}
	c.Flags().StringVar(&name, "name", "", "rename the automation")
	sched.bind(c)
	tpl.bind(c)
	return c
}

func automationRunCommand(d deps) *cobra.Command {
	var waiting waitFlags
	c := &cobra.Command{
		Use:   "run <automation>",
		Short: "Fire an automation now",
		Long: "Fires the automation once, now, under the same rules as a scheduled\n" +
			"fire (access, overlap, spend); the schedule does not move. With --wait or\n" +
			"--follow it then waits on the session and exits by its state. A run that\n" +
			"started no session (skipped_overlap, denied, failed_create) exits 6.",
		Args: automationArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			if err := waiting.validate(); err != nil {
				return err
			}
			a, cancel, err := d.startAutomations(c)
			if err != nil {
				return err
			}
			defer cancel()
			raw, err := a.call(http.MethodPost, nil, nil, automationPath(args[0], "run")...)
			if err != nil {
				return fmt.Errorf("run automation %s: %w", args[0], err)
			}
			var r automationRun
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("run automation %s: %w", args[0], err)
			}
			if r.Outcome != runStarted || r.SessionID == "" {
				if err := a.showRun(raw, r); err != nil {
					return err
				}
				reason := r.Outcome
				if r.Error != "" {
					reason += ": " + r.Error
				}
				return command.NewExitError(exitNotStarted, fmt.Errorf("automation %s started no session (%s)", args[0], reason))
			}
			if !waiting.blocking() {
				return a.showRun(raw, r)
			}
			_, _ = fmt.Fprintf(a.errOut, "Started %s\n", r.SessionID)
			return a.await(session{ID: r.SessionID}, 1, waiting)
		},
	}
	waiting.bindTurn(c)
	return c
}

func (inv *invocation) showAutomation(raw []byte) error {
	if inv.format != formatText {
		return writeRaw(inv.out, inv.format, raw)
	}
	var a automation
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("decode automation: %w", err)
	}
	writeAutomation(inv.out, a)
	return nil
}

func (inv *invocation) showRun(raw []byte, r automationRun) error {
	if inv.format != formatText {
		return writeRaw(inv.out, inv.format, raw)
	}
	line := fmt.Sprintf("Run of %s at %s: %s %s", r.AutomationID, r.FireAt.Local().Format(time.DateTime), r.Outcome, arrow(r.SessionID))
	if r.Error != "" {
		line += " (" + r.Error + ")"
	}
	_, _ = fmt.Fprintln(inv.out, strings.TrimSpace(line))
	return nil
}

// confirmed is upstream's global --confirm ("skip all confirmation
// prompts"), from the root pre-run or, when none ran, the raw flag.
func confirmed(c *cobra.Command) bool {
	if command.GetConfirmFromContext(c.Context()) {
		return true
	}
	yes, err := c.Flags().GetBool(command.ConfirmFlag)
	return err == nil && yes
}

// confirm asks question on a terminal unless --confirm skips prompts;
// elsewhere a destructive command needs --confirm, so a script never deletes
// by accident.
func (d deps) confirm(c *cobra.Command, question string) error {
	if confirmed(c) {
		return nil
	}
	if !d.interactive() {
		return usageError("%s needs confirmation: pass --confirm", c.CommandPath())
	}
	_, _ = fmt.Fprintf(c.ErrOrStderr(), "%s [y/N] ", question)
	line, _ := bufio.NewReader(d.stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return errors.New("aborted")
}

// scheduleTriggers is the one schedule trigger ADR101 phase 1 accepts.
func scheduleTriggers(schedule map[string]any) []any {
	return []any{map[string]any{"type": "schedule", "schedule": schedule}}
}

// storedSchedule is the stored automation's schedule, as a map.
func storedSchedule(stored map[string]any) map[string]any {
	triggers, _ := stored["triggers"].([]any)
	for _, t := range triggers {
		if trigger, ok := t.(map[string]any); ok {
			if schedule, ok := trigger["schedule"].(map[string]any); ok {
				return schedule
			}
		}
	}
	return nil
}

func writeAutomation(w io.Writer, a automation) {
	_, _ = fmt.Fprintf(w, "%s · %s · %s\n", a.ID, a.Name, automationStatus(a))
	tw := newTable(w)
	s := firstSchedule(a)
	tw.row("Schedule", scheduleWords(s))
	tw.row("Next fire", zoneTime(a.NextFireAt, s))
	if a.LastFireAt != nil {
		tw.row("Last fire", strings.TrimSpace(zoneTime(a.LastFireAt, s)+" "+arrow(a.LastSessionID)))
	}
	if a.ConsecutiveFailures > 0 {
		tw.row("Failures", strconv.Itoa(a.ConsecutiveFailures)+" in a row")
	}
	t := a.Template
	if t.Repo != "" {
		tw.row("Repo", strings.TrimSpace(t.Repo+" "+branchSuffix(t.Branch)))
	}
	tw.row("Agent", strings.TrimSuffix(t.AgentConfig.Agent+" · "+t.AgentConfig.Model, " · "))
	if t.MaxSpendUSD != nil {
		tw.row("Max spend", "$"+strconv.FormatFloat(*t.MaxSpendUSD, 'f', -1, 64)+" a run")
	}
	tw.row("Secrets", strings.Join(t.SecretNames, ", "))
	tw.row("Task", clip(firstLine(t.Task), 80))
	tw.row("Created by", a.CreatedBy)
	tw.row("Version", strconv.Itoa(a.Version))
	_ = tw.Flush()
}

func writeAutomationList(w io.Writer, list []automation) {
	if len(list) == 0 {
		_, _ = fmt.Fprintln(w, "No automations.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tSCHEDULE\tNEXT FIRE\tSTATUS\tID")
	for _, a := range list {
		s := firstSchedule(a)
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.Name, clip(scheduleWords(s), 60), firstNonEmpty(zoneTime(a.NextFireAt, s), "—"), automationStatus(a), a.ID)
	}
	_ = tw.Flush()
}

func writeRunList(w io.Writer, runs []automationRun) {
	if len(runs) == 0 {
		_, _ = fmt.Fprintln(w, "No runs.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "FIRED\tOUTCOME\tSESSION\tMISSED\tERROR")
	for _, r := range runs {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.FireAt.Local().Format(minuteLayout), r.Outcome, firstNonEmpty(r.SessionID, "—"), r.MissedTicks, clip(r.Error, 60))
	}
	_ = tw.Flush()
}

func automationStatus(a automation) string {
	switch {
	case a.Enabled:
		return "active"
	case a.PausedReason != "":
		return "paused (" + a.PausedReason + ")"
	}
	return "paused"
}

func firstSchedule(a automation) *automationSchedule {
	for _, t := range a.Triggers {
		if t.Schedule != nil {
			return t.Schedule
		}
	}
	return nil
}

func arrow(id string) string {
	if id == "" {
		return ""
	}
	return "→ " + id
}

// scheduleLocation is the schedule's own time zone, else this machine's.
func scheduleLocation(s *automationSchedule) *time.Location {
	if s != nil {
		if loc, err := time.LoadLocation(s.Timezone); err == nil {
			return loc
		}
	}
	return time.Local
}

// zoneTime shows t in the schedule's time zone.
func zoneTime(t *time.Time, s *automationSchedule) string {
	if t == nil {
		return ""
	}
	return t.In(scheduleLocation(s)).Format(minuteLayout + " MST")
}

// scheduleWords says when a schedule fires, as the dashboard does: "every day
// at 09:00 Europe/Berlin, ends before 2027-01-01 00:00".
func scheduleWords(s *automationSchedule) string {
	if s == nil {
		return "no schedule"
	}
	var words string
	switch s.Frequency {
	case frequencyHour:
		words = "every hour"
		if s.Minute != nil {
			words += fmt.Sprintf(" at minute %02d", *s.Minute)
		}
	case frequencyDay:
		words = "every day at " + s.At
	case frequencyWeek:
		days := make([]string, 0, len(s.Weekdays))
		for _, day := range s.Weekdays {
			if day != "" {
				days = append(days, strings.ToUpper(day[:1])+strings.ToLower(day[1:]))
			}
		}
		words = "every week on " + strings.Join(days, ", ") + " at " + s.At
	case frequencyOnce:
		words = "once"
		if s.RunAt != nil {
			words += " at " + s.RunAt.In(scheduleLocation(s)).Format(minuteLayout)
		}
	case frequencyCustom:
		words = fmt.Sprintf("cron %q", s.Cron)
	default:
		words = s.Frequency
	}
	words += " " + s.Timezone
	if s.EndsAt != nil {
		words += ", ends before " + s.EndsAt.In(scheduleLocation(s)).Format(minuteLayout)
	}
	return words
}
