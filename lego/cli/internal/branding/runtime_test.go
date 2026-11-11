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
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func knownWords(words ...string) func(string) bool {
	return func(w string) bool {
		for _, k := range words {
			if k == w {
				return true
			}
		}
		return false
	}
}

func TestRewriteCommandRefs(t *testing.T) {
	known := knownWords("ea", "workspace", "services", "deploys", "login")
	for in, want := range map[string]string{
		"render ea blueprints approve exs-1":                      "bex ea blueprints approve exs-1",
		"  render services list":                                  "  bex services list",
		"no workspace set. Use `render workspace set` to set one": "no workspace set. Use `bex workspace set` to set one",
		`then re-plan with "render ea blueprints plan exs-1".`:    `then re-plan with "bex ea blueprints plan exs-1".`,
		"run 'render workspace set <name|ID>'":                    "run 'bex workspace set <name|ID>'",
		"echo x | render services":                                "echo x | bex services",
		"`render services` and `render deploys` commands":         "`bex services` and `bex deploys` commands",
		"Run `render login` to log back in.":                      "Run `bex login` to log back in.",
		"line one\nrender deploys list":                           "line one\nbex deploys list",
		"the `render` binary":                                     "the `bex` binary",
		// Not command references: left byte-exact.
		"update \"render.yaml\" and push":         "update \"render.yaml\" and push",
		"https://render.com/docs":                 "https://render.com/docs",
		"RENDER_API_KEY and RENDER_OUTPUT":        "RENDER_API_KEY and RENDER_OUTPUT",
		"Render will start applying your changes": "Render will start applying your changes",
		"service render-x is live":                "service render-x is live",
		"render-x":                                "render-x",
		"\"render-x\"":                            "\"render-x\"",
		"we render services nightly":              "we render services nightly",
		"render unknowncmd":                       "render unknowncmd",
		"render ea2":                              "render ea2",
		"render deploys-x is live":                "render deploys-x is live",
		"render ea_x":                             "render ea_x",
		"render deploys-":                         "render deploys-",
		"render":                                  "render",
		"`render.yaml`":                           "`render.yaml`",
		"prerender ea":                            "prerender ea",
		"render workspaces is not a command here": "render workspaces is not a command here",
	} {
		if got := RewriteCommandRefs(in, known); got != want {
			t.Errorf("RewriteCommandRefs(%q) = %q, want %q", in, got, want)
		}
	}
}

func rewriteBex(s string) string {
	return RewriteCommandRefs(s, knownWords("ea", "services", "deploys"))
}

func TestRewriteJSONNextTouchesOnlyMetaNext(t *testing.T) {
	doc := map[string]any{
		"data": map[string]any{
			"blueprint": map[string]any{"name": "render-x", "path": "render.yaml", "next": "render ea blueprints plan x"},
			"notices":   []any{map[string]any{"message": "`render services` <&> RENDER_API_KEY"}},
		},
		"meta": map[string]any{"next": "render ea blueprints approve exs-1 <now> & \"quoted\""},
	}
	in, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out := rewriteJSONNext(in, rewriteBex)
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if next := got["meta"].(map[string]any)["next"]; next != "bex ea blueprints approve exs-1 <now> & \"quoted\"" {
		t.Fatalf("meta.next = %q", next)
	}
	want := bytes.Replace(in, []byte(`"render ea blueprints approve exs-1`), []byte(`"bex ea blueprints approve exs-1`), 1)
	if !bytes.Equal(out, want) {
		t.Fatalf("bytes outside meta.next changed:\n got %s\nwant %s", out, want)
	}
	for _, untouched := range [][]byte{[]byte("not json render ea x"), []byte(`{"meta":{"next":null}}`), []byte(`{"data":{"meta":{"next":"render ea x"}}}`)} {
		if out := rewriteJSONNext(untouched, rewriteBex); !bytes.Equal(out, untouched) {
			t.Errorf("rewriteJSONNext(%s) = %s, want unchanged", untouched, out)
		}
	}
}

func TestRewriteYAMLNextTouchesOnlyMetaNext(t *testing.T) {
	doc := map[string]any{
		"data": map[string]any{"blueprint": map[string]any{"name": "render-x", "path": "render.yaml"}, "next": "render ea x"},
		"meta": map[string]any{"next": "Render will start applying your planned changes. Check `render services` and `render deploys` commands.", "syncRequested": true},
	}
	in, err := yaml.Marshal(doc) // what upstream PrintData writes
	if err != nil {
		t.Fatal(err)
	}
	out := rewriteYAMLNext(in, rewriteBex)
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	doc["meta"].(map[string]any)["next"] = "Render will start applying your planned changes. Check `bex services` and `bex deploys` commands."
	wantBytes, _ := yaml.Marshal(doc)
	if !bytes.Equal(out, wantBytes) {
		t.Fatalf("YAML output:\n%s\nwant:\n%s", out, wantBytes)
	}
	plain := []byte("data:\n    next: render ea x\nmeta:\n    next: done\n")
	if out := rewriteYAMLNext(plain, rewriteBex); !bytes.Equal(out, plain) {
		t.Fatalf("a document without a reference in meta.next changed: %s", out)
	}
}

