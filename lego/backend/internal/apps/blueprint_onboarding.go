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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Blueprint onboarding (w2/m256; Render CLI v2.35 `render ea
// blueprints create | plan | approve`): creating a Blueprint plans its first
// sync without applying it, a sync can be re-planned at the branch head, and
// approving applies the newest planned sync — with the values of the
// sync:false env vars it creates. A Blueprint stays `created`, and auto-sync
// does not run, until a sync is approved — or until a plan finds nothing to
// change, which needs no approval (w2/m256 t014).

// Render's planning outcomes.
const (
	PlanningPlanned   = "planned"
	PlanningNoChanges = "no_changes"
	PlanningFailed    = "failed"
)

// blueprintPlanStore is the persistence onboarding needs; *store.PGStore
// satisfies it.
type blueprintPlanStore interface {
	CreatePlannedBlueprint(ctx context.Context, b store.Blueprint) (store.Blueprint, bool, error)
	BlueprintNameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error)
	InsertPlannedBlueprintSync(ctx context.Context, p store.PlannedBlueprintSync) (store.PlannedBlueprintSync, error)
	LatestBlueprintPlan(ctx context.Context, blueprintID string) (store.PlannedBlueprintSync, error)
	BlueprintPlannedSync(ctx context.Context, blueprintID, syncID string) (store.PlannedBlueprintSync, error)
	SettleNoChangesPlan(ctx context.Context, blueprintID, tenantID, manifest string) (store.Blueprint, error)
}

// RenderBlueprintSource is Render's createBlueprintRequest.source.
type RenderBlueprintSource struct {
	Repo   string  `json:"repo"`
	Branch *string `json:"branch,omitempty"`
	Path   *string `json:"path,omitempty"`
}

// RenderCreateBlueprintRequest is Render's createBlueprintRequest.
type RenderCreateBlueprintRequest struct {
	Name              string                `json:"name"`
	OwnerID           string                `json:"ownerId"`
	Source            RenderBlueprintSource `json:"source"`
	AutoSync          *bool                 `json:"autoSync,omitempty"`
	ExistingResources *string               `json:"existingResources,omitempty"`
}

// BlueprintNotice is Render's planning error or warning.
type BlueprintNotice struct {
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
	Path     *string `json:"path,omitempty"`
	Line     *int    `json:"line,omitempty"`
	Column   *int    `json:"column,omitempty"`
}

// ManualEnvVar names one sync:false value a sync needs; Value is set only on
// an approval's supplied values.
type ManualEnvVar struct {
	ServiceType string  `json:"serviceType"`
	ServiceName string  `json:"serviceName"`
	Key         string  `json:"key"`
	Value       *string `json:"value,omitempty"`
}

// BlueprintCommitRef is Render's commitRef.
type BlueprintCommitRef struct {
	ID string `json:"id"`
}

// SyncWithPlan is Render's persisted planning attempt.
type SyncWithPlan struct {
	ID                    string             `json:"id"`
	Commit                BlueprintCommitRef `json:"commit"`
	CommitID              string             `json:"-"`
	RawPlanJSON           *string            `json:"-"`
	State                 string             `json:"state"`
	StartedAt             *time.Time         `json:"startedAt,omitempty"`
	CompletedAt           *time.Time         `json:"completedAt,omitempty"`
	DashboardURL          string             `json:"dashboardUrl"`
	RawPlan               any                `json:"rawPlan"`
	RequiredManualEnvVars []ManualEnvVar     `json:"requiredManualEnvVars"`
	// Actions are rawPlan's actions, typed for bex's own GraphQL.
	Actions []BlueprintPlanAction `json:"-"`
}

// BlueprintPlanResult is Render's createBlueprintSyncResponse; the create
// response adds the Blueprint.
type BlueprintPlanResult struct {
	Blueprint       *renderBlueprint  `json:"blueprint,omitempty"`
	PlanningOutcome string            `json:"planningOutcome"`
	Notices         []BlueprintNotice `json:"notices"`
	Sync            *SyncWithPlan     `json:"sync"`
}

// ApprovedSync is Render's approvedSync.
type ApprovedSync struct {
	ID           string             `json:"id"`
	Commit       BlueprintCommitRef `json:"commit"`
	CommitID     string             `json:"-"`
	State        string             `json:"state"`
	DashboardURL string             `json:"dashboardUrl"`
	StartedAt    *time.Time         `json:"startedAt,omitempty"`
	CompletedAt  *time.Time         `json:"completedAt,omitempty"`
}

func (s *Service) blueprintPlans() (blueprintPlanStore, error) {
	if s.Blueprints == nil {
		return nil, ErrBlueprintsUnavailable
	}
	plans, ok := s.Blueprints.(blueprintPlanStore)
	if !ok {
		return nil, ErrBlueprintsUnavailable
	}
	return plans, nil
}

