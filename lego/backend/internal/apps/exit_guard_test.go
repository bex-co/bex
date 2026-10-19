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
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m232: a Blueprint re-apply that regroups an existing service or datastore
// takes it out of its environment, so an ACL-bearing one needs its admin like
// every other placement writer. Regrouping into one stays open.

func refuseProd(exits *[]string) core.ExitGuardLoader {
	return func(context.Context, string) (core.EnvironmentExitGuard, error) {
		return func(ids ...string) error {
			*exits = append(*exits, ids...)
			if slices.Contains(ids, "env-prod") {
				return core.ErrForbidden
			}
			return nil
		}, nil
	}
}

func TestBlueprintServiceRegroupChecksTheEnvironmentItLeaves(t *testing.T) {
	ctx := context.Background()
	st := &movingEnvironmentStore{blueprintGroupingTestStore: &blueprintGroupingTestStore{recordingStore: &recordingStore{}}, appEnv: map[string]string{"srv-1": "env-prod"}}
	var exits []string
	svc := &Service{Base: &core.Base{}, Store: st, ExitGuards: refuseProd(&exits)}
	existing := func() *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Labels: map[string]string{
			store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-1", core.LabelEnvironment: st.appEnv["srv-1"],
		}}}
	}

	err := svc.applyEnvironmentChange(ctx, existing(), core.EnvironmentAssignment{ID: "env-qa", ProjectID: "prj-1"})
	if !errors.Is(err, core.ErrForbidden) || st.appEnv["srv-1"] != "env-prod" {
		t.Fatalf("regroup out of prod = %v (row now %q), want ErrForbidden and the row in prod", err, st.appEnv["srv-1"])
	}
	st.appEnv["srv-1"] = "env-qa"
	if err := svc.applyEnvironmentChange(ctx, existing(), core.EnvironmentAssignment{ID: "env-prod", ProjectID: "prj-1"}); err != nil {
		t.Fatalf("regroup into prod: %v", err)
	}
	if st.appEnv["srv-1"] != "env-prod" || !slices.Equal(exits, []string{"env-prod", "env-qa"}) {
		t.Fatalf("row %q exits %v, want prod after checking only the bare source", st.appEnv["srv-1"], exits)
	}
}

func TestBlueprintDatastoreRegroupChecksTheEnvironmentItLeavesAndLocks(t *testing.T) {
	ctx := context.Background()
	var exits []string
	svc := &Service{Base: &core.Base{}, ExitGuards: refuseProd(&exits)}
	db := func(env string) *appv1alpha1.Database {
		return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-1", ResourceVersion: "7", Labels: map[string]string{core.LabelEnvironment: env}}}
	}

	if _, err := svc.regroupPatchBase(ctx, db("env-prod"), true, "env-qa"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("regroup out of prod = %v, want ErrForbidden", err)
	}
	// A spec-only re-apply leaves the grouping alone: no check, plain patch.
	if _, err := svc.regroupPatchBase(ctx, db("env-prod"), false, "env-prod"); err != nil {
		t.Fatalf("spec-only re-apply: %v", err)
	}
	patch, err := svc.regroupPatchBase(ctx, db("env-qa"), true, "env-prod")
	if err != nil {
		t.Fatalf("regroup into prod: %v", err)
	}
	// The regroup is locked to the snapshot the check read.
	moved := db("env-prod")
	data, err := patch.Data(moved)
	if err != nil || !strings.Contains(string(data), `"resourceVersion":"7"`) {
		t.Fatalf("regroup patch %s (err %v) is not optimistically locked", data, err)
	}
}

// TestBlueprintNoStoreRowRegroupChecksTheCRLabel: a service with no store row
// has only its CR label for a placement, so that label is the source vetted.
func TestBlueprintNoStoreRowRegroupChecksTheCRLabel(t *testing.T) {
	var exits []string
	svc := &Service{Base: &core.Base{}, ExitGuards: refuseProd(&exits)}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Labels: map[string]string{core.LabelEnvironment: "env-prod"}}}

	if err := svc.applyEnvironmentChange(context.Background(), app, core.EnvironmentAssignment{ID: "env-qa", ProjectID: "prj-1"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("regroup out of prod = %v, want ErrForbidden", err)
	}
	if !slices.Equal(exits, []string{"env-prod"}) {
		t.Fatalf("exits = %v, want the CR label's environment", exits)
	}
}

// TestBlueprintApplyRegroupingADatastoreOutOfAnEnvironmentIsVetted drives the
// stack apply end to end, so both datastore upserts are proven to route their
// regroup through the exit check.
func TestBlueprintApplyRegroupingADatastoreOutOfAnEnvironmentIsVetted(t *testing.T) {
	for kind, block := range map[string]string{
		"database":  "        databases:\n          - name: postgres\n            plan: basic-256mb\n",
		"key value": "        services:\n          - type: keyvalue\n            name: cache\n            plan: starter\n            ipAllowList: []\n",
	} {
		t.Run(kind, func(t *testing.T) {
			st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
			svc.BlueprintGroups, svc.Environments = st, st
			var refuse string
			svc.ExitGuards = func(context.Context, string) (core.EnvironmentExitGuard, error) {
				return func(ids ...string) error {
					if slices.Contains(ids, refuse) {
						return core.ErrForbidden
					}
					return nil
				}, nil
			}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
			manifest := func(environment string) string {
				return "version: \"1\"\nprojects:\n  - name: platform\n    environments:\n      - name: " + environment + "\n" + block
			}
			placed := func() string {
				t.Helper()
				var labels map[string]string
				if kind == "database" {
					var list appv1alpha1.DatabaseList
					if err := cl.List(ctx, &list); err != nil || len(list.Items) != 1 {
						t.Fatalf("databases = %v, %v", list.Items, err)
					}
					labels = list.Items[0].Labels
				} else {
					var list appv1alpha1.KeyValueList
					if err := cl.List(ctx, &list); err != nil || len(list.Items) != 1 {
						t.Fatalf("key values = %v, %v", list.Items, err)
					}
					labels = list.Items[0].Labels
				}
				return labels[core.LabelEnvironment]
			}

			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest("production")}); err != nil {
				t.Fatalf("first apply: %v", err)
			}
			refuse = placed()
			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest("staging")}); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("regroup out of production = %v, want ErrForbidden", err)
			}
			if got := placed(); got != refuse {
				t.Fatalf("a refused regroup moved the %s to %q", kind, got)
			}
			refuse = ""
			if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest("staging")}); err != nil {
				t.Fatalf("admitted regroup: %v", err)
			}
			if got := placed(); got == "" || got == st.environments[0].ID {
				t.Fatalf("admitted regroup left the %s in %q", kind, got)
			}
		})
	}
}
