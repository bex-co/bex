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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ServicePatch is the neutral, presence-aware value behind the two
// patch-shaped service surfaces — REST's PATCH /v1/services/{id}
// (rest.go patchService) and MCP's update_service (mcp.go applyServicePatch).
// Each adapter reduces to a wire→ServicePatch fill; ApplyServicePatch below
// holds the ONE ordered op table both share, so a new PATCH-able setting can
// no longer land on one surface and silently miss the other (w1/m78 — the
// "must stay identical" contract three comments used to assert is now
// structural).
//
// Every field follows the patch-pointer convention: nil (or false, for the
// one flag) means "not supplied — leave unchanged"; a present pointer writes
// exactly its value, including the empty value, which is how a caller clears
// a field.
//
// Two settings are deliberately single-surface — routing that matches Render,
// not accidental drift (w1/073):
//
//   - ImageOwnerID — REST-only because it validates Render's nested image
//     owner object; MCP's request-scoped workspace already supplies the owner.
//   - NotificationsToSend, Autoscaling — MCP-only convenience folds; REST
//     keeps Render's dedicated routes (PATCH …/notification-settings/
//     overrides/services/{id} and PUT …/autoscaling).
//
// MaintenanceBeforeFreeDowngrade is armed on BOTH fills: a simultaneous
// disable-maintenance + free downgrade must apply maintenance first, or
// SetPlan refuses the paid-feature validation. The flag is not a surface
// difference; it only exists so the table row has a field to own.
type ServicePatch struct {
	DisplayName *string
	// ImageOwnerID is REST-only; Repo/Image/Branch are shared with MCP.
	Repo                 *string
	Image                *string
	ImageOwnerID         *string
	Branch               *string
	RegistryCredentialID *string
	// MaintenanceBeforeFreeDowngrade arms the reorder rule: a simultaneous
	// "disable maintenance + downgrade to free" applies the maintenance
	// write BEFORE the plan write. The rule's CONDITION lives in the op
	// table (maintenanceBeforePlan); both REST and MCP fills set this true
	// (w1/073) so a multi-field update_service cannot hit SetPlan's
	// paid-feature refusal.
	MaintenanceBeforeFreeDowngrade bool
	MaintenanceMode                *MaintenanceModeView
	Plan                           *string
	IdleTTLSeconds                 *int32
	MaxShutdownDelaySeconds        *int32
	RootDir                        *string
	BuildFilter                    *BuildFilterView
	AutoDeploy                     *bool
	Schedule                       *string
	Command                        *string
	HealthCheckPath                *string
	PreDeployCommand               *string
	PublishPath                    *string
	BuildCommand                   *string
	StartCommand                   *string
	DockerfilePath                 *string
	// Port is the listening port (w4/m121). Before it, the port was
	// create-only on every surface, so bex's own PORT refusal — "change the
	// service port instead" — named a setting no caller could reach.
	Port         *int32
	NotifyOnFail *string
	// NotificationsToSend: MCP-only today (divergence — see type comment).
	NotificationsToSend   *string
	RenderSubdomainPolicy *string
	// IPAllowList: nil = not provided (leave unchanged); non-nil = replace,
	// including the empty list (clear).
	IPAllowList *[]core.IPAllowListEntry
	// Autoscaling: MCP-only today (divergence — see type comment).
	Autoscaling *SetAutoscalingRequest
}

// maintenanceBeforePlan reports whether the maintenance write must run BEFORE
// the plan write: a simultaneous downgrade to free must disable maintenance
// first; every other combination applies the plan first so validation sees
// the final plan. Both REST and MCP fills arm MaintenanceBeforeFreeDowngrade
// (w1/073); the flag is how the table row is owned, not a per-surface switch.
func (p ServicePatch) maintenanceBeforePlan() bool {
	return p.MaintenanceBeforeFreeDowngrade &&
		p.MaintenanceMode != nil && !p.MaintenanceMode.Enabled &&
		p.Plan != nil && *p.Plan == "free"
}

