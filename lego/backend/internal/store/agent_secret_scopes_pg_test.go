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
	"slices"
	"testing"
)

// w9/m385: scope rows upsert, list in name order and delete; the table
// refuses a repo scope without a repository and a repository on any other
// scope; a session's applied secrets round-trip on every read path.
func TestPGAgentSecretScopesAndAppliedSecrets(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "secret-scopes")
	ctx := context.Background()
	for _, row := range []AgentSecretScope{
		{Name: "ORG", Scope: AgentSecretScopeWorkspace},
		{Name: "APP", Scope: AgentSecretScopeRepo, Repo: "bex-co/app"},
		{Name: "NAMED", Scope: AgentSecretScopeExplicit},
	} {
		if err := s.UpsertAgentSecretScope(ctx, tenant.ID, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertAgentSecretScope(ctx, tenant.ID, AgentSecretScope{Name: "ORG", Scope: AgentSecretScopeRepo, Repo: "bex-co/other"}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListAgentSecretScopes(ctx, tenant.ID)
	want := []AgentSecretScope{
		{Name: "APP", Scope: AgentSecretScopeRepo, Repo: "bex-co/app"},
		{Name: "NAMED", Scope: AgentSecretScopeExplicit},
		{Name: "ORG", Scope: AgentSecretScopeRepo, Repo: "bex-co/other"},
	}
	if err != nil || !slices.Equal(rows, want) {
		t.Fatalf("scopes = %v / %v", rows, err)
	}
	for _, bad := range []AgentSecretScope{
		{Name: "X", Scope: AgentSecretScopeRepo},
		{Name: "X", Scope: AgentSecretScopeWorkspace, Repo: "a/b"},
		{Name: "X", Scope: "team"},
	} {
		if err := s.UpsertAgentSecretScope(ctx, tenant.ID, bad); err == nil {
			t.Errorf("the table accepted %+v", bad)
		}
	}
	if err := s.DeleteAgentSecretScope(ctx, tenant.ID, "APP"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAgentSecretScope(ctx, tenant.ID, "APP"); err != nil {
		t.Fatalf("deleting a missing row = %v", err)
	}
	if rows, _ := s.ListAgentSecretScopes(ctx, tenant.ID); len(rows) != 2 {
		t.Fatalf("after delete = %v", rows)
	}

	session, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", IdempotencyKey: "k", IdempotencyHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if session.AppliedSecrets == nil || len(session.AppliedSecrets) != 0 {
		t.Fatalf("a new session's applied secrets = %#v", session.AppliedSecrets)
	}
	applied := []AppliedSecret{{Name: "ORG", Source: "workspace"}, {Name: "STRIPE_KEY", Source: "explicit"}}
	if err := s.SetAgentSessionAppliedSecrets(ctx, session.ID, applied); err != nil {
		t.Fatal(err)
	}
	read, err := s.GetAgentSession(ctx, session.ID)
	if err != nil || !slices.Equal(read.AppliedSecrets, applied) {
		t.Fatalf("applied after set = %v / %v", read.AppliedSecrets, err)
	}
	found, _, err := s.FindAgentSessionByIdempotencyKey(ctx, tenant.ID, "k")
	if err != nil || !slices.Equal(found.AppliedSecrets, applied) {
		t.Fatalf("idempotency lookup applied = %v / %v", found.AppliedSecrets, err)
	}
	if err := s.SetAgentSessionAppliedSecrets(ctx, session.ID, nil); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := pool.QueryRow(ctx, `SELECT applied_secrets::text FROM agent_sessions WHERE id=$1`, session.ID).Scan(&raw); err != nil || raw != "[]" {
		t.Fatalf("cleared applied secrets = %q / %v", raw, err)
	}
}
