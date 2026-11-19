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
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// ADR047 D12 (w22/m413 t004): what became of the work of a turn a person
// stopped is on the turn's transcript entry, in the same words on REST,
// GraphQL and MCP, whichever way the disk ended up: kept until a time, being
// saved, continued, lost for a reason, or never kept.

// turnFacts is what one surface says about one turn, normalized so the three
// are compared field by field.
type turnFacts struct {
	Interrupted bool
	Retention   string
	// Until is the RFC 3339 time as sent; "" when null or absent.
	Until      string
	LostReason string
	StartNote  string
}

// retentionFacts reads turn n of the session's transcript through the REST
// route, the GraphQL query and the MCP tool, as alice.
func retentionFacts(t *testing.T, a *lineageAdapters, id string, n int) map[string]turnFacts {
	t.Helper()
	pick := func(turns []any) map[string]any {
		for _, raw := range turns {
			// A JSON number is a float64, a GraphQL Int an int.
			if turn := raw.(map[string]any); fmt.Sprint(turn["turn"]) == fmt.Sprint(n) {
				return turn
			}
		}
		t.Fatalf("no turn %d in %v", n, turns)
		return nil
	}
	str := func(turn map[string]any, key string) string {
		s, _ := turn[key].(string) // absent and null read as ""
		return s
	}
	facts := func(turn map[string]any) turnFacts {
		interrupted, _ := turn["interrupted"].(bool)
		return turnFacts{
			Interrupted: interrupted, Retention: str(turn, "retention"), Until: str(turn, "retainedUntil"),
			LostReason: str(turn, "retentionLostReason"), StartNote: str(turn, "startNote"),
		}
	}
	out := map[string]turnFacts{}

	status, body := a.rest(http.MethodGet, "/v1/agent-sessions/"+id+"/transcript", nil, "alice")
	if status != http.StatusOK {
		t.Fatalf("REST transcript = %d %v", status, body)
	}
	out["REST"] = facts(pick(body["turns"].([]any)))

	data, code := a.gql(`{ agentSessionTranscript(id: "`+id+`") { turns { turn interrupted retention retainedUntil retentionLostReason startNote } } }`, "alice")
	if code != "" {
		t.Fatalf("GraphQL transcript: %s", code)
	}
	gql := pick(data["agentSessionTranscript"].(map[string]any)["turns"].([]any))
	// GraphQL's non-null strings are "" where REST omits the field; its
	// timestamps are the same instant in the schema's own layout.
	out["GraphQL"] = facts(gql)

	mcp, errText := a.mcp("get_agent_session_transcript", map[string]any{"id": id}, "alice")
	if errText != "" {
		t.Fatalf("MCP transcript: %s", errText)
	}
	out["MCP"] = facts(pick(mcp["turns"].([]any)))
	return out
}

// sameInstant reports whether two RFC 3339 strings name one instant (the
// surfaces format a time with their own precision).
func sameInstant(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	return errA == nil && errB == nil && ta.Equal(tb)
}

