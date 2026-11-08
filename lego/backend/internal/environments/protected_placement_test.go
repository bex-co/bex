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
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

// w3/m281: Render lets only workspace Admins create resources in a protected
// environment or move them into or out of it (render.com/docs/projects).

// placementRefused reports whether err is the coded placement refusal (still
// a 403: it wraps ErrForbidden).
func placementRefused(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == core.ProtectedPlacementCode && errors.Is(err, core.ErrForbidden)
}

func TestCreatingInAProtectedEnvironmentNeedsAnAdmin(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	developer := NewCreateResolver(f.developer())
	admin := NewCreateResolver(f.admin)

	_, err := core.ResolveEnvironmentForCreate(ctx, developer, id.EnvironmentPublicID(f.prod.ID), "tea-a")
	if !placementRefused(err) {
		t.Fatalf("developer create in protected prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	var coded *core.CodedError
	if errors.As(err, &coded); coded.Params["environmentId"] != id.EnvironmentPublicID(f.prod.ID) {
		t.Fatalf("refusal params = %v, want the public environment id", coded.Params)
	}
	got, err := core.ResolveEnvironmentForCreate(ctx, admin, f.prod.ID, "tea-a")
	if err != nil || got.ID != f.prod.ID || !got.Protected {
		t.Fatalf("admin create in prod = %+v, %v; want prod, protected", got, err)
	}
	// Unprotected and no-environment creates are unchanged for a developer.
	if got, err := core.ResolveEnvironmentForCreate(ctx, developer, f.staging.ID, "tea-a"); err != nil || got.ID != f.staging.ID || got.Protected {
		t.Fatalf("developer create in staging = %+v, %v", got, err)
	}
	if got, err := core.ResolveEnvironmentForCreate(ctx, developer, "", "tea-a"); err != nil || got.ID != "" {
		t.Fatalf("developer create with no environment = %+v, %v", got, err)
	}
	// Re-reading an existing member's environment places nothing.
	if got, err := core.ResolveEnvironmentMembership(ctx, developer, f.prod.ID, "tea-a"); err != nil || got.ID != f.prod.ID {
		t.Fatalf("developer membership read of prod = %+v, %v", got, err)
	}
}

func TestAnAuthzOutageOnAProtectedCreateIsNotARoleRefusal(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	down := &Service{Base: &core.Base{Authz: unavailableChecker{}}, Store: f.st}
	_, err := core.ResolveEnvironmentForCreate(ctx, NewCreateResolver(down), f.prod.ID, "tea-a")
	if !errors.Is(err, core.ErrAuthzUnavailable) || placementRefused(err) {
		t.Fatalf("create with authz down = %v, want ErrAuthzUnavailable, not a role refusal", err)
	}
}

func TestMovingAServiceIntoOrOutOfAProtectedEnvironmentNeedsAnAdmin(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	dev := f.developer()

	// Out of protected prod into bare staging.
	if _, err := dev.SetServices(ctx, f.staging.ID, []string{"web"}); !placementRefused(err) {
		t.Fatalf("developer move out of prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	// Out of prod into Unassigned (prod's membership replaced without web).
	if _, err := dev.SetServices(ctx, f.prod.ID, nil); !placementRefused(err) {
		t.Fatalf("developer unassign from prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	if got := f.servicesIn(t, f.prod); !slices.Equal(got, []string{"web"}) {
		t.Fatalf("refused moves changed prod: %v", got)
	}
	// The admin moves it to staging; the developer cannot move it back in.
	if _, err := f.admin.SetServices(ctx, f.staging.ID, []string{"web"}); err != nil {
		t.Fatalf("admin move to staging: %v", err)
	}
	if _, err := dev.SetServices(ctx, f.prod.ID, []string{"web"}); !placementRefused(err) {
		t.Fatalf("developer move into prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	if got := f.servicesIn(t, f.staging); !slices.Equal(got, []string{"web"}) {
		t.Fatalf("refused move-in changed staging: %v", got)
	}
	if _, err := f.admin.SetServices(ctx, f.prod.ID, []string{"web"}); err != nil {
		t.Fatalf("admin move into prod: %v", err)
	}
}

func TestMovingADatastoreIntoAProtectedEnvironmentNeedsAnAdmin(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-one", OwnerID: "tea-a", ProjectID: "prj-1", EnvironmentID: f.staging.ID})
	dev := f.developer()
	dev.Databases, f.admin.Databases = dbs, dbs

	if _, err := dev.SetDatabases(ctx, f.prod.ID, []string{"dpg-one"}); !placementRefused(err) {
		t.Fatalf("developer move into prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	if got := id.EnvironmentStorageID(dbs.dbs["dpg-one"].EnvironmentID); got != f.staging.ID {
		t.Fatalf("a refused move-in changed placement to %q", got)
	}
	if _, err := f.admin.SetDatabases(ctx, f.prod.ID, []string{"dpg-one"}); err != nil {
		t.Fatalf("admin move into prod: %v", err)
	}
}

// TestLeavingAnIsolatedButUnprotectedEnvironmentKeepsItsPlainRefusal: the
// exit guard still covers every ACL-bearing source, and only a protected one
// answers with the placement code.
func TestLeavingAnIsolatedButUnprotectedEnvironmentKeepsItsPlainRefusal(t *testing.T) {
	ctx := ctxAs("user-a")
	f := newExitFixture(t)
	isolatedView, err := f.admin.CreateWithACL(ctx, CreateEnvironmentRequest{ProjectID: "prj-1", Name: "isolated", NetworkIsolationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	isolated, _ := f.st.GetEnvironment(ctx, isolatedView.ID)
	exit, err := f.developer().loadExitGuard(ctx, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := exit(isolated.ID); !errors.Is(err, core.ErrForbidden) || placementRefused(err) {
		t.Fatalf("developer leaving isolated = %v, want plain ErrForbidden", err)
	}
	if err := exit(f.prod.ID); !placementRefused(err) {
		t.Fatalf("developer leaving protected prod = %v, want %s", err, core.ProtectedPlacementCode)
	}
	// An isolated, unprotected target needs the admin as before, uncoded.
	if _, err := f.developer().SetServices(ctx, isolated.ID, nil); !errors.Is(err, core.ErrForbidden) || placementRefused(err) {
		t.Fatalf("developer membership change on isolated = %v, want plain ErrForbidden", err)
	}
}

// unavailableChecker is an OpenFGA that cannot answer.
type unavailableChecker struct{}

func (unavailableChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, errors.New("fga down")
}
