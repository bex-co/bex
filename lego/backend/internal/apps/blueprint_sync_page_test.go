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
	"encoding/json"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The dashboard's Syncs lists the plan waiting for approval where its start
// places it among the first page, newest first, and keeps the page at its
// limit, so the next page's cursor (the last row shown) loses no sync
// (w3/m678).
func TestWithWaitingPlanPlacesThePlanByItsStart(t *testing.T) {
	run := func(id, started string) BlueprintSyncView {
		return BlueprintSyncView{ID: id, StartedAt: "2026-11-26T10:" + started + ":00Z"}
	}
	waiting := run("exe-w", "30")
	ids := func(views []BlueprintSyncView) string {
		out := make([]string, len(views))
		for i, v := range views {
			out[i] = v.ID
		}
		return strings.Join(out, " ")
	}
	for _, tc := range []struct {
		name  string
		views []BlueprintSyncView
		limit int
		want  string
	}{
		{"no history", nil, 20, "exe-w"},
		{"newest of all", []BlueprintSyncView{run("exe-b", "20"), run("exe-a", "10")}, 20, "exe-w exe-b exe-a"},
		{"between two runs", []BlueprintSyncView{run("exe-c", "40"), run("exe-a", "10")}, 20, "exe-c exe-w exe-a"},
		{"older than a short page", []BlueprintSyncView{run("exe-c", "40")}, 20, "exe-c exe-w"},
		{"pushes a full page's oldest to the next", []BlueprintSyncView{run("exe-c", "40"), run("exe-a", "10")}, 2, "exe-c exe-w"},
		{"older than a full page", []BlueprintSyncView{run("exe-d", "50"), run("exe-c", "40")}, 2, "exe-d exe-c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := ids(tc.views)
			if got := ids(withWaitingPlan(tc.views, waiting, tc.limit)); got != tc.want {
				t.Fatalf("got [%s], want [%s]", got, tc.want)
			}
			if ids(tc.views) != before {
				t.Fatal("the history page was modified in place")
			}
		})
	}
}

// A sync's page shows each planned action in the sync's state while it waits,
// runs or has succeeded, and in no state once it failed: bex records a failed
// sync's error, not which action met it. A failed plan's errors are its
// error notices, warnings left out; any other failed sync's is its message.
func TestBlueprintSyncDetailStatesAndErrors(t *testing.T) {
	svc := &Service{}
	b := store.Blueprint{ID: "blp-x"}
	actions := json.RawMessage(`[{"operation":"create","kind":"service","name":"api"},{"operation":"noop","kind":"service","name":"web"}]`)
	msg := "planning failed: runtime rocket is not supported"
	for _, tc := range []struct {
		state, wantState string
		notices          json.RawMessage
		message          *string
		wantErrors       string
	}{
		{state: store.BlueprintSyncStateCreated, wantState: "created", wantErrors: ""},
		{state: store.BlueprintSyncStateRunning, wantState: "running", wantErrors: ""},
		{state: store.BlueprintSyncStateSuccess, wantState: "success", wantErrors: ""},
		{state: store.BlueprintSyncStateError, wantState: "", message: &msg, wantErrors: msg},
		{state: store.BlueprintSyncStateError, wantState: "", message: &msg,
			notices:    json.RawMessage(`[{"severity":"warning","message":"w"},{"severity":"error","message":"runtime rocket is not supported","line":4}]`),
			wantErrors: "runtime rocket is not supported"},
	} {
		d, err := svc.blueprintSyncDetail(b, store.PlannedBlueprintSync{
			BlueprintSync: store.BlueprintSync{ID: "bsr-1", State: tc.state, ErrorMessage: tc.message},
			PlanActions:   actions, ErrorNotices: tc.notices,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(d.ActionStates) != len(d.Actions) || len(d.Actions) != 2 {
			t.Fatalf("%s: %d actions, %d states", tc.state, len(d.Actions), len(d.ActionStates))
		}
		for _, s := range d.ActionStates {
			if s.State != tc.wantState {
				t.Errorf("%s: an action reads %q, want %q", tc.state, s.State, tc.wantState)
			}
		}
		var got []string
		for _, e := range d.Errors {
			got = append(got, e.Message)
		}
		if strings.Join(got, "|") != tc.wantErrors {
			t.Errorf("%s: errors %q, want %q", tc.state, got, tc.wantErrors)
		}
	}
}
