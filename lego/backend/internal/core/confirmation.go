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

// RequireEnvironmentConfirmation enforces an exact server-issued phrase for a
// resource in a protected Environment. Empty environment/store are no-ops so
// DB-less and environment-less resources preserve their original behavior.
func RequireEnvironmentConfirmation(
	ctx context.Context,
	store EnvironmentProtectionStore,
	environmentID, name, verb, required string,
) error {
	if store == nil || environmentID == "" {
		return nil
	}
	protectedStatus, err := store.GetEnvironmentProtectedStatus(ctx, environmentID)
	if err != nil {
		return err
	}
	if protectedStatus != ProtectedStatusProtected {
		return nil
	}
	if ConfirmFrom(ctx) != required {
		return fmt.Errorf("%w: %q is a member of a protected environment; retry with confirm=%q to %s it", ErrBadRequest, name, required, verb)
	}
	return nil
}
