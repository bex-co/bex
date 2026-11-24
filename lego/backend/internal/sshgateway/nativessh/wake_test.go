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
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// wake_test.go: `ssh ags-…@ssh.bex.co` into a session whose sandbox closed
// wakes it and opens the shell (w10/m597), as SSH into a sleeping Devin
// session does. The gateway runs the real agent-session resolver over rows
// the test settles as the Completer would; the bex-api verb is a fake that
// answers as Service.Resume does.

// liveRows is sessionRows the gateway may read while the test settles it.
type liveRows struct {
	mu   sync.Mutex
	rows map[string]store.AgentSession
}

func (r *liveRows) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	return row, nil
}

func (r *liveRows) add(phase, sandboxID string) string {
	id := ids.New(ids.AgentSession)
	r.set(id, func(row *store.AgentSession) {
		*row = store.AgentSession{ID: id, WorkspaceID: "tea-a", Phase: phase, SandboxID: sandboxID}
	})
	return id
}

func (r *liveRows) set(id string, change func(*store.AgentSession)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]store.AgentSession{}
	}
	row := r.rows[id]
	change(&row)
	r.rows[id] = row
}

func (r *liveRows) phase(id string) string {
	row, _ := r.GetAgentSession(context.Background(), id)
	return row.Phase
}

// settle is what the Completer does once the editor's start ends: the
// session returns to its finished phase, with the sandbox or without.
func (r *liveRows) settle(id, sandboxID string) {
	r.set(id, func(row *store.AgentSession) { row.Phase, row.SandboxID = agentsessions.PhaseCompleted, sandboxID })
}

// fakeWaker answers the wake verb as bex-api's Service.Resume would: a
// started wake moves the session to resuming.
type fakeWaker struct {
	rows   *liveRows
	answer agentsession.SessionWakeResponse
	err    error

	mu    sync.Mutex
	calls []agentsession.SessionWakeRequest
}

func (w *fakeWaker) Wake(_ context.Context, req agentsession.SessionWakeRequest) (agentsession.SessionWakeResponse, error) {
	w.mu.Lock()
	w.calls = append(w.calls, req)
	w.mu.Unlock()
	if w.err != nil {
		return agentsession.SessionWakeResponse{}, w.err
	}
	if w.answer.Refusal != "" {
		return w.answer, nil
	}
	w.rows.set(req.SessionID, func(row *store.AgentSession) { row.Phase = agentsessions.PhaseResuming })
	return agentsession.SessionWakeResponse{SessionID: req.SessionID, Phase: agentsessions.PhaseResuming}, nil
}

func (w *fakeWaker) requests() []agentsession.SessionWakeRequest {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]agentsession.SessionWakeRequest(nil), w.calls...)
}

// podExecutor records the pod and command of every exec, from any channel.
type podExecutor struct {
	mu   sync.Mutex
	runs []string
}

func (e *podExecutor) Execute(_ context.Context, target apps.SSHInstanceTarget, command []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	e.mu.Lock()
	e.runs = append(e.runs, target.PodName+" "+strings.Join(command, " "))
	e.mu.Unlock()
	_, _ = io.WriteString(stdout, "ran in "+target.PodName+"\n")
	return 0, nil
}

func (e *podExecutor) ran() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.runs...)
}

type wakeGateway struct {
	addr     string
	signer   ssh.Signer
	registry *prometheus.Registry
	rows     *liveRows
	waker    *fakeWaker
	exec     *podExecutor
	end      context.CancelFunc
}

