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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestProductCreationOnlyAfterSuccessfulMaterialization(t *testing.T) {
	s := danaService()
	var events []core.ProductActivity
	s.ProductActivity = func(_ context.Context, e core.ProductActivity) error { events = append(events, e); return nil }
	ctx := ctxAs("dana")
	request := CreateRequest{Name: "site", Type: "static_site", Repo: "https://example.test/repo.git", PublishPath: "public", OwnerID: "tea-2"}
	dry := request
	dry.DryRun = true
	if _, err := s.Create(ctx, dry); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatal("dry run recorded creation")
	}
	// Use image-backed web service for a real materialization without a git provider.
	request = CreateRequest{Name: "web", Image: "nginx", OwnerID: "tea-2"}
	if _, err := s.Create(ctx, request); err != nil {
		t.Fatal(err)
	}
	app := createdApp(t, s, "web")
	if len(events) != 1 || events[0].ResourceID != app.Labels[core.LabelAppID] || events[0].WorkspaceID != "tea-2" || events[0].ResourceType != "web_service" {
		t.Fatalf("events: %+v", events)
	}
	if _, err := s.Create(ctx, request); err == nil {
		t.Fatal("duplicate create succeeded")
	}
	if len(events) != 1 {
		t.Fatal("failed duplicate recorded creation")
	}
	s.Client = createErrorClient{Client: s.Client, err: errors.New("CR rejected")}
	request.Name = "rejected"
	if _, err := s.Create(ctx, request); err == nil {
		t.Fatal("expected materialization failure")
	}
	if len(events) != 1 {
		t.Fatal("failed Kubernetes write recorded creation")
	}
}

func TestProductStaticSiteCreation(t *testing.T) {
	s := danaService()
	var events []core.ProductActivity
	s.ProductActivity = func(_ context.Context, e core.ProductActivity) error { events = append(events, e); return nil }
	if _, err := s.Create(ctxAs("dana"), CreateRequest{
		Name: "site", Type: "static_site", Repo: "https://github.com/acme/site", PublishPath: "public", OwnerID: "tea-2",
	}); err != nil {
		t.Fatal(err)
	}
	app := createdApp(t, s, "site")
	if len(events) != 1 || events[0].ResourceType != "static_site" || events[0].ResourceID != app.Labels[core.LabelAppID] {
		t.Fatalf("static creation: %+v", events)
	}
}

// TestProductBlueprintDatastoreCreation records postgres/keyvalue product-
// activity on Blueprint create (w5/056) and not again on idempotent re-apply.
func TestProductBlueprintDatastoreCreation(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"dana": "tea-2"})
	var events []core.ProductActivity
	svc.ProductActivity = func(_ context.Context, e core.ProductActivity) error {
		events = append(events, e)
		return nil
	}
	ctx := ctxAs("dana")
	manifest := `
databases:
  - name: db
    plan: basic-256mb
    diskSizeGB: 5
    postgresMajorVersion: "16"
services:
  - name: cache
    type: redis
    plan: free
    ipAllowList: []
`
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-2", Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.Databases) != 1 || len(res.KeyValues) != 1 {
		t.Fatalf("result databases=%d keyValues=%d", len(res.Databases), len(res.KeyValues))
	}
	var postgres, keyvalue int
	for _, e := range events {
		switch {
		case e.ResourceType == "postgres" && e.EventType == "created" && e.ResourceID == res.Databases[0].ID && e.WorkspaceID == "tea-2":
			postgres++
		case e.ResourceType == "keyvalue" && e.EventType == "created" && e.ResourceID == res.KeyValues[0].ID && e.WorkspaceID == "tea-2":
			keyvalue++
		}
	}
	if postgres != 1 || keyvalue != 1 {
		t.Fatalf("create events postgres=%d keyvalue=%d all=%+v", postgres, keyvalue, events)
	}
	n := len(events)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-2", Manifest: manifest}); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if len(events) != n {
		t.Fatalf("re-apply recorded %d new events: %+v", len(events)-n, events[n:])
	}
}

type productRollbackStore struct {
	recordingStore
	rolledBack []string
}

func (s *productRollbackStore) RollbackAppCreation(_ context.Context, appID string) error {
	s.rolledBack = append(s.rolledBack, appID)
	return nil
}

func TestProductMaterializationUsesAnalyticsAwareRollback(t *testing.T) {
	s := danaService()
	if err := s.EnsureWorkspaceNamespace(ctxAs("dana"), "tea-2"); err != nil {
		t.Fatal(err)
	}
	st := &productRollbackStore{}
	s.Store = st
	s.Client = createErrorClient{Client: s.Client, err: errors.New("CR rejected")}
	if _, err := s.Create(ctxAs("dana"), CreateRequest{Name: "rejected", Image: "nginx", OwnerID: "tea-2"}); err == nil {
		t.Fatal("expected materialization failure")
	}
	if len(st.appCreates) != 1 || len(st.rolledBack) != 1 || st.rolledBack[0] != st.appCreates[0].ID || len(st.deleteCalls) != 0 {
		t.Fatalf("creates=%+v rollbacks=%v ordinary deletes=%v", st.appCreates, st.rolledBack, st.deleteCalls)
	}
}
