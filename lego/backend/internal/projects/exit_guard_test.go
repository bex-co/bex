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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m232: a project membership write drops the environment of every member
// it moves — omitted members and members arriving from another project — so
// each environment it leaves goes through the environments feature's exit
// check, which refuses an ACL-bearing one without its admin. A refusal changes
// nothing.

// recordingExitGuards records every environment a move asked to leave; refuse
// names one whose exit is refused (an ACL-bearing source).
type recordingExitGuards struct {
	exits  []string
	refuse string
}

func (r *recordingExitGuards) load(context.Context, string) (core.EnvironmentExitGuard, error) {
	return func(environmentIDs ...string) error {
		r.exits = append(r.exits, environmentIDs...)
		if slices.Contains(environmentIDs, r.refuse) {
			return core.ErrForbidden
		}
		return nil
	}, nil
}

func TestProjectServiceMovesOutOfARefusedEnvironmentChangeNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		project string
		members []string
		set     []string
	}{
		"omitted member":  {"prj-1", []string{"web", "worker"}, []string{"worker"}},
		"incoming member": {"prj-2", []string{"worker"}, []string{"worker", "web"}},
	} {
		t.Run(name, func(t *testing.T) {
			st := newFakeProjectStore(
				store.Project{ID: "prj-1", TenantID: "tea-a"},
				store.Project{ID: "prj-2", TenantID: "tea-a"},
			).withEnv("web")
			st.services[tc.project] = slices.Clone(tc.members)
			envIdx := &recordingEnvironmentIndex{}
			guards := &recordingExitGuards{refuse: "env-web"}
			svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Environments: envIdx, ExitGuards: guards.load}

			if _, err := svc.SetServices(ctxAs("user-a"), tc.project, tc.set); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("SetServices = %v, want the exit refusal", err)
			}
			if !slices.Equal(st.services[tc.project], tc.members) || !st.envAttached["web"] || len(envIdx.clearedServices) != 0 {
				t.Fatalf("a refused move changed state: members=%v webEnv=%v cleared=%v", st.services[tc.project], st.envAttached["web"], envIdx.clearedServices)
			}

			// An environment the check admits moves as before.
			guards.refuse = ""
			if _, err := svc.SetServices(ctxAs("user-a"), tc.project, tc.set); err != nil {
				t.Fatalf("admitted SetServices: %v", err)
			}
			if !slices.Contains(guards.exits, "env-web") || st.envAttached["web"] {
				t.Fatalf("exit not checked or environment kept: exits=%v webEnv=%v", guards.exits, st.envAttached["web"])
			}
		})
	}
}

func TestProjectDatastoreMovesOutOfARefusedEnvironmentChangeNothing(t *testing.T) {
	for _, kind := range setKinds {
		for name, tc := range map[string]struct {
			member projectResource
			set    []string
		}{
			"omitted member":  {projectResource{"member", "prj-1", "env-prod"}, nil},
			"incoming member": {projectResource{"member", "prj-2", "env-prod"}, []string{"member"}},
		} {
			t.Run(kind.name+" "+name, func(t *testing.T) {
				st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a"})
				idx := newFakeResourceIndex("tea-a", tc.member)
				guards := &recordingExitGuards{refuse: "env-prod"}
				svc := projectServiceWith(st, allowChecker{})
				svc.ExitGuards = guards.load
				kind.wire(svc, idx)

				if _, err := kind.set(svc, ctxAs("user-a"), "prj-1", tc.set); !errors.Is(err, core.ErrForbidden) {
					t.Fatalf("set = %v, want the exit refusal", err)
				}
				if got := idx.byWorkspace["tea-a"][0]; got != tc.member || len(idx.relabels) != 0 {
					t.Fatalf("a refused move changed placement: %+v relabels=%v", got, idx.relabels)
				}

				guards.refuse = ""
				if _, err := kind.set(svc, ctxAs("user-a"), "prj-1", tc.set); err != nil {
					t.Fatalf("admitted set: %v", err)
				}
				if got := idx.byWorkspace["tea-a"][0]; got.environmentID != "" {
					t.Fatalf("the moved member kept its environment: %+v", got)
				}
			})
		}
	}
}

// TestAProjectDatastoreMovedSinceItWasListedIsRefusedNotPulledOut: the check
// passes on the bare environment the listing showed, but the datastore is in
// prod by the time of the write; the write's compare refuses it.
func TestAProjectDatastoreMovedSinceItWasListedIsRefusedNotPulledOut(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a"})
			idx := newFakeResourceIndex("tea-a", projectResource{"member", "prj-1", "env-qa"})
			idx.afterList = func() { idx.byWorkspace["tea-a"][0].environmentID = "env-prod" }
			guards := &recordingExitGuards{refuse: "env-prod"}
			svc := projectServiceWith(st, allowChecker{})
			svc.ExitGuards = guards.load
			kind.wire(svc, idx)

			if _, err := kind.set(svc, ctxAs("user-a"), "prj-1", nil); !errors.Is(err, core.ErrConflict) {
				t.Fatalf("move of a concurrently moved %s = %v, want a conflict", kind.name, err)
			}
			if got := idx.byWorkspace["tea-a"][0]; got.projectID != "prj-1" || got.environmentID != "env-prod" {
				t.Fatalf("the datastore left prod: %+v", got)
			}
		})
	}
}
