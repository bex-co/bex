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
	"encoding/json"
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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m352: a server-ended session tells the browser why, as a stable reason
// plus an English sentence, and never says more than the caller's own
// GET /v1/services/{id} would.

// startHeldShell connects one browser shell whose exec blocks until its
// context ends, and returns the connection once the exec is live.
func startHeldShell(t *testing.T, resolver sshgateway.TargetResolver, serviceID string, configure func(*Server)) (*websocket.Conn, *gatewaytest.FakeStore, *gatewaytest.FakeExecutor) {
	t.Helper()
	st := &gatewaytest.FakeStore{}
	exec := &gatewaytest.FakeExecutor{Block: true, Started: make(chan struct{}, 1), Stopped: make(chan error, 1)}
	gw := newWSGateway(t, st, resolver, exec)
	gw.RevalidateInterval = 10 * time.Millisecond
	if configure != nil {
		configure(gw)
	}
	srv := httptest.NewServer(gw.Handler())
	t.Cleanup(srv.Close)
	conn, _, err := dialWS(t, srv, mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: serviceID}))
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

// readErrorFrame returns the first error control frame before the close.
func readErrorFrame(t *testing.T, conn *websocket.Conn) serverControl {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("connection closed without an error frame: %v", err)
		}
		var ctrl serverControl
		if mt == websocket.TextMessage && json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "error" {
			return ctrl
		}
	}
}

func wantEnd(t *testing.T, got serverControl, reason string) {
	t.Helper()
	if got.Reason != reason || got.Message != endMessages[reason] || got.Message == "" {
		t.Fatalf("end frame = %+v, want reason %q with %q", got, reason, endMessages[reason])
	}
}

// A deploy's build, pre-deploy and rollout leave the serving instance's shell
// open across every revalidation tick; the session ends, named, only when that
// instance's pod goes away — and it is never moved to another replica.
func TestWebSocketShellSurvivesADeployAndEndsWithItsInstance(t *testing.T) {
	serving := gatewaytest.NewServingApp(t)
	conn, st, exec := startHeldShell(t, serving.Resolver, gatewaytest.ServingAppID, nil)

	for _, phase := range []appv1alpha1.AppPhase{appv1alpha1.PhaseBuilding, appv1alpha1.PhaseDeploying, appv1alpha1.PhaseRunning} {
		serving.Update(t, func(a *appv1alpha1.App) { a.Status.Phase = phase })
		select {
		case err := <-exec.Stopped:
			t.Fatalf("the shell ended during %s: %v", phase, err)
		case <-time.After(80 * time.Millisecond): // ~8 ticks
		}
	}

	serving.DeletePod(t)
	wantEnd(t, readErrorFrame(t, conn), endInstanceGone)
	if ended := waitEndedSessions(t, st, 1); len(ended) != 1 || !strings.HasSuffix(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
	if exec.Target.PodName != "web-rs-pod01" {
		t.Fatalf("exec target = %+v, want the serving pod", exec.Target)
	}
}

// Suspending the service ends an open shell at the next tick and says so.
func TestWebSocketSuspensionEndsTheShellNamed(t *testing.T) {
	serving := gatewaytest.NewServingApp(t)
	conn, _, _ := startHeldShell(t, serving.Resolver, gatewaytest.ServingAppID, nil)
	serving.Update(t, func(a *appv1alpha1.App) { a.Spec.Suspended = true })
	got := readErrorFrame(t, conn)
	wantEnd(t, got, endSuspended)
	if strings.Contains(got.Message, "conflict") {
		t.Fatalf("end message %q leaks the transport class", got.Message)
	}
}

// A plan change to Free ends an open shell as ineligible.
func TestWebSocketFreePlanEndsTheShellNamed(t *testing.T) {
	serving := gatewaytest.NewServingApp(t)
	conn, _, _ := startHeldShell(t, serving.Resolver, gatewaytest.ServingAppID, nil)
	serving.Update(t, func(a *appv1alpha1.App) { a.Spec.Tier = "free" })
	wantEnd(t, readErrorFrame(t, conn), endIneligible)
}

// An authorization failure — a removed member, a lost role, a newly protected
// environment, a deleted service — reads only "your access ended", whichever
// it was.
func TestWebSocketRevocationEndsTheShellWithoutDetail(t *testing.T) {
	for name, err := range map[string]error{
		"forbidden": core.ErrForbidden,
		"not found": core.ErrNotFound,
		"coded":     core.NewForbiddenError("PROTECTED_ENVIRONMENT", "only an Admin can open a shell in a protected environment", nil),
	} {
		t.Run(name, func(t *testing.T) {
			resolver := &gatewaytest.FakeResolver{}
			conn, _, _ := startHeldShell(t, resolver, "srv-abcdeabcdeabcdeabcde", nil)
			resolver.SetFlip(err)
			wantEnd(t, readErrorFrame(t, conn), endRevoked)
		})
	}
}

// The resolver failing closed on an unreachable store is not a revocation.
func TestWebSocketUnverifiableAccessEndsTheShellAsUnverified(t *testing.T) {
	resolver := &gatewaytest.FakeResolver{}
	conn, _, _ := startHeldShell(t, resolver, "srv-abcdeabcdeabcdeabcde", nil)
	resolver.SetFlip(errors.New("dial tcp: connection refused"))
	wantEnd(t, readErrorFrame(t, conn), endUnverified)
}

// The session cap ends a shell as timeout, not as a revocation.
func TestWebSocketSessionTimeoutEndsTheShellNamed(t *testing.T) {
	conn, st, _ := startHeldShell(t, &gatewaytest.FakeResolver{}, "srv-abcdeabcdeabcdeabcde", func(s *Server) {
		s.SessionTimeout = 100 * time.Millisecond
		s.RevalidateInterval = -1
	})
	wantEnd(t, readErrorFrame(t, conn), endTimeout)
	if ended := waitEndedSessions(t, st, 1); len(ended) != 1 || !strings.HasSuffix(ended[0], ":failed") {
		t.Fatalf("session audit result = %v, want one entry ending :failed", ended)
	}
}

// The attached pod stopping under the stream (the executor sees it terminate)
// is named instance_gone, not "unable to start a shell in this image".
func TestWebSocketTerminatedTargetEndsTheShellNamed(t *testing.T) {
	st := &gatewaytest.FakeStore{}
	exec := &gatewaytest.FakeExecutor{Err: &sshgateway.TargetTerminatedError{Detail: "pod no longer exists"}}
	srv := httptest.NewServer(newWSGateway(t, st, &gatewaytest.FakeResolver{}, exec).Handler())
	defer srv.Close()
	conn, _, err := dialWS(t, srv, mintWS(t, shellticket.Claims{Subject: "user-1", ServiceID: "srv-abcdeabcdeabcdeabcde"}))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// The fake exec reads two terminal sizes before it returns.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":100,"rows":40}`)); err != nil {
		t.Fatal(err)
	}
	wantEnd(t, readErrorFrame(t, conn), endInstanceGone)
}
