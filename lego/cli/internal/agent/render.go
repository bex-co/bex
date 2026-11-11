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
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// format is the resolved --output: JSON and YAML print the API's own answer;
// text (and upstream's interactive default) prints a human summary.
type format string

const (
	formatText format = "text"
	formatJSON format = "json"
	formatYAML format = "yaml"
)

func parseFormat(value string) (format, error) {
	switch strings.ToLower(value) {
	case "", "interactive", "text":
		return formatText, nil
	case "json":
		return formatJSON, nil
	case "yaml":
		return formatYAML, nil
	}
	return "", usageError("--output must be json, yaml or text, not %q", value)
}

// writeRaw prints an API answer (or a JSON array of them) as JSON or YAML.
func writeRaw(w io.Writer, f format, raw []byte) error {
	if f == formatYAML {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			return err
		}
		// Decode stops after one value; the JSON branch refuses what follows.
		if _, err := dec.Token(); err != io.EOF {
			return fmt.Errorf("the API answer has data after its JSON value")
		}
		out, err := yaml.Marshal(yamlValue(value))
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// yamlValue prepares a decoded API answer for yaml.Marshal. Decoded the usual
// way every JSON number is a float64, which yaml.v3 prints in exponent form
// from 1e6 up and which cannot hold every integer above 2^53: a 2 MiB file was
// `size: 2.097152e+06`, and a 16-digit id came out as a different number
// (w32/011). An integer is emitted with the API's own digits instead; a
// number with a fraction or exponent stays a float.
func yamlValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = yamlValue(item)
		}
	case []any:
		for i, item := range v {
			v[i] = yamlValue(item)
		}
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			if f, err := v.Float64(); err == nil {
				return f
			}
			// Outside float64's range: keep the literal, like an integer.
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: v.String()}
	}
	return v
}

// rawArray joins API answers into one JSON array.
func rawArray(items [][]byte) []byte {
	if len(items) == 0 {
		return []byte("[]")
	}
	return append(append([]byte("["), bytes.Join(items, []byte(","))...), ']')
}

func writeSession(w io.Writer, s session) {
	headline := s.ID + " · " + s.State
	if s.Outcome != nil && s.Outcome.Turn > 0 {
		headline += fmt.Sprintf(" (turn %d)", s.Outcome.Turn)
	}
	if s.AgentConfig.Agent != "" {
		headline += " · " + s.AgentConfig.Agent
	}
	_, _ = fmt.Fprintln(w, headline)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(label, value string) {
		if value != "" {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", label, value)
		}
	}
	row("Title", s.Title)
	if s.Repo != "" {
		row("Repo", strings.TrimSpace(s.Repo+" "+branchSuffix(s.Branch)))
	}
	o := s.Outcome
	if o == nil {
		o = &outcome{}
	}
	if o.PullRequest != nil && o.PullRequest.URL != "" {
		row("PR", o.PullRequest.URL)
	} else {
		row("PR", s.PRURL)
	}
	row("Delivery", o.DeliveryNote)
	if c := o.Cost; c != nil {
		switch {
		case c.TotalUSD != "":
			row("Cost", fmt.Sprintf("$%s (model $%s, compute $%s)", c.TotalUSD, c.ModelUSD, c.ComputeUSD))
		case c.ModelUSD != "": // a server from before computeUSD
			row("Cost", "$"+c.ModelUSD)
		}
	}
	if code := firstNonEmpty(o.FailureReasonCode, s.FailureReasonCode); code != "" || s.FailureReason != "" {
		row("Failure", strings.TrimSpace(strings.TrimSuffix(code+": "+s.FailureReason, ": ")))
	}
	_ = tw.Flush()
	if o.Summary != "" {
		_, _ = fmt.Fprintf(w, "\nSummary\n%s\n", indent(o.Summary))
	}
	if len(o.Files) > 0 {
		_, _ = fmt.Fprintln(w, "\nFiles")
		for _, f := range o.Files {
			_, _ = fmt.Fprintf(w, "  %s  %s  %s\n", f.ID, f.Name, humanBytes(f.Size))
		}
	}
	if len(o.Result) > 0 && string(o.Result) != "null" {
		var buf bytes.Buffer
		if json.Indent(&buf, o.Result, "", "  ") == nil {
			_, _ = fmt.Fprintf(w, "\nResult\n%s\n", indent(buf.String()))
		}
	}
}

