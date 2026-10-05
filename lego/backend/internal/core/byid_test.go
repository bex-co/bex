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

// TestScopeByID pins the by-id routing contract (w4/m172).
func TestScopeByID(t *testing.T) {
	b := &Base{Workspace: multiWorkspace{"dana": {"tea-a", "tea-b"}, "eve": {"tea-c"}}}
	dana := WithIdentity(context.Background(), Identity{Subject: "dana", Method: "session"})
	eve := WithIdentity(context.Background(), Identity{Subject: "eve", Method: "session"})
	errMissing := errors.New("thing not found")
	inB := func(context.Context) (string, bool, error) { return "tea-b", true, nil }
	unknown := func(context.Context) (string, bool, error) { return "", false, nil }
	named := func(ctx context.Context) (string, bool) { return WorkspaceFrom(ctx) }

	ctx, err := b.ScopeByID(dana, "", inB, errMissing)
	if ws, ok := named(ctx); err != nil || !ok || ws != "tea-b" {
		t.Errorf("member, no owner: workspace=%q named=%v err=%v; want tea-b", ws, ok, err)
	}
	if ctx, err := b.ScopeByID(dana, "tea-a", inB, errMissing); err != nil || NamedWorkspace(ctx) != "tea-a" {
		t.Errorf("explicit ownerId must decide: %q %v", NamedWorkspace(ctx), err)
	}
	if ctx, err := b.ScopeByID(WithWorkspace(dana, "tea-a"), "", inB, errMissing); err != nil || NamedWorkspace(ctx) != "tea-a" {
		t.Errorf("a request-named workspace must decide: %q %v", NamedWorkspace(ctx), err)
	}
	if _, err := b.ScopeByID(eve, "", inB, errMissing); err != errMissing {
		t.Errorf("non-member = %v, want the not-found answer", err)
	}
	if ctx, err := b.ScopeByID(eve, "", unknown, errMissing); err != nil || NamedWorkspace(ctx) != "" {
		t.Errorf("unknown id stays on the default path: %q %v", NamedWorkspace(ctx), err)
	}
	broken := &Base{Workspace: brokenWorkspace{}}
	if _, err := broken.ScopeByID(dana, "", inB, errMissing); !errors.Is(err, ErrAuthzUnavailable) {
		t.Errorf("membership outage = %v, want ErrAuthzUnavailable (fail closed)", err)
	}
	readErr := errors.New("db down")
	if _, err := b.ScopeByID(dana, "", func(context.Context) (string, bool, error) { return "", false, readErr }, errMissing); err != readErr {
		t.Errorf("routing read failure = %v, want it surfaced", err)
	}
	if ctx, err := (&Base{}).ScopeByID(dana, "", inB, errMissing); err != nil || NamedWorkspace(ctx) != "" {
		t.Errorf("store off: nothing to route: %q %v", NamedWorkspace(ctx), err)
	}
}
