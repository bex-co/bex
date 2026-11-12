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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ADR106 D3: a command whose one argument is a session (or an automation)
// may leave it out on a terminal and pick from a list; shell completion
// offers the same list.

// completionBudget bounds one completion's API read, so a slow API never
// stalls the shell.
const completionBudget = 2 * time.Second

type pickItem struct{ value, label string }

// pickKind is what an argument names and how to list candidates for it.
type pickKind struct {
	noun string
	list func(workspaceAPI) ([]pickItem, error)
}

var (
	sessionsKind = pickKind{noun: "session", list: func(a workspaceAPI) ([]pickItem, error) {
		query := url.Values{"limit": {strconv.Itoa(defaultListLimit)}, "mine": {"true"}}
		raw, err := a.call(http.MethodGet, query, nil, "agent-sessions")
		if err != nil {
			return nil, fmt.Errorf("list agent sessions: %w", err)
		}
		var sessions []session
		if err := json.Unmarshal(raw, &sessions); err != nil {
			return nil, err
		}
		items := make([]pickItem, 0, len(sessions))
		for _, s := range sessions {
			label := []string{s.State, clip(sessionTitle(s), 50)}
			if s.Repo != "" {
				label = append(label, s.Repo)
			}
			if !s.CreatedAt.IsZero() {
				label = append(label, s.CreatedAt.Local().Format(minuteLayout))
			}
			items = append(items, pickItem{s.ID, strings.Join(label, " · ")})
		}
		return items, nil
	}}
	automationsKind = pickKind{noun: "automation", list: func(a workspaceAPI) ([]pickItem, error) {
		raw, err := a.call(http.MethodGet, url.Values{"limit": {"100"}}, nil, automationPath()...)
		if err != nil {
			return nil, fmt.Errorf("list automations: %w", err)
		}
		var list automationList
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		items := make([]pickItem, 0, len(list.Automations))
		for _, au := range list.Automations {
			items = append(items, pickItem{au.Name, scheduleWords(firstSchedule(au)) + " · " + automationStatus(au)})
		}
		return items, nil
	}}
)

// pick asks on the terminal which of kind to use.
func (d deps) pick(c *cobra.Command, kind pickKind) (string, error) {
	a, cancel, err := d.startWorkspace(c)
	if err != nil {
		return "", err
	}
	defer cancel()
	items, err := kind.list(a)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", usageError("no %ss to pick from; give one as an argument", kind.noun)
	}
	for i, item := range items {
		_, _ = fmt.Fprintf(a.errOut, "%3d  %s  %s\n", i+1, item.value, item.label)
	}
	n, err := strconv.Atoi(d.ask(a.errOut, fmt.Sprintf("Pick a %s [1-%d]: ", kind.noun, len(items))))
	if err != nil || n < 1 || n > len(items) {
		return "", usageError("no %s picked", kind.noun)
	}
	return items[n-1].value, nil
}

// withPicker lets c's one argument be left out on a terminal, to be picked
// (without a terminal it stays a usage error), and completes it.
func (d deps) withPicker(kind pickKind, c *cobra.Command) *cobra.Command {
	args, run := c.Args, c.RunE
	c.Args = func(cmd *cobra.Command, a []string) error {
		if len(a) == 0 && d.interactive() {
			return nil
		}
		return args(cmd, a)
	}
	c.RunE = func(cmd *cobra.Command, a []string) error {
		if len(a) == 0 {
			picked, err := d.pick(cmd, kind)
			if err != nil {
				return err
			}
			a = []string{picked}
		}
		return run(cmd, a)
	}
	return d.withCompletion(kind, 1, c)
}

// withCompletion completes c's first upTo arguments as kind (-1: all of
// them, for a command that takes several).
func (d deps) withCompletion(kind pickKind, upTo int, c *cobra.Command) *cobra.Command {
	c.ValidArgsFunction = func(cmd *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if upTo >= 0 && len(args) >= upTo {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, completionBudget)
		defer cancel()
		cmd.SetContext(ctx) // the completion process does nothing else
		a, stop, err := d.startWorkspace(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer stop()
		items, err := kind.list(a)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		out := make([]cobra.Completion, 0, len(items))
		for _, item := range items {
			out = append(out, cobra.CompletionWithDesc(item.value, item.label))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
	return c
}
