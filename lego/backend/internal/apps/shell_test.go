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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
)

func shellService() *Service {
	return &Service{
		Base:              &core.Base{Client: fakeClient(sshApp(), sshPod("web-rs-pod01", "example/web:v2", true)), Namespace: "default"},
		SSHHost:           "ssh.bex.co",
		ShellTicketSecret: []byte("web-shell-secret"),
		ShellWSURL:        "wss://ssh.bex.co/shell",
	}
}

func operatorCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: "session"})
}

func TestCreateShellSessionMintsVerifiableTicket(t *testing.T) {
	svc := shellService()
	view, err := svc.CreateShellSession(operatorCtx(), sshServiceID, "")
	if err != nil {
		t.Fatalf("CreateShellSession: %v", err)
	}
	if view.URL != "wss://ssh.bex.co/shell" || view.Ticket == "" {
		t.Fatalf("unexpected view: %+v", view)
	}
	claims, err := shellticket.Verify(svc.ShellTicketSecret, view.Ticket, time.Now())
	if err != nil {
		t.Fatalf("minted ticket did not verify: %v", err)
	}
	if claims.Subject != "user-1" || claims.ServiceID != sshServiceID || claims.InstanceID != "" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestCreateShellSessionPinsInstance(t *testing.T) {
	svc := shellService()
	instanceID := ids.DeriveServiceInstance(sshServiceID, "web-rs-pod01")
	view, err := svc.CreateShellSession(operatorCtx(), sshServiceID, instanceID)
	if err != nil {
		t.Fatalf("CreateShellSession: %v", err)
	}
	claims, err := shellticket.Verify(svc.ShellTicketSecret, view.Ticket, time.Now())
	if err != nil || claims.InstanceID != instanceID {
		t.Fatalf("pinned instance not carried: %+v, %v", claims, err)
	}
	if claims.Username() != instanceID {
		t.Errorf("Username() = %q, want the pinned instance", claims.Username())
	}
}

func TestCreateShellSessionUnconfiguredIs503(t *testing.T) {
	svc := shellService()
	svc.ShellTicketSecret = nil
	if _, err := svc.CreateShellSession(operatorCtx(), sshServiceID, ""); !errors.Is(err, core.ErrShellUnavailable) {
		t.Errorf("no secret => %v, want ErrShellUnavailable", err)
	}
	svc = shellService()
	svc.ShellWSURL = ""
	if _, err := svc.CreateShellSession(operatorCtx(), sshServiceID, ""); !errors.Is(err, core.ErrShellUnavailable) {
		t.Errorf("no url => %v, want ErrShellUnavailable", err)
	}
}

func TestCreateShellSessionForeignInstanceIsBadRequest(t *testing.T) {
	svc := shellService()
	if _, err := svc.CreateShellSession(operatorCtx(), sshServiceID, "srv-someoneelse-pod9"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("foreign instance => %v, want ErrBadRequest", err)
	}
}

func TestCreateShellSessionRequiresIdentity(t *testing.T) {
	svc := shellService()
	if _, err := svc.CreateShellSession(context.Background(), sshServiceID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("no identity => %v, want ErrForbidden", err)
	}
}

func TestREST_ShellTicketStatuses(t *testing.T) {
	// Unconfigured transport => 503, and no ticket leaks.
	bare, _ := newService(nil, sshApp())
	bareMux := http.NewServeMux()
	bare.RegisterREST(bareMux)
	rec := httptest.NewRecorder()
	bareMux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/"+sshServiceID+"/shell-ticket", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured => 503, got %d", rec.Code)
	}

	// Configured + an authenticated caller => 201 with a ticket for the gateway URL.
	svc, _ := newService(nil, sshApp())
	svc.ShellTicketSecret = []byte("web-shell-secret")
	svc.ShellWSURL = "wss://ssh.bex.co/shell"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/services/"+sshServiceID+"/shell-ticket", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-1", Method: "session"}))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("configured => 201, got %d: %s", rec.Code, rec.Body)
	}
	var out ShellSessionView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Ticket == "" || out.URL != "wss://ssh.bex.co/shell" {
		t.Errorf("unexpected body: %+v", out)
	}
	if _, err := shellticket.Verify(svc.ShellTicketSecret, out.Ticket, time.Now()); err != nil {
		t.Errorf("REST-minted ticket did not verify: %v", err)
	}
}

func TestCreateShellSessionSuspendedIsConflict(t *testing.T) {
	app := sshApp()
	app.Spec.Suspended = true
	svc := &Service{
		Base:              &core.Base{Client: fakeClient(app), Namespace: "default"},
		SSHHost:           "ssh.bex.co",
		ShellTicketSecret: []byte("web-shell-secret"),
		ShellWSURL:        "wss://ssh.bex.co/shell",
	}
	if _, err := svc.CreateShellSession(operatorCtx(), sshServiceID, ""); !errors.Is(err, core.ErrConflict) {
		t.Errorf("suspended => %v, want ErrConflict", err)
	}
}
