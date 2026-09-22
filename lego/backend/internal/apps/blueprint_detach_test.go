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

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBlueprintDetachCandidatesAreOwnedSurvivingResources(t *testing.T) {
	svc, fs := connectionService(t)
	bp := connectA(t, svc)
	ctx := ownershipCtx()
	dbID, kvID := ids.New(ids.Postgres), ids.New(ids.KeyValue)
	resolver := &blueprintActionResolver{
		services:  map[string]*appv1alpha1.App{},
		databases: map[string]*appv1alpha1.Database{"data": {ObjectMeta: metav1.ObjectMeta{Name: dbID}, Spec: appv1alpha1.DatabaseSpec{Name: "data"}}},
		keyValues: map[string]*appv1alpha1.KeyValue{"cache": {ObjectMeta: metav1.ObjectMeta{Name: kvID}, Spec: appv1alpha1.KeyValueSpec{Name: "cache"}}},
	}
	for _, c := range []struct{ kind, name, owner string }{{"database", "data", bp.ID}, {"key_value", "cache", bp.ID}, {"service", "already-deleted", bp.ID}, {"service", "unrelated", ids.New(ids.Blueprint)}} {
		if err := fs.ClaimBlueprintResource(ctx, connOwner, c.kind, c.name, c.owner, ""); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.blueprintDetachments(ctx, connOwner, bp.ID, parsedStack{}, resolver)
	want := []BlueprintResource{{ID: kvID, Name: "cache", Type: "key_value"}, {ID: dbID, Name: "data", Type: "postgres"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("detached = %+v, %v; want %+v", got, err, want)
	}
	if owner, _ := fs.GetBlueprintResourceOwner(ctx, connOwner, "database", "data"); owner != bp.ID {
		t.Fatal("planning released ownership")
	}
}

func TestBlueprintFailedSyncDoesNotReportCompletedDetach(t *testing.T) {
	for _, failure := range []string{"apply", "completion"} {
		t.Run(failure, func(t *testing.T) {
			svc, fs := connectionService(t)
			bp := connectA(t, svc)
			if failure == "apply" {
				svc.Client = createErrorClient{Client: svc.Client, err: errors.New("injected create failure")}
			} else {
				fs.updateSyncErr = errors.New("injected completion failure")
			}
			result, err := svc.SyncBlueprint(ownershipCtx(), bp.ID, connOwner, m125CronManifest, "", nil)
			if err == nil || len(result.DetachedResources) != 0 {
				t.Fatalf("failed sync = %+v, %v", result, err)
			}
			for _, run := range fs.syncs {
				if strings.Contains(run.Note, "Detached") {
					t.Fatalf("failed run claimed completed detach: %+v", run)
				}
			}
			if failure == "apply" {
				if owner, _ := fs.GetBlueprintResourceOwner(context.Background(), connOwner, "service", "static-site"); owner != bp.ID {
					t.Fatal("failed apply released original resource")
				}
				var live appv1alpha1.AppList
				if err := svc.Client.List(context.Background(), &live); err != nil {
					t.Fatal(err)
				}
				if len(live.Items) != 1 || live.Items[0].Labels[core.LabelBlueprint] != bp.ID {
					t.Fatalf("failed apply changed original: %+v", live.Items)
				}
			}
		})
	}
}

func TestBlueprintRepeatedSyncDoesNotRepeatDetachNotice(t *testing.T) {
	svc, fs := connectionService(t)
	bp := connectA(t, svc)
	first, err := svc.SyncBlueprint(ownershipCtx(), bp.ID, connOwner, m125CronManifest, "", nil)
	if err != nil || len(first.DetachedResources) != 1 {
		t.Fatalf("first sync = %+v, %v", first, err)
	}
	second, err := svc.SyncBlueprint(ownershipCtx(), bp.ID, connOwner, m125CronManifest, "", nil)
	if err != nil || len(second.DetachedResources) != 0 {
		t.Fatalf("repeated sync = %+v, %v", second, err)
	}
	notices := 0
	for _, run := range fs.syncs {
		if strings.Contains(run.Note, "Detached") {
			notices++
		}
	}
	if notices != 1 {
		t.Fatalf("detach notices=%d, want1", notices)
	}
}

func TestBlueprintLegacyInvalidManifestCompletesAsError(t *testing.T) {
	svc, fs := connectionService(t)
	bp := connectA(t, svc)
	row := fs.blueprints[bp.ID]
	row.Repo, row.Manifest = "", "services: ["
	fs.blueprints[bp.ID] = row
	result, err := svc.SyncBlueprint(ownershipCtx(), bp.ID, connOwner, "", "", nil)
	if err == nil || len(result.DetachedResources) != 0 {
		t.Fatalf("invalid legacy sync = %+v, %v", result, err)
	}
	stored := fs.blueprints[bp.ID]
	if stored.Status != store.BlueprintStatusError || stored.ActiveRunID != "" {
		t.Fatalf("invalid legacy sync not completed as error: %+v", stored)
	}
	if owner, _ := fs.GetBlueprintResourceOwner(context.Background(), connOwner, "service", "static-site"); owner != bp.ID {
		t.Fatal("invalid legacy manifest released ownership")
	}
	for _, run := range fs.syncs {
		if strings.Contains(run.Note, "Detached") {
			t.Fatalf("invalid legacy sync reported detach: %+v", run)
		}
	}
}
