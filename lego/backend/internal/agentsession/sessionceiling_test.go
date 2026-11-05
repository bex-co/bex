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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// sessionSpend is a SessionSpendStore returning a fixed total.
type sessionSpend struct {
	spent int64
	err   error
	reads int
}

func (s *sessionSpend) AgentSessionModelSpendMicroUSD(context.Context, string, string) (int64, error) {
	s.reads++
	return s.spent, s.err
}

func mintWithCeiling(t *testing.T, ceiling *int64, spend SessionSpendStore) (ModelMintResponse, error) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"agent": "claude"})
	session := store.AgentSession{ID: "ags-one", WorkspaceID: "tea-allow", SandboxID: "sbx-1", Phase: "running", AgentConfig: config, MaxSpendMicroUSD: ceiling}
	minter := &ModelMinter{Keys: &recordingKV{memoryKV: memoryKV{}}, Sessions: fakeSessions{session: session}, Spend: &spendLedger{plan: store.PlanPro}, SessionSpend: spend}
	return minter.Mint(context.Background(), ModelMintRequest{SessionID: "ags-one", Workspace: "tea-allow", PodName: "sbx-1-0", PodUID: "uid-one"})
}

// ADR094 D8: a session with a ceiling is refused once its recorded platform
// price reaches it; below it, and without a ceiling, the mint is unchanged.
func TestMintRefusesAtTheSessionCeiling(t *testing.T) {
	enablePlatformRoute(t, "tea-allow")
	ceiling := int64(5_000_000)

	spend := &sessionSpend{spent: 4_999_999}
	if _, err := mintWithCeiling(t, &ceiling, spend); err != nil || spend.reads != 1 {
		t.Fatalf("below the ceiling: err = %v after %d reads, want an allowed mint", err, spend.reads)
	}
	spend.spent = ceiling
	_, err := mintWithCeiling(t, &ceiling, spend)
	if !errors.Is(err, ErrModelSessionSpendCapReached) || !errors.Is(err, ErrModelSpendCapReached) {
		t.Fatalf("at the ceiling: err = %v, want the session ceiling refusal (also a spend cap refusal)", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Fatal("a ceiling refusal must not be the retried ErrForbidden")
	}

	// No ceiling: the session total is never read.
	spend = &sessionSpend{spent: 1 << 40}
	if _, err := mintWithCeiling(t, nil, spend); err != nil || spend.reads != 0 {
		t.Fatalf("no ceiling: err = %v after %d reads, want an allowed mint and no read", err, spend.reads)
	}
	// A ceiling the minter cannot check fails closed, like an unwired spend store.
	if _, err := mintWithCeiling(t, &ceiling, nil); !errors.Is(err, errSpendStoreUnwired) {
		t.Fatalf("unwired session spend = %v, want the mint failed closed", err)
	}
	if _, err := mintWithCeiling(t, &ceiling, &sessionSpend{err: errors.New("db down")}); err == nil || errors.Is(err, ErrModelSpendCapReached) {
		t.Fatalf("read failure = %v, want a plain error, never a cap refusal", err)
	}
}

// The refusal crosses the gateway→bex-api boundary as a 429 with its own
// refusal header and comes back as the session-scoped error; the monthly cap
// keeps its own header and stays unscoped.
func TestSessionCeilingRefusalSurvivesTheHTTPBoundary(t *testing.T) {
	enablePlatformRoute(t, "tea-allow")
	config, _ := json.Marshal(map[string]string{"agent": "claude"})
	ceiling := int64(1_000_000)
	session := store.AgentSession{ID: "ags-one", WorkspaceID: "tea-allow", SandboxID: "sbx-1", Phase: "running", AgentConfig: config, MaxSpendMicroUSD: &ceiling}
	secret := []byte("gateway-shared-secret")
	ledger := &spendLedger{plan: store.PlanPro}
	minter := &ModelMinter{Keys: &recordingKV{memoryKV: memoryKV{}}, Sessions: fakeSessions{session: session}, Spend: ledger, SessionSpend: &sessionSpend{spent: ceiling}}
	mux := http.NewServeMux()
	mux.Handle(InternalModelMintPath, &ModelHandler{Secret: secret, Minter: minter})
	api := httptest.NewServer(mux)
	defer api.Close()
	client := &ModelClient{URL: api.URL + InternalModelMintPath, Secret: secret, HTTP: api.Client()}
	req := ModelMintRequest{SessionID: "ags-one", Workspace: "tea-allow", PodName: "sbx-1-0", PodUID: "uid-one"}

	_, err := client.Mint(context.Background(), req)
	if !errors.Is(err, ErrModelSessionSpendCapReached) {
		t.Fatalf("session ceiling over HTTP = %v, want ErrModelSessionSpendCapReached", err)
	}
	// The monthly cap takes the unscoped refusal, even with a ceiling set.
	ledger.used = 1 << 40
	minter.ForgetSpend("tea-allow")
	_, err = client.Mint(context.Background(), req)
	if !errors.Is(err, ErrModelSpendCapReached) || errors.Is(err, ErrModelSessionSpendCapReached) {
		t.Fatalf("monthly cap over HTTP = %v, want the unscoped ErrModelSpendCapReached", err)
	}
}

func TestSessionCeilingFailureReasonCarriesTheSharedCode(t *testing.T) {
	reason, code, err := modelFailureReason(ModelFailureCauseSessionSpendCapReached)
	if err != nil || reason != ModelSessionSpendCapReachedReason || code != ModelSpendCapReachedCode {
		t.Fatalf("session ceiling cause = %q %q %v", reason, code, err)
	}
}

// A session stopped at its own ceiling is failed with the session wording
// even when the verb is wired to read the workspace's spend status.
func TestModelAuthFailerFailsTheSessionAtItsCeiling(t *testing.T) {
	st := &failerStore{session: activeSession()}
	f := &ModelAuthFailer{Sessions: st, Spend: &spendLedger{plan: store.PlanPro}}
	req := validModelRequest()
	req.Cause = ModelFailureCauseSessionSpendCapReached
	if resp, err := f.Fail(context.Background(), req); err != nil || !resp.Acknowledged {
		t.Fatalf("fail = %+v err=%v", resp, err)
	}
	if !st.finalized || st.finPhase != modelFailedPhase || st.finReason != ModelSessionSpendCapReachedReason {
		t.Fatalf("finalize phase=%q reason=%q finalized=%v", st.finPhase, st.finReason, st.finalized)
	}
}