// startWakeGateway serves native SSH as production wires it: the
// multi-channel carve-out on, the wake verb wired, and owner's key allowed to
// view every session in visible.
func startWakeGateway(t *testing.T, owner string, rows *liveRows, visible []string, configure func(*Server)) *wakeGateway {
	t.Helper()
	g := &wakeGateway{signer: signer(t), registry: prometheus.NewRegistry(), rows: rows, waker: &fakeWaker{rows: rows}, exec: &podExecutor{}}
	allowed := viewers{}
	for _, id := range visible {
		allowed["user:"+owner+" agent_session:"+id] = true
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.end = cancel
	server := &Server{
		Store: &gatewaytest.FakeStore{Key: store.SSHKey{Subject: owner, Fingerprint: ssh.FingerprintSHA256(g.signer.PublicKey())}},
		Apps: sshgateway.CompositeResolver{
			Apps:          &gatewaytest.FakeResolver{},
			AgentSessions: &agentsessions.SSHResolver{Base: &core.Base{Authz: allowed}, Store: rows},
		},
		Executor: g.exec, Signer: signer(t), SessionTimeout: time.Minute,
		Metrics:            sshgateway.NewMetrics(g.registry),
		MaxChannelsPerConn: 16,
		Wakes:              g.waker,
		wakePollInterval:   10 * time.Millisecond,
		// A watchdog tick that did not skip the wait would end the held login
		// after three ticks, long before the test settles the session.
		RevalidateInterval: 20 * time.Millisecond,
	}
	if configure != nil {
		configure(server)
	}
	go func() { _ = server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); _ = listener.Close() })
	g.addr = listener.Addr().String()
	return g
}

func (g *wakeGateway) wakes(t *testing.T, result string) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, g.registry, "bex_ssh_gateway_session_wakes_total", map[string]string{"result": result})
}

