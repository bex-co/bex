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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// guardedEnvironment is the create resolver for one protected environment
// whose placement gate refuses the caller unless admin, as
// environments.NewCreateResolver does for a non-Admin (w3/m281).
type guardedEnvironment struct {
	admin  bool
	placed int
}

func (g *guardedEnvironment) ResolveForCreate(context.Context, string, string) (core.EnvironmentAssignment, error) {
	return core.EnvironmentAssignment{ID: "env-prod", ProjectID: "prj-shop", WorkspaceID: "tea-a", Protected: true}, nil
}

func (g *guardedEnvironment) RequirePlacement(_ context.Context, assignment core.EnvironmentAssignment) error {
	g.placed++
	if !assignment.Protected || g.admin {
		return nil
	}
	return core.PlacementRefusal(assignment.ID, core.ErrForbidden)
}

// TestRecoveringIntoAProtectedEnvironmentNeedsAnAdmin: a recovery joins its
// source's environment, so it is a create there and passes the same placement
// gate a CreatePostgres does; a refusal writes no Database.
func TestRecoveringIntoAProtectedEnvironmentNeedsAnAdmin(t *testing.T) {
	for _, admin := range []bool{false, true} {
		svc, src := backedUpSource(t, "free")
		src.Labels[core.LabelEnvironment] = "env-prod"
		src.Labels[core.LabelProject] = "prj-shop"
		if err := svc.Client.Update(context.Background(), src); err != nil {
			t.Fatal(err)
		}
		guard := &guardedEnvironment{admin: admin}
		svc.Environments = guard

		view, err := svc.Recover(ctxAs("user-a"), "dpg-src", RecoverRequest{Name: "restored"})
		if guard.placed != 1 {
			t.Fatalf("admin=%v: placement gate ran %d times, want once", admin, guard.placed)
		}
		var databases appv1alpha1.DatabaseList
		if err := svc.Client.List(context.Background(), &databases); err != nil {
			t.Fatal(err)
		}
		if !admin {
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.ProtectedPlacementCode || !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("non-admin recovery = %v, want %s", err, core.ProtectedPlacementCode)
			}
			if len(databases.Items) != 1 {
				t.Fatalf("a refused recovery wrote a Database: %d items", len(databases.Items))
			}
			continue
		}
		if err != nil || view.EnvironmentID == "" {
			t.Fatalf("admin recovery = %+v, %v; want it in the source's environment", view, err)
		}
	}
}
