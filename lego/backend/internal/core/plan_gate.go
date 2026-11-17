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

package core

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// Workspace plan ids, ladder order. Mirrors store.PlanHobby…PlanEnterprise
// (core cannot import store).
const (
	planHobby      = "hobby"
	planPro        = "pro"
	planScale      = "scale"
	planEnterprise = "enterprise"
)

var planLadder = []string{planHobby, planPro, planScale, planEnterprise}

// PlanFeature is a capability Render gates by workspace plan
// (render.com/docs/platform-features-by-plan, w8/m137, w8/m138).
type PlanFeature string

const (
	FeatureInboundIPRules   PlanFeature = "inbound IP rules"
	FeatureNetworkIsolation PlanFeature = "network-isolated environments"
	FeatureWebhooks         PlanFeature = "webhooks"
	FeatureRequestLogs      PlanFeature = "HTTP request logs"
	FeatureAuditLogs        PlanFeature = "audit logs"
	FeatureLatencyMetrics   PlanFeature = "response latency metrics"
	FeatureAutoscaling      PlanFeature = "autoscaling"
)

// planFeatures is the one table of each gated feature's lowest plan and its
// stable machine name (a PLAN_FEATURE_REQUIRED refusal's params.feature). A
// feature absent from it is open on every plan. singular marks a feature
// named by a mass noun, which the refusal sentence reads with "requires".
var planFeatures = map[PlanFeature]struct {
	minimum, key string
	singular     bool
}{
	FeatureInboundIPRules:   {minimum: planScale, key: "inboundIPRules"},
	FeatureNetworkIsolation: {minimum: planPro, key: "networkIsolation"},
	FeatureWebhooks:         {minimum: planPro, key: "webhooks"},
	FeatureRequestLogs:      {minimum: planPro, key: "requestLogs"},
	FeatureAuditLogs:        {minimum: planPro, key: "auditLogs"},
	FeatureLatencyMetrics:   {minimum: planPro, key: "latencyMetrics"},
	// render.com/docs/scaling (read 2026-11-16): "Autoscaling: Available only
	// for Pro workspaces and higher" (w7/m429).
	FeatureAutoscaling: {minimum: planPro, key: "autoscaling", singular: true},
}

// Machine codes for the workspace-plan refusals (w8/m138 t013), on REST's
// `code`, GraphQL's extensions.code and MCP's message prefix alike. Webhooks
// keep their earlier WEBHOOK_PLAN_LIMIT (webhooks.PlanLimitCode), which
// clients already branch on.
const (
	// PlanLimitCode is a per-plan count cap (members, environments per
	// project): params {plan, limit[, field]}.
	PlanLimitCode = "PLAN_LIMIT"
	// PlanFeatureRequiredCode is a feature the workspace plan does not include:
	// params {plan, feature, requiredPlan[, field]}.
	PlanFeatureRequiredCode = "PLAN_FEATURE_REQUIRED"
)

// NewPlanFeatureError is the coded refusal for a feature plan does not
// include. field, when set, prefixes the message the way the request field's
// validation errors read ("bad request: type: HTTP request logs require …"),
// so the text is byte-identical to the uncoded refusal it replaced.
func NewPlanFeatureError(field, plan string, feature PlanFeature) *CodedError {
	msg := PlanFeatureMessage(plan, feature)
	gate := planFeatures[feature]
	params := map[string]any{"plan": plan, "feature": gate.key, "requiredPlan": gate.minimum}
	if field != "" {
		msg = field + ": " + msg
		params["field"] = field
	}
	return NewBadRequestError(PlanFeatureRequiredCode, ErrBadRequest.Error()+": "+msg, params)
}

// PlanAllows reports whether plan includes feature. An unknown plan is treated
// as allowed (store.LimitsFor's safe default: validation rejects unknown plans
// upstream, so this is never a silent downgrade).
func PlanAllows(plan string, feature PlanFeature) bool {
	gate, gated := planFeatures[feature]
	if !gated {
		return true
	}
	have := slices.Index(planLadder, plan)
	return have < 0 || have >= slices.Index(planLadder, gate.minimum)
}

