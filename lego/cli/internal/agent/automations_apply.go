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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// A definition is an automation as a file holds it: exactly the REST create
// body (ADR106 D7), so `export` output is valid `apply` input and nothing
// here is a CLI-only schema.
var definitionKeys = []string{"name", "enabled", "triggers", "template"}

// maxDefinitionBytes bounds one definition file (or stdin).
const maxDefinitionBytes = 8 << 20

type definition struct {
	source string // where the document came from, for messages
	name   string
	body   map[string]any
}

type planAction string

const (
	actionCreate    planAction = "create"
	actionUpdate    planAction = "update"
	actionUnchanged planAction = "unchanged"
)

func (a planAction) verb(preview bool) string {
	switch {
	case a == actionUnchanged:
		return string(a)
	case preview:
		return "will " + string(a)
	}
	return string(a) + "d"
}

// planned is what apply does with one definition.
type planned struct {
	Name    string          `json:"name"`
	Action  planAction      `json:"action"`
	Changes []string        `json:"changes,omitempty"`
	Error   string          `json:"error,omitempty"`
	Result  json.RawMessage `json:"automation,omitempty"`

	def     definition
	version any
	diffs   []string // "path: have → want", for --dry-run
	warning string
}

func automationApplyCommand(d deps) *cobra.Command {
	var (
		files  []string
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "apply -f FILE|DIR|-",
		Short: "Create or update automations from files",
		Long: "Reads automations from YAML or JSON files (several YAML documents per\n" +
			"file; a directory's *.yaml, *.yml and *.json), each the REST body:\n" +
			"name, enabled, triggers, template. An automation is matched by name:\n" +
			"missing ones are created, changed ones updated, the rest left alone.\n\n" +
			"apply compares only the fields a file sets (the API fills defaults such\n" +
			"as the title); to clear a field, set it empty. apply never deletes: an\n" +
			"automation missing from the files stays. --dry-run prints the plan and\n" +
			"each changed field without writing.",
		Example: "  bex agent automations apply -f .bex/automations/ --dry-run\n" +
			"  bex agent automations export --all | bex agent automations apply -f - --dry-run",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if len(files) == 0 {
				return usageError("give the definitions with -f FILE, DIR or - (stdin)")
			}
			defs, err := readDefinitions(d, files)
			if err != nil {
				return err
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			stored, err := a.allAutomations()
			if err != nil {
				return err
			}
			byName := map[string]map[string]any{}
			for _, item := range stored {
				if name, ok := item["name"].(string); ok {
					byName[name] = item
				}
			}
			plan := make([]*planned, 0, len(defs))
			writes := 0
			for _, def := range defs {
				p := planDefinition(def, byName[def.name])
				if p.Action != actionUnchanged {
					writes++
				}
				plan = append(plan, p)
			}
			if dryRun || writes == 0 {
				return a.printPlan(plan, true)
			}
			if a.format == formatText && d.interactive() && !confirmed(c) {
				_ = a.printPlan(plan, true)
			}
			if err := d.confirm(c, fmt.Sprintf("Apply %d change(s)?", writes)); err != nil {
				return err
			}
			failed := 0
			for _, p := range plan {
				if err := a.applyPlanned(p); err != nil {
					p.Error = err.Error()
					failed++
				}
			}
			if err := a.printPlan(plan, false); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d automation(s) could not be applied", failed, writes)
			}
			return nil
		},
	}
	c.Flags().StringArrayVarP(&files, "file", "f", nil, "definition file, directory, or - for stdin; repeatable")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and the changed fields; write nothing")
	return c
}

func automationExportCommand(d deps) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "export (<automation>… | --all)",
		Short: "Print automations as definitions `apply` reads",
		Long: "Prints each automation's name, enabled, triggers and template as YAML\n" +
			"documents (-o json: a JSON array). Read-only fields are dropped, so\n" +
			"`export | apply -f -` changes nothing.",
		RunE: func(c *cobra.Command, args []string) error {
			if all == (len(args) > 0) {
				return usageError("give automations to export, or --all")
			}
			a, cancel, err := d.startWorkspace(c)
			if err != nil {
				return err
			}
			defer cancel()
			var stored []map[string]any
			if all {
				if stored, err = a.allAutomations(); err != nil {
					return err
				}
			}
			for _, ref := range args {
				raw, err := a.getAutomation(ref)
				if err != nil {
					return err
				}
				var item map[string]any
				if err := decodeExact(raw, &item); err != nil {
					return fmt.Errorf("get automation %s: %w", ref, err)
				}
				stored = append(stored, item)
			}
			defs := make([]map[string]any, 0, len(stored))
			for _, item := range stored {
				def := map[string]any{}
				for _, key := range definitionKeys {
					if v, ok := item[key]; ok {
						def[key] = v
					}
				}
				defs = append(defs, def)
			}
			if a.format == formatJSON {
				return writeValue(a.out, formatJSON, defs)
			}
			return writeDefinitionsYAML(a.out, defs)
		},
	}
	c.Flags().BoolVar(&all, "all", false, "every automation in the workspace")
	return c
}