// waitForWake waits until the session wakes counter reads result once.
func (g *wakeGateway) waitForWake(t *testing.T, result string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for g.wakes(t, result) != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("session_wakes_total{result=%q} = %v, want 1", result, g.wakes(t, result))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// login dials id and fails the test unless it is admitted with no banner.
func (g *wakeGateway) login(t *testing.T, id string) *ssh.Client {
	t.Helper()
	client, banner, err := dialWithBanner(g.addr, id, g.signer)
	if err != nil {
		t.Fatalf("ssh %s@ was refused (banner %q): %v", id, banner, err)
	}
	if banner != "" {
		t.Fatalf("an admitted login was sent a banner: %q", banner)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// syncBuffer is a bytes.Buffer the client's channel goroutine writes while
// the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// command is one command run on its own channel, with what it printed.
type command struct {
	stdout, stderr syncBuffer
	done           chan error
}

// exitStatus is a command's non-zero exit status.
type exitStatus uint32

func (e exitStatus) Error() string { return fmt.Sprintf("exit status %d", uint32(e)) }

// start opens a channel and runs cmd on it (sftp for the subsystem),
// reading its output as it arrives. Its result is nil, an exitStatus, or
// the error that kept it from finishing.
func start(t *testing.T, client *ssh.Client, cmd string) *command {
	t.Helper()
	channel, requests, err := client.OpenChannel("session", nil)
	if err != nil {
		t.Fatalf("open a channel: %v", err)
	}
	c := &command{done: make(chan error, 1)}
	var copied sync.WaitGroup
	copied.Add(2)
	go func() { defer copied.Done(); _, _ = io.Copy(&c.stdout, channel) }()
	go func() { defer copied.Done(); _, _ = io.Copy(&c.stderr, channel.Stderr()) }()
	go func() {
		defer channel.Close()
		request, payload := "exec", ssh.Marshal(struct{ Command string }{cmd})
		if cmd == "sftp" {
			request, payload = "subsystem", ssh.Marshal(struct{ Name string }{"sftp"})
		}
		if ok, err := channel.SendRequest(request, true, payload); err != nil || !ok {
			c.done <- fmt.Errorf("%s request refused: %v", request, err)
			return
		}
		var result error = errors.New("the channel closed without an exit status")
		for r := range requests {
			if r.Type == "exit-status" && len(r.Payload) == 4 {
				if status := binary.BigEndian.Uint32(r.Payload); status != 0 {
					result = exitStatus(status)
				} else {
					result = nil
				}
			}
			if r.WantReply {
				_ = r.Reply(false, nil)
			}
		}
		copied.Wait()
		c.done <- result
	}()
	return c
}

func (c *command) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-c.done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the command did not finish")
		return nil
	}
}

// waitForStderr waits until the command printed want on stderr.
func (c *command) waitForStderr(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(c.stderr.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("stderr = %q, want it to say %q", c.stderr.String(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSSHIntoAClosedSessionWakesItAndOpensTheShell(t *testing.T) {
	rows := &liveRows{}
	closed := rows.add(agentsessions.PhaseCompleted, "")
	g := startWakeGateway(t, "user-1", rows, []string{closed}, nil)

	client := g.login(t, closed)
	calls := g.waker.requests()
	if len(calls) != 1 || calls[0].SessionID != closed || calls[0].Subject != "user-1" ||
		calls[0].Fingerprint != ssh.FingerprintSHA256(g.signer.PublicKey()) || calls[0].SourceIP != "127.0.0.1" {
		t.Fatalf("wake requests = %+v, want one for %s as the key's owner", calls, closed)
	}
	if got := rows.phase(closed); got != agentsessions.PhaseResuming {
		t.Fatalf("phase = %q, want resuming once the wake started the sandbox", got)
	}

	// A shell, Zed's next channel and its sftp all wait for the same start.
	shell := start(t, client, "uname -a")
	probe := start(t, client, "echo zed")
	upload := start(t, client, "sftp")
	shell.waitForStderr(t, "bex: starting the sandbox of agent session "+closed)
	time.Sleep(100 * time.Millisecond) // several watchdog ticks and polls
	if ran := g.exec.ran(); len(ran) != 0 {
		t.Fatalf("ran %v before the sandbox was up", ran)
	}

	rows.settle(closed, "os-woken")
	for name, c := range map[string]*command{"shell": shell, "zed probe": probe, "sftp": upload} {
		if err := c.wait(t); err != nil {
			t.Fatalf("%s: %v (stderr %q)", name, err, c.stderr.String())
		}
	}
	if !strings.Contains(shell.stdout.String(), "ran in os-woken-0") {
		t.Fatalf("stdout = %q, want the command run in the started sandbox", shell.stdout.String())
	}
	ran := strings.Join(g.exec.ran(), "\n")
	for _, want := range []string{"os-woken-0 /bin/sh -lc uname -a", "os-woken-0 /bin/sh -lc echo zed", "os-woken-0 /bin/sh -c " + sftpServerCommand} {
		if !strings.Contains(ran, want) {
			t.Errorf("execs = %q, want %q", ran, want)
		}
	}
	g.waitForWake(t, "opened")
	if got := gatewaytest.MetricValue(t, g.registry, "bex_ssh_gateway_authentications_total", map[string]string{"result": "accepted"}); got != 1 {
		t.Fatalf("accepted authentications = %v, want 1", got)
	}

	// A channel opened once the sandbox is up runs at once, like any.
	later := start(t, client, "pwd")
	if err := later.wait(t); err != nil || later.stderr.String() != "" {
		t.Fatalf("a later channel: err %v, stderr %q", err, later.stderr.String())
	}
	if len(g.waker.requests()) != 1 {
		t.Fatalf("wake requests = %d, want the one the login made", len(g.waker.requests()))
	}
}

// A login that finds the sandbox already starting (resuming, or a turn's
// first sandbox) waits for it instead of being told to come back, and
// starts nothing.
func TestSSHIntoAStartingSessionWaitsForItsSandbox(t *testing.T) {
	for _, phase := range []string{agentsessions.PhaseResuming, agentsessions.PhaseCreating} {
		t.Run(phase, func(t *testing.T) {
			rows := &liveRows{}
			starting := rows.add(phase, "")
			g := startWakeGateway(t, "user-1", rows, []string{starting}, nil)

			shell := start(t, g.login(t, starting), "uname -a")
			shell.waitForStderr(t, "starting the sandbox")
			rows.set(starting, func(row *store.AgentSession) { row.Phase, row.SandboxID = agentsessions.PhaseRunning, "os-turn" })
			if err := shell.wait(t); err != nil || !strings.Contains(shell.stdout.String(), "ran in os-turn-0") {
				t.Fatalf("err %v, stdout %q, want the command in the turn's sandbox", err, shell.stdout.String())
			}
			if calls := g.waker.requests(); len(calls) != 0 {
				t.Fatalf("wake requests = %+v, want none for a session already starting", calls)
			}
			g.waitForWake(t, "opened")
		})
	}
}

// A caller who may view the session but not start it keeps w10/043's
// banner, and so does a login whose wake got no answer (an older bex-api
// answers the new path 404). A start bex-api refuses says why.
func TestSSHWakeRefusalsAreToldInTheBanner(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer agentsession.SessionWakeResponse
		err    error
		want   []string
		metric string
	}{
		{"may not start it", agentsession.SessionWakeResponse{Refusal: agentsession.SessionWakeForbidden},
			nil, []string{"is not running, and its sandbox has closed", "send the session a message"}, "refused"},
		{"older bex-api", agentsession.SessionWakeResponse{}, errors.New("session wake returned status 404"),
			[]string{"is not running, and its sandbox has closed", "send the session a message"}, "unavailable"},
		{"live-sandbox cap", agentsession.SessionWakeResponse{Refusal: agentsession.SessionWakeRefused, Code: "AGENT_SESSION_LIVE_LIMIT",
			Message: "this workspace already has 3 live agent-session sandboxes (limit 3); stop or cancel an idle session before starting another"},
			nil, []string{"is not running, and its sandbox could not start: this workspace already has 3 live agent-session sandboxes"}, "refused_start"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := &liveRows{}
			closed := rows.add(agentsessions.PhaseFailed, "")
			g := startWakeGateway(t, "user-1", rows, []string{closed}, nil)
			g.waker.answer, g.waker.err = tc.answer, tc.err

			client, banner, err := dialWithBanner(g.addr, closed, g.signer)
			if err == nil {
				client.Close()
				t.Fatal("the login was admitted, want the banner")
			}
			for _, want := range append([]string{"bex: agent session " + closed}, tc.want...) {
				if !strings.Contains(banner, want) {
					t.Fatalf("banner = %q, want it to say %q", banner, want)
				}
			}
			if !strings.HasSuffix(banner, ".\n") || strings.Count(banner, "\n") != 1 {
				t.Fatalf("banner = %q, want one sentence-ended line", banner)
			}
			// MaxAuthTries lets the client retry its one key: each try asks again.
			if got := g.wakes(t, tc.metric); got < 1 {
				t.Fatalf("session_wakes_total{result=%q} = %v, want the refusal counted", tc.metric, got)
			}
			if got := rows.phase(closed); got != agentsessions.PhaseFailed {
				t.Fatalf("phase = %q, want the refused session untouched", got)
			}
		})
	}
}

// Canceled and archived sessions keep their banners and start nothing; a
// session the caller may not view, or that does not exist, stays a bare
// refusal; a key enrolled by a bex API key whose grant does not cover
// sandbox execution is not told and starts nothing.
func TestSSHWakeStartsNothingForWhatItMayNotWake(t *testing.T) {
	rows := &liveRows{}
	canceled := rows.add(agentsessions.PhaseCanceled, "")
	archived := rows.add(agentsessions.PhaseCompleted, "")
	rows.set(archived, func(row *store.AgentSession) { row.ArchivedAt = new(time.Now()) })
	foreign := rows.add(agentsessions.PhaseCompleted, "")
	g := startWakeGateway(t, "user-1", rows, []string{canceled, archived}, nil)

	for id, want := range map[string]string{canceled: "was canceled", archived: "is archived"} {
		if _, banner, err := dialWithBanner(g.addr, id, g.signer); err == nil || !strings.Contains(banner, want) {
			t.Errorf("%s: err %v, banner %q, want a refusal saying %q", id, err, banner, want)
		}
	}
	for _, id := range []string{foreign, ids.New(ids.AgentSession)} {
		if _, banner, err := dialWithBanner(g.addr, id, g.signer); err == nil || banner != "" {
			t.Errorf("%s: err %v, banner %q, want the bare refusal", id, err, banner)
		}
	}

	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "services:read_write")
	closed := rows.add(agentsessions.PhaseCompleted, "")
	keyed := startWakeGateway(t, "key-1", rows, []string{closed}, func(s *Server) {
		s.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	})
	if _, banner, err := dialWithBanner(keyed.addr, closed, keyed.signer); err == nil || banner != "" {
		t.Errorf("a key without sandbox execution: err %v, banner %q, want the bare refusal", err, banner)
	}
	if calls := append(g.waker.requests(), keyed.waker.requests()...); len(calls) != 0 {
		t.Fatalf("wake requests = %+v, want none", calls)
	}
	// Once its grant covers the sandbox, the key's owner is asked about:
	// bex-api holds it to its grant for starting the session.
	grants.Set("key-1", "sandbox_execution:execute")
	keyed.login(t, closed)
	if calls := keyed.waker.requests(); len(calls) != 1 || calls[0].Subject != "key-1" {
		t.Fatalf("wake requests = %+v, want one as the key", calls)
	}
}

// The wake is off where the multi-channel carve-out is (MaxChannelsPerConn
// 0) or no verb is wired: the login keeps w10/043's banner.
func TestSSHWakeIsOffWithoutTheVerbOrTheCarveOut(t *testing.T) {
	for name, configure := range map[string]func(*Server){
		"no verb":         func(s *Server) { s.Wakes = nil },
		"single channels": func(s *Server) { s.MaxChannelsPerConn = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			rows := &liveRows{}
			closed := rows.add(agentsessions.PhaseCompleted, "")
			g := startWakeGateway(t, "user-1", rows, []string{closed}, configure)
			if _, banner, err := dialWithBanner(g.addr, closed, g.signer); err == nil || !strings.Contains(banner, "is not running") {
				t.Fatalf("err %v, banner %q, want the banner", err, banner)
			}
			if calls := g.waker.requests(); len(calls) != 0 {
				t.Fatalf("wake requests = %+v, want none", calls)
			}
		})
	}
}

// A start that ends without a sandbox, or outlasts the start's ceiling, is
// told on every waiting channel's stderr with exit status 1, and the
// connection closes.
func TestSSHWakeThatEndsWithoutASandboxSaysSoAndCloses(t *testing.T) {
	for _, tc := range []struct {
		name, want, metric string
		settle             bool
	}{
		{"the start failed", "did not start. Connect again to try again", "no_sandbox", true},
		{"the ceiling passed", "did not start within 300ms", "timed_out", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := &liveRows{}
			closed := rows.add(agentsessions.PhaseCompleted, "")
			g := startWakeGateway(t, "user-1", rows, []string{closed}, func(s *Server) {
				if !tc.settle {
					s.WakeTimeout = 300 * time.Millisecond
				}
			})
			client := g.login(t, closed)
			shell := start(t, client, "uname -a")
			sftp := start(t, client, "sftp")
			shell.waitForStderr(t, "starting the sandbox")
			if tc.settle {
				rows.settle(closed, "")
			}
			for name, c := range map[string]*command{"shell": shell, "sftp": sftp} {
				if err := c.wait(t); !errors.Is(err, exitStatus(1)) {
					t.Fatalf("%s: err = %v, want exit status 1", name, err)
				}
				if !strings.Contains(c.stderr.String(), "bex: the sandbox of agent session "+closed+" "+tc.want) {
					t.Fatalf("%s: stderr = %q, want it to say why", name, c.stderr.String())
				}
			}
			closed2 := make(chan error, 1)
			go func() { closed2 <- client.Wait() }()
			client.Close() // the client hangs up once its session ended, as ssh does
			select {
			case <-closed2:
			case <-time.After(5 * time.Second):
				t.Fatal("the connection stayed open")
			}
			g.waitForWake(t, tc.metric)
			if ran := g.exec.ran(); len(ran) != 0 {
				t.Fatalf("ran %v without a sandbox", ran)
			}
		})
	}
}

