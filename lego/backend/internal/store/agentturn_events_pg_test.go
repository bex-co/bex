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
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func agentFeed(t *testing.T, s *PGStore, tenantID string) []WebhookEventRow {
	t.Helper()
	rows, err := s.ListWebhookEvents(context.Background(), time.Time{}, "", time.Now().Add(time.Minute), []string{}, []string{tenantID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []WebhookEventRow
	for _, r := range rows {
		if r.Source == EventSourceAgent {
			out = append(out, r)
		}
	}
	return out
}

// ADR094 D9 (w21/m260): every turn that ends succeeded or failed leaves one
// durable event row, whichever path moved the phase, and the webhook feed
// reads it as the session's {turn, phase}. Cancels and live phases leave none.
func TestPGAgentSessionTurnEventsFeedTheWebhookQuery(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "turn-events")
	ctx := context.Background()
	set := func(id, phase string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase=$2 WHERE id=$1`, id, phase); err != nil {
			t.Fatal(err)
		}
	}
	ended, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "one"})
	if err != nil {
		t.Fatal(err)
	}
	set(ended.ID, "running")
	if got := agentFeed(t, s, tenant.ID); len(got) != 0 {
		t.Fatalf("a live phase left events: %+v", got)
	}
	set(ended.ID, "completed")
	set(ended.ID, "completed") // an unchanged phase is not a second event
	if _, err := s.BeginAgentSessionTurn(ctx, ended.ID, "two", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil); err != nil {
		t.Fatal(err)
	}
	set(ended.ID, "failed")

	canceled, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	set(canceled.ID, "canceled")

	got := agentFeed(t, s, tenant.ID)
	if len(got) != 2 {
		t.Fatalf("feed = %+v, want turn 1 completed and turn 2 failed for one session only", got)
	}
	for i, want := range []struct {
		turn  int
		phase string
	}{{1, "completed"}, {2, "failed"}} {
		r := got[i]
		if r.ServiceID != ended.ID || r.Turn != want.turn || r.Phase != want.phase || r.TenantID != tenant.ID ||
			r.Key != "agent:"+ended.ID+":"+string(rune('0'+want.turn))+":"+want.phase {
			t.Errorf("row %d = %+v, want %s turn %d %s", i, r, ended.ID, want.turn, want.phase)
		}
	}
	// Another workspace's subscribers never see it.
	if other := agentFeed(t, s, "tea-nobody"); len(other) != 0 {
		t.Fatalf("another tenant's feed = %+v", other)
	}
	// The events go with the session.
	if _, err := pool.Exec(ctx, `DELETE FROM agent_sessions WHERE id=$1`, ended.ID); err != nil {
		t.Fatal(err)
	}
	if left := agentFeed(t, s, tenant.ID); len(left) != 0 {
		t.Fatalf("events outlived their session: %+v", left)
	}
}

// ADR094 D8: the ceiling round-trips on the session row, and the session's
// spend is the sum of its own recorded customer prices.
func TestPGAgentSessionMaxSpendAndModelSpend(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "max-spend")
	ctx := context.Background()
	ceiling := int64(2_500_000)
	row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", MaxSpendMicroUSD: &ceiling})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "y"})
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.GetAgentSession(ctx, row.ID)
	if err != nil || read.MaxSpendMicroUSD == nil || *read.MaxSpendMicroUSD != ceiling {
		t.Fatalf("stored ceiling = %v / %v", read.MaxSpendMicroUSD, err)
	}
	if plain, err := s.GetAgentSession(ctx, other.ID); err != nil || plain.MaxSpendMicroUSD != nil {
		t.Fatalf("a session without a ceiling = %v / %v", plain.MaxSpendMicroUSD, err)
	}

	if spent, err := s.AgentSessionModelSpendMicroUSD(ctx, tenant.ID, row.ID); err != nil || spent != 0 {
		t.Fatalf("no events: spent = %d / %v, want 0", spent, err)
	}
	record := func(sessionID string, price int64) {
		t.Helper()
		if _, err := s.RecordAgentModelUsage(ctx, AgentModelUsage{
			RequestID: ids.New(ids.AgentModelRequest), WorkspaceID: tenant.ID, SessionID: sessionID,
			Model: "m/a", At: time.Now().UTC(), CostSource: AgentModelCostReported, PriceMicroUSD: price,
		}); err != nil {
			t.Fatal(err)
		}
	}
	record(row.ID, 1_300_001)
	record(row.ID, 1_200_000)
	record(other.ID, 9_000_000)
	if spent, err := s.AgentSessionModelSpendMicroUSD(ctx, tenant.ID, row.ID); err != nil || spent != 2_500_001 {
		t.Fatalf("spent = %d / %v, want this session's 2500001 only", spent, err)
	}
	if spent, _ := s.AgentSessionModelSpendMicroUSD(ctx, "tea-nobody", row.ID); spent != 0 {
		t.Fatalf("another workspace read %d", spent)
	}
}