// PlanFeatureMessage is the one refusal sentence for a feature the workspace
// plan does not include.
func PlanFeatureMessage(plan string, feature PlanFeature) string {
	title := func(p string) string { return strings.ToUpper(p[:1]) + p[1:] }
	gate := planFeatures[feature]
	verb := "require"
	if gate.singular {
		verb = "requires"
	}
	return fmt.Sprintf("%s %s the %s plan or higher; this workspace is on %s", feature, verb, title(gate.minimum), title(plan))
}

// MaxEnvironmentsPerProject is Render's per-project environment cap for a plan:
// Hobby is limited to two, every paid plan is unlimited (0).
func MaxEnvironmentsPerProject(plan string) int {
	if plan == planHobby {
		return 2
	}
	return 0
}

// WebhookDestinationLimit is how many webhook destinations a plan allows:
// none on Hobby, one on Pro, up to 100 on Scale and above.
func WebhookDestinationLimit(plan string) int {
	switch plan {
	case planHobby:
		return 0
	case planPro:
		return 1
	default:
		return 100
	}
}

// keptBuilds is how many of a service's most recent builds each plan keeps
// for rollback (w7/m429; render.com/pricing's "Instant rollbacks" row, read
// 2026-11-16: 5, 15, 30 and 30 builds retained). bex-api projects the count
// onto every App of the workspace (appv1alpha1.LabelKeptBuilds) and the
// operator's registry retention keeps that many live builds per repository.
var keptBuilds = map[string]int{
	planHobby:      5,
	planPro:        15,
	planScale:      30,
	planEnterprise: 30,
}

// KeptBuilds is how many of a service's most recent builds plan keeps for
// rollback. ok is false for an unknown plan, which leaves the registry's
// default retention in place.
func KeptBuilds(plan string) (n int, ok bool) {
	n, ok = keptBuilds[plan]
	return n, ok
}

// includedCustomDomains is how many custom domains each plan includes before
// they are metered (w7/m429; render.com/pricing's "Custom domains" row, read
// 2026-11-16: "2 included then $0.25/domain/month" on Hobby, 15 on Pro, 25 on
// Scale, custom on Enterprise). Enterprise is absent: its domains are all
// included.
var includedCustomDomains = map[string]int{
	planHobby: 2,
	planPro:   15,
	planScale: 25,
}

// IncludedCustomDomains is how many custom domains plan includes. unlimited
// is true for Enterprise and for an unknown plan, which are never metered
// (an unknown plan never reaches here: tenants.plan's CHECK keeps it out).
func IncludedCustomDomains(plan string) (n int, unlimited bool) {
	n, ok := includedCustomDomains[plan]
	return n, !ok
}

// concurrentSandboxes is how many sandboxes a workspace on each plan may have
// active at once across all of its sandbox groups (w7/069; render.com/pricing's
// "Default concurrent sandboxes" row, read 2026-11-16: 100 on Hobby, 1,000 on
// Pro and Scale, custom on Enterprise). It is the workspace's total, not a
// group's default: Render's "Render Sandboxes vs Vercel Sandboxes" (2026-11-06)
// reads "Render Pro defaults to 1,000 per workspace", and
// render.com/docs/sandboxes-limits documents a group's own 1,000 separately.
// Enterprise gets Scale's number, as it does for kept builds; Render's raise
// "on request" is a support action, not a product surface.
var concurrentSandboxes = map[string]int{
	planHobby:      100,
	planPro:        1000,
	planScale:      1000,
	planEnterprise: 1000,
}

// ConcurrentSandboxes is how many sandboxes a workspace on plan may have active
// at once across its groups. An unknown plan, or none (no plan reader, the
// default tenant), gets the smallest, Hobby's.
func ConcurrentSandboxes(plan string) int {
	if n, ok := concurrentSandboxes[plan]; ok {
		return n
	}
	return concurrentSandboxes[planHobby]
}

// planRetention is how far back each plan's reads of one kind reach. Logs,
// metrics and Postgres point-in-time recovery each have their own table:
// Render documents them separately.
type planRetention map[string]time.Duration