// A held login with no channel (Zed's `-N` master) is closed by the gateway
// as soon as the start ends without a sandbox: there is no one to tell.
func TestSSHWakeClosesAChannelLessLoginWhoseStartFailed(t *testing.T) {
	rows := &liveRows{}
	closed := rows.add(agentsessions.PhaseCompleted, "")
	g := startWakeGateway(t, "user-1", rows, []string{closed}, func(s *Server) { s.RevalidateInterval = time.Hour })
	client := g.login(t, closed)
	rows.settle(closed, "")
	g.waitForWake(t, "no_sandbox")
	ended := make(chan error, 1)
	go func() { ended <- client.Wait() }()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the gateway kept the connection open")
	}
}

// The wait ends with the connection, and with the gateway's sessions (a
// drain's bound): nothing runs, and it is counted abandoned.
func TestSSHWakeEndsWithTheConnection(t *testing.T) {
	for name, end := range map[string]func(g *wakeGateway, client *ssh.Client){
		"client hangs up":   func(_ *wakeGateway, client *ssh.Client) { client.Close() },
		"gateway shut down": func(g *wakeGateway, _ *ssh.Client) { g.end() },
	} {
		t.Run(name, func(t *testing.T) {
			rows := &liveRows{}
			closed := rows.add(agentsessions.PhaseCompleted, "")
			g := startWakeGateway(t, "user-1", rows, []string{closed}, nil)
			client := g.login(t, closed)
			shell := start(t, client, "uname -a")
			shell.waitForStderr(t, "starting the sandbox")
			end(g, client)
			g.waitForWake(t, "abandoned")
			rows.settle(closed, "os-late")
			time.Sleep(50 * time.Millisecond)
			if ran := g.exec.ran(); len(ran) != 0 {
				t.Fatalf("ran %v after the connection ended", ran)
			}
		})
	}
}

