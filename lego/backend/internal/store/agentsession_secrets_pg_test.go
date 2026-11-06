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
	"errors"
	"slices"
	"testing"
)

// ADR098: a session's attached secret names round-trip on the row, through
// create, a turn and the idempotency lookup. Only names are stored.
func TestPGAgentSessionSecretNames(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "secret-names")
	ctx := context.Background()
	row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x",
		SecretNames: []string{"STRIPE_KEY"}, IdempotencyKey: "k1", IdempotencyHash: "h1"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(row.SecretNames, []string{"STRIPE_KEY"}) {
		t.Fatalf("created names = %v", row.SecretNames)
	}
	plain, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "y"})
	if err != nil || plain.SecretNames == nil || len(plain.SecretNames) != 0 {
		t.Fatalf("a session without secrets = %#v / %v", plain.SecretNames, err)
	}
	// A turn records a steer's name union with it; nil leaves the names alone.
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginAgentSessionTurn(ctx, row.ID, "more", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, []string{"LATER", "STRIPE_KEY"}); err != nil {
		t.Fatal(err)
	}
	read, err := s.GetAgentSession(ctx, row.ID)
	if err != nil || !slices.Equal(read.SecretNames, []string{"LATER", "STRIPE_KEY"}) {
		t.Fatalf("names after set = %v / %v", read.SecretNames, err)
	}
	found, _, err := s.FindAgentSessionByIdempotencyKey(ctx, tenant.ID, "k1")
	if err != nil || !slices.Equal(found.SecretNames, read.SecretNames) {
		t.Fatalf("idempotency lookup names = %v / %v", found.SecretNames, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	again, err := s.BeginAgentSessionTurn(ctx, row.ID, "again", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil)
	if err != nil || !slices.Equal(again.SecretNames, read.SecretNames) {
		t.Fatalf("names after a turn without secrets = %v / %v", again.SecretNames, err)
	}
	// A refused turn attaches nothing.
	if _, err := s.BeginAgentSessionTurn(ctx, row.ID, "refused", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, []string{"NEVER"}); !errors.Is(err, ErrAgentSessionTurnState) {
		t.Fatalf("turn while one is in flight = %v", err)
	}
	if read, _ := s.GetAgentSession(ctx, row.ID); slices.Contains(read.SecretNames, "NEVER") {
		t.Fatalf("a refused turn attached names: %v", read.SecretNames)
	}
}
