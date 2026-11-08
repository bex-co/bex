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
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Agent automations (ADR101): a schedule trigger plus a session template,
// owned by the member who created it. Every fire runs as that member
// (D4). The service is a set of methods on Service, like workspace skills
// (ADR099), and shares its workspace authorization and audit helpers.
const (
	maxWorkspaceAutomations = 50
	// AgentAutomationAuditTarget is the audit target of an automation write.
	AgentAutomationAuditTarget = "agent-automation"

	automationTriggerSchedule = "schedule"
	automationOverlapSkip     = "skip"
	defaultAutomationPageSize = 50

	// Pause reasons (ADR101 D7). The scheduler sets them; the service only
	// exposes them and clears them on resume.
	AutomationPausedRepeatedFailures  = "repeated_failures"
	AutomationPausedCreatorLostAccess = "creator_lost_access"
	AutomationPausedTemplateInvalid   = "template_invalid"

	// Named error codes.
	CodeAutomationInvalid           = "AGENT_AUTOMATION_INVALID"
	CodeAutomationNotFound          = "AGENT_AUTOMATION_NOT_FOUND"
	CodeAutomationConflict          = "AGENT_AUTOMATION_CONFLICT"
	CodeAutomationLimit             = "AGENT_AUTOMATION_LIMIT"
	CodeAutomationNotCreator        = "AGENT_AUTOMATION_NOT_CREATOR"
	CodeAutomationPaused            = "AGENT_AUTOMATION_PAUSED"
	CodeAutomationSpendLimitNeeded  = "AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED"
	CodeAutomationsUnavailableError = "AGENT_AUTOMATIONS_UNAVAILABLE"
)

// AutomationTrigger is one trigger of an automation. Phase 1 accepts exactly
// one, of type "schedule"; the array is the public contract so further types
// are additive.
type AutomationTrigger struct {
	Type     string              `json:"type" jsonschema:"schedule"`
	Schedule *AutomationSchedule `json:"schedule,omitempty"`
}

// AutomationAgentConfig is the agent part of a template.
type AutomationAgentConfig struct {
	Agent    string `json:"agent" jsonschema:"a supported agent profile, as on a session create"`
	Model    string `json:"model,omitempty"`
	Template string `json:"template,omitempty"`
}

// AutomationTemplate is the session an automation starts (ADR101 D5): the
// create-session request without resumeSessionId, attachments, inline
// secrets and idempotencyKey. Decoding rejects unknown fields, so those can
// never be stored.
type AutomationTemplate struct {
	Task            string                `json:"task"`
	Repo            string                `json:"repo,omitempty"`
	Branch          string                `json:"branch,omitempty"`
	Title           string                `json:"title,omitempty"`
	Tags            []string              `json:"tags,omitempty"`
	AgentConfig     AutomationAgentConfig `json:"agentConfig"`
	MaxSpendUSD     *float64              `json:"maxSpendUSD,omitempty" jsonschema:"required when the model is platform-metered"`
	SecretNames     []string              `json:"secretNames,omitempty"`
	Skills          []SkillInvocation     `json:"skills,omitempty"`
	OutputSchema    map[string]any        `json:"outputSchema,omitempty"`
	OutputRequired  *bool                 `json:"outputRequired,omitempty"`
	EgressAllowlist []string              `json:"egressAllowlist,omitempty"`
}

// UnmarshalJSON refuses any field a template does not define, with the named
// 400, so resumeSessionId, attachmentIds, idempotencyKey and the like are
// rejected on every surface.
func (t *AutomationTemplate) UnmarshalJSON(data []byte) error {
	type plain AutomationTemplate
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var out plain
	if err := dec.Decode(&out); err != nil {
		return automationInvalid("template", "the template is not a valid session template: "+err.Error())
	}
	*t = AutomationTemplate(out)
	return nil
}

