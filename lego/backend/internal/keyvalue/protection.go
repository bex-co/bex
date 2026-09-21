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

package keyvalue

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ProtectedConfirmation is the exact phrase required to perform verb on a Key
// Value instance in a protected Environment.
func ProtectedConfirmation(verb, name string) string {
	return "sudo " + verb + " key value " + name
}

// protectedKeyValuePatchVerb names the confirmation a PATCH needs on a member of
// a protected environment, or "" when the patch changes nothing protection
// covers. See postgres.protectedDatabasePatchVerb for the rule (w4/m127).
func protectedKeyValuePatchVerb(patch KeyValuePatch) string {
	switch {
	case patch.PersistenceMode != nil:
		// Turning persistence off makes every key in the store ephemeral — the
		// one PATCH that can lose the data outright, and it was accepted on the
		// same instance whose suspend was refused.
		return "change durability of"
	case patch.MaxmemoryPolicy != nil:
		// An eviction policy decides which keys the server is allowed to throw
		// away under memory pressure.
		return "change eviction on"
	case patch.Name != nil:
		return "rename"
	default:
		return ""
	}
}

func (s *Service) requireUnprotected(ctx context.Context, keyValue *appv1alpha1.KeyValue, verb string) error {
	environmentID := keyValue.Labels[core.LabelEnvironment]
	name := keyValue.Spec.Name
	return core.RequireEnvironmentConfirmation(ctx, s.Protection, environmentID, name, verb, ProtectedConfirmation(verb, name))
}
