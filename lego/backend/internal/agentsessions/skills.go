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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Workspace skills (ADR099): SKILL.md documents in the Agent Skills format,
// stored in Postgres, readable by every member who can start a session and
// managed by admins. Every dispatch delivers the enabled ones into the
// sandbox (skills_dispatch.go).
const (
	maxWorkspaceSkills     = 100
	maxSkillNameLen        = 64
	maxSkillDescriptionLen = 1024
	maxSkillBodyBytes      = 64 << 10
	maxSkillExtraBytes     = 16 << 10
	// maxEnabledSkillBytes bounds what one dispatch delivers: the sum of the
	// enabled skills' rendered documents.
	maxEnabledSkillBytes  = 1 << 20
	maxTurnSkills         = 5
	maxSkillArgumentBytes = 4 << 10
	// maxSkillExpandedBytes bounds an invoked skill's body once $ARGUMENTS
	// and $1-$9 are substituted: arguments repeated across a body would
	// otherwise multiply it.
	maxSkillExpandedBytes = 2 * maxSkillBodyBytes

	// AgentSkillAuditTarget is the audit target of a workspace skill write.
	AgentSkillAuditTarget = "agent-skill"

	skillTriggerUser  = "user"
	skillTriggerModel = "model"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// defaultSkillTriggers lets the agent apply a skill on its own as well as
// when asked, the Agent Skills default.
var defaultSkillTriggers = []string{skillTriggerUser, skillTriggerModel}

// AgentSkill is a workspace skill as every surface shows it. Body and Content
// (the whole SKILL.md: frontmatter and body) are on single reads only; a list
// carries the metadata.
type AgentSkill struct {
	ID          string `json:"id"`
	OwnerID     string `json:"ownerId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Triggers is ["user","model"] (the agent may apply it on its own) or
	// ["user"] (only when a prompt or the skills field invokes it).
	Triggers  []string  `json:"triggers"`
	Enabled   bool      `json:"enabled"`
	Body      string    `json:"body,omitempty"`
	Content   string    `json:"content,omitempty"`
	Version   int       `json:"version"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AgentSkillList is a workspace's skills by name, without bodies.
type AgentSkillList struct {
	OwnerID string       `json:"ownerId"`
	Skills  []AgentSkill `json:"skills"`
}

// AgentSkillInput creates or patches a skill. Content is a whole SKILL.md and
// cannot be combined with Name, Description, Triggers or Body; Enabled
// combines with either. On update an omitted field keeps its value, and
// IfVersion, when set, refuses the write unless the skill is still at it.
type AgentSkillInput struct {
	Name        *string  `json:"name,omitempty" jsonschema:"lowercase letters, digits and single hyphens, at most 64 (the Agent Skills name)"`
	Description *string  `json:"description,omitempty" jsonschema:"when to use the skill, one line of 1-1024 characters; the agent sees it on every turn"`
	Triggers    []string `json:"triggers,omitempty" jsonschema:"[\"user\",\"model\"] (default: the agent may apply it on its own) or [\"user\"] (only when invoked)"`
	Enabled     *bool    `json:"enabled,omitempty" jsonschema:"default true; a disabled skill is not delivered to sessions"`
	Body        *string  `json:"body,omitempty" jsonschema:"the instructions, markdown, 1 B to 64 KiB; $ARGUMENTS and $1-$9 take the invocation's arguments"`
	Content     *string  `json:"content,omitempty" jsonschema:"a whole SKILL.md (YAML frontmatter with name and description, then the body), instead of the separate fields"`
	IfVersion   *int     `json:"ifVersion,omitempty" jsonschema:"update only: refuse with AGENT_SKILL_CONFLICT unless the skill is still at this version"`
}

// SkillInvocation names a skill a turn must use (ADR099 D4), Devin's
// playbook_id and `@skills:name args`.
type SkillInvocation struct {
	Name      string `json:"name" jsonschema:"a workspace skill name (list_agent_skills)"`
	Arguments string `json:"arguments,omitempty" jsonschema:"substituted for $ARGUMENTS (and its words for $1-$9) in the skill, at most 4 KiB"`
}

// SkillStore is the agent_skills table.
type SkillStore interface {
	ListAgentSkills(ctx context.Context, workspaceID string) ([]store.AgentSkill, error)
	ListAgentSkillVersions(ctx context.Context, skillID string, before, limit int) ([]store.AgentSkillVersion, error)
	GetAgentSkillVersion(ctx context.Context, skillID string, version int) (store.AgentSkillVersion, error)
	AgentSkillsForDispatch(ctx context.Context, workspaceID string, invoked []string) ([]store.AgentSkill, error)
	GetAgentSkill(ctx context.Context, id string) (store.AgentSkill, error)
	GetAgentSkillByName(ctx context.Context, workspaceID, name string) (store.AgentSkill, error)
	CreateAgentSkill(ctx context.Context, in store.AgentSkill, limits store.AgentSkillLimits) (store.AgentSkill, error)
	UpdateAgentSkill(ctx context.Context, in store.AgentSkill, ifVersion int, limits store.AgentSkillLimits) (store.AgentSkill, error)
	DeleteAgentSkill(ctx context.Context, id string) error
}

func skillInvalid(field, msg string) error {
	return core.NewBadRequestError("AGENT_SKILL_INVALID", msg, map[string]any{"field": field})
}

func skillLimitError(msg string, limit int) error {
	return core.NewBadRequestError("AGENT_SKILL_LIMIT_EXCEEDED", msg, map[string]any{"limit": limit})
}

func errSkillNotFound(ref string) error {
	return core.NewNotFoundError("AGENT_SKILL_NOT_FOUND", "no workspace skill has this id or name", map[string]any{"skill": ref})
}

func errSkillConflict(msg string) error {
	return core.NewConflictError("AGENT_SKILL_CONFLICT", msg, nil)
}

func errSkillTriggers() error {
	return skillInvalid("triggers", `triggers are ["user","model"] or ["user"]`)
}

func errEnabledSkillBytes() error {
	return skillLimitError(fmt.Sprintf("a workspace's enabled skills total at most %d MiB", maxEnabledSkillBytes>>20), maxEnabledSkillBytes)
}

var skillLimits = store.AgentSkillLimits{Count: maxWorkspaceSkills, EnabledBytes: maxEnabledSkillBytes}

func errSkillsUnavailable() error {
	return core.NewUnavailableError("AGENT_SKILLS_UNAVAILABLE", "agent skills need the database, which this deployment does not configure", nil)
}

func isSkillID(ref string) bool {
	kind, ok := ids.KindOf(ref)
	return ok && kind == ids.AgentSkill
}

// skillView projects a row; full adds the body and the rendered SKILL.md.
func skillView(row store.AgentSkill, full bool) AgentSkill {
	out := AgentSkill{
		ID: row.ID, OwnerID: row.WorkspaceID, Name: row.Name, Description: row.Description,
		Triggers: slices.Clone(row.Triggers), Enabled: row.Enabled, Version: row.Version,
		UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if full {
		out.Body = row.Body
		out.Content = renderSkill(row, "")
	}
	return out
}

// AgentSkills lists the workspace's skills. Developers may list: they choose
// what to invoke.
func (s *Service) AgentSkills(ctx context.Context, ownerID string) (AgentSkillList, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return AgentSkillList{}, err
	}
	if s.Skills == nil {
		return AgentSkillList{}, errSkillsUnavailable()
	}
	rows, err := s.Skills.ListAgentSkills(ctx, workspaceID)
	if err != nil {
		return AgentSkillList{}, err
	}
	out := AgentSkillList{OwnerID: workspaceID, Skills: make([]AgentSkill, 0, len(rows))}
	for _, row := range rows {
		out.Skills = append(out.Skills, skillView(row, false))
	}
	return out, nil
}

// AgentSkill reads one skill by id or name, with its body and SKILL.md.
func (s *Service) AgentSkill(ctx context.Context, ownerID, ref string) (AgentSkill, error) {
	row, err := s.scopeSkill(ctx, ownerID, ref, core.RelCanCreate, "")
	if err != nil {
		return AgentSkill{}, err
	}
	return skillView(row, true), nil
}

// CreateAgentSkill adds a workspace skill (admins only).
func (s *Service) CreateAgentSkill(ctx context.Context, ownerID string, in AgentSkillInput) (AgentSkill, error) {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, ownerID, AgentSkillAuditTarget)
	if err != nil {
		return AgentSkill{}, err
	}
	if s.Skills == nil {
		return AgentSkill{}, errSkillsUnavailable()
	}
	if in.IfVersion != nil {
		return AgentSkill{}, skillInvalid("ifVersion", "ifVersion applies to an update only")
	}
	row := store.AgentSkill{WorkspaceID: workspaceID, Triggers: slices.Clone(defaultSkillTriggers), Enabled: true}
	if err := applySkillInput(&row, in, true); err != nil {
		return AgentSkill{}, err
	}
	row.ID, row.UpdatedBy = ids.New(ids.AgentSkill), core.SubjectFrom(ctx)
	stampSkillVersion(&row, true)
	created, err := s.Skills.CreateAgentSkill(ctx, row, skillLimits)
	switch {
	case errors.Is(err, store.ErrAgentSkillLimit):
		return AgentSkill{}, skillLimitError(fmt.Sprintf("a workspace holds at most %d skills", maxWorkspaceSkills), maxWorkspaceSkills)
	case errors.Is(err, store.ErrAgentSkillBytes):
		return AgentSkill{}, errEnabledSkillBytes()
	case errors.Is(err, store.ErrConflict):
		return AgentSkill{}, errSkillConflict("the workspace already has a skill with this name")
	case err != nil:
		return AgentSkill{}, err
	}
	return skillView(created, true), nil
}

// UpdateAgentSkill patches a workspace skill (admins only). Sessions apply
// the new version from their next turn.
func (s *Service) UpdateAgentSkill(ctx context.Context, ownerID, ref string, in AgentSkillInput) (AgentSkill, error) {
	row, err := s.scopeSkill(ctx, ownerID, ref, core.RelCanManage, AgentSkillAuditTarget)
	if err != nil {
		return AgentSkill{}, err
	}
	// The store's UPDATE matches the version, so a stale ifVersion and a
	// concurrent edit are refused the same way.
	version := row.Version
	if in.IfVersion != nil {
		version = *in.IfVersion
	}
	if err := applySkillInput(&row, in, false); err != nil {
		return AgentSkill{}, err
	}
	row.UpdatedBy = core.SubjectFrom(ctx)
	stampSkillVersion(&row, false)
	updated, err := s.Skills.UpdateAgentSkill(ctx, row, version, skillLimits)
	switch {
	case errors.Is(err, store.ErrAgentSkillBytes):
		return AgentSkill{}, errEnabledSkillBytes()
	case errors.Is(err, store.ErrAgentSkillVersion):
		return AgentSkill{}, errSkillConflict("the skill changed since that version; read it again")
	case errors.Is(err, store.ErrConflict):
		return AgentSkill{}, errSkillConflict("the workspace already has a skill with this name")
	case errors.Is(err, store.ErrNotFound):
		return AgentSkill{}, errSkillNotFound(ref)
	case err != nil:
		return AgentSkill{}, err
	}
	return skillView(updated, true), nil
}

// DeleteAgentSkill removes a workspace skill (admins only). A turn already
// accepted with it still invokes it by name and finds it gone at dispatch.
func (s *Service) DeleteAgentSkill(ctx context.Context, ownerID, ref string) error {
	row, err := s.scopeSkill(ctx, ownerID, ref, core.RelCanManage, AgentSkillAuditTarget)
	if err != nil {
		return err
	}
	if err := s.Skills.DeleteAgentSkill(ctx, row.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errSkillNotFound(ref)
		}
		return err
	}
	return nil
}

