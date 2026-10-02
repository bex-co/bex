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
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestBlueprintDatabaseCapacityRefusalsAreLocatedAndWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name, fields, field, reason string
	}{
		{"free replicas", "plan: free\n    readReplicas: [{name: reader}]", "readReplicas", "at least 0.5 CPU"},
		{"free storage", "plan: free\n    diskSizeGB: 15", "diskSizeGB", "fixed at 1 GB"},
		{"free autoscaling", "plan: free\n    storageAutoscalingEnabled: true", "storageAutoscalingEnabled", "requires a paid"},
		{"free pooler", "plan: free\n    connectionPool: pgbouncer", "connectionPool", "requires a paid"},
		{"replica storage floor", "plan: basic-1gb\n    readReplicas: [{name: reader}]", "readReplicas", "at least 10 GB"},
		{"replica count", "plan: basic-1gb\n    diskSizeGB: 10\n    readReplicas: [{name: a}, {name: b}, {name: c}, {name: d}, {name: e}, {name: f}]", "readReplicas", "at most 5"},
		{"paid storage cap", "plan: basic-256mb\n    diskSizeGB: 16385", "diskSizeGB", "limited to 16384 GB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := "databases:\n  - name: valid-first\n    plan: free\n  - name: invalid-second\n    " + tc.fields + "\n"
			cl := fakeClient()
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
			ctx := context.Background()
			validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
			if err != nil || validation.Valid || len(validation.Errors) != 1 {
				t.Fatalf("validation = %+v, %v", validation, err)
			}
			problem := validation.Errors[0]
			if problem.Path == nil || *problem.Path != "databases[1]."+tc.field || problem.Line == nil || *problem.Line < 6 || problem.Column == nil || !strings.Contains(problem.Error, tc.reason) {
				t.Fatalf("want located %s refusal containing %q, got %+v", tc.field, tc.reason, problem)
			}
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("apply = %v, want capacity refusal", err)
			}
			var databases appv1alpha1.DatabaseList
			if err := cl.List(ctx, &databases); err != nil || len(databases.Items) != 0 {
				t.Fatalf("refused stack partially created databases: %+v, %v", databases.Items, err)
			}
		})
	}
}

func TestBlueprintDatabaseCapacityMergesLiveStateBeforeAdmission(t *testing.T) {
	const create = `databases:
  - name: paid
    plan: basic-1gb
    diskSizeGB: 10
    readReplicas: [{name: first}]
`
	cl := fakeClient()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: create}); err != nil {
		t.Fatalf("supported create: %v", err)
	}
	const omittedPlan = "databases:\n  - name: paid\n    readReplicas: [{name: first}, {name: second}]\n"
	validation, err := svc.ValidateBlueprint(ctx, "", omittedPlan, "")
	if err != nil || !validation.Valid {
		t.Fatalf("omitted plan/storage must use eligible live state: %+v, %v", validation, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: omittedPlan}); err != nil {
		t.Fatalf("add replica preserving live capacity: %v", err)
	}
	var databases appv1alpha1.DatabaseList
	if err := cl.List(ctx, &databases); err != nil || len(databases.Items) != 1 {
		t.Fatalf("database inventory: %+v, %v", databases.Items, err)
	}
	before := databases.Items[0].DeepCopy()
	if before.Spec.Plan != "basic-1gb" || before.Spec.StorageGB != 10 || len(before.Spec.ReadReplicas) != 2 {
		t.Fatalf("omission changed capacity: %+v", before.Spec)
	}
	const downgrade = "databases:\n  - name: paid\n    plan: basic-256mb\n"
	validation, err = svc.ValidateBlueprint(ctx, "", downgrade, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 || validation.Errors[0].Path == nil || *validation.Errors[0].Path != "databases[0].plan" {
		t.Fatalf("downgrade must locate retained-replica conflict at plan: %+v, %v", validation, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: downgrade}); err == nil {
		t.Fatal("downgrade with retained replicas succeeded")
	}
	var after appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKeyFromObject(before), &after); err != nil || !reflect.DeepEqual(before, &after) {
		t.Fatalf("refusal mutated existing database: before=%+v after=%+v err=%v", before, after, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: downgrade + "    readReplicas: []\n"}); err != nil {
		t.Fatalf("explicit replica removal and paid downgrade: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(before), &after); err != nil || after.Spec.Plan != "basic-256mb" || after.Spec.StorageGB != 10 || len(after.Spec.ReadReplicas) != 0 {
		t.Fatalf("explicit removal lost storage or failed to apply: %+v, %v", after, err)
	}
}

func TestBlueprintDatabaseAdmissionPreservesAllocatedAndLegacyState(t *testing.T) {
	t.Run("allocated high water blocks downgrade and explicit shrink", func(t *testing.T) {
		db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "allocated", Namespace: "default"},
			Spec: appv1alpha1.DatabaseSpec{Name: "allocated", Plan: "basic-256mb", StorageGB: 1}, Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 15}}
		for _, tc := range []struct {
			field string
			want  appv1alpha1.DatabaseSpec
		}{
			{"plan", appv1alpha1.DatabaseSpec{Plan: "free"}},
			{"diskSizeGB", appv1alpha1.DatabaseSpec{StorageGB: 1}},
		} {
			got := db.DeepCopy()
			changed, err := ApplyBlueprintDatabaseSpec(got, tc.want, map[string]BlueprintField{tc.field: {}})
			var conflict *BlueprintFieldConflictError
			if changed || !errors.As(err, &conflict) || conflict.Path != tc.field || !reflect.DeepEqual(got, db) {
				t.Fatalf("%s: changed=%v err=%v database=%+v", tc.field, changed, err, got)
			}
		}
		svc := &Service{Base: &core.Base{Client: fakeClient(db), Namespace: "default"}}
		validation, err := svc.ValidateBlueprint(context.Background(), "", "databases:\n  - name: allocated\n    plan: free\n", "")
		if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, "15 GB") {
			t.Fatalf("stateful validation lost allocated storage: %+v, %v", validation, err)
		}
	})
	t.Run("legacy free state allows unrelated edits and disabling", func(t *testing.T) {
		db := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 15, Pooler: true, DiskAutoscaling: true,
			ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "legacy"}}}, Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 20}}
		changed, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{IPAllowList: []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8"}}}, map[string]BlueprintField{"ipAllowList": {}})
		if err != nil || !changed || !db.Spec.Pooler || !db.Spec.DiskAutoscaling || len(db.Spec.ReadReplicas) != 1 {
			t.Fatalf("unrelated legacy edit: %+v, %v", db, err)
		}
		changed, err = ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{}, map[string]BlueprintField{"connectionPool": {}, "storageAutoscalingEnabled": {}, "readReplicas": {}})
		if err != nil || !changed || db.Spec.Pooler || db.Spec.DiskAutoscaling || len(db.Spec.ReadReplicas) != 0 || db.Spec.StorageGB != 15 || db.Status.AllocatedStorageGB != 20 {
			t.Fatalf("disable must preserve storage: %+v, %v", db, err)
		}
	})
}