// AgentAutomation is an automation as every surface shows it.
type AgentAutomation struct {
	ID                  string              `json:"id"`
	OwnerID             string              `json:"ownerId"`
	Name                string              `json:"name"`
	Enabled             bool                `json:"enabled"`
	CreatedBy           string              `json:"createdBy"`
	Triggers            []AutomationTrigger `json:"triggers"`
	Template            AutomationTemplate  `json:"template"`
	OverlapPolicy       string              `json:"overlapPolicy"`
	NextFireAt          *time.Time          `json:"nextFireAt"`
	LastFireAt          *time.Time          `json:"lastFireAt"`
	LastSessionID       string              `json:"lastSessionId,omitempty"`
	PausedReason        string              `json:"pausedReason,omitempty"`
	ConsecutiveFailures int                 `json:"consecutiveFailures"`
	Version             int                 `json:"version"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
}

// AgentAutomationList is one page of a workspace's automations, newest first.
// NextCursor is empty on the last page.
type AgentAutomationList struct {
	OwnerID     string            `json:"ownerId"`
	Automations []AgentAutomation `json:"automations"`
	NextCursor  string            `json:"nextCursor,omitempty"`
}

// AgentAutomationInput creates or patches an automation. On update an omitted
// field keeps its value; Triggers and Template replace the stored ones whole.
// IfVersion, when set, refuses the write unless the automation is still at it.
type AgentAutomationInput struct {
	Name      *string             `json:"name,omitempty" jsonschema:"lowercase letters, digits and single hyphens, at most 64"`
	Enabled   *bool               `json:"enabled,omitempty" jsonschema:"default true on create; resume a paused automation with resume, not enabled"`
	Triggers  []AutomationTrigger `json:"triggers,omitempty" jsonschema:"exactly one trigger, of type schedule"`
	Template  *AutomationTemplate `json:"template,omitempty" jsonschema:"the session each fire starts"`
	IfVersion *int                `json:"ifVersion,omitempty" jsonschema:"update only: refuse with AGENT_AUTOMATION_CONFLICT unless the automation is still at this version"`
}

// AutomationStore is the agent_automations table.
type AutomationStore interface {
	CreateAgentAutomation(ctx context.Context, in store.AgentAutomation) (store.AgentAutomation, error)
	GetAgentAutomation(ctx context.Context, id string) (store.AgentAutomation, error)
	GetAgentAutomationByName(ctx context.Context, workspaceID, name string) (store.AgentAutomation, error)
	ListAgentAutomations(ctx context.Context, workspaceID, createdBy, cursor string, limit int) ([]store.AgentAutomation, error)
	UpdateAgentAutomation(ctx context.Context, in store.AgentAutomation, ifVersion int) (store.AgentAutomation, error)
	DeleteAgentAutomation(ctx context.Context, id string) error
	SetAgentAutomationPaused(ctx context.Context, id string, paused bool, reason string) (store.AgentAutomation, error)

	// The scheduler's side (ADR101 D3): claim due automations and stale runs,
	// settle a run, read the run history and the latest started session, and
	// record a run-now fire.
	ClaimDueAutomations(ctx context.Context, now time.Time, limit int, lease time.Duration,
		advance func(a store.AgentAutomation) (fireAt time.Time, next *time.Time, missedTicks int, err error)) (store.ClaimResult, error)
	ClaimStaleRuns(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]store.ClaimedRun, error)
	SettleRun(ctx context.Context, automationID string, fireAt time.Time, outcome, sessionID, errText string) (int, error)
	ListRuns(ctx context.Context, automationID string, cursor time.Time, limit int) ([]store.AgentAutomationRun, error)
	LatestStartedSession(ctx context.Context, automationID string) (string, bool, error)
	InsertManualRun(ctx context.Context, automationID string, fireAt, now time.Time, lease time.Duration) (store.AgentAutomationRun, bool, error)
}

func automationInvalid(field, msg string) error {
	return core.NewBadRequestError(CodeAutomationInvalid, msg, map[string]any{"field": field})
}

func errAutomationNotFound(ref string) error {
	return core.NewNotFoundError(CodeAutomationNotFound, "no workspace automation has this id or name", map[string]any{"automation": ref})
}

func errAutomationConflict(msg string) error {
	return core.NewConflictError(CodeAutomationConflict, msg, nil)
}

func errAutomationNotCreator() error {
	return core.NewForbiddenError(CodeAutomationNotCreator,
		"only the member who created an automation can change what it runs; a workspace admin can pause, resume, delete or take it over", nil)
}

func errAutomationsUnavailable() error {
	return core.NewUnavailableError(CodeAutomationsUnavailableError, "agent automations need the database, which this deployment does not configure", nil)
}

func isAutomationID(ref string) bool {
	kind, ok := ids.KindOf(ref)
	return ok && kind == ids.AgentAutomation
}

func automationView(row store.AgentAutomation) (AgentAutomation, error) {
	out := AgentAutomation{
		ID: row.ID, OwnerID: row.WorkspaceID, Name: row.Name, Enabled: row.Enabled, CreatedBy: row.CreatedBy,
		OverlapPolicy: row.OverlapPolicy, NextFireAt: row.NextFireAt, LastFireAt: row.LastFireAt,
		LastSessionID: row.LastSessionID, PausedReason: row.PausedReason, ConsecutiveFailures: row.ConsecutiveFailures,
		Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if err := json.Unmarshal(row.Triggers, &out.Triggers); err != nil {
		return AgentAutomation{}, fmt.Errorf("agent automation %s: stored triggers: %w", row.ID, err)
	}
	if err := json.Unmarshal(row.Template, &out.Template); err != nil {
		return AgentAutomation{}, fmt.Errorf("agent automation %s: stored template: %w", row.ID, err)
	}
	return out, nil
}

// AgentAutomations lists the workspace's automations, newest first. mine
// keeps the caller's own (the dashboard's Mine tab). Anyone who can start a
// session may list: they see what runs in their workspace.
func (s *Service) AgentAutomations(ctx context.Context, ownerID string, mine bool, cursor string, limit int) (AgentAutomationList, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanCreate, ownerID)
	if err != nil {
		return AgentAutomationList{}, err
	}
	if s.Automations == nil {
		return AgentAutomationList{}, errAutomationsUnavailable()
	}
	if limit <= 0 {
		limit = defaultAutomationPageSize
	}
	limit = min(limit, 200)
	createdBy := ""
	if mine {
		if createdBy = core.SubjectFrom(ctx); createdBy == "" {
			return AgentAutomationList{}, core.ErrForbidden
		}
	}
	rows, err := s.Automations.ListAgentAutomations(ctx, workspaceID, createdBy, cursor, limit+1)
	if err != nil {
		return AgentAutomationList{}, err
	}
	out := AgentAutomationList{OwnerID: workspaceID, Automations: make([]AgentAutomation, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		out.NextCursor = rows[limit-1].ID
	}
	for _, row := range rows {
		view, err := automationView(row)
		if err != nil {
			return AgentAutomationList{}, err
		}
		out.Automations = append(out.Automations, view)
	}
	return out, nil
}

// AgentAutomation reads one automation by id or name.
func (s *Service) AgentAutomation(ctx context.Context, ownerID, ref string) (AgentAutomation, error) {
	row, _, err := s.scopeAutomation(ctx, ownerID, ref, core.RelCanCreate, "")
	if err != nil {
		return AgentAutomation{}, err
	}
	return automationView(row)
}

// CreateAgentAutomation adds an automation owned by the caller (can_create).
func (s *Service) CreateAgentAutomation(ctx context.Context, ownerID string, in AgentAutomationInput) (AgentAutomation, error) {
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanCreate, ownerID, AgentAutomationAuditTarget)
	if err != nil {
		return AgentAutomation{}, err
	}
	if s.Automations == nil {
		return AgentAutomation{}, errAutomationsUnavailable()
	}
	caller := core.SubjectFrom(ctx)
	if caller == "" {
		return AgentAutomation{}, core.ErrForbidden
	}
	if in.IfVersion != nil {
		return AgentAutomation{}, automationInvalid("ifVersion", "ifVersion applies to an update only")
	}
	if in.Name == nil {
		return AgentAutomation{}, automationInvalid("name", "an automation needs a name")
	}
	if in.Template == nil {
		return AgentAutomation{}, automationInvalid("template", "an automation needs a template")
	}
	row := store.AgentAutomation{
		ID: ids.New(ids.AgentAutomation), WorkspaceID: workspaceID, Name: *in.Name, Enabled: in.Enabled == nil || *in.Enabled,
		CreatedBy: caller, OverlapPolicy: automationOverlapSkip,
	}
	if err := validateAutomationName(row.Name); err != nil {
		return AgentAutomation{}, err
	}
	if err := s.applyAutomationDefinition(ctx, workspaceID, &row, in.Triggers, in.Template); err != nil {
		return AgentAutomation{}, err
	}
	if row.NextFireAt, err = s.nextFireFor(row); err != nil {
		return AgentAutomation{}, err
	}
	// The count check is advisory: two concurrent creates at 49 may both pass.
	if existing, err := s.Automations.ListAgentAutomations(ctx, workspaceID, "", "", maxWorkspaceAutomations); err != nil {
		return AgentAutomation{}, err
	} else if len(existing) >= maxWorkspaceAutomations {
		return AgentAutomation{}, core.NewConflictError(CodeAutomationLimit,
			fmt.Sprintf("a workspace holds at most %d automations", maxWorkspaceAutomations), map[string]any{"limit": maxWorkspaceAutomations})
	}
	created, err := s.Automations.CreateAgentAutomation(ctx, row)
	if errors.Is(err, store.ErrConflict) {
		return AgentAutomation{}, errAutomationConflict("the workspace already has an automation with this name")
	}
	if err != nil {
		return AgentAutomation{}, err
	}
	return automationView(created)
}

// UpdateAgentAutomation patches an automation. Only its creator may: a
// change to what runs would otherwise execute under another member's
// identity (ADR101 D4). Changing the triggers moves nextFireAt.
func (s *Service) UpdateAgentAutomation(ctx context.Context, ownerID, ref string, in AgentAutomationInput) (AgentAutomation, error) {
	row, workspaceID, err := s.scopeAutomation(ctx, ownerID, ref, core.RelCanCreate, AgentAutomationAuditTarget)
	if err != nil {
		return AgentAutomation{}, err
	}
	if row.CreatedBy != core.SubjectFrom(ctx) {
		return AgentAutomation{}, errAutomationNotCreator()
	}
	version := row.Version
	if in.IfVersion != nil {
		version = *in.IfVersion
	}
	wasEnabled := row.Enabled
	if in.Name != nil {
		if err := validateAutomationName(*in.Name); err != nil {
			return AgentAutomation{}, err
		}
		row.Name = *in.Name
	}
	if in.Enabled != nil {
		if *in.Enabled && row.PausedReason != "" {
			return AgentAutomation{}, core.NewConflictError(CodeAutomationPaused,
				"the automation is paused ("+row.PausedReason+"); resume it instead", map[string]any{"pausedReason": row.PausedReason})
		}
		row.Enabled = *in.Enabled
	}
	if in.Triggers != nil || in.Template != nil {
		triggers, template := in.Triggers, in.Template
		if triggers == nil {
			var cur AgentAutomation
			if cur, err = automationView(row); err != nil {
				return AgentAutomation{}, err
			}
			triggers = cur.Triggers
		}
		if template == nil {
			var cur AgentAutomation
			if cur, err = automationView(row); err != nil {
				return AgentAutomation{}, err
			}
			template = &cur.Template
		}
		if err := s.applyAutomationDefinition(ctx, workspaceID, &row, triggers, template); err != nil {
			return AgentAutomation{}, err
		}
	}
	switch {
	case !row.Enabled:
		row.NextFireAt = nil
	case in.Triggers != nil || !wasEnabled || row.NextFireAt == nil:
		if row.NextFireAt, err = s.nextFireFor(row); err != nil {
			return AgentAutomation{}, err
		}
	}
	updated, err := s.Automations.UpdateAgentAutomation(ctx, row, version)
	return s.automationResult(updated, err, ref)
}

// DeleteAgentAutomation removes an automation and its run history; the
// sessions it started stay. The creator or a workspace admin may.
func (s *Service) DeleteAgentAutomation(ctx context.Context, ownerID, ref string) error {
	row, _, err := s.scopeAutomationManaged(ctx, ownerID, ref)
	if err != nil {
		return err
	}
	if err := s.Automations.DeleteAgentAutomation(ctx, row.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errAutomationNotFound(ref)
		}
		return err
	}
	return nil
}

// PauseAgentAutomation stops an automation firing (the creator or an admin).
// A manual pause carries no pausedReason: it is a disabled automation.
func (s *Service) PauseAgentAutomation(ctx context.Context, ownerID, ref string) (AgentAutomation, error) {
	row, _, err := s.scopeAutomationManaged(ctx, ownerID, ref)
	if err != nil {
		return AgentAutomation{}, err
	}
	updated, err := s.Automations.SetAgentAutomationPaused(ctx, row.ID, true, "")
	return s.automationResult(updated, err, ref)
}

// ResumeAgentAutomation re-enables an automation (the creator or an admin):
// it clears the pause reason and the failure count and sets nextFireAt to the
// schedule's next fire from now. The template is validated again first, so a
// template_invalid pause is not undone for a template that still fails. A
// schedule with no future fire (a past once) is refused.
func (s *Service) ResumeAgentAutomation(ctx context.Context, ownerID, ref string) (AgentAutomation, error) {
	row, workspaceID, err := s.scopeAutomationManaged(ctx, ownerID, ref)
	if err != nil {
		return AgentAutomation{}, err
	}
	return s.reactivate(ctx, workspaceID, row, ref)
}

// TakeOverAgentAutomation makes the calling admin (can_manage) the creator of
// an automation and validates its template under the workspace as the admin
// sees it (ADR101 D4). A creator_lost_access pause is resumed in the same
// call, since the cause is gone; a failed validation refuses the whole
// take-over and changes nothing. Any other pause or a manual pause stays: use
// resume.
func (s *Service) TakeOverAgentAutomation(ctx context.Context, ownerID, ref string) (AgentAutomation, error) {
	row, workspaceID, err := s.scopeAutomation(ctx, ownerID, ref, core.RelCanManage, AgentAutomationAuditTarget)
	if err != nil {
		return AgentAutomation{}, err
	}
	caller := core.SubjectFrom(ctx)
	if caller == "" {
		return AgentAutomation{}, core.ErrForbidden
	}
	if row.PausedReason == AutomationPausedCreatorLostAccess {
		row.CreatedBy = caller
		return s.reactivate(ctx, workspaceID, row, ref)
	}
	view, err := automationView(row)
	if err != nil {
		return AgentAutomation{}, err
	}
	if _, err := s.validateAutomationTemplate(ctx, workspaceID, view.Template); err != nil {
		return AgentAutomation{}, err
	}
	row.CreatedBy = caller
	if row.Enabled {
		if row.NextFireAt, err = s.nextFireFor(row); err != nil {
			return AgentAutomation{}, err
		}
	}
	updated, err := s.Automations.UpdateAgentAutomation(ctx, row, row.Version)
	return s.automationResult(updated, err, ref)
}

// reactivate validates row, clears its pause and writes the new nextFireAt
// (and row's other fields, such as a new creator).
func (s *Service) reactivate(ctx context.Context, workspaceID string, row store.AgentAutomation, ref string) (AgentAutomation, error) {
	if err := s.validateStoredAutomation(ctx, workspaceID, row); err != nil {
		return AgentAutomation{}, err
	}
	row.Enabled = true
	next, err := s.nextFireFor(row)
	if err != nil {
		return AgentAutomation{}, err
	}
	resumed, err := s.Automations.SetAgentAutomationPaused(ctx, row.ID, false, "")
	if err != nil {
		return s.automationResult(resumed, err, ref)
	}
	resumed.CreatedBy, resumed.NextFireAt = row.CreatedBy, next
	updated, err := s.Automations.UpdateAgentAutomation(ctx, resumed, resumed.Version)
	return s.automationResult(updated, err, ref)
}

func (s *Service) automationResult(row store.AgentAutomation, err error, ref string) (AgentAutomation, error) {
	switch {
	case errors.Is(err, store.ErrAgentAutomationVersion):
		return AgentAutomation{}, errAutomationConflict("the automation changed since that version; read it again")
	case errors.Is(err, store.ErrConflict):
		return AgentAutomation{}, errAutomationConflict("the workspace already has an automation with this name")
	case errors.Is(err, store.ErrNotFound):
		return AgentAutomation{}, errAutomationNotFound(ref)
	case err != nil:
		return AgentAutomation{}, err
	}
	return automationView(row)
}

// scopeAutomation authorizes relation on the workspace owning ref and loads
// the automation, as scopeSkill does: an aut- id finds its workspace from the
// row, a name is looked up in ownerID (or the caller's workspace), and an
// automation of another workspace reads as not found. It returns the
// authorized workspace id.
func (s *Service) scopeAutomation(ctx context.Context, ownerID, ref, relation, target string) (store.AgentAutomation, string, error) {
	var row store.AgentAutomation
	var owner core.ResourceOwner
	if s.Automations != nil && isAutomationID(ref) {
		owner = func(ctx context.Context) ([]string, error) {
			var err error
			row, err = s.Automations.GetAgentAutomation(ctx, ref)
			return store.OwnerWorkspaces(row.WorkspaceID, err)
		}
	}
	ctx, err := s.ScopeByID(ctx, ownerID, owner)
	if err != nil {
		return store.AgentAutomation{}, "", err
	}
	workspaceID, err := s.AuthorizeWorkspaceTarget(ctx, relation, ownerID, target)
	if err != nil {
		return store.AgentAutomation{}, "", err
	}
	if s.Automations == nil {
		return store.AgentAutomation{}, "", errAutomationsUnavailable()
	}
	switch {
	case isAutomationID(ref):
		if row.ID == "" {
			row, err = s.Automations.GetAgentAutomation(ctx, ref)
		}
	case validAutomationName(ref):
		row, err = s.Automations.GetAgentAutomationByName(ctx, workspaceID, ref)
	default:
		return store.AgentAutomation{}, "", errAutomationNotFound(ref)
	}
	if errors.Is(err, store.ErrNotFound) || (err == nil && row.WorkspaceID != workspaceID) {
		return store.AgentAutomation{}, "", errAutomationNotFound(ref)
	}
	return row, workspaceID, err
}

// scopeAutomationManaged is scopeAutomation for pause, resume and delete: the
// creator (who must still be able to create sessions) or a workspace admin.
func (s *Service) scopeAutomationManaged(ctx context.Context, ownerID, ref string) (store.AgentAutomation, string, error) {
	row, workspaceID, err := s.scopeAutomation(ctx, ownerID, ref, core.RelCanCreate, AgentAutomationAuditTarget)
	if err != nil {
		return store.AgentAutomation{}, "", err
	}
	if row.CreatedBy != core.SubjectFrom(ctx) {
		if _, err := s.AuthorizeWorkspaceTarget(ctx, core.RelCanManage, workspaceID, AgentAutomationAuditTarget); err != nil {
			return store.AgentAutomation{}, "", err
		}
	}
	return row, workspaceID, nil
}

func validAutomationName(name string) bool { return validSkillName(name) }

func validateAutomationName(name string) error {
	if !validAutomationName(name) {
		return automationInvalid("name", fmt.Sprintf("an automation name is lowercase letters, digits and single hyphens, at most %d characters", maxSkillNameLen))
	}
	return nil
}

// applyAutomationDefinition validates triggers and template and stores their
// normalized JSON on row.
func (s *Service) applyAutomationDefinition(ctx context.Context, workspaceID string, row *store.AgentAutomation, triggers []AutomationTrigger, template *AutomationTemplate) error {
	normalized, err := validateAutomationTriggers(triggers, s.Now())
	if err != nil {
		return err
	}
	tpl, err := s.validateAutomationTemplate(ctx, workspaceID, *template)
	if err != nil {
		return err
	}
	if row.Triggers, err = json.Marshal(normalized); err != nil {
		return err
	}
	row.Template, err = json.Marshal(tpl)
	return err
}

// validateAutomationTriggers enforces phase 1 (exactly one schedule trigger)
// and returns the triggers with the schedule normalized.
func validateAutomationTriggers(in []AutomationTrigger, now time.Time) ([]AutomationTrigger, error) {
	if len(in) != 1 {
		return nil, automationInvalid("triggers", "an automation has exactly one trigger")
	}
	if in[0].Type != automationTriggerSchedule || in[0].Schedule == nil {
		return nil, automationInvalid("triggers", `the trigger must be {"type":"schedule","schedule":{...}}`)
	}
	sched, err := in[0].Schedule.Normalize()
	if err != nil {
		return nil, err
	}
	if err := sched.Validate(now); err != nil {
		return nil, err
	}
	return []AutomationTrigger{{Type: automationTriggerSchedule, Schedule: &sched}}, nil
}

// validateAutomationTemplate runs the checks a manual session create runs
// before it persists anything (validateCreateInput, resolveCreateModel,
// parseSessionCeiling) against workspaceID, and adds the ADR101 D7 rule that
// a platform-metered template carries maxSpendUSD. It returns the template
// with the create-time normalization (agent name, repo, branch, title, tags)
// applied; the workspace-dependent model resolution is not stored, so a later
// change of the workspace environment still applies at fire time.
func (s *Service) validateAutomationTemplate(ctx context.Context, workspaceID string, t AutomationTemplate) (AutomationTemplate, error) {
	req := CreateRequest{
		OwnerID: workspaceID, Repo: t.Repo, Branch: t.Branch,
		AgentConfig:     AgentConfig{Agent: t.AgentConfig.Agent, Model: t.AgentConfig.Model, Task: t.Task, Template: t.AgentConfig.Template},
		EgressAllowlist: t.EgressAllowlist, Title: t.Title, Tags: t.Tags,
		OutputSchema: t.OutputSchema, OutputRequired: t.OutputRequired, MaxSpendUSD: t.MaxSpendUSD,
		SessionSecretsInput: SessionSecretsInput{SecretNames: t.SecretNames}, Skills: t.Skills,
	}
	if _, err := s.validateCreateInput(&req); err != nil {
		return AutomationTemplate{}, err
	}
	out := t
	out.Repo, out.Branch, out.Title, out.Tags = req.Repo, req.Branch, req.Title, req.Tags
	out.AgentConfig = AutomationAgentConfig{Agent: req.AgentConfig.Agent, Model: req.AgentConfig.Model, Template: req.AgentConfig.Template}

	env, _, err := s.resolveCreateModel(ctx, workspaceID, &req)
	if err != nil {
		return AutomationTemplate{}, err
	}
	metered := sessionMetered(workspaceID, req.AgentConfig.Agent, env.ModelKeySet())
	if metered && t.MaxSpendUSD == nil {
		return AutomationTemplate{}, core.NewBadRequestError(CodeAutomationSpendLimitNeeded,
			"a template on the platform model needs maxSpendUSD: a recurring run with no spend ceiling is refused",
			map[string]any{"field": "template.maxSpendUSD"})
	}
	if _, err := parseSessionCeiling(t.MaxSpendUSD, metered); err != nil {
		return AutomationTemplate{}, err
	}
	return out, nil
}

// validateStoredAutomation re-runs the save-time validation on a stored row
// (resume, take-over): the schedule must still have a future fire and the
// template must still pass under the workspace as it is now.
func (s *Service) validateStoredAutomation(ctx context.Context, workspaceID string, row store.AgentAutomation) error {
	view, err := automationView(row)
	if err != nil {
		return err
	}
	if _, err := validateAutomationTriggers(view.Triggers, s.Now()); err != nil {
		return err
	}
	_, err = s.validateAutomationTemplate(ctx, workspaceID, view.Template)
	return err
}

// nextFireFor is the schedule's first fire after now for an enabled row, nil
// for a disabled one.
func (s *Service) nextFireFor(row store.AgentAutomation) (*time.Time, error) {
	if !row.Enabled {
		return nil, nil
	}
	var triggers []AutomationTrigger
	if err := json.Unmarshal(row.Triggers, &triggers); err != nil || len(triggers) != 1 || triggers[0].Schedule == nil {
		return nil, automationInvalid("triggers", "an automation has exactly one schedule trigger")
	}
	next, ok := triggers[0].Schedule.Next(s.Now())
	if !ok {
		return nil, core.NewBadRequestError(CodeAutomationSchedulePast, "the schedule has no future fire", map[string]any{"field": "triggers"})
	}
	return &next, nil
}