// scopeSkill authorizes relation on the workspace owning ref and loads the
// skill. A skl- id finds its workspace from the row, as Render's by-id
// routes do; a name is looked up in ownerID (or the caller's workspace). A
// skill of another workspace reads as not found.
func (s *Service) scopeSkill(ctx context.Context, ownerID, ref, relation, target string) (store.AgentSkill, error) {
	var row store.AgentSkill
	var owner core.ResourceOwner
	if s.Skills != nil && isSkillID(ref) {
		owner = func(ctx context.Context) ([]string, error) {
			var err error
			row, err = s.Skills.GetAgentSkill(ctx, ref)
			return store.OwnerWorkspaces(row.WorkspaceID, err)
		}
	}
	ctx, err := s.ScopeByID(ctx, ownerID, owner)
	if err != nil {
		return store.AgentSkill{}, err
	}
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, relation, ownerID, target)
	if err != nil {
		return store.AgentSkill{}, err
	}
	if s.Skills == nil {
		return store.AgentSkill{}, errSkillsUnavailable()
	}
	switch {
	case isSkillID(ref):
		if row.ID == "" {
			row, err = s.Skills.GetAgentSkill(ctx, ref)
		}
	case validSkillName(ref):
		row, err = s.Skills.GetAgentSkillByName(ctx, workspaceID, ref)
	default:
		return store.AgentSkill{}, errSkillNotFound(ref)
	}
	if errors.Is(err, store.ErrNotFound) || (err == nil && row.WorkspaceID != workspaceID) {
		return store.AgentSkill{}, errSkillNotFound(ref)
	}
	return row, err
}

