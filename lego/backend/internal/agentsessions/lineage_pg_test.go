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
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// ADR102 against the real store: the service rules for parent and child
// sessions (authorization, limits, spend, secrets, cancel cascade) run on top
// of the store's row locks and recursive queries, with the same in-memory
// authorization, sandbox and secret fakes the other create tests use.

type lineageEnv struct {
	svc  *Service
	fga  *fakeFGA
	lc   *fakeLifecycle
	keys *fakeModelKeys
	pg   *store.PGStore
	ws   string
}

func newLineageEnv(t *testing.T) lineageEnv {
	t.Helper()
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
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	ws, err := pg.CreateTenant(ctx, fmt.Sprintf("agent-lineage-%d", time.Now().UnixNano()), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(ctx, ws.ID) })
	svc, _, fga, lc := fixture()
	svc.Store = pg
	fga.members["alice"] = ws.ID
	svc.ModelKeys = fixtureModelKeys(ws.ID)
	keys := svc.ModelKeys.(*fakeModelKeys)
	return lineageEnv{svc: svc, fga: fga, lc: lc, keys: keys, pg: pg, ws: ws.ID}
}

func (e lineageEnv) input(parentID string) CreateRequest {
	in := createInput()
	in.OwnerID, in.ParentSessionID = e.ws, parentID
	in.Branch = "" // each session gets its own bex-agent/<id>
	return in
}

func (e lineageEnv) create(t *testing.T, parentID string) View {
	t.Helper()
	view, err := e.svc.Create(caller("alice"), e.input(parentID))
	if err != nil {
		t.Fatalf("create under %q: %v", parentID, err)
	}
	return view
}

