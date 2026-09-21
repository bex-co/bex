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
	"errors"
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
// (core.ConfirmFrom) rather than taking a new parameter, because they are
// reached through generic REST/GraphQL/MCP forwarding helpers shared with
// unguarded verbs (Restart, Resume, …) that all share a fixed
// func(ctx, name string) shape. The context seam lives in core because Apps,
// Postgres, and Key Value now share the same transport-independent
// confirmation mechanism (core.WithConfirm records the phrase).

// ProtectedConfirmation is the exact phrase a caller must echo back (REST
// ?confirm=, a GraphQL confirm arg, or an MCP confirm field) to act on a
// service that belongs to a protectedStatus=protected Environment. Mirrors
// workspaces.DeleteConfirmation's confirm-phrase shape exactly, parameterized
// by the verb so delete/suspend/deploy each get their own distinct phrase (a
// confirm typed for one destructive verb can't accidentally arm another).
func ProtectedConfirmation(verb, name string) string {
	return "sudo " + verb + " service " + name
}

// The guarded set and the rule that generates it (w4/m126, recorded in
// ADR032): a protected environment guards a verb that can lose data, change
// what the resource IS, or take it offline — the same rule w4/m127 applied to
// datastores. For a service, "what it is" is the code it runs, which yields
// three confirmation verbs on top of delete/suspend/direct-deploy:
//
//   - "repoint"      — image, repo, branch, registry credential (which code)
//   - "redefine"     — build/start/pre-deploy/cron commands, Dockerfile path,
//     root dir (how it is built and what it executes)
//   - "take offline" — enabling maintenance mode, which 503s every host
//
// One phrase per class, not per setter: the per-verb design exists so a confirm
// typed for a cheap verb cannot arm an expensive one, and within a class the
// stakes are identical.
//
// The alternative rule considered and rejected — "guard every verb that mints a
// release" — is mechanically tidier and fails on the very finding that opened
// the milestone: setImage mints NO release (the image is staged and applied by
// the next deploy), so a release-based rule would have missed the headline bug
// while guarding a health-check-path edit that cannot hurt anyone.
//
// protectedSourceVerb is that rule applied to a source patch: the confirmation
// it needs on a protected member, or "" when it changes neither.
func protectedSourceVerb(patch sourcePatch) string {
	switch {
	case patch.Image != nil || patch.Repo != nil:
		// Repointing at different code. Worse than it looks: the change is
		// invisible until an unrelated trigger applies it, so nobody watching
		// the service sees a rollout at the moment the decision was made.
		return "repoint"
	case patch.Branch != nil || patch.RegistryCredentialID != nil:
		return "repoint"
	default:
		return ""
	}
}

// appProtected is the predicate half of requireUnprotected — shared with the
// capability projection (ADR087, w6/m136) so what it reports and what the
// guard enforces are structurally the same answer. False for: the store being
// unwired (hand-applied/DB-less mode has no environment concept at all), an
// App with no store row (same reason), or one whose environment is
// unprotected (or which has none) — protection is opt-in.
func (s *Service) appProtected(ctx context.Context, a *appv1alpha1.App) (bool, error) {
	if s.Store == nil {
		return false, nil
	}
	id := managedAppID(a)
	if id == "" {
		return false, nil
	}
	protectedStatus, err := s.Store.GetAppProtectedStatus(ctx, id)
	if err != nil {
		// Delete is deliberately row-first. If a process dies or the request is
		// cancelled after removing that source row but before deleting the CR,
		// the retry must still remove the orphaned Kubernetes object. A missing
		// row has no Environment whose protection could be bypassed.
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return protectedStatus == core.ProtectedStatusProtected, nil
}

// requireUnprotected blocks verb on a App belonging to a
// protectedStatus=protected Environment unless the context carries the
// matching ProtectedConfirmation phrase.
func (s *Service) requireUnprotected(ctx context.Context, a *appv1alpha1.App, verb string) error {
	protected, err := s.appProtected(ctx, a)
	if err != nil {
		return err
	}
	if !protected {
		return nil
	}
	name := a.Labels[core.LabelServiceName]
	if name == "" {
		name = a.Name
	}
	if want := ProtectedConfirmation(verb, name); core.ConfirmFrom(ctx) != want {
		return fmt.Errorf("%w: %q is a member of a protected environment; retry with confirm=%q to %s it", core.ErrBadRequest, name, want, verb)
	}
	return nil
}
