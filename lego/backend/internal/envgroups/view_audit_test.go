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

package envgroups

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type viewAuditSink struct{ events []core.AuditEvent }

func (s *viewAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.events = append(s.events, ev)
	return nil
}

// w4/m321: revealing an environment group's value records one Render
// ViewEnvVarValuesEvent naming the group; the keys-only view records none.
func TestEnvGroupValueRevealRecordsOneViewEvent(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "K", Value: "v"}}); err != nil {
		t.Fatal(err)
	}
	sink := &viewAuditSink{}
	svc.Audit = sink
	if _, err := svc.GetEnvGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbViewEnvVarValues {
			t.Fatalf("the keys-only view recorded %+v", ev)
		}
	}
	if _, err := svc.GetEnvGroupVar(ctx, g.ID, "K"); err != nil {
		t.Fatal(err)
	}
	views := 0
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbViewEnvVarValues {
			views++
			if ev.Target != core.EnvGroupTarget(g.ID) || ev.TargetName != "shared" || ev.Outcome != core.AuditAllowed {
				t.Errorf("view event %+v", ev)
			}
		}
	}
	if views != 1 {
		t.Fatalf("recorded %+v, want one ViewEnvVarValuesEvent", sink.events)
	}
}

// w4/563: every change to a service's linked groups records one row its feed
// shows, naming the group — a group created with services, the Link and
// Unlink controls, and a group deleted while linked.
func TestEveryLinkChangeRecordsANamedRow(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("api"), sampleApp("web"))
	ctx := testCtx()
	sink := &viewAuditSink{}
	svc.Audit = sink
	// A slice, not a map by target: a doubled row for one service must count.
	links := func(verb string) []core.AuditEvent {
		var out []core.AuditEvent
		for _, ev := range sink.events {
			if ev.Verb == verb && ev.Outcome == core.AuditAllowed {
				out = append(out, ev)
			}
		}
		return out
	}
	named := func(t *testing.T, rows []core.AuditEvent, id, name string, want int) {
		t.Helper()
		if len(rows) != want {
			t.Fatalf("rows = %+v, want %d", rows, want)
		}
		for _, ev := range rows {
			if ev.EnvGroupID != id || ev.EnvGroupName != name {
				t.Errorf("row %+v does not name group %s %q", ev, id, name)
			}
		}
	}

	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", ServiceIDs: []string{"api", "web"}})
	if err != nil {
		t.Fatal(err)
	}
	named(t, links(core.AuditVerbLinkService), g.ID, "shared", 2)

	sink.events = nil
	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	named(t, links(core.AuditVerbUnlinkService), g.ID, "shared", 1)

	sink.events = nil
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	named(t, links(core.AuditVerbLinkService), g.ID, "shared", 1)

	sink.events = nil
	if err := svc.DeleteEnvGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	named(t, links(core.AuditVerbUnlinkService), g.ID, "shared", 2)
}

// w4/563: a service a Blueprint creates with fromGroup records one named link
// row per group it starts with.
func TestBlueprintInitialGroupsRecordNamedLinks(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := testCtx()
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "first"})
	if err != nil {
		t.Fatal(err)
	}
	sink := &viewAuditSink{}
	svc.Audit = sink
	app := managedApp("web")
	if err := svc.WithInitialEnvGroups(ctx, []string{"first"}, app, func() error {
		return svc.Client.Create(ctx, app)
	}, nil); err != nil {
		t.Fatal(err)
	}
	var links []core.AuditEvent
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbLinkService && ev.Outcome == core.AuditAllowed {
			links = append(links, ev)
		}
	}
	if len(links) != 1 || links[0].EnvGroupID != group.ID || links[0].EnvGroupName != "first" {
		t.Fatalf("link rows = %+v, want one naming %s first", links, group.ID)
	}
}
