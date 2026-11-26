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

package projects

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// scopeGetterStore is a viewStore whose environments resolve by id, as
// *store.PGStore's GetEnvironment does (the public alias accepted).
type scopeGetterStore struct {
	*viewStore
	envs map[string]store.Environment // keyed by storage (env-) id
}

func (s *scopeGetterStore) GetEnvironment(_ context.Context, id string) (store.Environment, error) {
	if e, ok := s.envs[ids.EnvironmentStorageID(id)]; ok {
		return e, nil
	}
	return store.Environment{}, fmt.Errorf("environment: %w", store.ErrNotFound)
}

// scopeListerStore is a viewStore whose environments list by project only, for
// a store with the lister capability but no by-id getter.
type scopeListerStore struct {
	*viewStore
	envs []store.Environment
}

func (s *scopeListerStore) ListEnvironmentsForProjects(_ context.Context, projectIDs []string) ([]store.Environment, error) {
	var out []store.Environment
	for _, e := range s.envs {
		for _, pid := range projectIDs {
			if e.ProjectID == pid {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// scopeWorld is one caller-workspace project with an environment (and
// resources in and out of it), a second same-workspace project with its own
// environment, and a project in a workspace the caller cannot view — served
// through each store capability flavor.
type scopeWorld struct {
	getter, lister, neither *Service
	pubA, pubB, pubC        string // public (evm-) ids of envA, envB, envC
	storA                   string // storage (env-) id of envA
	missing                 string // a well-formed public id no environment carries
}

func newScopeWorld() *scopeWorld {
	second := store.Project{ID: "prj-second", TenantID: "tea-own", Name: "second"}
	envA := store.Environment{ID: ids.EnvironmentStorageID(ids.New(ids.Environment)), ProjectID: ownProject.ID, TenantID: ownProject.TenantID}
	envB := store.Environment{ID: ids.EnvironmentStorageID(ids.New(ids.Environment)), ProjectID: second.ID, TenantID: second.TenantID}
	envC := store.Environment{ID: ids.EnvironmentStorageID(ids.New(ids.Environment)), ProjectID: otherProject.ID, TenantID: otherProject.TenantID}

	mk := func(vs *viewStore, wrapped ProjectStore) *Service {
		vs.services[ownProject.ID] = []store.ProjectService{
			{ID: "srv-web", Name: "storefront", Type: "web_service", EnvironmentID: ids.EnvironmentPublicID(envA.ID)},
			{ID: "srv-cron", Name: "nightly", Type: "cron_job"},
		}
		svc := &Service{Base: &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))}, Store: vs}
		if wrapped != nil {
			svc.Store = wrapped
		}
		svc.Databases = newFakeResourceIndex("tea-own",
			projectResource{id: "dpg-db", projectID: ownProject.ID, environmentID: envA.ID})
		svc.KeyValues = newFakeResourceIndex("tea-own",
			projectResource{id: "red-kv", projectID: ownProject.ID})
		return svc
	}

	gv, lv, nv := newViewStore(ownProject, second, otherProject), newViewStore(ownProject, second, otherProject), newViewStore(ownProject, second, otherProject)
	envs := map[string]store.Environment{envA.ID: envA, envB.ID: envB, envC.ID: envC}
	return &scopeWorld{
		getter:  mk(gv, &scopeGetterStore{viewStore: gv, envs: envs}),
		lister:  mk(lv, &scopeListerStore{viewStore: lv, envs: []store.Environment{envA, envB, envC}}),
		neither: mk(nv, nil),
		pubA:    ids.EnvironmentPublicID(envA.ID),
		pubB:    ids.EnvironmentPublicID(envB.ID),
		pubC:    ids.EnvironmentPublicID(envC.ID),
		storA:   envA.ID,
		missing: ids.EnvironmentPublicID(ids.EnvironmentStorageID(ids.New(ids.Environment))),
	}
}

// TestProjectScopeResourcesDecisionMatrix pins the scope-resolution contract:
// what a project, environment, or paired project+environment read answers —
// the resources of exactly that scope, a refusal for what the caller may not
// view, not found for what does not exist, and bad request for a viewable
// environment outside the project — across every store capability flavor.
func TestProjectScopeResourcesDecisionMatrix(t *testing.T) {
	w := newScopeWorld()
	member := ctxAs("member")
	viewer := &Service{Base: &core.Base{Authz: viewerChecker{}}, Store: w.getter.Store}
	viewer.Databases, viewer.KeyValues = w.getter.Databases, w.getter.KeyValues

	all := []string{"srv-web", "srv-cron", "dpg-db", "red-kv"}
	inEnv := []string{"srv-web", "dpg-db"}

	read := func(t *testing.T, svc *Service, project, env string) []string {
		t.Helper()
		tenant, resources, err := svc.ProjectScopeResources(member, project, env)
		if err != nil {
			t.Fatalf("ProjectScopeResources(%q, %q) = %q, %v", project, env, tenant, err)
		}
		if tenant != "tea-own" {
			t.Fatalf("ProjectScopeResources(%q, %q) tenant = %q, want tea-own", project, env, tenant)
		}
		var got []string
		for _, r := range resources {
			got = append(got, r.ID)
		}
		return got
	}
	if got := read(t, w.getter, ownProject.ID, ""); fmt.Sprint(got) != fmt.Sprint(all) {
		t.Fatalf("project-only = %v, want %v", got, all)
	}
	if got := read(t, w.getter, "", w.pubA); fmt.Sprint(got) != fmt.Sprint(inEnv) {
		t.Fatalf("environment-only = %v, want %v", got, inEnv)
	}
	if got := read(t, w.getter, ownProject.ID, w.pubA); fmt.Sprint(got) != fmt.Sprint(inEnv) {
		t.Fatalf("project+environment = %v, want %v", got, inEnv)
	}
	// The storage spelling of the same environment resolves identically.
	if got := read(t, w.getter, ownProject.ID, w.storA); fmt.Sprint(got) != fmt.Sprint(inEnv) {
		t.Fatalf("project+environment storage id = %v, want %v", got, inEnv)
	}
	if got := read(t, viewer, ownProject.ID, ""); fmt.Sprint(got) != fmt.Sprint(all) {
		t.Fatalf("viewer project-only = %v, want %v", got, all)
	}

	refusals := []struct {
		name         string
		svc          *Service
		project, env string
		want         error
	}{
		{"neither scope", w.getter, "", "", core.ErrBadRequest},
		{"unknown project", w.getter, "prj-missing", "", core.ErrNotFound},
		{"cross-workspace project", w.getter, otherProject.ID, "", core.ErrForbidden},
		{"unknown environment", w.getter, "", w.missing, core.ErrNotFound},
		{"cross-workspace environment", w.getter, "", w.pubC, core.ErrForbidden},
		{"project with cross-workspace environment", w.getter, ownProject.ID, w.pubC, core.ErrForbidden},
		{"unknown environment with project", w.getter, ownProject.ID, w.missing, core.ErrNotFound},
		{"viewable environment of another project", w.getter, ownProject.ID, w.pubB, core.ErrBadRequest},
		{"cross-workspace project with own environment", w.getter, otherProject.ID, w.pubA, core.ErrForbidden},
		{"lister unknown environment with project", w.lister, ownProject.ID, w.missing, core.ErrBadRequest},
	}
	named := core.WithWorkspace(member, "tea-own")
	for _, tc := range []struct {
		name         string
		project, env string
		want         error
	}{
		// A scope outside the named workspace is not found in it, as every
		// by-id read in a named workspace is (ADR072's by-id matrix).
		{"project outside the named workspace", otherProject.ID, "", core.ErrNotFound},
		{"environment outside the named workspace", ownProject.ID, w.pubC, core.ErrNotFound},
	} {
		if _, _, err := w.getter.ProjectScopeResources(named, tc.project, tc.env); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, tc := range refusals {
		if _, _, err := tc.svc.ProjectScopeResources(member, tc.project, tc.env); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	// The lister flavor agrees with the getter on same-project scopes.
	if got := read(t, w.lister, ownProject.ID, w.pubA); fmt.Sprint(got) != fmt.Sprint(inEnv) {
		t.Fatalf("lister project+environment = %v, want %v", got, inEnv)
	}

	// Without either capability the project scope still reads, and an unknown
	// environment degrades to no resources rather than an error or a leak.
	if got := read(t, w.neither, ownProject.ID, ""); fmt.Sprint(got) != fmt.Sprint(all) {
		t.Fatalf("neither project-only = %v, want %v", got, all)
	}
	if tenant, got, err := w.neither.ProjectScopeResources(member, ownProject.ID, w.missing); err != nil || tenant != "tea-own" || len(got) != 0 {
		t.Fatalf("neither unknown environment = %q, %+v, %v; want tea-own, none, nil", tenant, got, err)
	}
	if _, _, err := w.neither.ProjectScopeResources(member, "", w.pubA); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("neither environment-only = %v, want unavailable", err)
	}
}
