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
// the Render sandbox SDK (bex offers only the `sandboxes` subset).
var droppedSkills = map[string]bool{"render-workflows": true, "render-sandboxes": true}

//go:embed overrides
var overrides embed.FS

// sharedBlocks are upstream's marked shared sections, replaced whole by bex's
// text: they carry the access and documentation workflows, where bex differs
// (its own OAuth sign-in, API keys, and CLI and MCP docs).
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
	"render-cli/SKILL.md": {
		// The CI example's Deploy step is upstream's, with the name rules
		// applied: one BEX_API_KEY secret sent as the bearer, as Render's
		// RENDER_API_KEY is (w7/m426). Only the secret holding the service ID
		// needs a bex name.
		r(`secrets\.RENDER_SERVICE_ID\b`, "secrets.BEX_SERVICE_ID"),
		r(`sudo sh /tmp/install-bex-cli\.sh`, "sudo BEX_INSTALL_DIR=/usr/local/bin sh /tmp/install-bex-cli.sh"),
		// install-bex.sh verifies the release signature with cosign >= 2.4.2,
		// which GitHub-hosted runners do not ship (w32/004). Pinned as
		// .github/workflows/cli-release.yml pins it.
		r(`(?m)^      - name: Install bex CLI\n`, "      - name: Install cosign (install-bex.sh verifies the release signature)\n"+
			"        uses: sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6 # v4.1.2\n"+
			"      - name: Install bex CLI\n"),
	},
	// Render's logical-replication guidance (render.com/docs/postgresql-logical-replication,
	// read 2026-11-20), which upstream's Postgres skill lacks; bex enables it
	// on request the same way, for publishers (w34/m541) and subscribers (w34/m542).
	"render-postgres/SKILL.md": {
		r(`\n## Useful MCP Commands\n`, "\n"+postgresLogicalReplication+"## Useful MCP Commands\n"),
	},
	// bex SSH carries no SCP (ADR035), so Magic-Wormhole, which every
	// native runtime has as on Render (w6/m549), is the transfer to name.
	"render-disks/SKILL.md": {
		r(`a currently supported transfer method such as SCP with SSH or Magic-Wormhole\.`, disksTransfer),
		// Upstream's example has no root directory; with one, the service
		// runs in it, on Render and on bex alike (w6/m558).
		r(`(?m)^Fetch the current disk documentation`, disksRootDirectory+"\n\nFetch the current disk documentation"),
	},
	"render-migrate-from-heroku/references/service-mapping.md": {
		r(`- \*\*Heroku Pipelines\*\* → [^\n]*`, "- **Heroku Pipelines** → separate bex services or projects per stage, promoted manually (bex has no preview environments)"),
		r(`- \*\*Review Apps\*\* → [^\n]*`, "- **Review Apps** → no bex equivalent (bex has no pull request previews)"),
	},
}

// postgresLogicalReplication is the pack's Logical Replication section.
const postgresLogicalReplication = "## Logical Replication\n\n" +
	"Logical replication is **off by default** and enabled on request, as on Render:\n\n" +
	"- Requires a **Pro** workspace or higher and at least **10 GB** of storage.\n" +
	"- To enable it, contact bex support with the database ID (`dpg-…`), the credential subscribers connect as (Render recommends a dedicated read-only user, not the default one), the schemas they read, and whether to publish all tables.\n" +
	"- `CREATE PUBLICATION … FOR ALL TABLES` needs a superuser, so support creates it as `bex_all_tables`. Create `CREATE PUBLICATION … FOR TABLE …` publications yourself as the database's user.\n" +
	"- Subscribers connect to the **external** URL over TLS, never the pooled URL (PgBouncer cannot carry replication). Create the subscriber's tables first: logical replication does not copy DDL.\n" +
	"- A bex Postgres can also be a **subscriber**, copying another database (on Render, bex or anywhere else) and following it until cutover. Ask support the same way, naming the credential that will create subscriptions (use the database's own user: it owns the tables the subscription writes). Then create the tables and run `CREATE SUBSCRIPTION … CONNECTION '…password=…' PUBLICATION …` as that user; sequences are not replicated, so copy them at cutover. On PostgreSQL 15 and older only a superuser can, so support creates the subscription from the publisher's URL and publication.\n" +
	"- **Through failovers, upgrades and recovery:** on a publisher with high availability running PostgreSQL 17 or later, create each subscription `WITH (failover = true)` (from a 17+ subscriber) so it keeps its place through a failover; on 13–16 a failover loses the slots and subscribers must be re-created. A major-version upgrade is refused (`POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND`, naming the slots) until every subscriber has caught up; from 13–16 it is accepted but drops the slots, which the answer's `upgradeNotice` names. A subscriber with high availability has an asynchronous standby, as on Render: an automatic failover can lose the changes it applied in its last seconds, which its publisher does not send again (a failover you request on a healthy primary loses nothing), so re-create a subscription over emptied tables after one when they must match the publisher exactly. A database recovered to a point in time starts with logical replication off.\n\n"

// disksTransfer is how the pack's disk skill moves a file off a service.
const disksTransfer = "Magic-Wormhole, which every native runtime has: run `wormhole send <file>` in the service's Shell or SSH session, " +
	"then `wormhole receive <code>` on the receiving machine with the code it prints (reverse the two to bring a file in). " +
	"bex SSH does not support SCP. A Docker-runtime service has `wormhole` only if its image installs it."

// disksRootDirectory is where the pack's disk skill puts a disk for a
// service with a root directory.
const disksRootDirectory = "A service with a root directory (`rootDir`) runs from it: with `rootDir: backend`, mount the disk at " +
	"`/opt/render/project/src/backend/uploads` to keep what the app writes to `uploads/`. " +
	"A path under `/opt/render/project/src` outside the root directory is outside the service's own files."

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
	// A bex API key is sent as the bearer, as a Render API key is (w7/m426).
	r(`RENDER_API_KEY`, "BEX_API_KEY"),
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
	// The pinned Render CLI's version floors (v2.x) read as "your bex v0.x is
	// too old" to an agent. Every released bex CLI has the commands they
	// guard, so the pack states no minimum (w8/233).
	r(`When bex CLI v\d+\.\d+\.\d+ or later is available`, "When the bex CLI is available"),
	r(`(bex CLI\s+)v\d+\.\d+\.\d+\+ `, "$1"),
	r(` \(CLI v\d+\.\d+\.\d+\+\)`, ""),
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