// CreatePlannedBlueprint is Render's create: it saves the Blueprint in the
// created status and plans its first sync. A planning failure still saves the
// Blueprint (with an error sync when the file was read).
func (s *Service) CreatePlannedBlueprint(ctx context.Context, req RenderCreateBlueprintRequest) (BlueprintPlanResult, error) {
	if req.OwnerID != "" {
		ctx = core.WithWorkspace(ctx, req.OwnerID)
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return BlueprintPlanResult{}, err
	}
	plans, err := s.blueprintPlans()
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	switch {
	case req.Name == "":
		return BlueprintPlanResult{}, fmt.Errorf("%w: name is required", core.ErrBadRequest)
	case strings.TrimSpace(req.Source.Repo) == "":
		return BlueprintPlanResult{}, fmt.Errorf("%w: source.repo is required", core.ErrBadRequest)
	}
	mode := store.BlueprintExistingAdopt
	if req.ExistingResources != nil {
		switch *req.ExistingResources {
		case store.BlueprintExistingAdopt, store.BlueprintExistingCreateNew:
			mode = *req.ExistingResources
		default:
			return BlueprintPlanResult{}, fmt.Errorf("%w: existingResources must be adopt or create_new", core.ErrBadRequest)
		}
	}
	if s.GitFetcher == nil {
		return BlueprintPlanResult{}, ErrBlueprintFetchUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	if taken, err := plans.BlueprintNameTaken(ctx, tenantID, req.Name, ""); err != nil {
		return BlueprintPlanResult{}, err
	} else if taken {
		return BlueprintPlanResult{}, core.NewConflictError("BLUEPRINT_NAME_TAKEN",
			fmt.Sprintf("a Blueprint named %q already exists in this workspace", req.Name), map[string]any{"name": req.Name})
	}
	branch := DefaultBlueprintBranch
	if req.Source.Branch != nil && strings.TrimSpace(*req.Source.Branch) != "" {
		branch = strings.TrimSpace(*req.Source.Branch)
	}
	path := ""
	if req.Source.Path != nil {
		path = *req.Source.Path
	}
	contents, commitSHA, discoveredPath, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, req.Source.Repo, branch, path)
	if err != nil {
		return BlueprintPlanResult{}, blueprintCreateFetchError(err, path, branch)
	}
	if existing, err := s.Blueprints.GetBlueprintByRepo(ctx, tenantID, req.Source.Repo, branch); err == nil {
		return BlueprintPlanResult{}, errBlueprintAlreadyConnected(existing)
	} else if !errors.Is(err, store.ErrNotFound) {
		return BlueprintPlanResult{}, err
	}
	autoSync := true
	if req.AutoSync != nil {
		autoSync = *req.AutoSync
	}
	b, revived, err := plans.CreatePlannedBlueprint(ctx, store.Blueprint{
		TenantID: tenantID, Name: req.Name, Repo: req.Source.Repo, Branch: branch, Path: discoveredPath,
		AutoSync: autoSync, ExistingResources: mode,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			if existing, getErr := s.Blueprints.GetBlueprintByRepo(ctx, tenantID, req.Source.Repo, branch); getErr == nil {
				return BlueprintPlanResult{}, errBlueprintAlreadyConnected(existing)
			}
		}
		return BlueprintPlanResult{}, err
	}
	// A revived row keeps its sync history: its planned sync marks where this
	// connection's history begins, as the legacy create's run does (w4/120).
	note := ""
	if revived {
		note = reconnectionNote(req.Source.Repo, branch)
	}
	result, err := s.planBlueprintSync(ctx, plans, &b, contents, commitSHA, note)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	bp := toRenderBlueprint(toBlueprintView(b))
	result.Blueprint = &bp
	return result, nil
}

// DefaultBlueprintBranch is the branch a create without source.branch reads.
// Render reads the repository's default branch; bex's GitHub reader resolves
// branches by name, so it uses the conventional default.
const DefaultBlueprintBranch = "main"

// PlanBlueprint plans a sync of an existing Blueprint at its branch head.
func (s *Service) PlanBlueprint(ctx context.Context, bpID, ownerID string) (BlueprintPlanResult, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	plans, err := s.blueprintPlans()
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return BlueprintPlanResult{}, store.MapError(err)
	}
	if b.Repo == "" {
		return BlueprintPlanResult{}, fmt.Errorf("%w: planning a sync requires a Git-backed Blueprint", core.ErrBadRequest)
	}
	if s.GitFetcher == nil {
		return BlueprintPlanResult{}, ErrBlueprintFetchUnavailable
	}
	contents, commitSHA, _, fetchErr := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, b.Repo, b.Branch, b.Path)
	if fetchErr != nil {
		_, message := classifyBlueprintFetchError(fetchErr, b.Path, b.Branch)
		return s.recordFailedPlan(ctx, plans, b, commitSHA, "", []BlueprintNotice{{Severity: "error", Message: message}})
	}
	return s.planBlueprintSync(ctx, plans, &b, contents, commitSHA, "")
}