// allAutomations reads every automation in the workspace, numbers kept exact.
func (a workspaceAPI) allAutomations() ([]map[string]any, error) {
	var out []map[string]any
	seen := map[string]bool{}
	page := pageFlags{limit: 200}
	for {
		raw, err := a.call(http.MethodGet, page.query(), nil, automationPath()...)
		if err != nil {
			return nil, fmt.Errorf("list automations: %w", err)
		}
		var list struct {
			Automations []map[string]any `json:"automations"`
			NextCursor  string           `json:"nextCursor"`
		}
		if err := decodeExact(raw, &list); err != nil {
			return nil, fmt.Errorf("list automations: %w", err)
		}
		out = append(out, list.Automations...)
		if list.NextCursor == "" || seen[list.NextCursor] {
			return out, nil
		}
		seen[list.NextCursor] = true
		page.cursor = list.NextCursor
	}
}

// writeDefinitionsYAML prints defs as YAML documents with the keys in
// definition order, so a file diff stays readable.
func writeDefinitionsYAML(w io.Writer, defs []map[string]any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	for _, def := range defs {
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, key := range definitionKeys {
			v, ok := def[key]
			if !ok {
				continue
			}
			var value yaml.Node
			if err := value.Encode(yamlValue(v)); err != nil {
				return err
			}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &value)
		}
		if err := enc.Encode(node); err != nil {
			return err
		}
	}
	return enc.Close()
}

// readDefinitions reads every document of the given files, directories and
// stdin, refusing a document that is not a definition before anything is
// sent.
func readDefinitions(d deps, sources []string) ([]definition, error) {
	var defs []definition
	seen := map[string]string{}
	for _, source := range sources {
		paths := []string{source}
		if source != "-" {
			info, err := os.Stat(source)
			if err != nil {
				return nil, usageError("-f %s: %v", source, err)
			}
			if info.IsDir() {
				if paths, err = definitionFiles(source); err != nil {
					return nil, err
				}
			}
		}
		for _, path := range paths {
			data, err := readPathOrStdin(d, path, maxDefinitionBytes)
			if err != nil {
				return nil, usageError("-f %s: %v", path, err)
			}
			docs, err := parseDocuments(path, data)
			if err != nil {
				return nil, err
			}
			for _, def := range docs {
				if prior, ok := seen[def.name]; ok {
					return nil, usageError("automation %q is defined twice: %s and %s", def.name, prior, def.source)
				}
				seen[def.name] = def.source
				defs = append(defs, def)
			}
		}
	}
	if len(defs) == 0 {
		return nil, usageError("no automation definitions found in %s", strings.Join(sources, ", "))
	}
	return defs, nil
}

