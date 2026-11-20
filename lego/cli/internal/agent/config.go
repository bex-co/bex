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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Secret values and the model key are write-only: they come from stdin, a
// hidden prompt or the environment, never from an argument, where shell
// history and the process list would keep them.

const (
	// maxSecretBytes bounds a secret value or model key read from stdin.
	maxSecretBytes = 64 << 10
	// maxSkillBytes bounds a SKILL.md read from a file or stdin.
	maxSkillBytes = 1 << 20
	// spendCapPlanDefault returns the spend cap to the plan's.
	spendCapPlanDefault = "default"
)

func secretPath(parts ...string) []string { return append([]string{"agent-secrets"}, parts...) }
func skillPath(parts ...string) []string  { return append([]string{"agent-skills"}, parts...) }

// envValue is the non-empty value of environment variable name, for flag.
func envValue(d deps, flag, name string) (string, error) {
	value, ok := d.lookupEnv(name)
	if !ok || value == "" {
		return "", usageError("%s: the environment variable %s is not set", flag, name)
	}
	return value, nil
}

// readSecret is a write-only value typed at the terminal without echo, else
// read from stdin. One trailing newline (from echo or a heredoc) is dropped.
func readSecret(d deps, what string) (string, error) {
	raw, err := d.readHidden(what)
	if err == nil && raw == nil {
		raw, err = readPathOrStdin(d, "-", maxSecretBytes)
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if value == "" {
		return "", usageError("the %s is empty: pipe it on stdin or use --from-env VAR", what)
	}
	return value, nil
}

func nameArg(what string) cobra.PositionalArgs { return refArgs("one "+what+" name", 0) }

type agentSecretList struct {
	Secrets []struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
		Repo  string `json:"repo"`
	} `json:"secrets"`
}

// secretBody is a secret write's body, leaving out what is unset: no value
// keeps the stored one, no scope keeps an existing secret's scope (a new one
// is workspace-wide).
func secretBody(value, scope, repo string) map[string]string {
	body := map[string]string{}
	for key, v := range map[string]string{"value": value, "scope": scope, "repo": repo} {
		if v != "" {
			body[key] = v
		}
	}
	return body
}

func addScopeFlags(c *cobra.Command, scope, repo *string) {
	c.Flags().StringVar(scope, "scope", "", "workspace (every session; the default for a new secret), repo (sessions on --repo) or explicit (only sessions that name it)")
	c.Flags().StringVar(repo, "repo", "", "owner/name, with --scope repo")
}

// scopeLabel is how list prints a scope: workspace, explicit, repo:owner/name.
func scopeLabel(scope, repo string) string {
	if scope == "repo" {
		return "repo:" + repo
	}
	if scope == "" {
		return "explicit"
	}
	return scope
}

func secretsCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "Manage the workspace's agent secrets (values are write-only)",
		Long: "Workspace agent secrets reach sessions as env vars by scope: workspace (every\n" +
			"session, the default for a new secret), repo (sessions on one repository) or\n" +
			"explicit (only sessions that name it: `run --secret NAME`, an automation's\n" +
			"--secret). Reads show names and scopes only; a value is set from stdin, a\n" +
			"hidden prompt or --from-env, never an argument.",
	}
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the workspace's agent secrets and their scopes",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "list agent secrets", func(w io.Writer, list agentSecretList) error {
				if len(list.Secrets) == 0 {
					_, _ = fmt.Fprintln(w, "No agent secrets.")
				}
				for _, s := range list.Secrets {
					_, _ = fmt.Fprintf(w, "%s\t%s\n", s.Name, scopeLabel(s.Scope, s.Repo))
				}
				return nil
			}, secretPath()...)
		},
	})
	var fromEnv, scope, repo string
	set := &cobra.Command{
		Use:   "set NAME",
		Short: "Create or replace an agent secret",
		Args:  nameArg("secret"),
		RunE: func(c *cobra.Command, args []string) error {
			var (
				value string
				err   error
			)
			if fromEnv != "" {
				value, err = envValue(d, "--from-env", fromEnv)
			} else {
				value, err = readSecret(d, "value of "+args[0])
			}
			if err != nil {
				return err
			}
			return workspaceShow(d, c, http.MethodPut, nil, secretBody(value, scope, repo), "set agent secret "+args[0],
				func(w io.Writer, _ struct{}) error {
					_, _ = fmt.Fprintf(w, "Set agent secret %s.\n", args[0])
					return nil
				}, secretPath(args[0])...)
		},
	}
	set.Flags().StringVar(&fromEnv, "from-env", "", "read the value from this environment variable")
	addScopeFlags(set, &scope, &repo)
	var rescopeScope, rescopeRepo string
	rescope := &cobra.Command{
		Use:   "rescope NAME --scope SCOPE",
		Short: "Change which sessions receive an agent secret, keeping its value",
		Args:  nameArg("secret"),
		RunE: func(c *cobra.Command, args []string) error {
			if rescopeScope == "" {
				return usageError("--scope is required: workspace, repo or explicit")
			}
			return workspaceShow(d, c, http.MethodPut, nil, secretBody("", rescopeScope, rescopeRepo), "rescope agent secret "+args[0],
				func(w io.Writer, out struct {
					Scope string `json:"scope"`
					Repo  string `json:"repo"`
				}) error {
					_, _ = fmt.Fprintf(w, "Agent secret %s is now %s.\n", args[0], scopeLabel(out.Scope, out.Repo))
					return nil
				}, secretPath(args[0])...)
		},
	}
	addScopeFlags(rescope, &rescopeScope, &rescopeRepo)
	root.AddCommand(set, rescope, &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete an agent secret",
		Args:  nameArg("secret"),
		RunE: func(c *cobra.Command, args []string) error {
			return d.workspaceDelete(c, "agent secret",
				fmt.Sprintf("Delete agent secret %s? Sessions that name it will fail to start.", args[0]), secretPath(args[0])...)
		},
	})
	return root
}

// agentSkill is a workspace skill (ADR099) as the API shows it.
type agentSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"`
	Enabled     bool     `json:"enabled"`
	Body        string   `json:"body"`
	Content     string   `json:"content"`
	Version     int      `json:"version"`
}

type agentSkillList struct {
	Skills []agentSkill `json:"skills"`
}