// planBlueprintSync validates manifest against the resources the workspace has
// now — never against an earlier sync, so a defined resource deleted since is
// planned as a create, as on Render — and persists the outcome: a planned sync
// awaiting approval, an error sync carrying the validation notices, or, when
// no resource would change, nothing to approve: a waiting plan is superseded
// and a `created` Blueprint is in sync (*b is updated to match), so a manual
// sync or auto-sync can run it from then on (w2/m256 t014).
// note is recorded on the sync row it writes ("" for none).
func (s *Service) planBlueprintSync(ctx context.Context, plans blueprintPlanStore, b *store.Blueprint, manifest, commitSHA, note string) (BlueprintPlanResult, error) {
	validation, err := s.blueprintValidationFor(ctx, b.Repo, b.Branch, manifest, b.ID)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	if !validation.Valid {
		notices := make([]BlueprintNotice, 0, len(validation.Errors))
		for _, e := range validation.Errors {
			notices = append(notices, BlueprintNotice{Severity: "error", Message: e.Error, Path: e.Path, Line: e.Line, Column: e.Column})
		}
		return s.recordFailedPlan(ctx, plans, *b, commitSHA, note, notices)
	}
	// Only a plan against the workspace's current resources can say what
	// would change; a structural plan (state unreadable) cannot.
	if validation.Plan == nil || validation.Plan.Mode != "current_state" {
		return BlueprintPlanResult{}, core.NewUnavailableError("BLUEPRINT_PLAN_UNAVAILABLE",
			"the workspace's current resources could not be read, so the sync cannot be planned; try again", nil)
	}
	actions := validation.Plan.Actions
	if !planChangesAnything(actions) {
		if b.Status == store.BlueprintStatusCreated {
			// Leaving created makes the name an approved Blueprint's, which
			// approval would have checked.
			if taken, err := plans.BlueprintNameTaken(ctx, b.TenantID, b.Name, b.ID); err != nil {
				return BlueprintPlanResult{}, err
			} else if taken {
				return BlueprintPlanResult{}, errBlueprintNameTakenAtApproval(b.Name)
			}
		}
		settled, err := plans.SettleNoChangesPlan(ctx, b.ID, b.TenantID, manifest)
		if err != nil {
			return BlueprintPlanResult{}, store.MapError(err)
		}
		*b = settled
		return BlueprintPlanResult{PlanningOutcome: PlanningNoChanges, Notices: []BlueprintNotice{}}, nil
	}
	required, err := requiredManualEnvVars(*b, manifest, actions)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	rawRequired, err := json.Marshal(required)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	rawActions, err := json.Marshal(actions)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	now := s.Now().UTC()
	planned, err := plans.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
		BlueprintSync: store.BlueprintSync{BlueprintID: b.ID, CommitID: commitSHA, State: store.BlueprintSyncStateCreated, StartedAt: now, Note: note},
		Path:          b.Path, RequiredManualEnvVars: rawRequired, PlanActions: rawActions,
	})
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	return BlueprintPlanResult{PlanningOutcome: PlanningPlanned, Notices: []BlueprintNotice{}, Sync: s.syncWithPlan(*b, planned, actions, required)}, nil
}

// recordFailedPlan persists a planning failure as an error sync.
func (s *Service) recordFailedPlan(ctx context.Context, plans blueprintPlanStore, b store.Blueprint, commitSHA, note string, notices []BlueprintNotice) (BlueprintPlanResult, error) {
	messages := make([]string, 0, len(notices))
	for _, n := range notices {
		messages = append(messages, n.Message)
	}
	summary := "planning failed: " + strings.Join(messages, "; ")
	now := s.Now().UTC()
	failed, err := plans.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
		BlueprintSync: store.BlueprintSync{BlueprintID: b.ID, CommitID: commitSHA, State: store.BlueprintSyncStateError,
			StartedAt: now, CompletedAt: &now, ErrorMessage: &summary, Note: note},
		Path: b.Path,
	})
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	return BlueprintPlanResult{PlanningOutcome: PlanningFailed, Notices: notices, Sync: s.syncWithPlan(b, failed, nil, []ManualEnvVar{})}, nil
}

// planChangesAnything reports whether approving the plan would change a
// resource.
func planChangesAnything(actions []BlueprintPlanAction) bool {
	return slices.ContainsFunc(actions, func(a BlueprintPlanAction) bool {
		return a.Operation == BlueprintPlanCreate || a.Operation == BlueprintPlanUpdate || a.Operation == BlueprintPlanDetach
	})
}

// requiredManualEnvVars lists the sync:false values without a manifest value
// on services the plan creates — the values approval must supply (an existing
// service keeps the value it already has).
func requiredManualEnvVars(b store.Blueprint, manifest string, actions []BlueprintPlanAction) ([]ManualEnvVar, error) {
	parsed, _, err := compileStack(DeployRequest{Repo: b.Repo, Branch: b.Branch, ContainingSource: true, Manifest: manifest, resourceSuffix: b.ResourceSuffix})
	if err != nil {
		return nil, err
	}
	created := map[string]bool{}
	for _, a := range actions {
		if a.Kind == BlueprintResourceService && a.Operation == BlueprintPlanCreate {
			created[a.Name] = true
		}
	}
	out := []ManualEnvVar{}
	for _, svc := range parsed.services {
		if !created[svc.req.Name] {
			continue
		}
		for _, key := range svc.promptKeys {
			out = append(out, ManualEnvVar{ServiceType: manualEnvVarServiceType(svc.req.Type), ServiceName: svc.req.Name, Key: key})
		}
	}
	return out, nil
}

// manualEnvVarServiceType is Render's manualEnvVarTargetType for a service type.
func manualEnvVarServiceType(serviceType string) string {
	switch effectiveType(serviceType) {
	case appv1alpha1.TypePrivateService:
		return "pserv"
	case appv1alpha1.TypeBackgroundWorker:
		return "worker"
	case appv1alpha1.TypeCronJob:
		return "cron"
	default:
		return "web"
	}
}

func (s *Service) syncWithPlan(b store.Blueprint, p store.PlannedBlueprintSync, actions []BlueprintPlanAction, required []ManualEnvVar) *SyncWithPlan {
	started := p.StartedAt
	var rawPlan any
	if actions != nil {
		rawPlan = map[string]any{"actions": actions}
	}
	return &SyncWithPlan{
		ID: ids.BlueprintSyncPublicID(p.ID), Commit: BlueprintCommitRef{ID: p.CommitID}, CommitID: p.CommitID, RawPlanJSON: rawPlanJSON(rawPlan), State: p.State,
		StartedAt: &started, CompletedAt: p.CompletedAt,
		DashboardURL: s.Metadata.DashboardURL("blueprints", ids.BlueprintPublicID(b.ID)),
		RawPlan:      rawPlan, RequiredManualEnvVars: required, Actions: actions,
	}
}

