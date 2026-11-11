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

package branding

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Run-time copy (w2/m263 D4=A, user decision 2026-11-07): upstream output
// that tells the user to run `render <command>` names `bex <command>`
// instead. The upstream RunE still runs unchanged; brandRuntimeOutput only
// wraps it, the way brandLoginOutput does, so three seams carry the rule:
//
//   - every command's returned error (cobra prints it to stderr), e.g.
//     "no workspace set. Use `render workspace set` …";
//   - the stdout of the commands in runtimeStdout, whose output is known to
//     carry next steps. Wrapping every command's stdout would hide the
//     terminal from upstream's *os.File checks (table width, color, ssh's
//     TTY) and would touch pass-through data (logs, exec, ssh);
//   - cobra Deprecated notices, rewritten as metadata in brandTree.
//
// Only command references change: `render` followed by a command the root
// actually has, at a line start or after a backtick, quote or "| ". URLs
// (render.com), file names (render.yaml) and RENDER_* variables never match.
// "Render" as the product name changes only in the exact upstream phrases of
// runtimeProse (w2/073, extending D4=A). A RENDER_* variable is renamed only
// in the phrases of NameBexInputs, and only when the launcher filled it from
// the Bex variable the user set (w32/009).
const runtimeWrapped = "bex.co/runtime-output"

// runtimeProse is upstream run-time copy that names Render as the product the
// user is talking to. Render's own CLI names Render, so the bex CLI names Bex
// (w2/073, user decision D2=A 2026-11-11). Exact phrases only, so API data
// around them never matches.
var runtimeProse = strings.NewReplacer(
	"Successfully logged out of Render.", "Successfully logged out of Bex.",
	"Render will start applying your planned changes.", "Bex will start applying your planned changes.",
	// `logout`'s revoke-failure warning and its environment-key notice (w2/082).
	"revoke your token in the Render dashboard:", "revoke your token in the Bex dashboard:",
	"delete the API key from your Render Dashboard.", "delete the API key from your Bex dashboard.",
)

// Upstream run-time copy that names a variable the launcher filled from a Bex
// input. Each replacer is nil unless that happened in this process.
var (
	// `logout` on stdout (cmd/logout.go).
	apiKeyProse *strings.Replacer
	// The output-format check, which upstream's root PersistentPreRunE prints
	// to stderr itself, before any RunE (pkg/command/outputresolver.go).
	outputProse *strings.Replacer
)

// NameBexInputs makes run-time copy about an environment variable name the
// one the user set. Upstream names its own: a user who signed in with
// BEX_ACCESS_TOKEN was told RENDER_API_KEY "is still set", a variable they
// cannot find to unset (w32/009). filled holds the upstream variables the
// launcher filled from a Bex input, each with that input's name
// (bridge.FilledFromBex); a RENDER_* the user set themselves is absent from
// it and keeps upstream's name. Exact phrases only, as in runtimeProse.
func NameBexInputs(filled map[string]string) {
	apiKeyProse = newInputProse(filled, "RENDER_API_KEY",
		"via the RENDER_API_KEY environment variable.",
		"Note: RENDER_API_KEY is still set in your environment.")
	outputProse = newInputProse(filled, "RENDER_OUTPUT", "invalid RENDER_OUTPUT value:")
}

func newInputProse(filled map[string]string, upstream string, phrases ...string) *strings.Replacer {
	bex := filled[upstream]
	if bex == "" {
		return nil
	}
	pairs := make([]string, 0, 2*len(phrases))
	for _, phrase := range phrases {
		pairs = append(pairs, phrase, strings.ReplaceAll(phrase, upstream, bex))
	}
	return strings.NewReplacer(pairs...)
}

// preRunWrapped marks a root whose PersistentPreRunE brandPreRunOutput wrapped.
const preRunWrapped = "bex.co/pre-run-output"

// brandPreRunOutput carries NameBexInputs to the one message upstream prints
// outside every RunE. Upstream installs the root pre-run during Execute, so
// Refresh calls this; stderr is wrapped for that call alone.
func brandPreRunOutput(root *cobra.Command) {
	run := root.PersistentPreRunE
	if run == nil || !markOnce(root, preRunWrapped) {
		return
	}
	root.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if outputProse == nil {
			return run(c, args)
		}
		stderr := c.ErrOrStderr()
		c.SetErr(proseOutput{Writer: stderr, prose: outputProse})
		defer c.SetErr(stderr)
		return run(c, args)
	}
}

// proseOutput rewrites the exact phrases of prose in each write.
type proseOutput struct {
	io.Writer
	prose *strings.Replacer
}

func (w proseOutput) Write(p []byte) (int, error) {
	return writeBranded(w.Writer, p, []byte(w.prose.Replace(string(p))))
}