// applySkillInput validates in onto row. create requires a name, a
// description and a body, from the fields or from content.
func applySkillInput(row *store.AgentSkill, in AgentSkillInput, create bool) error {
	if in.Content != nil {
		if in.Name != nil || in.Description != nil || in.Triggers != nil || in.Body != nil {
			return skillInvalid("content", "content is a whole SKILL.md: send it instead of name, description, triggers and body, not with them")
		}
		doc, err := parseSkill(*in.Content)
		if err != nil {
			return err
		}
		row.Name, row.Description, row.Body, row.Extra = doc.Name, doc.Description, doc.Body, doc.Extra
		row.Triggers = slices.Clone(defaultSkillTriggers)
		if doc.Triggers != nil {
			row.Triggers = doc.Triggers
		}
	} else {
		if in.Name != nil {
			row.Name = *in.Name
		}
		if in.Description != nil {
			row.Description = *in.Description
		}
		if in.Body != nil {
			row.Body = *in.Body
		}
		if in.Triggers != nil {
			row.Triggers = in.Triggers
		}
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if create && row.Name == "" {
		return skillInvalid("name", "a skill needs a name")
	}
	return validateSkill(row)
}

func validateSkill(row *store.AgentSkill) error {
	if !validSkillName(row.Name) {
		return skillInvalid("name", fmt.Sprintf("a skill name is lowercase letters, digits and single hyphens, at most %d characters", maxSkillNameLen))
	}
	row.Description = strings.TrimSpace(row.Description)
	// One line: the agent reads it in the overlay index on every turn.
	if row.Description == "" || utf8.RuneCountInString(row.Description) > maxSkillDescriptionLen || !validSkillLine(row.Description) {
		return skillInvalid("description", fmt.Sprintf("a skill description is one line of 1 to %d characters", maxSkillDescriptionLen))
	}
	if strings.TrimSpace(row.Body) == "" || len(row.Body) > maxSkillBodyBytes || !validSkillText(row.Body) {
		return skillInvalid("body", fmt.Sprintf("a skill body is 1 B to %d KiB of text", maxSkillBodyBytes>>10))
	}
	if len(row.Extra) > maxSkillExtraBytes {
		return skillInvalid("content", fmt.Sprintf("a skill's other frontmatter is at most %d KiB", maxSkillExtraBytes>>10))
	}
	triggers, err := normalizeTriggers(row.Triggers)
	if err != nil {
		return err
	}
	row.Triggers = triggers
	return nil
}

func validSkillName(s string) bool {
	return len(s) <= maxSkillNameLen && skillNamePattern.MatchString(s)
}

func validSkillText(s string) bool { return utf8.ValidString(s) && !strings.ContainsRune(s, 0) }

// validSkillLine is validSkillText without control characters or Unicode
// line and paragraph separators: a description is one line of the overlay
// index and of the frontmatter.
func validSkillLine(s string) bool {
	return validSkillText(s) && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
	})
}

