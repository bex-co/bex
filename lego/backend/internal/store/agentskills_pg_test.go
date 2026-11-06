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
	"strings"
	"sync"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func newSkill(ws, name string) AgentSkill {
	return AgentSkill{ID: ids.New(ids.AgentSkill), WorkspaceID: ws, Name: name, Description: "d",
		Triggers: []string{"user", "model"}, Enabled: true, Body: "b", UpdatedBy: "admin"}
}

// ADR099 D2: workspace skills are unique by name per workspace, capped
// atomically, updated only at their current version, and go with their
// workspace.
func TestPGAgentSkillsCRUD(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-skills")
	ctx := context.Background()
	lim := AgentSkillLimits{Count: 3, EnabledBytes: 1 << 20}
	created, err := s.CreateAgentSkill(ctx, newSkill(tenant.ID, "deploy"), lim)
	if err != nil || created.Version != 1 || created.CreatedAt.IsZero() || !slices.Equal(created.Triggers, []string{"user", "model"}) {
		t.Fatalf("create = %+v err=%v", created, err)
	}
	if _, err := s.CreateAgentSkill(ctx, newSkill(tenant.ID, "deploy"), lim); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name = %v, want ErrConflict", err)
	}
	byName, err := s.GetAgentSkillByName(ctx, tenant.ID, "deploy")
	if err != nil || byName.ID != created.ID {
		t.Fatalf("by name = %+v err=%v", byName, err)
	}
	if _, err := s.GetAgentSkillByName(ctx, "tea-elsewhere", "deploy"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("by name in another workspace = %v", err)
	}

	next := created
	next.Body, next.Triggers, next.Enabled = "new body", []string{"user"}, false
	updated, err := s.UpdateAgentSkill(ctx, next, 1, lim)
	if err != nil || updated.Version != 2 || updated.Body != "new body" || updated.Enabled || !slices.Equal(updated.Triggers, []string{"user"}) {
		t.Fatalf("update = %+v err=%v", updated, err)
	}
	if _, err := s.UpdateAgentSkill(ctx, next, 1, lim); !errors.Is(err, ErrAgentSkillVersion) {
		t.Fatalf("stale version = %v, want ErrAgentSkillVersion", err)
	}
	missing := newSkill(tenant.ID, "ghost")
	if _, err := s.UpdateAgentSkill(ctx, missing, 1, lim); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of a missing skill = %v", err)
	}
	bad := newSkill(tenant.ID, "Bad--Name")
	if _, err := s.CreateAgentSkill(ctx, bad, lim); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a name the CHECK refuses = %v, want ErrInvalid", err)
	}

	// Concurrent creates cannot overshoot the cap.
	var wg sync.WaitGroup
	results := make([]error, 6)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = s.CreateAgentSkill(ctx, newSkill(tenant.ID, "race-"+string(rune('a'+i))), lim)
		}(i)
	}
	wg.Wait()
	list, err := s.ListAgentSkills(ctx, tenant.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("skills after the race = %d err=%v, want the cap of 3", len(list), err)
	}
	limited := 0
	for _, err := range results {
		if errors.Is(err, ErrAgentSkillLimit) {
			limited++
		}
	}
	if limited != 4 {
		t.Fatalf("%d creates refused at the cap, want 4", limited)
	}
	if !slices.IsSortedFunc(list, func(a, b AgentSkill) int { return strings.Compare(a.Name, b.Name) }) {
		t.Fatalf("list is not by name: %+v", list)
	}

	if err := s.DeleteAgentSkill(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAgentSkill(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListAgentSkills(ctx, tenant.ID); len(list) != 0 {
		t.Fatalf("%d skills outlived their workspace", len(list))
	}
}

// ADR099 D2/D5: a list reads metadata only; a dispatch reads the enabled
// skills whole plus the invoked ones even if disabled; and the enabled-bytes
// cap holds under concurrent writes because the sum runs under the
// workspace's skill lock.
func TestPGAgentSkillListDispatchAndEnabledBytes(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-skills-bytes")
	ctx := context.Background()
	big := func(name string, enabled bool) AgentSkill {
		skill := newSkill(tenant.ID, name)
		skill.Body, skill.Enabled = strings.Repeat("x", 600), enabled
		return skill
	}
	// One skill counts AgentSkillBytes; the cap fits one, not two.
	lim := AgentSkillLimits{Count: 10, EnabledBytes: AgentSkillBytes(big("on-a", true)) + 100}
	on, err := s.CreateAgentSkill(ctx, big("on-a", true), lim)
	if err != nil {
		t.Fatal(err)
	}
	off, err := s.CreateAgentSkill(ctx, big("off-b", false), lim)
	if err != nil {
		t.Fatalf("a disabled skill does not count toward the cap: %v", err)
	}
	if _, err := s.CreateAgentSkill(ctx, big("on-c", true), lim); !errors.Is(err, ErrAgentSkillBytes) {
		t.Fatalf("an enabled create over the cap = %v, want ErrAgentSkillBytes", err)
	}
	enable := off
	enable.Enabled = true
	if _, err := s.UpdateAgentSkill(ctx, enable, off.Version, lim); !errors.Is(err, ErrAgentSkillBytes) {
		t.Fatalf("enabling over the cap = %v, want ErrAgentSkillBytes", err)
	}
	// Re-saving the enabled skill counts it once, not twice.
	if _, err := s.UpdateAgentSkill(ctx, on, on.Version, lim); err != nil {
		t.Fatalf("re-saving an enabled skill = %v", err)
	}

	list, err := s.ListAgentSkills(ctx, tenant.ID)
	if err != nil || len(list) != 2 || list[0].Body != "" || list[0].Description == "" {
		t.Fatalf("list = %+v err=%v, want metadata without bodies", list, err)
	}
	dispatch, err := s.AgentSkillsForDispatch(ctx, tenant.ID, nil)
	if err != nil || len(dispatch) != 1 || dispatch[0].Name != "on-a" || dispatch[0].Body == "" {
		t.Fatalf("dispatch without invocations = %+v err=%v, want the enabled skill whole", dispatch, err)
	}
	dispatch, err = s.AgentSkillsForDispatch(ctx, tenant.ID, []string{"off-b"})
	if err != nil || len(dispatch) != 2 {
		t.Fatalf("dispatch invoking a disabled skill = %+v err=%v", dispatch, err)
	}

	// Two enabled creates racing for the last slot under the cap: one wins.
	other, _, otherTenant := agentSessionPGStore(t, "agent-skills-bytes-race")
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			skill := big("race-"+string(rune('a'+i)), true)
			skill.WorkspaceID = otherTenant.ID
			_, results[i] = other.CreateAgentSkill(ctx, skill, lim)
		}(i)
	}
	wg.Wait()
	if (results[0] == nil) == (results[1] == nil) {
		t.Fatalf("racing enabled creates = %v, want exactly one accepted", results)
	}
}

// ADR099 D4: the skills a turn invokes are stored with it, on create and on
// a later turn, and read back in order.
func TestPGAgentSessionTurnSkills(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "turn-skills")
	ctx := context.Background()
	invoked := []TurnSkill{{Name: "release", Arguments: "v1.2.3"}, {Name: "deploy"}}
	row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x", InitialSkills: invoked})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginAgentSessionTurn(ctx, row.ID, "plain", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	turns, err := s.AgentSessionTurns(ctx, row.ID)
	if err != nil || len(turns) != 2 {
		t.Fatalf("turns = %+v err=%v", turns, err)
	}
	if !slices.Equal(turns[0].Skills, invoked) {
		t.Fatalf("turn 1 skills = %+v", turns[0].Skills)
	}
	if turns[1].Skills == nil || len(turns[1].Skills) != 0 {
		t.Fatalf("a turn without skills = %#v, want empty", turns[1].Skills)
	}
}