// PlannedSync is the Blueprint's newest planning attempt while it still waits
// for approval — the plan and the manual env vars approval must supply — or
// nil when there is nothing to approve (w2/m256 t012: the dashboard shows it
// with its Approve control).
func (s *Service) PlannedSync(ctx context.Context, bpID, ownerID string) (*SyncWithPlan, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanView, bpID, ownerID)
	if err != nil {
		return nil, err
	}
	plans, err := s.blueprintPlans()
	if err != nil {
		return nil, err
	}
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, s.resolveTenantID(ctx))
	if err != nil {
		return nil, store.MapError(err)
	}
	p, err := plans.LatestBlueprintPlan(ctx, b.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, store.MapError(err)
	}
	if p.State != store.BlueprintSyncStateCreated {
		return nil, nil
	}
	var actions []BlueprintPlanAction
	if len(p.PlanActions) > 0 {
		if err := json.Unmarshal(p.PlanActions, &actions); err != nil {
			return nil, fmt.Errorf("read the planned sync's actions: %w", err)
		}
	}
	required := []ManualEnvVar{}
	if len(p.RequiredManualEnvVars) > 0 {
		if err := json.Unmarshal(p.RequiredManualEnvVars, &required); err != nil {
			return nil, fmt.Errorf("read the planned sync's manual env vars: %w", err)
		}
	}
	return s.syncWithPlan(b, p, actions, required), nil
}

// ApproveBlueprint applies the Blueprint's newest planned sync, with the
// sync:false values it requires. A newest sync that failed planning, or that
// was already applied, cannot be approved.
func (s *Service) ApproveBlueprint(ctx context.Context, bpID, ownerID string, supplied []ManualEnvVar) (ApprovedSync, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
	if err != nil {
		return ApprovedSync{}, err
	}
	plans, err := s.blueprintPlans()
	if err != nil {
		return ApprovedSync{}, err
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return ApprovedSync{}, store.MapError(err)
	}
	if taken, err := plans.BlueprintNameTaken(ctx, tenantID, b.Name, b.ID); err != nil {
		return ApprovedSync{}, err
	} else if taken {
		return ApprovedSync{}, errBlueprintNameTakenAtApproval(b.Name)
	}
	latest, err := plans.LatestBlueprintPlan(ctx, b.ID)
	if errors.Is(err, store.ErrNotFound) {
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NOTHING_TO_APPROVE", "this Blueprint has no planned sync; plan one first", nil)
	}
	if err != nil {
		return ApprovedSync{}, err
	}
	syncID := ids.BlueprintSyncPublicID(latest.ID)
	switch {
	case latest.State == store.BlueprintSyncStateCreated:
	case latest.State == store.BlueprintSyncStateError && len(latest.PlanActions) > 0:
		// A plan that was approved (or superseded) and then failed: there is
		// nothing left to approve, and the next plan is the way forward.
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NOTHING_TO_APPROVE",
			fmt.Sprintf("the newest sync %s is no longer awaiting approval (%s); plan a new sync first", syncID, planErrorSummary(latest)), map[string]any{"syncId": syncID})
	case latest.State == store.BlueprintSyncStateError:
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_SYNC_PLANNING_FAILED",
			fmt.Sprintf("the newest sync %s failed planning and cannot be approved; fix the Blueprint file and re-plan", syncID), map[string]any{"syncId": syncID})
	default:
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NOTHING_TO_APPROVE",
			fmt.Sprintf("the newest sync %s is %s, not awaiting approval; plan a new sync first", syncID, latest.State), map[string]any{"syncId": syncID})
	}
	values, err := approvalEnvVarValues(latest.RequiredManualEnvVars, supplied)
	if err != nil {
		return ApprovedSync{}, err
	}
	if _, err := s.runSync(ctx, b, "", "", &ReviewedBlueprintSource{
		Repo: b.Repo, Path: latest.Path, CommitID: latest.CommitID, PlannedRunID: latest.ID, EnvVarValues: values,
	}, nil); err != nil {
		return ApprovedSync{}, err
	}
	applied, err := plans.BlueprintPlannedSync(ctx, b.ID, latest.ID)
	if err != nil {
		return ApprovedSync{}, err
	}
	started := applied.StartedAt
	return ApprovedSync{
		ID: ids.BlueprintSyncPublicID(applied.ID), Commit: BlueprintCommitRef{ID: applied.CommitID}, CommitID: applied.CommitID, State: applied.State,
		DashboardURL: s.Metadata.DashboardURL("blueprints", ids.BlueprintPublicID(b.ID)), StartedAt: &started, CompletedAt: applied.CompletedAt,
	}, nil
}

// errBlueprintNameTakenAtApproval refuses taking a Blueprint out of `created`
// under a name another approved Blueprint in the workspace has since taken.
func errBlueprintNameTakenAtApproval(name string) error {
	return core.NewConflictError("BLUEPRINT_NAME_TAKEN",
		fmt.Sprintf("another Blueprint named %q was approved in this workspace; rename this one before approving", name), map[string]any{"name": name})
}

// planErrorSummary is a failed planned sync's recorded reason, or "failed".
func planErrorSummary(p store.PlannedBlueprintSync) string {
	if p.ErrorMessage != nil && *p.ErrorMessage != "" {
		return *p.ErrorMessage
	}
	return "failed"
}

