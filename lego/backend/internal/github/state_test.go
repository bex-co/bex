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
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type connectStateWorkspaceResolver struct{}

func (connectStateWorkspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-default", true
}

func (connectStateWorkspaceResolver) IsMember(_ context.Context, _ core.Identity, workspaceID string) (bool, error) {
	return workspaceID == "tea-target", nil
}

func TestConnectStateRoundTripAndTamperCheck(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	st := newFakeStore()
	svc := &Service{
		Base:        &core.Base{Clock: func() time.Time { return now }},
		Store:       st,
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	token, err := svc.mintConnectState(testCallerCtx(), "tea-workspace", testCallerSubject)
	if err != nil {
		t.Fatalf("mintConnectState: %v", err)
	}
	if strings.Contains(token, "tea-workspace") {
		t.Fatalf("state token exposes its workspace: %q", token)
	}
	// Since w1/m67 F3 the state carries only an opaque nonce; the workspace and
	// the initiating subject live in the durable transaction it names.
	nonce, err := svc.verifyConnectState(token)
	if err != nil || nonce == "" {
		t.Fatalf("verifyConnectState = %q, %v; want a nonce, nil", nonce, err)
	}
	if strings.Contains(token, testCallerSubject) {
		t.Fatalf("state token exposes its initiator: %q", token)
	}
	txn, ok := st.txns[nonce]
	if !ok || txn.TenantID != "tea-workspace" || txn.Subject != testCallerSubject {
		t.Fatalf("connect transaction = %+v (present=%v); want tea-workspace/%s", txn, ok, testCallerSubject)
	}

	signed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	signed[0] ^= 0x01
	tampered := base64.RawURLEncoding.EncodeToString(signed)
	if _, err := svc.verifyConnectState(tampered); !errors.Is(err, errConnectStateInvalid) {
		t.Fatalf("tampered state error = %v, want errConnectStateInvalid", err)
	}
}

func TestConnectStateMissingAndExpired(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	svc := &Service{
		Base:        &core.Base{Clock: func() time.Time { return now }},
		Store:       newFakeStore(),
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	if _, err := svc.verifyConnectState(""); !errors.Is(err, errConnectStateMissing) {
		t.Fatalf("missing state error = %v, want errConnectStateMissing", err)
	}
	token, err := svc.mintConnectState(testCallerCtx(), "tea-workspace", testCallerSubject)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(connectStateTTL)
	if _, err := svc.verifyConnectState(token); !errors.Is(err, errConnectStateExpired) {
		t.Fatalf("expired state error = %v, want errConnectStateExpired", err)
	}
}

func TestStartConnectReturnsStatefulInstallURL(t *testing.T) {
	st := newFakeStore()
	svc := &Service{
		Base:        &core.Base{Namespace: "default", Workspace: connectStateWorkspaceResolver{}},
		GitHub:      &fakeClient{},
		Store:       st,
		Verifier:    &fakeVerifier{}, // §7: StartConnect preflights the verifier
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin", Method: "session"})
	conn, err := svc.StartConnect(ctx, "tea-target")
	if err != nil {
		t.Fatalf("StartConnect: %v", err)
	}
	assertStateWorkspace := func(installURL string) {
		t.Helper()
		u, err := url.Parse(installURL)
		if err != nil {
			t.Fatal(err)
		}
		token := u.Query().Get("state")
		if token == "" {
			t.Fatalf("install URL has no state: %s", installURL)
		}
		nonce, err := svc.verifyConnectState(token)
		if err != nil || nonce == "" {
			t.Fatalf("install URL state = %q, %v; want a nonce, nil", nonce, err)
		}
		// The state names an attempt; the durable row is what binds the workspace.
		txn, ok := st.txns[nonce]
		if !ok || txn.TenantID != "tea-target" {
			t.Fatalf("connect transaction = %+v (present=%v); want tea-target", txn, ok)
		}
	}
	assertStateWorkspace(conn.InstallURL)

	// An already-connected response must carry the newly minted state too; the
	// dashboard may use StartConnect to replace or update an installation.
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-target", InstallationID: 7, AccountLogin: "octo"})
	connected, err := svc.StartConnect(ctx, "tea-target")
	if err != nil {
		t.Fatalf("StartConnect existing connection: %v", err)
	}
	if !connected.Connected {
		t.Fatal("existing connection was not returned as connected")
	}
	assertStateWorkspace(connected.InstallURL)
}