// servicePatchOp is one row of the ordered service-patch table: the
// ServicePatch fields the row owns (bookkeeping for the completeness guard —
// every ServicePatch field must be owned by exactly one row, so a field added
// to the type cannot be forgotten in the table), the presence test that
// queues it, the permission its verb requires, the read-only preflight that
// decides whether its verb would be refused, and the Service verb it runs.
type servicePatchOp struct {
	fields  []string
	present func(p ServicePatch) bool
	// relation is the OpenFGA relation the row's verb authorizes on. The
	// preflight checks it up front so a patch whose LAST row the caller may
	// not perform cannot apply its earlier rows first (w9/m166).
	relation func(p ServicePatch) string
	// check runs the row's post-authorization validation and policy guards
	// against probe — a scratch App carrying the state this row will actually
	// see, i.e. the current App with every EARLIER row's spec change already
	// folded in. It must not write; instead it folds its own spec change into
	// probe so the rows after it validate against the proposed combined state
	// (a free downgrade sees the maintenance flip queued ahead of it; a build
	// setting sees the source this same patch is repointing).
	//
	// Only the fields some later row's check READS are projected — tier,
	// maintenance mode and the source quartet. Projecting the rest would be
	// dead weight, since nothing downstream consults it.
	//
	// A row whose verb validates nothing beyond authorization leaves this nil.
	check func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error
	apply func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error)
}

// canOperate/canCreate are the two fixed relations most rows use; only the
// cron row picks its relation from the patch.
func canOperate(ServicePatch) string { return core.RelCanOperate }
func canCreate(ServicePatch) string  { return core.RelCanCreate }

