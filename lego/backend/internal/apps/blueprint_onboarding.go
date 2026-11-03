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
	"slices"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Blueprint onboarding (w2/m256; Render CLI v2.35 `render ea
// blueprints create | plan | approve`): creating a Blueprint plans its first
// sync without applying it, a sync can be re-planned at the branch head, and
// approving applies the newest planned sync — with the values of the
// sync:false env vars it creates. A Blueprint stays `created`, and auto-sync
// does not run, until a sync is approved.

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
	result, err := s.planBlueprintSync(ctx, plans, b, contents, commitSHA, note)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	view := toBlueprintView(b)
	result.Blueprint = &renderBlueprint{ID: view.ID, Name: view.Name, OwnerID: view.OwnerID, Status: view.Status,
		AutoSync: view.AutoSync, Repo: view.Repo, Branch: view.Branch, Path: view.Path, LastSync: view.LastSync}
	return result, nil
}

// DefaultBlueprintBranch is the branch a create without source.branch reads.
// Render reads the repository's default branch; bex's GitHub reader resolves
// branches by name, so it uses the conventional default.
const DefaultBlueprintBranch = "main"

// PlanBlueprint plans a sync of an existing Blueprint at its branch head.
func (s *Service) PlanBlueprint(ctx context.Context, bpID, ownerID string) (BlueprintPlanResult, error) {
	ctx, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
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
	return s.planBlueprintSync(ctx, plans, b, contents, commitSHA, "")
}

// planBlueprintSync validates manifest against the workspace and persists the
// outcome: a planned sync awaiting approval, nothing when no resource would
// change, or an error sync carrying the validation notices.
// note is recorded on the sync row it writes ("" for none).
func (s *Service) planBlueprintSync(ctx context.Context, plans blueprintPlanStore, b store.Blueprint, manifest, commitSHA, note string) (BlueprintPlanResult, error) {
	validation, err := s.blueprintValidationFor(ctx, b.Repo, b.Branch, manifest, b.ID)
	if err != nil {
		return BlueprintPlanResult{}, err
	}
	if !validation.Valid {
		notices := make([]BlueprintNotice, 0, len(validation.Errors))
		for _, e := range validation.Errors {
			notices = append(notices, BlueprintNotice{Severity: "error", Message: e.Error, Path: e.Path, Line: e.Line, Column: e.Column})
		}
		return s.recordFailedPlan(ctx, plans, b, commitSHA, note, notices)
	}
	// Only a plan against the workspace's current resources can say what
	// would change; a structural plan (state unreadable) cannot.
	if validation.Plan == nil || validation.Plan.Mode != "current_state" {
		return BlueprintPlanResult{}, core.NewUnavailableError("BLUEPRINT_PLAN_UNAVAILABLE",
			"the workspace's current resources could not be read, so the sync cannot be planned; try again", nil)
	}
	actions := validation.Plan.Actions
	if !planChangesAnything(actions) {
		return BlueprintPlanResult{PlanningOutcome: PlanningNoChanges, Notices: []BlueprintNotice{}}, nil
	}
	required, err := requiredManualEnvVars(b, manifest, actions)
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
	return BlueprintPlanResult{PlanningOutcome: PlanningPlanned, Notices: []BlueprintNotice{}, Sync: s.syncWithPlan(b, planned, actions, required)}, nil
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
		ID: p.ID, Commit: BlueprintCommitRef{ID: p.CommitID}, CommitID: p.CommitID, RawPlanJSON: rawPlanJSON(rawPlan), State: p.State,
		StartedAt: &started, CompletedAt: p.CompletedAt,
		DashboardURL: s.Metadata.DashboardURL("blueprints", b.ID),
		RawPlan:      rawPlan, RequiredManualEnvVars: required, Actions: actions,
	}
}

