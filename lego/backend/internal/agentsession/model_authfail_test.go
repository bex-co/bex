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

package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// failerStore records the finalize call for the auth-failure verb (w5/m80 t003).
type failerStore struct {
	session   store.AgentSession
	getErr    error
	finErr    error
	finalized bool
	finPhase  string
	finReason string
}

func (f *failerStore) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	if f.getErr != nil {
		return store.AgentSession{}, f.getErr
	}
	if f.session.ID != "" && f.session.ID != id {
		return store.AgentSession{}, store.ErrNotFound
	}
	return f.session, nil
}

func (f *failerStore) FinalizeAgentSession(_ context.Context, _, phase, _, _ string, _ int, _ json.RawMessage, reason string) (store.AgentSession, store.TerminalTurnFact, error) {
	f.finalized = true
	f.finPhase, f.finReason = phase, reason
	if f.finErr != nil {
		return store.AgentSession{}, store.TerminalTurnFact{}, f.finErr
	}
	s := f.session
	s.Phase, s.Status, s.FailureReason = phase, "failed", reason
	return s, store.TerminalTurnFact{Turn: s.Turns, AcceptedAt: s.CreatedAt, TerminalAt: s.UpdatedAt}, nil
}

// A vendor auth rejection on a live, correctly-bound session terminalizes it with
// the actionable reason.
func TestModelAuthFailerTerminalizesLiveSession(t *testing.T) {
	st := &failerStore{session: activeSession()}
	f := &ModelAuthFailer{Sessions: st}
	resp, err := f.Fail(context.Background(), validModelRequest())
	if err != nil || !resp.Acknowledged {
		t.Fatalf("fail = %+v err=%v", resp, err)
	}
	if !st.finalized || st.finPhase != modelFailedPhase || st.finReason != ModelAuthFailureReason {
		t.Fatalf("finalize phase=%q reason=%q finalized=%v", st.finPhase, st.finReason, st.finalized)
	}
}

// A report whose pod triple does not match the session's current sandbox is
// refused — a stale or cross sandbox can never fail another session.
func TestModelAuthFailerRejectsCrossCaller(t *testing.T) {
	st := &failerStore{session: activeSession()}
	f := &ModelAuthFailer{Sessions: st}
	req := validModelRequest()
	req.PodName = "sbx-9-0" // not the session's sbx-1-0
	if _, err := f.Fail(context.Background(), req); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-caller err = %v, want ErrForbidden", err)
	}
	if st.finalized {
		t.Fatal("a cross-caller report must not finalize the session")
	}
}

// The concurrent-race loser (its CAS finds the row already failed → ErrNotFound)
// is acknowledged, not surfaced as an error — the desired end state already holds.
func TestModelAuthFailerAcknowledgesAlreadyFinalized(t *testing.T) {
	st := &failerStore{session: activeSession(), finErr: store.ErrNotFound}
	f := &ModelAuthFailer{Sessions: st}
	resp, err := f.Fail(context.Background(), validModelRequest())
	if err != nil || !resp.Acknowledged {
		t.Fatalf("already-finalized fail = %+v err=%v (want acknowledged, no error)", resp, err)
	}
}

// A missing session (never existed / fully gone) is refused, not acknowledged.
func TestModelAuthFailerRejectsMissingSession(t *testing.T) {
	st := &failerStore{getErr: store.ErrNotFound}
	f := &ModelAuthFailer{Sessions: st}
	if _, err := f.Fail(context.Background(), validModelRequest()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("missing-session err = %v, want ErrForbidden", err)
	}
}
