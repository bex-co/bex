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

package envgroups

import (
	"context"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/500: the environmentId filter asks the environment for its workspace when
// ownerId is omitted, and an explicit ownerId is never overridden.
func TestREST_EnvGroupEnvironmentFilterResolvesTheEnvironmentsWorkspace(t *testing.T) {
	svc := filterTestService(t)
	var asked []string
	svc.EnvironmentWorkspace = func(_ context.Context, id string) (string, error) {
		asked = append(asked, id)
		return "", core.ErrNotFound
	}
	envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?environmentId=env-one"))
	if len(asked) != 1 {
		t.Fatalf("resolver asked for %v, want one lookup", asked)
	}
	asked = nil
	envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?ownerId=tea-a&environmentId=env-one"))
	if !slices.Equal(asked, nil) {
		t.Fatalf("with ownerId the resolver asked for %v, want none", asked)
	}
}
