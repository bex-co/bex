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
	"io"
	"strings"
	"testing"

	"github.com/render-oss/cli/cmd"
	"github.com/spf13/cobra"
)

func TestRewriteTextPreservesRenderYAML(t *testing.T) {
	in := "Validate a render.yaml Blueprint, then run:\n  render blueprints validate ./render.yaml"
	got := RewriteText(in)
	if !strings.Contains(got, "render.yaml") {
		t.Fatalf("render.yaml was corrupted: %q", got)
	}
	if strings.Contains(got, "bex.yaml") {
		t.Fatalf("invented bex.yaml: %q", got)
	}
	if !strings.Contains(got, "bex blueprints validate") {
		t.Fatalf("bare command not rewritten: %q", got)
	}
}

func TestRewriteTextBrandingPhrases(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Use the Render CLI to manage resources on Render.", "Use the bex CLI to manage resources on Bex."},
		{"Open the Render Dashboard", "Open the Bex dashboard"},
		{"Connect to Render Postgres", "Connect to Bex Postgres"},
		{"Manage Render Key Value instances", "Manage Bex Key Value instances"},
		{"Install Render agent skills", "Install Bex agent skills"},
		{"Log out of Render", "Log out of Bex"},
		{"run `render login` later", "run `bex login` later"},
		{"  render services\n  render login", "  bex services\n  bex login"},
		{"Visit https://render.com/docs/ssh", "Visit https://render.com/docs/ssh"},
		{"bex.yml is a filename alias", "bex.yml is a filename alias"},
	}
	for _, tc := range cases {
		if got := RewriteText(tc.in); got != tc.want {
			t.Errorf("RewriteText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestApplySetsUseAndHelpChrome(t *testing.T) {
	root := &cobra.Command{
		Use:     "render",
		Short:   "Interact with resources on Render",
		Long:    "Welcome! Use the Render CLI on Render.",
		Example: "  render services",
	}
	root.SetHelpTemplate(cmd.CustomHelpTemplate)
	docs := &cobra.Command{
		Use:   "docs",
		Short: "Open the Render docs in your browser",
		RunE: func(_ *cobra.Command, _ []string) error {
			t.Fatal("upstream docs RunE must not run")
			return nil
		},
	}
	root.AddCommand(docs)
	ungrouped := &cobra.Command{Use: "upgrade", Short: "Update bex"}
	root.AddCommand(ungrouped)

	Apply(root, "1.2.3")

	if root.Use != "bex" {
		t.Errorf("Use = %q, want bex", root.Use)
	}
	if root.Short != "Interact with resources on Bex" {
		t.Errorf("Short = %q", root.Short)
	}
	if !strings.Contains(root.Example, "bex services") || strings.Contains(root.Example, "render services") {
		t.Errorf("Example = %q", root.Example)
	}
	if !strings.Contains(root.Long, "bex CLI") || strings.Contains(root.Long, "Render CLI") {
		t.Errorf("Long = %q", root.Long)
	}
	if !strings.Contains(root.HelpTemplate(), `(eq .CommandPath "bex")`) {
		t.Errorf("help template missing bex CommandPath check:\n%s", root.HelpTemplate())
	}
	if docs.Short != "Open the Bex docs in your browser" {
		t.Errorf("docs Short = %q", docs.Short)
	}
	if docs.Example != "  # Open Bex documentation\n  bex docs" {
		t.Errorf("docs Example = %q", docs.Example)
	}
}

func TestHelpFuncRebrandsLateAddedCommands(t *testing.T) {
	root := &cobra.Command{Use: "render", Short: "on Render"}
	root.SetHelpTemplate(cmd.CustomHelpTemplate)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	Apply(root, "9.9.9")

	late := &cobra.Command{
		Use:     "postgres",
		Short:   "Manage Render Postgres databases",
		Example: "  render postgres list",
	}
	root.AddCommand(late)

	// HelpFunc re-walks after upstream setupCommands-style late AddCommand.
	root.HelpFunc()(root, nil)

	if late.Short != "Manage Bex Postgres databases" {
		t.Errorf("late Short = %q", late.Short)
	}
	if !strings.Contains(late.Example, "bex postgres list") {
		t.Errorf("late Example = %q", late.Example)
	}
}