// servicePatchTable is THE ordered op table behind PATCH /v1/services/{id}
// and update_service. The order is REST's application order — the order MCP's
// tool always declared canonical — and is pinned by
// TestServicePatchTableOrderIsRESTApplicationOrder; changing it changes what
// a multi-field patch does on BOTH surfaces at once.
var servicePatchTable = []servicePatchOp{
	{
		fields:   []string{"DisplayName"},
		present:  func(p ServicePatch) bool { return p.DisplayName != nil },
		relation: canOperate,
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetDisplayName(ctx, id, *p.DisplayName)
		},
	},
	{
		// One combined source write: the registry credential is validated
		// against the proposed image host before either reaches the App.
		// ImageOwnerID rides along with Image and is never a trigger by
		// itself (REST fills it only when the image object is present).
		fields: []string{"Repo", "Image", "ImageOwnerID", "Branch", "RegistryCredentialID"},
		present: func(p ServicePatch) bool {
			return p.Repo != nil || p.Image != nil || p.Branch != nil || p.RegistryCredentialID != nil
		},
		relation: canCreate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			next, changed, err := s.checkSourcePatch(ctx, probe, servicePatchSource(p))
			if err != nil {
				return err
			}
			if !changed {
				return nil
			}
			// The build settings below (root dir, Dockerfile path, build
			// filter) refuse on a service with no repo, so they must see the
			// source this patch is moving them to, not the one it is leaving.
			next.applyTo(probe)
			return nil
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetSourceAndRegistryCredential(ctx, id, servicePatchSource(p))
		},
	},
	{
		// The maintenance-before-plan reorder as DATA: when armed and the
		// patch is a simultaneous free downgrade, the maintenance write
		// runs here, before the plan write, instead of at its late row
		// below. Exactly one of the two maintenance rows ever queues.
		fields:   []string{"MaintenanceBeforeFreeDowngrade"},
		present:  ServicePatch.maintenanceBeforePlan,
		relation: canOperate,
		check:    checkMaintenanceRow,
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.ConfigureMaintenanceMode(ctx, id, *p.MaintenanceMode)
		},
	},
	{
		fields:   []string{"Plan"},
		present:  func(p ServicePatch) bool { return p.Plan != nil },
		relation: canOperate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			tier, err := s.checkPlan(ctx, probe, *p.Plan)
			if err != nil {
				return err
			}
			// Maintenance eligibility and autoscaling targets are both
			// tier-dependent, so a later row must see the plan this patch
			// is moving to.
			probe.Spec.Tier = tier
			return nil
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPlan(ctx, id, *p.Plan)
		},
	},
	{
		fields:   []string{"IdleTTLSeconds"},
		present:  func(p ServicePatch) bool { return p.IdleTTLSeconds != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, _ *appv1alpha1.App, _ string, p ServicePatch) error {
			return checkIdleTTL(*p.IdleTTLSeconds)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetIdleTTL(ctx, id, *p.IdleTTLSeconds)
		},
	},
	{
		fields:   []string{"MaxShutdownDelaySeconds"},
		present:  func(p ServicePatch) bool { return p.MaxShutdownDelaySeconds != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			return validateMaxShutdownDelaySeconds(probe.Spec.Type, p.MaxShutdownDelaySeconds)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetMaxShutdownDelay(ctx, id, *p.MaxShutdownDelaySeconds)
		},
	},
	{
		fields:   []string{"RootDir"},
		present:  func(p ServicePatch) bool { return p.RootDir != nil },
		relation: canCreate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			return s.checkRootDir(ctx, probe, id, *p.RootDir)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetRootDir(ctx, id, *p.RootDir)
		},
	},
	{
		fields:   []string{"BuildFilter"},
		present:  func(p ServicePatch) bool { return p.BuildFilter != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			_, err := checkBuildFilter(probe, id, p.BuildFilter)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetBuildFilter(ctx, id, p.BuildFilter)
		},
	},
	{
		fields:   []string{"AutoDeploy"},
		present:  func(p ServicePatch) bool { return p.AutoDeploy != nil },
		relation: canOperate,
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetAutoDeploy(ctx, id, *p.AutoDeploy)
		},
	},
	{
		// Cron schedule/command share one verb: sending only one leaves the
		// other unchanged.
		fields:  []string{"Schedule", "Command"},
		present: func(p ServicePatch) bool { return p.Schedule != nil || p.Command != nil },
		// Supplying a command changes what the cron RUNS — create-like;
		// rescheduling when it runs stays lifecycle (SetCronJob's split).
		relation: func(p ServicePatch) string { return core.LifecycleOrCreate(p.Command != nil) },
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			return s.checkCronJob(ctx, probe, id, p.Schedule, p.Command)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetCronJob(ctx, id, p.Schedule, p.Command)
		},
	},
	{
		fields:   []string{"HealthCheckPath"},
		present:  func(p ServicePatch) bool { return p.HealthCheckPath != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			_, err := checkHealthCheckPath(probe, *p.HealthCheckPath)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetHealthCheckPath(ctx, id, *p.HealthCheckPath)
		},
	},
	{
		fields:   []string{"PreDeployCommand"},
		present:  func(p ServicePatch) bool { return p.PreDeployCommand != nil },
		relation: canCreate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, _ string, _ ServicePatch) error {
			return s.checkPreDeployCommand(ctx, probe)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPreDeployCommand(ctx, id, *p.PreDeployCommand)
		},
	},
	{
		fields:   []string{"PublishPath"},
		present:  func(p ServicePatch) bool { return p.PublishPath != nil },
		relation: canCreate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			return checkPublishPath(probe, id, *p.PublishPath)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPublishPath(ctx, id, *p.PublishPath)
		},
	},
	{
		// One SetCommands call for both: setting only one leaves the other
		// unchanged (nil), which is why the setter pair could fold without
		// either clearing the other.
		fields:   []string{"BuildCommand", "StartCommand"},
		present:  func(p ServicePatch) bool { return p.BuildCommand != nil || p.StartCommand != nil },
		relation: canCreate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			return s.checkCommands(ctx, probe, p.StartCommand)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetCommands(ctx, id, p.BuildCommand, p.StartCommand)
		},
	},
	{
		fields:   []string{"DockerfilePath"},
		present:  func(p ServicePatch) bool { return p.DockerfilePath != nil },
		relation: canCreate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, id string, p ServicePatch) error {
			_, err := s.checkDockerfilePath(ctx, probe, id, *p.DockerfilePath)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetDockerfilePath(ctx, id, *p.DockerfilePath)
		},
	},
	{
		fields:   []string{"Port"},
		present:  func(p ServicePatch) bool { return p.Port != nil },
		relation: canCreate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			return checkPort(probe, *p.Port)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPort(ctx, id, *p.Port)
		},
	},
	{
		fields:   []string{"NotifyOnFail"},
		present:  func(p ServicePatch) bool { return p.NotifyOnFail != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, _ *appv1alpha1.App, _ string, p ServicePatch) error {
			_, err := normalizeNotifyOnFail(*p.NotifyOnFail)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetNotifyOnFail(ctx, id, *p.NotifyOnFail)
		},
	},
	{
		fields:   []string{"NotificationsToSend"},
		present:  func(p ServicePatch) bool { return p.NotificationsToSend != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, _ *appv1alpha1.App, _ string, p ServicePatch) error {
			_, err := normalizeNotificationsToSend(*p.NotificationsToSend)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetNotificationsToSend(ctx, id, *p.NotificationsToSend)
		},
	},
	{
		fields:   []string{"RenderSubdomainPolicy"},
		present:  func(p ServicePatch) bool { return p.RenderSubdomainPolicy != nil },
		relation: canOperate,
		check: func(ctx context.Context, s *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			_, err := s.checkSubdomainPolicy(ctx, probe, *p.RenderSubdomainPolicy)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetSubdomainPolicy(ctx, id, *p.RenderSubdomainPolicy)
		},
	},
	{
		fields:   []string{"IPAllowList"},
		present:  func(p ServicePatch) bool { return p.IPAllowList != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, _ *appv1alpha1.App, _ string, p ServicePatch) error {
			return core.ValidateAllowList(*p.IPAllowList)
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetIPAllowList(ctx, id, *p.IPAllowList)
		},
	},
	{
		// The late (normal) maintenance position — skipped exactly when the
		// armed reorder row above already queued the write.
		fields: []string{"MaintenanceMode"},
		present: func(p ServicePatch) bool {
			return p.MaintenanceMode != nil && !p.maintenanceBeforePlan()
		},
		relation: canOperate,
		check:    checkMaintenanceRow,
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.ConfigureMaintenanceMode(ctx, id, *p.MaintenanceMode)
		},
	},
	{
		// Autoscaling is a subresource with its own view; the patch answers
		// with the service, so re-read it after the write (get_autoscaling
		// still serves the autoscaling view, and disable_autoscaling still
		// turns it off).
		fields:   []string{"Autoscaling"},
		present:  func(p ServicePatch) bool { return p.Autoscaling != nil },
		relation: canOperate,
		check: func(_ context.Context, _ *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
			_, err := checkAutoscaling(probe, *p.Autoscaling)
			return err
		},
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			if _, err := s.SetAutoscaling(ctx, id, *p.Autoscaling); err != nil {
				return AppView{}, err
			}
			return s.Get(ctx, id)
		},
	},
}

