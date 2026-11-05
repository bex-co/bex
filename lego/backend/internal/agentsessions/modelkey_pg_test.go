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
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

type oneSession struct{ session store.AgentSession }

func (o oneSession) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	if id != o.session.ID {
		return store.AgentSession{}, store.ErrNotFound
	}
	return o.session, nil
}

// ADR091 D7 against the real control-plane store: the key is write-only on
// REST, GraphQL and MCP, the persisted audit rows name the model-key target
// without the value, non-admins are refused, and the recorded flag decides
// the mint: the BYO key with no pinned model (so nothing is metered or
// capped), the platform key again once cleared.
func TestModelKeyPG(t *testing.T) {
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
	tenant, err := pg.CreateTenant(ctx, "agent-model-key-pg", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ws := tenant.ID
	t.Cleanup(func() { _ = pg.DeleteTenant(ctx, ws) })
	enablePlatformRoute(t, ws)

	keys := &fakeModelKeys{}
	config, _ := json.Marshal(map[string]string{"agent": "claude", "modelEndpoint": "https://openrouter.ai/api"})
	minter := &agentsession.ModelMinter{
		Keys: keys, Spend: pg, Audit: pg,
		Sessions: oneSession{store.AgentSession{ID: "ags-pg-one", WorkspaceID: ws, SandboxID: "sbx-1", Phase: PhaseRunning, AgentConfig: config}},
	}
	svc := environmentService(ws, pg)
	svc.Audit = pg
	svc.ModelKeys = keys
	svc.Spend = pg
	svc.ModelKeyChanged = minter.ForgetSpend
	keySet := func() bool {
		row, err := pg.GetAgentEnvironment(ctx, ws)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			t.Fatal(err)
		}
		return row.ModelKeySet()
	}
	auditDump := func() string {
		var trail string
		if err := pool.QueryRow(ctx, `SELECT coalesce(json_agg(a)::text, '') FROM audit_events a WHERE workspace_id = $1`, ws).Scan(&trail); err != nil {
			t.Fatal(err)
		}
		return trail
	}

	exerciseModelKeyWriteOnly(t, svc, ws, keys, auditDump, "PLATFORM")

	for _, subject := range []string{"dev", "viewer", "stranger"} {
		if _, err := svc.UpdateEnvironment(caller(subject), ws, EnvironmentInput{ModelAPIKey: ptr(restModelKey)}); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("%s key write = %v, want forbidden", subject, err)
		}
	}
	if keySet() {
		t.Fatal("flag set after refused writes")
	}
	assertNoModelKey(t, "the persisted audit trail", auditDump())

	// Set again; a later partial update keeps the flag.
	if _, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{ModelAPIKey: ptr(restModelKey)}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{NetworkTier: ptr(NetworkTierCustom)})
	if err != nil || !got.ModelKeySet || got.ModelSource != ModelSourceBYO || got.ModelPinned {
		t.Fatalf("partial update after set = %+v err=%v", got, err)
	}

	req := agentsession.ModelMintRequest{SessionID: "ags-pg-one", Workspace: ws, PodName: "sbx-1-0", PodUID: "uid-one"}
	mint, err := minter.Mint(ctx, req)
	if err != nil {
		t.Fatalf("BYO mint: %v", err)
	}
	if mint.Credential != restModelKey || mint.EndpointHost != "api.anthropic.com" || mint.PinnedModel != "" {
		t.Fatalf("BYO mint = host %q pin %q byo=%v", mint.EndpointHost, mint.PinnedModel, mint.Credential == restModelKey)
	}
	var costRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM usage_hourly WHERE workspace_id = $1 AND resource_kind = 'agent_session'`, ws).Scan(&costRows); err != nil {
		t.Fatal(err)
	}
	if costRows != 0 {
		t.Fatalf("a BYO workspace has %d agent-model cost rows", costRows)
	}

	// Cleared: the platform key, pinned (and therefore metered) again.
	if _, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{ClearModelAPIKey: true}); err != nil {
		t.Fatal(err)
	}
	if keySet() {
		t.Fatal("flag set after clear")
	}
	// The clear dropped this replica's cached BYO status, so the very next
	// mint is back on the platform key.
	mint, err = minter.Mint(ctx, req)
	if err != nil || mint.Credential != platformKey || mint.PinnedModel == "" {
		t.Fatalf("platform mint after clear = pin %q platformKey=%v err=%v", mint.PinnedModel, mint.Credential == platformKey, err)
	}

	// An OpenBao failure rolls the flag back with it.
	keys.err = errors.New("openbao down")
	if _, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{ModelAPIKey: ptr(restModelKey)}); err == nil || keySet() {
		t.Fatalf("set with OpenBao down = %v flag=%v", err, keySet())
	}
	keys.err = nil

	// Two admins' concurrent writes serialize on the environment row lock:
	// the second write's OpenBao Put starts only after the first committed.
	gated := &gatedKV{fakeModelKeys: keys, entered: make(chan string), release: make(chan struct{})}
	svc.ModelKeys = gated
	done := make(chan error, 2)
	write := func(key string) {
		_, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{ModelAPIKey: ptr(key)})
		done <- err
	}
	go write(restModelKey)
	if first := <-gated.entered; first != restModelKey {
		t.Fatalf("first Put = %q", first)
	}
	go write(gqlModelKey)
	select {
	case second := <-gated.entered:
		t.Fatalf("a second write (%q) reached OpenBao while the first held the row lock", second)
	case <-time.After(300 * time.Millisecond):
	}
	gated.release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if second := <-gated.entered; second != gqlModelKey {
		t.Fatalf("second Put = %q", second)
	}
	gated.release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if keys.data[agentsession.ModelKeySecretPath(ws)][agentsession.ModelKeyField] != gqlModelKey || !keySet() {
		t.Fatal("serialized writes did not leave the last key flagged")
	}

	// The startup backfill marker sets a missing flag, never clears one, and
	// skips a workspace that no longer exists.
	if _, err := svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{ClearModelAPIKey: true}); err != nil {
		t.Fatal(err)
	}
	if marked, err := pg.MarkAgentModelKeySet(ctx, ws); err != nil || !marked || !keySet() {
		t.Fatalf("mark unflagged = %v err=%v", marked, err)
	}
	if marked, err := pg.MarkAgentModelKeySet(ctx, ws); err != nil || marked || !keySet() {
		t.Fatalf("mark flagged = %v err=%v, want unchanged", marked, err)
	}
	if _, err := pg.MarkAgentModelKeySet(ctx, "tea-d98210cbbpdc73dcrkv0"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mark a missing workspace = %v, want ErrNotFound", err)
	}
}

// gatedKV holds each Put until released, announcing the key it carries.
type gatedKV struct {
	*fakeModelKeys
	mu      sync.Mutex
	entered chan string
	release chan struct{}
}

func (g *gatedKV) Put(ctx context.Context, path string, data map[string]string) error {
	g.entered <- data[agentsession.ModelKeyField]
	<-g.release
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.fakeModelKeys.Put(ctx, path, data)
}
