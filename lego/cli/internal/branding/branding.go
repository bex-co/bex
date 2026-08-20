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

// Package branding overlays Bex product identity onto the exported upstream
// cobra RootCmd without forking or vendoring render-oss/cli. Every pin bump
// must re-diff these seams — see lego/cli/UPSTREAM_RENDER_CLI.md.
package branding

import (
	"strings"

	"github.com/render-oss/cli/cmd"
	"github.com/render-oss/cli/pkg/dashboard"
	"github.com/render-oss/cli/pkg/style"
	"github.com/spf13/cobra"
)

// DocsURL is opened by the overridden `bex docs` command. There is no
// docs.bex.co host yet; the CLI guide is the closest stable public page.
const DocsURL = "https://github.com/bex-co/bex/blob/main/docs/bex-cli.md"

const (
	renderYAMLToken = "\x00RENDER_YAML\x00"
	bexYAMLAlias    = "\x00BEX_YAML\x00"
)

// Apply mutates root in place: Use/examples/Short/Long read as Bex, help
// chrome uses bexVersion, and `docs` opens DocsURL. Upstream setupCommands
// adds more children during Execute (after this returns), so HelpFunc
// re-walks the tree before any help output. It does not replace upstream
// RunE bodies except for `docs`.
func Apply(root *cobra.Command, bexVersion string) {
	if root == nil {
		return
	}

	cobra.AddTemplateFunc("cliVersion", func() string {
		return style.SubtleText.Faint(true).Render("bex CLI v" + bexVersion)
	})
	root.SetHelpTemplate(strings.Replace(
		cmd.CustomHelpTemplate,
		`(eq .CommandPath "render")`,
		`(eq .CommandPath "bex")`,
		1,
	))
	root.Use = "bex"

	brandTree(root)
	overrideDocs(root)

	prevHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		brandTree(root)
		overrideDocs(root)
		prevHelp(c, args)
	})
}

func brandTree(root *cobra.Command) {
	walk(root, func(c *cobra.Command) {
		c.Example = RewriteText(c.Example)
		c.Short = RewriteText(c.Short)
		c.Long = RewriteText(c.Long)
	})
}

func overrideDocs(root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Name() != "docs" {
			continue
		}
		c.Short = "Open the Bex docs in your browser"
		c.Long = "Open the Bex CLI guide in your browser."
		c.Example = "  # Open Bex documentation\n  bex docs"
		c.RunE = func(_ *cobra.Command, _ []string) error {
			return dashboard.Open(DocsURL)
		}
		return
	}
}

func walk(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, child := range c.Commands() {
		walk(child, fn)
	}
}

// RewriteText applies safe, ordered branding replacements. render.yaml and
// the bex.yml filename alias are preserved; command examples that invoke
// `render` become `bex`.
func RewriteText(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = strings.ReplaceAll(out, "render.yaml", renderYAMLToken)
	out = strings.ReplaceAll(out, "bex.yml", bexYAMLAlias)

	for _, pair := range []struct{ old, new string }{
		{"Render CLI", "bex CLI"},
		{"Render Dashboard", "Bex dashboard"},
		{"Render Postgres", "Bex Postgres"},
		{"Render Key Value", "Bex Key Value"},
		{"Render Workflows", "Bex Workflows"},
		{"Render agent skills", "Bex agent skills"},
		{"Render skills", "Bex skills"},
		{"Log out of Render", "Log out of Bex"},
		{"Log in to Render", "Log in to Bex"},
		{"logged out of Render", "logged out of Bex"},
		{"on Render", "on Bex"},
		{"to Render", "to Bex"},
		{"of Render", "of Bex"},
		{"with Render", "with Bex"},
		{"from Render", "from Bex"},
		{"the Render ", "the Bex "},
		{"`render ", "`bex "},
		{"`render`", "`bex`"},
	} {
		out = strings.ReplaceAll(out, pair.old, pair.new)
	}

	out = rewriteBareCommand(out)

	out = strings.ReplaceAll(out, renderYAMLToken, "render.yaml")
	out = strings.ReplaceAll(out, bexYAMLAlias, "bex.yml")
	return out
}

// rewriteBareCommand turns leading "render " on a line into "bex " without
// touching hostnames (render.com) or the protected yaml tokens.
func rewriteBareCommand(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		pad := line[:len(line)-len(trimmed)]
		switch {
		case strings.HasPrefix(trimmed, "render "):
			lines[i] = pad + "bex " + trimmed[len("render "):]
		case trimmed == "render":
			lines[i] = pad + "bex"
		}
	}
	return strings.Join(lines, "\n")
}
