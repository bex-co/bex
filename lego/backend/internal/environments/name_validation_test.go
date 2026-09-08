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

package environments

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestNameValidation(t *testing.T) {
	for _, verb := range []string{"create", "rename", "graphqlCreate", "graphqlRename"} {
		for _, name := range []string{"", " \t\n\u2003", "  production  "} {
			t.Run(fmt.Sprintf("%s/%q", verb, name), func(t *testing.T) {
				st := newFakeStore()
				st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "project"})
				st.envs["env-1"] = store.Environment{ID: "env-1", ProjectID: "prj-1", TenantID: "tea-a", Name: "original"}
				svc := newService(st)
				ctx := ctxAs("user-a")
				var view EnvironmentView
				var err error
				switch verb {
				case "create":
					view, err = svc.Create(ctx, "prj-1", name)
				case "rename":
					view, err = svc.Rename(ctx, "env-1", name)
				default:
					field := "createEnvironment"
					if verb == "graphqlRename" {
						field = "renameEnvironment"
					}
					var out any
					out, err = svc.GraphQLMutation()[field].Resolve(graphql.ResolveParams{Context: ctx, Args: map[string]any{"id": "env-1", "ownerId": "tea-a", "projectId": "prj-1", "name": name}})
					if err == nil {
						view = out.(EnvironmentView)
					}
				}
				if strings.TrimSpace(name) == "" {
					if !errors.Is(err, core.ErrBadRequest) {
						t.Fatalf("got %v, want bad request", err)
					}
					if len(st.envs) != 1 || st.envs["env-1"].Name != "original" {
						t.Fatalf("rejected request changed state: %+v", st.envs)
					}
				} else {
					if err != nil || view.Name != "production" {
						t.Fatalf("result: %+v, %v", view, err)
					}
					if st.envs[view.ID].Name != "production" {
						t.Fatal("trimmed name was not persisted")
					}
				}
			})
		}
	}
}