// window is how far back plan's reads reach. An unknown plan gets the shortest
// window. That is the opposite of PlanAllows, which opens a gate for an
// unknown plan: a retention window only ever hides data, so the safe default
// is to hide the most (validation and tenants.plan's CHECK keep unknown plans
// out anyway).
func (r planRetention) window(plan string) time.Duration {
	if window, ok := r[plan]; ok {
		return window
	}
	return r.shortest()
}

// shortest is the shortest plan window: no workspace's floor is ever later
// than now minus it.
func (r planRetention) shortest() time.Duration {
	return slices.Min(slices.Collect(maps.Values(r)))
}

// longest is the longest plan window: the store behind the reads must keep at
// least this much.
func (r planRetention) longest() time.Duration {
	return slices.Max(slices.Collect(maps.Values(r)))
}

// floor is the earliest instant a workspace on plan reaches back to at now:
// its plan's window back from now, or visibleFrom while that is later.
// visibleFrom is the floor a plan change kept (visibleFromAfterPlanChange);
// zero means none.
func (r planRetention) floor(plan string, visibleFrom, now time.Time) time.Time {
	floor := now.Add(-r.window(plan))
	if visibleFrom.After(floor) {
		return visibleFrom
	}
	return floor
}

// visibleFromAfterPlanChange is the floor a workspace keeps when its plan
// changes from oldPlan to newPlan at `at`. An upgrade keeps the old plan's
// floor at `at`: the longer window reaches back over it only as time passes,
// and the kept floor lapses on its own once the new window's floor overtakes
// it. A downgrade narrows at once, its window being shorter, and keeps an
// earlier upgrade's floor only while that still bites. Zero means the new
// plan's window alone applies.
func (r planRetention) visibleFromAfterPlanChange(oldPlan, newPlan string, visibleFrom, at time.Time) time.Time {
	floor := r.floor(oldPlan, visibleFrom, at)
	if !floor.After(at.Add(-r.window(newPlan))) {
		return time.Time{}
	}
	return floor
}

// firstUpgrade is the cheapest plan whose window is longer than plan's, and
// that window. ok is false on the longest window.
func (r planRetention) firstUpgrade(plan string) (next string, window time.Duration, ok bool) {
	longer := r.upgrades(plan)
	if len(longer) == 0 {
		return "", 0, false
	}
	return longer[0].Plan, longer[0].Window, true
}

// RetentionUpgrade is a plan that keeps more than another does, and its window.
type RetentionUpgrade struct {
	Plan   string
	Window time.Duration
}

// upgrades is every window longer than plan's, shortest first, each with the
// cheapest plan that has it: the plans the dashboard's retention hint names.
// Empty on the longest window.
func (r planRetention) upgrades(plan string) []RetentionUpgrade {
	have := r.window(plan)
	var out []RetentionUpgrade
	for _, p := range planLadder {
		if w := r.window(p); w > have {
			out = append(out, RetentionUpgrade{Plan: p, Window: w})
			have = w
		}
	}
	return out
}

// logRetention is how far back each plan's logs reach (w9/m389;
// render.com/docs/logging, checked 2026-11-14): Hobby 7 days, Pro 14, Scale
// and Enterprise 30. Every log read is floored at it, on every surface.
var logRetention = planRetention{
	planHobby:      7 * 24 * time.Hour,
	planPro:        14 * 24 * time.Hour,
	planScale:      30 * 24 * time.Hour,
	planEnterprise: 30 * 24 * time.Hour,
}

// metricsRetention is how far back each plan's metrics reach (w7/m427;
// render.com/docs/service-metrics § Metrics retention period, checked
// 2026-11-16): Hobby 7 days, Pro 14, Scale and Enterprise 30. Every metrics
// chart read is floored at it, on every surface (metrics.Service's
// withinRetention). The metrics store must hold the longest window:
// scripts/metrics-retention-guard.sh checks production's Prometheus against
// it, so change the two together.
var metricsRetention = planRetention{
	planHobby:      7 * 24 * time.Hour,
	planPro:        14 * 24 * time.Hour,
	planScale:      30 * 24 * time.Hour,
	planEnterprise: 30 * 24 * time.Hour,
}

