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
	"errors"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint's postgresMajorVersion move is a major-version upgrade, judged
// by the same rule as the API's upgrade verbs (REST PATCH, GraphQL
// updateDatabaseVersion/upgradeDatabase, MCP update_postgres): the file plans
// exactly what the API accepts, and an accepted sync records the run the
// dashboard's lastUpgradeRun and acknowledge read. Until this was pinned the
// Blueprint path set spec.version with only the schema enum and admission
// behind it, so a file could downgrade a live database (16 -> 15) or retarget
// below the in-place floor (16 -> 14) — both refused on every API surface —
// with no run recorded, and the operator projected the older image onto the
// cluster.
func versionedLiveDB(version, current, phase string) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "db", Plan: "basic-1gb", Version: version, StorageGB: 10},
		Status:     appv1alpha1.DatabaseStatus{CurrentVersion: current, Phase: appv1alpha1.DatabasePhase(phase)},
	}
}

func seededVersionedDB() *appv1alpha1.Database {
	return versionedLiveDB("16", "16", "")
}

// versionedBlueprintService is newService with databases seeded: the
// store-off shape other Blueprint datastore tests use (no Workspace resolver,
// fixtures in the shared namespace).
func versionedBlueprintService(dbs ...*appv1alpha1.Database) (*Service, client.Client) {
	objs := make([]client.Object, len(dbs))
	for i, d := range dbs {
		objs[i] = d
	}
	cl := fakeClient(objs...)
	return &Service{
		Base:          &core.Base{Client: cl, Namespace: "default"},
		DomainTraffic: allowDomainTraffic{},
	}, cl
}

func storedBlueprintDB(t *testing.T, cl client.Client) *appv1alpha1.Database {
	t.Helper()
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "db"}, &db); err != nil {
		t.Fatal(err)
	}
	return &db
}

func TestBlueprintPostgresMajorVersionFollowsTheUpgradeGate(t *testing.T) {
	t.Parallel()
	fields := map[string]BlueprintField{"postgresMajorVersion": {}}
	for _, tc := range []struct {
		name    string
		live    *appv1alpha1.Database
		want    string
		changed bool
		refused bool
	}{
		{name: "upgrade 16 to 17 applies", live: versionedLiveDB("16", "16", ""), want: "17", changed: true},
		{name: "same version is a no-op", live: versionedLiveDB("16", "16", ""), want: "16"},
		{name: "downgrade 16 to 15 is refused", live: versionedLiveDB("16", "16", ""), want: "15", refused: true},
		{name: "below-floor 14 to 15 is refused", live: versionedLiveDB("14", "14", ""), want: "15", refused: true},
		{name: "below-floor 16 to 14 is refused", live: versionedLiveDB("16", "16", ""), want: "14", refused: true},
		{name: "a running upgrade blocks a retarget", live: versionedLiveDB("17", "16", "Upgrading"), want: "18", refused: true},
		{name: "an unobserved version blocks a move", live: versionedLiveDB("16", "", ""), want: "17", refused: true},
		{name: "an unknown major is refused", live: versionedLiveDB("16", "16", ""), want: "19", refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := tc.live.Spec.DeepCopy()
			beforeStatus := tc.live.Status
			changed, err := ApplyBlueprintDatabaseSpec(tc.live, appv1alpha1.DatabaseSpec{Version: tc.want}, fields, blueprintTestNow)
			if tc.refused {
				var conflict *BlueprintFieldConflictError
				if !errors.As(err, &conflict) || conflict.Path != "postgresMajorVersion" {
					t.Fatalf("want a postgresMajorVersion conflict, got changed=%v err=%v", changed, err)
				}
				// The file refuses in the API's own words: the conflict
				// carries the gate's message for the same live state.
				probe := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Version: before.Version}, Status: beforeStatus}
				if apiErr := postgres.CheckVersionUpgradeTarget(probe, tc.want); apiErr == nil || conflict.Message != apiErr.Error() {
					t.Fatalf("conflict message = %q, API gate = %v", conflict.Message, apiErr)
				}
				if !reflect.DeepEqual(tc.live.Spec, *before) {
					t.Fatalf("refusal mutated the spec: %#v", tc.live.Spec)
				}
				return
			}
			if err != nil || changed != tc.changed {
				t.Fatalf("changed=%v err=%v, want changed=%v", changed, err, tc.changed)
			}
			if tc.live.Spec.Version != tc.want {
				t.Fatalf("spec.version = %q, want %q", tc.live.Spec.Version, tc.want)
			}
		})
	}
}

