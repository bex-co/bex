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

package agentsessions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func continuityFixture(t *testing.T, turns int) (*Service, *fakeStore, store.AgentSession) {
	t.Helper()
	f := newFakeStore()
	record := store.AgentSession{ID: "ags-cont", WorkspaceID: "tea-a", Turns: turns}
	f.rows[record.ID] = record
	f.turns[record.ID] = map[int]store.AgentSessionTurn{}
	for turn := 1; turn <= turns; turn++ {
		f.turns[record.ID][turn] = store.AgentSessionTurn{
			SessionID: record.ID, Turn: turn, Prompt: fmt.Sprintf("prompt %d", turn),
		}
	}
	return &Service{Store: f}, f, record
}

func appendTextParts(t *testing.T, f *fakeStore, sessionID string, turn int, deltas ...string) {
	t.Helper()
	parts := make([]store.AgentSessionTranscriptPart, 0, len(deltas))
	for i, delta := range deltas {
		chunk, err := json.Marshal(map[string]any{"type": "text-delta", "id": "t", "delta": delta})
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, store.AgentSessionTranscriptPart{Turn: turn, PartIndex: int64(i + 1), Part: chunk})
	}
	if err := f.AppendAgentSessionTranscript(context.Background(), sessionID, parts); err != nil {
		t.Fatal(err)
	}
}

// A fresh-generation dispatch of a session with real prior conversation gets
// the rung-2 extract: prior turns' prompts + assembled assistant text, in
// order, excluding the just-accepted turn (ADR047 D3 ladder, w5/m84).
func TestContinuityEnvBuildsTranscriptExtract(t *testing.T) {
	s, f, record := continuityFixture(t, 3)
	appendTextParts(t, f, record.ID, 1, "reply ", "one")
	appendTextParts(t, f, record.ID, 2, "reply two")

	env := map[string]string{}
	s.continuityEnv(context.Background(), env, record, "the task")
	if env[continuityTaskEnv] != "" {
		t.Fatalf("rung 3 fired with a real conversation present: %q", env[continuityTaskEnv])
	}
	var extract []continuityTurn
	if err := json.Unmarshal([]byte(env[continuityContextEnv]), &extract); err != nil {
		t.Fatalf("context env not valid JSON: %v (%q)", err, env[continuityContextEnv])
	}
	if len(extract) != 2 || extract[0].Turn != 1 || extract[1].Turn != 2 {
		t.Fatalf("extract = %+v, want prior turns 1 and 2 in order", extract)
	}
	if extract[0].Prompt != "prompt 1" || extract[0].Reply != "reply one" {
		t.Fatalf("turn 1 extract = %+v", extract[0])
	}
	if extract[1].Reply != "reply two" {
		t.Fatalf("turn 2 extract = %+v", extract[1])
	}
}

// Prior turns whose sandbox never produced a single durable part mean the
// agent never ran (the ags-da9mh5vj596c73en5eq0 setup-failure shape): rung 3
// re-delivers the task instead of shipping an empty context.
func TestContinuityEnvRedeliversTaskWhenAgentNeverRan(t *testing.T) {
	s, _, record := continuityFixture(t, 2)
	env := map[string]string{}
	s.continuityEnv(context.Background(), env, record, "fix the translation")
	if env[continuityContextEnv] != "" {
		t.Fatalf("context injected with no transcript: %q", env[continuityContextEnv])
	}
	if env[continuityTaskEnv] != "fix the translation" {
		t.Fatalf("task env = %q, want the original task", env[continuityTaskEnv])
	}
}

// A first-turn dispatch has nothing to prime — neither env may appear.
func TestContinuityEnvSkipsFirstTurn(t *testing.T) {
	s, _, record := continuityFixture(t, 1)
	env := map[string]string{}
	s.continuityEnv(context.Background(), env, record, "task")
	if len(env) != 0 {
		t.Fatalf("first turn must not be primed: %v", env)
	}
}

// The serialized extract stays under its cap by dropping the OLDEST exchanges;
// the newest prior turn always survives.
func TestContinuityEnvTrimsOldestUnderCap(t *testing.T) {
	s, f, record := continuityFixture(t, 40)
	big := strings.Repeat("x", 4<<10)
	for turn := 1; turn < 40; turn++ {
		appendTextParts(t, f, record.ID, turn, big)
	}
	env := map[string]string{}
	s.continuityEnv(context.Background(), env, record, "task")
	serialized := env[continuityContextEnv]
	if serialized == "" {
		t.Fatal("no context injected")
	}
	if len(serialized) > continuityMaxSerializedBytes {
		t.Fatalf("serialized extract %d bytes exceeds the %d cap", len(serialized), continuityMaxSerializedBytes)
	}
	var extract []continuityTurn
	if err := json.Unmarshal([]byte(serialized), &extract); err != nil {
		t.Fatal(err)
	}
	if extract[len(extract)-1].Turn != 39 {
		t.Fatalf("newest prior turn missing: last = %d", extract[len(extract)-1].Turn)
	}
	if extract[0].Turn == 1 {
		t.Fatal("cap did not trim the oldest exchanges")
	}
}

// One turn's reply extract keeps its TAIL when oversized — the conclusion, not
// the preamble, carries the state the next turn needs.
func TestContinuityReplyKeepsTail(t *testing.T) {
	s, f, record := continuityFixture(t, 2)
	appendTextParts(t, f, record.ID, 1, strings.Repeat("a", continuityMaxReplyBytes), "THE-END")
	env := map[string]string{}
	s.continuityEnv(context.Background(), env, record, "task")
	var extract []continuityTurn
	if err := json.Unmarshal([]byte(env[continuityContextEnv]), &extract); err != nil {
		t.Fatal(err)
	}
	if len(extract) != 1 || !strings.HasSuffix(extract[0].Reply, "THE-END") {
		t.Fatalf("reply tail lost: %q…", extract[0].Reply[:40])
	}
	if len(extract[0].Reply) > continuityMaxReplyBytes {
		t.Fatalf("reply %d bytes exceeds the %d cap", len(extract[0].Reply), continuityMaxReplyBytes)
	}
}
