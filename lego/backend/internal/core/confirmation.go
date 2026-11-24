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
// services, toggling maintenance mode (VerbToggleMaintenanceMode, both ways
// since w4/725) and shell access ("shell"; environment variables and secret
// files are the secrets package's, w8/193). The rest of a service's life —
// deploys, rollbacks, settings, queries — is open to anyone who may do it elsewhere, and no verb asks for a
// typed phrase (w8/187; the phrase gate of w4/m126 and w4/m127 is retired).
// The dashboard may still confirm in its own UI. Viewing a datastore's
// credentials, changing its inbound IP rules and viewing or changing an
// environment group's variables joined in w4/m594 (the Verb constants below).
func ProtectedVerbRestricted(verb string) bool {
	switch verb {
	case "create", "move", "delete", "suspend", "resume", "shell",
		VerbToggleMaintenanceMode, VerbViewCredentials, VerbChangeAccessControl, VerbEnvironmentVariables:
		return true
	}
	return false
}

// The restricted verbs named once because a verb and its capability
// projection both ask for each (render.com/docs/projects: only Admins in a
// protected environment are "toggling maintenance mode for any service",
// w4/725; and, w4/m594, "view passwords or connection URLs for any Postgres or
// Key Value instance", "modify access control IPs for any Postgres or Key
// Value instance" and "view or modify environment variables or secret files
// for any service or environment group").
const (
	// VerbToggleMaintenanceMode turns a service's maintenance mode on or off.
	// Before w4/725 only turning it on ("take offline") was restricted.
	VerbToggleMaintenanceMode = "toggle maintenance mode"
	// VerbViewCredentials reveals a Postgres's or Key Value's password or
	// connection URLs: its connection info, or a new Postgres user's one-time
	// password.
	VerbViewCredentials = "view credentials"
	// VerbChangeAccessControl changes the inbound IP rules in effect on a
	// Postgres's or Key Value's external endpoint.
	VerbChangeAccessControl = "change access control"
	// VerbEnvironmentVariables reveals or changes an environment group's
	// variables or secret files, or which services receive them.
	VerbEnvironmentVariables = "view or change environment variables"
)

// ProtectedPlacementCode is the 403 a non-Admin receives for every restricted
// verb on a member of a protected environment: creating a resource in it or
// moving one into or out of it (w3/m281), and every verb RequireEnvironmentAdmin
// guards (w4/m594, w4/728). A coded refusal, so every surface — and the
// dashboard's raced-403 path — can name the reason without parsing the message.
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
// environment unless admin (a fresh workspace-manage check) passes, answering
// admin's plain role refusal as ProtectedAdminRefusal (w4/m594, w4/728). A
// refusal coded with its own cause (a token missing the scope, a two-factor
// requirement) and an outage pass through, as in protectedAdminDecision. Empty
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
	return ProtectedAdminError(environmentID, verb, admin())
}

// ProtectedAdminRefusal is the ProtectedPlacementCode refusal of verb, a
// restricted verb other than a placement, on a member of the protected
// environment environmentID (w4/m594): the same code and params, so every
// surface, and the dashboard's raced-403 path, names the reason without
// parsing the message.
func ProtectedAdminRefusal(environmentID, verb string) *CodedError {
	return NewForbiddenError(ProtectedPlacementCode,
		"only workspace Admins can "+protectedVerbAction(verb)+" in a protected environment",
		map[string]any{"environmentId": ids.EnvironmentPublicID(environmentID)})
}

// protectedVerbAction words verb as the action a refusal names, so Render's
// operations, whose 403 carries no bex code (w8/411), still say what was
// refused: the guard's own name for shell access is not what a caller did.
func protectedVerbAction(verb string) string {
	switch verb {
	case "delete", "suspend", "resume":
		return verb + " resources"
	case "shell":
		return "open a shell"
	}
	return verb
}

// ProtectedAdminError is err, a fresh workspace-manage check's answer for verb
// on a member of the protected environment environmentID, with a plain role
// refusal coded as ProtectedAdminRefusal. nil, a refusal coded with its own
// cause and an outage pass through.
func ProtectedAdminError(environmentID, verb string, err error) error {
	if IsRoleRefusal(err) {
		return ProtectedAdminRefusal(environmentID, verb)
	}
	return err
}

// IsRoleRefusal reports the bare ErrForbidden, however wrapped: a role
// refusal, not a refusal coded with a cause of its own (an OAuth scope, a
// login requirement, a key's grant).
func IsRoleRefusal(err error) bool {
	var coded *CodedError
	return errors.As(err, &coded) && error(coded) == ErrForbidden
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
	if IsRoleRefusal(err) {
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
