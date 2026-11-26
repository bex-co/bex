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

package sandbox

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// agent_repeat_test.go is w3/m405. Draining the node of the one OpenSandbox
// lifecycle server failed an agent session's turn with `sandbox create
// failed` (2026-11-12): bex-api now tells a create the runtime did not serve
// from an answer about the sandbox, and a repeated create never leaves a turn
// two sandboxes.

// repeatFixture is an agent-session lifecycle on the stateful fake server.
func repeatFixture(t *testing.T) (*fakeOpenSandbox, *AgentSessionLifecycle, *fakeSessionEgress) {
	t.Helper()
	f, client := newFakeOpenSandbox(t)
	eg := &fakeSessionEgress{}
	svc := &Service{Base: &core.Base{}, Client: client, SessionEgress: eg,
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}, CPU: "2", Memory: "4Gi", SessionOnly: true}}}
	return f, NewAgentSessionLifecycle(svc), eg
}

// turnSpec is session ags-session's create for turn.
func turnSpec(turn string) AgentSessionSandbox {
	return AgentSessionSandbox{WorkspaceID: "tea-a", Template: "agent", SessionID: "ags-session", Repository: "bex-co/example", Branch: "bex-agent/session-test",
		ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: agentsession.ModelKeyPlaceholder("ags-session"),
		DriverEnv: map[string]string{"BEX_AGENT_PROMPT": "do it", "BEX_AGENT_TURN": turn}}
}

// turnBoxes lists the fake's boxes of session's turn, by id.
func (f *fakeOpenSandbox) turnBoxes(session, turn string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id, b := range f.boxes {
		if b.metadata[agentsession.LabelSession] == session && b.metadata[agentsession.LabelDispatchTurn] == turn {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// creates counts the creates the fake served.
func (f *fakeOpenSandbox) creates() int {
	n := 0
	for _, call := range f.lifecycleCalls() {
		if strings.HasPrefix(call, "create ") {
			n++
		}
	}
	return n
}

// A create whose answer was lost after the server made the sandbox is not
// served, and its repeat answers that sandbox instead of making a second one.
func TestARepeatAfterALostAnswerKeepsTheTurnsOneSandbox(t *testing.T) {
	f, lifecycle, eg := repeatFixture(t)
	f.createLost = true
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), turnSpec("2")); !IsTransientCreateFailure(err) {
		t.Fatalf("a create whose connection ended = %v, want a create the runtime did not serve", err)
	}
	made := f.turnBoxes("ags-session", "2")
	if len(made) != 1 {
		t.Fatalf("the lost create left %v, want the one sandbox the server made", made)
	}
	f.createLost = false
	repeat := turnSpec("2")
	repeat.Repeat = true
	got, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), repeat)
	if err != nil {
		t.Fatal(err)
	}
	if left := f.turnBoxes("ags-session", "2"); !slices.Equal(left, made) || f.creates() != 1 {
		t.Fatalf("after the repeat the turn has %v from %d creates, want only %v from one", left, f.creates(), made)
	}
	box := f.box(made[0])
	if want := canonicalID(f.view(made[0], &box)); got.ID != want || got.Status != StatusRunning || got.Workspace != "tea-a" {
		t.Fatalf("the repeat answered %+v, want running sandbox %s", got, want)
	}
	// The repeat sets the session's setup egress up again, as the first
	// attempt did, before it answers.
	if setups := len(eg.calls); setups != 2 || eg.calls[1].op != "setup" || eg.calls[1].session != "ags-session" || eg.calls[1].namespace != "tea-a-sandbox" {
		t.Fatalf("egress calls = %#v, want a setup for each attempt", eg.calls)
	}
}

// A sandbox an earlier attempt left half made, or failed, is ended before the
// repeat creates the turn's sandbox; another turn's and another session's
// sandboxes are left alone.
func TestARepeatEndsAHalfMadeSandboxBeforeItCreates(t *testing.T) {
	for _, state := range []string{"Pending", "Failed"} {
		t.Run(state, func(t *testing.T) {
			f, lifecycle, _ := repeatFixture(t)
			now := time.Now()
			turn := func(session, n string) map[string]string {
				return map[string]string{agentsession.LabelSession: session, agentsession.LabelDispatchTurn: n}
			}
			f.add("half", state, now, "", turn("ags-session", "2"))
			f.add("earlier-turn", "Running", now, "", turn("ags-session", "1"))
			f.add("other-session", "Running", now, "", turn("ags-other", "2"))
			f.add("other-workspace", "Pending", now, "", map[string]string{metadataWorkspace: "tea-b", agentsession.LabelSession: "ags-session", agentsession.LabelDispatchTurn: "2"})
			repeat := turnSpec("2")
			repeat.Repeat = true
			if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), repeat); err != nil {
				t.Fatal(err)
			}
			if got := f.turnBoxes("ags-session", "2"); len(got) != 2 || !slices.Contains(got, "other-workspace") || slices.Contains(got, "half") {
				t.Fatalf("turn 2's sandboxes = %v, want the new one and the other workspace's, and %q ended", got, "half")
			}
			for _, kept := range []string{"earlier-turn", "other-session", "other-workspace"} {
				if f.state(kept) == "" {
					t.Fatalf("the repeat ended %s, which is not this turn's in this workspace", kept)
				}
			}
			calls := f.lifecycleCalls()
			if end, create := slices.Index(calls, "terminate half"), slices.IndexFunc(calls, func(c string) bool { return strings.HasPrefix(c, "create ") }); end < 0 || create < end {
				t.Fatalf("lifecycle calls = %v, want the half-made sandbox ended before the create", calls)
			}
		})
	}
}

