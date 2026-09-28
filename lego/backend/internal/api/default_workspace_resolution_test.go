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

package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

type failingDefaultWorkspace struct{ err error }

func (f failingDefaultWorkspace) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-home", true
}
func (f failingDefaultWorkspace) IsMember(context.Context, core.Identity, string) (bool, error) {
	return false, f.err
}

type unexpectedDefaultWorkspaceCheck struct{ calls int }

func (c *unexpectedDefaultWorkspaceCheck) Check(context.Context, string, string, string) (bool, error) {
	c.calls++
	return true, nil
}
func (c *unexpectedDefaultWorkspaceCheck) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return c.Check(ctx, subject, relation, object)
}

type workspaceResolutionAudit struct{ events []core.AuditEvent }

func (a *workspaceResolutionAudit) Record(_ context.Context, ev core.AuditEvent) error {
	a.events = append(a.events, ev)
	return nil
}

func TestOmittedWorkspaceNeverFallsBackAfterResolutionFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cause, want error
	}{
		{"not a member", nil, core.ErrForbidden},
		{"membership unavailable", errors.New("membership store unavailable"), core.ErrAuthzUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := &unexpectedDefaultWorkspaceCheck{}
			sink := &workspaceResolutionAudit{}
			base := &core.Base{Workspace: failingDefaultWorkspace{err: tc.cause}, Authz: checker, Audit: sink}
			projectSvc := &projects.Service{Base: base}
			memberSvc := &members.Service{Base: base}
			workspaceSvc := &workspaces.Service{Base: base}
			environmentSvc := &environments.Service{Base: base}
			auditSvc := &audit.Service{Base: base}
			ctx := core.WithWorkspace(core.WithIdentity(context.Background(), core.Identity{Subject: "member", Method: "session"}), "tea-foreign")
			calls := map[string]func() error{
				"projects list":     func() error { _, err := projectSvc.List(ctx, ""); return err },
				"projects create":   func() error { _, err := projectSvc.CreateWithEnvironments(ctx, "", "name", nil); return err },
				"environments list": func() error { _, err := environmentSvc.ListWorkspace(ctx, ""); return err },
				"audit list":        func() error { _, err := auditSvc.List(ctx, "", audit.Filter{}); return err },
				"members list":      func() error { _, err := memberSvc.List(ctx, ""); return err },
				"seat usage":        func() error { _, err := memberSvc.SeatUsage(ctx, ""); return err },
				"list invites":      func() error { _, err := memberSvc.ListInvites(ctx, ""); return err },
				"invite":            func() error { _, err := memberSvc.Invite(ctx, "", "test@example.com", "viewer"); return err },
				"resend invite":     func() error { _, err := memberSvc.ResendInvite(ctx, "", "inv-id"); return err },
				"change role":       func() error { _, err := memberSvc.ChangeRole(ctx, "", "member", "viewer"); return err },
				"remove member":     func() error { return memberSvc.Remove(ctx, "", "member") },
				"leave workspace":   func() error { return memberSvc.LeaveWorkspace(ctx, "") },
				"revoke invite":     func() error { return memberSvc.RevokeInvite(ctx, "", "inv-id") },
				"get workspace":     func() error { _, err := workspaceSvc.GetWorkspace(ctx, ""); return err },
				"workspace members": func() error { _, err := workspaceSvc.ListMembers(ctx, ""); return err },
				"rename workspace":  func() error { _, err := workspaceSvc.Rename(ctx, "", "new-name"); return err },
				"change plan":       func() error { _, err := workspaceSvc.ChangePlan(ctx, "", "pro"); return err },
				"delete workspace":  func() error { return workspaceSvc.Delete(ctx, "", "name") },
			}
			for name, call := range calls {
				t.Run(name, func(t *testing.T) {
					if err := call(); !errors.Is(err, tc.want) {
						t.Fatalf("error=%v want %v", err, tc.want)
					}
				})
			}
			if len(sink.events) != len(calls) {
				t.Fatalf("denied audit count=%d want %d", len(sink.events), len(calls))
			}
			for _, ev := range sink.events {
				if ev.Resource != "workspace:tea-foreign" || ev.Outcome != core.AuditDenied || strings.Contains(ev.Verb, "AuthorizeWorkspace") {
					t.Fatalf("wrong denied audit: %+v", ev)
				}
				if ev.Verb == "members.RevokeInvite" && ev.Target != core.InviteTarget("inv-id") {
					t.Fatalf("lost invite target: %+v", ev)
				}
			}
			if checker.calls != 0 {
				t.Fatalf("resolution failure reached default-workspace checker %d times", checker.calls)
			}
			// Explicit resource scope continues through its own authorization, independent
			// of a caller's selected-workspace override.
			if _, err := projectSvc.List(ctx, "tea-explicit"); !errors.Is(err, projects.ErrProjectsUnavailable) {
				t.Fatalf("explicit workspace changed behavior: %v", err)
			}
			if checker.calls != 1 {
				t.Fatalf("explicit workspace check count=%d want1", checker.calls)
			}
		})
	}
}
