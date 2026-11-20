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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
)

// w9/125: bex-api marks the gateway ticket of an exec or a file transfer a
// bex API key starts with the action its mint needed, so the gateway holds
// the stream to the key's grant; a person's, and bex-api's own, carry none.

func keyIdentityCtx(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, ClientID: subject, Method: core.MethodOAuth2})
}

// heldExec runs every command until its context ends.
type heldExec struct{ started chan struct{} }

func (e heldExec) Execute(ctx context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, _, _ io.Writer) (int, error) {
	select {
	case e.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return 0, ctx.Err()
}

// TestAKeysExecEndsWhenItsGrantNarrows drives the whole path: bex-api mints
// the exec ticket under a key's identity, the real gateway redeems it and
// holds it to the key's grant over bex-api's real grant verb, and narrowing
// the grant ends the stream the key's client is reading, whether the key
// called /exec or redeemed a run connect token it minted.
func TestAKeysExecEndsWhenItsGrantNarrows(t *testing.T) {
	for _, path := range []string{"exec", "connect token"} {
		t.Run(path, func(t *testing.T) {
			secret := []byte("exec-secret")
			grants := &gatewaytest.KeyGrants{}
			grants.Set("key-1", "sandbox_execution:execute")
			exec := heldExec{started: make(chan struct{}, 1)}
			gateway := httptest.NewServer((&sandboxsse.Server{
				Secret: secret, Executor: exec,
				Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
				Limits:  sshgateway.NewSessionLimiter(100, 5), SessionTimeout: time.Minute,
				Revalidator: &sshgateway.ExecRevalidator{
					Base:      &core.Base{Authz: allowEverything{}},
					KeyGrants: &sshgateway.KeyGrantGate{Asker: grants.Client(t)},
				},
				RevalidateInterval: 10 * time.Millisecond,
			}).Handler())
			t.Cleanup(gateway.Close)
			svc := &Service{
				Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"key-1": "tea-a"}},
				Client: execSandboxClient(t, "key-1"),
				Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client(), TTL: time.Minute},
			}

			body := make(chan string, 1)
			switch path {
			case "exec":
				rr := httptest.NewRecorder()
				go func() {
					_ = svc.StreamExec(keyIdentityCtx("key-1"), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "sleep 300"}, rr, func() {})
					body <- rr.Body.String()
				}()
			case "connect token":
				api := httptest.NewServer(connectMux(svc))
				t.Cleanup(api.Close)
				run, err := svc.ConnectRun(keyIdentityCtx("key-1"), ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: ConnectOperationStream, Command: "sleep 300"}, api.URL)
				if err != nil {
					t.Fatal(err)
				}
				uri, _ := url.Parse(run.URI)
				req, _ := http.NewRequest(http.MethodPost, api.URL+uri.Path, strings.NewReader(`{"command":"sleep 300"}`))
				req.Header.Set("Authorization", "Bearer "+run.Token)
				go func() {
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						body <- err.Error()
						return
					}
					defer resp.Body.Close()
					b, _ := io.ReadAll(resp.Body)
					body <- string(b)
				}()
			}
			select {
			case <-exec.started:
			case <-time.After(3 * time.Second):
				t.Fatal("the key's command never started")
			}
			grants.Set("key-1", "sandboxes:read")
			select {
			case got := <-body:
				if !strings.Contains(got, "revoked") {
					t.Fatalf("the key's stream ended without the revocation event:\n%s", got)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("narrowing the key's grant did not end its exec")
			}
		})
	}
}

type allowEverything struct{}

func (allowEverything) Check(context.Context, string, string, string) (bool, error) { return true, nil }

// ticketActions is a gateway that records the KeyAction of every exec ticket
// it is handed and answers one finished command.
type ticketActions struct {
	mu   sync.Mutex
	seen []string
}

func (g *ticketActions) serve(secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil {
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
			return
		}
		g.mu.Lock()
		g.seen = append(g.seen, claims.KeyAction)
		g.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"{\\\"state\\\":\\\"succeeded\\\"}\"}\n\nevent: exit\ndata: {\"exit_code\":0}\n\n"))
	}
}

func (g *ticketActions) last(t *testing.T) string {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.seen) == 0 {
		t.Fatal("no ticket reached the gateway")
	}
	return g.seen[len(g.seen)-1]
}