// pitrRetention is how far back each plan may restore a Postgres to a point in
// time (w7/m453; render.com/docs/postgresql-backups, read 2026-11-16: "Past 3
// days" on Hobby, "Past 7 days" on Pro or higher; render.com/pricing's
// "Point-in-time recovery window: 3 days | 7 days | 7 days | 7 days"). Every
// recovery-info answer starts no earlier than it and every recovery before it
// is refused, on every surface. The tenant backup store must keep at least
// the longest window: a backend test reads its chart value against
// LongestPITRRetention, so change the two together.
var pitrRetention = planRetention{
	planHobby:      3 * 24 * time.Hour,
	planPro:        7 * 24 * time.Hour,
	planScale:      7 * 24 * time.Hour,
	planEnterprise: 7 * 24 * time.Hour,
}

// LogRetention is how far back plan's logs reach; an unknown plan gets the
// shortest window (ShortestLogRetention).
func LogRetention(plan string) time.Duration { return logRetention.window(plan) }

// ShortestLogRetention is the shortest plan window. No workspace's log floor
// is ever later than now minus it: LogFloor subtracts at least this much, and
// a floor a plan change kept lies at least this far before the change.
func ShortestLogRetention() time.Duration { return logRetention.shortest() }

// LogRetentionUpgrade is the cheapest plan that keeps logs longer than plan,
// and its window. ok is false on the longest window.
func LogRetentionUpgrade(plan string) (next string, window time.Duration, ok bool) {
	return logRetention.firstUpgrade(plan)
}

// PITRRetention is how far back plan may restore a Postgres to a point in
// time; an unknown plan gets the shortest window.
func PITRRetention(plan string) time.Duration { return pitrRetention.window(plan) }

// LongestPITRRetention is the longest plan window: the tenant backup store's
// retentionPolicy must keep at least this much.
func LongestPITRRetention() time.Duration { return pitrRetention.longest() }

// PITRRetentionUpgrade is the cheapest plan that may restore further back than
// plan, and its window. ok is false on the longest window.
func PITRRetentionUpgrade(plan string) (next string, window time.Duration, ok bool) {
	return pitrRetention.firstUpgrade(plan)
}

// PITRFloor is the earliest instant a workspace on plan may restore a Postgres
// to at now: its plan's window back from now, or visibleFrom while that is
// later. visibleFrom is the floor a plan change kept
// (PITRVisibleFromAfterPlanChange); zero means none.
func PITRFloor(plan string, visibleFrom, now time.Time) time.Time {
	return pitrRetention.floor(plan, visibleFrom, now)
}

// PITRVisibleFromAfterPlanChange is the recovery floor a workspace keeps when
// its plan changes from oldPlan to newPlan at `at` (w7/m453). Render:
// "Upgrading from Hobby does not retroactively "backfill" your recovery
// window. Instead, your existing 3-day window extends to 7 days going
// forward." So a Hobby -> Pro change at T keeps T-3d until Pro's own floor
// passes it four days later. A downgrade narrows at once (Render does not say;
// bex does what its log floor does). Zero means the new plan's window alone
// applies.
func PITRVisibleFromAfterPlanChange(oldPlan, newPlan string, visibleFrom, at time.Time) time.Time {
	return pitrRetention.visibleFromAfterPlanChange(oldPlan, newPlan, visibleFrom, at)
}

// MetricsRetention is how far back plan's metrics reach; an unknown plan gets
// the shortest window (ShortestMetricsRetention).
func MetricsRetention(plan string) time.Duration { return metricsRetention.window(plan) }

// ShortestMetricsRetention is the shortest plan window: a metrics read that
// starts no earlier than now minus it reaches past no workspace's floor.
func ShortestMetricsRetention() time.Duration { return metricsRetention.shortest() }

