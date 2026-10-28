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
		{"export RENDER_API_KEY; RENDER_EXTERNAL_HOSTNAME", "export BEX_ACCESS_TOKEN; RENDER_EXTERNAL_HOSTNAME"},
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