// approvalEnvVarValues checks the supplied manual env vars against the plan's
// required ones — every required value present, nothing unknown — and keys
// them per service for the apply.
func approvalEnvVarValues(requiredJSON json.RawMessage, supplied []ManualEnvVar) (map[string]string, error) {
	var required []ManualEnvVar
	if len(requiredJSON) > 0 {
		if err := json.Unmarshal(requiredJSON, &required); err != nil {
			return nil, err
		}
	}
	id := func(v ManualEnvVar) string { return v.ServiceType + "\x00" + v.ServiceName + "\x00" + v.Key }
	want := map[string]ManualEnvVar{}
	for _, r := range required {
		want[id(r)] = r
	}
	values := map[string]string{}
	seen := map[string]bool{}
	for _, v := range supplied {
		k := id(v)
		if _, ok := want[k]; !ok {
			return nil, core.NewBadRequestError("BLUEPRINT_MANUAL_ENV_VAR_UNKNOWN",
				fmt.Sprintf("manualEnvVars names %s %q key %q, which this sync does not require", v.ServiceType, v.ServiceName, v.Key), nil)
		}
		if seen[k] {
			return nil, fmt.Errorf("%w: manualEnvVars repeats %s %q key %q", core.ErrBadRequest, v.ServiceType, v.ServiceName, v.Key)
		}
		seen[k] = true
		if v.Value == nil {
			continue
		}
		values[BlueprintEnvVarValueKey(v.ServiceName, v.Key)] = *v.Value
	}
	var missing []string
	for _, r := range want {
		if _, ok := values[BlueprintEnvVarValueKey(r.ServiceName, r.Key)]; !ok {
			missing = append(missing, fmt.Sprintf("%s %q key %q", r.ServiceType, r.ServiceName, r.Key))
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, core.NewBadRequestError("BLUEPRINT_MANUAL_ENV_VARS_MISSING",
			"approving this sync requires values for: "+strings.Join(missing, ", "), map[string]any{"missing": missing})
	}
	return values, nil
}

var blueprintNoticeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintNotice",
	Fields: graphql.Fields{
		"severity": gqlutil.StrField(func(n BlueprintNotice) any { return n.Severity }),
		"message":  gqlutil.StrField(func(n BlueprintNotice) any { return n.Message }),
		"path":     gqlutil.StrField(func(n BlueprintNotice) any { return n.Path }),
		"line":     gqlutil.IntField(func(n BlueprintNotice) any { return n.Line }),
		"column":   gqlutil.IntField(func(n BlueprintNotice) any { return n.Column }),
	},
})

var manualEnvVarGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintManualEnvVar",
	Fields: graphql.Fields{
		"serviceType": gqlutil.StrField(func(v ManualEnvVar) any { return v.ServiceType }),
		"serviceName": gqlutil.StrField(func(v ManualEnvVar) any { return v.ServiceName }),
		"key":         gqlutil.StrField(func(v ManualEnvVar) any { return v.Key }),
	},
})

var manualEnvVarInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BlueprintManualEnvVarInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"serviceType": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"serviceName": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"key":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value":       &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var syncWithPlanGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPlannedSync",
	Fields: graphql.Fields{
		"id":           gqlutil.StrField(func(s SyncWithPlan) any { return s.ID }),
		"commitId":     gqlutil.StrField(func(s SyncWithPlan) any { return s.CommitID }),
		"state":        gqlutil.StrField(func(s SyncWithPlan) any { return s.State }),
		"startedAt":    gqlutil.TimeField(func(s SyncWithPlan) any { return s.StartedAt }),
		"completedAt":  gqlutil.TimeField(func(s SyncWithPlan) any { return s.CompletedAt }),
		"dashboardUrl": gqlutil.StrField(func(s SyncWithPlan) any { return s.DashboardURL }),
		// rawPlan is Render's unstable plan output, JSON-encoded.
		"rawPlan": gqlutil.StrField(func(s SyncWithPlan) any { return s.RawPlanJSON }),
		"requiredManualEnvVars": gqlutil.Typed(graphql.NewList(manualEnvVarGQLType),
			func(s SyncWithPlan) any { return s.RequiredManualEnvVars }),
		"actions": gqlutil.Typed(graphql.NewList(blueprintPlanActionGQLType), func(s SyncWithPlan) any { return s.Actions }),
	},
})

var blueprintPlanResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPlanResult",
	Fields: graphql.Fields{
		"blueprint":       gqlutil.Typed(blueprintGQLType, func(r BlueprintPlanResult) any { return planResultBlueprint(r) }),
		"planningOutcome": gqlutil.StrField(func(r BlueprintPlanResult) any { return r.PlanningOutcome }),
		"notices":         gqlutil.Typed(graphql.NewList(blueprintNoticeGQLType), func(r BlueprintPlanResult) any { return r.Notices }),
		"sync":            gqlutil.Typed(syncWithPlanGQLType, func(r BlueprintPlanResult) any { return plannedSyncValue(r.Sync) }),
	},
})

var approvedSyncGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintApprovedSync",
	Fields: graphql.Fields{
		"id":           gqlutil.StrField(func(a ApprovedSync) any { return a.ID }),
		"commitId":     gqlutil.StrField(func(a ApprovedSync) any { return a.CommitID }),
		"state":        gqlutil.StrField(func(a ApprovedSync) any { return a.State }),
		"dashboardUrl": gqlutil.StrField(func(a ApprovedSync) any { return a.DashboardURL }),
		"startedAt":    gqlutil.TimeField(func(a ApprovedSync) any { return a.StartedAt }),
		"completedAt":  gqlutil.TimeField(func(a ApprovedSync) any { return a.CompletedAt }),
	},
})

