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

package github

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// claimSvc builds a claim-ready service: configured store+client, a fake
// verifier resolving `claimable`, and the standard test caller as an admin.
func claimSvc(claimable []Installation) (*Service, *fakeVerifier) {
	fv := &fakeVerifier{claimable: claimable}
	svc := &Service{
		Base:        &core.Base{Namespace: "default"},
		GitHub:      &fakeClient{login: "octo"},
		Store:       newFakeStore(),
		Verifier:    fv,
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	return svc, fv
}

// TestStartClaimMintsTransactionAndAuthorizeURL: the claim URL is the OAuth
// authorize endpoint (the state-preserving flow, ADR078 §3a) carrying a signed
// state whose nonce names a server-side transaction for the target workspace.
func TestStartClaimMintsTransactionAndAuthorizeURL(t *testing.T) {
	svc, _ := claimSvc(nil)
	claim, err := svc.StartClaim(testCallerCtx(), "", 0)
	if err != nil {
		t.Fatalf("StartClaim: %v", err)
	}
	if !strings.HasPrefix(claim.ClaimURL, "https://github.example/login/oauth/authorize?client_id=test-client&state=") {
		t.Fatalf("claim URL = %q, want the authorize endpoint with state", claim.ClaimURL)
	}
	if len(svc.Store.(*fakeStore).txns) != 1 {
		t.Fatalf("want exactly one connect transaction minted, got %d", len(svc.Store.(*fakeStore).txns))
	}
}

// TestStartVerbsRefuseWithoutVerifier is ADR078 §7: with no verifier, connect
// and claim starts refuse immediately and mint NO transaction — never an
// install/claim URL whose callback is doomed to 503.
func TestStartVerbsRefuseWithoutVerifier(t *testing.T) {
	svc := &Service{
		Base:        &core.Base{Namespace: "default"},
		GitHub:      &fakeClient{login: "octo"},
		Store:       newFakeStore(),
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	if _, err := svc.StartConnect(testCallerCtx(), ""); !errors.Is(err, core.ErrGitHubUnavailable) {
		t.Errorf("StartConnect without verifier = %v, want ErrGitHubUnavailable", err)
	}
	if _, err := svc.StartClaim(testCallerCtx(), "", 0); !errors.Is(err, core.ErrGitHubUnavailable) {
		t.Errorf("StartClaim without verifier = %v, want ErrGitHubUnavailable", err)
	}
	if n := len(svc.Store.(*fakeStore).txns); n != 0 {
		t.Errorf("refused starts must mint no transaction, got %d", n)
	}
}

// TestClaimBindsSoleUnboundAdminInstallation: exactly one unbound admin
// candidate binds to the TRANSACTION's workspace; the code reaches the resolver.
func TestClaimBindsSoleUnboundAdminInstallation(t *testing.T) {
	svc, fv := claimSvc([]Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}})
	nonce := seedConnectTxn(t, svc, "tea-personal", testCallerSubject)

	conn, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "oauth-code")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !conn.Connected || conn.InstallationID != 42 {
		t.Fatalf("claimed view = %+v, want installation 42 connected", conn)
	}
	if fv.gotClaimCode != "oauth-code" {
		t.Errorf("resolver saw code %q, want oauth-code", fv.gotClaimCode)
	}
	got, _ := svc.Store.(*fakeStore).firstFor("tea-personal")
	if got.InstallationID != 42 {
		t.Fatalf("bound row = %+v, want installation 42 in tea-personal", got)
	}
	if _, ok := svc.Store.(*fakeStore).txns[nonce]; ok {
		t.Error("claim must consume the nonce")
	}
}

