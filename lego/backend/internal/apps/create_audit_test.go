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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A service create records one CreateServerEvent once the service exists,
// naming it; a create refused after authorization (a name in use) and a
// dry-run record none; a denial still records its error row (w4/m321 t011,
// t012).
func TestCreateRecordsOneNamedRowOnlyForACreateThatHappened(t *testing.T) {
	sink := &captureAuditSink{}
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "img:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	created := createRows(sink)
	if len(created) != 1 {
		t.Fatalf("a create recorded %d apps.Create rows, want 1: %+v", len(created), created)
	}
	row := created[0]
	if kind, _, _ := core.SplitTarget(row.Target); kind != "service" || row.TargetName != "web" {
		t.Errorf("create row target = %q name %q, want a service target named web", row.Target, row.TargetName)
	}
	if row.Outcome != core.AuditAllowed || row.Resource != core.WorkspaceObject("tea-a") {
		t.Errorf("create row = outcome %q resource %q, want allowed on workspace:tea-a", row.Outcome, row.Resource)
	}

	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "img:2"}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create on a name in use: got %v, want ErrConflict", err)
	}
	if _, err := svc.Create(ctx, CreateRequest{Name: "preview", Image: "img:1", DryRun: true}); err != nil {
		t.Fatalf("dry-run create: %v", err)
	}
	if n := len(createRows(sink)); n != 1 {
		t.Fatalf("a refused create and a dry-run added rows: %d apps.Create rows, want still 1", n)
	}

	svc.Authz = denyRelationChecker{deny: core.RelCanCreate}
	if _, err := svc.Create(ctx, CreateRequest{Name: "denied", Image: "img:1"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied create: got %v, want ErrForbidden", err)
	}
	rows := createRows(sink)
	if len(rows) != 2 || rows[1].Outcome != core.AuditDenied {
		t.Fatalf("a denied create must still record its denial: %+v", rows)
	}
}

func createRows(sink *captureAuditSink) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range sink.events {
		if ev.Verb == "apps.Create" {
			out = append(out, ev)
		}
	}
	return out
}
