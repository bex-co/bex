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

package webshell

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w9/125: a shell a bex API key opened is held to the key's grant ("Manage
// preview environments and one-off jobs", what its ticket's mint needed) at
// connect and on every revalidation tick, over bex-api's real grant verb.

const keyShellTick = 10 * time.Millisecond

// openGrantShell connects one shell for subject whose ticket names keyAction
// ("" for a person's) and returns it once its exec is live.
func openGrantShell(t *testing.T, grants *gatewaytest.KeyGrants, subject, keyAction string) (*websocket.Conn, *gatewaytest.FakeStore, *gatewaytest.FakeExecutor) {
	t.Helper()
	st := &gatewaytest.FakeStore{}
	exec := &gatewaytest.FakeExecutor{Block: true, Started: make(chan struct{}, 1), Stopped: make(chan error, 1)}
	gw := newWSGateway(t, st, &gatewaytest.FakeResolver{}, exec)
	gw.RevalidateInterval = keyShellTick
	gw.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	srv := httptest.NewServer(gw.Handler())
	t.Cleanup(srv.Close)
	conn, _, err := dialWS(t, srv, mintWS(t, shellticket.Claims{Subject: subject, ServiceID: "srv-abcdeabcdeabcdeabcde", KeyAction: keyAction}))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	select {
	case <-exec.Started:
	case <-time.After(2 * time.Second):
		t.Fatal("shell exec never started")
	}
	return conn, st, exec
}

func wantShellLive(t *testing.T, exec *gatewaytest.FakeExecutor, after string) {
	t.Helper()
	select {
	case err := <-exec.Stopped:
		t.Fatalf("the shell ended after %s: %v", after, err)
	default:
	}
}

// A narrowing ends the shell at the first tick after it, named as a lost
// access, and audited revoked.
func TestWebSocketKeyShellEndsAtTheFirstTickAfterItsGrantNarrows(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	conn, st, _ := openGrantShell(t, grants, "key-1", string(core.KeyServicesJobs))

	mark := grants.Set("key-1", "services:read")
	wantEnd(t, readErrorFrame(t, conn), endRevoked)
	if n := grants.Reads() - mark; n != 1 {
		t.Fatalf("the shell ended after %d grant reads since the narrowing, want 1 (the first tick)", n)
	}
	if ended := waitEndedSessions(t, st, 1); len(ended) != 1 || !strings.HasSuffix(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
}

// A widening, a change to a permission the shell does not use, and a grant
// that keeps covering the shell leave it running through every tick.
func TestWebSocketKeyShellSurvivesAWideningAndAnUnrelatedChange(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	conn, _, exec := openGrantShell(t, grants, "key-1", string(core.KeyServicesJobs))

	for _, change := range []struct {
		name string
		set  func() int
	}{
		{"no change", grants.Reads},
		{"a widening", func() int { return grants.Set("key-1", "services:read_write", "services_env:read") }},
		{"an unrelated change", func() int { return grants.Set("key-1", "services:read_write", "postgres:read") }},
		{"a widening to All", func() int { return grants.SetAll("key-1") }},
	} {
		mark := change.set()
		grants.WaitReads(t, mark+3)
		wantShellLive(t, exec, change.name)
	}
	// It is still the grant that holds it: narrowing now ends it.
	grants.Set("key-1", "services:read")
	wantEnd(t, readErrorFrame(t, conn), endRevoked)
}

// A key whose grant no longer covers a shell by the time it connects gets
// none: the ticket's mint-time decision does not outlive the narrowing.
func TestWebSocketKeyShellIsRefusedAtConnectOnceItsGrantNarrowed(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read")
	st := &gatewaytest.FakeStore{}
	exec := &gatewaytest.FakeExecutor{Started: make(chan struct{}, 1)}
	gw := newWSGateway(t, st, &gatewaytest.FakeResolver{}, exec)
	gw.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()
	conn, _, err := dialWS(t, srv, mintWS(t, shellticket.Claims{Subject: "key-1", ServiceID: "srv-abcdeabcdeabcdeabcde", KeyAction: string(core.KeyServicesJobs)}))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if got := readErrorFrame(t, conn); !strings.Contains(got.Message, "no longer granted") {
		t.Fatalf("refusal frame = %+v, want the grant refusal", got)
	}
	select {
	case <-exec.Started:
		t.Fatal("a refused key's shell started")
	default:
	}
	if started := st.StartedSessions(); len(started) != 0 {
		t.Fatalf("a refused shell was audited as started: %v", started)
	}
}

// A grant that cannot be read ends a key's shell (fail closed), as an
// unreachable OpenFGA does.
func TestWebSocketKeyShellEndsWhenItsGrantCannotBeRead(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	conn, _, _ := openGrantShell(t, grants, "key-1", string(core.KeyServicesJobs))
	grants.Fail(errors.New("postgres down"))
	wantEnd(t, readErrorFrame(t, conn), endUnverified)
}

// A person's shell names no action, so it never asks the verb: a bex-api
// that cannot answer, or a change to some key's grant, leaves it running.
func TestWebSocketPersonShellNeverAsksForAGrant(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Fail(errors.New("bex-api unreachable"))
	_, _, exec := openGrantShell(t, grants, "user-1", "")
	time.Sleep(10 * keyShellTick)
	wantShellLive(t, exec, "ten ticks with the verb down")
	if n := grants.Reads(); n != 0 {
		t.Fatalf("a person's shell asked the grant verb %d times", n)
	}
}
