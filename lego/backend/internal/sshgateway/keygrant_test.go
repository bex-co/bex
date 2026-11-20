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

package sshgateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// TestKeyGrantGateHoldsAKeysTicketToItsCurrentGrant (w9/125): the gate asks
// bex-api's real verb about a ticket a key minted, every time, and refuses it
// with bex-api's own coded 403 once the grant stops covering the ticket's
// action. A ticket that names no action never asks. Anything the gate cannot
// establish is ErrAuthzUnavailable, never an allow.
func TestKeyGrantGateHoldsAKeysTicketToItsCurrentGrant(t *testing.T) {
	ctx := context.Background()
	grants := &gatewaytest.KeyGrants{}
	gate := &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	jobs := string(core.KeyServicesJobs)

	grants.Set("key-1", "services:read_write")
	if err := gate.RequireTicket(ctx, "key-1", jobs); err != nil {
		t.Fatalf("a covering grant refused: %v", err)
	}
	grants.Set("key-1", "services:read")
	err := gate.RequireTicket(ctx, "key-1", jobs)
	var coded *core.CodedError
	if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != core.KeyPermissionDeniedCode {
		t.Fatalf("a narrowed grant = %v, want the coded %s 403", err, core.KeyPermissionDeniedCode)
	}
	grants.SetAll("key-1")
	if err := gate.RequireTicket(ctx, "key-1", jobs); err != nil {
		t.Fatalf("a widened grant refused: %v", err)
	}

	before := grants.Reads()
	if err := gate.RequireTicket(ctx, "person", ""); err != nil || grants.Reads() != before {
		t.Fatalf("a ticket naming no action = %v after %d questions, want nil without asking", err, grants.Reads()-before)
	}
	if err := (*sshgateway.KeyGrantGate)(nil).RequireTicket(ctx, "person", ""); err != nil {
		t.Fatalf("an unwired gate refused a person's ticket: %v", err)
	}

	grants.Fail(errors.New("postgres down"))
	if err := gate.RequireTicket(ctx, "key-1", jobs); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("an unreadable grant = %v, want ErrAuthzUnavailable", err)
	}
	for name, unwired := range map[string]*sshgateway.KeyGrantGate{"nil gate": nil, "no asker": {}} {
		if err := unwired.RequireTicket(ctx, "key-1", jobs); !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Errorf("%s = %v, want ErrAuthzUnavailable for a key's ticket", name, err)
		}
	}
}

// RequireSubject tells a native SSH key's owner apart: a person is allowed
// and is not a key, so the connection never asks again; a key is held to its
// grant like a ticket.
func TestKeyGrantGateTellsAPersonFromAKey(t *testing.T) {
	ctx := context.Background()
	grants := &gatewaytest.KeyGrants{}
	gate := &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	exec := string(core.KeySandboxesExec)

	if key, err := gate.RequireSubject(ctx, "person", exec); err != nil || key {
		t.Fatalf("a person = (key %v, %v), want an allowed non-key", key, err)
	}
	grants.Set("key-1", "sandbox_execution:execute")
	if key, err := gate.RequireSubject(ctx, "key-1", exec); err != nil || !key {
		t.Fatalf("a covered key = (key %v, %v), want an allowed key", key, err)
	}
	grants.Set("key-1", "sandboxes:read_write")
	if _, err := gate.RequireSubject(ctx, "key-1", exec); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("an uncovered key = %v, want ErrForbidden", err)
	}
	grants.Fail(errors.New("postgres down"))
	if _, err := gate.RequireSubject(ctx, "person", exec); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("an unanswered question = %v, want ErrAuthzUnavailable", err)
	}
}

// The sandbox exec and file revalidator adds the key's grant after the
// subject's relations, and never asks about bex-api's own tickets.
func TestExecRevalidatorHoldsAKeysExecToItsGrant(t *testing.T) {
	ctx := context.Background()
	grants := &gatewaytest.KeyGrants{}
	revalidator := &sshgateway.ExecRevalidator{
		Base:      &core.Base{Authz: allowAll{}},
		KeyGrants: &sshgateway.KeyGrantGate{Asker: grants.Client(t)},
	}
	claims := sandboxexec.Claims{
		Subject: "key-1", SandboxID: "os-1", Namespace: "tea-a-sandbox", Workspace: "tea-a",
		Command: []string{"sh"}, KeyAction: string(core.KeySandboxesExec),
	}
	grants.Set("key-1", "sandbox_execution:execute")
	if err := revalidator.RevalidateExec(ctx, claims); err != nil {
		t.Fatalf("a covered key's exec: %v", err)
	}
	grants.Set("key-1")
	if err := revalidator.RevalidateExec(ctx, claims); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a key cleared of every permission = %v, want ErrForbidden", err)
	}
	grants.Fail(errors.New("postgres down"))
	before := grants.Reads()
	system := claims
	system.Subject, system.KeyAction = sandboxexec.SystemSubject, ""
	person := claims
	person.Subject, person.KeyAction = "person", ""
	for _, c := range []sandboxexec.Claims{system, person} {
		if err := revalidator.RevalidateExec(ctx, c); err != nil {
			t.Errorf("%s's exec = %v, want nil with the verb down", c.Subject, err)
		}
	}
	if grants.Reads() != before {
		t.Errorf("tickets naming no action asked the verb %d times", grants.Reads()-before)
	}
}

type allowAll struct{}

func (allowAll) Check(context.Context, string, string, string) (bool, error) { return true, nil }
