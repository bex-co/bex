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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// skillsRoot is where the agent finds the workspace's skills: the image links
// it to sandboxfiles.AgentSkillsPath, where copySkills extracts them (ADR099
// D5). The driver's skillsRoot is the same path.
const skillsRoot = "/bex/skills"

const (
	// The overlay index is re-sent every turn, so the descriptions the
	// manifest carries for it are bounded: each one, and all of them.
	maxIndexDescriptionRunes = 200
	maxIndexDescriptionBytes = 8 << 10
)

var (
	skillCopyFailure = store.AgentSessionFailure{
		Reason: "the workspace's skills could not be copied into the session's sandbox",
		Code:   "SKILL_COPY_FAILED",
	}
	skillListFailure = store.AgentSessionFailure{
		Reason: "the workspace's skills could not be read",
		Code:   "SKILL_LIST_FAILED",
	}
	skillTooLargeFailure = store.AgentSessionFailure{
		Reason: "an invoked skill is too large once its arguments are substituted",
		Code:   "SKILL_TOO_LARGE",
	}
	errSkillExpansion = errors.New("invoked skill over the expanded size cap")
)

// skillManifest is BEX_AGENT_SKILLS: what the driver waits for, links into
// each engine's skills folder and indexes in the overlay.
type skillManifest struct {
	WaitMs int64                `json:"waitMs"`
	Root   string               `json:"root"`
	Skills []skillManifestSkill `json:"skills"`
}

