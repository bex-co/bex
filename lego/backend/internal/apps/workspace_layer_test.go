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

package apps

import (
	"context"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type workspaceRules map[string][]core.IPAllowListEntry

func (w workspaceRules) WorkspaceIPAllowList(_ context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	entries, ok := w[id]
	return entries, ok, nil
}

// countingRules counts how often the rules are read.
type countingRules struct {
	workspaceRules
	reads int
}

func (c *countingRules) WorkspaceIPAllowList(ctx context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	c.reads++
	return c.workspaceRules.WorkspaceIPAllowList(ctx, id)
}

var officeRules = workspaceRules{"tea-a": {{CIDRBlock: "203.0.113.0/24", Description: "office"}}}

// w9/m388: a new web service or static site is born inside its workspace's
// inbound IP rules; a private service, worker or cron job (no public entry
// point) carries none; a workspace that never saved rules adds no layer.
func TestCreateStampsTheWorkspaceLayerOnEligibleTypes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		req   CreateRequest
		rules workspaceRules
		want  []string
	}{
		{"web service", CreateRequest{Name: "web", Image: "nginx:1"}, officeRules, []string{"203.0.113.0/24"}},
		{"saved deny-all", CreateRequest{Name: "web", Image: "nginx:1"}, workspaceRules{"tea-a": {}}, []string{core.DenyAllCIDR}},
		{"no rules", CreateRequest{Name: "web", Image: "nginx:1"}, workspaceRules{}, nil},
		{"private service", CreateRequest{Name: "pserv", Type: appv1alpha1.TypePrivateService, Image: "nginx:1"}, officeRules, nil},
		{"background worker", CreateRequest{Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1"}, officeRules, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{})
			svc.WorkspaceIPRules = tc.rules
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
			if _, err := svc.create(ctx, tc.req); err != nil {
				t.Fatalf("create: %v", err)
			}
			got := getTenantApp(t, cl, "tea-a", tc.req.Name).Spec.WorkspaceIPAllowList
			if !slices.Equal(got, tc.want) || (tc.want == nil) != (got == nil) {
				t.Fatalf("layer = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// A Blueprint apply creates every resource inside the workspace's rules: its
// web service, Postgres and Key Value carry the layer, its private service
// does not.
func TestBlueprintCreatesStampTheWorkspaceLayer(t *testing.T) {
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{})
	rules := &countingRules{workspaceRules: officeRules}
	svc.WorkspaceIPRules = rules
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	manifest := `databases:
  - name: layer-db
    plan: free
services:
  - type: web
    name: layer-web
    runtime: image
    image:
      url: nginx:alpine
  - type: pserv
    name: layer-pserv
    runtime: image
    image:
      url: nginx:alpine
  - type: keyvalue
    name: layer-cache
    plan: free
    ipAllowList: []
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	// Four creates, one read of the workspace's rules.
	if rules.reads != 1 {
		t.Errorf("the stack read the workspace rules %d times, want once", rules.reads)
	}
	want := []string{"203.0.113.0/24"}
	if got := getTenantApp(t, cl, "tea-a", "layer-web").Spec.WorkspaceIPAllowList; !slices.Equal(got, want) {
		t.Errorf("web service layer = %v, want %v", got, want)
	}
	if got := getTenantApp(t, cl, "tea-a", "layer-pserv").Spec.WorkspaceIPAllowList; got != nil {
		t.Errorf("private service carries layer %v", got)
	}
	var dbs appv1alpha1.DatabaseList
	if err := cl.List(ctx, &dbs); err != nil || len(dbs.Items) != 1 {
		t.Fatalf("databases = %d, %v", len(dbs.Items), err)
	}
	if got := dbs.Items[0].Spec.WorkspaceIPAllowList; !slices.Equal(got, want) {
		t.Errorf("Postgres layer = %v, want %v", got, want)
	}
	var kvs appv1alpha1.KeyValueList
	if err := cl.List(ctx, &kvs); err != nil || len(kvs.Items) != 1 {
		t.Fatalf("key values = %d, %v", len(kvs.Items), err)
	}
	if got := kvs.Items[0].Spec.WorkspaceIPAllowList; !slices.Equal(got, want) {
		t.Errorf("Key Value layer = %v, want %v", got, want)
	}
}