// runtimeStdout maps a command path below the root to how its stdout is
// rewritten. Extend it on a pin bump when a new command prints a next step
// (lego/cli/UPSTREAM_RENDER_CLI.md re-diff checklist).
var runtimeStdout = map[string]stdoutMode{
	"ea blueprints create":  nextStepOutput,
	"ea blueprints plan":    nextStepOutput,
	"ea blueprints approve": nextStepOutput,
	"ea blueprints update":  nextStepOutput,
	"logout":                plainOutput,
	// The interactive wizard's success screen ends "Run `render pg get <name>`".
	"postgres create": wizardOutput,
}

type stdoutMode int

const (
	// plainOutput rewrites every command reference: the output is upstream
	// copy only.
	plainOutput stdoutMode = iota + 1
	// nextStepOutput rewrites only the next step: JSON/YAML meta.next, or the
	// line after the text "Next:" title. API data around it stays byte-exact.
	nextStepOutput
	// wizardOutput is plainOutput while the interactive TUI draws and
	// nextStepOutput for --output json/yaml/text. The frames carry API data too
	// (workspace, project and environment names), which only a real
	// `render <command>` reference or a runtimeProse phrase would change.
	wizardOutput
)

func brandRuntimeOutput(root *cobra.Command) {
	walk(root, func(c *cobra.Command) {
		if c.RunE == nil || !markOnce(c, runtimeWrapped) {
			return
		}
		mode := runtimeStdout[strings.TrimPrefix(c.CommandPath(), root.CommandPath()+" ")]
		run := c.RunE
		c.RunE = func(c *cobra.Command, args []string) error {
			if mode != 0 {
				out := c.OutOrStdout()
				c.SetOut(keepTerminal(&runtimeOutput{Writer: out, cmd: c, root: root, mode: mode}, out))
				defer c.SetOut(out)
			}
			return brandError(root, run(c, args))
		}
	})
}

// markOnce records key on c and reports whether this call was the first, so
// a wrapper re-applied by Refresh wraps once.
func markOnce(c *cobra.Command, key string) bool {
	if c.Annotations[key] != "" {
		return false
	}
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations[key] = "1"
	return true
}

// brandError rewrites the command references in err's message. errors.Is
// and errors.As still reach err (exit codes, ErrLogin) through Unwrap.
func brandError(root *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	branded := RewriteCommandRefs(msg, knownCommands(root))
	if branded == msg {
		return err
	}
	return &brandedError{err: err, msg: branded}
}

type brandedError struct {
	err error
	msg string
}

func (e *brandedError) Error() string { return e.msg }
func (e *brandedError) Unwrap() error { return e.err }

type runtimeOutput struct {
	io.Writer
	cmd  *cobra.Command
	root *cobra.Command
	mode stdoutMode
}

// terminalFile is the interface bubbletea (via charmbracelet/x/term) checks
// its output against to size and drive a TUI.
type terminalFile interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// runtimeTerminal keeps a wrapped terminal stdout a terminal: the `postgres
// create` wizard's bubbletea program writes its frames through the wrapper
// but sizes the screen from the file it passes through.
type runtimeTerminal struct {
	*runtimeOutput
	file terminalFile
}

func (t runtimeTerminal) Read(p []byte) (int, error) { return t.file.Read(p) }
func (t runtimeTerminal) Close() error               { return t.file.Close() }
func (t runtimeTerminal) Fd() uintptr                { return t.file.Fd() }

func keepTerminal(w *runtimeOutput, out io.Writer) io.Writer {
	if f, ok := out.(terminalFile); ok {
		return runtimeTerminal{runtimeOutput: w, file: f}
	}
	return w
}

// Write rewrites one upstream write. PrintData emits a whole JSON, YAML or
// text document in a single write, and Println a whole line.
func (w *runtimeOutput) Write(p []byte) (int, error) {
	known := knownCommands(w.root)
	rewrite := func(s string) string {
		s = runtimeProse.Replace(RewriteCommandRefs(s, known))
		if apiKeyProse != nil {
			s = apiKeyProse.Replace(s)
		}
		return s
	}
	format := outputFormat(w.cmd)
	mode := w.mode
	if mode == wizardOutput {
		mode = nextStepOutput
		if format == command.Interactive {
			mode = plainOutput
		}
	}
	var branded []byte
	if mode == plainOutput {
		branded = []byte(rewrite(string(p)))
	} else {
		switch format {
		case command.JSON:
			branded = rewriteJSONNext(p, rewrite)
		case command.YAML:
			branded = rewriteYAMLNext(p, rewrite)
		default:
			branded = []byte(rewriteTextNext(string(p), rewrite))
		}
	}
	return writeBranded(w.Writer, p, branded)
}

func outputFormat(c *cobra.Command) command.Output {
	if f := command.GetFormatFromContext(c.Context()); f != nil {
		return *f
	}
	return command.Interactive
}

// knownCommands reports whether a word names one of root's commands.
func knownCommands(root *cobra.Command) func(string) bool {
	return func(word string) bool {
		for _, c := range root.Commands() {
			if c.Name() == word || c.HasAlias(word) {
				return true
			}
		}
		return false
	}
}

