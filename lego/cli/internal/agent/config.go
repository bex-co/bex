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
		Name string `json:"name"`
	} `json:"secrets"`
}

func secretsCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:     "secrets",
		Aliases: []string{"secret"},
		Short:   "Manage the workspace's agent secrets (values are write-only)",
		Long: "Workspace agent secrets reach a session as env vars when it names them\n" +
			"(`run --secret NAME`, an automation's --secret). Reads show names only;\n" +
			"a value is set from stdin, a hidden prompt or --from-env, never an argument.",
	}
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List the workspace's agent secret names",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "list agent secrets", func(w io.Writer, list agentSecretList) error {
				if len(list.Secrets) == 0 {
					_, _ = fmt.Fprintln(w, "No agent secrets.")
				}
				for _, s := range list.Secrets {
					_, _ = fmt.Fprintln(w, s.Name)
				}
				return nil
			}, secretPath()...)
		},
	})
	var fromEnv string
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
			return workspaceShow(d, c, http.MethodPut, nil, map[string]string{"value": value}, "set agent secret "+args[0],
				func(w io.Writer, _ struct{}) error {
					_, _ = fmt.Fprintf(w, "Set agent secret %s.\n", args[0])
					return nil
				}, secretPath(args[0])...)
		},
	}
	set.Flags().StringVar(&fromEnv, "from-env", "", "read the value from this environment variable")
	root.AddCommand(set, &cobra.Command{
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

// agentEnvironment is the workspace agent environment, as text output needs it.
type agentEnvironment struct {
	NetworkTier                   string   `json:"networkTier"`
	CustomHosts                   []string `json:"customHosts"`
	DefaultModel                  *string  `json:"defaultModel"`
	ModelSource                   string   `json:"modelSource"`
	ModelKeySet                   bool     `json:"modelKeySet"`
	AgentModelMonthlySpendCapUSD  *float64 `json:"agentModelMonthlySpendCapUsd"`
	AgentModelSpendMonthToDateUSD *float64 `json:"agentModelSpendMonthToDateUsd"`
	TrustTier                     *string  `json:"trustTier"`
}

func envCommand(d deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "env",
		Short: "Show the workspace's agent environment: network, default model, spend cap",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return workspaceShow(d, c, http.MethodGet, nil, nil, "read the agent environment", writeEnvironment, "agent-environment")
		},
	}
	var (
		tier, model, spendCap string
		hosts                 []string
		noHosts, keyStdin     bool
		clearKey              bool
	)
	set := &cobra.Command{
		Use:   "set [flags]",
		Short: "Change the workspace's agent environment (admins)",
		Long: "Changes only what the flags name. --spend-cap takes USD or `" + spendCapPlanDefault + "` (the\n" +
			"plan's cap). The workspace's own model key is read from stdin or a hidden\n" +
			"prompt with --model-key-stdin, never from an argument.",
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
	f.StringVar(&spendCap, "spend-cap", "", "monthly agent-model spend cap in USD, or "+spendCapPlanDefault)
	f.BoolVar(&keyStdin, "model-key-stdin", false, "set the workspace's own model API key, read from stdin")
	f.BoolVar(&clearKey, "clear-model-key", false, "remove the workspace's own model API key")
	root.AddCommand(set)
	return root
}

func writeEnvironment(w io.Writer, e agentEnvironment) error {
	tw := newTable(w)
	tw.row("Network tier", e.NetworkTier)
	tw.row("Custom hosts", strings.Join(e.CustomHosts, ", "))
	if e.DefaultModel != nil {
		tw.row("Default model", *e.DefaultModel)
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
	} `json:"agents"`
}

func modelsCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Show the agents this workspace can run and whether it is ready",
		Args:  cobra.NoArgs,
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
	_, _ = fmt.Fprintln(tw, "AGENT\tNAME\tREADY")
	for _, a := range caps.Agents {
		ready := "yes"
		if !a.ModelKeyReady {
			ready = "no: needs the workspace's own model key (`bex agent env set --model-key-stdin`)"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", a.ID, a.Label, ready)
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
