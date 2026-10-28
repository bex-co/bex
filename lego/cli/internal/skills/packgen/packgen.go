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

// Package packgen derives bex's agent skills pack from Render's MIT-licensed
// render-oss/skills (w8/m143). Render's skills teach an agent to operate
// Render: the `render` CLI, mcp.render.com and dashboard.render.com. The pack
// keeps their guidance, which describes behavior bex implements, and points
// every operating step at bex instead.
package packgen

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UpstreamCommit is the render-oss/skills commit the pack is derived from.
const UpstreamCommit = "33801b5"

// droppedSkills are bex non-goals: Render Workflows, and sandbox groups and
// the Render sandbox SDK (bex offers only the `ea sandboxes` subset).
var droppedSkills = map[string]bool{"render-workflows": true, "render-sandboxes": true}

//go:embed overrides
var overrides embed.FS

// sharedBlocks are upstream's marked shared sections, replaced whole by bex's
// text: they carry the access and documentation workflows, where bex differs
// (OAuth or an exchanged API-key token, bex's own CLI and MCP docs).
var sharedBlocks = []string{"bex-access", "documentation-retrieval"}

// fileOverrides replace an upstream file whose content is about operating
// Render's MCP server, which bex authenticates differently.
var fileOverrides = map[string]string{
	"render-mcp/SKILL.md":                      "bex-mcp/SKILL.md",
	"render-mcp/references/troubleshooting.md": "bex-mcp/references/troubleshooting.md",
}

// filePatches correct, after the rewrite, guidance where bex deliberately
// differs from Render (ADR018): bex rejects wildcard custom domains and
// issues certificates only from Let's Encrypt (ADR005), and offers no
// preview environments (.pm/DO_NOT_DO.md).
var filePatches = map[string][]rule{
	"render-domains/SKILL.md": {
		r(`(?s)## Wildcard Domains\n.*?\n## CAA Records`, "## Wildcard Domains\n\n"+
			"bex does not support wildcard custom domains (`*.example.com`). Add each subdomain as its own custom domain.\n\n## CAA Records"),
		r(`- Setting up a \*\*wildcard domain\*\* \(`+"`"+`\*\.example\.com`+"`"+`\)\n`, ""),
		r(`\nexample\.com IN CAA 0 issue "pki\.goog; cansignhttpexchanges=yes"\nexample\.com IN CAA 0 issuewild "pki\.goog; cansignhttpexchanges=yes"`, ""),
		r(`\| Cloudflare proxy \+ wildcard without root domain \|[^\n]*\n`, ""),
	},
	// A bex access token lasts 15 minutes, so CI stores the API key and
	// exchanges it per run rather than storing a token (ADR025).
	"render-cli/SKILL.md": {
		r(`(?s)      - name: Deploy\n        env:\n          BEX_ACCESS_TOKEN: \$\{\{ secrets\.BEX_ACCESS_TOKEN \}\}\n        run: bex deploys create \$\{\{ secrets\.RENDER_SERVICE_ID \}\}`,
			"      - name: Deploy\n        env:\n          BEX_KEY_ID: ${{ secrets.BEX_KEY_ID }}\n          BEX_KEY_SECRET: ${{ secrets.BEX_KEY_SECRET }}\n        run: |\n"+
				"          export BEX_ACCESS_TOKEN=\"$(curl -fsS -X POST https://oauth.bex.co/oauth2/token \\\n"+
				"            -d \"grant_type=client_credentials&client_id=$$BEX_KEY_ID&client_secret=$$BEX_KEY_SECRET\" | jq -r .access_token)\"\n"+
				"          bex deploys create ${{ secrets.BEX_SERVICE_ID }}"),
		r(`sudo sh /tmp/install-bex-cli\.sh`, "sudo BEX_INSTALL_DIR=/usr/local/bin sh /tmp/install-bex-cli.sh"),
	},
	"render-migrate-from-heroku/references/service-mapping.md": {
		r(`- \*\*Heroku Pipelines\*\* → [^\n]*`, "- **Heroku Pipelines** → separate bex services or projects per stage, promoted manually (bex has no preview environments)"),
		r(`- \*\*Review Apps\*\* → [^\n]*`, "- **Review Apps** → no bex equivalent (bex has no pull request previews)"),
	},
}

// cliCommands are the CLI's top-level commands a skill names; only these turn
// `render <command>` into `bex <command>`, so prose ("render the page") stays.
const cliCommands = `blueprints|deploys|docs|ea|environments|help|jobs|keyvalues|kv-cli|login|logout|logs|pgcli|postgres|projects|psql|restart|resume|services|skills|ssh|suspend|whoami|workspace|workspaces`

type rule struct {
	re   *regexp.Regexp
	with string
}

func r(pattern, with string) rule { return rule{regexp.MustCompile(pattern), with} }

// endpointRules run first, in order, over every kept file.
var endpointRules = []rule{
	r(`https://mcp\.render\.com/mcp`, "https://api.bex.co/mcp"),
	r(`mcp\.render\.com(/mcp)?`, "api.bex.co/mcp"),
	r(`dashboard\.render\.com`, "dashboard.bex.co"),
	r(`api\.render\.com`, "api.bex.co"),
	r(`onrender\.com`, "onbex.co"),
	r(`https://raw\.githubusercontent\.com/render-oss/cli/(refs/heads/)?main/bin/install\.sh`, "https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh"),
	// Docs that tell the agent how to operate the platform are bex's own.
	r(`https://render\.com/docs/(cli-reference|cli)(\.md)?`, "https://bex.co/docs/cli"),
	r(`https://render\.com/docs/mcp-server(\.md)?`, "https://bex.co/docs/mcp-server"),
	r(`https://render\.com/(pricing|docs/compute-plans(\.md)?)(#[a-z0-9-]+)?`, "https://bex.co/pricing"),
	r(`\[contact Render( support)?\]\(https://render\.com/contact\)`, "[contact bex support](https://bex.co)"),
	r(`- https://status\.render\.com`, "- the service's **Events** in the bex dashboard; report a suspected platform-wide issue to bex support (https://bex.co)"),
}

