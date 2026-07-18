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
	if name == "" {
		name = database.Name
	}
	return core.RequireEnvironmentConfirmation(ctx, s.Protection, environmentID, name, verb, ProtectedConfirmation(verb, name))
}
