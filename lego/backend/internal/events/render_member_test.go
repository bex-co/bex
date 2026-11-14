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

package events

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestRenderEventDetailsMatchTheirMember projects one stored row of every
// Render-documented type bex records and checks its details against the member
// of Render's details union that type names (w3/m302). missing is the
// Render-required fields bex does not record, each a divergence ADR018's
// "Service event types and details" row records: an
// actor's email, ids of objects bex has no record of, Render's deprecated
// integer status and autoscaling criteria. Anything else, a renamed or
// mistyped key included, fails.
func TestRenderEventDetailsMatchTheirMember(t *testing.T) {
	route := rendertest.Load(t).Operation("retrieve-event")
	at := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	two, three, sixty := int32(2), int32(3), int32(60)
	disk := "dsk-0123456789abcdefghij"
	instance := "srv-0123456789abcdefghij-abcde"
	for _, tc := range []struct {
		row     store.ServiceEventRow
		missing []string
	}{
		{row: store.ServiceEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerManual, Caller: "usr-1"}, missing: []string{"trigger/user/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceDeploy, Phase: "ended", DeployID: "dep-1", Status: "live"}, missing: []string{"status"}},
		// w4/639: Build Started carries its deploy's trigger; buildId stays
		// unrecorded (w3/m302 t002).
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeBuildStarted, DeployID: "dep-1"}, missing: []string{"buildId"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeBuildEnded, DeployID: "dep-1", FactStatus: store.EventStatusSucceeded}, missing: []string{"buildId", "status"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePreDeployStarted, DeployID: "dep-1"}, missing: []string{"deployCommandExecutionId"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePreDeployEnded, DeployID: "dep-1", FactStatus: store.EventStatusFailed}, missing: []string{"deployCommandExecutionId", "status"}},
		{row: store.ServiceEventRow{Key: "job:job-0123456789abcdefghij:run_ended", Source: store.EventSourceFact, FactType: TypeJobRunEnded, FactStatus: store.EventStatusSucceeded}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeCronJobRunStarted, FactRunID: "crn-0123456789abcdefghij"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeCronJobRunEnded, FactRunID: "crn-0123456789abcdefghij", FactStatus: store.EventStatusFailed, ReasonCode: store.EventReasonEvicted}},
		// w4/631: a user-canceled run names its user.
		{row: store.ServiceEventRow{Key: "cron:canceled", Source: store.EventSourceFact, FactType: TypeCronJobRunEnded, FactRunID: "crn-0123456789abcdefghij", FactStatus: store.EventStatusCanceled, Caller: "usr-1"}, missing: []string{"user/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeServerFailed, InstanceID: instance, ReasonCode: store.EventReasonReadinessFailed}},
		// w8/m158: a running service's process that crashed, was killed for
		// memory or ended on its own, each with the instance that failed.
		{row: store.ServiceEventRow{Key: "exit:crash", Source: store.EventSourceFact, FactType: TypeServerFailed, InstanceID: instance, ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &three}},
		{row: store.ServiceEventRow{Key: "exit:oom", Source: store.EventSourceFact, FactType: TypeServerFailed, InstanceID: instance, ReasonCode: store.EventReasonOOMKilled}},
		{row: store.ServiceEventRow{Key: "exit:early", Source: store.EventSourceFact, FactType: TypeServerFailed, InstanceID: instance, ReasonCode: store.EventReasonEarlyExit}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeServerAvailable, InstanceID: instance}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeImagePullFailed, Image: "docker.io/library/nginx:1", ReasonCode: store.EventReasonImagePullBackoff}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeCommitIgnored, CommitID: "abc123", CommitURL: "https://github.com/o/r/commit/abc123", ReasonCode: store.EventReasonSkipPhrase}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeBranchDeleted, BranchFrom: "feature"}, missing: []string{"newBranch"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeAutoscalingStarted, FromCount: &two, ToCount: &three}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeAutoscalingEnded, FromCount: &three, ToCount: &two}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.Suspend", Caller: "usr-1"}, missing: []string{"suspendedByUser/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.Resume", Caller: "usr-1"}, missing: []string{"resumedByUser/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.Restart", Caller: "usr-1"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetPlan", PlanFrom: new("starter"), PlanTo: new("standard")}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.Scale", InstanceCountFrom: &two, InstanceCountTo: &three}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoscaling", AutoscalingMinTo: &two, AutoscalingMaxTo: &three, AuditChanges: store.AuditChanges{AutoscalingCPUTo: &sixty}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoscaling", AutoscalingMinFrom: &two, AutoscalingMaxFrom: &three, AutoscalingMinTo: &two, AutoscalingMaxTo: &three, AuditChanges: store.AuditChanges{AutoscalingMemoryFrom: &sixty, AutoscalingCPUTo: &sixty}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.DeleteAutoscaling", AutoscalingMinFrom: &two, AutoscalingMaxFrom: &three, AuditChanges: store.AuditChanges{AutoscalingCPUFrom: &sixty}}},
		// A row recorded before the targets were (w4/m335).
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoscaling", AutoscalingMinTo: &two, AutoscalingMaxTo: &three}, missing: []string{"toConfig/criteria"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(true), AuditChanges: store.AuditChanges{AutoDeployTrigger: new("checksPass")}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(false), AuditChanges: store.AuditChanges{AutoDeployTrigger: new("commit"), AutoDeployReason: new("rollback")}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceMode", AuditChanges: store.AuditChanges{MaintenanceModeTo: new(false)}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceModeURI", AuditChanges: store.AuditChanges{MaintenanceURIFrom: new("https://status.example.com"), MaintenanceURITo: new("")}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.AddDisk", Caller: "usr-1", DiskID: &disk, DiskSizeGB: &two}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.UpdateDisk", Caller: "usr-1", DiskID: &disk, DiskSizeGB: &three, AuditChanges: store.AuditChanges{DiskSizeFromGB: &two}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.DeleteDisk", Caller: "usr-1", DiskID: &disk}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.RestoreDiskSnapshot", Caller: "usr-1", DiskID: &disk}},
		// w4/m379: the Postgres and Key Value members. The user's email is
		// resolved at read time, which this projection-only test does not do.
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresCreated, Caller: "usr-1"}, missing: []string{"user/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresRestarted}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresPoolerChanged, ConnectionPoolEnabled: new(true)}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresDiskSizeChanged, DiskSizeGB: &sixty, AuditChanges: store.AuditChanges{DiskSizeFromGB: &three}}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresHAChanged, HighAvailabilityEnabled: new(false)}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresReadReplicasChanged, AuditChanges: store.AuditChanges{ReadReplicasFrom: &two, ReadReplicasTo: &three}}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePostgresUpgradeStarted, VersionFrom: "16", VersionTo: "17"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePostgresUpgradeSucceeded, VersionFrom: "16", VersionTo: "17"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePostgresUpgradeFailed, VersionFrom: "16", VersionTo: "17"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: core.AuditVerbKeyValueConfigChanged, PersistenceMode: new("off")}},
	} {
		tc.row.Key, tc.row.At = cmp.Or(tc.row.Key, fmt.Sprintf("k:%s:%s:%s", tc.row.Source, tc.row.FactType, tc.row.Verb)), at
		event := toRenderEvent(view(tc.row, "srv-0123456789abcdefghij"))
		t.Run(event.Type, func(t *testing.T) {
			body, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, field := range tc.missing {
				want = append(want, rendertest.MissingEventDetail(event.Type, field))
			}
			// The type itself is not this test's subject: Render documents
			// members for suspender_added/_removed but leaves both out of its
			// type enum, and that violation is the Allowlist's event/type entry.
			got := slices.DeleteFunc(rendertest.Validate(route, http.StatusOK, http.Header{"Content-Type": {"application/json"}}, body),
				func(v string) bool { return strings.HasPrefix(v, "/type: ") })
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s details %s\n got violations %q\nwant %q", event.Type, body, got, want)
			}
		})
	}
}

