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

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w21/093: core.RetryPlacementClear runs the authorizing fetch from a closure, so
// a conditional clear recorded "postgres.ClearProjectID.func1" (and the
// environment one the same) until the fetch named its verb. No HTTP route
// reaches the clears — a project or environment delete calls them — so this
// drives them and reads the rows.
func TestPlacementClearsRecordTheirOwnAuditVerb(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clear    func(*Service) func(context.Context, string, string) error
		expected string
		verb     string
	}{
		{"project", func(s *Service) func(context.Context, string, string) error { return s.ClearProjectID }, "prj-a", AuditVerbClearProjectID},
		{"environment", func(s *Service) func(context.Context, string, string) error { return s.ClearEnvironmentID }, "env-a", AuditVerbClearEnvironmentID},
		{"ungroup", func(s *Service) func(context.Context, string, string) error { return s.Ungroup }, "env-a", AuditVerbUngroup},
	} {
		for _, allow := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/allowed=%v", tc.name, allow), func(t *testing.T) {
				svc, _ := newService(placementResource())
				svc.Authz = &fakeChecker{allow: allow}
				sink := &webhookAuditSink{}
				svc.Audit = sink

				err := tc.clear(svc)(ctxAs("user-a"), "placement", tc.expected)
				if allow && err != nil || !allow && !errors.Is(err, core.ErrForbidden) {
					t.Fatalf("allow=%v: error = %v", allow, err)
				}
				if len(sink.events) != 1 {
					t.Fatalf("allow=%v recorded %d rows, want 1: %+v", allow, len(sink.events), sink.events)
				}
				ev := sink.events[0]
				wantOutcome := core.AuditAllowed
				if !allow {
					wantOutcome = core.AuditDenied
				}
				if ev.Verb != tc.verb || ev.Outcome != wantOutcome || ev.Target != core.DatabaseTarget("placement") || ev.Relation != core.RelCanCreate {
					t.Fatalf("allow=%v recorded %s %s (%s on %s), want %s %s on the resource",
						allow, ev.Outcome, ev.Verb, ev.Relation, ev.Target, wantOutcome, tc.verb)
				}
			})
		}
	}
}

// Each attempt of a retried clear re-authorizes and records under the same verb.
func TestPlacementClearRetriesRecordOneVerb(t *testing.T) {
	svc, cl := newService(placementResource())
	sink := &webhookAuditSink{}
	svc.Audit = sink
	svc.Authz = &fakeChecker{allow: true}
	conflicts := 2
	tracking := &placementPatchClient{Client: cl}
	svc.Client = tracking
	tracking.beforeEach = func() {
		if conflicts > 0 {
			conflicts--
			tracking.fail = apierrors.NewConflict(schema.GroupResource{Resource: "x"}, "placement", errors.New("changed"))
		} else {
			tracking.fail = nil
		}
	}

	if err := svc.ClearProjectID(ctxAs("user-a"), "placement", "prj-a"); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 3 {
		t.Fatalf("a clear that lost its patch twice recorded %d rows, want 3 (one per attempt)", len(sink.events))
	}
	for _, ev := range sink.events {
		if ev.Verb != AuditVerbClearProjectID {
			t.Errorf("an attempt recorded %q, want %q", ev.Verb, AuditVerbClearProjectID)
		}
	}
}
