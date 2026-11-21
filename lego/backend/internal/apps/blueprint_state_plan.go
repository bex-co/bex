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
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintActionPlan returns an authorized current-state plan when the
// complete non-secret state needed to make one is available. An absent env
// group is a create; an existing one is an update only when the apply would
// write one of its variables, which the plan learns without reading the
// values out (planEnvGroupVars).
func (s *Service) blueprintActionPlan(ctx context.Context, ir BlueprintIR, st parsedStack, blueprintID string, snap *workspaceSnapshot) (BlueprintPlan, bool, error) {
	if s.Client == nil || (len(st.envGroups) > 0 && s.EnvGroups == nil) {
		if err := checkBlueprintDatabaseCreates(st); err != nil {
			return BlueprintPlan{}, false, err
		}
		return BlueprintPlan{}, false, checkBlueprintEnvGroupCreates(st)
	}
	resolver, err := newBlueprintActionResolver(ctx, s, st, snap)
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	plan, err := PlanBlueprintIR(ctx, ir, resolver)
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	// The apply writes the environments before any resource placed in them.
	environments, err := resolver.planEnvironments(ctx)
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	plan.Actions = append(environments, plan.Actions...)
	detached, err := s.blueprintDetachments(ctx, s.resolveTenantID(ctx), blueprintID, st, func() (*blueprintActionResolver, error) { return resolver, nil })
	if err != nil {
		return BlueprintPlan{}, false, err
	}
	for _, resource := range detached {
		plan.Actions = append(plan.Actions, BlueprintPlanAction{Operation: BlueprintPlanDetach, Kind: resource.kind, Name: resource.Name, ResourceID: resource.ID, Message: "Stops being managed by this blueprint; continues running and may incur charges."})
	}
	return plan, true, nil
}

type blueprintActionResolver struct {
	services  map[string]*appv1alpha1.App
	databases map[string]*appv1alpha1.Database
	keyValues map[string]*appv1alpha1.KeyValue
	envGroups map[string]string
	// buildSources are the workspace's build source carriers by name; links
	// resolves each service's buildSource name (nil: one the apply creates).
	buildSources map[string]*appv1alpha1.App
	links        map[string]*appv1alpha1.App
	parsed       parsedStack
	// service resolves a source switch's branch as the apply does (w8/m80).
	service *Service
	// snap reads the workspace's projects and environments for the moves.
	snap *workspaceSnapshot
}

func newBlueprintActionResolver(ctx context.Context, s *Service, parsed parsedStack, snap *workspaceSnapshot) (*blueprintActionResolver, error) {
	resolver := &blueprintActionResolver{
		databases: map[string]*appv1alpha1.Database{},
		keyValues: map[string]*appv1alpha1.KeyValue{},
		envGroups: map[string]string{},
		parsed:    parsed,
		service:   s,
		snap:      snap,
	}
	if len(parsed.envGroups) > 0 {
		groups, err := s.EnvGroups.GroupIDsByName(ctx)
		if err != nil {
			return nil, err
		}
		resolver.envGroups = groups
	}
	tenantID, scoped := s.Tenant(ctx)
	services, err := snap.services(ctx)
	if err != nil {
		return nil, err
	}
	resolver.services = services
	if parsed.linksBuildSources() || len(parsed.buildSources) > 0 {
		if resolver.buildSources, err = snap.buildSources(ctx); err != nil {
			return nil, err
		}
		if resolver.links, err = blueprintBuildSourceLinks(parsed, resolver.buildSources); err != nil {
			return nil, err
		}
	}
	databases, err := snap.databases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range databases.Items {
		database := &databases.Items[i]
		if scoped && database.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if _, duplicate := resolver.databases[database.Spec.Name]; duplicate {
			return nil, fmt.Errorf("%w: database name %q is already used more than once in this workspace", core.ErrConflict, database.Spec.Name)
		}
		resolver.databases[database.Spec.Name] = database
	}
	keyValues, err := snap.keyValues(ctx)
	if err != nil {
		return nil, err
	}
	for i := range keyValues.Items {
		keyValue := &keyValues.Items[i]
		if scoped && keyValue.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if _, duplicate := resolver.keyValues[keyValue.Spec.Name]; duplicate {
			return nil, fmt.Errorf("%w: key-value name %q is already used more than once in this workspace", core.ErrConflict, keyValue.Spec.Name)
		}
		resolver.keyValues[keyValue.Spec.Name] = keyValue
	}
	return resolver, nil
}