type skillVersion struct {
	Version   int       `json:"version"`
	ShortID   string    `json:"shortId"`
	Message   string    `json:"message"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	Current   bool      `json:"current"`
}

type skillVersionList struct {
	Versions   []skillVersion `json:"versions"`
	NextCursor string         `json:"nextCursor"`
}

func skillsCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:     "skills",
		Aliases: []string{"skill"},
		Short:   "Manage the workspace's agent skills (SKILL.md documents sessions use)",
		Long: "Workspace skills are SKILL.md documents every session receives (ADR099);\n" +
			"`run --skill NAME` makes a turn use one. Writes need a workspace admin.\n" +
			"Skills for your own local coding agents are `bex skills`.",
	}
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the workspace's agent skills",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "list agent skills", writeSkillList, skillPath()...)
		},
	})
	var version int
	get := &cobra.Command{
		Use:   "get NAME",
		Short: "Print a skill's SKILL.md (or one saved version of it)",
		Args:  nameArg("skill"),
		RunE: func(c *cobra.Command, args []string) error {
			path := skillPath(args[0])
			if c.Flags().Changed("version") {
				path = append(path, "versions", strconv.Itoa(version))
			}
			return workspaceShow(d, c, http.MethodGet, nil, nil, "get agent skill "+args[0], func(w io.Writer, s agentSkill) error {
				_, err := io.WriteString(w, strings.TrimSuffix(firstNonEmpty(s.Content, s.Body), "\n")+"\n")
				return err
			}, path...)
		},
	}
	get.Flags().IntVar(&version, "version", 0, "a saved version's number")
	var page pageFlags
	versions := &cobra.Command{
		Use:   "versions NAME",
		Short: "List a skill's saved versions, newest first",
		Args:  nameArg("skill"),
		RunE: func(c *cobra.Command, args []string) error {
			return listPage(d, c, page, nil, "list versions of agent skill "+args[0], func(w io.Writer, l skillVersionList) string {
				writeSkillVersions(w, l.Versions)
				return l.NextCursor
			}, skillPath(args[0], "versions")...)
		},
	}
	page.bind(versions, 20)
	root.AddCommand(get,
		skillWriteCommand(d, "create -f SKILL.md", "Create an agent skill from a SKILL.md", http.MethodPost, cobra.NoArgs),
		skillWriteCommand(d, "update NAME -f SKILL.md", "Replace an agent skill with a SKILL.md (keeps a version)", http.MethodPatch, nameArg("skill")),
		&cobra.Command{
			Use:   "delete NAME",
			Short: "Delete an agent skill and its versions",
			Args:  nameArg("skill"),
			RunE: func(c *cobra.Command, args []string) error {
				return d.workspaceDelete(c, "agent skill",
					fmt.Sprintf("Delete agent skill %s and its version history?", args[0]), skillPath(args[0])...)
			},
		},
		versions)
	return root
}

// skillWriteCommand saves a whole SKILL.md: POST for create (no argument),
// PATCH of args[0] for update.
func skillWriteCommand(d deps, use, short, method string, args cobra.PositionalArgs) *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  args,
		RunE: func(c *cobra.Command, args []string) error {
			if file == "" {
				return usageError("give the SKILL.md with -f FILE (- for stdin)")
			}
			content, err := readPathOrStdin(d, file, maxSkillBytes)
			if err != nil {
				return err
			}
			return workspaceShow(d, c, method, nil, map[string]string{"content": string(content)}, "save agent skill",
				func(w io.Writer, s agentSkill) error {
					_, _ = fmt.Fprintf(w, "Saved skill %s, version %d.\n", s.Name, s.Version)
					return nil
				}, skillPath(args...)...)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "the SKILL.md (YAML frontmatter with name and description, then the body); - for stdin")
	return c
}

func writeSkillList(w io.Writer, list agentSkillList) error {
	if len(list.Skills) == 0 {
		_, _ = fmt.Fprintln(w, "No agent skills.")
		return nil
	}
	tw := newTable(w)
	_, _ = fmt.Fprintln(tw, "NAME\tVERSION\tENABLED\tTRIGGERS\tDESCRIPTION")
	for _, s := range list.Skills {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%t\t%s\t%s\n", s.Name, s.Version, s.Enabled, strings.Join(s.Triggers, ","), clip(s.Description, 60))
	}
	return tw.Flush()
}

func writeSkillVersions(w io.Writer, versions []skillVersion) {
	if len(versions) == 0 {
		_, _ = fmt.Fprintln(w, "No versions.")
		return
	}
	tw := newTable(w)
	_, _ = fmt.Fprintln(tw, "VERSION\tID\tSAVED\tBY\tMESSAGE")
	for _, v := range versions {
		mark := ""
		if v.Current {
			mark = " (current)"
		}
		_, _ = fmt.Fprintf(tw, "%d%s\t%s\t%s\t%s\t%s\n", v.Version, mark, v.ShortID, v.CreatedAt.Local().Format(minuteLayout), v.CreatedBy, v.Message)
	}
	_ = tw.Flush()
}

// sandboxTemplate is an agent sandbox template a session can name with
// --template, as bex-api lists it (w10/m519).
type sandboxTemplate struct {
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	MemoryGB float64 `json:"memoryGb"`
}

// size words t's limits: "4 CPU, 8 GB".
func (t sandboxTemplate) size() string {
	return strconv.FormatFloat(t.CPU, 'f', -1, 64) + " CPU, " + strconv.FormatFloat(t.MemoryGB, 'f', -1, 64) + " GB"
}

// templateSize is name and its size when templates list it: "agent-4c-8g
// (4 CPU, 8 GB)".
func templateSize(name string, templates []sandboxTemplate) string {
	if i := slices.IndexFunc(templates, func(t sandboxTemplate) bool { return t.Name == name }); i >= 0 {
		return name + " (" + templates[i].size() + ")"
	}
	return name
}

// agentEnvironment is the workspace agent environment, as text output needs it.
type agentEnvironment struct {
	NetworkTier                   string            `json:"networkTier"`
	CustomHosts                   []string          `json:"customHosts"`
	DefaultModel                  *string           `json:"defaultModel"`
	Models                        []string          `json:"models"`
	DefaultTemplate               string            `json:"defaultTemplate"`
	Templates                     []sandboxTemplate `json:"templates"`
	ModelSource                   string            `json:"modelSource"`
	ModelKeySet                   bool              `json:"modelKeySet"`
	AgentModelMonthlySpendCapUSD  *float64          `json:"agentModelMonthlySpendCapUsd"`
	AgentModelSpendMonthToDateUSD *float64          `json:"agentModelSpendMonthToDateUsd"`
	TrustTier                     *string           `json:"trustTier"`
	EnabledAgents                 []string          `json:"enabledAgents"`
	// EnabledModels are the catalog models members may choose (w10/m527);
	// nil from a bex-api that predates the list, which offers every one.
	EnabledModels []string `json:"enabledModels"`
	// GitHubCLIEnabled is whether the workspace's sessions get a `gh`
	// (w10/m524); nil from a bex-api that predates the setting, which
	// cannot turn it off, so it reads as on.
	GitHubCLIEnabled *bool `json:"githubCliEnabled"`
	AgentDataUse     []struct {
		Agent          string     `json:"agent"`
		AcknowledgedAt *time.Time `json:"acknowledgedAt"`
		AcknowledgedBy *string    `json:"acknowledgedBy"`
	} `json:"agentDataUse"`
}

func envCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "env",
		Short: "Show the workspace's agent environment: network, GitHub CLI, models and sandbox, spend cap, agents",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "read the agent environment", writeEnvironment, "agent-environment")
		},
	}
	var (
		tier, model, spendCap string
		template, githubCLI   string
		hosts, agents, acks   []string
		models                []string
		noHosts, keyStdin     bool
		clearKey              bool
	)
	set := &cobra.Command{
		Use:   "set [flags]",
		Short: "Change the workspace's agent environment (admins)",
		Long: "Changes only what the flags name. --spend-cap takes USD or `" + spendCapPlanDefault + "` (the\n" +
			"plan's cap). The workspace's own model key is read from stdin or a hidden\n" +
			"prompt with --model-key-stdin, never from an argument. --agents replaces the\n" +
			"agents members can start sessions with; an agent whose model provider may\n" +
			"train on session data also needs --acknowledge-data-use naming it.\n" +
			"--default-template sets the sandbox template sessions and automation runs\n" +
			"that name none run on (`bex agent models` lists them; \"\" returns to the\n" +
			"platform's); it applies to sessions created afterwards.\n" +
			"--github-cli off turns gh off for the workspace's agent sessions: turns\n" +
			"dispatched afterwards get no gh configuration, and a running session's next\n" +
			"gh call is refused with a message naming this setting. Git pushes to the\n" +
			"session branch and the bex MCP tools keep working. --github-cli on turns it\n" +
			"back on.\n" +
			"--models replaces the platform catalog models members may choose; the\n" +
			"catalog's others are turned off. It must keep the catalog's default (the\n" +
			"first of `bex agent env`'s Models). A session, message or automation naming\n" +
			"a turned-off model is refused, and one already on it runs the workspace\n" +
			"default from its next turn.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			f := c.Flags()
			patch := map[string]any{}
			if f.Changed("network-tier") {
				patch["networkTier"] = tier
			}
			switch {
			case noHosts && f.Changed("custom-host"):
				return usageError("use --custom-host or --no-custom-hosts, not both")
			case noHosts:
				patch["customHosts"] = []string{}
			case f.Changed("custom-host"):
				patch["customHosts"] = hosts
			}
			if f.Changed("default-model") {
				patch["defaultModel"] = model
			}
			if f.Changed("default-template") {
				patch["defaultTemplate"] = template
			}
			if f.Changed("github-cli") {
				switch githubCLI {
				case "on", "off":
					patch["githubCliEnabled"] = githubCLI == "on"
				default:
					return usageError("--github-cli takes on or off, not %q", githubCLI)
				}
			}
			if f.Changed("spend-cap") {
				if spendCap == spendCapPlanDefault {
					patch["resetAgentModelMonthlySpendCap"] = true
				} else if usd, err := strconv.ParseFloat(spendCap, 64); err == nil && usd > 0 {
					patch["agentModelMonthlySpendCapUsd"] = usd
				} else {
					return usageError("--spend-cap takes USD (more than 0) or %s, not %q", spendCapPlanDefault, spendCap)
				}
			}
			switch {
			case keyStdin && clearKey:
				return usageError("use --model-key-stdin or --clear-model-key, not both")
			case clearKey:
				patch["clearModelApiKey"] = true
			case keyStdin:
				key, err := readSecret(d, "model API key")
				if err != nil {
					return err
				}
				patch["modelApiKey"] = key
			}
			if f.Changed("agents") {
				patch["enabledAgents"] = agents
			}
			if f.Changed("models") {
				if len(models) == 0 {
					return usageError("--models takes the models members may choose, comma-separated; the catalog's default always stays on")
				}
				patch["enabledModels"] = models
			}
			if len(acks) > 0 {
				patch["acknowledgeAgentDataUse"] = acks
			}
			if len(patch) == 0 {
				return usageError("nothing to change: give a flag")
			}
			return workspaceShow(d, c, http.MethodPatch, nil, patch, "update the agent environment", writeEnvironment, "agent-environment")
		},
	}
	f := set.Flags()
	f.StringVar(&tier, "network-tier", "", "the sandbox network tier")
	f.StringArrayVar(&hosts, "custom-host", nil, "a host sessions may reach; repeatable, replaces the list")
	f.BoolVar(&noHosts, "no-custom-hosts", false, "remove every custom host")
	f.StringVar(&model, "default-model", "", "the workspace's default model id (\"\" returns to the provider default)")
	f.StringVar(&template, "default-template", "", "the workspace's default sandbox template (\"\" returns to the platform's; `bex agent models` lists them)")
	f.StringVar(&githubCLI, "github-cli", "", "on or off: whether the workspace's agent sessions get gh (on unless an admin turned it off)")
	f.StringVar(&spendCap, "spend-cap", "", "monthly agent-model spend cap in USD, or "+spendCapPlanDefault)
	f.BoolVar(&keyStdin, "model-key-stdin", false, "set the workspace's own model API key, read from stdin")
	f.BoolVar(&clearKey, "clear-model-key", false, "remove the workspace's own model API key")
	f.StringSliceVar(&agents, "agents", nil, "the agents members can start sessions with, comma-separated; replaces the list (\"\" turns every agent off)")
	f.StringSliceVar(&models, "models", nil, "the platform catalog models members may choose, comma-separated; replaces the list and must keep the catalog's default")
	f.StringArrayVar(&acks, "acknowledge-data-use", nil, "acknowledge, as an admin, that this agent's model provider may train on what its sessions send; repeatable")
	root.AddCommand(set)
	return root
}

func writeEnvironment(w io.Writer, e agentEnvironment) error {
	tw := newTable(w)
	tw.row("Network tier", e.NetworkTier)
	tw.row("Custom hosts", strings.Join(e.CustomHosts, ", "))
	if e.GitHubCLIEnabled == nil || *e.GitHubCLIEnabled {
		tw.row("GitHub CLI", "on")
	} else {
		tw.row("GitHub CLI", "off (sessions get no gh; Git pushes and the bex tools still work)")
	}
	// A bex-api that predates the list offers the whole catalog.
	enabled := e.EnabledModels
	if enabled == nil {
		enabled = e.Models
	}
	switch {
	case e.DefaultModel != nil && len(e.Models) > 0 && !slices.Contains(enabled, *e.DefaultModel):
		why := "turned off"
		if !slices.Contains(e.Models, *e.DefaultModel) {
			why = "no longer offered"
		}
		tw.row("Default model", *e.DefaultModel+" ("+why+": sessions run "+e.Models[0]+", the platform default)")
	case e.DefaultModel != nil:
		tw.row("Default model", *e.DefaultModel)
	case len(e.Models) > 0:
		tw.row("Default model", e.Models[0]+" (the platform default)")
	}
	if len(e.Models) > 0 {
		on := make([]string, 0, len(enabled))
		for _, m := range enabled {
			if m == e.Models[0] {
				m += " (the platform default, always on)"
			}
			on = append(on, m)
		}
		tw.row("Models", strings.Join(on, ", "))
		var off []string
		for _, m := range e.Models {
			if !slices.Contains(enabled, m) {
				off = append(off, m)
			}
		}
		if len(off) > 0 {
			tw.row("Models off", strings.Join(off, ", ")+" (turned off by an admin)")
		}
	}
	if e.DefaultTemplate != "" {
		tw.row("Default sandbox", templateSize(e.DefaultTemplate, e.Templates))
	}
	source := e.ModelSource
	if e.ModelKeySet {
		source += " (the workspace's own key)"
	}
	tw.row("Model source", source)
	if e.AgentModelMonthlySpendCapUSD != nil {
		used := ""
		if e.AgentModelSpendMonthToDateUSD != nil {
			used = fmt.Sprintf(", $%.2f used this month", *e.AgentModelSpendMonthToDateUSD)
		}
		tw.row("Spend cap", fmt.Sprintf("$%.2f a month%s", *e.AgentModelMonthlySpendCapUSD, used))
	}
	if e.TrustTier != nil {
		tw.row("Trust tier", *e.TrustTier)
	}
	if e.EnabledAgents != nil {
		tw.row("Agents", firstNonEmpty(strings.Join(e.EnabledAgents, ", "), "none"))
	}
	for _, use := range e.AgentDataUse {
		state := "its provider may train on session data; needs an admin's --acknowledge-data-use to turn on"
		if use.AcknowledgedAt != nil {
			by := ""
			if use.AcknowledgedBy != nil {
				by = " by " + *use.AcknowledgedBy
			}
			state = fmt.Sprintf("data use acknowledged%s on %s", by, use.AcknowledgedAt.Local().Format(minuteLayout))
		}
		tw.row("Data use: "+use.Agent, state)
	}
	return tw.Flush()
}

type agentCapabilities struct {
	Ready  bool `json:"ready"`
	GitHub struct {
		Connected    bool   `json:"connected"`
		AccountLogin string `json:"accountLogin"`
		InstallURL   string `json:"installUrl"`
	} `json:"github"`
	Agents []struct {
		ID            string `json:"id"`
		Label         string `json:"label"`
		ModelKeyReady bool   `json:"modelKeyReady"`
		// Model is what a session that names none runs (nil: the provider's
		// own default); Models are the models one may name, empty where any
		// model the workspace's own provider serves is accepted (w10/m516).
		Model  *string  `json:"model"`
		Models []string `json:"models"`
	} `json:"agents"`
	// Templates are the sandbox templates a session may name, and
	// DefaultTemplate the one a session that names none runs on (w10/m519).
	Templates       []sandboxTemplate `json:"templates"`
	DefaultTemplate string            `json:"defaultTemplate"`
}

func modelsCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Show the agents this workspace can run, the models they can choose and the sandbox templates",
		Long: "Lists each agent the workspace can start, whether it is ready, and the model\n" +
			"a session runs when it names none. Where an agent offers a choice of models\n" +
			"(the platform model catalog), they are listed below the table with the\n" +
			"default marked: the platform catalog models the workspace's admins left on\n" +
			"(`bex agent env set --models`). Pick one per session with\n" +
			"`bex agent run --model ID`, switch a session from its next message with\n" +
			"`bex agent send --model ID`, and change the workspace default with\n" +
			"`bex agent env set --default-model ID`.\n\n" +
			"Then the sandbox templates a session can run on, with their CPU and memory\n" +
			"and the workspace default marked: pick one per session with\n" +
			"`bex agent run --template NAME`, and change the default with\n" +
			"`bex agent env set --default-template NAME`.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "read agent capabilities", writeCapabilities, "agent-sessions", "capabilities")
		},
	}
}

func writeCapabilities(w io.Writer, caps agentCapabilities) error {
	github := "not connected"
	switch {
	case caps.GitHub.Connected:
		github = "connected (" + caps.GitHub.AccountLogin + ")"
	case caps.GitHub.InstallURL != "":
		github += ": install the GitHub App at " + caps.GitHub.InstallURL
	}
	tw := newTable(w)
	tw.row("Ready", strconv.FormatBool(caps.Ready))
	tw.row("GitHub", github)
	_ = tw.Flush()
	_, _ = fmt.Fprintln(w)
	tw = newTable(w)
	_, _ = fmt.Fprintln(tw, "AGENT\tNAME\tREADY\tMODEL")
	for _, a := range caps.Agents {
		ready := "yes"
		if !a.ModelKeyReady {
			ready = "no: needs the workspace's own model key (`bex agent env set --model-key-stdin`)"
		}
		model := "the provider's default"
		switch {
		case a.Model != nil:
			model = *a.Model
		case len(a.Models) > 0:
			model = a.Models[0]
		}
		if len(a.Models) == 0 {
			model += " (any model its provider serves)"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ID, a.Label, ready, model)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, a := range caps.Agents {
		if len(a.Models) < 2 {
			continue // one fixed model, or free-form
		}
		_, _ = fmt.Fprintf(w, "\nModels %s can run (--model ID):\n", a.ID)
		for _, m := range a.Models {
			if a.Model != nil && *a.Model == m {
				m += " (default)"
			}
			_, _ = fmt.Fprintf(w, "  %s\n", m)
		}
	}
	if len(caps.Templates) == 0 {
		return nil
	}
	_, _ = fmt.Fprintln(w, "\nSandbox templates (--template NAME):")
	tw = newTable(w)
	for _, t := range caps.Templates {
		mark := ""
		if t.Name == caps.DefaultTemplate {
			mark = " (default)"
		}
		_, _ = fmt.Fprintf(tw, "  %s\t%s%s\n", t.Name, t.size(), mark)
	}
	return tw.Flush()
}

// workspaceShow sends one ownerId-addressed request and prints the answer:
// the API's own for -o json/yaml, else decoded as T and written by text.
func workspaceShow[T any](d deps, c *cobra.Command, method string, query url.Values, payload any, what string, text func(io.Writer, T) error, path ...string) error {
	a, cancel, err := d.startWorkspace(c)
	if err != nil {
		return err
	}
	defer cancel()
	raw, err := a.call(method, query, payload, path...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return showAs(a.invocation, raw, what, text)
}

// workspaceDelete deletes one ownerId-addressed resource after confirm.
func (d deps) workspaceDelete(c *cobra.Command, what, question string, path ...string) error {
	if err := d.confirm(c, question); err != nil {
		return err
	}
	a, cancel, err := d.startWorkspace(c)
	if err != nil {
		return err
	}
	defer cancel()
	name := path[len(path)-1]
	if _, err := a.call(http.MethodDelete, nil, nil, path...); err != nil {
		return fmt.Errorf("delete %s %s: %w", what, name, err)
	}
	if a.format == formatText {
		_, _ = fmt.Fprintf(a.out, "Deleted %s %s.\n", what, name)
	}
	return nil
}
