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

package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeAuditSink records every event handed to it — the acceptance seam t001
// asked for ("unit-verifiable with a fake sink").
type fakeAuditSink struct {
	mu     sync.Mutex
	events []AuditEvent
}

func (f *fakeAuditSink) Record(_ context.Context, ev AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeAuditSink) len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

// failingAuditSink always errors — proves a sink outage never fails the verb.
type failingAuditSink struct{}

func (failingAuditSink) Record(context.Context, AuditEvent) error {
	return errors.New("sink unreachable")
}

// suspendLikeVerb stands in for a feature verb (e.g. apps.Service.Suspend):
// its whole body is the one Authorize call every verb makes, so callerVerb
// resolves to this function's own name — the case the inventory sweep (t004)
// depends on for every real write verb in the codebase.
func suspendLikeVerb(ctx context.Context, b *Base) error {
	return b.Authorize(ctx, RelCanOperate)
}

// deleteWorkspaceLikeVerb stands in for a verb that calls AuthorizeOn directly
// against a named object (workspaces.Service.Delete's shape) — the
// cross-tenant-capable entry point.
func deleteWorkspaceLikeVerb(ctx context.Context, b *Base, object string) error {
	return b.AuthorizeOn(ctx, RelCanManage, object)
}

// listLikeVerb stands in for a read verb (apps.Service.List's shape): a read
// relation must never reach the sink at all.
func listLikeVerb(ctx context.Context, b *Base) error {
	return b.Authorize(ctx, RelCanView)
}

// scaleLikeVerb stands in for a SERVICE-SCOPED write verb (apps.Service.Scale's
// shape, w3/m7): it names the resource it acts on, which is what makes its audit
// row attributable to one service — and therefore what makes the events feed a
// view rather than a second write path.
func scaleLikeVerb(ctx context.Context, b *Base, service string) error {
	return b.AuthorizeTarget(ctx, RelCanOperate, ServiceTarget(service))
}

// TestAuditTargetNamesTheResourceActedOn is w3/m7's write side: AuthorizeTarget
// records WHICH service a verb changed (Resource stays the workspace it was
// authorized against — the two are different questions), it resolves the verb
// name from the same frame depth Authorize does, and a plain Authorize still
// records no target at all.
func TestAuditTargetNamesTheResourceActedOn(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeAllowChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})

	if err := scaleLikeVerb(ctx, b, "web"); err != nil {
		t.Fatalf("scaleLikeVerb: %v", err)
	}
	if err := suspendLikeVerb(ctx, b); err != nil { // a workspace-wide verb, for contrast
		t.Fatalf("suspendLikeVerb: %v", err)
	}
	if sink.len() != 2 {
		t.Fatalf("got %d events, want 2", sink.len())
	}
	targeted, untargeted := sink.events[0], sink.events[1]
	if targeted.Target != "service:web" {
		t.Errorf("Target = %q, want %q — the events feed keys on this", targeted.Target, "service:web")
	}
	if targeted.Resource != DefaultWorkspace {
		t.Errorf("Resource = %q, want the workspace the verb was authorized against (%q)", targeted.Resource, DefaultWorkspace)
	}
	// The frame skip must be identical on both entry points, or the verb name
	// silently becomes "Base.AuthorizeTarget" and every event type goes unmapped.
	if targeted.Verb != "core.scaleLikeVerb" {
		t.Errorf("Verb = %q, want %q", targeted.Verb, "core.scaleLikeVerb")
	}
	if untargeted.Target != "" {
		t.Errorf("a workspace-wide verb recorded Target = %q, want empty", untargeted.Target)
	}
}

// TestAuditTargetRecordsDeniedAttempts pins the property internal/events relies
// on to keep a stranger out of a service's feed: a DENIED authorize still writes
// a targeted row (that is the audit log's job), so the feed — not the writer —
// must be what filters it out (store.ListServiceEvents: allowed-only,
// workspace-scoped).
func TestAuditTargetRecordsDeniedAttempts(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeDenyChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "stranger", Method: "oauth2"})

	if err := scaleLikeVerb(ctx, b, "victim"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("denied verb = %v, want ErrForbidden", err)
	}
	if sink.len() != 1 {
		t.Fatalf("got %d events, want 1 (a denial is recorded, not swallowed)", sink.len())
	}
	if ev := sink.events[0]; ev.Outcome != AuditDenied || ev.Target != "service:victim" {
		t.Errorf("denied event = %+v, want outcome=denied target=service:victim", ev)
	}
}

