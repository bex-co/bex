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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/358: a Blueprint sync has no confirm channel (and the auto-sync worker
// runs unattended), so it refuses the first external-access enable on a
// running Persistence Off Key Value, whose restart deletes every key.
func TestBlueprintRefusesExternalAccessThatWouldEmptyAPersistenceOffKeyValue(t *testing.T) {
	const open = `
services:
  - name: cache
    type: redis
    ipAllowList:
      - source: 203.0.113.0/24
        description: office
    plan: free
`
	for _, tc := range []struct {
		name    string
		mutate  func(*appv1alpha1.KeyValue)
		refused bool
	}{
		{"persistence off, first enable", func(kv *appv1alpha1.KeyValue) { kv.Spec.PersistenceMode = appv1alpha1.KeyValuePersistenceOff }, true},
		{"persistence on", func(kv *appv1alpha1.KeyValue) { kv.Spec.PersistenceMode = "" }, false},
		{"persistence off, already public", func(kv *appv1alpha1.KeyValue) {
			kv.Spec.PersistenceMode, kv.Spec.Public = appv1alpha1.KeyValuePersistenceOff, true
		}, false},
		{"persistence off, suspended", func(kv *appv1alpha1.KeyValue) {
			kv.Spec.PersistenceMode, kv.Spec.Suspended = appv1alpha1.KeyValuePersistenceOff, true
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			ctx := context.Background()
			res, err := svc.DeployStack(ctx, DeployRequest{Manifest: "services:\n  - name: cache\n    type: redis\n    ipAllowList: []\n    plan: free\n"})
			if err != nil {
				t.Fatalf("first deploy: %v", err)
			}
			var kv appv1alpha1.KeyValue
			if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&kv)
			if err := cl.Update(ctx, &kv); err != nil {
				t.Fatal(err)
			}
			_, err = svc.DeployStack(ctx, DeployRequest{Manifest: open})
			if !tc.refused {
				if err != nil {
					t.Fatalf("sync refused unexpectedly: %v", err)
				}
				return
			}
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "deletes every key") {
				t.Fatalf("sync error = %v, want a named bad-request refusal", err)
			}
			if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil || kv.Spec.Public {
				t.Fatalf("refused sync still changed the store: public=%v err=%v", kv.Spec.Public, err)
			}
		})
	}
}

// w4/m316: a maxmemoryPolicy change also replaces a running store's pod, and
// with Persistence Off that deletes every key. Like a plan change (w4/m263),
// and unlike the bex-only external-access restart above, Blueprints apply it
// as Render does: the field is Render's, and refusing it would make a
// Persistence Off store's policy unchangeable from its own Blueprint.
func TestBlueprintAppliesAMaxmemoryChangeToARunningPersistenceOffKeyValue(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	manifest := func(policy string) string {
		return "services:\n  - name: cache\n    type: redis\n    ipAllowList: []\n    plan: free\n    maxmemoryPolicy: " + policy + "\n"
	}
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("allkeys-lru")})
	if err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
		t.Fatal(err)
	}
	kv.Spec.PersistenceMode = appv1alpha1.KeyValuePersistenceOff
	if err := cl.Update(ctx, &kv); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest("noeviction")}); err != nil {
		t.Fatalf("sync refused the policy change: %v", err)
	}
	if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil || kv.Spec.MaxmemoryPolicy != "noeviction" {
		t.Fatalf("policy after sync = %q (err %v), want noeviction", kv.Spec.MaxmemoryPolicy, err)
	}
}