func writeSessionList(w io.Writer, sessions []session) {
	if len(sessions) == 0 {
		_, _ = fmt.Fprintln(w, "No agent sessions.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tSTATE\tAGENT\tREPO\tTITLE\tCREATED")
	for _, s := range sessions {
		created := ""
		if !s.CreatedAt.IsZero() {
			created = s.CreatedAt.Local().Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.State, s.AgentConfig.Agent, s.Repo, clip(s.Title, 48), created)
	}
	_ = tw.Flush()
}

func writeAttachments(w io.Writer, files []attachment) {
	if len(files) == 0 {
		_, _ = fmt.Fprintln(w, "No files.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTURN\tDIRECTION\tNAME\tSIZE")
	for _, f := range files {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", f.ID, f.Turn, f.Direction, f.Name, humanBytes(f.Size))
	}
	_ = tw.Flush()
}

// uiPart is the AI SDK UI message part a transcript row carries (ADR051).
type uiPart struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ToolName  string          `json:"toolName"`
	State     string          `json:"state"`
	Input     json.RawMessage `json:"input"`
	ErrorText string          `json:"errorText"`
	Filename  string          `json:"filename"`
}

// partPrinter renders transcript parts as terminal text, once each. A row's
// identity is (turn, partIndex); a partial row is the item the stream ended
// inside of, superseded by its complete copy under a later seq (ADR051), so
// add holds it and flush shows it only if no complete copy arrived.
type partPrinter struct {
	w         io.Writer
	jsonLines bool
	reasoning bool
	prompts   map[int]string
	printed   map[partKey]bool
	held      []transcriptPart
	lastTurn  int
}

type partKey struct {
	turn  int
	index int64
}

func newPartPrinter(w io.Writer, jsonLines, reasoning bool) *partPrinter {
	return &partPrinter{w: w, jsonLines: jsonLines, reasoning: reasoning, prompts: map[int]string{}, printed: map[partKey]bool{}}
}

func (p *partPrinter) turns(turns []transcriptTurn) {
	for _, t := range turns {
		p.prompts[t.Turn] = t.Prompt
	}
}

// add prints a complete row and holds a partial one.
func (p *partPrinter) add(part transcriptPart) {
	if part.Partial {
		p.held = append(p.held, part)
		return
	}
	p.print(part)
}

// flush prints the held partial rows that no complete copy replaced.
func (p *partPrinter) flush() {
	for _, part := range p.held {
		p.print(part)
	}
	p.held = nil
}

func (p *partPrinter) print(part transcriptPart) {
	key := partKey{part.Turn, part.PartIndex}
	if p.printed[key] {
		return
	}
	p.printed[key] = true
	if p.jsonLines {
		_, _ = fmt.Fprintf(p.w, "%s\n", compactJSON(part.raw))
		return
	}
	if part.Turn != p.lastTurn {
		p.lastTurn = part.Turn
		_, _ = fmt.Fprintf(p.w, "── turn %d ──\n", part.Turn)
		if prompt := strings.TrimSpace(p.prompts[part.Turn]); prompt != "" {
			_, _ = fmt.Fprintf(p.w, "> %s\n", clip(strings.ReplaceAll(prompt, "\n", " "), 200))
		}
	}
	var ui uiPart
	if json.Unmarshal(part.Part, &ui) != nil {
		return
	}
	switch {
	case ui.Type == "text":
		if text := strings.TrimRight(ui.Text, "\n"); text != "" {
			_, _ = fmt.Fprintln(p.w, text)
		}
	case ui.Type == "reasoning":
		if p.reasoning && strings.TrimSpace(ui.Text) != "" {
			_, _ = fmt.Fprintf(p.w, "(thinking) %s\n", strings.TrimSpace(ui.Text))
		}
	case ui.Type == "dynamic-tool" || strings.HasPrefix(ui.Type, "tool-"):
		name := ui.ToolName
		if name == "" {
			name = strings.TrimPrefix(ui.Type, "tool-")
		}
		line := "▸ " + name
		if summary := inputSummary(ui.Input); summary != "" {
			line += " " + summary
		}
		switch ui.State {
		case "output-error":
			line += " — error: " + clip(firstLine(ui.ErrorText), 120)
		case "output-denied":
			line += " — denied"
		}
		_, _ = fmt.Fprintln(p.w, line)
	case ui.Type == "file":
		_, _ = fmt.Fprintf(p.w, "file: %s\n", ui.Filename)
	}
}

// inputSummary is the one argument that says what a tool call did.
func inputSummary(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var fields map[string]any
	if json.Unmarshal(raw, &fields) == nil {
		for _, key := range []string{"command", "file_path", "path", "url", "pattern", "query"} {
			if value, ok := fields[key].(string); ok && value != "" {
				return clip(strings.Join(strings.Fields(value), " "), 120)
			}
		}
	}
	return clip(string(compactJSON(raw)), 120)
}

func compactJSON(raw []byte) []byte {
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return raw
	}
	return buf.Bytes()
}

func branchSuffix(branch string) string {
	if branch == "" {
		return ""
	}
	return "@ " + branch
}

func indent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// clip shortens text to at most n runes, marking the cut.
func clip(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n-1]) + "…"
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
