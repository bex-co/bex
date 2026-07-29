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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

func TestStreamExecAuthorizesMintsTicketAndRelaysSSE(t *testing.T) {
	secret := []byte("exec-secret")
	var gotNamespace, gotSandbox string
	var gotCommand []string
	// Stub gateway: verify the ticket bex-api minted, then emit the CLI's SSE.
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		gotNamespace, gotSandbox, gotCommand = claims.Namespace, claims.SandboxID, claims.Command
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hi\\n\"}\n\nevent: exit\ndata: {\"exitCode\":0}\n\n"))
	}))
	defer gw.Close()

	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	rr := httptest.NewRecorder()
	err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush)
	if err != nil {
		t.Fatalf("StreamExec: %v", err)
	}
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("resp code=%d ct=%q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if body := rr.Body.String(); !strings.Contains(body, `"stream":"stdout"`) || !strings.Contains(body, `"exitCode":0`) {
		t.Errorf("relayed body missing events:\n%s", body)
	}
	// bex-api derived the sandbox namespace from the RESOLVED workspace and signed
	// the exact command — the scoping + integrity guarantees.
	if gotNamespace != "tea-a-sandbox" || gotSandbox != "os-1" {
		t.Errorf("ticket namespace/sandbox = %q/%q", gotNamespace, gotSandbox)
	}
	if len(gotCommand) != 3 || gotCommand[0] != "/bin/sh" || gotCommand[2] != "echo hi" {
		t.Errorf("signed command = %v", gotCommand)
	}
}

func TestStreamExecUnavailableWhenUnconfigured(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}}
	rr := httptest.NewRecorder()
	err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush)
	if !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Errorf("unconfigured err = %v, want ErrSandboxesUnavailable", err)
	}
}

func TestStreamExecAuthzRefusalNeverReachesGateway(t *testing.T) {
	gw := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("authz-refused exec must not reach the gateway")
	}))
	defer gw.Close()
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}, Authz: denyChecker{}},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: []byte("s"), GatewayURL: gw.URL, Client: gw.Client()},
	}
	rr := httptest.NewRecorder()
	if err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"}, rr, rr.Flush); err == nil {
		t.Fatal("expected authz refusal")
	}
}

func TestExecBufferedParsesSSE(t *testing.T) {
	secret := []byte("exec-secret")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"hi\\n\"}\n\n" +
			"event: output\ndata: {\"stream\":\"stderr\",\"data\":\"oops\\n\"}\n\n" +
			"event: exit\ndata: {\"exitCode\":7}\n\n"))
	}))
	defer gw.Close()
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient("http://unused"),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
	}
	res, err := svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stdout != "hi\n" || res.Stderr != "oops\n" || res.ExitCode != 7 {
		t.Errorf("buffered result = %+v", res)
	}
}