// Access revoked while the login waits ends the wait: nothing runs once the
// sandbox is up.
func TestSSHWakeEndsWhenAccessIsRevokedMeanwhile(t *testing.T) {
	rows := &liveRows{}
	closed := rows.add(agentsessions.PhaseCompleted, "")
	g := startWakeGateway(t, "user-1", rows, []string{closed}, nil)
	shell := start(t, g.login(t, closed), "uname -a")
	shell.waitForStderr(t, "starting the sandbox")
	rows.mu.Lock() // the session is deleted: the resolver finds no such session
	delete(rows.rows, closed)
	rows.mu.Unlock()
	if err := shell.wait(t); !errors.Is(err, exitStatus(1)) {
		t.Fatalf("err = %v, want exit status 1", err)
	}
	if !strings.Contains(shell.stderr.String(), "bex: "+reauthorizeFailed) {
		t.Fatalf("stderr = %q, want the generic refusal", shell.stderr.String())
	}
	g.waitForWake(t, "abandoned")
}

// A held channel says every 30 s that the sandbox is still starting, so a
// slow start never looks hung, and says nothing more once the wait ends
// (ADR054 D9). Production's starts settle in seconds, before the first
// line is due (w10/m597 t006), so only this test sees it.
func TestAHeldChannelSaysEvery30SecondsThatItStillWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &sandboxWait{session: "ags-held", done: make(chan struct{})}
		var stderr syncBuffer
		result := make(chan error, 1)
		go func() { result <- w.await(context.Background(), &stderr) }()

		starting := "bex: starting the sandbox of agent session ags-held. This connection continues once it is up.\n"
		time.Sleep(29 * time.Second)
		synctest.Wait()
		if got := stderr.String(); got != starting {
			t.Fatalf("stderr after 29 s = %q, want only %q", got, starting)
		}

		time.Sleep(32 * time.Second)
		synctest.Wait()
		want := starting + "bex: still starting the sandbox (30s).\nbex: still starting the sandbox (1m0s).\n"
		if got := stderr.String(); got != want {
			t.Fatalf("stderr after 61 s = %q, want %q", got, want)
		}

		close(w.done)
		if err := <-result; err != nil {
			t.Fatalf("await = %v, want nil once the sandbox is up", err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if got := stderr.String(); got != want {
			t.Fatalf("stderr after the wait ended = %q, want no more lines", got)
		}
	})
}

// bex-api's refusal message reaches the client's terminal, so it is one
// bounded line of printable text, cut on a character boundary.
func TestWakeBannerTextIsOneBoundedPrintableLine(t *testing.T) {
	if got := bannerText("payment method required.\x1b[2J\nthen\tmore."); got != "payment method required. [2J then more" {
		t.Fatalf("bannerText = %q", got)
	}
	long := bannerText(strings.Repeat("é", maxWakeMessageLength+10))
	if !utf8.ValidString(long) || utf8.RuneCountInString(long) != maxWakeMessageLength+1 || !strings.HasSuffix(long, "…") {
		t.Fatalf("a long message became %d runes, valid %v", utf8.RuneCountInString(long), utf8.ValidString(long))
	}
}
