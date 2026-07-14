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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// protection.go is the destructive-verb guard w6/m19 (protected-environment
// ACLs) adds on top of every App-lifecycle verb: Delete, Suspend, and
// applyCreate's redeploy-an-existing-service path (the "direct-deploy-
// override" the milestone names — a manual apply that overrides what's
// already running, as opposed to the git-push auto-deploy pipeline's
// unexported redeploy, which is NOT guarded: it is triggered by an HMAC-
// verified webhook, not a human, and Render's own protected environments
// don't block a service's own configured auto-deploy).
//
// Guarded verbs read the caller's confirmation phrase off the context
// (confirmFrom) rather than taking a new parameter, because they are reached
// through generic REST/GraphQL/MCP forwarding helpers shared with unguarded
// verbs (Restart, Resume, …) that all share a fixed func(ctx, name string)
// shape. This context seam is local to this package — unlike
// core.WithWorkspace (consumed by core.Base itself, every feature), no
// package outside apps needs a confirmation phrase, so it doesn't belong in
// the shared kernel.

type confirmKey struct{}

// withConfirm records a caller-supplied confirmation phrase for this request.
// Empty is a no-op (no confirmation offered).
func withConfirm(ctx context.Context, confirm string) context.Context {
	if confirm == "" {
		return ctx
	}
	return context.WithValue(ctx, confirmKey{}, confirm)
}

// confirmFrom returns the confirmation phrase the caller supplied, or "" if none.
func confirmFrom(ctx context.Context) string {
	confirm, _ := ctx.Value(confirmKey{}).(string)
	return confirm
}

// ProtectedConfirmation is the exact phrase a caller must echo back (REST
// ?confirm=, a GraphQL confirm arg, or an MCP confirm field) to act on a
// service that belongs to a protectedStatus=protected Environment. Mirrors
// workspaces.DeleteConfirmation's confirm-phrase shape exactly, parameterized
// by the verb so delete/suspend/deploy each get their own distinct phrase (a
// confirm typed for one destructive verb can't accidentally arm another).
func ProtectedConfirmation(verb, name string) string {
	return "sudo " + verb + " service " + name
}

// requireUnprotected blocks verb on a App belonging to a
// protectedStatus=protected Environment unless the context carries the
// matching ProtectedConfirmation phrase. A no-op for: the store being
// unwired (hand-applied/DB-less mode has no environment concept at all), or
// an App with no store row (same reason), or one whose environment is
// unprotected (or which has none) — protection is opt-in, so every App this
// milestone predates behaves byte-identically.
func (s *Service) requireUnprotected(ctx context.Context, a *appv1alpha1.App, verb string) error {
	if s.Store == nil {
		return nil
	}
	id := a.Labels[store.LabelAppID]
	if id == "" {
		return nil
	}
	protectedStatus, err := s.Store.GetAppProtectedStatus(ctx, id)
	if err != nil {
		return err
	}
	if protectedStatus != core.ProtectedStatusProtected {
		return nil
	}
	name := a.Labels[core.LabelServiceName]
	if name == "" {
		name = a.Name
	}
	if want := ProtectedConfirmation(verb, name); confirmFrom(ctx) != want {
		return fmt.Errorf("%w: %q is a member of a protected environment; retry with confirm=%q to %s it", core.ErrBadRequest, name, want, verb)
	}
	return nil
}