// PlannedSync is the Blueprint's newest planning attempt while it still waits
// for approval — the plan and the manual env vars approval must supply — or
// nil when there is nothing to approve (w2/m256 t012: the dashboard shows it
// with its Approve control).
func (s *Service) PlannedSync(ctx context.Context, bpID, ownerID string) (*SyncWithPlan, error) {
	ctx, err := s.blueprintScope(ctx, core.RelCanView, bpID, ownerID)
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
	ctx, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
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
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NAME_TAKEN",
			fmt.Sprintf("another Blueprint named %q was approved in this workspace; rename this one before approving", b.Name), map[string]any{"name": b.Name})
	}
	latest, err := plans.LatestBlueprintPlan(ctx, b.ID)
	if errors.Is(err, store.ErrNotFound) {
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NOTHING_TO_APPROVE", "this Blueprint has no planned sync; plan one first", nil)
	}
	if err != nil {
		return ApprovedSync{}, err
	}
	switch latest.State {
	case store.BlueprintSyncStateCreated:
	case store.BlueprintSyncStateError:
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_SYNC_PLANNING_FAILED",
			fmt.Sprintf("the newest sync %s failed planning and cannot be approved; fix the Blueprint file and re-plan", latest.ID), map[string]any{"syncId": latest.ID})
	default:
		return ApprovedSync{}, core.NewConflictError("BLUEPRINT_NOTHING_TO_APPROVE",
			fmt.Sprintf("the newest sync %s is %s, not awaiting approval; plan a new sync first", latest.ID, latest.State), map[string]any{"syncId": latest.ID})
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
		ID: applied.ID, Commit: BlueprintCommitRef{ID: applied.CommitID}, CommitID: applied.CommitID, State: applied.State,
		DashboardURL: s.Metadata.DashboardURL("blueprints", b.ID), StartedAt: &started, CompletedAt: applied.CompletedAt,
	}, nil
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

type createPlannedBlueprintArgs struct {
	Name              string  `json:"name" jsonschema:"Blueprint name, unique among approved Blueprints in the workspace"`
	Repo              string  `json:"repo" jsonschema:"Git repository URL"`
	Branch            *string `json:"branch,omitempty" jsonschema:"branch to read (default main)"`
	Path              *string `json:"path,omitempty" jsonschema:"Blueprint file path (default render.yaml)"`
	AutoSync          *bool   `json:"autoSync,omitempty" jsonschema:"sync on every push once a sync is approved (default true)"`
	ExistingResources *string `json:"existingResources,omitempty" jsonschema:"adopt (default) brings matching existing resources under the Blueprint; create_new manages a separate copy whose names carry a random suffix, leaving existing resources unchanged"`
}

type planBlueprintArgs struct {
	ID string `json:"id" jsonschema:"blueprint id (blp-…)"`
}

type approveBlueprintArgs struct {
	ID            string         `json:"id" jsonschema:"blueprint id (blp-…)"`
	ManualEnvVars []ManualEnvVar `json:"manualEnvVars,omitempty" jsonschema:"values for the planned sync's requiredManualEnvVars"`
}

// registerOnboardingMCP adds create_planned_blueprint, plan_blueprint_sync and
// approve_blueprint — the REST onboarding verbs as bex MCP extensions.
func (s *Service) registerOnboardingMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_planned_blueprint",
		Description: "Create a Blueprint and plan its first sync without applying it (Render's Blueprint onboarding). Returns {blueprint, planningOutcome: planned|no_changes|failed, notices, sync}; approve the planned sync with approve_blueprint. bex extension (w2/m256).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createPlannedBlueprintArgs) (*mcp.CallToolResult, BlueprintPlanResult, error) {
		res, err := s.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: in.Name, OwnerID: core.NamedWorkspace(ctx),
			Source: RenderBlueprintSource{Repo: in.Repo, Branch: in.Branch, Path: in.Path}, AutoSync: in.AutoSync, ExistingResources: in.ExistingResources})
		return nil, res, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "plan_blueprint_sync",
		Description: "Plan a sync of a Blueprint at its branch head without applying it. Returns {planningOutcome, notices, sync}. bex extension (w2/m256).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in planBlueprintArgs) (*mcp.CallToolResult, BlueprintPlanResult, error) {
		res, err := s.PlanBlueprint(ctx, in.ID, core.NamedWorkspace(ctx))
		return nil, res, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "approve_blueprint",
		Description: "Apply a Blueprint's newest planned sync, supplying values for its requiredManualEnvVars. Refused when the newest sync failed planning or is not awaiting approval. bex extension (w2/m256).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in approveBlueprintArgs) (*mcp.CallToolResult, ApprovedSync, error) {
		res, err := s.ApproveBlueprint(ctx, in.ID, core.NamedWorkspace(ctx), in.ManualEnvVars)
		return nil, res, err
	})
}
