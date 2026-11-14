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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// ADR091 D5 acceptance (real DB): a paid (T1) Hobby workspace at $19.99
// month-to-date mints the platform key; at $20.00 the mint answers 429
// AGENT_MODEL_SPEND_CAP_REACHED and the session fails with the readable
// reason and failureReasonCode. Unpaid (T0), the same workspace stops at $10.
func TestPGSpendCapRefusesTheMintAtThePlanCap(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "spend-cap-"+time.Now().Format("150405.000000000"), store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	enablePlatformRoute(t, tenant.ID)
	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "octo/repo", Branch: "bex-agent/task-1",
		AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session, err = pg.SetAgentSessionLifecycle(ctx, session.ID, "sbx-cap", "running", "running", false); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	spend := func(microUSD int64) {
		t.Helper()
		if _, err := pg.RecordAgentModelUsage(ctx, store.AgentModelUsage{
			RequestID: id.New(id.AgentModelRequest), WorkspaceID: tenant.ID, SessionID: session.ID,
			Model: DefaultDevModel, At: now, InputTokens: 1, CostSource: store.AgentModelCostReported,
			UpstreamCostMicroUSD: microUSD, PriceMicroUSD: microUSD,
		}); err != nil {
			t.Fatal(err)
		}
	}
	secret := []byte("spend-cap-test-secret-0123456789")
	mux := http.NewServeMux()
	// The mint caches a workspace's spend status for spendStatusTTL; each
	// step below moves the mint's clock past it (spend stays in the month).
	clock := now
	mux.Handle(InternalModelMintPath, &ModelHandler{Secret: secret, Minter: &ModelMinter{Sessions: pg, Spend: pg, Now: func() time.Time { return clock }}})
	mux.Handle(InternalModelAuthFailurePath, &ModelAuthFailureHandler{Secret: secret, Failer: &ModelAuthFailer{Sessions: pg, Spend: pg}})
	api := httptest.NewServer(mux)
	defer api.Close()
	client := &ModelClient{URL: api.URL + InternalModelMintPath, Secret: secret, HTTP: api.Client()}
	req := ModelMintRequest{SessionID: session.ID, Workspace: tenant.ID, PodName: "sbx-cap-0", PodUID: "uid-cap"}

	// Unpaid and with no charger (billing off): T0 is charged at $10, and
	// without that charge it caps even Hobby at $10 (ADR091 D10.2a).
	spend(10_000_000)
	if _, err := client.Mint(ctx, req); !errors.Is(err, ErrModelSpendCapReached) {
		t.Fatalf("T0 at $10: %v, want the spend cap", err)
	}
	// First successful payment → T1: the Hobby plan cap ($20) applies.
	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: "in_cap_" + tenant.ID, WorkspaceID: tenant.ID, AmountCents: 1000, PaidAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	spend(9_990_000) // $19.99
	clock = clock.Add(spendStatusTTL)
	got, err := client.Mint(ctx, req)
	if err != nil || got.Credential != testPlatformKey {
		t.Fatalf("at $19.99: err=%v platformKey=%v, want the platform key", err, got.Credential == testPlatformKey)
	}
	spend(10_000) // $20.00
	clock = clock.Add(spendStatusTTL)
	if _, err := client.Mint(ctx, req); !errors.Is(err, ErrModelSpendCapReached) {
		t.Fatalf("at $20.00: %v, want ErrModelSpendCapReached", err)
	}
	// The status and header the gateway maps: 429 + the refusal name.
	resp, err := signedPost(ctx, client.URL, secret, time.Now(), api.Client(), ModelMintRequest{
		SessionID: req.SessionID, Workspace: req.Workspace, PodName: req.PodName, PodUID: req.PodUID, Nonce: newNonce(),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get(RefusalHeader) != RefusalModelSpendCapReached {
		t.Fatalf("mint = %d %q, want 429 %s", resp.StatusCode, resp.Header.Get(RefusalHeader), RefusalModelSpendCapReached)
	}

	if err := client.ReportSpendCapReached(ctx, req); err != nil {
		t.Fatalf("report: %v", err)
	}
	failed, err := pg.GetAgentSession(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Phase != modelFailedPhase || failed.FailureReasonCode != ModelSpendCapReachedCode ||
		failed.FailureReason != ModelSpendCapReachedReason || !strings.Contains(failed.FailureReason, "Settings → Agents") {
		t.Fatalf("session = phase %q code %q reason %q", failed.Phase, failed.FailureReasonCode, failed.FailureReason)
	}
}

// ADR091 D10.3 acceptance (real DB): a failed invoice payment refuses the
// next platform-route mint with 429 BILLING_BALANCE_DUE, far under the cap and
// with no grace period, and the session fails with the readable reason and
// failureReasonCode. Paying that invoice lifts the refusal (the workspace
// stays at T0 for 30 days, whose $10 cap still has room).
func TestPGBillingBalanceDueRefusesTheMintUntilPaid(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "balance-due-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	enablePlatformRoute(t, tenant.ID)
	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "octo/repo", Branch: "bex-agent/task-1",
		AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session, err = pg.SetAgentSessionLifecycle(ctx, session.ID, "sbx-due", "running", "running", false); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	secret := []byte("balance-due-test-secret-012345678")
	mux := http.NewServeMux()
	clock := now
	mux.Handle(InternalModelMintPath, &ModelHandler{Secret: secret, Minter: &ModelMinter{Sessions: pg, Spend: pg, Now: func() time.Time { return clock }}})
	mux.Handle(InternalModelAuthFailurePath, &ModelAuthFailureHandler{Secret: secret, Failer: &ModelAuthFailer{Sessions: pg, Spend: pg}})
	api := httptest.NewServer(mux)
	defer api.Close()
	client := &ModelClient{URL: api.URL + InternalModelMintPath, Secret: secret, HTTP: api.Client()}
	req := ModelMintRequest{SessionID: session.ID, Workspace: tenant.ID, PodName: "sbx-due-0", PodUID: "uid-due"}

	// A paying (T1) workspace mints.
	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: "in_due_paid_" + tenant.ID, WorkspaceID: tenant.ID, AmountCents: 5000, PaidAt: now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Mint(ctx, req); err != nil || got.Credential != testPlatformKey {
		t.Fatalf("paying workspace: err=%v, want the platform key", err)
	}
	// Its threshold charge fails: the next mint (past the 20s cache) refuses.
	failed := "in_due_failed_" + tenant.ID
	if _, _, _, err := pg.RecordBillingTrustFailure(ctx, tenant.ID, failed, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(spendStatusTTL)
	if _, err := client.Mint(ctx, req); !errors.Is(err, ErrBillingBalanceDue) {
		t.Fatalf("after a failed invoice: %v, want ErrBillingBalanceDue", err)
	}
	resp, err := signedPost(ctx, client.URL, secret, time.Now(), api.Client(), ModelMintRequest{
		SessionID: req.SessionID, Workspace: req.Workspace, PodName: req.PodName, PodUID: req.PodUID, Nonce: newNonce(),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get(RefusalHeader) != RefusalBillingBalanceDue {
		t.Fatalf("mint = %d %q, want 429 %s", resp.StatusCode, resp.Header.Get(RefusalHeader), RefusalBillingBalanceDue)
	}
	// Paying a different invoice does not lift it.
	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: "in_due_other_" + tenant.ID, WorkspaceID: tenant.ID, AmountCents: 100, PaidAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(spendStatusTTL)
	if _, err := client.Mint(ctx, req); !errors.Is(err, ErrBillingBalanceDue) {
		t.Fatalf("after paying another invoice: %v, want ErrBillingBalanceDue", err)
	}
	// Paying the failed invoice lifts the refusal.
	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: failed, WorkspaceID: tenant.ID, AmountCents: 1000, PaidAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(spendStatusTTL)
	if got, err := client.Mint(ctx, req); err != nil || got.Credential != testPlatformKey {
		t.Fatalf("after paying the failed invoice: err=%v, want the platform key", err)
	}

	if err := client.ReportBillingBalanceDue(ctx, req); err != nil {
		t.Fatalf("report: %v", err)
	}
	ended, err := pg.GetAgentSession(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ended.Phase != modelFailedPhase || ended.FailureReasonCode != BillingBalanceDueCode || ended.FailureReason != BillingBalanceDueReason {
		t.Fatalf("session = phase %q code %q reason %q", ended.Phase, ended.FailureReasonCode, ended.FailureReason)
	}
}
