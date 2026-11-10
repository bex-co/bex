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
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestRenderEventDetailsMatchTheirMember projects one stored row of every
// Render-documented type bex records and checks its details against the member
// of Render's details union that type names (w3/m302). missing is the
// Render-required fields bex does not record, each a divergence ADR018's
// "Service event types and details" row records: an
// actor's email, ids of objects bex has no record of, Render's deprecated
// integer status, autoscaling criteria and the disk, maintenance and
// auto-deploy facts the audit row omits (w3/061). Anything else, a renamed or
// mistyped key included, fails.
func TestRenderEventDetailsMatchTheirMember(t *testing.T) {
	route := rendertest.Load(t).Operation("retrieve-event")
	at := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	two, three := int32(2), int32(3)
	disk := "dsk-0123456789abcdefghij"
	instance := "srv-0123456789abcdefghij-abcde"
	for _, tc := range []struct {
		row     store.ServiceEventRow
		missing []string
	}{
		{row: store.ServiceEventRow{Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerManual, Caller: "usr-1"}, missing: []string{"trigger/user/email"}},
		{row: store.ServiceEventRow{Source: store.EventSourceDeploy, Phase: "ended", DeployID: "dep-1", Status: "live"}, missing: []string{"status"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeBuildStarted, DeployID: "dep-1"}, missing: []string{"buildId", "trigger"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeBuildEnded, DeployID: "dep-1", FactStatus: store.EventStatusSucceeded}, missing: []string{"buildId", "status"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePreDeployStarted, DeployID: "dep-1"}, missing: []string{"deployCommandExecutionId"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypePreDeployEnded, DeployID: "dep-1", FactStatus: store.EventStatusFailed}, missing: []string{"deployCommandExecutionId", "status"}},
		{row: store.ServiceEventRow{Key: "job:job-0123456789abcdefghij:run_ended", Source: store.EventSourceFact, FactType: TypeJobRunEnded, FactStatus: store.EventStatusSucceeded}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeCronJobRunStarted, FactRunID: "crn-0123456789abcdefghij"}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeCronJobRunEnded, FactRunID: "crn-0123456789abcdefghij", FactStatus: store.EventStatusFailed, ReasonCode: store.EventReasonEvicted}},
		{row: store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeServerFailed, InstanceID: instance, ReasonCode: store.EventReasonReadinessFailed}},
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
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoscaling", AutoscalingMinTo: &two, AutoscalingMaxTo: &three}, missing: []string{"fromConfig/criteria", "fromConfig/max", "fromConfig/min", "toConfig/criteria"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(true)}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetAutoDeploy", AutoDeployEnabled: new(false)}, missing: []string{"reason"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceMode"}, missing: []string{"enabled"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceModeURI"}, missing: []string{"fromURI", "toURI"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.AddDisk", Caller: "usr-1"}, missing: []string{"diskId", "sizeGB"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.UpdateDisk", Caller: "usr-1"}, missing: []string{"diskId", "fromSizeGB", "toSizeGB"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.DeleteDisk", Caller: "usr-1"}, missing: []string{"diskId"}},
		{row: store.ServiceEventRow{Source: store.EventSourceAudit, Verb: "apps.RestoreDiskSnapshot", Caller: "usr-1", DiskID: &disk}},
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
