package agentsessions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestIdempotentCreateReplaysTheFirstSession(t *testing.T) {
	svc, st, _, lc := fixture()
	req := createInput()
	req.IdempotencyKey = "retry-1"

	first, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Created == nil || !*first.Created {
		t.Fatalf("first create Created = %v, want true", first.Created)
	}
	sandboxesAfterFirst := lc.created

	second, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Created == nil || *second.Created {
		t.Fatalf("retry = %+v, want the first session with created=false", second)
	}
	if lc.created != sandboxesAfterFirst || len(st.rows) != 1 {
		t.Fatalf("retry provisioned again: sandboxes %d→%d rows=%d", sandboxesAfterFirst, lc.created, len(st.rows))
	}
}

func TestIdempotentCreateWithADifferentRequestIsRefused(t *testing.T) {
	svc, st, _, _ := fixture()
	req := createInput()
	req.IdempotencyKey = "k"
	if _, err := svc.Create(caller("alice"), req); err != nil {
		t.Fatal(err)
	}
	req.AgentConfig.Task = "a different task"
	_, err := svc.Create(caller("alice"), req)
	if !isCode(err, "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("different request, same key = %v, want AGENT_SESSION_IDEMPOTENCY_KEY_REUSED", err)
	}
	if len(st.rows) != 1 {
		t.Fatalf("a refused retry created a session: rows=%d", len(st.rows))
	}
}

func TestIdempotencyKeyIsScopedToTheWorkspaceAndExpires(t *testing.T) {
	svc, st, _, _ := fixture()
	req := createInput()
	req.IdempotencyKey = "shared"
	first, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	// The same key in another workspace is a different key.
	other := createInput()
	other.OwnerID, other.IdempotencyKey = "tea-b", "shared"
	got, err := svc.Create(caller("bob"), other)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == first.ID || *got.Created != true {
		t.Fatalf("cross-workspace key replayed %s", got.ID)
	}
	// After the 24 h window the key no longer names the old session.
	st.now = st.now.Add(25 * time.Hour)
	again, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID == first.ID || !*again.Created {
		t.Fatalf("expired key replayed the old session %s", again.ID)
	}
}

func TestIdempotencyKeyIsValidated(t *testing.T) {
	svc, _, _, _ := fixture()
	for name, key := range map[string]string{
		"too long": strings.Repeat("k", 129),
		"space":    "has space",
		"control":  "a\x01b",
		"unicode":  "café",
	} {
		req := createInput()
		req.IdempotencyKey = key
		if _, err := svc.Create(caller("alice"), req); !isCode(err, "AGENT_SESSION_IDEMPOTENCY_KEY_INVALID") {
			t.Errorf("%s: err = %v, want AGENT_SESSION_IDEMPOTENCY_KEY_INVALID", name, err)
		}
	}
}

func TestIdempotentCreateLosingTheRaceReplaysTheWinner(t *testing.T) {
	svc, st, _, _ := fixture()
	req := createInput()
	req.IdempotencyKey = "race"
	winner, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the lookup missing the winner: the create hits the unique key
	// and must fall back to replaying it.
	svc.Store = &blindFirstFind{fakeStore: st}
	got, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != winner.ID || *got.Created {
		t.Fatalf("race loser = %+v, want replay of %s", got, winner.ID)
	}
}

// blindFirstFind answers the first idempotency lookup as if the key were free,
// the way a concurrent create that has not committed yet looks to a rival.
type blindFirstFind struct {
	*fakeStore
	blinded bool
}

func (b *blindFirstFind) FindAgentSessionByIdempotencyKey(ctx context.Context, ws, key string) (store.AgentSession, string, error) {
	if !b.blinded {
		b.blinded = true
		return store.AgentSession{}, "", store.ErrNotFound
	}
	return b.fakeStore.FindAgentSessionByIdempotencyKey(ctx, ws, key)
}

func TestRESTIdempotencyKeyHeaderReplaysWith200(t *testing.T) {
	svc, _, _, _ := fixture()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	post := func(header string) *httptest.ResponseRecorder {
		body := `{"ownerId":"tea-a","repo":"bex-co/example","branch":"bex-agent/session-test","agentConfig":{"agent":"codex","task":"fix the tests"}}`
		r := httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(body)).WithContext(caller("alice"))
		if header != "" {
			r.Header.Set("Idempotency-Key", header)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := post("rest-key"); w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"created":true`) {
		t.Fatalf("first = %d %s", w.Code, w.Body.String())
	}
	if w := post("rest-key"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"created":false`) {
		t.Fatalf("replay = %d %s, want 200 created=false", w.Code, w.Body.String())
	}
}