// plannedSyncValue is p as the value its GraphQL fields read, nil for none.
func plannedSyncValue(p *SyncWithPlan) any {
	if p == nil {
		return nil
	}
	return *p
}

func rawPlanJSON(plan any) *string {
	if plan == nil {
		return nil
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil
	}
	out := string(raw)
	return &out
}

// planResultBlueprint is the create result's Blueprint as a BlueprintView, nil
// on a plan of an existing Blueprint.
func planResultBlueprint(r BlueprintPlanResult) any {
	if r.Blueprint == nil {
		return nil
	}
	b := r.Blueprint
	return BlueprintView{ID: b.ID, Name: b.Name, OwnerID: b.OwnerID, Repo: b.Repo, Branch: b.Branch, Path: b.Path,
		AutoSync: b.AutoSync, Status: b.Status, LastSync: b.LastSync}
}

// onboardingGraphQLMutations are createPlannedBlueprint, planBlueprintSync and
// approveBlueprint — the REST onboarding verbs as bex GraphQL extensions.
func (s *Service) onboardingGraphQLMutations() graphql.Fields {
	return graphql.Fields{
		"createPlannedBlueprint": &graphql.Field{
			Type: blueprintPlanResultGQLType,
			Args: graphql.FieldConfigArgument{
				"name":              gqlutil.ReqArg(graphql.String),
				"repo":              gqlutil.ReqArg(graphql.String),
				"branch":            gqlutil.Arg(graphql.String),
				"path":              gqlutil.Arg(graphql.String),
				"ownerId":           gqlutil.Arg(graphql.String),
				"autoSync":          gqlutil.Arg(graphql.Boolean),
				"existingResources": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := RenderCreateBlueprintRequest{Name: p.Args["name"].(string), OwnerID: gqlutil.Str(p.Args, "ownerId"),
					Source: RenderBlueprintSource{Repo: p.Args["repo"].(string)}}
				if v, ok := p.Args["branch"].(string); ok {
					req.Source.Branch = &v
				}
				if v, ok := p.Args["path"].(string); ok {
					req.Source.Path = &v
				}
				if v, ok := p.Args["autoSync"].(bool); ok {
					req.AutoSync = &v
				}
				if v, ok := p.Args["existingResources"].(string); ok {
					req.ExistingResources = &v
				}
				return s.CreatePlannedBlueprint(p.Context, req)
			},
		},
		"planBlueprintSync": &graphql.Field{
			Type: blueprintPlanResultGQLType,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String), "ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PlanBlueprint(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"))
			},
		},
		"approveBlueprint": &graphql.Field{
			Type: approvedSyncGQLType,
			Args: graphql.FieldConfigArgument{
				"id":            gqlutil.ReqArg(graphql.String),
				"ownerId":       gqlutil.Arg(graphql.String),
				"manualEnvVars": gqlutil.Arg(graphql.NewList(manualEnvVarInputGQLType)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var supplied []ManualEnvVar
				if list, ok := p.Args["manualEnvVars"].([]any); ok {
					for _, item := range list {
						m, _ := item.(map[string]any)
						v := ManualEnvVar{ServiceType: gqlutil.Str(m, "serviceType"), ServiceName: gqlutil.Str(m, "serviceName"), Key: gqlutil.Str(m, "key")}
						if value, ok := m["value"].(string); ok {
							v.Value = &value
						}
						supplied = append(supplied, v)
					}
				}
				return s.ApproveBlueprint(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"), supplied)
			},
		},
	}
}

// Render's Blueprint MCP tools (w5/m283; render-oss/render-mcp-server #205,
// #218, #221, #222, #224, #230): create_blueprint plans the first sync without
// applying it, plan_blueprint re-plans, approve_blueprint applies, and
// get_blueprint/update_blueprint read and edit settings. Upstream's tools wrap
// Render's REST onboarding routes, so bex's run the same verbs REST does.

type createBlueprintArgs struct {
	RepoURL           string  `json:"repoUrl" jsonschema:"Repository URL starting with https:// or ssh://, without a query string or fragment. SSH URLs are converted to HTTPS. Specify the branch separately with branch."`
	Branch            *string `json:"branch,omitempty" jsonschema:"Branch containing the Blueprint file (default main)."`
	Path              *string `json:"path,omitempty" jsonschema:"Blueprint file path relative to the repository root (default render.yaml)."`
	Name              *string `json:"name,omitempty" jsonschema:"Blueprint name (default: the repository name without the .git suffix)."`
	AutoSync          *bool   `json:"autoSync,omitempty" jsonschema:"Automatically apply later pushes to the branch after the first sync is approved (default true)."`
	ExistingResources *string `json:"existingResources,omitempty" jsonschema:"create_new (default) creates a separate set of resources whose names carry a Blueprint-specific suffix, leaving existing resources unchanged; adopt brings matching existing resources under this Blueprint and updates them to match the file.,enum=adopt,enum=create_new"`
}

type blueprintIDArgs struct {
	BlueprintID string `json:"blueprintId" jsonschema:"The Blueprint ID (exs-…, or the legacy blp-…). Use the Blueprint ID rather than a sync ID."`
}

type approveBlueprintArgs struct {
	BlueprintID   string         `json:"blueprintId" jsonschema:"The Blueprint ID (exs-…, or the legacy blp-…). Use the Blueprint ID, not a sync ID."`
	ManualEnvVars []ManualEnvVar `json:"manualEnvVars,omitempty" jsonschema:"Values for the environment variables listed in sync.requiredManualEnvVars: {serviceType, serviceName, key, value}."`
}