func TestOnlyAKeysExecTicketNamesItsAction(t *testing.T) {
	secret := []byte("exec-secret")
	key, person := keyIdentityCtx("key-1"), identityCtx("id-a")
	exec := ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}

	for name, tc := range map[string]struct {
		owner string
		run   func(*Service) error
		want  string
	}{
		"a key's exec": {"key-1", func(s *Service) error {
			return s.StreamExec(key, exec, httptest.NewRecorder(), func() {})
		}, string(ExecKeyAction)},
		"a key's MCP exec": {"key-1", func(s *Service) error {
			_, err := s.ExecBuffered(key, exec)
			return err
		}, string(ExecKeyAction)},
		"a person's exec": {"id-a", func(s *Service) error {
			return s.StreamExec(person, exec, httptest.NewRecorder(), func() {})
		}, ""},
		"a key's run connect token":    {"key-1", func(s *Service) error { return redeemRun(t, s, key) }, string(ExecKeyAction)},
		"a person's run connect token": {"id-a", func(s *Service) error { return redeemRun(t, s, person) }, ""},
	} {
		t.Run(name, func(t *testing.T) {
			tickets := &ticketActions{}
			gateway := httptest.NewServer(tickets.serve(secret))
			t.Cleanup(gateway.Close)
			if err := tc.run(&Service{
				Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{tc.owner: "tea-a"}},
				Client: execSandboxClient(t, tc.owner),
				Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client(), TTL: time.Minute},
			}); err != nil {
				t.Fatal(err)
			}
			if got := tickets.last(t); got != tc.want {
				t.Fatalf("the gateway ticket's KeyAction = %q, want %q", got, tc.want)
			}
		})
	}
}

// bex-api's own commands are not the key's to lose: a status read made while
// a key's identity is in the context (a request the key made) names nothing,
// so a later narrowing never stops work the key started (ADR118 § Accepted,
// by design).
func TestBexAPIsOwnExecNamesNoAction(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sb := osSandbox{ID: "os-agent", Metadata: map[string]string{
			metadataOwner: "key-1", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
		}}
		sb.Status.State = "running"
		_ = json.NewEncoder(w).Encode(sb)
	}))
	t.Cleanup(upstream.Close)
	secret := []byte("exec-secret")
	tickets := &ticketActions{}
	gateway := httptest.NewServer(tickets.serve(secret))
	t.Cleanup(gateway.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"key-1": "tea-a"}},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
	}
	if _, err := (&AgentSessionLifecycle{service: svc}).ReadSessionStatus(keyIdentityCtx("key-1"), "tea-a", "ags-one", "os-agent"); err != nil {
		t.Fatal(err)
	}
	if got := tickets.last(t); got != "" {
		t.Fatalf("bex-api's own status read named %q", got)
	}
}

// redeemRun mints a run connect token as ctx's caller and redeems it.
func redeemRun(t *testing.T, s *Service, ctx context.Context) error {
	t.Helper()
	api := httptest.NewServer(connectMux(s))
	defer api.Close()
	run, err := s.ConnectRun(ctx, ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: ConnectOperationStream, Command: "true"}, api.URL)
	if err != nil {
		return err
	}
	uri, _ := url.Parse(run.URI)
	req, _ := http.NewRequest(http.MethodPost, api.URL+uri.Path, bytes.NewReader([]byte(`{"command":"true"}`)))
	req.Header.Set("Authorization", "Bearer "+run.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("redeem answered %d: %s", resp.StatusCode, b)
	}
	return nil
}

// connectMux serves the run connect token's redeem route.
func connectMux(s *Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(ConnectStreamPattern, s.ConnectStreamHandler())
	return mux
}

// A key's file token names Upload and download files, and the gateway ticket
// its redemption signs carries it; a person's carries none.
func TestOnlyAKeysFileTransferTicketNamesItsAction(t *testing.T) {
	for name, tc := range map[string]struct {
		ctx   context.Context
		owner string
		want  string
	}{
		"a key's":    {keyIdentityCtx("key-1"), "key-1", string(FileKeyAction)},
		"a person's": {identityCtx("id-a"), "id-a", ""},
	} {
		t.Run(name, func(t *testing.T) {
			var got []string
			f := newFileConnectFixture(t, tc.owner, nil, func(w http.ResponseWriter, r *http.Request) {
				claims, err := sandboxfiles.Verify([]byte("file-connect-test-secret"), r.Header.Get(sandboxfiles.TicketHeader), time.Now())
				if err == nil {
					got = append(got, claims.KeyAction)
				}
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(http.StatusNoContent)
			})
			f.svc.Workspace.(fakeWorkspace)[tc.owner] = "tea-a"
			out, err := f.svc.ConnectFile(tc.ctx, FileConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: sandboxfiles.OperationUpload, Path: "/workspace/file"}, f.api.URL)
			if err != nil {
				t.Fatal(err)
			}
			if resp := redeemFile(t, out, strings.NewReader("bytes")); resp.StatusCode != http.StatusNoContent {
				t.Fatalf("transfer answered %d", resp.StatusCode)
			}
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("the gateway file ticket's KeyAction = %v, want [%q]", got, tc.want)
			}
		})
	}
}