// TestChangeEventDetailsCarryTheRecordedFacts pins w3/m317's projection: the
// disk, maintenance-mode and auto-deploy facts land on Render's keys with their
// recorded values, and a row recorded before they were kept carries none of
// those keys rather than an invented zero.
func TestChangeEventDetailsCarryTheRecordedFacts(t *testing.T) {
	ten, twenty := int32(10), int32(20)
	disk := "dsk-0123456789abcdefghij"
	for _, tc := range []struct {
		row  store.ServiceEventRow
		want map[string]any
	}{
		{store.ServiceEventRow{Verb: "apps.AddDisk", DiskID: &disk, DiskSizeGB: &ten}, map[string]any{"diskId": disk, "sizeGB": 10.0}},
		{store.ServiceEventRow{Verb: "apps.UpdateDisk", DiskID: &disk, DiskSizeGB: &twenty, AuditChanges: store.AuditChanges{DiskSizeFromGB: &ten}}, map[string]any{"diskId": disk, "fromSizeGB": 10.0, "toSizeGB": 20.0}},
		{store.ServiceEventRow{Verb: "apps.DeleteDisk", DiskID: &disk, DiskSizeGB: &ten}, map[string]any{"diskId": disk}},
		{store.ServiceEventRow{Verb: "apps.SetMaintenanceMode", AuditChanges: store.AuditChanges{MaintenanceModeTo: new(true)}}, map[string]any{"enabled": true}},
		{store.ServiceEventRow{Verb: "apps.SetMaintenanceModeURI", AuditChanges: store.AuditChanges{MaintenanceURIFrom: new(""), MaintenanceURITo: new("https://status.example.com")}}, map[string]any{"fromURI": "", "toURI": "https://status.example.com"}},
		{store.ServiceEventRow{Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(false), AuditChanges: store.AuditChanges{AutoDeployTrigger: new("checksPass"), AutoDeployReason: new("setting_change")}}, map[string]any{"reason": "setting_change", "fromTrigger": "checksPass"}},
		{store.ServiceEventRow{Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(true), AuditChanges: store.AuditChanges{AutoDeployTrigger: new("commit")}}, map[string]any{"newTrigger": "commit"}},
		// Rows recorded before w3/m317.
		{store.ServiceEventRow{Verb: "apps.AddDisk"}, map[string]any{}},
		{store.ServiceEventRow{Verb: "apps.UpdateDisk"}, map[string]any{}},
		{store.ServiceEventRow{Verb: "apps.SetMaintenanceMode"}, map[string]any{}},
		{store.ServiceEventRow{Verb: "apps.SetMaintenanceModeURI"}, map[string]any{}},
		{store.ServiceEventRow{Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(false)}, map[string]any{}},
	} {
		tc.row.Source, tc.row.Key = store.EventSourceAudit, "aud-1:"
		event := toRenderEvent(view(tc.row, "srv-0123456789abcdefghij"))
		body, err := json.Marshal(event.Details)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(got, tc.want) {
			t.Errorf("%s (%s) details = %s, want %v", event.Type, tc.row.Verb, body, tc.want)
		}
	}
}

