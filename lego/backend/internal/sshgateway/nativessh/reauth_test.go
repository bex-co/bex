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

package nativessh

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// codex round-8 #5: a transport may outlive its auth-time decision by hours,
// so every channel re-authorizes (key re-read + fresh target resolution)
// before it is accepted. These tests flip the store/resolver AFTER transport
// auth and assert the next channel — and the whole transport — die.

// holdChannel opens one session channel whose exec blocks until released, so a
// test can hold slots open the way a live editor session does.
func holdChannel(t *testing.T, client *ssh.Client, exec *countingExecutor) {
	t.Helper()
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("channel should open before revocation: %v", err)
	}
	go func() { _ = session.Run("hold") }()
	waitFor(t, func() bool { return exec.activeNow() >= 1 })
}

// A membership revocation (or session teardown) landing after transport auth
// must reject the next channel AND end the transport — including its active
// channels, not just future ones.
func TestRevocationAfterTransportAuthEndsTransportAndActiveChannels(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4 })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// One live channel is executing when the revocation lands.
	holdChannel(t, client, exec)

	resolver.SetFlip(errors.New("revoked"))

	// The next channel must be refused — no new exec on a stale decision.
	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel opened after revocation must be rejected")
	}
	// The revocation must also tear down the ACTIVE channel (mctx cancels
	// before the channel goroutines are waited on), not merely gate new ones.
	// The audit row lands after the channel goroutines unwind, so wait for the
	// row itself rather than polling the executor — the two events race.
	waitFor(t, func() bool {
		if exec.activeNow() != 0 {
			return false
		}
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
	if ended := st.EndedSessions(); len(ended) != 1 || !endsWith(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// codex round-9 #6: channel-open reauthorization alone leaves an ADMITTED exec
// running until the client disconnects or the 4h cap. The watchdog re-runs the
// same reauthorization on the interval and cancels the LIVE stream — here a
// revocation lands while the only channel's exec is mid-flight, with no new
// channel to carry the check.
func TestMidStreamRevocationEndsActiveMultiChannelExec(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4; s.RevalidateInterval = 10 * time.Millisecond })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	holdChannel(t, client, exec)
	resolver.SetFlip(errors.New("revoked"))

	waitFor(t, func() bool {
		if exec.activeNow() != 0 {
			return false
		}
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

// The same watchdog on the single-channel App path: the admitted exec dies
// with a revoked audit result, without the client opening anything new.
func TestMidStreamRevocationEndsActiveSingleChannelExec(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{} // default srv- target: single-channel
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, resolver, exec, clientSigner,
		func(s *Server) { s.RevalidateInterval = 10 * time.Millisecond })
	defer stop()

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	holdChannel(t, client, exec)
	resolver.SetFlip(errors.New("revoked"))

	waitFor(t, func() bool {
		if exec.activeNow() != 0 {
			return false
		}
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

// Deleting the SSH key mid-connection is the same revocation: the credential
// itself is gone, so no new channel may start and the transport ends.
func TestKeyDeletionAfterTransportAuthEndsTransport(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4 })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

	st.SetFailKeyLookups(true)

	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel opened after key deletion must be rejected")
	}
	waitFor(t, func() bool {
		if exec.activeNow() != 0 {
			return false
		}
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
	if ended := st.EndedSessions(); len(ended) != 1 || !endsWith(ended[0], ":revoked") {
		t.Fatalf("session audit result = %v, want one entry ending :revoked", ended)
	}
}

// The single-channel App path re-authorizes too: a client may connect, wait,
// and only then open its one channel — that open rides a fresh decision.
func TestSingleChannelPathReauthorizesBeforeAccept(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{} // default srv- target: single-channel
	st := &gatewaytest.FakeStore{}
	addr, stop := startGatewayConfigured(t, st, resolver, exec, clientSigner, nil)
	defer stop()

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	resolver.SetFlip(errors.New("revoked"))

	if _, err := client.NewSession(); err == nil {
		t.Fatal("the App path must re-authorize before accepting its one channel")
	}
	// The audit row races the channel rejection (it lands in serveConn's
	// defer), so wait for it rather than asserting immediately.
	waitFor(t, func() bool {
		ended := st.EndedSessions()
		return len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

// codex round-8 #7: the per-identity channel budget bounds exec STREAMS, not
// transports — a second connection of the SAME identity must not multiply the
// streams the first connection's multiplexing already claimed. The per-conn
// semaphore alone would happily admit this channel (conn2 holds none).
func TestChannelBudgetSpansConnectionsOfOneIdentity(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	// ONE channel limiter shared by both connections, as the process does.
	channels := sshgateway.NewChannelLimiter(512, 2)
	dial := func() *ssh.Client {
		t.Helper()
		addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner,
			func(s *Server) { s.MaxChannelsPerConn = 4; s.ChannelLimits = channels })
		t.Cleanup(stop)
		client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	c1 := dial()
	defer c1.Close()
	c2 := dial()
	defer c2.Close()

	// Connection 1 spends the whole per-identity channel budget (2).
	holdChannel(t, c1, exec)
	session2, err := c1.NewSession()
	if err != nil {
		t.Fatalf("second channel on conn1 should be admitted: %v", err)
	}
	go func() { _ = session2.Run("hold") }()
	waitFor(t, func() bool { return exec.activeNow() == 2 })

	// Connection 2, same identity: its FIRST channel must be shed even though
	// its own per-connection semaphore is empty.
	if _, err := c2.NewSession(); err == nil {
		t.Fatal("per-identity channel budget must bound streams across connections, not per connection")
	}
	close(exec.release)
}

// A different identity is unaffected by another's channel budget.
func TestChannelBudgetIsPerIdentity(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	channels := sshgateway.NewChannelLimiter(512, 1)
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4; s.ChannelLimits = channels })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	holdChannel(t, client, exec)
	if _, err := client.NewSession(); err == nil {
		t.Fatal("second stream of the same identity must be shed at the per-identity cap")
	}
	close(exec.release)
}

// The limiter slots are released when channels end, so the budget is a
// concurrency bound, not a lifetime quota.
func TestChannelBudgetReleasesEndedStreams(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{}
	resolver := &gatewaytest.FakeResolver{Target: sandboxTarget()}
	channels := sshgateway.NewChannelLimiter(512, 1)
	addr, stop := startGatewayConfigured(t, &gatewaytest.FakeStore{}, resolver, exec, clientSigner,
		func(s *Server) { s.MaxChannelsPerConn = 4; s.ChannelLimits = channels })
	defer stop()

	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	s1, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Run("work"); err != nil {
		t.Fatal(err)
	} // exec completes, channel closes, slot releases

	s2, err := client.NewSession()
	if err != nil {
		t.Fatalf("stream slot must be reusable after a channel ends: %v", err)
	}
	_ = s2.Close()
}
