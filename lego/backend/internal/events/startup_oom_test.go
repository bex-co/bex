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
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/630: a release that ran out of memory while starting ends with
// reason.failure.oomKilled naming its plan's limit, on the service's feed and
// on the by-id read, as server_failed does for a running instance. A crash
// that was not a memory kill carries no oomKilled at all.
func TestDeployEndedOOMNamesThePlansMemoryLimit(t *testing.T) {
	exit := int32(137)
	oom := store.ServiceEventRow{Key: "dep-oom:ended", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded,
		DeployID: "dep-oom", Status: store.DeployUpdateFailed, End: store.DeployEnd{ExitCode: &exit, OOMKilled: true}}
	limit := func(ev Event) string {
		t.Helper()
		if ev.Type != TypeDeployEnded || ev.Details.EndReason == nil || ev.Details.EndReason.Failure == nil ||
			ev.Details.EndReason.Failure.OOMKilled == nil {
			t.Fatalf("event %+v carries no oomKilled", ev)
		}
		return ev.Details.EndReason.Failure.OOMKilled.MemoryLimit
	}
	app := sampleApp("web", "srv-1", "")
	app.Spec.Tier = "free"

	listed, err := newService(&fakeStore{rows: []store.ServiceEventRow{oom}}, app).List(t.Context(), "web", Filter{})
	if err != nil || len(listed) != 1 {
		t.Fatalf("List = %+v, %v", listed, err)
	}
	if got := limit(listed[0]); got != "512Mi" {
		t.Errorf("listed memoryLimit = %q, want 512Mi", got)
	}

	st := &fakeStore{lookup: store.ServiceEventLookup{Event: oom, ServiceID: "srv-1"}}
	got, err := newService(st, app).Get(t.Context(), ids.Derive(ids.Event, oom.Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := limit(got); got != "512Mi" {
		t.Errorf("by-id memoryLimit = %q, want 512Mi", got)
	}

	crash := oom
	crash.End = store.DeployEnd{ExitCode: new(int32(3))}
	ev := view(crash, "srv-1")
	withMemoryLimit(&ev, "free")
	if f := ev.Details.EndReason.Failure; f == nil || f.OOMKilled != nil || ev.Details.MemoryLimit != "" {
		t.Errorf("a crash reads %+v, want no memory limit", ev.Details)
	}
}
