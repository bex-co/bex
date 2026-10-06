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
	"fmt"
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

// EnvironmentProtected is the predicate half of RequireEnvironmentConfirmation
// — shared with the capability projections (ADR087, w6/m136) so what a
// projection reports and what the guard enforces are structurally the same
// answer. Empty environment/store are unprotected, exactly like the guard.
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

// ProtectionPrecondition classifies a protection predicate's answer for a
// capability projection — the ONE mapping the App-row and Environment-label
// protection paths share: protected → the confirmation requirement; an
// unanswerable read → unavailable (blocked, never silently unguarded).
func ProtectionPrecondition(protected bool, err error) string {
	if err != nil {
		return PrecondUnavailable
	}
	if protected {
		return PrecondProtectedConfirmation
	}
	return ""
}

// EnvironmentProtectionPrecondition projects the protected-environment guard
// as a bounded precondition, by the same predicate the guard enforces.
func EnvironmentProtectionPrecondition(ctx context.Context, store EnvironmentProtectionStore, environmentID string) string {
	return ProtectionPrecondition(EnvironmentProtected(ctx, store, environmentID))
}

// RequireEnvironmentConfirmation enforces an exact server-issued phrase for a
// resource in a protected Environment. Empty environment/store are no-ops so
// DB-less and environment-less resources preserve their original behavior.
func RequireEnvironmentConfirmation(
	ctx context.Context,
	store EnvironmentProtectionStore,
	environmentID, name, verb, required string,
) error {
	protected, err := EnvironmentProtected(ctx, store, environmentID)
	if err != nil {
		return err
	}
	if !protected {
		return nil
	}
	return RequireProtectedConfirmation(ctx, name, verb, required)
}

// RequireProtectedConfirmation refuses verb on name, a member of a protected
// environment, unless the caller sent confirm. The refusal is 400
// PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED with params confirm (the phrase a
// retry must send), verb and name, so a client asks for the phrase without
// parsing the message (w5/m128).
func RequireProtectedConfirmation(ctx context.Context, name, verb, confirm string) error {
	if ConfirmFrom(ctx) == confirm {
		return nil
	}
	return NewBadRequestError("PROTECTED_ENVIRONMENT_CONFIRMATION_REQUIRED",
		fmt.Sprintf("%q is a member of a protected environment; retry with confirm=%q to %s it", name, confirm, verb),
		map[string]any{"confirm": confirm, "verb": verb, "name": name})
}
