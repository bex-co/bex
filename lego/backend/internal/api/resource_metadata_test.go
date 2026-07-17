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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type metadataIdentities struct{}

func (metadataIdentities) Lookup(_ context.Context, subject string) (workspaces.IdentityAttrs, bool) {
	return workspaces.IdentityAttrs{Email: subject + "@example.com"}, true
}

func metadataResources(ownerID string) (*appv1alpha1.App, *appv1alpha1.Database, *appv1alpha1.KeyValue) {
	created := metav1.NewTime(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC))
	annotations := map[string]string{resourcemeta.UpdatedAtAnnotation: "2026-07-15T12:05:00Z"}
	labels := map[string]string{core.LabelTenant: ownerID, core.LabelWorkspace: ownerID}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc-metadata", Namespace: "default", CreationTimestamp: created,
			Labels: map[string]string{
				core.LabelTenant: ownerID, core.LabelWorkspace: ownerID,
				core.LabelAppID: "srv-metadata00000000000", core.LabelServiceName: "svc-metadata",
			},
			Annotations: annotations,
		},
		Spec: appv1alpha1.AppSpec{Image: "nginx", Runtime: "docker", Tier: "free", Replicas: 1},
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "pg-metadata", Namespace: "default", CreationTimestamp: created, Labels: labels, Annotations: annotations},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free", Version: "16"},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "kv-metadata", Namespace: "default", CreationTimestamp: created, Labels: labels, Annotations: annotations},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", Version: "8"},
	}
	return app, db, kv
}

func decodeObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode object: %v (%s)", err, body)
	}
	return out
}

func assertResourceMetadata(t *testing.T, object map[string]any, workspaceName, dashboardURL string, service bool) {
	t.Helper()
	owner, ok := object["owner"].(map[string]any)
	if !ok || owner["name"] != workspaceName || owner["type"] != "team" || owner["email"] != "client-1@example.com" {
		t.Fatalf("owner = %#v, want real workspace metadata", object["owner"])
	}
	if object["dashboardUrl"] != dashboardURL {
		t.Fatalf("dashboardUrl = %#v, want %q", object["dashboardUrl"], dashboardURL)
	}
	if object["updatedAt"] != "2026-07-15T12:05:00Z" || object["updatedAt"] == object["createdAt"] {
		t.Fatalf("timestamps created=%#v updated=%#v", object["createdAt"], object["updatedAt"])
	}
	if service {
		details, _ := object["serviceDetails"].(map[string]any)
		if details["region"] != "fsn1" {
			t.Fatalf("serviceDetails.region = %#v, want fsn1", details["region"])
		}
	} else if object["region"] != "fsn1" {
		t.Fatalf("region = %#v, want fsn1", object["region"])
	}
}

func TestResourceRESTMetadataUsesOwnWorkspaceAndAuthoritativeSources(t *testing.T) {
	st := newFakeWSStore()
	_ = mustCreate(t, st, "default-workspace", store.PlanHobby, "client-1")
	target := mustCreate(t, st, "target-workspace", store.PlanHobby, "client-1")
	other := mustCreate(t, st, "other-workspace", store.PlanHobby, "client-2")
	app, db, kv := metadataResources(target.ID)
	foreign, _, _ := metadataResources(other.ID)
	foreign.Name = "foreign-service"
	foreign.Labels[core.LabelServiceName] = foreign.Name
	foreign.Labels[core.LabelAppID] = "srv-foreign000000000000"

	base := serverBase(t, st)
	base.Client = fakeClient(app, db, kv, foreign)
	base.Clock = func() time.Time { return time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC) }
	h, _ := serverWith(t, base, Deps{
		WorkspaceStore: st,
		Identities:     metadataIdentities{},
		Region:         "fsn1",
		DashboardURL:   "https://dashboard.bex.co",
	})

	tests := []struct {
		path, envelope, dashboard string
		service                   bool
	}{
		// Render-shaped dashboard routes (docs/render-artifacts/dashboard-routes.md):
		// type-aware service segment, /d/ for Postgres, /r/ for Key Value.
		{"/v1/services?ownerId=" + target.ID, "service", "https://dashboard.bex.co/web/srv-metadata00000000000", true},
		{"/v1/postgres?ownerId=" + target.ID, "postgres", "https://dashboard.bex.co/d/pg-metadata", false},
		{"/v1/key-value?ownerId=" + target.ID, "keyValue", "https://dashboard.bex.co/r/kv-metadata", false},
	}
	for _, tt := range tests {
		rec := do(t, h, http.MethodGet, tt.path, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", tt.path, rec.Code, rec.Body.String())
		}
		var list []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
			t.Fatalf("GET %s list = %#v, err %v", tt.path, list, err)
		}
		object, _ := list[0][tt.envelope].(map[string]any)
		assertResourceMetadata(t, object, target.Name, tt.dashboard, tt.service)
	}

	// A caller cannot use the resource endpoint or owner enrichment to discover
	// a workspace they do not belong to.
	rec := do(t, h, http.MethodGet, "/v1/services/srv-foreign000000000000", testToken, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-workspace GET = %d: %s", rec.Code, rec.Body.String())
	}

	// Each supported mutation advances updatedAt; a subsequent read is stable.
	mutations := []struct {
		path, body string
	}{
		{"/v1/services/srv-metadata00000000000", `{"serviceDetails":{"plan":"standard"}}`},
		{"/v1/postgres/pg-metadata", `{"plan":"basic-1gb"}`},
		{"/v1/key-value/kv-metadata", `{"plan":"standard"}`},
	}
	for _, mutation := range mutations {
		rec = do(t, h, http.MethodPatch, mutation.path, testToken, mutation.body)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s = %d: %s", mutation.path, rec.Code, rec.Body.String())
		}
		updated := decodeObject(t, rec.Body.Bytes())["updatedAt"]
		if updated != "2026-07-15T13:00:00Z" {
			t.Fatalf("PATCH %s updatedAt = %#v", mutation.path, updated)
		}
		rec = do(t, h, http.MethodGet, mutation.path, testToken, "")
		if rec.Code != http.StatusOK || decodeObject(t, rec.Body.Bytes())["updatedAt"] != updated {
			t.Fatalf("read after PATCH %s changed timestamp: %d %s", mutation.path, rec.Code, rec.Body.String())
		}
	}
}