// MetricsRetentionUpgrades is every metrics window longer than plan's,
// shortest first, each with the cheapest plan that has it (Hobby: Pro's 14
// days, then Scale's 30). Empty on the longest window.
func MetricsRetentionUpgrades(plan string) []RetentionUpgrade {
	return metricsRetention.upgrades(plan)
}

// LogFloor is the earliest instant a workspace on plan sees logs from at now:
// its plan's window back from now, or visibleFrom while that is later.
// visibleFrom is the floor a plan change kept (LogsVisibleFromAfterPlanChange);
// zero means none.
func LogFloor(plan string, visibleFrom, now time.Time) time.Time {
	return logRetention.floor(plan, visibleFrom, now)
}

// LogsVisibleFromAfterPlanChange is the floor a workspace keeps when its plan
// changes from oldPlan to newPlan at `at` (w9/m389 t004). Render: "Logs older
// than your current retention period are no longer available, even if you
// upgrade your plan to extend the period." So an upgrade keeps the old plan's
// floor at `at`: the longer window reaches back over it only as time passes,
// and the kept floor lapses on its own once the new window's floor overtakes
// it. A downgrade narrows at once, its window being shorter, and keeps an
// earlier upgrade's floor only while that still bites. Zero means the new
// plan's window alone applies.
func LogsVisibleFromAfterPlanChange(oldPlan, newPlan string, visibleFrom, at time.Time) time.Time {
	return logRetention.visibleFromAfterPlanChange(oldPlan, newPlan, visibleFrom, at)
}

// WorkspaceLogPlanReader is a WorkspacePlanReader that also reads, in the
// same read, the log floor the workspace's last plan change kept
// (LogsVisibleFromAfterPlanChange; zero when none). The tenants store is one;
// a plain WorkspacePlanReader keeps no floor, so the plan's window alone
// applies.
type WorkspaceLogPlanReader interface {
	WorkspaceLogPlan(ctx context.Context, workspaceID string) (plan string, visibleFrom time.Time, err error)
}

// WorkspacePITRPlanReader is WorkspaceLogPlanReader for the Postgres recovery
// floor the workspace's last plan change kept (PITRVisibleFromAfterPlanChange;
// zero when none, w7/m453).
type WorkspacePITRPlanReader interface {
	WorkspacePITRPlan(ctx context.Context, workspaceID string) (plan string, visibleFrom time.Time, err error)
}

// WorkspaceLogPlanOf is WorkspacePlanOf plus the log floor the workspace's
// last plan change kept, in one read where the reader offers both
// (WorkspaceLogPlanReader). ok is false where no plan applies, as for
// WorkspacePlanOf.
func (b *Base) WorkspaceLogPlanOf(ctx context.Context, workspaceID string) (plan string, visibleFrom time.Time, ok bool, err error) {
	var read planFloorRead
	if r, isBoth := b.planReader().(WorkspaceLogPlanReader); isBoth {
		read = r.WorkspaceLogPlan
	}
	return b.workspacePlanFloorOf(ctx, workspaceID, read)
}

// WorkspacePITRPlanOf is WorkspaceLogPlanOf for the Postgres recovery floor
// (WorkspacePITRPlanReader).
func (b *Base) WorkspacePITRPlanOf(ctx context.Context, workspaceID string) (plan string, visibleFrom time.Time, ok bool, err error) {
	var read planFloorRead
	if r, isBoth := b.planReader().(WorkspacePITRPlanReader); isBoth {
		read = r.WorkspacePITRPlan
	}
	return b.workspacePlanFloorOf(ctx, workspaceID, read)
}

// planFloorRead reads a workspace's plan and one floor its last plan change
// kept.
type planFloorRead func(ctx context.Context, workspaceID string) (plan string, visibleFrom time.Time, err error)