func normalizeTriggers(in []string) ([]string, error) {
	set := map[string]bool{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != skillTriggerUser && t != skillTriggerModel {
			return nil, errSkillTriggers()
		}
		set[t] = true
	}
	switch {
	case set[skillTriggerUser] && set[skillTriggerModel]:
		return slices.Clone(defaultSkillTriggers), nil
	case set[skillTriggerUser] && len(set) == 1:
		return []string{skillTriggerUser}, nil
	}
	return nil, errSkillTriggers()
}

func userOnly(triggers []string) bool { return !slices.Contains(triggers, skillTriggerModel) }

// validateSkillInvocations checks a create or steer's skills before anything
// is read.
func validateSkillInvocations(in []SkillInvocation) error {
	if len(in) > maxTurnSkills {
		return skillLimitError(fmt.Sprintf("a turn invokes at most %d skills", maxTurnSkills), maxTurnSkills)
	}
	seen := map[string]bool{}
	for _, inv := range in {
		if !validSkillName(inv.Name) {
			return skillInvalid("skills.name", "a skill name is lowercase letters, digits and single hyphens")
		}
		if len(inv.Arguments) > maxSkillArgumentBytes || !validSkillLine(inv.Arguments) {
			return skillInvalid("skills.arguments", fmt.Sprintf("skill arguments are at most %d KiB of text", maxSkillArgumentBytes>>10))
		}
		if seen[inv.Name] {
			return skillInvalid("skills.name", "a skill is invoked twice")
		}
		seen[inv.Name] = true
	}
	return nil
}

