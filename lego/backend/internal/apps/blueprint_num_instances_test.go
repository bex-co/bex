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
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/utils/ptr"
)

// w4/m694, render.com/docs/blueprint-spec: numInstances "has no effect for
// services with autoscaling enabled", and an existing service "retains any
// existing autoscaling settings unless you add the scaling field in your
// Blueprint". A file that declares numInstances and no scaling validates on
// REST, GraphQL and MCP as an update of numInstances alone, and its sync
// stores the count and keeps the service autoscaling with its range and
// targets. A disk beside numInstances: 1 is refused, with advice that names
// the service, not the file, as the way to turn autoscaling off.
func TestBlueprintNumInstancesLeavesAutoscalingOnAcrossSurfaces(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: diskRuleWeb + diskRuleScaling}); err != nil {
		t.Fatal(err)
	}
	svc.Client = fakeClient(serializedBlueprintApp(t, getApp(t, cl, "qa-bp-web")))
	scaling := getApp(t, svc.Client, "qa-bp-web").Spec.Autoscaling.DeepCopy()
	if scaling == nil || !scaling.Enabled {
		t.Fatalf("autoscaling = %+v, want the declared range on", scaling)
	}
	counted := diskRuleWeb + "    numInstances: 3\n"
	withDisk := diskRuleWeb + "    numInstances: 1\n" + diskRuleDisk

	for _, surface := range blueprintValidationSurfaces(t, svc) {
		t.Run(surface.name, func(t *testing.T) {
			out := surface.validate(t, counted)
			actions, _ := out["plan"].(map[string]any)["actions"].([]any)
			if out["valid"] != true || len(actions) != 1 {
				t.Fatalf("validation = %+v", out)
			}
			action := actions[0].(map[string]any)
			if paths := actionPaths(t, surface.name, action); action["operation"] != "update" || !slices.Equal(paths, []string{"numInstances"}) {
				t.Errorf("%v changedFields = %v, want an update of exactly [numInstances]", action["operation"], paths)
			}
			if out := surface.validate(t, withDisk); out["valid"] != false {
				t.Errorf("a disk beside numInstances on an autoscaled service validated: %+v", out)
			}
		})
	}

	validation, err := svc.ValidateBlueprint(ctx, "", withDisk, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = %+v, err %v; want the one disk refusal", validation, err)
	}
	if got := validation.Errors[0]; !strings.HasSuffix(got.Error, "a service with a disk cannot use autoscaling; turn autoscaling off on the service first") ||
		got.Path == nil || *got.Path != "services[0].disk" {
		t.Fatalf("refusal = %q at %q, want the service-side advice at services[0].disk", got.Error, ptr.Deref(got.Path, "<nil>"))
	}

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: counted}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	got := getApp(t, svc.Client, "qa-bp-web").Spec
	if got.Replicas != 3 || !reflect.DeepEqual(got.Autoscaling, scaling) {
		t.Fatalf("after the sync replicas %d, autoscaling %+v; want the count 3 stored beside %+v as it was", got.Replicas, got.Autoscaling, scaling)
	}
}
