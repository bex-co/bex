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
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const blueprintDatastoreRules = `[{source: "203.0.113.7/32", description: office}, {source: "2001:db8::7/128", description: office-v6}]`

const blueprintBareRules = `[{source: 203.0.113.7, description: office}, {source: "2001:db8::7", description: office-v6}]`

func blueprintDatastoreManifest(databaseList, keyValueList string) string {
	return fmt.Sprintf(`databases:
  - name: access-db
    plan: free
%s
services:
  - type: keyvalue
    name: access-cache
    plan: free
    ipAllowList: %s
`, databaseList, keyValueList)
}

func TestBlueprintDatastoreCreateAccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		databaseList string
		keyValueList string
		wantPublic   bool
	}{
		{"explicit empty creates private datastores", "    ipAllowList: []", "[]", false},
		{"explicit rules publish datastores", "    ipAllowList: " + blueprintDatastoreRules, blueprintDatastoreRules, true},
		// Render's Blueprint source is "the IP address or CIDR block" (w8/142).
		{"bare addresses are single-host rules", "    ipAllowList: " + blueprintBareRules, blueprintBareRules, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil)
			manifest := blueprintDatastoreManifest(tc.databaseList, tc.keyValueList)
			result, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest})
			if err != nil {
				t.Fatal(err)
			}
			var wantRules []appv1alpha1.IPAllowEntry
			if tc.wantPublic {
				wantRules = []appv1alpha1.IPAllowEntry{
					{CIDR: "203.0.113.7/32", Description: "office"},
					{CIDR: "2001:db8::7/128", Description: "office-v6"},
				}
			}
			assertBlueprintDatastoreAccess(t, svc, result, tc.wantPublic, wantRules)
			assertBlueprintDatastorePlan(t, svc, manifest, BlueprintPlanNoop)
		})
	}
}

// Render: an omitted Postgres ipAllowList "allows all IPs for a new database"
// (credentials still required); Key Value requires the field, so its [] stays
// private. A re-sync of the same manifest is a no-op (w8/157).
func TestBlueprintOmittedPostgresAllowListCreatesPublic(t *testing.T) {
	t.Parallel()
	svc, _ := newService(nil)
	manifest := blueprintDatastoreManifest("", "[]")
	result, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	db := &appv1alpha1.Database{}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: result.Databases[0].ID}, db); err != nil {
		t.Fatal(err)
	}
	want := []appv1alpha1.IPAllowEntry{{CIDR: "0.0.0.0/0"}, {CIDR: "::/0"}}
	if !db.Spec.Public || !slices.Equal(db.Spec.IPAllowList, want) {
		t.Errorf("Postgres access = public %v rules %+v, want public %+v", db.Spec.Public, db.Spec.IPAllowList, want)
	}
	assertBlueprintDatastorePlan(t, svc, manifest, BlueprintPlanNoop)
}

func TestBlueprintDatastoreAccessPlanAndApply(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, cl := newService(nil)
	publicManifest := blueprintDatastoreManifest("    ipAllowList: "+blueprintDatastoreRules, blueprintDatastoreRules)
	result, err := svc.DeployStack(ctx, DeployRequest{Manifest: publicManifest})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		manifest   string
		wantPublic bool
		wantRules  []appv1alpha1.IPAllowEntry
	}{
		{"clear", blueprintDatastoreManifest("    ipAllowList: []", "[]"), false, nil},
		{"restore", publicManifest, true, []appv1alpha1.IPAllowEntry{
			{CIDR: "203.0.113.7/32", Description: "office"}, {CIDR: "2001:db8::7/128", Description: "office-v6"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertBlueprintDatastorePlan(t, svc, tc.manifest, BlueprintPlanUpdate)
			applied, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(applied, result) {
				t.Fatalf("update replaced datastore identity: got %+v, want %+v", applied, result)
			}
			assertBlueprintDatastoreAccess(t, svc, result, tc.wantPublic, tc.wantRules)
			assertBlueprintDatastorePlan(t, svc, tc.manifest, BlueprintPlanNoop)
		})
	}

	// The compiler must preserve field absence through the real sync path.
	const omitted = "databases:\n  - name: access-db\n"
	before := &appv1alpha1.Database{}
	key := client.ObjectKey{Namespace: "default", Name: result.Databases[0].ID}
	if err := cl.Get(ctx, key, before); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: omitted}); err != nil {
		t.Fatal(err)
	}
	after := &appv1alpha1.Database{}
	if err := cl.Get(ctx, key, after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Spec, after.Spec) {
		t.Fatalf("omitted ipAllowList changed live policy: before=%+v after=%+v", before.Spec, after.Spec)
	}
}