func (r *blueprintActionResolver) ResolveBlueprintResource(_ context.Context, kind BlueprintResourceKind, name string) (BlueprintCurrentResource, bool, error) {
	switch kind {
	case BlueprintResourceService:
		app, ok := r.services[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		id := store.ManagedAppID(app.Labels)
		if id == "" {
			id = app.Name
		}
		return BlueprintCurrentResource{ID: id}, true, nil
	case BlueprintResourcePostgres:
		database, ok := r.databases[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		return BlueprintCurrentResource{ID: database.Name}, true, nil
	case BlueprintResourceKeyValue:
		keyValue, ok := r.keyValues[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		return BlueprintCurrentResource{ID: keyValue.Name}, true, nil
	case BlueprintResourceEnvVarGroup:
		groupID, ok := r.envGroups[name]
		return BlueprintCurrentResource{ID: groupID}, ok, nil
	case BlueprintResourceBuildSource:
		carrier, ok := r.buildSources[name]
		if !ok {
			return BlueprintCurrentResource{}, false, nil
		}
		return BlueprintCurrentResource{ID: core.AppPublicID(carrier)}, true, nil
	default:
		return BlueprintCurrentResource{}, false, nil
	}
}

// checkBlueprintServicePlanGates runs the deterministic paid-tier admission
// the service write path runs on every create and update: the Scale-workspace
// inbound-rules gate, then the Pro-workspace autoscaling gate
// (validateBlueprintService and applyStackService run the same two calls on
// the same spec). Both degrade open where no workspace plan applies, exactly
// as the write path does, so the verdicts agree there too. current is the
// existing App, nil on create: a re-sent rule or an unchanged policy stays
// allowed on any plan.
func (s *Service) checkBlueprintServicePlanGates(ctx context.Context, want appv1alpha1.AppSpec, current *appv1alpha1.App) error {
	tenantID := s.resolveTenantID(ctx)
	if err := s.requireServiceInboundRules(ctx, tenantID, want, current); err != nil {
		return err
	}
	return s.requireAutoscalingPlan(ctx, tenantID, want.Autoscaling, current, "scaling")
}

func (r *blueprintActionResolver) PlanBlueprintResource(ctx context.Context, resource BlueprintResourceIR, current BlueprintCurrentResource, exists bool) (BlueprintPlanAction, error) {
	action := BlueprintPlanAction{Kind: resource.Kind, Name: resource.Name, SourcePath: resource.SourcePath, ResourceID: current.ID}
	if !exists {
		if resource.Kind == BlueprintResourcePostgres {
			db, ok := parsedBlueprintDatabase(r.parsed, resource.Name)
			if !ok {
				return BlueprintPlanAction{}, fmt.Errorf("compiled database is missing")
			}
			if err := postgres.CheckNewDatabase(db.spec); err != nil {
				return BlueprintPlanAction{}, blueprintDatabaseResourceError(resource.Name, blueprintDatabaseAdmissionConflict(err, db.fields))
			}
		}
		if resource.Kind == BlueprintResourceEnvVarGroup {
			group, ok := parsedBlueprintEnvGroup(r.parsed, resource.Name)
			if !ok {
				return BlueprintPlanAction{}, fmt.Errorf("compiled env group is missing")
			}
			if err := checkNewEnvGroupQuota(group); err != nil {
				return BlueprintPlanAction{}, blueprintResourceErrors{{kind: resource.Kind, name: resource.Name, err: err}}
			}
		}
		if resource.Kind == BlueprintResourceKeyValue {
			kv, ok := parsedBlueprintKeyValue(r.parsed, resource.Name)
			if !ok {
				return BlueprintPlanAction{}, fmt.Errorf("compiled key value is missing")
			}
			// A paused tier is refused here as the apply refuses it:
			// applyKeyValue judges blueprintKeyValueGatePlan before its first
			// write, and the preview judges the same value. Without this the
			// plan shows a create the sync only refuses after writing the
			// grouping and env-group rows. Downgrades and external access are
			// update-only judgments on both sides, so a create judges neither
			// here either.
			if err := keyvalue.ErrPausedPlan(blueprintKeyValueGatePlan(kv, ""), ""); err != nil {
				return BlueprintPlanAction{}, blueprintResourceErrors{{kind: resource.Kind, name: resource.Name, err: err}}
			}
		}
		// A create the apply would refuse for a paid-tier feature is a plan
		// refusal, not a create: validate and the apply run these gates on
		// every service write (validateBlueprintService, applyStackService).
		// A spec that does not build is left to those stages, which report
		// the spec error; the gates below need the built spec to judge.
		if resource.Kind == BlueprintResourceService && r.service != nil {
			if svc, ok := parsedBlueprintService(r.parsed, resource.Name); ok {
				if want, err := blueprintReadSpec(ctx, svc.req); err == nil {
					if err := r.service.checkBlueprintServicePlanGates(ctx, want, nil); err != nil {
						return BlueprintPlanAction{}, blueprintServiceResourceError(resource, err)
					}
				}
			}
		}
		action.Operation = BlueprintPlanCreate
		if svc, ok := parsedBlueprintService(r.parsed, resource.Name); ok && resource.Kind == BlueprintResourceService && len(svc.generatedCopies) > 0 {
			action.Message = generatedCopyMessage(svc.generatedCopies)
		}
		return action, nil
	}
	switch resource.Kind {
	case BlueprintResourceBuildSource:
		return r.planBlueprintBuildSource(ctx, resource, action)
	case BlueprintResourceService:
		svc, ok := parsedBlueprintService(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled service is missing")
		}
		var refsPending bool
		var err error
		if svc.req, refsPending, err = r.withPlannedRefs(svc); err != nil {
			return BlueprintPlanAction{}, err
		}
		want, err := blueprintReadSpec(ctx, withCurrentTier(svc.req, r.services[resource.Name]))
		if err != nil {
			return BlueprintPlanAction{}, err
		}
		want.BuildSource = desiredBuildSourceLink(svc, r.links)
		// The sync refuses a paid-tier feature the workspace plan does not
		// include, as validate and the apply do (validateBlueprintService,
		// applyStackService run the same gates on the same spec): a plan
		// that promised the change would approve what the apply refuses.
		if r.service != nil {
			if err := r.service.checkBlueprintServicePlanGates(ctx, want, r.services[resource.Name]); err != nil {
				return BlueprintPlanAction{}, blueprintServiceResourceError(resource, err)
			}
		}
		// The sync checks an existing service's link to a workspace build
		// source as PATCH buildSourceId does (blueprintServiceLink), so the
		// plan refuses what the sync would. A source this file declares was
		// checked at parse against its declared runtime (w3/069).
		if carrier := r.links[svc.req.blueprintBuildSource]; carrier != nil && r.service != nil {
			if _, declared := parsedBlueprintBuildSource(r.parsed, svc.req.blueprintBuildSource); !declared {
				req := svc.req
				req.BuildSourceID = core.AppPublicID(carrier)
				if _, err := r.service.blueprintServiceLink(ctx, req, svc.fields, r.services[resource.Name]); err != nil {
					return BlueprintPlanAction{}, blueprintResourceErrors{{kind: resource.Kind, name: resource.Name, err: err}}
				}
			}
		}
		// The sync's own hook reconciliation, so a completed legacy hook does
		// not plan a change the apply never makes (w8/m63).
		initialDeployHookState(svc.req, r.services[resource.Name], &want)
		if err := r.service.resolveBlueprintSwitchBranch(ctx, svc.req, svc.fields, r.services[resource.Name], &want); err != nil {
			return BlueprintPlanAction{}, err
		}
		// served is the live spec as the plan compares against it, the whole
		// apply and each field's probe alike. Pending domain claims are
		// configured domains the serving spec does not carry yet (w4/411):
		// compare against the same union the exporter writes, or a generated
		// manifest re-plans as an update against its own source and would
		// promote a pending domain into serving.
		served := r.services[resource.Name].Spec
		if r.service != nil {
			domains, err := r.service.exportedDomains(ctx, r.services[resource.Name])
			if err != nil {
				return BlueprintPlanAction{}, err
			}
			if len(domains) > 0 {
				served.Host, served.Hosts = domains[0], domains[1:]
			}
		}
		dropServedPortEnv(&want, r.services[resource.Name])
		probe := *served.DeepCopy()
		changed, err := ApplyBlueprintServiceSpec(&probe, want, svc.fields)
		if err != nil {
			return BlueprintPlanAction{}, blueprintServiceResourceError(resource, err)
		}
		if changed {
			action.Operation = BlueprintPlanUpdate
			// The probes run over the applied set, as the build-source plan
			// does: x-bex.builder is lifted onto svc.fields and never a
			// top-level resource field, so probing the raw set could move
			// the strategy without naming the builder.
			action.ChangedFields = blueprintPlanFieldChanges(blueprintServiceProbePaths(svc.fields), func(name string) (bool, error) {
				probe := *served.DeepCopy()
				return probeBlueprintServiceSpec(&probe, want, svc.fields, name)
			})
		}
		// A copy of a sibling's generated value lands in the store, which the
		// plan does not read: like a group's write-only values, it is a
		// possible update, named but never valued (w1/m175).
		if len(svc.generatedCopies) > 0 {
			addChangedFields(&action, "envVars")
			action.Message = generatedCopyMessage(svc.generatedCopies)
		}
		if refsPending {
			addChangedFields(&action, "envVars")
		}
		// A release the sync rolls runs the declared image, replacing a
		// per-deploy image override the service serves; the spec diff cannot
		// see that override (spec.releaseConfig), so name it (w4/m418).
		current := r.services[resource.Name]
		rolls := len(svc.generatedCopies) > 0 || refsPending || (changed && appv1alpha1.SpecRollsRelease(current.Spec, probe))
		if running := servingImage(current); rolls && running != "" {
			addChangedFields(&action, "image")
			message := imageOverrideMessage(running, probe.Image)
			if action.Message != "" {
				message = action.Message + "; " + message
			}
			action.Message = message
		}
		if err := r.planMoves(ctx, &action, svc.grouping, svc.ungrouped, true, r.services[resource.Name].Labels); err != nil {
			return BlueprintPlanAction{}, err
		}
		if action.Operation == BlueprintPlanUpdate {
			return action, nil
		}
	case BlueprintResourcePostgres:
		database, ok := parsedBlueprintDatabase(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled database is missing")
		}
		probe := r.databases[resource.Name].DeepCopy()
		now := r.service.Now()
		changed, err := ApplyBlueprintDatabaseSpec(probe, database.spec, database.fields, now)
		if err != nil {
			return BlueprintPlanAction{}, blueprintDatabaseResourceError(resource.Name, err)
		}
		live := r.databases[resource.Name]
		if changed {
			action.Operation = BlueprintPlanUpdate
			action.ChangedFields = blueprintPlanFieldChanges(maps.Keys(resource.Fields), func(name string) (bool, error) {
				return ApplyBlueprintDatabaseSpec(live.DeepCopy(), database.spec, oneField(database.fields, name), now)
			})
		}
		if err := r.planMoves(ctx, &action, database.grouping, database.ungrouped, false, live.Labels); err != nil {
			return BlueprintPlanAction{}, err
		}
		if action.Operation == BlueprintPlanUpdate {
			return action, nil
		}
	case BlueprintResourceKeyValue:
		keyValue, ok := parsedBlueprintKeyValue(r.parsed, resource.Name)
		if !ok {
			return BlueprintPlanAction{}, fmt.Errorf("compiled key value is missing")
		}
		live := r.keyValues[resource.Name]
		probe := live.Spec
		if ApplyBlueprintKeyValueSpec(&probe, keyValue.spec, keyValue.fields) {
			if err := keyvalue.RefuseBlueprintExternalAccessRestart(live, probe.Public); err != nil {
				return BlueprintPlanAction{}, blueprintResourceErrors{{kind: BlueprintResourceKeyValue, name: resource.Name, err: err}}
			}
			if err := keyvalue.ErrPlanDowngrade(probe.Plan, live.Spec.Plan); err != nil {
				return BlueprintPlanAction{}, blueprintResourceErrors{{kind: BlueprintResourceKeyValue, name: resource.Name, err: err}}
			}
			// A plan change onto a paused tier is refused in the plan too, as
			// the apply refuses it; re-declaring the store's own paused plan
			// (probe.Plan == live.Spec.Plan) stays allowed.
			if err := keyvalue.ErrPausedPlan(probe.Plan, live.Spec.Plan); err != nil {
				return BlueprintPlanAction{}, blueprintResourceErrors{{kind: BlueprintResourceKeyValue, name: resource.Name, err: err}}
			}
			action.Operation = BlueprintPlanUpdate
			liveSpec := live.Spec
			action.ChangedFields = blueprintPlanFieldChanges(maps.Keys(resource.Fields), func(name string) (bool, error) {
				probe := *liveSpec.DeepCopy()
				return ApplyBlueprintKeyValueSpec(&probe, keyValue.spec, oneField(keyValue.fields, name)), nil
			})
		}
		if err := r.planMoves(ctx, &action, keyValue.grouping, keyValue.ungrouped, false, live.Labels); err != nil {
			return BlueprintPlanAction{}, err
		}
		if action.Operation == BlueprintPlanUpdate {
			return action, nil
		}
	case BlueprintResourceEnvVarGroup:
		// The group was matched by its name, so name is never a change.
		if err := r.planEnvGroupVars(ctx, &action, resource.Name); err != nil {
			return BlueprintPlanAction{}, err
		}
		if err := r.planEnvGroupMove(ctx, &action, resource.Name); err != nil {
			return BlueprintPlanAction{}, err
		}
		if action.Operation == BlueprintPlanUpdate {
			return action, nil
		}
	}
	action.Operation = BlueprintPlanNoop
	return action, nil
}

// withPlannedRefs is svc's request with its references resolved as the apply
// writes them (resolveServiceRefs), against the datastores and services that
// exist now, so the envVars probe compares them with the live service. Until
// w8/300 the probe saw only the file's literals: a sync that added
// DATABASE_URL, or pointed it at another database, planned no change to it.
// pending reports a reference to a resource this file declares and this sync
// creates: it cannot resolve before that create, and the service cannot hold
// it yet, so it is a change. A reference to nothing is the apply's error.
func (r *blueprintActionResolver) withPlannedRefs(svc parsedService) (req CreateRequest, pending bool, err error) {
	if len(svc.databaseRefs)+len(svc.kvRefs)+len(svc.hostRefs) == 0 {
		return svc.req, false, nil
	}
	for _, ref := range svc.databaseRefs {
		if _, ok := r.databases[ref.FromDatabase.Name]; !ok {
			if _, declared := parsedBlueprintDatabase(r.parsed, ref.FromDatabase.Name); declared {
				return svc.req, true, nil
			}
		}
	}
	for _, ref := range svc.kvRefs {
		if _, ok := r.keyValues[ref.FromService.Name]; !ok {
			if _, declared := parsedBlueprintKeyValue(r.parsed, ref.FromService.Name); declared {
				return svc.req, true, nil
			}
		}
	}
	dbRefs := make(map[string]databaseRef, len(r.databases))
	for name, database := range r.databases {
		dbRefs[name] = databaseRefOf(database)
	}
	kvCRNames := make(map[string]string, len(r.keyValues))
	for name, keyValue := range r.keyValues {
		kvCRNames[name] = keyValue.Name
	}
	svc.req.Env = slices.Clone(svc.req.Env)
	// A forward host reference names a sibling this sync creates (an unknown
	// target is refused by hostReferenceProblems before the plan).
	later, err := resolveServiceRefs(&svc, dbRefs, kvCRNames, func(name string) *appv1alpha1.App { return r.services[name] }, r.service.BaseDomain)
	if err != nil {
		return CreateRequest{}, false, err
	}
	return svc.req, len(later) > 0, nil
}

// planMoves adds to an existing resource's action the placement its apply
// changes (blueprintPlacements.moves): a move into another project or
// environment, or out of every one, and a service's network isolation, are an
// update (w9/094), so a Blueprint whose syncs wait for approval offers them
// and its approval applies them. Without the grouping store a resource
// declared in a grouping is not compared: its apply is refused
// (applyBlueprintGroupings).
func (r *blueprintActionResolver) planMoves(ctx context.Context, action *BlueprintPlanAction, grouping string, ungrouped, isolation bool, labels map[string]string) error {
	var placements blueprintPlacements
	if grouping != "" {
		if _, scoped := r.service.Tenant(ctx); r.service.BlueprintGroups == nil || !scoped {
			return nil
		}
		var err error
		if placements, err = r.service.blueprintEnvironmentPlacements(ctx, r.snap, r.parsed.groupings); err != nil {
			return err
		}
	}
	if moves := placements.moves(grouping, ungrouped, isolation, labels); len(moves) > 0 {
		addChangedFields(action, moves...)
	}
	return nil
}

// envGroupValuesUnread is an existing group's update message when the plan
// cannot compare the declared variables with the group's values.
const envGroupValuesUnread = "This plan cannot read the group's values, so its variables are listed as changed; a sync writes only those that differ."

// planEnvGroupVars lists envVars for an existing group when the apply would
// write one of its declared variables: EnvGroupWritesOwed runs
// ApplyEnvGroup's own diff, so an unchanged group plans no change (w8/376).
// Until then every existing group planned as an update of its declared
// fields, and approving that plan wrote nothing. When the plan cannot tell,
// because the seam is not wired or the caller may not read the group's
// values, envVars is listed with envGroupValuesUnread: a write the plan
// cannot rule out must not pass as no change, and a viewer's plan must not
// reveal whether a declared value matches the stored one.
func (r *blueprintActionResolver) planEnvGroupVars(ctx context.Context, action *BlueprintPlanAction, name string) error {
	group, ok := parsedBlueprintEnvGroup(r.parsed, name)
	if !ok {
		return fmt.Errorf("compiled env group is missing")
	}
	owed, known := true, false
	if writesOwed := r.service.EnvGroupWritesOwed; writesOwed != nil {
		var err error
		owed, err = writesOwed(ctx, group.name, group.literals, group.generates)
		switch {
		case errors.Is(err, core.ErrForbidden):
			owed = true
		case err != nil:
			return err
		default:
			known = true
		}
	}
	if owed {
		addChangedFields(action, "envVars")
	}
	if !known {
		action.Message = envGroupValuesUnread
	}
	return nil
}

// planEnvGroupMove names the environment move the apply makes for an existing
// group declared under a projects[].environments[] nesting
// (applyStackEnvGroups' SetGroupEnvironment), as planMoves does for the
// kinds the regroup moves. A group declared at the top level keeps its scope,
// so nothing is named; a grouping the apply has yet to create is always a
// move. The group's current environment comes from the EnvGroupPlacements
// seam; when it is unavailable the move is named, as a move the plan cannot
// prove absent must not pass as a value-only update.
func (r *blueprintActionResolver) planEnvGroupMove(ctx context.Context, action *BlueprintPlanAction, name string) error {
	group, ok := parsedBlueprintEnvGroup(r.parsed, name)
	if !ok || group.grouping == "" {
		return nil
	}
	if r.service == nil {
		return nil
	}
	if _, scoped := r.service.Tenant(ctx); r.service.BlueprintGroups == nil || !scoped {
		return nil
	}
	placements, err := r.service.blueprintEnvironmentPlacements(ctx, r.snap, r.parsed.groupings)
	if err != nil {
		return err
	}
	current, unknown := "", true
	if r.service.EnvGroupPlacements != nil {
		byName, err := r.service.EnvGroupPlacements(ctx)
		if err != nil {
			return err
		}
		current, unknown = byName[name], false
	}
	if target := placements.targets[group.grouping]; !unknown && target.environmentID == current && target.environmentID != "" {
		return nil // stays put
	}
	addChangedFields(action, "environment")
	return nil
}

// planEnvironments lists each projects[].environments[] entry the apply writes
// (applyBlueprintGroupingsWith): a create when its project has no environment
// of that name yet, and an update listing the settings of its own
// (permissions.protection, networking.isolation) the file changes. An
// unchanged environment is not listed, so an unchanged file's plan stays as
// it was. Until w9/095 the plan listed none, so a change to those settings
// alone planned no_changes, and a Blueprint whose syncs wait for approval
// never applied it. Without the grouping store nothing is listed: the apply
// refuses a grouped file.
func (r *blueprintActionResolver) planEnvironments(ctx context.Context) ([]BlueprintPlanAction, error) {
	groupings := r.parsed.groupings
	if len(groupings) == 0 {
		return nil, nil
	}
	if _, scoped := r.service.Tenant(ctx); r.service.BlueprintGroups == nil || !scoped {
		return nil, nil
	}
	placements, err := r.service.blueprintEnvironmentPlacements(ctx, r.snap, groupings)
	if err != nil {
		return nil, err
	}
	var actions []BlueprintPlanAction
	for _, g := range groupings {
		target := placements.targets[blueprintGroupingKey(g.projectName, g.environmentName)]
		action := BlueprintPlanAction{Operation: BlueprintPlanCreate, Kind: BlueprintResourceEnvironment, Name: g.environmentName, SourcePath: g.pointer}
		if target.environmentID != "" {
			if len(target.changed) == 0 {
				continue
			}
			action.ResourceID = ids.EnvironmentPublicID(target.environmentID)
			addChangedFields(&action, target.changed...)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// addChangedFields makes action an update that lists paths, each once, in
// path order.
func addChangedFields(action *BlueprintPlanAction, paths ...string) {
	action.Operation = BlueprintPlanUpdate
	for _, path := range paths {
		if !slices.ContainsFunc(action.ChangedFields, func(c BlueprintFieldChange) bool { return c.Path == path }) {
			action.ChangedFields = append(action.ChangedFields, BlueprintFieldChange{Path: path})
		}
	}
	sort.Slice(action.ChangedFields, func(i, j int) bool { return action.ChangedFields[i].Path < action.ChangedFields[j].Path })
}

func parsedBlueprintService(st parsedStack, name string) (parsedService, bool) {
	for _, service := range st.services {
		if service.req.Name == name {
			return service, true
		}
	}
	return parsedService{}, false
}

func parsedBlueprintDatabase(st parsedStack, name string) (parsedDatabase, bool) {
	for _, database := range st.databases {
		if database.name == name {
			return database, true
		}
	}
	return parsedDatabase{}, false
}

func parsedBlueprintKeyValue(st parsedStack, name string) (parsedKeyValue, bool) {
	for _, keyValue := range st.keyValues {
		if keyValue.name == name {
			return keyValue, true
		}
	}
	return parsedKeyValue{}, false
}

func parsedBlueprintEnvGroup(st parsedStack, name string) (parsedEnvGroup, bool) {
	for _, group := range st.envGroups {
		if group.name == name {
			return group, true
		}
	}
	return parsedEnvGroup{}, false
}

// blueprintPlanFieldChanges reports which of a manifest's declared fields
// actually differ from the live resource — not, as it did until w4/m124, every
// field the manifest happens to declare.
//
// The old behavior made the plan unreadable in the way that matters: two
// key-value manifests differing in `maxmemoryPolicy` produced byte-identical
// output, and a service manifest reported all ten of its declared paths
// whatever the difference was. A field list that does not depend on the diff is
// not a diff.
//
// It is computed by applying each path ALONE to a copy of the live spec and
// asking whether the spec moved, so the answer comes from the very appliers a
// real sync would run. A diff derived any other way can disagree with what
// apply does, which is the failure this whole milestone is about. A service
// also probes the paths it omits, each judged beside the fields it declares
// (probeBlueprintServiceSpec).
//
// applyOne returns whether applying just `name` changed the resource.
func blueprintPlanFieldChanges(paths iter.Seq[string], applyOne func(name string) (bool, error)) []BlueprintFieldChange {
	changes := []BlueprintFieldChange{}
	for name := range paths {
		changed, err := applyOne(name)
		if err != nil {
			// A field that cannot be applied in isolation is reported rather
			// than dropped: the plan must not claim a resource is unaffected by
			// something it could not evaluate.
			changes = append(changes, BlueprintFieldChange{Path: name})
			continue
		}
		if changed {
			changes = append(changes, BlueprintFieldChange{Path: name})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

// blueprintServiceProbePaths are the paths a service plan probes: each declared
// field, and each service field the file omits, since an omission can change a
// service too — an omitted buildFilter clears, an image service takes the
// Blueprint's repo, and an omitted buildSource unlinks the service — and is
// then listed under the path it omits, once.
func blueprintServiceProbePaths(declared map[string]BlueprintField) iter.Seq[string] {
	return func(yield func(string) bool) {
		for name := range declared {
			if !yield(name) {
				return
			}
		}
		// buildSource is no policy row: a service keeps a link the file stops
		// naming while it has no source of its own, so the omission is not a
		// value the file owns. But an entry that inherits the Blueprint's repo
		// does have one, and the apply unlinks the service and points it there;
		// the change belongs to the path the file dropped. Left unprobed, that
		// sync planned an update naming nothing.
		if _, ok := declared["buildSource"]; !ok && !yield("buildSource") {
			return
		}
		for _, policy := range BlueprintServiceFieldPolicies {
			if _, ok := declared[policy.Name]; !ok && !yield(policy.Name) {
				return
			}
		}
	}
}

// oneField narrows a declared field set to a single entry, so an applier can be
// run in isolation.
func oneField(fields map[string]BlueprintField, name string) map[string]BlueprintField {
	if f, ok := fields[name]; ok {
		return map[string]BlueprintField{name: f}
	}
	return nil
}

// imageOverrideMessage says a sync replaces the image override a service is
// running with the image its manifest declares (w4/m418).
func imageOverrideMessage(running, declared string) string {
	return fmt.Sprintf("replaces the running image %s with the declared %s", store.PublicImageRef(running), store.PublicImageRef(declared))
}

// generatedCopyMessage names a service's copies of generated values for the
// plan, keys only.
func generatedCopyMessage(copies []generatedCopy) string {
	parts := make([]string, 0, len(copies))
	for _, c := range copies {
		parts = append(parts, fmt.Sprintf("%s from %s's generated %s", c.key, c.source, c.sourceKey))
	}
	return "copies at apply: " + strings.Join(parts, ", ")
}
