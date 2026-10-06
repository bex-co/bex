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
	"testing"
)

// TestScopeByID pins the by-id routing contract (w4/m172, w5/m115): it only
// routes, and the verb's own Authorize decides and audits.
func TestScopeByID(t *testing.T) {
	b := &Base{Workspace: multiWorkspace{"dana": {"tea-a", "tea-b"}, "eve": {"tea-c"}}}
	dana := WithIdentity(context.Background(), Identity{Subject: "dana", Method: "session"})
	eve := WithIdentity(context.Background(), Identity{Subject: "eve", Method: "session"})
	in := func(workspaces ...string) ResourceOwner {
		return func(context.Context) ([]string, error) { return workspaces, nil }
	}

	if ctx, err := b.ScopeByID(dana, "", in("tea-b")); err != nil || NamedWorkspace(ctx) != "tea-b" {
		t.Errorf("member, no owner: %q %v; want tea-b", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByID(dana, "tea-a", in("tea-b")); err != nil || NamedWorkspace(ctx) != "tea-a" {
		t.Errorf("explicit ownerId must decide: %q %v", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByID(WithWorkspace(dana, "tea-a"), "", in("tea-b")); err != nil || NamedWorkspace(ctx) != "tea-a" {
		t.Errorf("a request-named workspace must decide: %q %v", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByID(eve, "", in("tea-b")); err != nil || NamedWorkspace(ctx) != "tea-b" {
		t.Errorf("non-member: %q %v; want routed to tea-b, where the verb refuses", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByID(eve, "", in()); err != nil || NamedWorkspace(ctx) != "" {
		t.Errorf("unknown id stays on the default path: %q %v", NamedWorkspace(ctx), err)
	}
	readErr := errors.New("db down")
	if _, err := b.ScopeByID(dana, "", func(context.Context) ([]string, error) { return nil, readErr }); err != readErr {
		t.Errorf("routing read failure = %v, want it surfaced", err)
	}
	if ctx, err := (&Base{}).ScopeByID(dana, "", in("tea-b")); err != nil || NamedWorkspace(ctx) != "" {
		t.Errorf("store off: nothing to route: %q %v", NamedWorkspace(ctx), err)
	}

	// Several workspaces hold the resource: the caller's default, else the only
	// one of theirs, else ambiguity; none of theirs routes to the first.
	for _, tc := range []struct {
		name       string
		workspaces []string
		want       string
		wantErr    error
	}{
		{"default among them", []string{"tea-b", "tea-a"}, "tea-a", nil},
		{"the only one of theirs", []string{"tea-c", "tea-b"}, "tea-b", nil},
		{"none of theirs", []string{"tea-x", "tea-y"}, "tea-x", nil},
	} {
		if ctx, err := b.ScopeByID(dana, "", in(tc.workspaces...)); !errors.Is(err, tc.wantErr) || NamedWorkspace(ctx) != tc.want {
			t.Errorf("%s: %q %v; want %q", tc.name, NamedWorkspace(ctx), err, tc.want)
		}
	}
	three := &Base{Workspace: multiWorkspace{"dana": {"tea-a", "tea-b", "tea-c"}}}
	if _, err := three.ScopeByID(dana, "", in("tea-b", "tea-c")); !errors.Is(err, ErrConflict) {
		t.Errorf("two of the caller's non-default workspaces = %v, want ErrConflict naming ownerId", err)
	}
	broken := &Base{Workspace: brokenWorkspace{multiWorkspace{"dana": {"tea-a"}}}}
	if _, err := broken.ScopeByID(dana, "", in("tea-b", "tea-c")); !errors.Is(err, ErrAuthzUnavailable) {
		t.Errorf("membership outage while choosing = %v, want ErrAuthzUnavailable (fail closed)", err)
	}
}

// TestScopeByIDLeavesTheRefusalToTheVerb is the w5/m115 audit fix: a caller
// outside the owning workspace is refused by the verb's own Authorize — the
// typed-id 403 — and that refusal is recorded once, where ScopeByID's old
// membership check refused before any audit point.
func TestScopeByIDLeavesTheRefusalToTheVerb(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Authz: &fakeAllowChecker{}, Audit: sink, Workspace: multiWorkspace{"eve": {"tea-c"}}}
	eve := WithIdentity(context.Background(), Identity{Subject: "eve", Method: "session"})
	ctx, err := b.ScopeByID(eve, "", func(context.Context) ([]string, error) { return []string{"tea-b"}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := suspendLikeVerb(ctx, b); !errors.Is(err, ErrForbidden) {
		t.Fatalf("verb = %v, want the typed-id 403", err)
	}
	if sink.len() != 1 {
		t.Fatalf("audit rows = %d, want the one denial", sink.len())
	}
}

// TestScopeByVisibleID is the matrix's exemption: only the caller's own
// workspaces count, so a resource held nowhere they belong is not routed.
func TestScopeByVisibleID(t *testing.T) {
	b := &Base{Workspace: multiWorkspace{"dana": {"tea-a", "tea-b"}, "eve": {"tea-c"}}}
	dana := WithIdentity(context.Background(), Identity{Subject: "dana", Method: "session"})
	eve := WithIdentity(context.Background(), Identity{Subject: "eve", Method: "session"})
	inB := func(context.Context) ([]string, error) { return []string{"tea-b"}, nil }
	if ctx, err := b.ScopeByVisibleID(dana, "", inB); err != nil || NamedWorkspace(ctx) != "tea-b" {
		t.Errorf("member: %q %v; want tea-b", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByVisibleID(eve, "", inB); err != nil || NamedWorkspace(ctx) != "" {
		t.Errorf("non-member: %q %v; want the default path", NamedWorkspace(ctx), err)
	}
}
