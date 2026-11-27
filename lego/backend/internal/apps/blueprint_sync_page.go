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
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A Blueprint sync's own page (w3/m678, docs/render-artifacts/blueprint-pages.md
// § A sync's page): every sync records the commit message and the actions it
// planned, the dashboard reads one sync with its actions, their states and its
// errors, and every sync link bex-api emits opens that page.

// blueprintCommitMessage is sha's message on repo's host, "" when the host
// cannot say: provenance for a sync's card, never a reason to fail the sync,
// as a deploy's commit message is (resolveDeployCommit).
func (s *Service) blueprintCommitMessage(ctx context.Context, tenantID, repo, sha string) string {
	if sha == "" {
		return ""
	}
	return s.resolveDeployCommit(ctx, tenantID, repo, sha).Message
}

// blueprintSyncPlanRecorder is the store seam an admitted sync records its
// planned actions through; *store.PGStore satisfies it.
type blueprintSyncPlanRecorder interface {
	RecordBlueprintSyncPlan(ctx context.Context, runID string, actions json.RawMessage) error
}

// recordBlueprintSyncPlan stores the actions an admitted run planned, before
// it applies them, so its page lists them while it runs and after. A planned
// sync keeps the actions it was approved with. A failure to record is logged,
// not returned: the apply it describes goes on.
func (s *Service) recordBlueprintSyncPlan(ctx context.Context, runID string, actions []BlueprintPlanAction) {
	recorder, ok := s.Blueprints.(blueprintSyncPlanRecorder)
	if !ok || runID == "" {
		return
	}
	if actions == nil {
		actions = []BlueprintPlanAction{}
	}
	raw, err := json.Marshal(actions)
	if err == nil {
		err = recorder.RecordBlueprintSyncPlan(ctx, runID, raw)
	}
	if err != nil {
		log.Printf("blueprint sync %s: recording its planned actions failed: %v", runID, err)
	}
}

// blueprintSyncDashboardURL is the page of sync syncID of Blueprint
// blueprintID, either id in either spelling.
func (s *Service) blueprintSyncDashboardURL(blueprintID, syncID string) string {
	return s.Metadata.BlueprintSyncDashboardURL(ids.BlueprintPublicID(blueprintID), ids.BlueprintSyncPublicID(syncID))
}

// BlueprintSyncActionState is one planned action's state, Render's
// actionStates entry. State is "" when bex cannot say: it records a failed
// sync's error, not which of its actions met it.
type BlueprintSyncActionState struct {
	State string
}

// BlueprintSyncDetail is one sync as its page draws it, Render's
// iacExecution: the sync, the actions it planned with a state each, its
// errors and, while it waits for approval, the sync:false values approving it
// needs. bex GraphQL only.
type BlueprintSyncDetail struct {
	// The sync's own fields, BlueprintSyncView's, kept flat so the wire-time
	// invariant (gqlutil's TestNoTimeReachesAStringField) resolves each.
	ID            string
	CommitID      string
	CommitMessage string
	State         string
	StartedAt     string
	CompletedAt   *string
	ErrorMessage  *string
	Note          string
	// ActionsRecorded is false for a sync recorded before every sync kept its
	// actions (migration 0353), which the page says instead of claiming the
	// sync changed nothing.
	ActionsRecorded       bool
	Actions               []BlueprintPlanAction
	ActionStates          []BlueprintSyncActionState
	Errors                []BlueprintNotice
	RequiredManualEnvVars []ManualEnvVar
	DashboardURL          string
}

// GetBlueprintSync reads one sync of a Blueprint by id (either spelling),
// authorized as the Blueprint's sync history is. A sync of another Blueprint
// is not found.
func (s *Service) GetBlueprintSync(ctx context.Context, bpID, ownerID, syncID string) (BlueprintSyncDetail, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanView, bpID, ownerID)
	if err != nil {
		return BlueprintSyncDetail{}, err
	}
	plans, err := s.blueprintPlans()
	if err != nil {
		return BlueprintSyncDetail{}, err
	}
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, s.resolveTenantID(ctx))
	if err != nil {
		return BlueprintSyncDetail{}, store.MapError(err)
	}
	syncID = strings.TrimSpace(syncID)
	if syncID == "" {
		return BlueprintSyncDetail{}, fmt.Errorf("%w: syncId is required", core.ErrBadRequest)
	}
	p, err := plans.BlueprintPlannedSync(ctx, b.ID, ids.BlueprintSyncStorageID(syncID))
	if err != nil {
		return BlueprintSyncDetail{}, store.MapError(err)
	}
	return s.blueprintSyncDetail(b, p)
}

func (s *Service) blueprintSyncDetail(b store.Blueprint, p store.PlannedBlueprintSync) (BlueprintSyncDetail, error) {
	actions, required, err := plannedSyncRecord(p)
	if err != nil {
		return BlueprintSyncDetail{}, err
	}
	if actions == nil {
		actions = []BlueprintPlanAction{}
	}
	// Only a sync that waits needs values to be approved.
	if p.State != store.BlueprintSyncStateCreated {
		required = []ManualEnvVar{}
	}
	v := toBlueprintSyncView(p.BlueprintSync)
	d := BlueprintSyncDetail{
		ID: v.ID, CommitID: v.CommitID, CommitMessage: v.CommitMessage, State: v.State,
		StartedAt: v.StartedAt, CompletedAt: v.CompletedAt, ErrorMessage: v.ErrorMessage, Note: v.Note,
		ActionsRecorded:       len(p.PlanActions) > 0,
		Actions:               actions,
		ActionStates:          make([]BlueprintSyncActionState, len(actions)),
		RequiredManualEnvVars: required,
		DashboardURL:          s.blueprintSyncDashboardURL(b.ID, p.ID),
	}
	state := blueprintSyncActionState(p.State)
	for i := range d.ActionStates {
		d.ActionStates[i] = state
	}
	errs, err := blueprintSyncErrors(p)
	if err != nil {
		return BlueprintSyncDetail{}, fmt.Errorf("read sync %s's errors: %w", d.ID, err)
	}
	d.Errors = errs
	return d, nil
}

