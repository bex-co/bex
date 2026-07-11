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
