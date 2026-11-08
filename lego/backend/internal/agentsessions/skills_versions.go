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
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Version history of workspace skills (ADR099 D2/D3): every save keeps a
// version, readable and diffable on every surface. Restore is a read of an
// old version plus an ordinary update, so there is nothing to write here.
const (
	defaultSkillVersionPage = 50
	maxSkillVersionPage     = 200
)

// AgentSkillVersion is one saved version of a workspace skill. A list leaves
// Content, Diff and PreviousVersion out; a single read carries them.
type AgentSkillVersion struct {
	SkillID string `json:"skillId"`
	OwnerID string `json:"ownerId"`
	Version int    `json:"version"`
	// ShortID is the 7-hex display handle of the version.
	ShortID string `json:"shortId"`
	// Message is "Add skills/<name>/SKILL.md" for the first save and
	// "Update skills/<name>/SKILL.md" for every later one.
	Message   string    `json:"message"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// Current marks the version the skill is at now.
	Current bool `json:"current"`
	// Content is the whole SKILL.md at that save.
	Content string `json:"content,omitempty"`
	// PreviousVersion is the version Diff compares against; absent when there
	// is none (the first save, or the previous one fell out of retention).
	PreviousVersion int `json:"previousVersion,omitempty"`
	// Diff is the unified line diff from the previous version to this one (see
	// unifiedLineDiff); every line is an addition when there is no previous
	// version. Empty when the save changed nothing.
	Diff string `json:"diff,omitempty"`
}

// AgentSkillVersionList is a skill's versions, newest first.
type AgentSkillVersionList struct {
	SkillID  string              `json:"skillId"`
	OwnerID  string              `json:"ownerId"`
	Versions []AgentSkillVersion `json:"versions"`
	// NextCursor, when set, is the cursor of the next (older) page.
	NextCursor string `json:"nextCursor,omitempty"`
}

func errSkillVersionNotFound(ref string, version int) error {
	return core.NewNotFoundError("AGENT_SKILL_VERSION_NOT_FOUND", "the skill has no such version (it may have been dropped by the 200-version retention)",
		map[string]any{"skill": ref, "version": version})
}

// skillSaveMessage is the generated commit-like message of a save.
func skillSaveMessage(create bool, name string) string {
	verb := "Update"
	if create {
		verb = "Add"
	}
	return verb + " skills/" + name + "/SKILL.md"
}

// stampSkillVersion sets the version content and message the store writes
// with row.
func stampSkillVersion(row *store.AgentSkill, create bool) {
	row.VersionContent = renderSkill(*row, "")
	row.VersionMessage = skillSaveMessage(create, row.Name)
}

func skillVersionView(v store.AgentSkillVersion, ownerID string, current int) AgentSkillVersion {
	return AgentSkillVersion{
		SkillID: v.SkillID, OwnerID: ownerID, Version: v.Version, ShortID: v.ShortID, Message: v.Message,
		CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, Current: v.Version == current,
	}
}

// AgentSkillVersions lists a skill's saved versions, newest first, without
// their content. Reading versions needs what reading the skill does.
func (s *Service) AgentSkillVersions(ctx context.Context, ownerID, ref, cursor string, limit int) (AgentSkillVersionList, error) {
	row, err := s.scopeSkill(ctx, ownerID, ref, core.RelCanCreate, "")
	if err != nil {
		return AgentSkillVersionList{}, err
	}
	before := 0
	if cursor != "" {
		if before, err = strconv.Atoi(cursor); err != nil || before < 1 {
			return AgentSkillVersionList{}, skillInvalid("cursor", "cursor must be a value returned as nextCursor")
		}
	}
	if limit <= 0 {
		limit = defaultSkillVersionPage
	}
	limit = min(limit, maxSkillVersionPage)
	rows, err := s.Skills.ListAgentSkillVersions(ctx, row.ID, before, limit+1)
	if err != nil {
		return AgentSkillVersionList{}, err
	}
	out := AgentSkillVersionList{SkillID: row.ID, OwnerID: row.WorkspaceID, Versions: make([]AgentSkillVersion, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		out.NextCursor = strconv.Itoa(rows[limit-1].Version)
	}
	for _, v := range rows {
		out.Versions = append(out.Versions, skillVersionView(v, row.WorkspaceID, row.Version))
	}
	return out, nil
}

// AgentSkillVersion reads one saved version with its content and the diff
// against the previous version. The current version's content is rendered
// from the live skill, so a version backfilled by the migration reads exactly
// as the skill does.
func (s *Service) AgentSkillVersion(ctx context.Context, ownerID, ref string, version int) (AgentSkillVersion, error) {
	row, err := s.scopeSkill(ctx, ownerID, ref, core.RelCanCreate, "")
	if err != nil {
		return AgentSkillVersion{}, err
	}
	stored, err := s.Skills.GetAgentSkillVersion(ctx, row.ID, version)
	if errors.Is(err, store.ErrNotFound) {
		return AgentSkillVersion{}, errSkillVersionNotFound(ref, version)
	}
	if err != nil {
		return AgentSkillVersion{}, err
	}
	if stored.Version == row.Version {
		stored.Content = renderSkill(row, "")
	}
	out := skillVersionView(stored, row.WorkspaceID, row.Version)
	out.Content = stored.Content
	previous := ""
	if stored.Version > 1 {
		prev, err := s.Skills.GetAgentSkillVersion(ctx, row.ID, stored.Version-1)
		switch {
		case err == nil:
			previous, out.PreviousVersion = prev.Content, prev.Version
		case !errors.Is(err, store.ErrNotFound):
			return AgentSkillVersion{}, err
		}
	}
	out.Diff = unifiedLineDiff(previous, out.Content)
	return out, nil
}

const (
	diffContextLines = 3
	// diffMaxCells bounds the line-matching table of one diff; beyond it the
	// changed middle is shown as a whole replacement.
	diffMaxCells = 4 << 20
)

type diffOp struct {
	kind byte // ' ', '-' or '+'
	text string
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// unifiedLineDiff is the line diff from old to new as unified hunks
// ("@@ -a,b +c,d @@" followed by " ", "-" and "+" lines, 3 lines of context,
// no file headers), or "" when they are equal. An empty old text makes every
// line an addition.
func unifiedLineDiff(oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a, b := splitDiffLines(oldText), splitDiffLines(newText)
	ops := diffLines(a, b)

	// Group the changed ops into hunks with context on both sides.
	var out strings.Builder
	n := len(ops)
	oldLine, newLine := make([]int, n+1), make([]int, n+1) // 1-based line before op i
	oldLine[0], newLine[0] = 1, 1
	for i, op := range ops {
		oldLine[i+1], newLine[i+1] = oldLine[i], newLine[i]
		if op.kind != '+' {
			oldLine[i+1]++
		}
		if op.kind != '-' {
			newLine[i+1]++
		}
	}
	for i := 0; i < n; {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := max(0, i-diffContextLines)
		// Changes less than two contexts apart share one hunk.
		last := i
		for j := i + 1; j < n && j-last <= 2*diffContextLines+1; j++ {
			if ops[j].kind != ' ' {
				last = j
			}
		}
		end := min(n, last+1+diffContextLines)
		oldCount, newCount := oldLine[end]-oldLine[start], newLine[end]-newLine[start]
		oldStart, newStart := oldLine[start], newLine[start]
		if oldCount == 0 {
			oldStart--
		}
		if newCount == 0 {
			newStart--
		}
		out.WriteString("@@ -" + strconv.Itoa(oldStart) + "," + strconv.Itoa(oldCount) +
			" +" + strconv.Itoa(newStart) + "," + strconv.Itoa(newCount) + " @@\n")
		for _, op := range ops[start:end] {
			out.WriteByte(op.kind)
			out.WriteString(op.text)
			out.WriteByte('\n')
		}
		i = end
	}
	return out.String()
}

// diffLines is an edit script from a to b: common prefix and suffix are
// trimmed, the middle is matched by longest common subsequence, and a middle
// too large for the table is a whole replacement.
func diffLines(a, b []string) []diffOp {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	ma, mb := a[prefix:len(a)-suffix], b[prefix:len(b)-suffix]
	ops := make([]diffOp, 0, len(a)+len(b))
	for _, line := range a[:prefix] {
		ops = append(ops, diffOp{' ', line})
	}
	ops = append(ops, diffMiddle(ma, mb)...)
	for _, line := range a[len(a)-suffix:] {
		ops = append(ops, diffOp{' ', line})
	}
	return ops
}

func diffMiddle(a, b []string) []diffOp {
	replace := func() []diffOp {
		ops := make([]diffOp, 0, len(a)+len(b))
		for _, line := range a {
			ops = append(ops, diffOp{'-', line})
		}
		for _, line := range b {
			ops = append(ops, diffOp{'+', line})
		}
		return ops
	}
	if len(a) == 0 || len(b) == 0 || (len(a)+1)*(len(b)+1) > diffMaxCells {
		return replace()
	}
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int32, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	ops := make([]diffOp, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case i < len(a) && (j == len(b) || lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	return ops
}