// servicePatchSource is the source quartet (plus its REST-only owner id) the
// source row hands to both its check and its verb, so the preflight validates
// exactly the request the write will make.
func servicePatchSource(p ServicePatch) sourcePatch {
	return sourcePatch{
		Repo:                 p.Repo,
		Image:                p.Image,
		Branch:               p.Branch,
		RegistryCredentialID: p.RegistryCredentialID,
		ImageOwnerID:         p.ImageOwnerID,
	}
}

// checkMaintenanceRow is shared by the two maintenance rows — the armed
// before-plan position and the normal late one. Exactly one of them ever
// queues, so this runs at most once per patch.
func checkMaintenanceRow(ctx context.Context, s *Service, probe *appv1alpha1.App, _ string, p ServicePatch) error {
	in, err := s.checkMaintenanceMode(ctx, probe, *p.MaintenanceMode)
	if err != nil {
		return err
	}
	// SetPlan refuses a free downgrade while maintenance is enabled, so a
	// patch that disables maintenance in the same request must let the plan
	// row see the disable that is queued ahead of it.
	probe.Spec.MaintenanceMode = maintenanceModeSpec(in)
	return nil
}

// preflightServicePatch answers "would every present row be allowed and
// accepted?" without writing anything (w9/m166).
//
// It exists because the op table below applies each present field through its
// own setter, and each setter authorized and validated only its own field at
// the moment it ran. A patch carrying a valid field ahead of an invalid one
// therefore persisted the valid one — and, for a build-relevant field, rolled
// the service — and then returned the second field's 400. The caller saw a
// failed command and a changed, redeployed service, which makes the PATCH
// unusable for safe automation (RFC 5789 §2 asks for the opposite).
//
// Two passes, in table order:
//
//   - Authorization, once per DISTINCT relation the present rows need. Row
//     order is preserved across the distinct set, so the error a mixed-relation
//     patch returns is the one the old sequential application returned. The
//     audit context is the deferred one, so an allowed preflight records
//     nothing and each row's own verb still records its own effect; a DENIED
//     preflight records the denial exactly once and stops here.
//   - Each row's check against a scratch App that accumulates the earlier
//     rows' proposed spec changes, so coupled settings (disable maintenance
//     then downgrade to free; repoint the source then set a build filter)
//     validate against the combined state the request asks for rather than
//     against the state on disk.
//
// This closes the deterministic refusals — validation, service kind, plan
// policy, protected environment, permission. It is NOT a distributed
// transaction: an unexpected store or Kubernetes failure PARTWAY THROUGH the
// apply pass below can still leave a patch half-written, and those errors stay
// truthful errors. Making that boundary atomic too would need a cross-store
// rollback this deliberately does not invent (w4/123, w4/m130 own the
// mid-persistence failure cases).
// It returns the context the apply pass must use: the caller's, carrying the
// source the source row already probed so the apply pass does not repeat those
// outbound calls.
func (s *Service) preflightServicePatch(ctx context.Context, id string, p ServicePatch) (context.Context, error) {
	rows := presentServicePatchRows(p)
	if len(rows) == 0 {
		// No present field: ApplyServicePatch is a read-only no-op that
		// authorizes can_view through Get. Nothing to preflight.
		return ctx, nil
	}
	auditCtx := core.WithDeferredAllowedWriteAudit(ctx)
	var probe *appv1alpha1.App
	seen := make(map[string]bool, 2)
	for _, row := range rows {
		relation := row.relation(p)
		if seen[relation] {
			continue
		}
		seen[relation] = true
		a, err := s.AuthorizeApp(auditCtx, relation, id)
		if err != nil {
			return ctx, err
		}
		if probe == nil {
			probe = a.DeepCopy()
		}
	}
	for _, row := range rows {
		if row.check == nil {
			continue
		}
		if err := row.check(ctx, s, probe, id, p); err != nil {
			return ctx, err
		}
	}
	// The source row's check folded the source it probed onto probe, so the
	// probe now carries exactly what the apply pass will resolve to. Publishing
	// it unconditionally is safe because the reuse is gated on value equality:
	// a patch with no source row has no reader, and one with a source row
	// resolves to precisely this value.
	return withProbedSource(ctx, sourceFields{
		repo:                 probe.Spec.Repo,
		image:                probe.Spec.Image,
		branch:               probe.Spec.Branch,
		registryCredentialID: clonePtr(probe.Spec.RegistryCredentialID),
	}), nil
}

