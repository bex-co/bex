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
	"encoding/json"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m114: a failed cron run's cron_job_run_ended event says why, in the shape
// of Render's "Cron Job Run Ended" details: cronJobRunId plus a reason object.
// REST and MCP share toRenderEvent, so this pins both.
func TestCronJobRunEndedCarriesRendersReasonObject(t *testing.T) {
	exit := int32(3)
	for _, tc := range []struct {
		name       string
		reasonCode string
		exitCode   *int32
		status     string
		want       string
	}{
		{"non-zero exit", store.EventReasonNonZeroExit, &exit, store.EventStatusFailed, `{"evicted":false,"nonZeroExit":3}`},
		{"out of memory", store.EventReasonOOMKilled, nil, store.EventStatusFailed, `{"evicted":false,"oomKilled":{"memoryLimit":""}}`},
		{"evicted", store.EventReasonEvicted, nil, store.EventStatusFailed, `{"evicted":true}`},
		{"timed out", store.EventReasonTimedOut, nil, store.EventStatusFailed, `{"evicted":false,"timedOutSeconds":43200}`},
		{"cause not observed", "", nil, store.EventStatusFailed, ``},
		{"succeeded", "", nil, store.EventStatusSucceeded, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := store.ServiceEventRow{
				Key: "cron:srv-cron:nightly-100:ended", At: time.Date(2026, 10, 5, 5, 51, 27, 0, time.UTC),
				Source: store.EventSourceFact, FactType: TypeCronJobRunEnded, FactStatus: tc.status,
				ReasonCode: tc.reasonCode, FactExitCode: tc.exitCode, FactRunID: "crr-abc",
			}
			body, err := json.Marshal(toRenderEvent(view(row, "srv-cron")).Details)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				CronJobRunID string          `json:"cronJobRunId"`
				Status       string          `json:"status"`
				Reason       json.RawMessage `json:"reason"`
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got.CronJobRunID != "crr-abc" || got.Status != tc.status {
				t.Fatalf("details = %s, want cronJobRunId crr-abc and status %s", body, tc.status)
			}
			if string(got.Reason) != tc.want {
				t.Fatalf("reason = %s, want %s", got.Reason, tc.want)
			}
		})
	}
}

// Only cron run events carry a cronJobRunId, and only cron_job_run_ended a
// reason: other event types never grow either, even from a fact row that
// happens to carry the columns.
func TestReasonObjectIsOnlyOnCronJobRunEnded(t *testing.T) {
	exit := int32(1)
	row := store.ServiceEventRow{
		Key: "deploy:dep-1:build_ended", Source: store.EventSourceFact, FactType: TypeBuildEnded,
		FactStatus: store.EventStatusFailed, ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &exit, FactRunID: "crr-x",
	}
	d := toRenderEvent(view(row, "srv-1")).Details
	if d.Reason != nil || d.CronJobRunID != "" {
		t.Fatalf("build_ended details = %+v, want no cron run reason or id", d)
	}
}

// w4/259: Render's "Cron Job Run Started" details require cronJobRunId, and
// bex's starts carried none — not stored, and not formatted for REST/MCP even
// when present. A start now names the same crr- id as its run and its end;
// starts stored before the fix derive it from their canonical source key.
func TestCronJobRunStartedCarriesItsRunID(t *testing.T) {
	derived := ids.Derive(ids.CronRun, "nightly-100")
	for _, tc := range []struct {
		name, key, stored, want string
	}{
		{"stored id", "fact:cron:srv-cron:nightly-100:started", derived, derived},
		{"stored id wins over the key", "fact:cron:srv-cron:other-1:started", "crr-stored", "crr-stored"},
		{"pre-fix start, canonical key", "fact:cron:srv-cron:nightly-100:started", "", derived},
		{"manual run job name", "fact:cron:srv-cron:" + "manual-abc123:started", "", ids.Derive(ids.CronRun, "manual-abc123")},
		{"no fact prefix", "cron:srv-cron:nightly-100:started", "", ""},
		{"ended key", "fact:cron:srv-cron:nightly-100:ended", "", ""},
		{"no app id", "fact:cron::nightly-100:started", "", ""},
		{"no run name", "fact:cron:srv-cron::started", "", ""},
		{"not a job name", "fact:cron:srv-cron:Nightly_100:started", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit := int32(3)
			row := store.ServiceEventRow{
				Key: tc.key, At: time.Date(2026, 10, 11, 16, 34, 0, 0, time.UTC),
				Source: store.EventSourceFact, FactType: TypeCronJobRunStarted, FactRunID: tc.stored,
				// Reason columns on a start must still never become a reason.
				ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &exit,
			}
			ev := view(row, "srv-cron")
			if ev.Details.CronJobRunID != tc.want {
				t.Fatalf("GraphQL/neutral cronJobRunId = %q, want %q", ev.Details.CronJobRunID, tc.want)
			}
			if ev.ID != ids.Derive(ids.Event, tc.key) {
				t.Fatalf("event id changed: %q", ev.ID)
			}
			body, err := json.Marshal(toRenderEvent(ev).Details)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if _, has := got["reason"]; has {
				t.Fatalf("started details carry a reason: %s", body)
			}
			id, has := got["cronJobRunId"]
			if tc.want == "" {
				if has {
					t.Fatalf("a malformed key manufactured an id: %s", body)
				}
				return
			}
			if id != tc.want {
				t.Fatalf("REST/MCP details = %s, want cronJobRunId %s", body, tc.want)
			}
		})
	}
}