// resolveSkillInvocations requires every invoked skill to exist and be
// enabled in the workspace, and its arguments to expand within
// maxSkillExpandedBytes. A user-only skill may be invoked: that is what it
// is for. A deployment that cannot deliver skills refuses an invocation
// rather than accepting one it would drop.
func (s *Service) resolveSkillInvocations(ctx context.Context, workspaceID string, in []SkillInvocation) ([]store.TurnSkill, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if !s.skillsEnabled() {
		return nil, errSkillsUnavailable()
	}
	out := make([]store.TurnSkill, 0, len(in))
	for _, inv := range in {
		row, err := s.Skills.GetAgentSkillByName(ctx, workspaceID, inv.Name)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !row.Enabled) {
			return nil, errSkillNotFound(inv.Name)
		}
		if err != nil {
			return nil, err
		}
		if _, ok := substituteSkillArguments(row.Body, inv.Arguments); !ok {
			return nil, skillLimitError(fmt.Sprintf("a skill with its arguments substituted is at most %d KiB", maxSkillExpandedBytes>>10), maxSkillExpandedBytes)
		}
		out = append(out, store.TurnSkill{Name: inv.Name, Arguments: inv.Arguments})
	}
	return out, nil
}

// turnSkills projects a turn's invoked skills, never null.
func turnSkills(in []store.TurnSkill) []SkillInvocation {
	out := make([]SkillInvocation, len(in))
	for i, skill := range in {
		out[i] = SkillInvocation{Name: skill.Name, Arguments: skill.Arguments}
	}
	return out
}

