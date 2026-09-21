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

package postgres

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ProtectedConfirmation is the exact phrase required to perform verb on a
// database in a protected Environment.
func ProtectedConfirmation(verb, name string) string {
	return "sudo " + verb + " database " + name
}

func (s *Service) requireUnprotected(ctx context.Context, database *appv1alpha1.Database, verb string) error {
	environmentID := database.Labels[core.LabelEnvironment]
	name := database.Spec.Name
	return core.RequireEnvironmentConfirmation(ctx, s.Protection, environmentID, name, verb, ProtectedConfirmation(verb, name))
}

// protectedDatabasePatchVerb names the confirmation a PATCH needs on a member of
// a protected environment, or "" when the patch changes nothing protection
// covers.
//
// w6/m19 built the guard for services and w6/m37 extended it to datastores, but
// both asked only WHICH RESOURCES it covers. w4/m127 asked which OPERATIONS, and
// found a protected environment that refused to pause a database while allowing
// the operations that change its identity and take it offline. The rule ADR032
// now records: protection covers what can destroy data, break identity, or
// interrupt availability — not merely the two verbs that stop the object.
//
// One verb per call: a patch that both renames and upgrades asks for the more
// destructive phrase, so a caller cannot satisfy the cheaper one and get both.
func protectedDatabasePatchVerb(patch PostgresPatch) string {
	switch {
	case patch.Version != nil:
		// A major upgrade takes the database offline for pg_upgrade and cannot
		// be undone — the most destructive thing reachable through PATCH.
		return "upgrade"
	case patch.Name != nil:
		return "rename"
	default:
		return ""
	}
}
