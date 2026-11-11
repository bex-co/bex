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