// nameRules run next, in order, with keepAsIs protected.
var nameRules = []rule{
	// The CLI and its configuration.
	r("(^|[\\s`$(|;&\"'/])render ("+cliCommands+`|--version|--help)\b`, "${1}bex $2"),
	r(`\brender CLI\b`, "bex CLI"),
	r(`brew install render\b`, "brew install bex-co/tap/bex"),
	r(`~/\.render/cli\.yaml`, "~/.bex/cli.yaml"),
	r(`RENDER_API_KEY`, "BEX_ACCESS_TOKEN"),
	r(`RENDER_CLI_CONFIG_(PATH|DIR)`, "BEX_CLI_CONFIG_$1"),
	r(`\bRENDER_WORKSPACE\b`, "BEX_WORKSPACE"),
	// Skill identities.
	r(`\brender-([a-z]+(-[a-z]+)*)\b`, "bex-$1"),
	// The platform's name. Render's docs stay linked as the reference for
	// the behavior bex implements, under their own name.
	r(`\bRender Dashboard\b`, "bex dashboard"),
	r(`\bRender(’|')s Dashboard\b`, "the bex dashboard"),
	r(`\bRender(’|')s\b`, "bex${1}s"),
	r(`\bRender\b`, "bex"),
}

// keepAsIs protects names that must survive the identity rules.
var keepAsIs = []string{"render-oss/", "render.yaml", "render.com/docs", "render.com/schema", "Render's documentation", "Render’s documentation"}

// Rewrite maps one upstream file (path relative to the upstream skills/
// directory) to its pack path and content. keep is false for a dropped skill.
func Rewrite(path string, content []byte) (newPath string, out []byte, keep bool, err error) {
	skill, _, _ := strings.Cut(path, "/")
	if droppedSkills[skill] {
		return "", nil, false, nil
	}
	newPath = renameSkillPath(path)
	override, overridden := fileOverrides[path]
	if overridden {
		if content, err = overrides.ReadFile("overrides/" + override); err != nil {
			return "", nil, false, err
		}
	}
	text := string(content)
	if !overridden {
		text = rewriteText(text)
	}
	for _, rl := range filePatches[path] {
		if !rl.re.MatchString(text) {
			return "", nil, false, fmt.Errorf("%s: patch %q no longer matches upstream", path, rl.re)
		}
		text = rl.re.ReplaceAllString(text, rl.with)
	}
	// Shared blocks are bex's own text, so they go in after the rewrite.
	for _, block := range sharedBlocks {
		if text, err = replaceShared(text, block); err != nil {
			return "", nil, false, fmt.Errorf("%s: %w", path, err)
		}
	}
	return newPath, []byte(text), true, nil
}

func renameSkillPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "render-access.md" {
			parts[i] = "bex-access.md"
		} else if rest, ok := strings.CutPrefix(part, "render-"); ok && i == 0 {
			parts[i] = "bex-" + rest
		}
	}
	return strings.Join(parts, "/")
}

func replaceShared(text, block string) (string, error) {
	start, end := "<!-- shared:"+block+" -->", "<!-- /shared:"+block+" -->"
	i := strings.Index(text, start)
	if i < 0 {
		return text, nil
	}
	j := strings.Index(text[i:], end)
	if j < 0 {
		return "", fmt.Errorf("unterminated shared block %s", block)
	}
	j += i
	body, err := overrides.ReadFile("overrides/shared/" + block + ".md")
	if err != nil {
		return "", err
	}
	return text[:i] + start + "\n" + string(body) + end + text[j+len(end):], nil
}

func rewriteText(text string) string {
	for _, rl := range endpointRules {
		text = rl.re.ReplaceAllString(text, rl.with)
	}
	placeholders := make([]string, len(keepAsIs))
	for i, keep := range keepAsIs {
		placeholders[i] = fmt.Sprintf("\x00keep%d\x00", i)
		text = strings.ReplaceAll(text, keep, placeholders[i])
	}
	for _, rl := range nameRules {
		text = rl.re.ReplaceAllString(text, rl.with)
	}
	for i, keep := range keepAsIs {
		text = strings.ReplaceAll(text, placeholders[i], keep)
	}
	return text
}

// Generate writes the pack derived from the upstream checkout at src into
// dst, replacing dst/skills. The license and provenance travel with it.
func Generate(src, dst string) error {
	skillsSrc := filepath.Join(src, "skills")
	if err := os.RemoveAll(filepath.Join(dst, "skills")); err != nil {
		return err
	}
	var paths []string
	err := filepath.WalkDir(skillsSrc, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(skillsSrc, p)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(skillsSrc, rel))
		if err != nil {
			return err
		}
		newPath, out, keep, err := Rewrite(rel, content)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}
		target := filepath.Join(dst, "skills", filepath.FromSlash(newPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, out, 0o644); err != nil {
			return err
		}
	}
	license, err := os.ReadFile(filepath.Join(src, "LICENSE"))
	if err != nil {
		return err
	}
	notice := "This pack is derived from https://github.com/render-oss/skills at commit " + UpstreamCommit +
		" (MIT License, reproduced below), rewritten to operate bex.\n" +
		"Regenerate it with lego/cli/hack/sync-skills.sh.\n\n" + string(license)
	return os.WriteFile(filepath.Join(dst, "LICENSE"), []byte(notice), 0o644)
}
