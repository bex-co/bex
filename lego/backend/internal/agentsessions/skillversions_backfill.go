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

package agentsessions

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// SkillVersionRewriter rewrites stored skill version contents; *store.PGStore
// satisfies it.
type SkillVersionRewriter interface {
	RewriteAgentSkillVersions(ctx context.Context, rewrite func(content string) (string, bool)) (int, error)
}

// BackfillSkillVersionContent re-renders the skill version rows that
// migration 0210 backfilled in SQL with the Go renderer, so a description the
// two quote differently ("Deploy: staging", "Fix #12", "yes") does not show as
// a changed description line in the diff of the skill's next save. It returns
// how many rows it rewrote. bex-api runs it once at startup; it is idempotent
// (a rewritten row is the Go rendering, which no later pass changes).
func BackfillSkillVersionContent(ctx context.Context, versions SkillVersionRewriter) (int, error) {
	return versions.RewriteAgentSkillVersions(ctx, rerenderBackfilledSkillVersion)
}

// rerenderBackfilledSkillVersion is the Go rendering of content when content
// is exactly what 0210's SQL wrote for the same fields and differs from the Go
// rendering. That test identifies a backfilled row without relying on its
// message or time: every row a save wrote is already the Go rendering (never
// changed), and a row that merely parses is not touched unless it equals the
// SQL rendering byte for byte.
func rerenderBackfilledSkillVersion(content string) (string, bool) {
	doc, err := parseSkill(content)
	if err != nil {
		return "", false
	}
	triggers := doc.Triggers
	if triggers == nil { // no triggers key: both, as a create defaults them
		triggers = defaultSkillTriggers
	}
	row := store.AgentSkill{Name: doc.Name, Description: doc.Description, Triggers: triggers, Extra: doc.Extra, Body: doc.Body}
	rendered := renderSkill(row, "")
	if rendered == content || legacySQLRenderedSkill(row) != content {
		return "", false
	}
	return rendered, true
}

// legacySQLRenderedSkill is the content 0210's SQL wrote for a skill row.
func legacySQLRenderedSkill(row store.AgentSkill) string {
	var b strings.Builder
	b.WriteString("---\nname: " + legacySQLScalar(row.Name) + "\ndescription: " + legacySQLScalar(row.Description) + "\n")
	if slices.Equal(row.Triggers, []string{skillTriggerUser}) {
		b.WriteString("triggers: [user]\n")
	}
	b.WriteString(row.Extra + "---\n" + row.Body)
	return b.String()
}

var (
	legacyPlainScalar    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9 .,()/_-]*$`)
	legacyReservedScalar = regexp.MustCompile(`(?i)^(true|false|null|yes|no|on|off|y|n)$`)
)

// legacySQLScalar is 0210's pg_temp.skill_yaml_scalar: plain when it is plainly
// a word or phrase, else a double-quoted JSON string (to_json: no HTML
// escaping, U+2028/2029 left as they are).
func legacySQLScalar(v string) string {
	if legacyPlainScalar.MatchString(v) && !legacyReservedScalar.MatchString(v) && !strings.HasSuffix(v, " ") {
		return v
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	out := strings.TrimSuffix(buf.String(), "\n")
	return strings.NewReplacer(`\u2028`, "\u2028", `\u2029`, "\u2029").Replace(out)
}
