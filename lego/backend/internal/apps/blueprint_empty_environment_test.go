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
	"encoding/json"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintSerializedClient puts App reads through their real JSON contract.
// Fake clients otherwise retain Go-only nil/empty distinctions that disappear
// when Kubernetes persists an omitempty spec field. Both the planner's List
// and the apply path's Get must see the same serialized representation.
type blueprintSerializedClient struct{ client.Client }

func (c blueprintSerializedClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if app, ok := obj.(*appv1alpha1.App); ok {
		return blueprintJSONRoundTrip(app)
	}
	return nil
}

func (c blueprintSerializedClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, obj, opts...); err != nil {
		return err
	}
	if apps, ok := obj.(*appv1alpha1.AppList); ok {
		return blueprintJSONRoundTrip(apps)
	}
	return nil
}

func blueprintJSONRoundTrip[T any](value *T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var decoded T
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = decoded
	return nil
}

func assertBlueprintAppUnchanged(t *testing.T, before, after *appv1alpha1.App) {
	t.Helper()
	if after.ResourceVersion != before.ResourceVersion || after.Generation != before.Generation || after.Spec.RestartedAt != before.Spec.RestartedAt || after.Status.ActiveRevision != before.Status.ActiveRevision || !reflect.DeepEqual(after.Spec, before.Spec) {
		t.Errorf("unchanged apply mutated App: resourceVersion %s -> %s, generation %d -> %d, restartedAt %q -> %q, revision %q -> %q", before.ResourceVersion, after.ResourceVersion, before.Generation, after.Generation, before.Spec.RestartedAt, after.Spec.RestartedAt, before.Status.ActiveRevision, after.Status.ActiveRevision)
	}
}

func TestBlueprintEmptyInlineEnvironmentKeepsGroupAndSeedOwnership(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		group    bool
	}{
		{"group-only", "envVarGroups:\n  - name: shared\n    envVars: [{key: GROUP_VALUE, value: group}]\nservices:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: nginx:1}\n    envVars: [{fromGroup: shared}]\n", true},
		{"seed-only", "services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: nginx:1}\n    envVars:\n      - {key: SEEDED, value: initial, sync: false}\n      - {key: GENERATED, generateValue: true}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, raw := newService(nil)
			svc.Client = blueprintSerializedClient{Client: raw}
			kv := newMemKV()
			groups := &envgroups.Service{Base: svc.Base, Store: kv}
			sec := &secrets.Service{Base: svc.Base, Store: kv}
			svc.EnvGroups, svc.EnvSeeder = groups, sec
			ctx := context.Background()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest}); err != nil {
				t.Fatal(err)
			}
			generated := ""
			if !tc.group {
				value, err := sec.GetEnvVar(ctx, "web", "GENERATED")
				if err != nil || value.Value == "" {
					t.Fatalf("generated seed missing: %v", err)
				}
				generated = value.Value
				if _, err := sec.SetEnvVar(ctx, "web", "SEEDED", secrets.EnvVarWrite{Value: "dashboard-edit"}); err != nil {
					t.Fatal(err)
				}
			}
			before := getApp(t, svc.Client, "web")
			if before.Spec.Env != nil || (tc.group && len(before.Spec.EnvFromSecrets) != 1) || (!tc.group && before.Spec.EnvFromSecret == "") {
				t.Fatalf("inline environment/source ownership = %+v", before.Spec)
			}
			for range 2 {
				validation, err := svc.ValidateBlueprint(ctx, "", tc.manifest, "")
				if err != nil || !validation.Valid || validation.Plan == nil {
					t.Fatalf("validation = %+v, %v", validation, err)
				}
				wantActions := 1
				if tc.group {
					wantActions++
				}
				if len(validation.Plan.Actions) != wantActions {
					t.Fatalf("plan = %+v, want %d actions", validation.Plan, wantActions)
				}
				for _, action := range validation.Plan.Actions {
					if action.Kind == BlueprintResourceService && (action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0) {
						t.Errorf("service plan = %+v, want noop", action)
					}
					if action.Kind == BlueprintResourceEnvVarGroup && action.Operation != BlueprintPlanUpdate {
						t.Errorf("existing env group must keep its conservative update: %+v", action)
					}
				}
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest}); err != nil {
					t.Fatal(err)
				}
				assertBlueprintAppUnchanged(t, before, getApp(t, svc.Client, "web"))
			}
			if !tc.group {
				seeded, err := sec.GetEnvVar(ctx, "web", "SEEDED")
				if err != nil || seeded.Value != "dashboard-edit" {
					t.Errorf("sync:false overwrote mutable value: %v", err)
				}
				value, err := sec.GetEnvVar(ctx, "web", "GENERATED")
				if err != nil || value.Value != generated {
					t.Errorf("generated value was reminted: %v", err)
				}
			}
		})
	}
}
