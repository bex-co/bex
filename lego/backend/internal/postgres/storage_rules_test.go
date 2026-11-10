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

package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's storage contract (w2/m317): create takes 1 GB or a multiple of
// 5 GB, an increase is a multiple of 5 GB above the current size, and a second
// increase must wait 6 hours. Every case runs through REST, GraphQL and MCP.

var storageRulesNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func TestStorageIncreaseRulesMatchAcrossSurfaces(t *testing.T) {
	const setDisk = `mutation($id:String!, $gb:Int!) { updateDatabaseDiskSize(id:$id, diskSizeGB:$gb) { id } }`
	recent := storageRulesNow.Add(-5*time.Hour - 59*time.Minute).Format(time.RFC3339)
	old := storageRulesNow.Add(-6 * time.Hour).Format(time.RFC3339)
	for _, tc := range []struct {
		name        string
		before, gb  int32
		annotations map[string]string
		wantCode    string // "" accepted; "refused" a code-less refusal
	}{
		{name: "a 5 GB step", before: 10, gb: 15},
		{name: "an off-step increase", before: 10, gb: 12, wantCode: "POSTGRES_STORAGE_STEP_INVALID"},
		{name: "an unchanged legacy size", before: 7, gb: 7},
		{name: "a legacy size growing to the next step", before: 7, gb: 10},
		{name: "a legacy size growing off-step", before: 7, gb: 9, wantCode: "POSTGRES_STORAGE_STEP_INVALID"},
		{name: "within 6 hours of a manual increase", before: 10, gb: 15, annotations: map[string]string{appv1alpha1.AnnotationStorageIncreasedAt: recent}, wantCode: "POSTGRES_STORAGE_INCREASE_TOO_SOON"},
		{name: "within 6 hours of an autoscale", before: 10, gb: 15, annotations: map[string]string{appv1alpha1.AnnotationDiskAutoscaleAt: recent}, wantCode: "POSTGRES_STORAGE_INCREASE_TOO_SOON"},
		{name: "6 hours after the last increase", before: 10, gb: 15, annotations: map[string]string{appv1alpha1.AnnotationStorageIncreasedAt: old}},
		{name: "a shrink stays the grow-only refusal", before: 10, gb: 5, wantCode: "refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := pgGQLNewService()
			svc.Clock = func() time.Time { return storageRulesNow }
			for _, id := range []string{"rest-disk", "gql-disk", "mcp-disk"} {
				db := seedDatabaseSpec(t, cl, id, appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: tc.before}, false)
				if tc.annotations != nil {
					db.Annotations = tc.annotations
					if err := cl.Update(context.Background(), db); err != nil {
						t.Fatal(err)
					}
				}
			}
			restCode, restMessage := restPatchOutcome(t, svc, "rest-disk", fmt.Sprintf(`{"diskSizeGB":%d}`, tc.gb))
			gqlCode, gqlMessage := gqlSetterOutcome(t, svc, setDisk, map[string]any{"id": "gql-disk", "gb": int(tc.gb)})
			mcpCode := mcpPatchOutcome(t, svc, map[string]any{"postgresId": "mcp-disk", "diskSizeGB": tc.gb})
			if restMessage != "" && restCode == "" {
				restCode = "refused"
			}
			if gqlMessage != "" && gqlCode == "" {
				gqlCode = "refused"
			}
			if restCode != tc.wantCode || gqlCode != tc.wantCode || mcpCode != tc.wantCode {
				t.Fatalf("codes REST=%q GraphQL=%q MCP=%q, want %q (REST message %q)", restCode, gqlCode, mcpCode, tc.wantCode, restMessage)
			}
			wantGB := tc.gb
			if tc.wantCode != "" {
				wantGB = tc.before
			}
			for _, id := range []string{"rest-disk", "gql-disk", "mcp-disk"} {
				db := getDatabase(t, cl, id)
				if db.Spec.StorageGB != wantGB {
					t.Fatalf("%s storageGB = %d, want %d", id, db.Spec.StorageGB, wantGB)
				}
				stamped := db.Annotations[appv1alpha1.AnnotationStorageIncreasedAt] == storageRulesNow.Format(time.RFC3339)
				if increased := tc.wantCode == "" && tc.gb > tc.before; stamped != increased {
					t.Fatalf("%s stamped=%v, want %v (annotations %v)", id, stamped, increased, db.Annotations)
				}
			}
		})
	}
}

