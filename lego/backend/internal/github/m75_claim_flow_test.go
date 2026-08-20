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
	claim, err := svc.StartClaim(testCallerCtx(), "")
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
	if _, err := svc.StartClaim(testCallerCtx(), ""); !errors.Is(err, core.ErrGitHubUnavailable) {
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

// TestClaimResolutionMatrix: zero candidates and several candidates are the two
// bounded failures; an installation bound to ANOTHER workspace is never a
// candidate; one bound to THIS workspace keeps the claim idempotent.
func TestClaimResolutionMatrix(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		svc, _ := claimSvc(nil)
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		if _, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c"); !errors.Is(err, errNoClaimableInstallation) {
			t.Fatalf("zero candidates err = %v, want errNoClaimableInstallation", err)
		}
	})
	t.Run("several", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{
			{ID: 1, AccountLogin: "a", AccountType: "User"},
			{ID: 2, AccountLogin: "b", AccountType: "Organization"},
		})
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		if _, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c"); !errors.Is(err, errAmbiguousClaim) {
			t.Fatalf("several candidates err = %v, want errAmbiguousClaim", err)
		}
		if len(svc.Store.(*fakeStore).conns) != 0 {
			t.Error("ambiguous claim must bind nothing")
		}
	})
	t.Run("bound elsewhere excluded", func(t *testing.T) {
		svc, _ := claimSvc([]Installation{
			{ID: 7, AccountLogin: "octo", AccountType: "Organization"}, // bound to tea-other below
			{ID: 42, AccountLogin: "puncsky", AccountType: "User"},    // unbound
		})
		st := svc.Store.(*fakeStore)
		st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-other", InstallationID: 7, AccountLogin: "octo"})
		nonce := seedConnectTxn(t, svc, "tea-a", testCallerSubject)
		conn, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
		if err != nil {
			t.Fatalf("claim with one bound-elsewhere + one unbound: %v", err)
		}
		if conn.InstallationID != 42 {
			t.Fatalf("bound installation = %d, want the unbound 42 (never the foreign 7)", conn.InstallationID)
		}
		if owner, _ := st.GitConnectionByInstallation(context.Background(), 7); owner.WorkspaceID != "tea-other" {
			t.Error("the foreign binding must be untouched")
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