// ApplyServicePatch runs the present fields of p against service id as an
// ordered list of the same Service verbs the two patch adapters always
// called, per servicePatchTable — one table, so REST and MCP cannot drift. A
// patch with no present field is a read-only no-op that reflects current
// state (core.PatchOps.Run's contract), exactly as both surfaces documented;
// the first failing op stops the chain, so a rejected value never reports
// success. Authorization stays where it always was: each queued verb starts
// with its own AuthorizeApp.
//
// The preflight below is what makes a rejected multi-field patch leave the
// service untouched; see preflightServicePatch for the boundary it does and
// does not cover.
func (s *Service) ApplyServicePatch(ctx context.Context, id string, p ServicePatch) (AppView, error) {
	// Validating twice means asking the same request-invariant questions twice
	// — and, within the preflight alone, asking some of them once per guarded
	// row. Memoize them for this request so the guarantee costs round trips
	// proportional to the patch, not to the number of rows in it.
	ctx = withRequestMemo(ctx)
	ctx, err := s.preflightServicePatch(ctx, id, p)
	if err != nil {
		return AppView{}, err
	}
	// One PATCH is one rollout. The table below applies each present field as
	// its own setter, and every build-relevant setter opens a deploy row
	// (w6/m51) — so without this a four-field save would read back as four
	// deploys, three of them immediately canceled. Deferred rather than run on
	// success only: a table that fails partway has still rolled the service for
	// the fields it did apply, and that rollout is still owed its row.
	ctx, flushRollout := rollout.Batch(ctx)
	defer flushRollout()
	var ops core.PatchOps[AppView]
	// The patch answers with the LAST row's view, so the allowlist row's
	// proxied-domain warning (w1/m171) would be lost behind any later row;
	// carry it to the final view.
	var proxied []string
	for _, row := range presentServicePatchRows(p) {
		ops.Add(true, func() (AppView, error) {
			v, err := row.apply(ctx, s, id, p)
			if v.IPAllowListProxiedDomains != nil {
				proxied = v.IPAllowListProxiedDomains
			}
			return v, err
		})
	}
	v, err := ops.Run(func() (AppView, error) { return s.Get(ctx, id) })
	if err == nil && v.IPAllowListProxiedDomains == nil {
		v.IPAllowListProxiedDomains = proxied
	}
	return v, err
}

// presentServicePatchRows is the rows p actually asks for, in table order.
// Both passes go through it so they cannot disagree about which rows the patch
// contains — a preflight that checked a different set from the one applied
// would be no preflight at all.
func presentServicePatchRows(p ServicePatch) []servicePatchOp {
	rows := make([]servicePatchOp, 0, len(servicePatchTable))
	for _, row := range servicePatchTable {
		if row.present(p) {
			rows = append(rows, row)
		}
	}
	return rows
}
