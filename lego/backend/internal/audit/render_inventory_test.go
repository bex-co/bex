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

package audit

import (
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// renderWorkspaceEvent is one workspace event of Render's audit log and what
// bex records for it: the verbs that report it, or why none does (a non-goal,
// or the milestone that owns it).
type renderWorkspaceEvent struct {
	verbs   []string
	notYet  string
	nonGoal string
}

// renderWorkspaceEvents is every workspace event render.com/docs/audit-logs
// lists (read 2026-11-16), the inventory docs/render-artifacts/audit-logs-api.md
// tabulates (w9/m449). Organization events are out of scope: bex has no
// organization above a workspace.
var renderWorkspaceEvents = map[string]renderWorkspaceEvent{
	// Member management.
	"InviteToTeamEvent":                  {verbs: []string{core.AuditVerbMemberInvited}},
	"RemoveUserFromTeamEvent":            {verbs: []string{core.AuditVerbMemberRemoved, core.AuditVerbMemberLeft}},
	"AcceptTeamInviteEvent":              {verbs: []string{core.AuditVerbInviteAccepted}},
	"ChangeTeamMemberRoleEvent":          {verbs: []string{core.AuditVerbMemberRoleChanged}},
	"ChangeTeamAllowedLoginMethodsEvent": {nonGoal: "bex has no allowed-login-methods setting"},
	"ChangeTeam2FAEnforcementEvent":      {verbs: []string{core.AuditVerbSetRequireTwoFactor}},
	"LoginEvent":                         {notYet: "w9/m452"},
	"LogoutEvent":                        {notYet: "w9/m452"},
	// Apps and services.
	"CreateServerEvent":              {verbs: []string{"apps.Create"}},
	"SuspendServiceEvent":            {verbs: []string{"apps.Suspend"}},
	"ResumeServiceEvent":             {verbs: []string{"apps.Resume"}},
	"MaintenanceModeEnabledEvent":    {verbs: []string{core.AuditVerbMaintenanceModeEnabled}},
	"MaintenanceModeURIUpdatedEvent": {verbs: []string{core.AuditVerbMaintenanceModeURIUpdated}},
	"UpdateServiceNameEvent":         {verbs: []string{"apps.SetDisplayName"}},
	"DeleteServerEvent":              {verbs: []string{"apps.Delete"}},
	// A shell session's own rows, and a refused shell's denial (w9/m451).
	"StartShellEvent":              {verbs: []string{core.AuditVerbStartShell, "apps.ResolveSSHSession", "apps.CreateShellSession"}},
	"EndShellEvent":                {verbs: []string{core.AuditVerbEndShell}},
	"ApplyBlueprintEvent":          {verbs: []string{"apps.ApproveBlueprint", "apps.SyncBlueprint"}},
	"CreateCronJobEvent":           {verbs: []string{core.AuditVerbCreateCronJob}},
	"DeleteCronJobEvent":           {verbs: []string{core.AuditVerbDeleteCronJob}},
	"UpdateAutoDeployEvent":        {verbs: []string{core.AuditVerbSetAutoDeploy}},
	"UpdateAutoDeployTriggerEvent": {verbs: []string{core.AuditVerbSetAutoDeployTrigger, core.AuditVerbSetBuildFilter}},
	"CreateCustomDomainEvent":      {verbs: []string{core.AuditVerbAddDomain}},
	"DeleteCustomDomainEvent":      {verbs: []string{core.AuditVerbDeleteDomain}},
	// Datastores and disks.
	"UpdateIPAllowListEvent": {verbs: []string{
		"apps.SetIPAllowList", "postgres.SetIPAllowList", "keyvalue.SetIPAllowList",
		core.AuditVerbSetWorkspaceIPAllowList, core.AuditVerbSetEnvironmentIPAllowList,
	}},
	"CreatePostgresEvent":         {verbs: []string{"postgres.CreatePostgres"}},
	"DeletePostgresEvent":         {verbs: []string{"postgres.DeletePostgres"}},
	"SuspendPostgresEvent":        {verbs: []string{"postgres.Suspend"}},
	"ResumePostgresEvent":         {verbs: []string{"postgres.Resume"}},
	"DownloadDatabaseBackupEvent": {verbs: []string{core.AuditVerbDownloadDatabaseBackup}},
	"ViewConnectionInfoEvent":     {verbs: []string{core.AuditVerbViewConnectionInfo}},
	"CreateRedisEvent":            {verbs: []string{"keyvalue.CreateKeyValue"}},
	"DeleteRedisEvent":            {verbs: []string{"keyvalue.DeleteKeyValue"}},
	"CreateServerDiskEvent":       {verbs: []string{core.AuditVerbAddDisk}},
	"DeleteServerDiskEvent":       {verbs: []string{core.AuditVerbDeleteDisk}},
	"RestoreDiskSnapshotEvent":    {verbs: []string{core.AuditVerbRestoreDiskSnapshot}},
	// Environment variables. secrets.DeleteEnvVar is a single delete's row
	// recorded before w9/m449, and a refused one's.
	"UpdateEnvVarsEvent":    {verbs: []string{core.AuditVerbUpdateEnvVars}},
	"CreateEnvVarsEvent":    {verbs: []string{core.AuditVerbCreateEnvVars}},
	"DeleteEnvVarsEvent":    {verbs: []string{core.AuditVerbDeleteEnvVars, core.AuditVerbDeleteEnvVar}},
	"ViewEnvVarValuesEvent": {verbs: []string{core.AuditVerbViewEnvVarValues}},
	"DeleteEnvGroupEvent":   {verbs: []string{"envgroups.DeleteEnvGroup"}},
	// Webhooks.
	"CreateWebhookEvent": {verbs: []string{"webhooks.Create"}},
	"UpdateWebhookEvent": {verbs: []string{"webhooks.Update"}},
	"DeleteWebhookEvent": {verbs: []string{"webhooks.Delete"}},
	// Metrics streams.
	"CreateOtelIntegrationEvent": {nonGoal: "external metrics streams (.pm/DO_NOT_DO.md, external log/metric drains)"},
	"DeleteOtelIntegrationEvent": {nonGoal: "external metrics streams (.pm/DO_NOT_DO.md, external log/metric drains)"},
	"UpdateOtelIntegrationEvent": {nonGoal: "external metrics streams (.pm/DO_NOT_DO.md, external log/metric drains)"},
	// Projects and environments.
	"CreateProjectEvent":     {verbs: []string{"projects.Create", "projects.CreateWithEnvironments"}},
	"DeleteProjectEvent":     {verbs: []string{"projects.Delete", "projects.DeleteIfEmpty"}},
	"CreateEnvironmentEvent": {verbs: []string{"environments.Create", "environments.CreateWithACL"}},
	"DeleteEnvironmentEvent": {verbs: []string{"environments.Delete", "environments.DeleteIfEmpty"}},
	"MoveEnvironmentResourceEvent": {verbs: []string{
		core.AuditVerbProjectServiceMoved, core.AuditVerbEnvironmentServiceMoved,
	}},
	"ChangeEnvironmentProtectionEvent": {verbs: []string{core.AuditVerbSetEnvironmentProtection}},
	"UpdateEnvironmentIsolatedEvent":   {verbs: []string{core.AuditVerbSetEnvironmentIsolation}},
	// Compliance documents.
	"DocumentDownloadEvent": {nonGoal: "bex has no compliance-document portal"},
	"SignNDAEvent":          {nonGoal: "bex has no compliance-document portal"},
}

// TestRenderWorkspaceEventsAreRecorded holds the workspace audit log to
// Render's list (w9/m449). Each listed event is either recorded, under every
// verb the inventory names for it, or carries a written reason it is not; and
// a verb mapped onto one of these events is listed, so the inventory is the
// whole truth both ways. Removing any one mapping fails it.
func TestRenderWorkspaceEventsAreRecorded(t *testing.T) {
	for event, want := range renderWorkspaceEvents {
		reasons := 0
		for _, r := range []string{want.notYet, want.nonGoal} {
			if r != "" {
				reasons++
			}
		}
		switch {
		case len(want.verbs) == 0 && reasons == 0:
			t.Errorf("%s: no verb records it and no non-goal or owning milestone says why", event)
		case len(want.verbs) > 0 && reasons > 0:
			t.Errorf("%s: lists verbs and a reason it is not recorded", event)
		case reasons > 1:
			t.Errorf("%s: both a non-goal and an owning milestone", event)
		}
		for _, verb := range want.verbs {
			if got := renderEvents[verb]; got != event {
				t.Errorf("%s: verb %s reports %q, want %s", event, verb, renderEvent(verb), event)
			}
		}
	}
	for verb, event := range renderEvents {
		want, listed := renderWorkspaceEvents[event]
		if listed && !slices.Contains(want.verbs, verb) {
			t.Errorf("renderEvents maps %s onto %s, which renderWorkspaceEvents does not list for it", verb, event)
		}
	}
}
