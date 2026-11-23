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

package core

import (
	"context"
	"errors"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

type confirmationKey struct{}

// WithConfirm records a caller-supplied confirmation phrase for this request.
// Empty is a no-op, preserving the original context for ordinary calls.
func WithConfirm(ctx context.Context, confirm string) context.Context {
	if confirm == "" {
		return ctx
	}
	return context.WithValue(ctx, confirmationKey{}, confirm)
}

// ConfirmFrom returns the confirmation phrase supplied by the caller, or "".
func ConfirmFrom(ctx context.Context) string {
	confirm, _ := ctx.Value(confirmationKey{}).(string)
	return confirm
}

// EnvironmentProtectionStore resolves the protection state for resources whose
// Environment membership lives on their Kubernetes object rather than in an
// App row in the control-plane database.
type EnvironmentProtectionStore interface {
	GetEnvironmentProtectedStatus(ctx context.Context, environmentID string) (string, error)
}

// EnvironmentProtected is the predicate half of RequireEnvironmentAdmin —
// shared with the capability projections (ADR087, w6/m136; RestrictedDecision)
// so what a projection reports and what the guard enforces are structurally
// the same answer. Empty environment/store are unprotected, exactly like the
// guard.
func EnvironmentProtected(ctx context.Context, store EnvironmentProtectionStore, environmentID string) (bool, error) {
	if store == nil || environmentID == "" {
		return false, nil
	}
	protectedStatus, err := store.GetEnvironmentProtectedStatus(ctx, environmentID)
	if err != nil {
		return false, err
	}
	return protectedStatus == ProtectedStatusProtected, nil
}

// ProtectedVerbRestricted reports whether verb is one of the actions Render
// restricts to workspace Admins in a protected environment
// (render.com/docs/projects): creating resources in the environment
// ("create"), moving resources into or out of it ("move"; PlacementRefusal
// below), deleting the environment's resources, suspending or resuming
// services, turning maintenance mode on ("take offline") and shell access
// ("shell"; environment variables and secret files are the secrets package's,
// w8/193). The rest of a service's life — deploys, rollbacks, settings,
// queries — is open to anyone who may do it elsewhere, and no verb asks for a
// typed phrase (w8/187; the phrase gate of w4/m126 and w4/m127 is retired).
// The dashboard may still confirm in its own UI.
func ProtectedVerbRestricted(verb string) bool {
	switch verb {
	case "create", "move", "delete", "suspend", "resume", "take offline", "shell":
		return true
	}
	return false
}

// ProtectedPlacementCode is the 403 a non-Admin receives for creating a
// resource in, or moving one into or out of, a protected environment (w3/m281).
// A coded refusal, unlike the bare FORBIDDEN the other restricted verbs
// return, so every surface — and the dashboard's raced-403 path — can name the
// reason without parsing the message.
const ProtectedPlacementCode = "PROTECTED_ENVIRONMENT_ADMIN_REQUIRED"

// ProtectedPlacementRefusal is the ProtectedPlacementCode refusal for
// environmentID (either id spelling; the params carry the public one).
func ProtectedPlacementRefusal(environmentID string) *CodedError {
	return NewForbiddenError(ProtectedPlacementCode,
		"only workspace Admins can create resources in, or move resources into or out of, a protected environment",
		map[string]any{"environmentId": ids.EnvironmentPublicID(environmentID)})
}

// PlacementRefusal is err, a fresh workspace-manage check's answer for a
// "create" or "move" in the protected environment environmentID, with its role
// refusal coded as ProtectedPlacementRefusal. nil and any other error (an
// unreachable authorizer) pass through, so an outage never reads as a role
// refusal.
func PlacementRefusal(environmentID string, err error) error {
	if errors.Is(err, ErrForbidden) {
		return ProtectedPlacementRefusal(environmentID)
	}
	return err
}

// RequireEnvironmentAdmin refuses a restricted verb on a member of a protected
// environment unless admin (a fresh workspace-manage check) passes. Empty
// environment/store are no-ops so DB-less and environment-less resources keep
// their original behavior, as do verbs Render leaves open.
func RequireEnvironmentAdmin(ctx context.Context, store EnvironmentProtectionStore, environmentID, verb string, admin func() error) error {
	if !ProtectedVerbRestricted(verb) {
		return nil
	}
	protected, err := EnvironmentProtected(ctx, store, environmentID)
	if err != nil {
		return err
	}
	if !protected {
		return nil
	}
	return admin()
}

// RestrictedDecision projects RequireEnvironmentAdmin onto a capability
// decision (ADR087, w4/m578): what a projection reports for verb, given
// operate (the permission the verb itself opens with), protected (the guard's
// own predicate) and admin (the guard's own fresh workspace-manage check). A
// verb Render leaves open, and any verb on an unprotected member, reads as
// operate. On a protected member a restricted verb reads as admin's answer,
// with a role refusal reported as ReasonProtectedEnvironmentAdmin so a client
// can say only an Admin may. Neither read runs unless operate is allowed: a
// Viewer pays no protection or admin read and learns nothing from them. A
// failed protection read is unavailable, never a role refusal. isProtected
// reports the predicate's answer, so a projection can keep its confirmation
// hint for the Admin who may proceed.
func RestrictedDecision(verb string, operate Decision, protected func() (bool, error), admin func() error) (d Decision, isProtected bool) {
	if !operate.Allowed() || !ProtectedVerbRestricted(verb) {
		return operate, false
	}
	isProtected, err := protected()
	switch {
	case err != nil:
		return Decision{Outcome: DecisionUnavailable, Reason: ReasonAuthzUnavailable}, false
	case !isProtected:
		return operate, false
	}
	return protectedAdminDecision(admin()), true
}

// protectedAdminDecision classifies a protected member's fresh admin check. A
// plain role refusal (the bare ErrForbidden, however wrapped) is
// ReasonProtectedEnvironmentAdmin; a refusal coded with its own cause (a token
// missing the scope, a two-factor requirement) and an outage keep the reason
// ClassifyDecision gives them.
func protectedAdminDecision(err error) Decision {
	var coded *CodedError
	if errors.As(err, &coded) && error(coded) == ErrForbidden {
		return Decision{Outcome: DecisionDenied, Reason: ReasonProtectedEnvironmentAdmin}
	}
	return ClassifyDecision(err)
}

// RequireConfirmation passes when the caller sent confirm, and otherwise refuses
// with a 400 coded error whose params carry confirm (the phrase a retry must
// send) alongside params, so a client asks for the phrase without parsing the
// message.
func RequireConfirmation(ctx context.Context, confirm, code, message string, params map[string]any) error {
	if ConfirmFrom(ctx) == confirm {
		return nil
	}
	params["confirm"] = confirm
	return NewBadRequestError(code, message, params)
}
