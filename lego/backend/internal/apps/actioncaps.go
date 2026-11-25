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
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ActionCapabilities projects the apps-owned lifecycle verbs' decisions for
// ONE service in the acting workspace (ADR087, w6/m136): per action, the
// tri-state permission (core.CanDecision — the exact relation the verb gates
// on, pinned by api/roleladder_test.go) plus the bounded precondition its
// execute path would enforce, computed by the same predicate the verb calls
// (appProtected, RequireBillingMutation, requireFreeHoursAvailable,
// requireBandwidthAvailable, pendingCronRun). Suspend and resume
// are restricted verbs, so on a protected member they read as
// requireUnprotected's own fresh workspace-manage check (core.RestrictedDecision,
// w4/m578). So does shell, the permission CreateShellSession and native SSH
// open with (can_view_sensitive), so a client can say only an Admin may before
// it mints a ticket (w4/726); whether the service can host a shell at all is
// its sshAddress, not a precondition here. Toggling maintenance mode reads the
// same way on a web service, with SetMaintenanceMode's permission (can_create,
// a service setting since w9/m619), so a Contributor sees the switch disabled
// and a Developer on a protected member sees it disabled for the reason the
// verb would refuse it (w4/725); its plan eligibility is the service's plan,
// which the client already reads. Restart reads with the wired verb's guards:
// a cron job has no restart so the row is absent there, a suspended service
// reports suspended unless a saved paid plan waits for the deploy that lifts
// it, an unmanaged service reports unavailable, and otherwise the billing
// gate; store-off (RestartDeploy nil) the verb runs no guards, so the row
// stays unconditioned. Delete reads the same way, with
// Delete's own permission (can_create), on every type, so a Developer on a
// protected member sees Delete disabled before typing its phrase (w4/724).
// Read-only and non-auditing: can_view gates the projection itself; a
// snapshot is never a bearer grant — the verb still runs its own guards at
// dispatch.
//
// The target binds to the ACTING workspace (RequireResourceInActingWorkspace):
// a service owned elsewhere reads as absent here even when the caller could
// reach it there, so a client cannot render workspace A's screen from
// workspace B's resource. Cron actions appear only on a cron_job, and shell
// only on a type that runs instances — an absent action means "does not exist
// for this type", never "forbidden"; so does the maintenance toggle on any
// type but a web service.
func (s *Service) ActionCapabilities(ctx context.Context, name string) ([]core.ActionDecision, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, a.Labels); err != nil {
		return nil, err
	}
	// The binding above proved the resource lives in the acting workspace, so
	// its own workspace object IS the acting one — probe it directly rather
	// than re-resolving per relation. Preconditions are computed only for an
	// allowed decision (DecideAction drops them otherwise, and a denied
	// caller's projection must not pay store reads — or leak protection
	// state — for buttons it will never enable), and the billing gate, the
	// protection read and the admin check each run at most once per request
	// however many actions consult them.
	workspace := core.WorkspaceObject(s.WorkspaceOrDefault(ctx))
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, workspace)
	billing := sync.OnceValue(func() string { return s.BillingPreconditionFor(ctx, a.Labels[core.LabelTenant]) })
	protected := sync.OnceValues(func() (bool, error) { return s.appProtected(ctx, a) })
	admin := sync.OnceValue(func() error { return s.AuthorizeAppFresh(ctx, core.RelCanManage, a) })
	suspend, isProtected := core.RestrictedDecision("suspend", operate, protected, admin)
	resume, _ := core.RestrictedDecision("resume", operate, protected, admin)
	precond := func(compute func() string) string {
		if !operate.Allowed() {
			return ""
		}
		return compute()
	}
	suspendPre, resumePre := core.LifecyclePreconditions(a.Spec.Suspended, isProtected, resume, billing)
	if resume.Allowed() && resumePre == "" {
		// The allowance gates Resume enforces after dunning, in the same
		// order: a service the monthly Free instance hours or the included
		// bandwidth suspended reports why, so the button is disabled with
		// the reason instead of failing after the click.
		if ferr := s.requireFreeHoursAvailable(a); ferr != nil {
			resumePre = core.PrecondFreeHoursExhausted
		} else if berr := s.requireBandwidthAvailable(ctx, a); berr != nil {
			if errors.Is(berr, core.ErrBillingUnavailable) {
				resumePre = core.PrecondUnavailable
			} else {
				resumePre = core.PrecondBandwidthExhausted
			}
		}
	}
	create := s.CanDecisionOn(ctx, core.RelCanCreate, workspace)
	remove, _ := core.RestrictedDecision("delete", create, protected, admin)
	restartPre := ""
	if operate.Allowed() && s.RestartDeploy != nil {
		// The wired restart (deploys.Restart — what GraphQL restartServer,
		// REST and MCP run) refuses a suspended service, an unmanaged one
		// and a billing-blocked workspace; the projection names the first
		// stable block in the usual state-then-dunning order, exempting the
		// saved paid plan validateTrigger lifts past. The store-off verb
		// runs no guards, so its row stays unconditioned.
		switch {
		case a.Spec.Suspended && !core.DeployLiftsFreeHoursSuspension(a):
			restartPre = core.PrecondSuspended
		case a.Labels[core.LabelAppID] == "":
			restartPre = core.PrecondUnavailable
		default:
			restartPre = billing()
		}
	}
	out := []core.ActionDecision{}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		// No restart exists for a cron job — the verb refuses it 400 in both
		// modes — so there is no row: absent means "not offered here", as for
		// the shell and the maintenance toggle on the types they do not serve.
		out = append(out, core.DecideAction(core.ActionRestart, operate, restartPre))
	}
	out = append(out,
		core.DecideAction(core.ActionSuspend, suspend, suspendPre),
		core.DecideAction(core.ActionResume, resume, resumePre),
		core.DecideAction(core.ActionDelete, remove, ""),
	)
	typ := effectiveType(a.Spec.Type)
	if sshTypeHasInstances(typ) {
		shell, _ := core.RestrictedDecision("shell", s.CanDecisionOn(ctx, core.RelCanViewSensitive, workspace), protected, admin)
		out = append(out, core.DecideAction(core.ActionShell, shell, ""))
	}
	if typ == appv1alpha1.TypeWebService {
		toggle, _ := core.RestrictedDecision(core.VerbToggleMaintenanceMode, create, protected, admin)
		out = append(out, core.DecideAction(core.ActionToggleMaintenanceMode, toggle, ""))
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob {
		out = append(out,
			core.DecideAction(core.ActionCronRunNow, operate, precond(func() string {
				if a.Spec.Suspended { // TriggerCronRun's guard order: state, then billing
					return core.PrecondSuspended
				}
				return billing()
			})),
			core.DecideAction(core.ActionCronCancelRun, operate, precond(func() string {
				if _, ok := pendingCronRun(a, s.Now()); !ok { // CancelCurrentCronRun's scan
					return core.PrecondNoActiveRun
				}
				return ""
			})),
		)
	}
	return out, nil
}
