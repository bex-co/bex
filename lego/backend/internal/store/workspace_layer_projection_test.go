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

package store

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rulesMemStore is memStore plus the workspace rules read, counted.
type rulesMemStore struct {
	*memStore
	rules  map[string][]core.IPAllowListEntry
	broken map[string]bool
	reads  map[string]int
}

func (s *rulesMemStore) WorkspaceIPAllowList(_ context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	s.reads[id]++
	if s.broken[id] {
		return nil, false, errors.New("db down")
	}
	entries, ok := s.rules[id]
	return entries, ok, nil
}

// w9/m388: the projector stamps a CR it creates with its workspace's layer —
// web services and static sites only, the saved CIDRs or nothing for a
// workspace that never saved rules — reading each workspace's rules once per
// pass and only when it creates. A failed read creates nothing that pass
// rather than a CR outside the rules; the next pass creates it.
func TestProjectorCreatesAppsInsideTheirWorkspaceLayer(t *testing.T) {
	ctx := context.Background()
	rec, mem, cl := newTestReconciler(t)
	st := &rulesMemStore{memStore: mem, reads: map[string]int{}, broken: map[string]bool{}}
	rec.Store = st
	ruled, _ := mem.CreateTenant(ctx, "ruled", "scale")
	open, _ := mem.CreateTenant(ctx, "open", "scale")
	flaky, _ := mem.CreateTenant(ctx, "flaky", "scale")
	st.rules = map[string][]core.IPAllowListEntry{ruled.ID: {{CIDRBlock: "203.0.113.0/24"}}}
	st.broken[flaky.ID] = true
	create := func(tenant, name, typ string) App {
		t.Helper()
		row, err := mem.CreateApp(ctx, App{TenantID: tenant, Name: name, Type: typ, Image: "traefik/whoami", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	web := create(ruled.ID, "web", appv1alpha1.TypeWebService)
	site := create(ruled.ID, "site", appv1alpha1.TypeStaticSite)
	private := create(ruled.ID, "private", appv1alpha1.TypePrivateService)
	unruled := create(open.ID, "web", appv1alpha1.TypeWebService)
	stranded := create(flaky.ID, "web", appv1alpha1.TypeWebService)

	if err := rec.ReconcileOnce(ctx); err == nil {
		t.Fatal("a failed rules read was not reported")
	}
	layers := func() map[string][]string {
		t.Helper()
		var apps appv1alpha1.AppList
		if err := cl.List(ctx, &apps); err != nil {
			t.Fatal(err)
		}
		out := map[string][]string{}
		for _, a := range apps.Items {
			out[a.Labels[core.LabelAppID]] = a.Spec.WorkspaceIPAllowList
		}
		return out
	}
	got := layers()
	want := []string{"203.0.113.0/24"}
	if !slices.Equal(got[web.ID], want) || !slices.Equal(got[site.ID], want) {
		t.Errorf("web %v / static %v, want %v", got[web.ID], got[site.ID], want)
	}
	if layer, ok := got[private.ID]; !ok || layer != nil {
		t.Errorf("private service: created=%v layer=%v; want created with no layer", ok, layer)
	}
	if layer, ok := got[unruled.ID]; !ok || layer != nil {
		t.Errorf("unruled workspace: created=%v layer=%v; want created with no layer", ok, layer)
	}
	if _, ok := got[stranded.ID]; ok {
		t.Error("a CR was created although its workspace's rules could not be read")
	}
	if st.reads[ruled.ID] != 1 || st.reads[open.ID] != 1 {
		t.Errorf("rule reads = %v, want one per workspace", st.reads)
	}
	// Later passes read nothing for workspaces whose CRs exist, and create
	// the stranded CR once its rules can be read.
	st.broken[flaky.ID] = false
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if st.reads[ruled.ID] != 1 || st.reads[open.ID] != 1 {
		t.Errorf("a resync read the rules again: %v", st.reads)
	}
	if layer, ok := layers()[stranded.ID]; !ok || layer != nil {
		t.Errorf("stranded CR: created=%v layer=%v", ok, layer)
	}
}