// skillDocument is a parsed SKILL.md. Triggers is nil when the frontmatter
// names none; Extra is every other frontmatter key, as YAML.
type skillDocument struct {
	Name, Description, Body, Extra string
	Triggers                       []string
}

const frontmatterFence = "---"

// parseSkill reads a SKILL.md: a `---` fenced YAML frontmatter mapping, then
// the body.
func parseSkill(content string) (skillDocument, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	rest, ok := strings.CutPrefix(content, frontmatterFence+"\n")
	if !ok {
		return skillDocument{}, skillInvalid("content", "a SKILL.md starts with --- and YAML frontmatter")
	}
	var front, body string
	if end := strings.Index(rest, "\n"+frontmatterFence+"\n"); end >= 0 {
		front, body = rest[:end], rest[end+len(frontmatterFence)+2:]
	} else if trimmed, found := strings.CutSuffix(rest, "\n"+frontmatterFence); found {
		front = trimmed
	} else {
		return skillDocument{}, skillInvalid("content", "the SKILL.md frontmatter has no closing ---")
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(front), &node); err != nil {
		return skillDocument{}, skillInvalid("content", "the SKILL.md frontmatter is not valid YAML")
	}
	if len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return skillDocument{}, skillInvalid("content", "the SKILL.md frontmatter is a YAML mapping")
	}
	doc := skillDocument{Body: body}
	extra := &yaml.Node{Kind: yaml.MappingNode}
	pairs := node.Content[0].Content
	for i := 0; i+1 < len(pairs); i += 2 {
		key, value := pairs[i], pairs[i+1]
		switch key.Value {
		case "name", "description":
			if value.Kind != yaml.ScalarNode {
				return skillDocument{}, skillInvalid("content", "the frontmatter "+key.Value+" is a string")
			}
			if key.Value == "name" {
				doc.Name = value.Value
			} else {
				doc.Description = value.Value
			}
		case "triggers":
			var triggers []string
			if err := value.Decode(&triggers); err != nil {
				return skillDocument{}, errSkillTriggers()
			}
			doc.Triggers = triggers
			if doc.Triggers == nil {
				doc.Triggers = []string{}
			}
		default:
			extra.Content = append(extra.Content, key, value)
		}
	}
	if len(extra.Content) > 0 {
		encoded, err := encodeYAML(extra)
		if err != nil {
			return skillDocument{}, skillInvalid("content", "the SKILL.md frontmatter is not valid YAML")
		}
		doc.Extra = encoded
	}
	return doc, nil
}

// renderSkill writes a row back as a SKILL.md. A non-empty body replaces the
// stored one: the delivered copy of an invoked skill has its arguments
// substituted.
func renderSkill(row store.AgentSkill, body string) string {
	front := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, value *yaml.Node) {
		front.Content = append(front.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
	}
	add("name", &yaml.Node{Kind: yaml.ScalarNode, Value: row.Name})
	add("description", &yaml.Node{Kind: yaml.ScalarNode, Value: row.Description})
	if userOnly(row.Triggers) {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, t := range row.Triggers {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: t})
		}
		add("triggers", seq)
	}
	if row.Extra != "" {
		var extra yaml.Node
		if err := yaml.Unmarshal([]byte(row.Extra), &extra); err == nil && len(extra.Content) > 0 && extra.Content[0].Kind == yaml.MappingNode {
			front.Content = append(front.Content, extra.Content[0].Content...)
		}
	}
	encoded, _ := encodeYAML(front)
	if body == "" {
		body = row.Body
	}
	return frontmatterFence + "\n" + encoded + frontmatterFence + "\n" + body
}

func encodeYAML(node *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