// A create names its birth version; only a move away from a version already
// in effect is an upgrade the gate judges.
func TestBlueprintPostgresMajorVersionCreateIsNotAnUpgrade(t *testing.T) {
	t.Parallel()
	live := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Name: "db", Plan: "basic-1gb", StorageGB: 10}}
	changed, err := ApplyBlueprintDatabaseSpec(live,
		appv1alpha1.DatabaseSpec{Version: "13"},
		map[string]BlueprintField{"postgresMajorVersion": {}}, blueprintTestNow)
	if err != nil || !changed || live.Spec.Version != "13" {
		t.Fatalf("birth version refused: changed=%v err=%v spec=%#v", changed, err, live.Spec)
	}
}

// A downgrade file is invalid on every validation surface, naming the field
// the sync would refuse.
func TestBlueprintPostgresDowngradeInvalidOnEverySurface(t *testing.T) {
	manifest := "databases:\n  - name: db\n    plan: basic-1gb\n    postgresMajorVersion: \"15\"\n"
	svc, _ := versionedBlueprintService(seededVersionedDB())
	for _, surface := range blueprintValidationSurfaces(t, svc) {
		t.Run(surface.name, func(t *testing.T) {
			out := surface.validate(t, manifest)
			if out["valid"] != false {
				t.Fatalf("a downgrade file validated: %+v", out)
			}
		})
	}
	v, err := svc.ValidateBlueprint(ctxAs("dana"), "", manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Errors) != 1 {
		t.Fatalf("errors = %+v, want the one version refusal", v.Errors)
	}
	e := v.Errors[0]
	if e.Path == nil || *e.Path != "databases[0].postgresMajorVersion" || !strings.Contains(e.Error, "upward-only") {
		t.Fatalf("refusal = %q at %v, want the upward-only rule at databases[0].postgresMajorVersion", e.Error, ptrValue(e.Path))
	}
}

// An accepted Blueprint upgrade records its run on the sync, as the API's
// upgrade verbs do: from the observed major, to the declared one.
func TestBlueprintUpgradeStampsItsRunOnSync(t *testing.T) {
	svc, cl := versionedBlueprintService(seededVersionedDB())
	manifest := "databases:\n  - name: db\n    plan: basic-1gb\n    postgresMajorVersion: \"17\"\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	stored := storedBlueprintDB(t, cl)
	if stored.Spec.Version != "17" {
		t.Fatalf("spec.version = %q, want 17", stored.Spec.Version)
	}
	var record struct {
		RunID       string `json:"runId"`
		FromVersion string `json:"fromVersion"`
		ToVersion   string `json:"toVersion"`
		StartedAt   string `json:"startedAt"`
	}
	raw := stored.Annotations["app.bex.co/upgrade-run"]
	if err := json.Unmarshal([]byte(raw), &record); err != nil ||
		record.RunID == "" || record.FromVersion != "16" || record.ToVersion != "17" || record.StartedAt == "" {
		t.Fatalf("upgrade-run record = %q (err %v), want a 16 -> 17 run", raw, err)
	}
	// A re-sync of the same file keeps the run instead of stamping a new one.
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	again := storedBlueprintDB(t, cl)
	if again.Annotations["app.bex.co/upgrade-run"] != raw {
		t.Fatalf("re-sync restamped the run:\n first: %s\nsecond: %s", raw, again.Annotations["app.bex.co/upgrade-run"])
	}
}

// A downgrade sync is refused before any write, like the API's verbs.
func TestBlueprintDowngradeSyncWritesNothing(t *testing.T) {
	svc, cl := versionedBlueprintService(seededVersionedDB())
	manifest := "databases:\n  - name: db\n    plan: basic-1gb\n    postgresMajorVersion: \"15\"\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err == nil ||
		!strings.Contains(err.Error(), "postgresMajorVersion") {
		t.Fatalf("sync err = %v, want a postgresMajorVersion refusal", err)
	}
	if got := storedBlueprintDB(t, cl).Spec.Version; got != "16" {
		t.Fatalf("refused sync moved spec.version to %q", got)
	}
}