// TestClaimResolutionMatrix covers the ADR078 §3a candidate rule AFTER the
// w2/m162 N:N reversal: being bound to another workspace no longer disqualifies
// an installation, ambiguity becomes a deferred choice instead of a dead end, and
// a start-time selector narrows the server-proved set without widening it.
func TestClaimResolutionMatrix(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		svc, _ := claimSvc(nil)
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		if _, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c"); !errors.Is(err, errNoClaimableInstallation) {
			t.Fatalf("zero candidates err = %v, want errNoClaimableInstallation", err)
		}
	})
	t.Run("several defer to a selection", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{
			{ID: 1, AccountLogin: "a", AccountType: "User"},
			{ID: 2, AccountLogin: "b", AccountType: "Organization"},
		})
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		_, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		var pending *claimSelectionRequiredError
		if !errors.As(err, &pending) {
			t.Fatalf("several candidates err = %v, want a pending selection", err)
		}
		st := svc.Store.(*fakeStore)
		if len(st.conns) != 0 {
			t.Error("an ambiguous claim must bind nothing until the human chooses")
		}
		sel, ok := st.selections[pending.SelectionID]
		if !ok {
			t.Fatalf("selection %q was not recorded", pending.SelectionID)
		}
		if len(sel.Candidates) != 2 || sel.WorkspaceID != "tea-a" || sel.Subject != testCallerSubject {
			t.Fatalf("selection = %+v, want both candidates bound to tea-a/%s", sel, testCallerSubject)
		}
	})
	// THE REVERSAL (ADR078 §2): an installation another workspace already holds is
	// a candidate now. Before w2/m162 it was filtered out and the user was told to
	// "install the App first" — the reported dead end.
	t.Run("bound elsewhere is claimable", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{{ID: 7, AccountLogin: "octo", AccountType: "Organization"}})
		st := svc.Store.(*fakeStore)
		st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-other", InstallationID: 7, AccountLogin: "octo"})
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		conn, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		if err != nil {
			t.Fatalf("claiming an installation bound elsewhere = %v, want success", err)
		}
		if conn.InstallationID != 7 {
			t.Fatalf("bound installation = %d, want 7", conn.InstallationID)
		}
		got := st.workspacesFor(7)
		if len(got) != 2 {
			t.Fatalf("bindings for 7 = %v, want both tea-other and tea-a", got)
		}
		// The other workspace's binding is untouched, not transferred.
		foreign := false
		for _, ws := range got {
			if ws == "tea-other" {
				foreign = true
			}
		}
		if !foreign {
			t.Error("the pre-existing binding must survive, not be transferred")
		}
	})
	t.Run("idempotent re-claim of own binding", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}})
		st := svc.Store.(*fakeStore)
		st.conns = append(st.conns, store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 42, AccountLogin: "puncsky"})
		nonce := seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject)
		conn, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		if err != nil || conn.InstallationID != 42 {
			t.Fatalf("re-claim of own binding = %+v, %v; want idempotent success", conn, err)
		}
		if n, _ := st.CountGitConnections(context.Background(), core.DefaultTenant); n != 1 {
			t.Fatalf("re-claim duplicated the row: count=%d", n)
		}
	})
	t.Run("start-time selector narrows to one", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{
			{ID: 1, AccountLogin: "a", AccountType: "User"},
			{ID: 2, AccountLogin: "b", AccountType: "Organization"},
		})
		nonce := seedConnectTxnFor(t, svc, "tea-a", testCallerSubject, 2)
		conn, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		if err != nil {
			t.Fatalf("narrowed claim = %v, want success", err)
		}
		if conn.InstallationID != 2 {
			t.Fatalf("bound installation = %d, want the named 2", conn.InstallationID)
		}
	})
	// The selector can only INTERSECT the server-proved set. Naming an
	// installation the OAuth user does not administer must resolve to nothing, not
	// bind it.
	t.Run("start-time selector cannot widen", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{{ID: 1, AccountLogin: "a", AccountType: "User"}})
		nonce := seedConnectTxnFor(t, svc, "tea-a", testCallerSubject, 999)
		if _, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c"); !errors.Is(err, errNoClaimableInstallation) {
			t.Fatalf("selector naming an unadministered installation = %v, want errNoClaimableInstallation", err)
		}
		if len(svc.Store.(*fakeStore).conns) != 0 {
			t.Fatal("nothing may be bound for an installation the user does not administer")
		}
	})
}