func TestRewriteTextNextTouchesOnlyTheNextLine(t *testing.T) {
	in := "Blueprint render-x (exs-1)\n  `render services` in a notice\n\n\x1b[1mNext:\x1b[0m\n  render ea blueprints approve exs-1\n"
	want := "Blueprint render-x (exs-1)\n  `render services` in a notice\n\n\x1b[1mNext:\x1b[0m\n  bex ea blueprints approve exs-1\n"
	if got := rewriteTextNext(in, rewriteBex); got != want {
		t.Fatalf("rewriteTextNext = %q, want %q", got, want)
	}
}

func TestBrandRuntimeOutputWrapsErrorsAndListedStdout(t *testing.T) {
	sentinel := errors.New("no workspace set. Use `render workspace set` to set a workspace")
	root := &cobra.Command{Use: "bex"}
	workspace := &cobra.Command{Use: "workspace"}
	current := &cobra.Command{Use: "current", RunE: func(*cobra.Command, []string) error { return sentinel }}
	workspace.AddCommand(current)
	logout := &cobra.Command{Use: "logout", RunE: func(c *cobra.Command, _ []string) error {
		command.Println(c, "Run `render login` to log back in.")
		return nil
	}}
	other := &cobra.Command{Use: "logs", RunE: func(c *cobra.Command, _ []string) error {
		command.Println(c, "render workspace set happened in my app log")
		return nil
	}}
	login := &cobra.Command{Use: "login"}
	root.AddCommand(workspace, logout, other, login)
	brandRuntimeOutput(root)
	brandRuntimeOutput(root) // idempotent

	err := current.RunE(current, nil)
	if err == nil || err.Error() != "no workspace set. Use `bex workspace set` to set a workspace" || !errors.Is(err, sentinel) {
		t.Fatalf("error = %v (Is sentinel: %v)", err, errors.Is(err, sentinel))
	}
	var out bytes.Buffer
	logout.SetOut(&out)
	if err := logout.RunE(logout, nil); err != nil || out.String() != "Run `bex login` to log back in.\n" {
		t.Fatalf("logout = %q, %v", out.String(), err)
	}
	if _, wrapped := logout.OutOrStdout().(*runtimeOutput); wrapped {
		t.Fatal("stdout wrapper must be removed after the command runs")
	}
	out.Reset()
	other.SetOut(&out)
	if err := other.RunE(other, nil); err != nil || out.String() != "render workspace set happened in my app log\n" {
		t.Fatalf("unlisted command stdout changed: %q", out.String())
	}
}

func TestRuntimeOutputFollowsTheOutputFormat(t *testing.T) {
	root := &cobra.Command{Use: "bex"}
	root.AddCommand(&cobra.Command{Use: "ea"})
	for format, write := range map[command.Output]string{
		command.JSON: `{"meta":{"next":"render ea blueprints approve exs-1"}}`,
		command.YAML: "meta:\n    next: render ea blueprints approve exs-1\n",
		command.TEXT: "Next:\n  render ea blueprints approve exs-1\n",
	} {
		c := &cobra.Command{Use: "create"}
		c.SetContext(command.SetFormatInContext(context.Background(), &format))
		var out bytes.Buffer
		w := &runtimeOutput{Writer: &out, cmd: c, root: root, mode: nextStepOutput}
		if n, err := w.Write([]byte(write)); err != nil || n != len(write) {
			t.Fatalf("%s: Write = %d, %v", format, n, err)
		}
		if !bytes.Contains(out.Bytes(), []byte("bex ea blueprints approve exs-1")) || bytes.Contains(out.Bytes(), []byte("render ea")) {
			t.Errorf("%s: output = %q", format, out.String())
		}
	}
}