func assertBlueprintDatastoreAccess(t *testing.T, svc *Service, result StackResult, public bool, rules []appv1alpha1.IPAllowEntry) {
	t.Helper()
	if len(result.Databases) != 1 || len(result.KeyValues) != 1 {
		t.Fatalf("expected one database and Key Value, got %+v", result)
	}
	ctx := context.Background()
	db := &appv1alpha1.Database{}
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: result.Databases[0].ID}, db); err != nil {
		t.Fatal(err)
	}
	kv := &appv1alpha1.KeyValue{}
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: result.KeyValues[0].ID}, kv); err != nil {
		t.Fatal(err)
	}
	if db.Spec.Public != public || !slices.Equal(db.Spec.IPAllowList, rules) {
		t.Errorf("Postgres access = public %v rules %+v, want %v %+v", db.Spec.Public, db.Spec.IPAllowList, public, rules)
	}
	if kv.Spec.Public != public || !slices.Equal(kv.Spec.IPAllowList, rules) {
		t.Errorf("Key Value access = public %v rules %+v, want %v %+v", kv.Spec.Public, kv.Spec.IPAllowList, public, rules)
	}
}

func assertBlueprintDatastorePlan(t *testing.T, svc *Service, manifest string, operation BlueprintPlanOperation) {
	t.Helper()
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !validation.Valid || validation.Plan == nil {
		t.Fatalf("ValidateBlueprint: %+v, err %v", validation, err)
	}
	if len(validation.Plan.Actions) != 2 {
		t.Fatalf("expected two datastore actions, got %+v", validation.Plan.Actions)
	}
	for _, action := range validation.Plan.Actions {
		if action.Operation != operation {
			t.Errorf("%s operation = %s, want %s", action.Kind, action.Operation, operation)
		}
		if operation == BlueprintPlanUpdate && !slices.Equal(fieldPaths(action.ChangedFields), []string{"ipAllowList"}) {
			t.Errorf("%s changed fields = %v, want [ipAllowList]", action.Kind, action.ChangedFields)
		}
	}
}

// TestBlueprintKeyValueNameIsValidated (w5/m118): a Key Value name the CRD
// refuses is a validation error, as a Postgres one always was. Render puts no
// rule on a Blueprint name, so a free-form one maps onto a resource name first
// (w5/m150).
func TestBlueprintKeyValueNameIsValidated(t *testing.T) {
	t.Parallel()
	svc, _ := newService(nil)
	for _, name := range []string{"Cache_1", "cache-cache-cache-cache-cache-31", "Plan_Cache"} {
		manifest := fmt.Sprintf("services:\n  - type: keyvalue\n    name: %s\n    plan: free\n    ipAllowList: []\n", name)
		if v, err := svc.ValidateBlueprint(context.Background(), "", manifest, ""); err != nil || !v.Valid {
			t.Errorf("%s: validation = %+v, %v; want the free-form name accepted", name, v, err)
		}
	}
}

// A Render Postgres plan bex does not sell is a bex limit, not a Render syntax
// error; a name Render itself lacks fails the schema instead (w8/160).
func TestBlueprintUnsoldPostgresPlanNamesBexLimit(t *testing.T) {
	t.Parallel()
	svc, _ := newService(nil)
	for _, tc := range []struct {
		plan, want string
	}{
		{"pro-4gb", `plan "pro-4gb" is not available on bex; choose one of free|basic-256mb|basic-1gb`},
		{"basic-4gb", `plan "basic-4gb" is not available on bex`},
		{"accelerated-16gb", `plan "accelerated-16gb" is not available on bex`},
		{"pro-5gb", "value must be one of"},
	} {
		v, err := svc.ValidateBlueprint(context.Background(), "", "databases:\n  - name: d\n    plan: "+tc.plan+"\n", "")
		if err != nil || v.Valid || len(v.Errors) != 1 || !strings.Contains(v.Errors[0].Error, tc.want) {
			t.Errorf("plan %s: %+v, err %v; want %q", tc.plan, v.Errors, err, tc.want)
		}
	}
}
