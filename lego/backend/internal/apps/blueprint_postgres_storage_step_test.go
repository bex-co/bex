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
	"errors"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintTestNow is the fixed clock the Blueprint database tests apply with.
var blueprintTestNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// A Blueprint sync is a manual storage write, held to Render's contract as the
// API is (w2/m317): an increase is a multiple of 5 GB, at most once every 6
// hours, and an unchanged size keeps syncing whatever it is.
func TestBlueprintDatabaseStorageIncreaseFollowsRenderRules(t *testing.T) {
	disk := map[string]BlueprintField{"diskSizeGB": {}}
	paid := func(storageGB int32, annotations map[string]string) *appv1alpha1.Database {
		return &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", Annotations: annotations},
			Spec:       appv1alpha1.DatabaseSpec{Name: "db", Plan: "basic-256mb", StorageGB: storageGB},
		}
	}
	conflictOn := func(t *testing.T, err error, want string) {
		t.Helper()
		var conflict *BlueprintFieldConflictError
		if !errors.As(err, &conflict) || conflict.Path != "diskSizeGB" || !strings.Contains(conflict.Message, want) {
			t.Fatalf("err = %v, want a diskSizeGB conflict naming %q", err, want)
		}
	}

	t.Run("a 5 GB step is applied and stamped", func(t *testing.T) {
		db := paid(10, nil)
		changed, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{StorageGB: 15}, disk, blueprintTestNow)
		if err != nil || !changed || db.Spec.StorageGB != 15 {
			t.Fatalf("changed=%v err=%v storage=%d", changed, err, db.Spec.StorageGB)
		}
		if got := db.Annotations[appv1alpha1.AnnotationStorageIncreasedAt]; got != "2026-11-10T12:00:00Z" {
			t.Fatalf("storage-increased-at = %q", got)
		}
	})

	t.Run("an off-step increase is a conflict and writes nothing", func(t *testing.T) {
		db := paid(10, nil)
		_, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{StorageGB: 12}, disk, blueprintTestNow)
		conflictOn(t, err, "multiple of 5 GB")
		if db.Spec.StorageGB != 10 || db.Annotations != nil {
			t.Fatalf("refusal mutated the database: %+v", db)
		}
	})

	t.Run("an increase within 6 hours of the last is a conflict naming when it opens", func(t *testing.T) {
		for _, stamp := range []string{appv1alpha1.AnnotationStorageIncreasedAt, appv1alpha1.AnnotationDiskAutoscaleAt} {
			last := blueprintTestNow.Add(-5*time.Hour - 59*time.Minute).Format(time.RFC3339)
			db := paid(10, map[string]string{stamp: last})
			_, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{StorageGB: 15}, disk, blueprintTestNow)
			conflictOn(t, err, "2026-11-10T12:01:00Z")
			if db.Spec.StorageGB != 10 {
				t.Fatalf("%s: a refused increase changed storage to %d", stamp, db.Spec.StorageGB)
			}
		}
	})

	t.Run("an increase 6 hours after the last is applied", func(t *testing.T) {
		db := paid(10, map[string]string{appv1alpha1.AnnotationStorageIncreasedAt: blueprintTestNow.Add(-6 * time.Hour).Format(time.RFC3339)})
		if _, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{StorageGB: 15}, disk, blueprintTestNow); err != nil || db.Spec.StorageGB != 15 {
			t.Fatalf("err=%v storage=%d", err, db.Spec.StorageGB)
		}
	})

	t.Run("an unchanged legacy size keeps syncing", func(t *testing.T) {
		recent := map[string]string{appv1alpha1.AnnotationStorageIncreasedAt: blueprintTestNow.Add(-time.Hour).Format(time.RFC3339)}
		db := paid(7, recent)
		if _, err := ApplyBlueprintDatabaseSpec(db, appv1alpha1.DatabaseSpec{StorageGB: 7}, disk, blueprintTestNow); err != nil || db.Spec.StorageGB != 7 {
			t.Fatalf("err=%v storage=%d", err, db.Spec.StorageGB)
		}
	})
}

// A new Blueprint database takes Render's create sizes: 1 GB or a multiple of
// 5 GB. The plan default (diskSizeGB omitted) is untouched.
func TestBlueprintDatabaseCreateStorageStep(t *testing.T) {
	for _, tc := range []struct {
		manifest string
		refused  bool
	}{
		{"databases:\n  - name: odd\n    plan: basic-256mb\n    diskSizeGB: 7\n", true},
		{"databases:\n  - name: step\n    plan: basic-256mb\n    diskSizeGB: 15\n", false},
		{"databases:\n  - name: one\n    plan: free\n    diskSizeGB: 1\n", false},
		{"databases:\n  - name: default\n    plan: basic-256mb\n", false},
	} {
		parsed, err := parseStack(DeployRequest{Manifest: tc.manifest})
		if err != nil {
			t.Fatalf("%q: %v", tc.manifest, err)
		}
		err = checkBlueprintDatabaseCreates(parsed)
		if refused := err != nil; refused != tc.refused {
			t.Fatalf("%q: err=%v, want refused=%v", tc.manifest, err, tc.refused)
		}
		if tc.refused && !strings.Contains(err.Error(), "multiple of 5 GB") {
			t.Fatalf("%q: refusal %v does not name the step", tc.manifest, err)
		}
	}
}