func TestResourceRESTMetadataOmitsUnavailableConfigurationAndLegacyOwner(t *testing.T) {
	st := newFakeWSStore()
	owner := mustCreate(t, st, "workspace", store.PlanHobby, "client-1")
	app, db, kv := metadataResources(owner.ID)
	legacy := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: "default", Labels: map[string]string{core.LabelAppID: "srv-legacy0000000000000"}},
		Spec:       appv1alpha1.AppSpec{Image: "nginx"},
	}
	legacyDB := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "legacy-db", Namespace: "default"}}
	legacyKV := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "legacy-kv", Namespace: "default"}}
	base := serverBase(t, st)
	base.Client = fakeClient(app, db, kv, legacy)
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st})

	configured := []struct {
		path    string
		service bool
	}{
		{"/v1/services/srv-metadata00000000000", true},
		{"/v1/postgres/pg-metadata", false},
		{"/v1/key-value/kv-metadata", false},
	}
	for _, tt := range configured {
		rec := do(t, h, http.MethodGet, tt.path, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("configured-owner GET %s = %d: %s", tt.path, rec.Code, rec.Body.String())
		}
		object := decodeObject(t, rec.Body.Bytes())
		if _, ok := object["owner"]; !ok {
			t.Fatalf("available owner should be present: %s", rec.Body.String())
		}
		if _, ok := object["dashboardUrl"]; ok {
			t.Fatalf("dashboardUrl should be omitted: %s", rec.Body.String())
		}
		if tt.service {
			details, _ := object["serviceDetails"].(map[string]any)
			if _, ok := details["region"]; ok {
				t.Fatalf("region should be omitted: %s", rec.Body.String())
			}
		} else if _, ok := object["region"]; ok {
			t.Fatalf("region should be omitted: %s", rec.Body.String())
		}
	}

	// Store-off mode preserves the historical hand-applied-resource read path;
	// it has no workspace resolver, so owner metadata is honestly unavailable.
	storeOffBase := &core.Base{Client: fakeClient(legacy.DeepCopy(), legacyDB, legacyKV), Namespace: "default"}
	storeOff, _ := serverWith(t, storeOffBase, Deps{})
	legacyPaths := []string{
		"/v1/services/srv-legacy0000000000000",
		"/v1/postgres/legacy-db",
		"/v1/key-value/legacy-kv",
	}
	for _, path := range legacyPaths {
		rec := do(t, storeOff, http.MethodGet, path, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("legacy GET %s = %d: %s", path, rec.Code, rec.Body.String())
		}
		legacyObject := decodeObject(t, rec.Body.Bytes())
		if _, ok := legacyObject["owner"]; ok {
			t.Fatalf("legacy owner should be omitted: %s", rec.Body.String())
		}
		if _, ok := legacyObject["updatedAt"]; ok {
			t.Fatalf("legacy updatedAt should be omitted: %s", rec.Body.String())
		}
	}
}

func TestResourceRESTOwnerTracksWorkspaceRenameAndOmitsDeletedWorkspace(t *testing.T) {
	st := newFakeWSStore()
	owner := mustCreate(t, st, "before-rename", store.PlanHobby, "client-1")
	app, _, _ := metadataResources(owner.ID)
	base := serverBase(t, st)
	base.Client = fakeClient(app)
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st})

	st.mu.Lock()
	st.tenants[0].Name = "after-rename"
	st.mu.Unlock()
	rec := do(t, h, http.MethodGet, "/v1/services/srv-metadata00000000000", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("renamed owner GET = %d: %s", rec.Code, rec.Body.String())
	}
	object := decodeObject(t, rec.Body.Bytes())
	if object["owner"].(map[string]any)["name"] != "after-rename" {
		t.Fatalf("owner did not follow rename: %s", rec.Body.String())
	}

	st.mu.Lock()
	st.tenants = nil
	st.mu.Unlock()
	rec = do(t, h, http.MethodGet, "/v1/services/srv-metadata00000000000", testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("deleted owner GET = %d: %s", rec.Code, rec.Body.String())
	}
	object = decodeObject(t, rec.Body.Bytes())
	if _, ok := object["owner"]; ok {
		t.Fatalf("deleted owner should be omitted: %s", rec.Body.String())
	}
}