// The too-soon refusal says when the next increase is allowed.
func TestStorageIncreaseTooSoonNamesTheNextTime(t *testing.T) {
	db := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 10}}
	db.Annotations = map[string]string{appv1alpha1.AnnotationStorageIncreasedAt: "2026-11-10T08:00:00Z"}
	err := CheckStorageIncreaseWait(db, 15, storageRulesNow)
	if err == nil || err.Error() == "" {
		t.Fatal("an increase 4 hours after the last was accepted")
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("refusal is not coded: %v", err)
	}
	if coded.Code != "POSTGRES_STORAGE_INCREASE_TOO_SOON" || coded.Params["nextIncreaseAt"] != "2026-11-10T14:00:00Z" {
		t.Fatalf("refusal = %+v", coded)
	}
	if err := CheckStorageIncreaseWait(db, 10, storageRulesNow); err != nil {
		t.Fatalf("an unchanged size is not an increase: %v", err)
	}
}

// A dry run previews the increase without stamping it, so it cannot start the
// 6-hour wait.
func TestStorageIncreaseDryRunDoesNotStamp(t *testing.T) {
	svc, cl := pgGQLNewService()
	svc.Clock = func() time.Time { return storageRulesNow }
	seedDatabaseSpec(t, cl, "dry-disk", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 10}, false)
	if w := serveREST(svc, http.MethodPatch, "/v1/postgres/dry-disk", `{"diskSizeGB":15,"dryRun":true}`); w.Code != http.StatusOK {
		t.Fatalf("dry run => %d: %s", w.Code, w.Body)
	}
	if db := getDatabase(t, cl, "dry-disk"); db.Spec.StorageGB != 10 || db.Annotations[appv1alpha1.AnnotationStorageIncreasedAt] != "" {
		t.Fatalf("dry run wrote: storage=%d annotations=%v", db.Spec.StorageGB, db.Annotations)
	}
}

// Create takes 1 GB or a multiple of 5 GB on every surface; an omitted size
// keeps the plan default.
func TestCreateStorageStepMatchesAcrossSurfaces(t *testing.T) {
	for _, tc := range []struct {
		diskSizeGB int
		wantCode   string
	}{
		{0, ""},
		{1, ""},
		{15, ""},
		{7, "POSTGRES_STORAGE_STEP_INVALID"},
	} {
		for _, surface := range []string{"REST", "GraphQL", "MCP"} {
			svc, _ := pgGQLNewService()
			req := CreatePostgresRequest{Name: fmt.Sprintf("create-%d", tc.diskSizeGB), Plan: "basic-256mb", DiskSizeGB: int32(tc.diskSizeGB)}
			_, code := admissionCreateCall(t, svc, surface, req)
			if code != tc.wantCode {
				t.Fatalf("%s create diskSizeGB=%d: code %q, want %q", surface, tc.diskSizeGB, code, tc.wantCode)
			}
		}
	}
}

// A restore keeps its source's size whatever it is: recovery is not a
// caller-chosen create.
func TestRecoveryAdmitsAnOffStepSourceSize(t *testing.T) {
	if err := CheckDatabaseAdmission(nil, appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 7}); err != nil {
		t.Fatalf("recovery admission refused a 7 GB source: %v", err)
	}
	if err := CheckNewDatabase(appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 7}); err == nil {
		t.Fatal("a caller-chosen 7 GB create was admitted")
	}
}
