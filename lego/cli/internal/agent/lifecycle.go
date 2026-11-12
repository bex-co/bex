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
	"strings"

	"github.com/spf13/cobra"
)

// archiveCommand is archive or unarchive, on one or more sessions: a failure
// does not stop the others, and the command then exits 1 naming each.
func archiveCommand(d deps, verb, done, short string) *cobra.Command {
	return &cobra.Command{
		Use:   verb + " <ags-id>…",
		Short: short,
		Args:  sessionIDs(100),
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			var (
				raws   [][]byte
				failed []string
			)
			for i, id := range args {
				if inv.ctx.Err() != nil {
					failed = append(failed, "interrupted; not attempted: "+strings.Join(args[i:], " "))
					break
				}
				raw, err := inv.api.post(inv.ctx, request{path: []string{"agent-sessions", id, verb}}, nil)
				if err != nil {
					failed = append(failed, fmt.Sprintf("%s: %v", id, err))
					continue
				}
				raws = append(raws, raw)
				if inv.format == formatText {
					_, _ = fmt.Fprintf(inv.out, "%s %s\n", done, id)
				}
			}
			if inv.format != formatText {
				if err := writeRaw(inv.out, inv.format, rawArray(raws)); err != nil {
					return err
				}
			}
			if len(failed) > 0 {
				return fmt.Errorf("could not %s %d of %d session(s):\n  %s", verb, len(args)-len(raws), len(args), strings.Join(failed, "\n  "))
			}
			return nil
		},
	}
}

func deleteCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <ags-id>",
		Short: "Delete an agent session and its transcript",
		Long: "Deletes the session, its transcript and its files. The pull request it\n" +
			"opened stays on GitHub.",
		Args: sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			if err := d.confirm(c, fmt.Sprintf("Delete agent session %s, its transcript and its files?", args[0])); err != nil {
				return err
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			if _, err := inv.api.send(inv.ctx, http.MethodDelete, request{path: []string{"agent-sessions", args[0]}}, nil); err != nil {
				return fmt.Errorf("delete agent session %s: %w", args[0], err)
			}
			if inv.format == formatText {
				_, _ = fmt.Fprintf(inv.out, "Deleted %s.\n", args[0])
			}
			return nil
		},
	}
}

func tagsCommand(d deps) *cobra.Command {
	var clear bool
	c := &cobra.Command{
		Use:   "tags <ags-id> (<tag>… | --clear)",
		Short: "Replace an agent session's tags",
		Long: "Replaces all of the session's tags with the ones given, as the API does;\n" +
			"--clear removes them all. Tags are lowercase letters, digits, ':', '_'\n" +
			"and '-', at most 20.",
		Args: sessionArgs(-1),
		RunE: func(c *cobra.Command, args []string) error {
			tags := args[1:]
			if clear == (len(tags) > 0) {
				return usageError("give the new tags, or --clear")
			}
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			raw, err := inv.api.send(inv.ctx, http.MethodPut, request{path: []string{"agent-sessions", args[0], "tags"}},
				map[string][]string{"tags": tags})
			if err != nil {
				return fmt.Errorf("set tags of agent session %s: %w", args[0], err)
			}
			s, raw, err := decodeSession(raw)
			if err != nil {
				return err
			}
			return inv.show(s, raw)
		},
	}
	c.Flags().BoolVar(&clear, "clear", false, "remove every tag")
	return c
}

// sessionTree is the ADR102 tree a session belongs to, root first.
type sessionTree struct {
	RootSessionID string    `json:"rootSessionId"`
	Sessions      []session `json:"sessions"`
}

// sessionRead is a command that reads one sub-resource of a session and
// prints it: the API's answer for -o json/yaml, else text.
func sessionRead[T any](d deps, use, short, sub, what string, text func(w io.Writer, v T, id string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  sessionArgs(0),
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			what := fmt.Sprintf("read the %s of agent session %s", what, args[0])
			raw, err := inv.api.getJSON(inv.ctx, nil, "agent-sessions", args[0], sub)
			if err != nil {
				return fmt.Errorf("%s: %w", what, err)
			}
			return showAs(inv, raw, what, func(w io.Writer, v T) error { return text(w, v, args[0]) })
		},
	}
}

func treeCommand(d deps) *cobra.Command {
	return sessionRead(d, "tree <ags-id>", "Show the tree of child sessions a session belongs to", "tree", "tree", writeTree)
}

func costCommand(d deps) *cobra.Command {
	return sessionRead(d, "cost <ags-id>", "Show an agent session's model cost, per turn", "model-cost", "model cost",
		func(w io.Writer, cost modelCost, _ string) error { return writeModelCost(w, cost) })
}

// writeTree prints the tree indented by depth, marking the session asked
// about.
func writeTree(w io.Writer, tree sessionTree, asked string) error {
	for _, s := range tree.Sessions {
		line := strings.Repeat("  ", s.Depth) + s.ID + " · " + s.State
		if cost := costUSD(s); cost != "" {
			line += " · $" + cost
		}
		line += " · " + clip(sessionTitle(s), 60)
		if s.ID == asked {
			line += "  ←"
		}
		_, _ = fmt.Fprintln(w, line)
	}
	return nil
}

// modelSourceBYO is a session on the workspace's own model key, which the
// provider bills directly.
const modelSourceBYO = "byo"

// modelCost is a session's ADR091 model charge, in total and per turn.
type modelCost struct {
	SessionID   string          `json:"sessionId"`
	ModelSource string          `json:"modelSource"`
	CostUSD     string          `json:"costUsd"`
	Requests    int             `json:"requests"`
	Turns       []turnModelCost `json:"turns"`
}

type turnModelCost struct {
	Turn         int      `json:"turn"`
	CostUSD      string   `json:"costUsd"`
	Requests     int      `json:"requests"`
	InputTokens  int64    `json:"inputTokens"`
	OutputTokens int64    `json:"outputTokens"`
	Models       []string `json:"models"`
}

func writeModelCost(w io.Writer, cost modelCost) error {
	_, _ = fmt.Fprintf(w, "%s · $%s · %d request(s) · %s model\n", cost.SessionID, firstNonEmpty(cost.CostUSD, "0"), cost.Requests, cost.ModelSource)
	if cost.ModelSource == modelSourceBYO {
		_, _ = fmt.Fprintln(w, "The workspace's own model key carries this session; its provider bills the requests.")
	}
	if len(cost.Turns) == 0 {
		return nil
	}
	tw := newTable(w)
	_, _ = fmt.Fprintln(tw, "TURN\tCOST\tREQUESTS\tIN TOKENS\tOUT TOKENS\tMODELS")
	for _, t := range cost.Turns {
		_, _ = fmt.Fprintf(tw, "%d\t$%s\t%d\t%d\t%d\t%s\n", t.Turn, t.CostUSD, t.Requests, t.InputTokens, t.OutputTokens, strings.Join(t.Models, ", "))
	}
	return tw.Flush()
}