// definitionFiles is a directory's definition files, in name order.
func definitionFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, usageError("-f %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".yaml", ".yml", ".json":
			if !e.IsDir() {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	return out, nil
}

// parseDocuments reads YAML documents (JSON is YAML) and turns each into the
// JSON value the API will receive.
func parseDocuments(source string, data []byte) ([]definition, error) {
	var out []definition
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for n := 1; ; n++ {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		where := source
		if n > 1 {
			where = fmt.Sprintf("%s (document %d)", source, n)
		}
		if err != nil {
			return nil, usageError("%s: %v", where, err)
		}
		if doc == nil {
			continue
		}
		value, err := jsonValue(doc)
		body, ok := value.(map[string]any)
		if err != nil || !ok {
			return nil, usageError("%s: an automation definition is a mapping of %s", where, strings.Join(definitionKeys, ", "))
		}
		for key := range body {
			if !slices.Contains(definitionKeys, key) {
				return nil, usageError("%s: %q is not part of a definition (only %s; `export` prints that shape)", where, key, strings.Join(definitionKeys, ", "))
			}
		}
		name, _ := body["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, usageError("%s: the definition has no name", where)
		}
		out = append(out, definition{source: where, name: name, body: body})
	}
}

// jsonValue is v as encoding/json decodes its JSON, so a YAML document and
// an API answer (exact numbers included) compare like for like.
func jsonValue(v any) (any, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	return out, json.Unmarshal(encoded, &out)
}

// planDefinition decides what apply does with def, given the stored
// automation of that name (nil when there is none).
func planDefinition(def definition, stored map[string]any) *planned {
	p := &planned{Name: def.name, def: def, Action: actionCreate}
	if stored == nil {
		return p
	}
	normalized, _ := jsonValue(stored)
	have, _ := normalized.(map[string]any)
	p.version = have["version"]
	for _, key := range definitionKeys {
		want, ok := def.body[key]
		if !ok || key == "name" {
			continue
		}
		if key == "enabled" && want == true && have["enabled"] == false {
			if reason, _ := have["pausedReason"].(string); reason != "" {
				p.warning = fmt.Sprintf("paused by the platform (%s): `bex agent automations resume %s` once fixed", reason, def.name)
				continue
			}
		}
		if diffs := diffValue(key, have[key], want); len(diffs) > 0 {
			p.Changes = append(p.Changes, key)
			p.diffs = append(p.diffs, diffs...)
		}
	}
	p.Action = actionUnchanged
	if len(p.Changes) > 0 {
		p.Action = actionUpdate
	}
	return p
}

// applyPlanned performs one planned write and records the API's answer.
func (a workspaceAPI) applyPlanned(p *planned) error {
	var (
		raw []byte
		err error
	)
	switch p.Action {
	case actionCreate:
		if raw, err = a.call(http.MethodPost, nil, p.def.body, automationPath()...); err != nil {
			return fmt.Errorf("create automation %s: %w", p.Name, err)
		}
	case actionUpdate:
		patch := map[string]any{}
		for _, key := range p.Changes {
			patch[key] = p.def.body[key]
		}
		if raw, err = a.patchAutomation(p.Name, p.version, patch); err != nil {
			return err
		}
	default:
		return nil
	}
	p.Result = raw
	return nil
}

func (a workspaceAPI) printPlan(plan []*planned, preview bool) error {
	if a.format != formatText {
		return writeValue(a.out, a.format, plan)
	}
	for _, p := range plan {
		line := p.Action.verb(preview) + " " + p.Name
		if p.Error != "" {
			line = fmt.Sprintf("failed %s: %s", p.Name, p.Error)
		}
		_, _ = fmt.Fprintln(a.out, line)
		if preview {
			for _, diff := range p.diffs {
				_, _ = fmt.Fprintf(a.out, "  %s\n", diff)
			}
		}
		if p.warning != "" {
			_, _ = fmt.Fprintf(a.out, "  note: %s\n", p.warning)
		}
	}
	return nil
}

// diffValue lists where want differs from have, looking only at what want
// sets: a field the file leaves out is the API's to fill.
func diffValue(path string, have, want any) []string {
	switch w := want.(type) {
	case map[string]any:
		h, _ := have.(map[string]any)
		var out []string
		for _, key := range slices.Sorted(maps.Keys(w)) {
			out = append(out, diffValue(path+"."+key, h[key], w[key])...)
		}
		return out
	case []any:
		h, _ := have.([]any)
		if len(h) == len(w) && len(w) > 0 {
			var out []string
			for i := range w {
				out = append(out, diffValue(path+"["+strconv.Itoa(i)+"]", h[i], w[i])...)
			}
			return out
		}
	}
	if sameValue(have, want) {
		return nil
	}
	return []string{fmt.Sprintf("%s: %s → %s", path, shortJSON(have), shortJSON(want))}
}

// sameValue is equality as the API means it: empty equals absent, and two
// times are equal when they are the same instant.
func sameValue(have, want any) bool {
	if empty(have) && empty(want) {
		return true
	}
	if hs, ok := have.(string); ok {
		if ws, ok := want.(string); ok {
			ht, herr := time.Parse(time.RFC3339, hs)
			wt, werr := time.Parse(time.RFC3339, ws)
			if herr == nil && werr == nil {
				return ht.Equal(wt)
			}
		}
	}
	return reflect.DeepEqual(have, want)
}

func empty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

func shortJSON(v any) string {
	if v == nil {
		return "(none)"
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return clip(string(encoded), 80)
}