func TestAuditRecordsWriteVerbSuccessWithResolvedVerbName(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeAllowChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})

	if err := suspendLikeVerb(ctx, b); err != nil {
		t.Fatalf("suspendLikeVerb: %v", err)
	}
	if sink.len() != 1 {
		t.Fatalf("got %d events, want exactly 1", sink.len())
	}
	ev := sink.events[0]
	if ev.Outcome != AuditAllowed {
		t.Errorf("Outcome = %q, want %q", ev.Outcome, AuditAllowed)
	}
	if ev.Caller != "user-1" || ev.CallerMethod != "oauth2" {
		t.Errorf("Caller/CallerMethod = %q/%q, want user-1/oauth2", ev.Caller, ev.CallerMethod)
	}
	if ev.Verb != "core.suspendLikeVerb" {
		t.Errorf("Verb = %q, want %q", ev.Verb, "core.suspendLikeVerb")
	}
	if ev.Resource != DefaultWorkspace {
		t.Errorf("Resource = %q, want %q", ev.Resource, DefaultWorkspace)
	}
	if ev.At.IsZero() {
		t.Error("At must be set")
	}
}

func TestAuditRecordsDenialOnCrossTenantAuthorizeOn(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeDenyChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})

	err := deleteWorkspaceLikeVerb(ctx, b, WorkspaceObject("tea-other"))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if sink.len() != 1 {
		t.Fatalf("got %d events, want exactly 1", sink.len())
	}
	ev := sink.events[0]
	if ev.Outcome != AuditDenied {
		t.Errorf("Outcome = %q, want %q", ev.Outcome, AuditDenied)
	}
	if ev.Resource != "workspace:tea-other" {
		t.Errorf("Resource = %q, want workspace:tea-other", ev.Resource)
	}
	if ev.Verb != "core.deleteWorkspaceLikeVerb" {
		t.Errorf("Verb = %q, want core.deleteWorkspaceLikeVerb", ev.Verb)
	}
}

func TestAuditSkipsReadRelations(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeDenyChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})

	if err := listLikeVerb(ctx, b); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if sink.len() != 0 {
		t.Fatalf("got %d events for a read relation, want 0", sink.len())
	}
}

func TestAuditNilSinkIsNoop(t *testing.T) {
	b := &Base{Authz: &fakeAllowChecker{}} // Audit left nil — store-off mode
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})
	if err := suspendLikeVerb(ctx, b); err != nil {
		t.Fatalf("suspendLikeVerb: %v", err)
	}
	// No assertion beyond "did not panic/block" — NoopAuditSink is exercised.
}

func TestAuditSinkFailureNeverFailsTheVerb(t *testing.T) {
	b := &Base{Authz: &fakeAllowChecker{}, Audit: failingAuditSink{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})
	if err := suspendLikeVerb(ctx, b); err != nil {
		t.Fatalf("suspendLikeVerb: %v — a failing audit sink must not fail the verb", err)
	}
}

// fakeDenyChecker denies everything — the counterpart to fakeAllowChecker.
type fakeDenyChecker struct{}

func (fakeDenyChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// helperVerbService stands in for a feature service whose exported verb
// (Suspend) delegates to shared UNEXPORTED helper methods before reaching an
// authorize entry point — apps.Service's post-w2/m30 shape (Suspend →
// setSuspended → writeThroughStore → AuthorizeTarget). callerVerb must walk
// past the helpers and record the VERB, or every consolidated verb collapses
// into one meaningless helper name and vanishes from the events feed's and
// outbound webhooks' verb-keyed vocabularies (found live by w3/m11/t008).
type helperVerbService struct{ base *Base }

func (s *helperVerbService) Suspend(ctx context.Context, service string) error {
	return s.setSuspended(ctx, service)
}

func (s *helperVerbService) setSuspended(ctx context.Context, service string) error {
	return s.writeThroughStore(ctx, service)
}

func (s *helperVerbService) writeThroughStore(ctx context.Context, service string) error {
	return s.base.AuthorizeTarget(ctx, RelCanOperate, ServiceTarget(service))
}

func TestCallerVerbWalksPastUnexportedHelperMethods(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeAllowChecker{}, Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: "oauth2"})

	svc := &helperVerbService{base: b}
	if err := svc.Suspend(ctx, "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if sink.len() != 1 {
		t.Fatalf("got %d events, want 1", sink.len())
	}
	if got := sink.events[0].Verb; got != "core.Suspend" {
		t.Errorf("Verb = %q, want %q — helper frames must not be recorded as the verb", got, "core.Suspend")
	}

	// A plain unexported FUNCTION is a terminal caller (the stand-in-verb
	// shape above), never walked past — the two shapes must coexist.
	if err := scaleLikeVerb(ctx, b, "web"); err != nil {
		t.Fatalf("scaleLikeVerb: %v", err)
	}
	if got := sink.events[1].Verb; got != "core.scaleLikeVerb" {
		t.Errorf("Verb = %q, want %q — plain functions are terminal", got, "core.scaleLikeVerb")
	}
}