// TestClaimSelectionGrantsNoNewAuthority is the security contract of the
// deferred picker (ADR078 §3a): the selection row is a memo of a completed proof,
// so it must be single-use, subject-bound, and CLOSED to ids outside the set.
func TestClaimSelectionGrantsNoNewAuthority(t *testing.T) {
	// offer runs an ambiguous claim and returns the pending selection id.
	offer := func(t *testing.T) (*Service, string) {
		t.Helper()
		svc, _ := claimSvc([]Installation{
			{ID: 1, AccountLogin: "a", AccountType: "User"},
			{ID: 2, AccountLogin: "b", AccountType: "Organization"},
		})
		nonce := seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject)
		_, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		var pending *claimSelectionRequiredError
		if !errors.As(err, &pending) {
			t.Fatalf("setup: want a pending selection, got %v", err)
		}
		return svc, pending.SelectionID
	}

	t.Run("choosing binds one option", func(t *testing.T) {
		svc, selID := offer(t)
		conn, err := svc.SelectClaim(testCallerCtx(), "", selID, 2)
		if err != nil {
			t.Fatalf("SelectClaim: %v", err)
		}
		if conn.InstallationID != 2 {
			t.Fatalf("bound = %d, want the chosen 2", conn.InstallationID)
		}
		if n, _ := svc.Store.(*fakeStore).CountGitConnections(context.Background(), core.DefaultTenant); n != 1 {
			t.Fatalf("connections = %d, want exactly the one chosen", n)
		}
	})
	t.Run("replay binds nothing", func(t *testing.T) {
		svc, selID := offer(t)
		if _, err := svc.SelectClaim(testCallerCtx(), "", selID, 2); err != nil {
			t.Fatalf("first select: %v", err)
		}
		if _, err := svc.SelectClaim(testCallerCtx(), "", selID, 1); !errors.Is(err, errClaimSelectionGone) {
			t.Fatalf("replayed selection = %v, want errClaimSelectionGone", err)
		}
		if n, _ := svc.Store.(*fakeStore).CountGitConnections(context.Background(), core.DefaultTenant); n != 1 {
			t.Fatalf("a replay must bind nothing more: count=%d", n)
		}
	})
	t.Run("id outside the proved set is refused", func(t *testing.T) {
		svc, selID := offer(t)
		if _, err := svc.SelectClaim(testCallerCtx(), "", selID, 999); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("out-of-set selection = %v, want ErrBadRequest", err)
		}
		if n := len(svc.Store.(*fakeStore).conns); n != 0 {
			t.Fatalf("an out-of-set choice must bind nothing: count=%d", n)
		}
	})
	t.Run("foreign subject is refused on read and on select", func(t *testing.T) {
		svc, selID := offer(t)
		other := core.WithIdentity(context.Background(), core.Identity{Subject: "someone-else", Method: "session"})
		if _, err := svc.GetClaimSelection(other, "", selID); !errors.Is(err, errClaimSelectionGone) {
			t.Errorf("foreign read = %v, want errClaimSelectionGone", err)
		}
		if _, err := svc.SelectClaim(other, "", selID, 2); !errors.Is(err, errClaimSelectionGone) {
			t.Errorf("foreign select = %v, want errClaimSelectionGone", err)
		}
		if n := len(svc.Store.(*fakeStore).conns); n != 0 {
			t.Fatalf("a foreign select must bind nothing: count=%d", n)
		}
	})
	t.Run("anonymous caller is refused", func(t *testing.T) {
		svc, selID := offer(t)
		if _, err := svc.SelectClaim(context.Background(), "", selID, 2); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("anonymous select = %v, want Forbidden", err)
		}
	})
	t.Run("the picker shows only the proved options", func(t *testing.T) {
		svc, selID := offer(t)
		sel, err := svc.GetClaimSelection(testCallerCtx(), "", selID)
		if err != nil {
			t.Fatalf("GetClaimSelection: %v", err)
		}
		if sel.ID != selID || len(sel.Candidates) != 2 {
			t.Fatalf("selection view = %+v, want the two proved candidates", sel)
		}
		// Reading must NOT spend it — only choosing does.
		if _, err := svc.SelectClaim(testCallerCtx(), "", selID, 1); err != nil {
			t.Fatalf("select after read: %v", err)
		}
	})
}

// TestClaimProofsEnforced: the claim path keeps every connect-flow proof — the
// nonce is single-use and subject-bound, and an anonymous or mismatched caller
// is refused before any GitHub exchange.
func TestClaimProofsEnforced(t *testing.T) {
	svc, fv := claimSvc([]Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}})

	// Unknown nonce.
	if _, err := svc.claimFromCallback(context.Background(), "nope", testCallerSubject, "c"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unknown nonce err = %v, want Forbidden", err)
	}
	// Anonymous caller.
	nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
	if _, err := svc.claimFromCallback(context.Background(), nonce, "", "c"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("anonymous caller err = %v, want Forbidden", err)
	}
	// Different subject consumes the nonce and is refused — and the consumed
	// nonce cannot be replayed by the rightful subject either.
	nonce = seedConnectTxn(t, svc, "tea-a", testCallerSubject)
	if _, err := svc.claimFromCallback(context.Background(), nonce, "someone-else", "c"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("subject mismatch err = %v, want Forbidden", err)
	}
	if _, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("replayed nonce err = %v, want Forbidden", err)
	}
	// No GitHub exchange happened on any refused path.
	if fv.gotClaimCode != "" {
		t.Errorf("resolver was reached on a refused path (code %q)", fv.gotClaimCode)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Error("refused claims must bind nothing")
	}
}