// RewriteCommandRefs turns each `render <command>` reference into
// `bex <command>`. A reference is "render", then a space and a word known
// reports true, at the start of a line (after indentation), or right after
// a backtick, a quote, or "| ". A bare "`render`" also counts.
func RewriteCommandRefs(s string, known func(string) bool) string {
	const word = "render"
	var b strings.Builder
	for i := 0; ; {
		j := strings.Index(s[i:], word)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		j += i
		end := j + len(word)
		b.WriteString(s[i:j])
		if commandRefAt(s, j, end, known) {
			b.WriteString("bex")
		} else {
			b.WriteString(word)
		}
		i = end
	}
}

func commandRefAt(s string, start, end int, known func(string) bool) bool {
	if !refBoundary(s, start) {
		return false
	}
	rest := s[end:]
	if strings.HasPrefix(rest, "`") {
		return start > 0 && s[start-1] == '`'
	}
	if !strings.HasPrefix(rest, " ") {
		return false
	}
	name := rest[1:]
	if n := strings.IndexFunc(name, notCommandRune); n >= 0 {
		name = name[:n]
	}
	return name != "" && known(name)
}

// notCommandRune ends the word after "render": the whole word, hyphens and
// digits included, must name a command, so "render deploys-x" or
// "render ea2" is not a reference to deploys or ea.
func notCommandRune(r rune) bool {
	return r != '-' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func refBoundary(s string, start int) bool {
	if start == 0 {
		return true
	}
	switch s[start-1] {
	case '`', '"', '\'', '\n':
		return true
	}
	// "Usage: " is upstream's usage hint inside a returned error, where the
	// command follows the label on its line (`render psql`'s, w8/255).
	if strings.HasSuffix(s[:start], "| ") || strings.HasSuffix(s[:start], "Usage: ") {
		return true
	}
	line := s[strings.LastIndexByte(s[:start], '\n')+1 : start]
	return strings.TrimLeft(line, " \t") == ""
}

// rewriteTextNext rewrites the line after each "Next:" title — upstream's
// text formatters print the next step there and API data elsewhere.
func rewriteTextNext(s string, rewrite func(string) string) string {
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(ansi.ReplaceAllString(lines[i-1], "")) == "Next:" {
			lines[i] = rewrite(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// rewriteJSONNext rewrites the string at meta.next in a JSON document and
// leaves every other byte as upstream wrote it. Anything else (not JSON, no
// meta.next) passes through unchanged.
func rewriteJSONNext(p []byte, rewrite func(string) string) []byte {
	dec := json.NewDecoder(bytes.NewReader(p))
	type frame struct {
		object    bool
		key       string
		expectKey bool
	}
	var stack []frame
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].expectKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return p
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, frame{object: true, expectKey: true})
			case '[':
				stack = append(stack, frame{})
			default:
				stack = stack[:len(stack)-1]
				valueDone()
			}
			continue
		case string:
			top := len(stack) - 1
			if top >= 0 && stack[top].object && stack[top].expectKey {
				stack[top].key, stack[top].expectKey = t, false
				continue
			}
			if len(stack) == 2 && stack[0].key == "meta" && stack[1].key == "next" {
				return spliceJSONString(p, int(dec.InputOffset()), t, rewrite(t))
			}
		}
		valueDone()
	}
}

// spliceJSONString replaces the encoded string old that ends at end with
// new, encoded the way encoding/json (and so upstream) encodes strings.
func spliceJSONString(p []byte, end int, old, new string) []byte {
	if old == new {
		return p
	}
	oldRaw, err1 := json.Marshal(old)
	newRaw, err2 := json.Marshal(new)
	if err1 != nil || err2 != nil || end < len(oldRaw) || !bytes.Equal(p[end-len(oldRaw):end], oldRaw) {
		return p
	}
	out := make([]byte, 0, len(p)+len(newRaw)-len(oldRaw))
	out = append(out, p[:end-len(oldRaw)]...)
	out = append(out, newRaw...)
	return append(out, p[end:]...)
}

// rewriteYAMLNext rewrites the scalar at meta.next in a YAML document. Only a
// changed document is re-encoded, with upstream's yaml.Marshal indent.
func rewriteYAMLNext(p []byte, rewrite func(string) string) []byte {
	var doc yaml.Node
	if err := yaml.Unmarshal(p, &doc); err != nil || len(doc.Content) != 1 {
		return p
	}
	next := mappingValue(mappingValue(doc.Content[0], "meta"), "next")
	if next == nil || next.Kind != yaml.ScalarNode || next.Tag != "!!str" {
		return p
	}
	branded := rewrite(next.Value)
	if branded == next.Value {
		return p
	}
	next.Value = branded
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(&doc); err != nil || enc.Close() != nil {
		return p
	}
	return buf.Bytes()
}

func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
