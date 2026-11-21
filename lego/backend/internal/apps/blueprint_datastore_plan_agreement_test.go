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

// The datastore plan/apply cycle rests on the same two invariants the service
// matrix pins in blueprint_plan_property_test.go, which has no database or
// key-value equivalent:
//
//  1. Apply is idempotent. Applying a manifest entry to the spec it just
//     produced is a no-op: a sync re-planned against its own result must
//     report no change, or every sync forever plans an update it need not
//     make (the w4/m124 family; the env-group member was fixed in w8/376).
//  2. The plan names what apply does. The per-field oneField probes agree
//     with the whole apply: a no-op names nothing that moves, an update
//     names at least one field that really moves in isolation, and a refusal
//     writes nothing.
//
// The manifests below are valid render.yaml documents, each declaring a
// distinct field family; the live specs are the shapes a real datastore
// reaches (zero value, a free store, a populated paid store). A refused whole
// apply is a plan conflict, never an update, so the probes are only judged
// when the whole apply succeeds — but the refusal must leave the live spec
// untouched, as the applier returns before its first write. A probe that
// errors in isolation is reported rather than dropped, exactly as
// blueprintPlanFieldChanges does (a plan change that strands kept autoscaling
// is refused whole, while each half alone applies), so only a clean isolated
// no-op contradicts a report.

import (
	"reflect"
	"sort"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var blueprintDatastorePlanPropertyManifests = map[string]string{
	"db-free": `databases:
  - name: db
    plan: free
`,
	"db-paid": `databases:
  - name: db
    plan: basic-1gb
    postgresMajorVersion: "16"
    diskSizeGB: 10
    storageAutoscalingEnabled: true
    connectionPool: pgbouncer
    ipAllowList:
      - source: 10.0.0.0/8
        description: internal
`,
	"db-growth": `databases:
  - name: db
    plan: basic-1gb
    diskSizeGB: 15
`,
	"db-bad-step": `databases:
  - name: db
    plan: basic-1gb
    diskSizeGB: 12
`,
	"db-plan-change": `databases:
  - name: db
    plan: basic-256mb
`,
	"db-ha": `databases:
  - name: db
    plan: 1c-2g
    diskSizeGB: 10
    highAvailability:
      enabled: true
`,
	"db-replicas": `databases:
  - name: db
    plan: basic-1gb
    diskSizeGB: 10
    readReplicas:
      - name: ro
`,
	"kv-free": `services:
  - name: cache
    type: keyvalue
    plan: free
    ipAllowList: []
`,
	"kv-starter": `services:
  - name: cache
    type: keyvalue
    plan: starter
    ipAllowList:
      - source: 10.0.0.0/8
        description: internal
    maxmemoryPolicy: volatile-lru
    persistenceMode: snapshot
`,
	"kv-plan-change": `services:
  - name: cache
    type: keyvalue
    plan: standard
    ipAllowList: []
`,
}

// blueprintDatastorePlanPropertyLiveDBs are the existing databases a manifest
// entry can be planned against.
func blueprintDatastorePlanPropertyLiveDBs() map[string]appv1alpha1.DatabaseSpec {
	return map[string]appv1alpha1.DatabaseSpec{
		"zero": {},
		"free": {
			Name: "db", Plan: "free", Version: "16", StorageGB: 1,
		},
		"paid": {
			Name: "db", Plan: "basic-1gb", Version: "16", StorageGB: 10,
			DiskAutoscaling: true, Pooler: true, Public: true,
			IPAllowList: []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "internal"}},
		},
	}
}

// blueprintDatastorePlanPropertyLiveKVs are the existing stores a manifest
// entry can be planned against.
func blueprintDatastorePlanPropertyLiveKVs() map[string]appv1alpha1.KeyValueSpec {
	return map[string]appv1alpha1.KeyValueSpec{
		"zero": {},
		"free": {Name: "cache", Plan: "free"},
		"full": {
			Name: "cache", Plan: "starter", Public: true,
			IPAllowList:     []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "internal"}},
			MaxmemoryPolicy: "volatile-lru", PersistenceMode: "snapshot",
		},
	}
}

