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

package store

import (
	"context"
	"testing"
)

// ADR102 D9: the in-sandbox read is limited to strict descendants, and a
// session remembers who created it.
func TestPGAgentSessionDescendantAndCreator(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "descendant")
	_, _, other := agentSessionPGStore(t, "descendant-other")
	ctx := context.Background()
	root := lineageSession(t, s, tenant.ID, "", nil)
	a := lineageSession(t, s, tenant.ID, root.ID, nil)
	b := lineageSession(t, s, tenant.ID, root.ID, nil)
	grand := lineageSession(t, s, tenant.ID, a.ID, nil)
	foreign := lineageSession(t, s, other.ID, "", nil)

	for name, tc := range map[string]struct {
		ancestor, id string
		want         bool
	}{
		"child":                    {root.ID, a.ID, true},
		"grandchild":               {root.ID, grand.ID, true},
		"grandchild of its parent": {a.ID, grand.ID, true},
		"itself":                   {root.ID, root.ID, false},
		"parent":                   {a.ID, root.ID, false},
		"sibling":                  {a.ID, b.ID, false},
		"niece":                    {b.ID, grand.ID, false},
		"unrelated":                {root.ID, foreign.ID, false},
		"unknown":                  {root.ID, "ags-nope", false},
	} {
		if got, err := s.IsAgentSessionDescendant(ctx, tenant.ID, tc.ancestor, tc.id); err != nil || got != tc.want {
			t.Errorf("%s: descendant = %v / %v, want %v", name, got, err, tc.want)
		}
	}
	// Scoped to the workspace: another workspace's id for the ancestor never matches.
	if got, _ := s.IsAgentSessionDescendant(ctx, other.ID, root.ID, a.ID); got {
		t.Error("a descendant check crossed workspaces")
	}

	made, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", CreatedBy: "user-1"})
	if err != nil || made.CreatedBy != "user-1" {
		t.Fatalf("created_by = %q / %v", made.CreatedBy, err)
	}
	if got, err := s.GetAgentSession(ctx, made.ID); err != nil || got.CreatedBy != "user-1" {
		t.Fatalf("read back created_by = %q / %v", got.CreatedBy, err)
	}
	if root.CreatedBy != "" {
		t.Fatalf("a row created without a creator reads %q", root.CreatedBy)
	}
}