func TestAStoppedTurnsRetentionIsTheSameOnEverySurface(t *testing.T) {
	type want struct {
		turn int
		// stopped is whether the turn is flagged interrupted.
		stopped   bool
		retention string
		reason    string
		note      string
		// until is how long after the test clock's start the disk is kept
		// until; 0 for none.
		until time.Duration
	}
	cases := []struct {
		name string
		// arrange brings the session to the state under test; the turn read
		// is want.turn.
		arrange func(t *testing.T, e *retainEnv)
		want    want
	}{
		{
			name:    "kept",
			arrange: func(t *testing.T, e *retainEnv) { e.stopped(t) },
			want:    want{turn: 1, stopped: true, retention: RetentionKept, until: retentionTTL},
		},
		{
			name: "being saved",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stop(t)
				entered, finish := e.holdSave()
				e.settle(t)
				<-entered
				t.Cleanup(finish)
			},
			want: want{turn: 1, stopped: true, retention: RetentionSaving},
		},
		{
			name: "continued by the next turn",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				e.send(t, "alice", "carry on")
			},
			want: want{turn: 1, stopped: true, retention: RetentionContinued},
		},
		{
			name: "lost when the retention time ran out, swept",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				e.clock.Advance(retentionTTL + time.Minute)
				e.sweep()
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonExpired},
		},
		{
			// The sweep has not run yet: the answer is not "kept until" a time in
			// the past, which the next message would not honor either.
			name: "lost when the retention time ran out, not yet swept",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				e.clock.Advance(retentionTTL + time.Minute)
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonExpired},
		},
		{
			name: "lost because the storage cap was full",
			arrange: func(t *testing.T, e *retainEnv) {
				e.rs.retainErr = fmt.Errorf("%w: full", sandbox.ErrAgentImageStorageLimit)
				e.stop(t)
				e.settle(t)
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonStorageFull},
		},
		{
			name: "lost because the save failed",
			arrange: func(t *testing.T, e *retainEnv) {
				e.rs.retainErr = fmt.Errorf("%w: the commit did not finish", sandbox.ErrAgentSandboxPaused)
				e.stop(t)
				e.settle(t)
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonSaveFailed},
		},
		{
			name: "lost because the transcript could not be read",
			arrange: func(t *testing.T, e *retainEnv) {
				e.lc.transcriptErr = errors.New("the pod did not answer")
				e.stop(t)
				e.settle(t)
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonSaveFailed},
		},
		{
			name: "lost because an editor was open",
			arrange: func(t *testing.T, e *retainEnv) {
				e.c.Store = sshOpenStore{e.pg}
				e.stop(t)
				e.settle(t)
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonEditorOpen},
		},
		{
			name: "lost because the session was canceled",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
					t.Fatal(err)
				}
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonCanceled},
		},
		{
			name: "lost because the next turn could not start from it",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				e.lc.imageCreateErr = errors.New("ImagePullBackOff")
				e.send(t, "alice", "continue")
			},
			want: want{turn: 1, stopped: true, retention: RetentionLost, reason: RetentionLostReasonNotRestored},
		},
		{
			name: "the turn that started fresh says so",
			arrange: func(t *testing.T, e *retainEnv) {
				e.stopped(t)
				e.lc.imageCreateErr = errors.New("ImagePullBackOff")
				e.send(t, "alice", "continue")
			},
			want: want{turn: 2, retention: RetentionNone, note: notRestoredNote(1)},
		},
		{
			name: "none when retention is off",
			arrange: func(t *testing.T, e *retainEnv) {
				e.r.TTL = 0
				e.stop(t)
				e.settle(t)
			},
			want: want{turn: 1, stopped: true, retention: RetentionNone},
		},
		{
			name: "none for a turn that ended on its own",
			arrange: func(t *testing.T, e *retainEnv) {
				e.lc.status = `{"state":"succeeded"}`
				e.settle(t)
			},
			want: want{turn: 1, retention: RetentionNone},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newRetainEnv(t)
			start := e.clock.Now()
			c.arrange(t, e)
			a := &lineageAdapters{t: t, e: e.lineageEnv, mux: http.NewServeMux()}
			e.svc.RegisterREST(a.mux)

			got := retentionFacts(t, a, e.id, c.want.turn)
			wantUntil := ""
			if c.want.until > 0 {
				wantUntil = start.Add(c.want.until).UTC().Format(time.RFC3339Nano)
			}
			for surface, facts := range got {
				if facts.Retention != c.want.retention || facts.LostReason != c.want.reason || facts.StartNote != c.want.note || facts.Interrupted != c.want.stopped {
					t.Errorf("%s: %+v, want retention %q reason %q note %q interrupted %v", surface, facts, c.want.retention, c.want.reason, c.want.note, c.want.stopped)
				}
				if wantUntil == "" && facts.Until != "" {
					t.Errorf("%s: retainedUntil = %q, want none", surface, facts.Until)
				}
				if wantUntil != "" && !sameInstant(facts.Until, wantUntil) {
					t.Errorf("%s: retainedUntil = %q, want %s", surface, facts.Until, wantUntil)
				}
			}
			if len(got) != 3 {
				t.Fatalf("read %d surfaces, want REST, GraphQL and MCP", len(got))
			}
		})
	}
}