// w2/073 (user decision D2=A, 2026-11-11): run-time copy that names Render as
// the product reads Bex, in its exact upstream phrases only.
func TestRuntimeOutputRebrandsProductProse(t *testing.T) {
	root := &cobra.Command{Use: "bex"}
	root.AddCommand(&cobra.Command{Use: "services"}, &cobra.Command{Use: "deploys"})
	text := command.TEXT
	c := &cobra.Command{Use: "approve"}
	c.SetContext(command.SetFormatInContext(context.Background(), &text))
	for _, tc := range []struct {
		mode    stdoutMode
		in, out string
	}{
		{plainOutput, "Successfully logged out of Render.\n", "Successfully logged out of Bex.\n"},
		{nextStepOutput,
			"Next:\n  Render will start applying your planned changes. Monitor progress at the provided dashboard URL, or check the affected services with `render services` and `render deploys` commands.\n",
			"Next:\n  Bex will start applying your planned changes. Monitor progress at the provided dashboard URL, or check the affected services with `bex services` and `bex deploys` commands.\n"},
		{plainOutput,
			"Warning: something went wrong revoking your CLI token: boom. Your local credentials have been cleared, but you'll need to revoke your token in the Render dashboard: https://dashboard.bex.co/settings#cli-tokens\n",
			"Warning: something went wrong revoking your CLI token: boom. Your local credentials have been cleared, but you'll need to revoke your token in the Bex dashboard: https://dashboard.bex.co/settings#cli-tokens\n"},
		{plainOutput, "To revoke access, delete the API key from your Render Dashboard.\n", "To revoke access, delete the API key from your Bex dashboard.\n"},
		// Data that merely mentions Render stays byte-exact.
		{plainOutput, "Render-hosted app my-render-db\n", "Render-hosted app my-render-db\n"},
	} {
		var out bytes.Buffer
		w := &runtimeOutput{Writer: &out, cmd: c, root: root, mode: tc.mode}
		if _, err := w.Write([]byte(tc.in)); err != nil || out.String() != tc.out {
			t.Errorf("Write(%q) = %q, %v; want %q", tc.in, out.String(), err, tc.out)
		}
	}
}

// The `postgres create` wizard's success screen ("Run `render pg get <name>`")
// is rewritten while the TUI draws; --output json keeps its API data exact and
// rewrites only meta.next.
func TestWizardOutputRewritesTheInteractiveScreenOnly(t *testing.T) {
	root := &cobra.Command{Use: "bex"}
	root.AddCommand(&cobra.Command{Use: "pg"})
	for _, tc := range []struct {
		format  command.Output
		in, out string
	}{
		{command.Interactive,
			"\nSuccess!\n\nDatabase my-db (dpg-1) was created.\n\nRun `render pg get my-db` to check if it's ready yet.\n",
			"\nSuccess!\n\nDatabase my-db (dpg-1) was created.\n\nRun `bex pg get my-db` to check if it's ready yet.\n"},
		{command.JSON,
			`{"name":"render pg note","meta":{"next":"render pg get my-db"}}`,
			`{"name":"render pg note","meta":{"next":"bex pg get my-db"}}`},
	} {
		c := &cobra.Command{Use: "create"}
		format := tc.format
		c.SetContext(command.SetFormatInContext(context.Background(), &format))
		var out bytes.Buffer
		w := &runtimeOutput{Writer: &out, cmd: c, root: root, mode: wizardOutput}
		if _, err := w.Write([]byte(tc.in)); err != nil || out.String() != tc.out {
			t.Errorf("%s: Write = %q, %v; want %q", tc.format, out.String(), err, tc.out)
		}
	}
}

type fakeTerminal struct {
	bytes.Buffer
	fd uintptr
}

func (f *fakeTerminal) Close() error { return nil }
func (f *fakeTerminal) Fd() uintptr  { return f.fd }

// A TUI sizes its screen from a terminal stdout, so wrapping a terminal keeps
// one: the wrapper still satisfies the term.File shape bubbletea checks, passes
// Fd through, and rewrites what is written. A non-terminal stays a plain writer.
func TestRuntimeOutputKeepsATerminalStdoutATerminal(t *testing.T) {
	root := &cobra.Command{Use: "bex"}
	pg := &cobra.Command{Use: "postgres", Aliases: []string{"pg"}}
	var seen io.Writer
	create := &cobra.Command{Use: "create", RunE: func(c *cobra.Command, _ []string) error {
		seen = c.OutOrStdout()
		_, err := io.WriteString(c.OutOrStdout(), "Run `render pg get my-db` to check if it's ready yet.\n")
		return err
	}}
	pg.AddCommand(create)
	root.AddCommand(pg)
	brandRuntimeOutput(root)

	tty := &fakeTerminal{fd: 7}
	create.SetOut(tty)
	if err := create.RunE(create, nil); err != nil {
		t.Fatal(err)
	}
	term, ok := seen.(terminalFile)
	if !ok || term.Fd() != 7 {
		t.Fatalf("wrapped terminal stdout = %T, want a terminalFile passing Fd 7 through", seen)
	}
	if got := tty.String(); got != "Run `bex pg get my-db` to check if it's ready yet.\n" {
		t.Fatalf("terminal output = %q", got)
	}
	var buf bytes.Buffer
	create.SetOut(&buf)
	if err := create.RunE(create, nil); err != nil {
		t.Fatal(err)
	}
	if _, isTerm := seen.(terminalFile); isTerm {
		t.Fatalf("a non-terminal stdout became a terminalFile: %T", seen)
	}
}
