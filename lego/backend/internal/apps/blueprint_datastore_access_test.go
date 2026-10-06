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

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const blueprintDatastoreRules = `[{source: "203.0.113.7/32", description: office}, {source: "2001:db8::7/128", description: office-v6}]`

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
		{"omitted Postgres retains private create default", "", "[]", false},
		{"explicit empty creates private datastores", "    ipAllowList: []", "[]", false},
		{"explicit rules publish datastores", "    ipAllowList: " + blueprintDatastoreRules, blueprintDatastoreRules, true},
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
// refuses is a validation error, as a Postgres one always was. Before, validate
// passed and the apply's CR create answered the apiserver's refusal as a 500.
func TestBlueprintKeyValueNameIsValidated(t *testing.T) {
	t.Parallel()
	svc, _ := newService(nil)
	for _, name := range []string{"Cache_1", "cache-cache-cache-cache-cache-31", "Plan_Cache"} {
		manifest := fmt.Sprintf("services:\n  - type: keyvalue\n    name: %s\n    plan: free\n    ipAllowList: []\n", name)
		v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
		if err != nil {
			t.Fatalf("%s: ValidateBlueprint = %v, want a validation result", name, err)
		}
		if v.Valid || len(v.Errors) == 0 || !strings.Contains(v.Errors[0].Error, core.ResourceNameRule) ||
			v.Errors[0].Code != "RESOURCE_NAME_INVALID" || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].name" {
			t.Errorf("%s: validation = %+v, want the name refused with RESOURCE_NAME_INVALID at services[0].name (w5/093)", name, v)
		}
	}
}