// workspacePlanFloorOf reads workspaceID's plan and floor with read in one
// read, or the plan alone where the reader keeps no floor (read nil). ok is
// false where no plan applies, as for WorkspacePlanOf.
func (b *Base) workspacePlanFloorOf(ctx context.Context, workspaceID string, read planFloorRead) (plan string, visibleFrom time.Time, ok bool, err error) {
	if read == nil || workspaceID == "" || workspaceID == DefaultTenant {
		plan, ok, err = b.WorkspacePlanOf(ctx, workspaceID)
		return plan, time.Time{}, ok, err
	}
	plan, visibleFrom, err = read(ctx, workspaceID)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("reading workspace plan: %w", err)
	}
	return plan, visibleFrom, true, nil
}

// planReader is b's plan reader, nil-safe on a nil Base.
func (b *Base) planReader() WorkspacePlanReader {
	if b == nil {
		return nil
	}
	return b.Plans
}

// WorkspacePlanReader reads a workspace's plan (hobby, pro, scale, enterprise).
// The composition root wires it from the tenants store; nil leaves every
// plan-gated feature open, the store-off degrade the other Base gates use.
type WorkspacePlanReader interface {
	WorkspacePlan(ctx context.Context, workspaceID string) (string, error)
}

// WorkspacePlanFunc adapts a function to WorkspacePlanReader.
type WorkspacePlanFunc func(ctx context.Context, workspaceID string) (string, error)

func (f WorkspacePlanFunc) WorkspacePlan(ctx context.Context, workspaceID string) (string, error) {
	return f(ctx, workspaceID)
}

// InboundIPRulesMessage is the one sentence every surface uses when a workspace
// below Scale sets inbound IP rules on a service, static site or environment
// (w8/m137). Postgres and Key Value rules are available on every plan.
const InboundIPRulesMessage = "inbound IP rules for web services, static sites and environments require the Scale plan or higher"

// InboundIPRulesPlan reports whether a workspace plan may set inbound IP rules
// on services, static sites and environments (Render: Scale and Enterprise).
func InboundIPRulesPlan(plan string) bool {
	return PlanAllows(plan, FeatureInboundIPRules)
}

// RestrictsInbound reports whether entries restrict who may connect: anything
// beyond the allow-all pair. An empty list restricts most of all: it denies
// every source, on an environment (w4/m28) and on a service (w8/m157) alike.
func RestrictsInbound(entries []IPAllowListEntry) bool {
	if len(entries) == 0 {
		return true
	}
	return slices.ContainsFunc(entries, func(e IPAllowListEntry) bool {
		return e.CIDRBlock != "0.0.0.0/0" && e.CIDRBlock != "::/0"
	})
}

// RestrictsServiceInbound is RestrictsInbound for a web service or static
// site, whose only reachable family is IPv4 (no service host publishes an AAAA
// record): a list that admits no IPv4 source, ::/0 alone, blocks every client
// (w8/284).
func RestrictsServiceInbound(entries []IPAllowListEntry) bool {
	return RestrictsInbound(entries) || !slices.ContainsFunc(entries, func(e IPAllowListEntry) bool { return e.CIDRBlock == "0.0.0.0/0" })
}

// RequireInboundIPRules refuses next below the Scale plan when it restricts
// inbound traffic and differs from current. A rule that already exists keeps
// being enforced and can always be cleared or re-sent unchanged, so a client
// that round-trips a whole settings object never fails on a legacy rule.
func (b *Base) RequireInboundIPRules(ctx context.Context, workspaceID string, next, current []IPAllowListEntry) error {
	return b.requireInboundIPRules(ctx, workspaceID, RestrictsInbound(next) && !slices.Equal(next, current))
}

// RequireServiceInboundIPRules is RequireInboundIPRules for a web service or
// static site, judged by RestrictsServiceInbound.
func (b *Base) RequireServiceInboundIPRules(ctx context.Context, workspaceID string, next, current []IPAllowListEntry) error {
	return b.requireInboundIPRules(ctx, workspaceID, RestrictsServiceInbound(next) && !slices.Equal(next, current))
}

func (b *Base) requireInboundIPRules(ctx context.Context, workspaceID string, restricts bool) error {
	if !restricts {
		return nil
	}
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || InboundIPRulesPlan(plan) {
		return err
	}
	return fmt.Errorf("%w: ipAllowList: %s", ErrBadRequest, InboundIPRulesMessage)
}