func (e lineageEnv) phase(t *testing.T, id string) string {
	t.Helper()
	row, err := e.pg.GetAgentSession(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return row.Phase
}

// children lists parentID's direct children in every phase; live counts the
// ones the fan-out limit counts.
func (e lineageEnv) children(t *testing.T, parentID string, phases ...string) []store.AgentSession {
	t.Helper()
	kids, err := e.pg.ListAgentSessions(context.Background(), e.ws, store.AgentSessionListQuery{ParentSessionID: parentID, Phases: phases, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return kids
}

func (e lineageEnv) setPhase(t *testing.T, id, phase string) {
	t.Helper()
	if _, err := e.pg.Pool.Exec(context.Background(), `UPDATE agent_sessions SET phase = $2 WHERE id = $1`, id, phase); err != nil {
		t.Fatal(err)
	}
}

func (e lineageEnv) spend(t *testing.T, sessionID string, micro int64) {
	t.Helper()
	if _, err := e.pg.RecordAgentModelUsage(context.Background(), store.AgentModelUsage{
		RequestID: ids.New(ids.AgentModelRequest), WorkspaceID: e.ws, SessionID: sessionID,
		Model: "m/a", At: time.Now().UTC(), CostSource: store.AgentModelCostReported, PriceMicroUSD: micro,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestChildSessionLineageAuthorizationAndLimits(t *testing.T) {
	e := newLineageEnv(t)
	parent := e.create(t, "")
	if parent.ParentSessionID != "" || parent.RootSessionID != "" || parent.Depth != 0 {
		t.Fatalf("a top-level view has lineage: %+v", parent)
	}
	child := e.create(t, parent.ID)
	if child.ParentSessionID != parent.ID || child.RootSessionID != parent.ID || child.Depth != 1 {
		t.Fatalf("child lineage = %+v", child)
	}
	// The lineage reads back on every view.
	if got, err := e.svc.Get(caller("alice"), child.ID); err != nil || got.ParentSessionID != parent.ID || got.Depth != 1 {
		t.Fatalf("Get(child) = %+v / %v", got, err)
	}
	if kids, err := e.svc.List(caller("alice"), ListRequest{OwnerID: e.ws, ParentSessionID: parent.ID}); err != nil || len(kids) != 1 || kids[0].ID != child.ID {
		t.Fatalf("List by parent = %+v / %v", kids, err)
	}

	// can_operate on the parent is required on top of can_create on the workspace.
	e.fga.members["limited"] = e.ws
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if strings.TrimPrefix(subject, "user:") == "limited" && relation == core.RelCanOperate && strings.HasPrefix(object, "agent_session:") {
			return false, nil
		}
		return e.fga.Check(ctx, subject, relation, object)
	})
	if _, err := e.svc.Create(caller("limited"), e.input(parent.ID)); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a caller who cannot operate the parent = %v, want forbidden", err)
	}
	if _, err := e.svc.Create(caller("limited"), e.input("")); err != nil {
		t.Fatalf("the same caller can still create top-level sessions: %v", err)
	}
	// A parent in another workspace is not reachable.
	e.fga.parents["ags-foreign"] = "tea-elsewhere"
	if _, err := e.svc.Create(caller("alice"), e.input("ags-foreign")); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a foreign parent = %v, want forbidden", err)
	}
	if _, err := e.svc.Create(caller("alice"), e.input("not-a-session-id")); err == nil {
		t.Fatal("a malformed parent id was accepted")
	}
	unknown := ids.New(ids.AgentSession)
	e.fga.parents[unknown] = e.ws
	if _, err := e.svc.Create(caller("alice"), e.input(unknown)); codeOf(err) != ParentNotFoundCode {
		t.Fatalf("an unknown parent = %v, want %s", err, ParentNotFoundCode)
	}

	// A parent that is not working cannot spawn.
	for _, phase := range []string{PhaseCompleted, PhaseFailed, PhaseCanceled} {
		e.setPhase(t, child.ID, phase)
		if _, err := e.svc.Create(caller("alice"), e.input(child.ID)); codeOf(err) != ParentNotActiveCode {
			t.Fatalf("a %s parent = %v, want %s", phase, err, ParentNotActiveCode)
		}
	}
	e.setPhase(t, child.ID, PhaseRunning)

	// Depth: root > child > grandchild > great-grandchild, and no deeper.
	grand := e.create(t, child.ID)
	great := e.create(t, grand.ID)
	if great.Depth != 3 || great.RootSessionID != parent.ID {
		t.Fatalf("depth-3 lineage = %+v", great)
	}
	if _, err := e.svc.Create(caller("alice"), e.input(great.ID)); codeOf(err) != DepthLimitCode {
		t.Fatalf("a fourth level = %v, want %s", err, DepthLimitCode)
	}

	// Fan-out: five live children, no sixth.
	for range store.MaxLiveAgentSessionChildren - 1 {
		e.create(t, parent.ID)
	}
	if _, err := e.svc.Create(caller("alice"), e.input(parent.ID)); codeOf(err) != ChildLimitCode {
		t.Fatalf("a sixth live child = %v, want %s", err, ChildLimitCode)
	}
	e.setPhase(t, child.ID, PhaseCompleted)
	if _, err := e.svc.Create(caller("alice"), e.input(parent.ID)); err != nil {
		t.Fatalf("a finished child still counted against the limit: %v", err)
	}

	// The workspace's live-sandbox limit still applies to children.
	e.svc.MaxLiveSandboxes = 1
	if _, err := e.svc.Create(caller("alice"), e.input(parent.ID)); codeOf(err) != "AGENT_SESSION_LIVE_LIMIT" {
		t.Fatalf("a child at the workspace live limit = %v, want AGENT_SESSION_LIVE_LIMIT", err)
	}
}

func TestChildCreateIsIdempotentWithAParent(t *testing.T) {
	e := newLineageEnv(t)
	parent, other := e.create(t, ""), e.create(t, "")
	in := e.input(parent.ID)
	in.IdempotencyKey = "spawn-1"
	first, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.svc.Create(caller("alice"), in)
	if err != nil || again.ID != first.ID || again.Created == nil || *again.Created {
		t.Fatalf("replay = %+v / %v, want the first child with created=false", again, err)
	}
	if n := len(e.children(t, parent.ID, store.ActiveAgentSessionPhases...)); n != 1 {
		t.Fatalf("a replay made %d children, want 1", n)
	}
	// The parent is part of the request's identity.
	in.ParentSessionID = other.ID
	if _, err := e.svc.Create(caller("alice"), in); codeOf(err) != "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("same key under another parent = %v, want AGENT_SESSION_IDEMPOTENCY_KEY_REUSED", err)
	}
	// A retry after the parent was canceled still returns the first child.
	in.ParentSessionID = parent.ID
	if _, err := e.svc.Cancel(caller("alice"), parent.ID); err != nil {
		t.Fatal(err)
	}
	again, err = e.svc.Create(caller("alice"), in)
	if err != nil || again.ID != first.ID {
		t.Fatalf("replay after the parent was canceled = %+v / %v", again, err)
	}
}

// ADR102 D3: a child's ceiling cannot exceed what is left of its parent's, its
// spend counts in the parent's total, and the model proxy's mint refuses a
// request once the tree has spent the ceiling.
func TestChildSpendCountsAgainstTheParentCeiling(t *testing.T) {
	e := newLineageEnv(t)
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{e.ws}, "sk-platform-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	input := func(parentID string, usd *float64) CreateRequest {
		in := e.input(parentID)
		in.AgentConfig = AgentConfig{Agent: "claude", Task: "split the work"}
		in.MaxSpendUSD = usd
		return in
	}
	mint := func(id string) error {
		row, err := e.pg.GetAgentSession(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if row.SandboxID == "" {
			e.sandbox(t, id)
			row.SandboxID = "sbx-" + id
		}
		minter := &agentsession.ModelMinter{Keys: e.keys, Sessions: e.pg, Spend: e.pg, SessionSpend: e.pg}
		_, err = minter.Mint(context.Background(), agentsession.ModelMintRequest{SessionID: id, Workspace: e.ws, PodName: row.SandboxID + "-0", PodUID: "uid"})
		return err
	}

	parent, err := e.svc.Create(caller("alice"), input("", ptr(5.0)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Create(caller("alice"), input(parent.ID, ptr(6.0))); codeOf(err) != ChildSpendExceedsParentCode {
		t.Fatalf("a child ceiling above the parent's = %v, want %s", err, ChildSpendExceedsParentCode)
	}
	capped, err := e.svc.Create(caller("alice"), input(parent.ID, ptr(3.0)))
	if err != nil {
		t.Fatalf("a child ceiling within the parent's: %v", err)
	}
	free, err := e.svc.Create(caller("alice"), input(parent.ID, nil))
	if err != nil || free.MaxSpendUSD != "" {
		t.Fatalf("a child without a ceiling = %+v / %v", free, err)
	}

	// The child's spend is the parent's: $2 spent by the child leaves $3 under
	// the parent's $5, so a $4 grandchild is refused and a $3 one is not.
	e.spend(t, capped.ID, 2_000_000)
	if _, err := e.svc.Create(caller("alice"), input(capped.ID, ptr(4.0))); codeOf(err) != ChildSpendExceedsParentCode {
		t.Fatalf("a grandchild above what is left = %v", err)
	}
	if _, err := e.svc.Create(caller("alice"), input(parent.ID, ptr(3.0))); err != nil {
		t.Fatalf("a child of exactly what is left: %v", err)
	}
	if err := mint(free.ID); err != nil {
		t.Fatalf("under every ceiling: %v", err)
	}

	// The sibling with no ceiling of its own spends the rest of the parent's $5.
	e.spend(t, free.ID, 3_000_000)
	for _, id := range []string{free.ID, parent.ID, capped.ID} {
		if err := mint(id); !errors.Is(err, agentsession.ErrModelSessionSpendCapReached) {
			t.Fatalf("mint for %s with the tree's ceiling spent = %v, want ErrModelSessionSpendCapReached", id, err)
		}
	}
	if _, err := e.svc.Create(caller("alice"), input(parent.ID, nil)); codeOf(err) != ChildSpendExhaustedCode {
		t.Fatalf("a child of a spent tree = %v, want %s", err, ChildSpendExhaustedCode)
	}
	// A session outside the tree is unaffected.
	unrelated, err := e.svc.Create(caller("alice"), input("", nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := mint(unrelated.ID); err != nil {
		t.Fatalf("an unrelated session: %v", err)
	}
}

// sandbox gives a session a sandbox id and the running phase, as the dispatch
// would, so a mint request from its pod passes the identity check.
func (e lineageEnv) sandbox(t *testing.T, id string) {
	t.Helper()
	if _, err := e.pg.Pool.Exec(context.Background(), `UPDATE agent_sessions SET sandbox_id = 'sbx-' || id, phase = 'running' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
}

// ADR102 D4: a child inherits its parent's secrets, may narrow them, and can
// never take one its parent lacks.
func TestChildSecretsInheritAndNarrowButNeverWiden(t *testing.T) {
	e := newLineageEnv(t)
	e.keys.data[workspaceSecretPath(e.ws, "STRIPE_KEY")] = map[string]string{agentSecretValueField: stripeValue}
	e.keys.data[workspaceSecretPath(e.ws, "OTHER_KEY")] = map[string]string{agentSecretValueField: "other-value"}
	in := e.input("")
	in.SecretNames = []string{"STRIPE_KEY"}
	in.Secrets = []SecretInput{{Name: "TEST_TOKEN", Value: inlineValue}}
	parent, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}

	child := func(names []string, inline []SecretInput) (View, error) {
		in := e.input(parent.ID)
		in.SecretNames, in.Secrets = names, inline
		return e.svc.Create(caller("alice"), in)
	}
	all, err := child(nil, nil)
	if err != nil || !slices.Equal(all.SecretNames, []string{"STRIPE_KEY", "TEST_TOKEN"}) {
		t.Fatalf("an inheriting child = %+v / %v", all.SecretNames, err)
	}
	// The parent's inline value reaches the child's sandbox, never the view.
	if e.lc.driverEnv["TEST_TOKEN"] != inlineValue || e.lc.driverEnv["STRIPE_KEY"] != stripeValue {
		t.Fatalf("the child's sandbox env lacks the inherited secrets: %v", slices.Sorted(keysOf(e.lc.driverEnv)))
	}

	narrow, err := child([]string{"TEST_TOKEN"}, nil)
	if err != nil || !slices.Equal(narrow.SecretNames, []string{"TEST_TOKEN"}) {
		t.Fatalf("a narrowed child = %+v / %v", narrow.SecretNames, err)
	}
	if _, present := e.lc.driverEnv["STRIPE_KEY"]; present {
		t.Fatal("the narrowed child's sandbox still holds STRIPE_KEY")
	}
	none, err := child([]string{}, nil)
	if err != nil || len(none.SecretNames) != 0 {
		t.Fatalf("a child with an empty secretNames = %+v / %v, want none", none.SecretNames, err)
	}

	for name, call := range map[string]func() (View, error){
		"a workspace secret the parent lacks": func() (View, error) { return child([]string{"OTHER_KEY"}, nil) },
		"one it has plus one it lacks":        func() (View, error) { return child([]string{"STRIPE_KEY", "OTHER_KEY"}, nil) },
		"an inline secret":                    func() (View, error) { return child(nil, []SecretInput{{Name: "NEW_TOKEN", Value: "v"}}) },
		"an inline override of an inherited":  func() (View, error) { return child(nil, []SecretInput{{Name: "TEST_TOKEN", Value: "v"}}) },
	} {
		if _, err := call(); codeOf(err) != ChildSecretNotInheritedCode {
			t.Errorf("%s = %v, want %s", name, err, ChildSecretNotInheritedCode)
		}
	}
	// A parent with no secrets passes none on.
	bare := e.create(t, "")
	in = e.input(bare.ID)
	in.SecretNames = []string{"STRIPE_KEY"}
	if _, err := e.svc.Create(caller("alice"), in); codeOf(err) != ChildSecretNotInheritedCode {
		t.Fatalf("a secret under a parent with none = %v", err)
	}
}

func keysOf(m map[string]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// ADR102 D6: canceling a parent cancels every unfinished descendant, deepest
// first, records why, and leaves finished ones alone.
func TestCancelParentCancelsUnfinishedDescendants(t *testing.T) {
	e := newLineageEnv(t)
	parent := e.create(t, "")
	a := e.create(t, parent.ID)
	a1 := e.create(t, a.ID)
	done := e.create(t, parent.ID)
	b := e.create(t, parent.ID)
	unrelated := e.create(t, "")
	e.setPhase(t, done.ID, PhaseCompleted)
	e.setPhase(t, b.ID, PhaseFailed)

	// A sandbox failure on one descendant stops the parent in canceling; the
	// retry converges, including the descendant left canceling.
	e.lc.cancelErr = errors.New("sandbox api down")
	if _, err := e.svc.Cancel(caller("alice"), parent.ID); err == nil {
		t.Fatal("a failed descendant teardown was swallowed")
	}
	if got := e.phase(t, parent.ID); got != PhaseCanceling {
		t.Fatalf("parent phase after a failed cascade = %s, want canceling", got)
	}
	if _, err := e.svc.Create(caller("alice"), e.input(parent.ID)); codeOf(err) != ParentNotActiveCode {
		t.Fatalf("a canceling parent started a child: %v", err)
	}
	e.lc.cancelErr = nil
	view, err := e.svc.Cancel(caller("alice"), parent.ID)
	if err != nil || view.Phase != PhaseCanceled {
		t.Fatalf("retried cancel = %+v / %v", view, err)
	}
	for _, id := range []string{a.ID, a1.ID} {
		row, err := e.pg.GetAgentSession(context.Background(), id)
		if err != nil || row.Phase != PhaseCanceled || row.FailureReasonCode != ParentCanceledCode || !strings.Contains(row.FailureReason, parent.ID) {
			t.Fatalf("descendant %s = phase %s reason %q code %q / %v", id, row.Phase, row.FailureReason, row.FailureReasonCode, err)
		}
	}
	if got := e.phase(t, done.ID); got != PhaseCompleted {
		t.Fatalf("a completed child became %s", got)
	}
	if got := e.phase(t, b.ID); got != PhaseFailed {
		t.Fatalf("a failed child became %s", got)
	}
	if got := e.phase(t, unrelated.ID); got == PhaseCanceled || got == PhaseCanceling {
		t.Fatalf("an unrelated session became %s", got)
	}
	// The user's own cancel records no cause.
	if row, _ := e.pg.GetAgentSession(context.Background(), parent.ID); row.FailureReason != "" {
		t.Fatalf("the canceled parent carries a cause: %q", row.FailureReason)
	}
	// Canceling again is still idempotent.
	if again, err := e.svc.Cancel(caller("alice"), parent.ID); err != nil || again.Phase != PhaseCanceled {
		t.Fatalf("second cancel = %+v / %v", again, err)
	}
}

// w5/186: when a member leaves or is removed, the sessions they started,
// directly or through an API key they created, are canceled with their
// descendants and the cause recorded; their finished sessions and everyone
// else's are left alone. A sandbox outage does not fail the exit: every
// session is already canceling (no longer minting), and a later cancel
// converges it.
func TestCancelMemberSessionsEndsOnlyTheDepartedMembersUnfinishedSessions(t *testing.T) {
	e := newLineageEnv(t)
	ctx := context.Background()
	setCreator := func(id, creator string) {
		t.Helper()
		if _, err := e.pg.Pool.Exec(ctx, `UPDATE agent_sessions SET created_by = $2 WHERE id = $1`, id, creator); err != nil {
			t.Fatal(err)
		}
	}
	running := e.create(t, "")
	child := e.create(t, running.ID)
	byKey := e.create(t, "")
	setCreator(byKey.ID, "key-client-1")
	finished := e.create(t, "")
	e.setPhase(t, finished.ID, PhaseCompleted)
	someoneElses := e.create(t, "")
	setCreator(someoneElses.ID, "bob")
	creators := []string{"alice", "key-client-1"}

	e.lc.cancelErr = errors.New("sandbox api down")
	if err := e.svc.CancelMemberSessions(ctx, e.ws, creators); err != nil {
		t.Fatalf("a sandbox outage failed the exit: %v", err)
	}
	for _, id := range []string{running.ID, child.ID, byKey.ID} {
		if got := e.phase(t, id); got != PhaseCanceling {
			t.Fatalf("session %s after a failed teardown = %s, want canceling (no longer minting)", id, got)
		}
	}
	e.lc.cancelErr = nil
	if err := e.svc.CancelMemberSessions(ctx, e.ws, creators); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	for _, id := range []string{running.ID, byKey.ID} {
		row, err := e.pg.GetAgentSession(ctx, id)
		if err != nil || row.Phase != PhaseCanceled || row.FailureReasonCode != CreatorRemovedCode {
			t.Fatalf("the departed member's session %s = phase %s code %q / %v", id, row.Phase, row.FailureReasonCode, err)
		}
	}
	if got := e.phase(t, child.ID); got != PhaseCanceled {
		t.Fatalf("its child = %s, want canceled", got)
	}
	if got := e.phase(t, finished.ID); got != PhaseCompleted {
		t.Fatalf("a finished session became %s", got)
	}
	if got := e.phase(t, someoneElses.ID); got == PhaseCanceled || got == PhaseCanceling {
		t.Fatalf("another member's session became %s", got)
	}
}