// blueprintSyncActionState is each planned action's state for a sync in
// state: the sync's own while it waits, runs or has succeeded, unknown once
// it failed.
func blueprintSyncActionState(state string) BlueprintSyncActionState {
	switch state {
	case store.BlueprintSyncStateCreated, store.BlueprintSyncStateRunning, store.BlueprintSyncStateSuccess:
		return BlueprintSyncActionState{State: state}
	default:
		return BlueprintSyncActionState{}
	}
}

// blueprintSyncErrors are a sync's errors as Render's located list: a failed
// plan's notices of severity error, else the one message a failed sync
// recorded.
func blueprintSyncErrors(p store.PlannedBlueprintSync) ([]BlueprintNotice, error) {
	if len(p.ErrorNotices) > 0 {
		var notices []BlueprintNotice
		if err := json.Unmarshal(p.ErrorNotices, &notices); err != nil {
			return nil, err
		}
		notices = slices.DeleteFunc(notices, func(n BlueprintNotice) bool { return n.Severity != "error" })
		if len(notices) > 0 {
			return notices, nil
		}
	}
	if p.ErrorMessage != nil && *p.ErrorMessage != "" {
		return []BlueprintNotice{{Severity: "error", Message: *p.ErrorMessage}}, nil
	}
	return []BlueprintNotice{}, nil
}

// dashboardBlueprintSyncs is blueprintSyncs on bex's GraphQL: the sync
// history, and, on its first page, the plan waiting for approval where its
// start places it. Render's dashboard lists a waiting sync (its sync card has
// an icon for one), where its API, and so bex's REST and MCP, leave it out.
func (s *Service) dashboardBlueprintSyncs(ctx context.Context, bpID, ownerID, cursor string, limit int) ([]BlueprintSyncView, error) {
	return s.listBlueprintSyncs(ctx, bpID, ownerID, cursor, limit, true)
}

// waitingBlueprintPlan is the plan of Blueprint bpID (its storage id, in a
// scoped ctx) still waiting for approval, or nil.
func (s *Service) waitingBlueprintPlan(ctx context.Context, bpID string) (*store.BlueprintSync, error) {
	plans, err := s.blueprintPlans()
	if err != nil {
		return nil, err
	}
	p, err := waitingPlan(ctx, plans, bpID, s.Now())
	if err != nil || p == nil {
		return nil, err
	}
	return &p.BlueprintSync, nil
}

// withWaitingPlan places the waiting plan among a first page of syncs, newest
// first, and keeps the page at limit. A plan older than a full page's oldest
// sync is on no page.
func withWaitingPlan(views []BlueprintSyncView, waiting BlueprintSyncView, limit int) []BlueprintSyncView {
	at := slices.IndexFunc(views, func(v BlueprintSyncView) bool {
		return v.StartedAt < waiting.StartedAt || (v.StartedAt == waiting.StartedAt && v.ID < waiting.ID)
	})
	if at < 0 {
		if len(views) >= limit {
			return views
		}
		at = len(views)
	}
	out := slices.Insert(slices.Clone(views), at, waiting)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

var blueprintSyncActionStateGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintSyncActionState",
	Fields: graphql.Fields{
		"state": gqlutil.StrField(func(a BlueprintSyncActionState) any { return a.State }),
	},
})

// blueprintSyncDetailGQLType is one sync for its dashboard page, Render's
// iacExecution (w3/m678): a BlueprintSync's fields and the page's own.
var blueprintSyncDetailGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintSyncDetail",
	Fields: graphql.Fields{
		"id":            gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.ID }),
		"commitId":      gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.CommitID }),
		"commitMessage": gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.CommitMessage }),
		"state":         gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.State }),
		"startedAt":     gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.StartedAt }),
		"completedAt":   gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.CompletedAt }),
		"errorMessage":  gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.ErrorMessage }),
		"note":          gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.Note }),
		// actionsRecorded is false for a sync recorded before every sync kept
		// its actions.
		"actionsRecorded": gqlutil.BoolField(func(d BlueprintSyncDetail) any { return d.ActionsRecorded }),
		"actions":         gqlutil.Typed(graphql.NewList(blueprintPlanActionGQLType), func(d BlueprintSyncDetail) any { return d.Actions }),
		// actionStates is parallel to actions, as Render's is to its plan's.
		"actionStates": gqlutil.Typed(graphql.NewList(blueprintSyncActionStateGQLType), func(d BlueprintSyncDetail) any { return d.ActionStates }),
		"errors":       gqlutil.Typed(graphql.NewList(blueprintNoticeGQLType), func(d BlueprintSyncDetail) any { return d.Errors }),
		"requiredManualEnvVars": gqlutil.Typed(graphql.NewList(manualEnvVarGQLType),
			func(d BlueprintSyncDetail) any { return d.RequiredManualEnvVars }),
		"dashboardUrl": gqlutil.StrField(func(d BlueprintSyncDetail) any { return d.DashboardURL }),
	},
})

// blueprintSyncGQLQueryFields is the sync page's read, `blueprintSync`.
func (s *Service) blueprintSyncGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"blueprintSync": &graphql.Field{
			Type: blueprintSyncDetailGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"syncId":  gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetBlueprintSync(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"), p.Args["syncId"].(string))
			},
		},
	}
}