func TestBlueprintDatabasePlanAndApplyAgree(t *testing.T) {
	t.Parallel()
	for name, manifest := range blueprintDatastorePlanPropertyManifests {
		st := parseBlueprintStackForTest(t, manifest)
		if len(st.databases) != 1 {
			continue
		}
		db := st.databases[0]
		for liveName, live := range blueprintDatastorePlanPropertyLiveDBs() {
			t.Run(name+"/"+liveName, func(t *testing.T) {
				t.Parallel()
				liveDB := &appv1alpha1.Database{Spec: *live.DeepCopy()}
				before := liveDB.Spec.DeepCopy()

				changed, err := ApplyBlueprintDatabaseSpec(liveDB, db.spec, db.fields, blueprintTestNow)
				if err != nil {
					// A refusal is a plan conflict, never an update: the
					// applier returns before its first write.
					if !reflect.DeepEqual(liveDB.Spec, *before) {
						t.Fatalf("refused apply mutated the live spec:\n before: %#v\n  after: %#v", *before, liveDB.Spec)
					}
					return
				}

				// Invariant 1: apply is idempotent.
				again := &appv1alpha1.Database{Spec: *liveDB.Spec.DeepCopy()}
				changedAgain, err := ApplyBlueprintDatabaseSpec(again, db.spec, db.fields, blueprintTestNow)
				if err != nil {
					t.Fatalf("second apply refused after a successful first: %v", err)
				}
				if changedAgain || !reflect.DeepEqual(again.Spec, liveDB.Spec) {
					t.Fatalf("apply is not idempotent: second apply changed=%v\n first: %#v\nsecond: %#v", changedAgain, liveDB.Spec, again.Spec)
				}

				// Invariant 1b: a stamped storage increase does not flap.
				// The first apply stamps the increase; a re-plan an hour
				// later against the stamped live spec is still a no-op —
				// the high-water mark rose with the request, so the wait
				// gate does not judge an increase that already happened.
				stamped := &appv1alpha1.Database{Spec: *liveDB.Spec.DeepCopy()}
				stamped.Annotations = liveDB.Annotations
				later := blueprintTestNow.Add(time.Hour)
				changedLater, err := ApplyBlueprintDatabaseSpec(stamped, db.spec, db.fields, later)
				if err != nil {
					t.Fatalf("re-apply an hour later refused after a successful first: %v", err)
				}
				if changedLater || !reflect.DeepEqual(stamped.Spec, liveDB.Spec) {
					t.Fatalf("stamped re-apply is not a no-op: changed=%v\n first: %#v\nsecond: %#v", changedLater, liveDB.Spec, stamped.Spec)
				}

				// Invariant 1c: the 6-hour wait binds the next increase on
				// both sides. When the first apply stamped an increase, a
				// further step-valid growth an hour later is refused and
				// writes nothing — the same conflict the plan surfaces, so
				// the re-plan refuses exactly what the apply refuses.
				if _, ok := liveDB.Annotations[appv1alpha1.AnnotationStorageIncreasedAt]; ok {
					next := *liveDB.Spec.DeepCopy()
					next.StorageGB += 5
					growing := &appv1alpha1.Database{Spec: *liveDB.Spec.DeepCopy()}
					growing.Annotations = liveDB.Annotations
					growFields := map[string]BlueprintField{"diskSizeGB": {}}
					if _, err := ApplyBlueprintDatabaseSpec(growing, next, growFields, later); err == nil {
						t.Fatalf("further growth an hour after a stamped increase was allowed")
					} else if !reflect.DeepEqual(growing.Spec, liveDB.Spec) {
						t.Fatalf("refused growth mutated the live spec:\n before: %#v\n  after: %#v", liveDB.Spec, growing.Spec)
					}
				}

				// Invariant 2: the per-field probes agree with the whole
				// apply. A probe that errors in isolation is reported,
				// exactly as the plan lists it.
				probed := map[string]bool{}
				for field := range db.fields {
					probe := &appv1alpha1.Database{Spec: *before.DeepCopy()}
					moved, err := ApplyBlueprintDatabaseSpec(probe, db.spec, oneField(db.fields, field), blueprintTestNow)
					if err != nil || moved {
						probed[field] = true
					}
				}
				switch {
				case changed && len(probed) == 0:
					t.Fatalf("apply changes the spec but no probed field reports it:\n before: %#v\n  after: %#v", *before, liveDB.Spec)
				case !changed:
					for field := range probed {
						probe := &appv1alpha1.Database{Spec: *before.DeepCopy()}
						if moved, err := ApplyBlueprintDatabaseSpec(probe, db.spec, oneField(db.fields, field), blueprintTestNow); err == nil && moved {
							t.Fatalf("apply is a no-op but the plan would report %q", field)
						}
					}
				}
				if !changed {
					return
				}
				// Every reported field really moves in isolation or is
				// unevaluable there — never a clean no-op.
				for field := range probed {
					probe := &appv1alpha1.Database{Spec: *before.DeepCopy()}
					if moved, err := ApplyBlueprintDatabaseSpec(probe, db.spec, oneField(db.fields, field), blueprintTestNow); err == nil && !moved {
						t.Fatalf("plan reports %q but its isolated apply is a clean no-op", field)
					}
				}
				var names []string
				for field := range probed {
					names = append(names, field)
				}
				sort.Strings(names)
				t.Logf("update names %v", names)
			})
		}
	}
}

func TestBlueprintKeyValuePlanAndApplyAgree(t *testing.T) {
	t.Parallel()
	for name, manifest := range blueprintDatastorePlanPropertyManifests {
		st := parseBlueprintStackForTest(t, manifest)
		if len(st.keyValues) != 1 {
			continue
		}
		kv := st.keyValues[0]
		for liveName, live := range blueprintDatastorePlanPropertyLiveKVs() {
			t.Run(name+"/"+liveName, func(t *testing.T) {
				t.Parallel()
				got := *live.DeepCopy()
				before := *live.DeepCopy()

				changed := ApplyBlueprintKeyValueSpec(&got, kv.spec, kv.fields)

				// Invariant 1: apply is idempotent.
				again := *got.DeepCopy()
				if changedAgain := ApplyBlueprintKeyValueSpec(&again, kv.spec, kv.fields); changedAgain || !reflect.DeepEqual(again, got) {
					t.Fatalf("apply is not idempotent: second apply changed=%v\n first: %#v\nsecond: %#v", changedAgain, got, again)
				}

				// Invariant 2: the per-field probes agree with the whole apply.
				var probed []string
				for field := range kv.fields {
					probe := *before.DeepCopy()
					if ApplyBlueprintKeyValueSpec(&probe, kv.spec, oneField(kv.fields, field)) {
						probed = append(probed, field)
					}
				}
				switch {
				case changed && len(probed) == 0:
					t.Fatalf("apply changes the spec but no probed field reports it:\n before: %#v\n  after: %#v", before, got)
				case !changed && len(probed) > 0:
					t.Fatalf("apply is a no-op but the plan would report %v", probed)
				}
				if !changed {
					return
				}
				for _, field := range probed {
					probe := *before.DeepCopy()
					if !ApplyBlueprintKeyValueSpec(&probe, kv.spec, oneField(kv.fields, field)) {
						t.Fatalf("plan reports %q but its isolated apply is a no-op", field)
					}
				}
			})
		}
	}
}
