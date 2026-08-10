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
	"net/http"
	"net/url"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	attackerSubject  = "identity-attacker"
	attackerWorkspce = "tea-attacker"
	victimSubject    = "identity-victim"
)

// connectService builds a service whose store, GitHub client, and installation
// verifier all succeed, so the ONLY thing that can refuse a callback is the
// binding under test.
func connectService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		Base:        &core.Base{Namespace: "default"},
		GitHub:      &fakeClient{login: "victim-org"},
		Store:       newFakeStore(),
		Verifier:    &fakeVerifier{ok: true}, // the victim really does administer it
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
}

// TestInstallationLoginCSRFIsRefused is the w1/m67 F3 regression, written as the
// attack: the attacker starts a connect for ITS OWN workspace, hands the signed
// install URL to a victim GitHub org admin, and the victim completes a perfectly
// genuine installation. Before the fix both proofs verified — the state was valid
// and the victim really did administer the installation — and the victim's
// repositories were bound to the attacker's workspace.
func TestInstallationLoginCSRFIsRefused(t *testing.T) {
	svc := connectService(t)
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	_, err := svc.connectFromCallback(context.Background(), nonce, victimSubject, 42, "victim-oauth-code")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("victim completing the attacker's link = %v, want ErrForbidden", err)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Fatal("no installation may be bound when the completing user is not the initiator")
	}
}

// The legitimate single-browser flow still completes end to end.
func TestInitiatorCompletesTheirOwnFlow(t *testing.T) {
	svc := connectService(t)
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	conn, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 42, "oauth-code")
	if err != nil {
		t.Fatalf("initiator completing their own flow: %v", err)
	}
	if !conn.Connected || conn.InstallationID != 42 {
		t.Fatalf("connection = %+v", conn)
	}
}

// The transaction is single-use: a replayed callback finds nothing, so a captured
// URL cannot be redeemed a second time against another installation.
func TestConnectNonceIsSingleUse(t *testing.T) {
	svc := connectService(t)
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	if _, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 42, "oauth-code"); err != nil {
		t.Fatalf("first use: %v", err)
	}
	_, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 99, "oauth-code")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("replayed nonce = %v, want ErrForbidden", err)
	}
}

// A nonce that was never minted is refused identically to a consumed one, so a
// caller cannot probe which attempts exist.
func TestUnknownConnectNonceIsRefused(t *testing.T) {
	svc := connectService(t)

	_, err := svc.connectFromCallback(context.Background(), "never-minted", attackerSubject, 42, "oauth-code")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unknown nonce = %v, want ErrForbidden", err)
	}
}

// The nonce is consumed even when a later step refuses, so a rejected attempt
// cannot be retried against a different installation.
func TestNonceIsConsumedEvenWhenTheAttemptIsRefused(t *testing.T) {
	svc := connectService(t)
	svc.Verifier = &fakeVerifier{ok: false} // the caller does not administer it
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	if _, err := svc.connectFromCallback(context.Background(), nonce, attackerSubject, 42, "oauth-code"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin = %v, want ErrForbidden", err)
	}
	if _, ok := svc.Store.(*fakeStore).txns[nonce]; ok {
		t.Error("a refused attempt must still have consumed its nonce")
	}
}

// An anonymous callback — no bex session at all — can no longer bind anything.
// The gate lets this route through without a credential (GitHub has none to
// give), so the refusal has to come from the service.
func TestAnonymousCallbackIsRefused(t *testing.T) {
	svc := connectService(t)
	nonce := seedConnectTxn(t, svc, attackerWorkspce, attackerSubject)

	_, err := svc.connectFromCallback(context.Background(), nonce, "", 42, "oauth-code")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("anonymous callback = %v, want ErrForbidden", err)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Fatal("an anonymous callback must bind nothing")
	}
}

// The same attack over the real HTTP route, so the wiring (not just the service
// verb) is covered: the attacker's install URL, opened by the signed-in victim.
func TestCallbackRouteRefusesAnotherUsersLink(t *testing.T) {
	svc := connectService(t)
	svc.DashboardURL = "https://dash.bex.co"
	m := mux(svc)

	token, err := svc.mintConnectState(
		core.WithIdentity(context.Background(), core.Identity{Subject: attackerSubject}),
		attackerWorkspce, attackerSubject)
	if err != nil {
		t.Fatalf("mint state: %v", err)
	}
	path := "/v1/git/callback?installation_id=42&code=oauth-code&state=" + url.QueryEscape(token)

	rec := doAs(t, m, http.MethodGet, path, victimSubject)
	// The route redirects failures to the dashboard with a code rather than
	// rendering an error page; either way it must not be a success.
	if rec.Code == http.StatusOK {
		t.Fatalf("victim following the attacker's link succeeded (%d)", rec.Code)
	}
	if len(svc.Store.(*fakeStore).conns) != 0 {
		t.Fatal("no connection may be recorded")
	}
}
