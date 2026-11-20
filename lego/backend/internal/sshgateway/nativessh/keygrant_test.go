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
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w9/125: no API key can enroll an SSH key since ADR072 F3, but one enrolled
// before then authenticates as the key. Such a connection is held to the
// key's grant (a shell on a service needs "Manage preview environments and
// one-off jobs", one in an agent session's sandbox "Run commands in
// sandboxes") at authentication, on every channel and on every revalidation
// tick. A person's connection asks once, at authentication, and never again.

const keySSHTick = 10 * time.Millisecond

// startGrantGateway serves native SSH for one registered key owned by owner,
// with the key grant gate over bex-api's real verb.
func startGrantGateway(t *testing.T, grants *gatewaytest.KeyGrants, owner string, resolver sshgateway.TargetResolver, exec sshgateway.Executor, clientSigner ssh.Signer) (*gatewaytest.FakeStore, string) {
	t.Helper()
	st := &gatewaytest.FakeStore{Key: store.SSHKey{Subject: owner, Fingerprint: ssh.FingerprintSHA256(clientSigner.PublicKey())}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		Store: st, Apps: resolver, Executor: exec, Signer: signer(t), SessionTimeout: time.Minute,
		MaxChannelsPerConn: 4, RevalidateInterval: keySSHTick,
		KeyGrants: &sshgateway.KeyGrantGate{Asker: grants.Client(t)},
	}
	go func() { _ = server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); _ = listener.Close() })
	return st, listener.Addr().String()
}

func waitRevoked(t *testing.T, st *gatewaytest.FakeStore, exec *countingExecutor) {
	t.Helper()
	waitFor(t, func() bool {
		ended := st.EndedSessions()
		return exec.activeNow() == 0 && len(ended) == 1 && endsWith(ended[0], ":revoked")
	})
}

func TestNativeSSHKeyOwnedSessionEndsAtTheFirstTickAfterItsGrantNarrows(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	st, addr := startGrantGateway(t, grants, "key-1", &gatewaytest.FakeResolver{}, exec, clientSigner)

	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatalf("a covered key's SSH key was refused: %v", err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

	mark := grants.Set("key-1", "services:read")
	waitRevoked(t, st, exec)
	if n := grants.Reads() - mark; n != 1 {
		t.Fatalf("the session ended after %d grant reads since the narrowing, want 1 (the first tick)", n)
	}
}

func TestNativeSSHKeyOwnedSessionSurvivesAWideningAndAnUnrelatedChange(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	st, addr := startGrantGateway(t, grants, "key-1", &gatewaytest.FakeResolver{}, exec, clientSigner)
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

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
		if exec.activeNow() != 1 || len(st.EndedSessions()) != 0 {
			t.Fatalf("the session ended after %s", change.name)
		}
	}
	grants.Set("key-1", "postgres:read")
	waitRevoked(t, st, exec)
}

// The action follows the target: a key that may run jobs on services but not
// commands in sandboxes reaches a service and not an agent session's sandbox.
func TestNativeSSHKeyOwnerIsRefusedATargetItsGrantDoesNotCover(t *testing.T) {
	clientSigner := signer(t)
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	exec := &countingExecutor{}
	_, addr := startGrantGateway(t, grants, "key-1", &gatewaytest.FakeResolver{Target: sandboxTarget()}, exec, clientSigner)
	if client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner); err == nil {
		client.Close()
		t.Fatal("a key without Sandbox execution reached an agent session's sandbox")
	}
	grants.Set("key-1", "services:read_write", "sandbox_execution:execute")
	client, err := dialGateway(addr, "ags-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatalf("a key with Sandbox execution was refused: %v", err)
	}
	defer client.Close()
	// A channel opened after a narrowing is refused before it runs.
	grants.Set("key-1", "services:read_write")
	if _, err := client.NewSession(); err == nil {
		t.Fatal("a channel opened after the narrowing was accepted")
	}
}

// A grant the verb cannot read is retried as every other unanswered check is
// (w6/147), and ends the session once maxUnansweredRevalidations ticks in a
// row went unanswered: fail closed, within a bound.
func TestNativeSSHKeyOwnedSessionEndsWhenItsGrantCannotBeRead(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	st, addr := startGrantGateway(t, grants, "key-1", &gatewaytest.FakeResolver{}, exec, clientSigner)
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)

	mark := grants.Fail(errors.New("postgres down"))
	waitRevoked(t, st, exec)
	if n := grants.Reads() - mark; n != maxUnansweredRevalidations {
		t.Fatalf("the session ended after %d unanswered questions, want %d", n, maxUnansweredRevalidations)
	}
}

// A person's connection asks once, at authentication, learns its owner is no
// key, and never asks again: the verb going down leaves it running.
func TestNativeSSHPersonSessionAsksOnceAtAuthentication(t *testing.T) {
	clientSigner := signer(t)
	exec := &countingExecutor{release: make(chan struct{})}
	grants := &gatewaytest.KeyGrants{}
	st, addr := startGrantGateway(t, grants, "user-1", &gatewaytest.FakeResolver{}, exec, clientSigner)
	client, err := dialGateway(addr, "srv-abcdeabcdeabcdeabcde", clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	holdChannel(t, client, exec)
	if n := grants.Reads(); n != 1 {
		t.Fatalf("a person's connection asked %d times to authenticate, want once", n)
	}
	grants.Fail(errors.New("bex-api unreachable"))
	time.Sleep(10 * keySSHTick)
	if exec.activeNow() != 1 || len(st.EndedSessions()) != 0 {
		t.Fatal("a person's session ended with the grant verb down")
	}
	if n := grants.Reads(); n != 1 {
		t.Fatalf("a person's connection asked the grant verb %d times, want once", n)
	}
}