// TestAutoscalingEventDetailsCarryTheTargets pins w4/m335: each side of
// autoscaling_config_changed is Render's autoscaling config, its criteria the
// recorded CPU and memory targets, so a change to a target alone shows on both
// sides. A disable keeps the config turned off, as Render's does; a side with
// no config reads as GET …/autoscaling does without autoscaling; and a row
// recorded before the targets were carries no criteria.
func TestAutoscalingEventDetailsCarryTheTargets(t *testing.T) {
	one, three, sixty, seventy := int32(1), int32(3), int32(60), int32(70)
	const off = `{"enabled":false,"min":0,"max":0,"criteria":{"cpu":{"enabled":false,"percentage":0},"memory":{"enabled":false,"percentage":0}}}`
	for _, tc := range []struct {
		name string
		row  store.ServiceEventRow
		want string
	}{
		{"a target change", store.ServiceEventRow{Verb: "apps.SetAutoscaling", AutoscalingMinFrom: &one, AutoscalingMaxFrom: &three, AutoscalingMinTo: &one, AutoscalingMaxTo: &three,
			AuditChanges: store.AuditChanges{AutoscalingCPUFrom: &seventy, AutoscalingCPUTo: &sixty, AutoscalingMemoryTo: &seventy}},
			`{"fromConfig":{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":70},"memory":{"enabled":false,"percentage":0}}},` +
				`"toConfig":{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":true,"percentage":70}}}}`},
		{"disabling", store.ServiceEventRow{Verb: "apps.DeleteAutoscaling", AutoscalingMinFrom: &one, AutoscalingMaxFrom: &three, AuditChanges: store.AuditChanges{AutoscalingMemoryFrom: &sixty}},
			`{"fromConfig":{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":false,"percentage":0},"memory":{"enabled":true,"percentage":60}}},` +
				`"toConfig":{"enabled":false,"min":1,"max":3,"criteria":{"cpu":{"enabled":false,"percentage":0},"memory":{"enabled":true,"percentage":60}}}}`},
		{"enabling the settings a disable kept (w4/597)", store.ServiceEventRow{Verb: "apps.SetAutoscaling", AutoscalingMinFrom: &one, AutoscalingMaxFrom: &three, AutoscalingMinTo: &one, AutoscalingMaxTo: &three,
			AuditChanges: store.AuditChanges{AutoscalingEnabledFrom: new(false), AutoscalingCPUFrom: &sixty, AutoscalingCPUTo: &sixty}},
			`{"fromConfig":{"enabled":false,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}},` +
				`"toConfig":{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":60},"memory":{"enabled":false,"percentage":0}}}}`},
		{"recorded before the targets", store.ServiceEventRow{Verb: "apps.SetAutoscaling", AutoscalingMinTo: &one, AutoscalingMaxTo: &three},
			`{"fromConfig":` + off + `,"toConfig":{"enabled":true,"min":1,"max":3}}`},
	} {
		tc.row.Source, tc.row.Key = store.EventSourceAudit, "aud-1:"
		event := toRenderEvent(view(tc.row, "srv-0123456789abcdefghij"))
		body, err := json.Marshal(event.Details)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != tc.want {
			t.Errorf("%s: details %s\nwant %s", tc.name, body, tc.want)
		}
	}
}

