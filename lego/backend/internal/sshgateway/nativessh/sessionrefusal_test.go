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
	"net"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// sessionrefusal_test.go: `ssh ags-…@ssh.bex.co` into a session whose sandbox
// is not up tells a member who may open it what is wrong and how to start it
// again, in the auth banner the SSH client prints (w10/043). Someone who may
// not view the session, or names one that does not exist, gets the bare
// refusal, so the banner never reveals that a session exists. The gateway runs
// with the real agent-session resolver, as cmd/ssh-gateway wires it.

type sessionRows map[string]store.AgentSession

func (r sessionRows) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	row, ok := r[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	return row, nil
}

// viewers answers can_view_sensitive as OpenFGA does: true only for the
// "subject object" pairs it holds, so an object with no tuples (a session that
// does not exist) is denied to everyone.
type viewers map[string]bool

func (v viewers) Check(_ context.Context, subject, relation, object string) (bool, error) {
	return relation == core.RelCanViewSensitive && v[subject+" "+object], nil
}

type sessionGateway struct {
	addr     string
	signer   ssh.Signer
	registry *prometheus.Registry
}

// startSessionGateway serves native SSH for one key owned by owner, resolving
// ags-… usernames through agentsessions.SSHResolver over rows. owner may view
// every session in visible; grants, when set, holds owner to an API key's
// grant.
func startSessionGateway(t *testing.T, owner string, rows sessionRows, visible []string, grants *gatewaytest.KeyGrants) *sessionGateway {
	t.Helper()
	g := &sessionGateway{signer: signer(t), registry: prometheus.NewRegistry()}
	allowed := viewers{}
	for _, id := range visible {
		allowed["user:"+owner+" agent_session:"+id] = true
	}
	resolver := sshgateway.CompositeResolver{
		Apps:          &gatewaytest.FakeResolver{},
		AgentSessions: &agentsessions.SSHResolver{Base: &core.Base{Authz: allowed}, Store: rows},
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		Store:    &gatewaytest.FakeStore{Key: store.SSHKey{Subject: owner, Fingerprint: ssh.FingerprintSHA256(g.signer.PublicKey())}},
		Apps:     resolver,
		Executor: &gatewaytest.FakeExecutor{}, Signer: signer(t), SessionTimeout: time.Minute,
		Metrics: sshgateway.NewMetrics(g.registry),
	}
	if grants != nil {
		server.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	}
	go func() { _ = server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); _ = listener.Close() })
	g.addr = listener.Addr().String()
	return g
}

// add stores a session of workspace tea-a and returns its id.
func (r sessionRows) add(phase, sandboxID string) string {
	id := ids.New(ids.AgentSession)
	r[id] = store.AgentSession{ID: id, WorkspaceID: "tea-a", Phase: phase, SandboxID: sandboxID}
	return id
}

func (g *sessionGateway) authentications(t *testing.T, result string) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, g.registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": result})
}

// refused dials username and fails the test unless the login is refused; it
// returns the banner the client was sent.
func (g *sessionGateway) refused(t *testing.T, username string) string {
	t.Helper()
	client, banner, err := dialWithBanner(g.addr, username, g.signer)
	if err == nil {
		client.Close()
		t.Fatalf("ssh %s@ was admitted, want a refusal", username)
	}
	return banner
}

func TestSSHIntoASessionWhoseSandboxClosedSaysHowToStartIt(t *testing.T) {
	rows := sessionRows{}
	closed := rows.add(agentsessions.PhaseCompleted, "")
	running := rows.add(agentsessions.PhaseRunning, "os-live")
	g := startSessionGateway(t, "user-1", rows, []string{closed, running}, nil)

	banner := g.refused(t, closed)
	for _, want := range []string{"bex: agent session " + closed + " is not running", "send the session a message", "connect again"} {
		if !strings.Contains(banner, want) {
			t.Fatalf("banner = %q, want it to say %q", banner, want)
		}
	}
	if !strings.HasSuffix(banner, ".\n") {
		t.Fatalf("banner = %q, want one sentence-ended line for the client to print", banner)
	}
	if got := g.authentications(t, "rejected_no_sandbox"); got != 1 {
		t.Fatalf("rejected_no_sandbox authentications = %v, want 1", got)
	}

	// The same member reaches a session whose sandbox is up, with no banner.
	client, banner, err := dialWithBanner(g.addr, running, g.signer)
	if err != nil {
		t.Fatalf("a running session's sandbox was refused: %v", err)
	}
	client.Close()
	if banner != "" {
		t.Fatalf("an admitted login was sent a banner: %q", banner)
	}

	// Once a message starts a turn and the sandbox is up again, the login the
	// banner refused goes through.
	row := rows[closed]
	row.Phase, row.SandboxID = agentsessions.PhaseRunning, "os-again"
	rows[closed] = row
	client, _, err = dialWithBanner(g.addr, closed, g.signer)
	if err != nil {
		t.Fatalf("the session was refused after its sandbox started again: %v", err)
	}
	client.Close()
}

func TestSSHIntoASessionTheCallerCannotViewOrThatDoesNotExistIsABareRefusal(t *testing.T) {
	rows := sessionRows{}
	foreign := rows.add(agentsessions.PhaseCompleted, "") // closed, but not user-1's to view
	missing := ids.New(ids.AgentSession)                  // no row and no tuples
	deleted := ids.New(ids.AgentSession)                  // a tuple left behind, no row
	g := startSessionGateway(t, "user-1", rows, []string{deleted}, nil)

	for name, username := range map[string]string{"another member's session": foreign, "no such session": missing, "a deleted session": deleted} {
		if banner := g.refused(t, username); banner != "" {
			t.Errorf("%s: banner = %q, want the bare refusal", name, banner)
		}
	}
	if got := g.authentications(t, "rejected_target"); got != 3 {
		t.Fatalf("rejected_target authentications = %v, want 3", got)
	}
	if got := g.authentications(t, "rejected_no_sandbox"); got != 0 {
		t.Fatalf("rejected_no_sandbox authentications = %v, want 0", got)
	}
}

// A key enrolled by a bex API key is held to the key's grant (w9/125): one
// that may not run commands in sandboxes is not told about a session either.
func TestSSHKeyOwnerWithoutSandboxExecutionIsNotToldAboutTheSession(t *testing.T) {
	rows := sessionRows{}
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	closed := rows.add(agentsessions.PhaseCompleted, "")
	g := startSessionGateway(t, "key-1", rows, []string{closed}, grants)

	if banner := g.refused(t, closed); banner != "" {
		t.Fatalf("banner = %q, want the bare refusal for a key whose grant does not cover the sandbox", banner)
	}
	grants.Set("key-1", "sandbox_execution:execute")
	if banner := g.refused(t, closed); !strings.Contains(banner, "is not running") {
		t.Fatalf("banner = %q, want the session's state once the key's grant covers the sandbox", banner)
	}
}
