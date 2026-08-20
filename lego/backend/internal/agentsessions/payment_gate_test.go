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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rejectingPaymentGate refuses every workspace, recording the ids consulted.
// ADR075 D7 (w6/m42): every agent-session verb that dispatches fresh sandbox
// compute must consult the bound-payment-method marker at ADMISSION and 402
// synchronously — the dispatch-time check inside the sandbox lifecycle runs in
// a background goroutine where no error can reach the caller, which is exactly
// the "admitted, then dies async" gap this seam closes.
type rejectingPaymentGate struct{ calls []string }

func (g *rejectingPaymentGate) RequirePaymentMethod(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	return core.NewPaymentRequiredError()
}

func TestCreateRequiresPaymentMethodAtAdmission(t *testing.T) {
	svc, st, _, lc := fixture()
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	_, err := svc.Create(caller("alice"), createInput())
	if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
		t.Fatalf("cardless create err=%v calls=%v, want synchronous 402 for tea-a", err, gate.calls)
	}
	if lc.created != 0 {
		t.Fatalf("sandbox provisioned despite payment refusal (created=%d)", lc.created)
	}
	if len(st.rows) != 0 {
		t.Fatalf("a session row was minted despite the admission refusal: rows=%d", len(st.rows))
	}
}

func TestSteerRequiresPaymentMethodAtAdmission(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	beforeCreated := lc.created
	_, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"})
	if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
		t.Fatalf("cardless steer err=%v calls=%v, want synchronous 402", err, gate.calls)
	}
	if lc.created != beforeCreated {
		t.Fatalf("steer dispatched a sandbox despite payment refusal")
	}
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("refused steer advanced phase to %q", st.rows[id].Phase)
	}
}

func TestRehydrateRequiresPaymentMethodAtAdmission(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	seedHibernated(st, id)
	svc.Snapshots = newFakeSnapshots()
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	beforeCreated := lc.created
	_, err := svc.Resume(caller("alice"), id)
	if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
		t.Fatalf("cardless rehydrate err=%v calls=%v, want synchronous 402", err, gate.calls)
	}
	if lc.created != beforeCreated {
		t.Fatalf("rehydrate dispatched a sandbox despite payment refusal")
	}
	if st.rows[id].Phase != PhaseHibernated {
		t.Fatalf("refused rehydrate advanced phase to %q", st.rows[id].Phase)
	}
}