// A first create never looks: only a repeat can find what an earlier attempt
// left, so a dispatch's first create costs no list.
func TestAFirstCreateDoesNotLookForAnEarlierAttempt(t *testing.T) {
	f, lifecycle, _ := repeatFixture(t)
	f.add("stale", "Pending", time.Now(), "", map[string]string{agentsession.LabelSession: "ags-session", agentsession.LabelDispatchTurn: "2"})
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), turnSpec("2")); err != nil {
		t.Fatal(err)
	}
	if calls := f.lifecycleCalls(); len(calls) != 1 || !strings.HasPrefix(calls[0], "create ") {
		t.Fatalf("a first create made the calls %v, want the create alone", calls)
	}
}

// A repeat needs the turn to find the earlier attempt by; without one it is
// refused rather than sent as a create that could make a second sandbox.
func TestARepeatWithoutItsTurnIsRefused(t *testing.T) {
	f, lifecycle, _ := repeatFixture(t)
	repeat := turnSpec("")
	repeat.Repeat = true
	if _, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), repeat); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a repeat without its turn = %v, want a bad request", err)
	}
	if f.creates() != 0 {
		t.Fatal("a repeat without its turn was sent")
	}
}

// Which create failures are the runtime not serving it: a refused connection,
// one that ended, any 503, a 502 or 504 the server did not send with its own
// code, and the server's own Kubernetes API failing (a control-plane roll).
// The server's answers about the sandbox, its pod-ready timeout and every
// other status, are not; nor is the caller's own deadline.
func TestWhichCreateFailuresTheRuntimeDidNotServe(t *testing.T) {
	answer := func(status int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
	}
	cases := []struct {
		name      string
		handler   http.HandlerFunc
		transient bool
	}{
		{"503", answer(http.StatusServiceUnavailable, ""), true},
		{"503 tenant lookup down", answer(http.StatusServiceUnavailable, `{"code":"TENANT_PROVIDER_UNAVAILABLE","message":"Tenant authentication service is temporarily unavailable."}`), true},
		{"502 from a hop", answer(http.StatusBadGateway, "upstream connect error"), true},
		{"504 from a hop", answer(http.StatusGatewayTimeout, "upstream request timeout"), true},
		{"connection ended", func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}, true},
		{"504 pod-ready timeout", answer(http.StatusGatewayTimeout, `{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"Timeout waiting for sandbox"}`), false},
		{"500 the server's Kubernetes API failed", answer(http.StatusInternalServerError, `{"code":"KUBERNETES::API_ERROR","message":"Failed to create sandbox: connection refused"}`), true},
		{"502 with the server's code", answer(http.StatusBadGateway, `{"code":"KUBERNETES::API_ERROR","message":"boom"}`), false},
		{"500", answer(http.StatusInternalServerError, "boom"), false},
		{"500 with another code", answer(http.StatusInternalServerError, `{"code":"SANDBOX::INTERNAL_ERROR","message":"boom"}`), false},
		{"400", answer(http.StatusBadRequest, `{"code":"SANDBOX::INVALID_METADATA_LABEL"}`), false},
		{"409", answer(http.StatusConflict, "conflict"), false},
		{"422", answer(http.StatusUnprocessableEntity, "bad entrypoint"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := stubServer(t, tc.handler)
			_, err := svc.Client.Create(context.Background(), "", "node:20", []string{"sh"}, osResourceLimits{CPU: "1", Memory: "1Gi"}, 0, nil, nil)
			if err == nil || IsTransientCreateFailure(err) != tc.transient {
				t.Fatalf("create = %v; transient = %v, want %v", err, IsTransientCreateFailure(err), tc.transient)
			}
		})
	}
	t.Run("refused", func(t *testing.T) {
		_, err := refusingRuntime(t).Client.Create(context.Background(), "", "node:20", []string{"sh"}, osResourceLimits{CPU: "1", Memory: "1Gi"}, 0, nil, nil)
		if !IsTransientCreateFailure(err) {
			t.Fatalf("a refused create = %v, want one the runtime did not serve", err)
		}
	})
	t.Run("the caller's deadline", func(t *testing.T) {
		release := make(chan struct{})
		svc := stubServer(t, func(http.ResponseWriter, *http.Request) { <-release })
		t.Cleanup(func() { close(release) })
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := svc.Client.Create(ctx, "", "node:20", []string{"sh"}, osResourceLimits{CPU: "1", Memory: "1Gi"}, 0, nil, nil)
		if err == nil || IsTransientCreateFailure(err) {
			t.Fatalf("a create past its caller's deadline = %v, want the deadline itself", err)
		}
	})
	// The sandbox answers keep their own classes.
	if IsTransientCreateFailure(sandboxCapacityError()) || IsTransientCreateFailure(sandboxStartError(StartCausePoolFull)) || IsTransientCreateFailure(nil) {
		t.Fatal("a capacity refusal, a start failure or no error read as a create the runtime did not serve")
	}
}