// TestRenameAndDomainEventDetailsNameThem pins w4/m337: a rename sends both
// names as from/to, a custom-domain event its hostname, and a row recorded
// before either keeps empty details.
func TestRenameAndDomainEventDetailsNameThem(t *testing.T) {
	for _, tc := range []struct {
		row  store.ServiceEventRow
		want string
	}{
		{store.ServiceEventRow{Verb: "apps.SetDisplayName", AuditChanges: store.AuditChanges{DisplayNameFrom: new("api"), DisplayNameTo: new("Checkout API")}}, `{"from":"api","to":"Checkout API"}`},
		{store.ServiceEventRow{Verb: "apps.SetDisplayName"}, `{}`},
		{store.ServiceEventRow{Verb: "apps.AddDomain", AuditChanges: store.AuditChanges{Domain: new("app.example.com")}}, `{"domain":"app.example.com"}`},
		{store.ServiceEventRow{Verb: "apps.VerifyDomain", AuditChanges: store.AuditChanges{Domain: new("app.example.com")}}, `{"domain":"app.example.com"}`},
		{store.ServiceEventRow{Verb: "apps.DeleteDomain", AuditChanges: store.AuditChanges{Domain: new("app.example.com")}}, `{"domain":"app.example.com"}`},
		{store.ServiceEventRow{Verb: "apps.DeleteDomain"}, `{}`},
	} {
		tc.row.Source, tc.row.Key = store.EventSourceAudit, "aud-1:"
		event := toRenderEvent(view(tc.row, "srv-0123456789abcdefghij"))
		body, err := json.Marshal(event.Details)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != tc.want {
			t.Errorf("%s (%s): details %s, want %s", event.Type, tc.row.Verb, body, tc.want)
		}
	}
}