type updateBlueprintArgs struct {
	BlueprintID string  `json:"blueprintId" jsonschema:"The Blueprint ID (exs-…, or the legacy blp-…). Use the Blueprint ID rather than a sync ID."`
	Name        *string `json:"name,omitempty" jsonschema:"The Blueprint's new name. Omit to keep the current name."`
	Branch      *string `json:"branch,omitempty" jsonschema:"The Git branch the Blueprint file is read from. A different branch also creates a plan. Omit to keep the current branch."`
	Path        *string `json:"path,omitempty" jsonschema:"The Blueprint file's new path relative to the repository root. A different path also creates a plan. Omit to keep the current path."`
	AutoSync    *bool   `json:"autoSync,omitempty" jsonschema:"true enables automatic syncing on later Git pushes, false disables it. Omit to keep the current value."`
}

// updateBlueprintResult is upstream's update_blueprint response: the
// Blueprint, plus the plan a branch or path change creates. planningOutcome
// and error both null means planning was skipped.
type updateBlueprintResult struct {
	Blueprint       renderBlueprint        `json:"blueprint"`
	PlanningOutcome *string                `json:"planningOutcome"`
	Sync            *SyncWithPlan          `json:"sync"`
	Notices         []BlueprintNotice      `json:"notices"`
	Error           *updateBlueprintFailed `json:"error"`
}

type updateBlueprintFailed struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// toBlueprintDetail is Render's by-id Blueprint: the view plus its resources,
// an empty array rather than null when it manages none.
func toBlueprintDetail(v BlueprintView) blueprintDetail {
	detail := blueprintDetail{BlueprintView: v, Resources: v.Resources}
	if detail.Resources == nil {
		detail.Resources = []BlueprintResource{}
	}
	return detail
}

func toRenderBlueprint(v BlueprintView) renderBlueprint {
	return renderBlueprint{ID: v.ID, Name: v.Name, OwnerID: v.OwnerID, Status: v.Status,
		AutoSync: v.AutoSync, Repo: v.Repo, Branch: v.Branch, Path: v.Path, LastSync: v.LastSync}
}

// parseBlueprintRepoURL is upstream's repoUrl rule: https:// or ssh:// only,
// no query or fragment, a repository name present; an ssh:// URL becomes its
// https:// form. It returns the URL and the default Blueprint name.
func parseBlueprintRepoURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "ssh") {
		return "", "", fmt.Errorf("%w: repoUrl must be an HTTPS or SSH repository URL starting with https:// or ssh://", core.ErrBadRequest)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("%w: repoUrl must be a repository URL without a query or fragment; specify the branch with branch", core.ErrBadRequest)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	name := repoName(u.Path)
	if name == "" || name == "." || name == "/" {
		return "", "", fmt.Errorf("%w: repoUrl must include a repository name", core.ErrBadRequest)
	}
	if u.Scheme == "ssh" {
		u.Scheme, u.Host, u.User = "https", u.Hostname(), nil
	}
	return u.String(), name, nil
}

// blankSetting refuses a supplied but blank string setting, as upstream does.
func blankSetting(field string, v *string, omit string) error {
	if v != nil && strings.TrimSpace(*v) == "" {
		return fmt.Errorf("%w: %s cannot be empty; omit it to %s", core.ErrBadRequest, field, omit)
	}
	return nil
}

func (s *Service) createBlueprintTool(ctx context.Context, in createBlueprintArgs) (BlueprintPlanResult, error) {
	repo, name, err := parseBlueprintRepoURL(in.RepoURL)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	for _, c := range []struct {
		field string
		v     *string
		omit  string
	}{{"name", in.Name, "use the repository name"}, {"branch", in.Branch, "use the default branch"}, {"path", in.Path, "use render.yaml"}} {
		if err := blankSetting(c.field, c.v, c.omit); err != nil {
			return BlueprintPlanResult{}, err
		}
	}
	if in.Name != nil {
		name = *in.Name
	}
	mode := store.BlueprintExistingCreateNew
	if in.ExistingResources != nil {
		mode = *in.ExistingResources
	}
	return s.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: name, OwnerID: core.NamedWorkspace(ctx),
		Source: RenderBlueprintSource{Repo: repo, Branch: in.Branch, Path: in.Path}, AutoSync: in.AutoSync, ExistingResources: &mode})
}