type skillManifestSkill struct {
	Name string `json:"name"`
	// Description is the overlay index line, shortened to its budget; empty
	// once the budget is spent (the agent reads the file).
	Description string `json:"description,omitempty"`
	// UserOnly skills are applied only when invoked; Invoked marks the ones
	// this turn names, with their Arguments.
	UserOnly  bool   `json:"userOnly,omitempty"`
	Invoked   bool   `json:"invoked,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// deliveredSkill is one skill a dispatch copies, already rendered.
type deliveredSkill struct {
	manifest skillManifestSkill
	document string
}

func (s *Service) skillsEnabled() bool { return s.Skills != nil && s.AttachmentCopier != nil }

// dispatchSkills snapshots what a dispatch delivers: every enabled skill, and
// any skill the turn invoked even if it was disabled since. An invoked skill
// that was deleted is skipped; the overlay cannot point at it. A workspace
// without skills costs one metadata query.
func (s *Service) dispatchSkills(ctx context.Context, record store.AgentSession, currentTurn func() (store.AgentSessionTurn, error)) ([]deliveredSkill, error) {
	if !s.skillsEnabled() {
		return nil, nil
	}
	meta, err := s.Skills.ListAgentSkills(ctx, record.WorkspaceID)
	if err != nil || len(meta) == 0 {
		return nil, err
	}
	turn, err := currentTurn()
	if err != nil {
		return nil, err
	}
	invoked := map[string]store.TurnSkill{}
	names := make([]string, 0, len(turn.Skills))
	for _, skill := range turn.Skills {
		invoked[skill.Name] = skill
		names = append(names, skill.Name)
	}
	rows, err := s.Skills.AgentSkillsForDispatch(ctx, record.WorkspaceID, names)
	if err != nil {
		return nil, err
	}
	out := make([]deliveredSkill, 0, len(rows))
	total, indexBytes := 0, 0
	for _, row := range rows {
		inv, named := invoked[row.Name]
		delete(invoked, row.Name)
		document, err := deliveredDocument(row, inv, named)
		if err != nil {
			return nil, err
		}
		// The write path holds enabled skills under the cap; this guards a
		// snapshot taken across concurrent edits, never dropping an invoked one.
		if !named && total+len(document) > maxEnabledSkillBytes {
			log.Printf("agent-session dispatch: skill %s over the delivery cap, skipped (session=%s)", row.Name, record.ID)
			continue
		}
		total += len(document)
		entry := deliveredSkill{
			manifest: skillManifestSkill{Name: row.Name, UserOnly: userOnly(row.Triggers), Invoked: named, Arguments: inv.Arguments},
			document: document,
		}
		if desc := indexDescription(row.Description); indexBytes+len(desc) <= maxIndexDescriptionBytes {
			entry.manifest.Description = desc
			indexBytes += len(desc)
		}
		out = append(out, entry)
	}
	for name := range invoked {
		log.Printf("agent-session dispatch: invoked skill %s no longer exists (session=%s)", name, record.ID)
	}
	return out, nil
}

// indexDescription shortens a description to its overlay index budget.
func indexDescription(description string) string {
	if utf8.RuneCountInString(description) <= maxIndexDescriptionRunes {
		return description
	}
	return string([]rune(description)[:maxIndexDescriptionRunes-1]) + "…"
}

// deliveredDocument is the SKILL.md a sandbox receives: an invoked skill has
// its arguments substituted, and a user-only one carries Claude Code's
// disable-model-invocation key, forced true whatever the stored frontmatter
// says, so that engine does not apply it on its own.
func deliveredDocument(row store.AgentSkill, inv store.TurnSkill, invoked bool) (string, error) {
	if userOnly(row.Triggers) {
		row.Extra = withFrontmatterKey(row.Extra, "disable-model-invocation", "true", "!!bool")
	}
	body := ""
	if invoked {
		expanded, ok := substituteSkillArguments(row.Body, inv.Arguments)
		if !ok {
			return "", errSkillExpansion
		}
		body = expanded
	}
	return renderSkill(row, body), nil
}

// withFrontmatterKey sets key to a scalar in a skill's extra frontmatter,
// replacing any value it had. Unparsable extra is dropped, as renderSkill
// would drop it.
func withFrontmatterKey(extra, key, value, tag string) string {
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	var doc yaml.Node
	if extra != "" && yaml.Unmarshal([]byte(extra), &doc) == nil && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		mapping = doc.Content[0]
	}
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	replaced := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1], replaced = scalar, true
		}
	}
	if !replaced {
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, scalar)
	}
	encoded, err := encodeYAML(mapping)
	if err != nil {
		return ""
	}
	return encoded
}

var skillArgumentPattern = regexp.MustCompile(`\$ARGUMENTS|\$[1-9]`)

// substituteSkillArguments fills $ARGUMENTS with the whole argument string
// and $1-$9 with its whitespace-separated words, as `@skills:name args` does.
// ok is false when the result would pass maxSkillExpandedBytes; the size is
// bounded before anything is built.
func substituteSkillArguments(body, arguments string) (string, bool) {
	matches := skillArgumentPattern.FindAllStringIndex(body, -1)
	if len(body)+len(matches)*len(arguments) > maxSkillExpandedBytes {
		return "", false
	}
	words := strings.Fields(arguments)
	return skillArgumentPattern.ReplaceAllStringFunc(body, func(m string) string {
		if m == "$ARGUMENTS" {
			return arguments
		}
		if i, _ := strconv.Atoi(m[1:]); i <= len(words) {
			return words[i-1]
		}
		return ""
	}), true
}

func withSkillManifest(env map[string]string, skills []deliveredSkill) map[string]string {
	manifest := skillManifest{WaitMs: attachmentWait.Milliseconds(), Root: skillsRoot, Skills: make([]skillManifestSkill, len(skills))}
	for i, skill := range skills {
		manifest.Skills[i] = skill.manifest
	}
	encoded, _ := json.Marshal(manifest)
	return withEnv(env, "BEX_AGENT_SKILLS", string(encoded))
}

// copySkills writes the skills into the sandbox as one tar archive of
// `<name>/SKILL.md` entries, each directory before its file.
func (s *Service) copySkills(ctx context.Context, workspaceID, sessionID, sandboxID string, skills []deliveredSkill) error {
	if len(skills) == 0 {
		return nil
	}
	var archive bytes.Buffer
	if err := writeSkillTar(&archive, skills, time.Now()); err != nil {
		return err
	}
	data := archive.Bytes()
	return s.AttachmentCopier.UploadAgentSessionFiles(ctx, workspaceID, sessionID, sandboxID, sandboxfiles.AgentSkillsPath, func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(data))
	})
}

func writeSkillTar(w io.Writer, skills []deliveredSkill, at time.Time) error {
	archive := tar.NewWriter(w)
	for _, skill := range skills {
		if err := tarDir(archive, skill.manifest.Name, at); err != nil {
			return err
		}
		if err := tarFile(archive, skill.manifest.Name+"/SKILL.md", int64(len(skill.document)), at); err != nil {
			return err
		}
		if _, err := io.WriteString(archive, skill.document); err != nil {
			return err
		}
	}
	return archive.Close()
}
