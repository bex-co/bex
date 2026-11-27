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
	"testing"

	"k8s.io/utils/ptr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const workerScalingBlueprint = "services:\n  - type: worker\n    name: qa-bp-worker\n    runtime: image\n" +
	"    image: {url: nginx:1.27}\n    plan: starter\n" + diskRuleScaling

// w4/m695, render.com/docs/blueprint-spec § Scaling: the scaling fields "apply
// scaling settings for the following service types: Web services, Private
// services, Background workers". A worker's scaling block validates on REST,
// GraphQL and MCP, its sync turns the worker's autoscaling on with the
// declared range and target, and its export writes the same block back and
// re-plans as no change. A disk beside it is refused as a web service's is.
func TestBlueprintWorkerScalingAcrossSurfaces(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	withDisk := workerScalingBlueprint + diskRuleDisk
	for _, surface := range blueprintValidationSurfaces(t, svc) {
		t.Run(surface.name, func(t *testing.T) {
			if out := surface.validate(t, workerScalingBlueprint); out["valid"] != true {
				t.Fatalf("a worker's scaling block = %+v, want valid", out)
			}
			if out := surface.validate(t, withDisk); out["valid"] != false {
				t.Errorf("a worker's disk beside scaling validated: %+v", out)
			}
		})
	}

	validation, err := svc.ValidateBlueprint(ctx, "", withDisk, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = %+v, err %v; want the one disk refusal", validation, err)
	}
	const refusal = `service "qa-bp-worker": a service with a disk cannot use autoscaling`
	if got := validation.Errors[0]; got.Error != refusal || ptr.Deref(got.Path, "") != "services[0].scaling" {
		t.Fatalf("refusal = %q at %q, want %q at services[0].scaling, as a web service's", got.Error, ptr.Deref(got.Path, "<nil>"), refusal)
	}

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: workerScalingBlueprint}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	stored := getApp(t, cl, "qa-bp-worker")
	if a := stored.Spec.Autoscaling; stored.Spec.Type != appv1alpha1.TypeBackgroundWorker || a == nil || !a.Enabled ||
		a.MinReplicas != 1 || a.MaxReplicas != 2 || ptr.Deref(a.TargetCPUPercent, 0) != 60 || a.TargetMemoryPercent != nil {
		t.Fatalf("after the sync %s autoscaling = %+v, want on at 1..2, CPU 60", stored.Spec.Type, a)
	}

	svc.Client = fakeClient(serializedBlueprintApp(t, stored))
	entry, manifest := generatedService(t, svc, "qa-bp-worker")
	declared := map[string]any{"minInstances": float64(1), "maxInstances": float64(2), "targetCPUPercent": float64(60)}
	if !reflect.DeepEqual(entry["scaling"], declared) || entry["numInstances"] != nil {
		t.Fatalf("the export's scaling = %v beside numInstances %v, want the file's %v alone\n%s", entry["scaling"], entry["numInstances"], declared, manifest)
	}
	resync, err := svc.ValidateBlueprint(ctx, "", manifest, "")
	if err != nil || !resync.Valid || resync.Plan == nil || len(resync.Plan.Actions) != 1 {
		t.Fatalf("the export validates as %+v, err %v\n%s", resync, err, manifest)
	}
	if action := resync.Plan.Actions[0]; action.Operation != BlueprintPlanNoop {
		t.Fatalf("the export re-plans as %q, changing %v\n%s", action.Operation, fieldPaths(action.ChangedFields), manifest)
	}
}
