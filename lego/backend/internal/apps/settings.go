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
// Three fields are deliberately single-surface — routing that matches Render,
// not accidental drift (w1/073):
//
//   - Repo, Image, ImageOwnerID — REST-only (Render's PATCH source object);
//     update_service has no repo/image argument (source-kind switches stay
//     on REST; MCP still carries branch + registryCredentialId).
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
	// Repo/Image/ImageOwnerID: REST-only today (divergence — see type comment).
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
	NotifyOnFail                   *string
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
// queues it, and the Service verb it runs.
type servicePatchOp struct {
	fields  []string
	present func(p ServicePatch) bool
	apply   func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error)
}

// servicePatchTable is THE ordered op table behind PATCH /v1/services/{id}
// and update_service. The order is REST's application order — the order MCP's
// tool always declared canonical — and is pinned by
// TestServicePatchTableOrderIsRESTApplicationOrder; changing it changes what
// a multi-field patch does on BOTH surfaces at once.
var servicePatchTable = []servicePatchOp{
	{
		fields:  []string{"DisplayName"},
		present: func(p ServicePatch) bool { return p.DisplayName != nil },
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
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetSourceAndRegistryCredential(ctx, id, sourcePatch{
				Repo:                 p.Repo,
				Image:                p.Image,
				Branch:               p.Branch,
				RegistryCredentialID: p.RegistryCredentialID,
				ImageOwnerID:         p.ImageOwnerID,
			})
		},
	},
	{
		// The maintenance-before-plan reorder as DATA: when armed and the
		// patch is a simultaneous free downgrade, the maintenance write
		// runs here, before the plan write, instead of at its late row
		// below. Exactly one of the two maintenance rows ever queues.
		fields:  []string{"MaintenanceBeforeFreeDowngrade"},
		present: ServicePatch.maintenanceBeforePlan,
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.ConfigureMaintenanceMode(ctx, id, *p.MaintenanceMode)
		},
	},
	{
		fields:  []string{"Plan"},
		present: func(p ServicePatch) bool { return p.Plan != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPlan(ctx, id, *p.Plan)
		},
	},
	{
		fields:  []string{"IdleTTLSeconds"},
		present: func(p ServicePatch) bool { return p.IdleTTLSeconds != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetIdleTTL(ctx, id, *p.IdleTTLSeconds)
		},
	},
	{
		fields:  []string{"MaxShutdownDelaySeconds"},
		present: func(p ServicePatch) bool { return p.MaxShutdownDelaySeconds != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetMaxShutdownDelay(ctx, id, *p.MaxShutdownDelaySeconds)
		},
	},
	{
		fields:  []string{"RootDir"},
		present: func(p ServicePatch) bool { return p.RootDir != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetRootDir(ctx, id, *p.RootDir)
		},
	},
	{
		fields:  []string{"BuildFilter"},
		present: func(p ServicePatch) bool { return p.BuildFilter != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetBuildFilter(ctx, id, p.BuildFilter)
		},
	},
	{
		fields:  []string{"AutoDeploy"},
		present: func(p ServicePatch) bool { return p.AutoDeploy != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetAutoDeploy(ctx, id, *p.AutoDeploy)
		},
	},
	{
		// Cron schedule/command share one verb: sending only one leaves the
		// other unchanged.
		fields:  []string{"Schedule", "Command"},
		present: func(p ServicePatch) bool { return p.Schedule != nil || p.Command != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetCronJob(ctx, id, p.Schedule, p.Command)
		},
	},
	{
		fields:  []string{"HealthCheckPath"},
		present: func(p ServicePatch) bool { return p.HealthCheckPath != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetHealthCheckPath(ctx, id, *p.HealthCheckPath)
		},
	},
	{
		fields:  []string{"PreDeployCommand"},
		present: func(p ServicePatch) bool { return p.PreDeployCommand != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPreDeployCommand(ctx, id, *p.PreDeployCommand)
		},
	},
	{
		fields:  []string{"PublishPath"},
		present: func(p ServicePatch) bool { return p.PublishPath != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetPublishPath(ctx, id, *p.PublishPath)
		},
	},
	{
		// One SetCommands call for both: setting only one leaves the other
		// unchanged (nil), which is why the setter pair could fold without
		// either clearing the other.
		fields:  []string{"BuildCommand", "StartCommand"},
		present: func(p ServicePatch) bool { return p.BuildCommand != nil || p.StartCommand != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetCommands(ctx, id, p.BuildCommand, p.StartCommand)
		},
	},
	{
		fields:  []string{"DockerfilePath"},
		present: func(p ServicePatch) bool { return p.DockerfilePath != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetDockerfilePath(ctx, id, *p.DockerfilePath)
		},
	},
	{
		fields:  []string{"NotifyOnFail"},
		present: func(p ServicePatch) bool { return p.NotifyOnFail != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetNotifyOnFail(ctx, id, *p.NotifyOnFail)
		},
	},
	{
		fields:  []string{"NotificationsToSend"},
		present: func(p ServicePatch) bool { return p.NotificationsToSend != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetNotificationsToSend(ctx, id, *p.NotificationsToSend)
		},
	},
	{
		fields:  []string{"RenderSubdomainPolicy"},
		present: func(p ServicePatch) bool { return p.RenderSubdomainPolicy != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.SetSubdomainPolicy(ctx, id, *p.RenderSubdomainPolicy)
		},
	},
	{
		fields:  []string{"IPAllowList"},
		present: func(p ServicePatch) bool { return p.IPAllowList != nil },
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
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			return s.ConfigureMaintenanceMode(ctx, id, *p.MaintenanceMode)
		},
	},
	{
		// Autoscaling is a subresource with its own view; the patch answers
		// with the service, so re-read it after the write (get_autoscaling
		// still serves the autoscaling view, and disable_autoscaling still
		// turns it off).
		fields:  []string{"Autoscaling"},
		present: func(p ServicePatch) bool { return p.Autoscaling != nil },
		apply: func(ctx context.Context, s *Service, id string, p ServicePatch) (AppView, error) {
			if _, err := s.SetAutoscaling(ctx, id, *p.Autoscaling); err != nil {
				return AppView{}, err
			}
			return s.Get(ctx, id)
		},
	},
}

// ApplyServicePatch runs the present fields of p against service id as an
// ordered list of the same Service verbs the two patch adapters always
// called, per servicePatchTable — one table, so REST and MCP cannot drift. A
// patch with no present field is a read-only no-op that reflects current
// state (core.PatchOps.Run's contract), exactly as both surfaces documented;
// the first failing op stops the chain, so a rejected value never reports
// success. Authorization stays where it always was: each queued verb starts
// with its own AuthorizeApp.
func (s *Service) ApplyServicePatch(ctx context.Context, id string, p ServicePatch) (AppView, error) {
	// One PATCH is one rollout. The table below applies each present field as
	// its own setter, and every build-relevant setter opens a deploy row
	// (w6/m51) — so without this a four-field save would read back as four
	// deploys, three of them immediately canceled. Deferred rather than run on
	// success only: a table that fails partway has still rolled the service for
	// the fields it did apply, and that rollout is still owed its row.
	ctx, flushRollout := rollout.Batch(ctx)
	defer flushRollout()
	var ops core.PatchOps[AppView]
	for _, row := range servicePatchTable {
		if !row.present(p) {
			continue
		}
		ops.Add(true, func() (AppView, error) { return row.apply(ctx, s, id, p) })
	}
	return ops.Run(func() (AppView, error) { return s.Get(ctx, id) })
}