func (s *Service) updateBlueprintTool(ctx context.Context, in updateBlueprintArgs) (updateBlueprintResult, error) {
	for _, c := range []struct {
		field string
		v     *string
	}{{"name", in.Name}, {"branch", in.Branch}, {"path", in.Path}} {
		if err := blankSetting(c.field, c.v, "keep its current value"); err != nil {
			return updateBlueprintResult{}, err
		}
	}
	if in.Name == nil && in.Branch == nil && in.Path == nil && in.AutoSync == nil {
		return updateBlueprintResult{}, fmt.Errorf("%w: provide at least one of name, branch, path, or autoSync", core.ErrBadRequest)
	}
	if in.Path != nil {
		clean, err := approvedBlueprintPath(*in.Path)
		if err != nil {
			return updateBlueprintResult{}, err
		}
		in.Path = &clean
	}
	// The edit right is checked even when nothing changes: a no-op answer must
	// not tell a read-only member the update succeeded.
	owner := core.NamedWorkspace(ctx)
	scoped, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, in.BlueprintID, owner)
	if err != nil {
		return updateBlueprintResult{}, err
	}
	stored, err := s.Blueprints.GetBlueprint(scoped, bpID, s.resolveTenantID(scoped))
	if err != nil {
		return updateBlueprintResult{}, store.MapError(err)
	}
	current := toBlueprintView(stored)
	branchChanged := in.Branch != nil && strings.TrimSpace(*in.Branch) != current.Branch
	pathChanged := in.Path != nil && *in.Path != current.Path
	changed := branchChanged || pathChanged ||
		(in.Name != nil && *in.Name != current.Name) || (in.AutoSync != nil && *in.AutoSync != current.AutoSync)
	out := updateBlueprintResult{Blueprint: toRenderBlueprint(current), Notices: []BlueprintNotice{}}
	if !changed {
		return out, nil
	}
	updated, err := s.UpdateBlueprint(ctx, in.BlueprintID, owner, UpdateBlueprintRequest{
		Name: in.Name, AutoSync: in.AutoSync, Path: in.Path, Branch: in.Branch})
	if err != nil {
		return updateBlueprintResult{}, err
	}
	out.Blueprint = toRenderBlueprint(updated)
	if branchChanged || pathChanged {
		plan, err := s.PlanBlueprint(ctx, in.BlueprintID, owner)
		if err != nil {
			out.Error = &updateBlueprintFailed{Stage: "planning", Message: fmt.Sprintf(
				"The Blueprint was updated, but no planning result is available: %v. Review the error before calling plan_blueprint again. Do not approve changes based on this response.", core.MCPError(err))}
			return out, nil
		}
		out.PlanningOutcome, out.Sync, out.Notices = &plan.PlanningOutcome, plan.Sync, plan.Notices
	}
	return out, nil
}

// planningGuidance is upstream's next-step guidance for a planning outcome.
const planningGuidance = "Read planningOutcome to determine the next step: failed — inspect the errors in notices, fix and push the Blueprint file, then call plan_blueprint again (never approve a failed plan); no_changes — nothing to apply, do not call approve_blueprint; planned — sync holds the proposed changes: summarize them for the user (or share sync.dashboardUrl) and call approve_blueprint only after the user authorizes applying them."

// registerOnboardingMCP registers Render's five Blueprint tools.
func (s *Service) registerOnboardingMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_blueprint",
		Description: "Create a Blueprint from a Blueprint YAML file in a public repository or one reachable through the workspace's GitHub connection, and plan its first sync. Resource changes are applied only after approval with approve_blueprint; after the first approval, later pushes deploy automatically when autoSync is enabled. All Blueprint changes are read from the remote repository: push local changes first. Returns {blueprint, planningOutcome, sync, notices}. " + planningGuidance,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createBlueprintArgs) (*mcp.CallToolResult, BlueprintPlanResult, error) {
		res, err := s.createBlueprintTool(ctx, in)
		return nil, res, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_blueprint",
		Description: "Retrieve a Blueprint's settings and status, with the IDs of the resources it manages (the REST GET /v1/blueprints/{blueprintId} object). The status stays \"created\" until the first sync is approved. Do not use the status alone to judge service health or whether infrastructure matches the file; use plan_blueprint for that. This tool creates no plan and applies nothing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in blueprintIDArgs) (*mcp.CallToolResult, blueprintDetail, error) {
		v, err := s.GetBlueprintByID(ctx, in.BlueprintID, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, blueprintDetail{}, err
		}
		return nil, toBlueprintDetail(v), nil
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "plan_blueprint",
		Description: "Preview infrastructure changes for an existing Blueprint using its configured branch and Blueprint file path; push local changes first. Planning applies nothing: review the plan with the user before calling approve_blueprint, even when auto-sync is enabled. Returns {planningOutcome, sync, notices}. " + planningGuidance,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in blueprintIDArgs) (*mcp.CallToolResult, BlueprintPlanResult, error) {
		res, err := s.PlanBlueprint(ctx, in.BlueprintID, core.NamedWorkspace(ctx))
		return nil, res, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "approve_blueprint",
		Description: "Approve and apply the Blueprint's newest planned sync after confirming that planning succeeded; approval can update existing resources. Call it only after the user authorizes applying the reviewed plan. Supply a user-provided value in manualEnvVars for every entry in sync.requiredManualEnvVars; never invent a value (they appear as plaintext in the agent context — the user can instead approve through POST /v1/blueprints/{blueprintId}/approve or the dashboard). Refused when the newest sync failed planning or is not awaiting approval. Returns the approved sync, before its deployments finish.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in approveBlueprintArgs) (*mcp.CallToolResult, ApprovedSync, error) {
		res, err := s.ApproveBlueprint(ctx, in.BlueprintID, core.NamedWorkspace(ctx), in.ManualEnvVars)
		return nil, res, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_blueprint",
		Description: "Update a Blueprint's name, Git branch, Blueprint file path, or auto-sync setting; omitted settings keep their values, and blank values are refused. If every supplied value already matches, the Blueprint is returned unchanged and no plan is created. Changing the branch or path also creates a plan (from the remote repository: push first); changing only the name or auto-sync does not. Returns {blueprint, planningOutcome, sync, notices, error}: planningOutcome and error both null means planning was skipped; a non-null error means the Blueprint was updated but no plan is available. " + planningGuidance,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateBlueprintArgs) (*mcp.CallToolResult, updateBlueprintResult, error) {
		res, err := s.updateBlueprintTool(ctx, in)
		return nil, res, err
	})
}
