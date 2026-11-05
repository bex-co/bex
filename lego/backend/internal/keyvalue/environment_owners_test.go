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
	"net/http"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/500: the environmentId filter asks the environment for its workspace
// (public spelling) when ownerId is omitted, and an environment that is not
// viewable is the same empty list as one with no Key Value instances.
func TestRESTListKeyValueEnvironmentFilterResolvesTheEnvironmentsWorkspace(t *testing.T) {
	svc, _ := newService()
	var asked []string
	svc.EnvironmentWorkspace = func(_ context.Context, id string) (string, error) {
		asked = append(asked, id)
		return "", core.ErrForbidden
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	if got := listKVNames(t, mux, "?environmentId=env-d98210cbbpdc73dcrkvg"); len(got) != 0 {
		t.Fatalf("instances = %v, want none", got)
	}
	if !slices.Equal(asked, []string{"evm-d98210cbbpdc73dcrkvg"}) {
		t.Fatalf("resolver asked for %v, want the public environment id", asked)
	}
}
