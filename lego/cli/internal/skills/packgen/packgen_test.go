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

package packgen

import (
	"os"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Run `render deploys create` then render logs -r x", "Run `bex deploys create` then bex logs -r x"},
		{"pg_dump/render psql required", "pg_dump/bex psql required"},
		{"render --version", "bex --version"},
		{"We render the page", "We render the page"},
		{"https://mcp.render.com/mcp", "https://api.bex.co/mcp"},
		{"https://dashboard.render.com/u/settings", "https://dashboard.bex.co/u/settings"},
		{"https://myapp.onrender.com/", "https://myapp.onbex.co/"},
		{"[docs](https://render.com/docs/cli.md)", "[docs](https://bex.co/docs/cli)"},
		{"[spec](https://render.com/docs/blueprint-spec)", "[spec](https://render.com/docs/blueprint-spec)"},
		{"See render-deploy and references/render-access.md", "See bex-deploy and references/bex-access.md"},
		{"Use the Render Dashboard. Render's CLI", "Use the bex dashboard. bex's CLI"},
		{"edit render.yaml from render-oss/skills", "edit render.yaml from render-oss/skills"},
		{"https://raw.githubusercontent.com/render-oss/cli/refs/heads/main/bin/install.sh", "https://raw.githubusercontent.com/bex-co/bex/main/scripts/install-bex.sh"},
		{"see github.com/render-oss/cli", "see github.com/render-oss/cli"},
		{"export RENDER_API_KEY; RENDER_EXTERNAL_HOSTNAME", "export BEX_API_KEY; RENDER_EXTERNAL_HOSTNAME"},
		// The Render CLI's version floors are not bex's (w8/233).
		{"compatibility: Render CLI v2.7.0+ (Homebrew, Linux/macOS)", "compatibility: bex CLI (Homebrew, Linux/macOS)"},
		{"Blueprint sync. Render CLI\n  v2.7.0+ recommended for `render blueprints validate`.", "Blueprint sync. bex CLI\n  recommended for `bex blueprints validate`."},
		{"When Render CLI v2.7.0 or later is available, validate the finished file:", "When the bex CLI is available, validate the finished file:"},
		{"1. `render blueprints validate` (CLI v2.7.0+).", "1. `bex blueprints validate`."},
		{"Requires Heroku CLI v10.8.1+ and image ghcr.io/org/app:v1.2.3", "Requires Heroku CLI v10.8.1+ and image ghcr.io/org/app:v1.2.3"},
	} {
		if got := rewriteText(tc.in); got != tc.want {
			t.Errorf("rewriteText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRewriteDropsNonGoalsAndRenamesPaths(t *testing.T) {
	if _, _, keep, _ := Rewrite("render-workflows/SKILL.md", nil); keep {
		t.Error("render-workflows kept")
	}
	path, out, keep, err := Rewrite("render-cli/references/render-access.md", []byte("x\n<!-- shared:render-access -->\nold\n<!-- /shared:render-access -->\n"))
	if err != nil || !keep || path != "bex-cli/references/bex-access.md" {
		t.Fatalf("Rewrite = %q, %v, %v", path, keep, err)
	}
	if strings.Contains(string(out), "old") || !strings.Contains(string(out), "https://api.bex.co/mcp") {
		t.Errorf("shared block not replaced with bex's text:\n%s", out)
	}
}

// A patch that no longer matches upstream fails generation instead of
// silently shipping Render's guidance.
func TestStalePatchFailsGeneration(t *testing.T) {
	if _, _, _, err := Rewrite("render-domains/SKILL.md", []byte("no wildcard section here")); err == nil {
		t.Error("stale patch passed")
	}
}

// upstreamCIExample is the GitHub Actions example of render-oss/skills'
// render-cli/SKILL.md at UpstreamCommit.
const upstreamCIExample = "```yaml\n" + `name: Deploy to Render
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Install Render CLI
        run: |
          curl --fail-with-body --silent --show-error --location \
            https://raw.githubusercontent.com/render-oss/cli/refs/heads/main/bin/install.sh \
            --output /tmp/install-render-cli.sh
          sudo sh /tmp/install-render-cli.sh
          render --version
      - name: Deploy
        env:
          RENDER_API_KEY: ${{ secrets.RENDER_API_KEY }}
        run: render deploys create ${{ secrets.RENDER_SERVICE_ID }} --wait --confirm -o json
` + "```\n"

// tokenExchange is the vocabulary of the client_credentials recipe the pack
// carried while a bex API key could not be sent as a bearer.
var tokenExchange = []string{"oauth2/token", "client_credentials", "BEX_KEY_ID", "BEX_KEY_SECRET", "BEX_ACCESS_TOKEN", "access_token"}

// w7/m426: a bex API key is sent as the bearer, as a Render API key is, so
// the CI example keeps Render's one-variable Deploy step.
func TestCIExampleDeploysWithAnAPIKey(t *testing.T) {
	_, out, keep, err := Rewrite("render-cli/SKILL.md", []byte(upstreamCIExample))
	if err != nil || !keep {
		t.Fatalf("Rewrite = %v, %v", keep, err)
	}
	const deploy = `      - name: Deploy
        env:
          BEX_API_KEY: ${{ secrets.BEX_API_KEY }}
        run: bex deploys create ${{ secrets.BEX_SERVICE_ID }} --wait --confirm -o json
` + "```\n"
	if !strings.HasSuffix(string(out), deploy) {
		t.Errorf("the Deploy step is not Render's with bex's names:\n%s", out)
	}
	for _, word := range append(tokenExchange, "RENDER_", "curl -fsS", "jq ") {
		if strings.Contains(string(out), word) {
			t.Errorf("the CI example contains %q:\n%s", word, out)
		}
	}
	// The other two patches of the file still apply.
	for _, want := range []string{"sigstore/cosign-installer@", "sudo BEX_INSTALL_DIR=/usr/local/bin sh /tmp/install-bex-cli.sh"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the CI example lost %q:\n%s", want, out)
		}
	}
}

// bex's own text (the shared access block and the MCP skill) tells an agent
// to send the API key itself, never to exchange it for a token first.
func TestBexTextSendsTheAPIKeyAsTheBearer(t *testing.T) {
	for path, want := range map[string]string{
		"render-cli/references/render-access.md":   "use a securely stored API key in `BEX_API_KEY`",
		"render-mcp/SKILL.md":                      `--header "Authorization: Bearer ${BEX_API_KEY}"`,
		"render-mcp/references/troubleshooting.md": "revoked API key",
	} {
		_, out, keep, err := Rewrite(path, []byte("<!-- shared:render-access -->\n<!-- /shared:render-access -->\n"))
		if err != nil || !keep {
			t.Fatalf("Rewrite(%s) = %v, %v", path, keep, err)
		}
		if !strings.Contains(string(out), want) {
			t.Errorf("%s does not say %q:\n%s", path, want, out)
		}
		for _, word := range tokenExchange {
			if strings.Contains(string(out), word) {
				t.Errorf("%s contains %q", path, word)
			}
		}
	}
}

// bex's Logical Replication section rides a patch on upstream's Postgres
// skill, so regenerating the pack keeps it, and the checked-in pack carries
// the same text (w34/m541).
func TestPostgresSkillKeepsLogicalReplication(t *testing.T) {
	upstream := "## Read Replicas\n\n- Up to **5 read replicas** per database.\n\n## Useful MCP Commands\n\nUse the Render MCP tools.\n"
	_, out, keep, err := Rewrite("render-postgres/SKILL.md", []byte(upstream))
	if err != nil || !keep {
		t.Fatalf("Rewrite = %v, %v", keep, err)
	}
	if !strings.Contains(string(out), "\n"+postgresLogicalReplication+"## Useful MCP Commands\n") {
		t.Fatalf("the Logical Replication section is not before Useful MCP Commands:\n%s", out)
	}
	pack, err := os.ReadFile("../pack/skills/bex-postgres/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pack), postgresLogicalReplication) {
		t.Error("the checked-in bex-postgres skill lacks the generated Logical Replication section")
	}
}

// disksUpstream carries the upstream disk skill's lines its patches rewrite:
// a patch that no longer matches fails the generation.
const disksUpstream = "```\n\nFetch the current disk documentation and Blueprint specification through `references/persistent-disks.md`.\n\n" +
	"Move backup files off the service with a currently supported transfer method such as SCP with SSH or Magic-Wormhole. Snapshot restores replace the entire disk.\n"

// bex SSH has no SCP (ADR035), so the disk skill names the transfer every
// native runtime has, Magic-Wormhole, and the checked-in pack says so (w6/m549).
func TestDisksSkillMovesFilesWithWormhole(t *testing.T) {
	_, out, keep, err := Rewrite("render-disks/SKILL.md", []byte(disksUpstream))
	if err != nil || !keep {
		t.Fatalf("Rewrite = %v, %v", keep, err)
	}
	if !strings.Contains(string(out), "off the service with "+disksTransfer+" Snapshot") || strings.Contains(string(out), "SCP with SSH") {
		t.Fatalf("the disk skill still offers SCP or lost the wormhole transfer:\n%s", out)
	}
	pack, err := os.ReadFile("../pack/skills/bex-disks/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pack), disksTransfer) || strings.Contains(string(pack), "SCP with SSH") {
		t.Error("the checked-in bex-disks skill does not carry the generated wormhole transfer")
	}
}

// A service with a root directory runs from it (w6/m558), so the disk skill
// says where its disk goes, after upstream's example, which has none.
func TestDisksSkillPlacesARootDirectorysDisk(t *testing.T) {
	_, out, keep, err := Rewrite("render-disks/SKILL.md", []byte(disksUpstream))
	if err != nil || !keep {
		t.Fatalf("Rewrite = %v, %v", keep, err)
	}
	if !strings.Contains(string(out), "```\n\n"+disksRootDirectory+"\n\nFetch the current disk documentation") {
		t.Fatalf("the root-directory placement is not after the example:\n%s", out)
	}
	pack, err := os.ReadFile("../pack/skills/bex-disks/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pack), disksRootDirectory) {
		t.Error("the checked-in bex-disks skill lacks the generated root-directory placement")
	}
}
