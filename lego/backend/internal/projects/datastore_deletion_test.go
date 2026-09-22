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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestDeleteUnassignsDatastoresInProjectWorkspace(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a"})
			idx := newFakeResourceIndex("tea-a", projectResource{"member", "prj-1"}, projectResource{"other", "prj-2"}).
				in("tea-b", projectResource{"foreign", "prj-1"})
			svc := projectServiceWith(st, allowChecker{})
			kind.wire(svc, idx)
			if err := svc.Delete(ctxAs("user-a"), "prj-1"); err != nil {
				t.Fatal(err)
			}
			if got := join(idx.relabelPairs()); got != "member=>" {
				t.Errorf("cleared members = %q, want only member=>", got)
			}
			if _, err := st.GetProject(t.Context(), "prj-1"); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("project still exists: %v", err)
			}
		})
	}
}

func TestDeleteDatastoreCleanupFailureKeepsProjectForRetry(t *testing.T) {
	boom := errors.New("injected key-value cleanup failure")
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a"})
	dbs := newFakeResourceIndex("tea-a", projectResource{"database", "prj-1"})
	kvs := newFakeResourceIndex("tea-a", projectResource{"key-value", "prj-1"})
	kvs.clearErr = boom
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Databases: dbs, KeyValues: kvs}
	if err := svc.Delete(ctxAs("user-a"), "prj-1"); !errors.Is(err, boom) {
		t.Fatalf("Delete = %v, want cleanup failure", err)
	}
	if _, err := st.GetProject(t.Context(), "prj-1"); err != nil {
		t.Fatalf("failed cleanup deleted project: %v", err)
	}
	if got := join(dbs.relabelPairs()); got != "database=>" {
		t.Fatalf("database cleanup before failure = %q", got)
	}
	// A completed member can join another project before the interrupted
	// deletion resumes. The retry must leave that new membership alone.
	if err := dbs.SetProjectID(t.Context(), "database", "prj-new"); err != nil {
		t.Fatal(err)
	}
	kvs.clearErr = nil
	if err := svc.Delete(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatalf("retry Delete: %v", err)
	}
	if got := dbs.byWorkspace["tea-a"][0].projectID; got != "prj-new" {
		t.Errorf("retry cleared reassigned database: %q", got)
	}
	if got := kvs.byWorkspace["tea-a"][0].projectID; got != "" {
		t.Errorf("retry retained deleted project on key-value: %q", got)
	}
}

func TestDeleteDatastoreCleanupPreservesReassignmentAfterList(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a"})
			idx := newFakeResourceIndex("tea-a", projectResource{"member", "prj-1"})
			idx.afterList = func() { idx.byWorkspace["tea-a"][0].projectID = "prj-new" }
			svc := projectServiceWith(st, allowChecker{})
			kind.wire(svc, idx)
			if err := svc.Delete(ctxAs("user-a"), "prj-1"); err != nil {
				t.Fatal(err)
			}
			if got := idx.byWorkspace["tea-a"][0].projectID; got != "prj-new" {
				t.Errorf("cleanup erased concurrent membership: %q", got)
			}
		})
	}
}