// WorkspacePlanOf reads workspaceID's plan. ok is false, with no error, where no
// plan applies (no reader wired, no workspace, the default tenant), so callers
// leave the feature open exactly as the other Base gates degrade.
func (b *Base) WorkspacePlanOf(ctx context.Context, workspaceID string) (plan string, ok bool, err error) {
	if b == nil || b.Plans == nil || workspaceID == "" || workspaceID == DefaultTenant {
		return "", false, nil
	}
	plan, err = b.Plans.WorkspacePlan(ctx, workspaceID)
	if err != nil {
		return "", false, fmt.Errorf("reading workspace plan: %w", err)
	}
	return plan, true, nil
}

// RequirePlanFeature refuses a workspace whose plan does not include feature.
func (b *Base) RequirePlanFeature(ctx context.Context, workspaceID string, feature PlanFeature) error {
	return b.RequirePlanFeatureAt(ctx, workspaceID, "", feature)
}

// RequirePlanFeatureAt is RequirePlanFeature with the refusal naming field.
func (b *Base) RequirePlanFeatureAt(ctx context.Context, workspaceID, field string, feature PlanFeature) error {
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || PlanAllows(plan, feature) {
		return err
	}
	return NewPlanFeatureError(field, plan, feature)
}

// RequireWorkspaceInboundIPRules refuses a workspace inbound IP rule change
// below the Scale plan (w9/m388; Render: Scale and Enterprise only) with the
// coded refusal every plan-gated feature uses: PLAN_FEATURE_REQUIRED,
// params.feature "inboundIPRules". As for environment rules (w8/m137), a
// change that restricts nothing — the allow-all pair — and re-sending the
// stored list are always accepted, so a downgraded workspace keeps enforcing
// what it saved and can always reset to allow-all, but cannot set new
// restrictions.
func (b *Base) RequireWorkspaceInboundIPRules(ctx context.Context, workspaceID string, next, current []IPAllowListEntry) error {
	if !RestrictsInbound(next) || slices.Equal(next, current) {
		return nil
	}
	return b.RequirePlanFeatureAt(ctx, workspaceID, "ipAllowList", FeatureInboundIPRules)
}

// RequireNetworkIsolation refuses turning environment network isolation on
// below Pro. Leaving it off, or re-sending the state it already has, is always
// accepted, so a client that round-trips a whole ACL never trips on a legacy
// isolated environment.
func (b *Base) RequireNetworkIsolation(ctx context.Context, workspaceID string, next, current bool) error {
	if !next || current {
		return nil
	}
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return err
	}
	return NetworkIsolationRefusal(plan, next, current)
}

// NetworkIsolationRefusal is RequireNetworkIsolation for a plan already read:
// nil unless next turns isolation on below Pro.
func NetworkIsolationRefusal(plan string, next, current bool) error {
	if !next || current || PlanAllows(plan, FeatureNetworkIsolation) {
		return nil
	}
	return NewPlanFeatureError("networkIsolationEnabled", plan, FeatureNetworkIsolation)
}

// RequireEnvironmentCapacity refuses a project's next environment once the
// workspace plan's per-project cap (Hobby: two) is reached. existing is the
// project's current environment count.
func (b *Base) RequireEnvironmentCapacity(ctx context.Context, workspaceID string, existing int) error {
	plan, ok, err := b.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return err
	}
	return EnvironmentCapacityRefusal(plan, existing)
}

// EnvironmentCapacityRefusal is RequireEnvironmentCapacity for a plan already
// read: nil while a project with existing environments may add another.
func EnvironmentCapacityRefusal(plan string, existing int) error {
	if limit := MaxEnvironmentsPerProject(plan); limit > 0 && existing >= limit {
		return NewBadRequestError(PlanLimitCode,
			fmt.Sprintf("%s: environments: a %s workspace is limited to %d environments per project; upgrade the plan to add more",
				ErrBadRequest, strings.ToUpper(plan[:1])+plan[1:], limit),
			map[string]any{"plan": plan, "limit": limit, "field": "environments"})
	}
	return nil
}
