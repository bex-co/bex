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

// blueprint_grouping_test.go covers the w8/m20 grouping hardening: quota
// refusal, audit emission, idempotent re-sync (no ACL churn), and the
// disconnect reclaim flow. The transactional rollback itself is proven
// against real Postgres in internal/store.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type groupingAuditSink struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (s *groupingAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	return nil
}

func (s *groupingAuditSink) groupingEvents() []core.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []core.AuditEvent
	for _, ev := range s.events {
		if strings.HasPrefix(ev.Verb, "apps.BlueprintGrouping.") {
			out = append(out, ev)
		}
	}
	return out
}

const groupedManifest = `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        networking:
          isolation: enabled
        permissions:
          protection: enabled
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
`

func TestBlueprintGroupingQuotaRefusesWithCodedError(t *testing.T) {
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	svc.MaxGroupings = 1
	// The workspace already holds one project — creating the manifest's new
	// project would exceed the cap.
	st.projects = append(st.projects, store.Project{ID: "prj-existing", TenantID: "tea-a", Name: "held"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedManifest})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_GROUPING_LIMIT" {
		t.Fatalf("over-quota deploy error = %v, want BLUEPRINT_GROUPING_LIMIT", err)
	}
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("quota refusal must be conflict-class, got %v", err)
	}

	// 0 disables the cap.
	svc.MaxGroupings = 0
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedManifest}); err != nil {
		t.Fatalf("uncapped deploy: %v", err)
	}
}

func TestBlueprintGroupingAuditAndIdempotentResync(t *testing.T) {
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	sink := &groupingAuditSink{}
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedManifest}); err != nil {
		t.Fatal(err)
	}
	first := sink.groupingEvents()
	if len(first) != 2 {
		t.Fatalf("first sync grouping audit events = %+v, want project_created + environment_created", first)
	}
	if first[0].Verb != "apps.BlueprintGrouping.project_created" || first[1].Verb != "apps.BlueprintGrouping.environment_created" {
		t.Fatalf("audit verbs = %q, %q", first[0].Verb, first[1].Verb)
	}
	if first[0].Caller != "id-a" || first[0].Resource != core.WorkspaceObject("tea-a") {
		t.Fatalf("audit attribution = %+v", first[0])
	}
	aclAfterFirst := st.aclWrites

	// Unchanged re-sync: no new grouping rows, no ACL churn, no new audit rows.
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: groupedManifest}); err != nil {
		t.Fatal(err)
	}
	if len(st.projects) != 1 || len(st.environments) != 1 {
		t.Fatalf("re-sync duplicated groupings: %d projects, %d environments", len(st.projects), len(st.environments))
	}
	if st.aclWrites != aclAfterFirst {
		t.Fatalf("idempotent re-sync rewrote the environment ACL (%d → %d writes)", aclAfterFirst, st.aclWrites)
	}
	if again := sink.groupingEvents(); len(again) != len(first) {
		t.Fatalf("idempotent re-sync emitted spurious audit events: %+v", again[len(first):])
	}
}

type fakeGroupingReclaimer struct {
	pairs       []store.GroupingPair
	removedEnvs []string
	removedPrj  []string
}

func (f *fakeGroupingReclaimer) ReclaimEmptyBlueprintGroupings(_ context.Context, _ string, pairs []store.GroupingPair, _, _ map[string]bool) ([]string, []string, error) {
	f.pairs = pairs
	return f.removedEnvs, f.removedPrj, nil
}

func TestDisconnectBlueprintSweepsMintedGroupings(t *testing.T) {
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: groupedManifest,
		Status: "active", Name: "app",
	})
	reclaimer := &fakeGroupingReclaimer{removedEnvs: []string{"platform/production"}, removedPrj: []string{"platform"}}
	sink := &groupingAuditSink{}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}, Audit: sink}, Blueprints: fs, GroupingReclaim: reclaimer}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	if err := svc.DisconnectBlueprint(ctx, "blp-1", "tea-a"); err != nil {
		t.Fatal(err)
	}
	if len(reclaimer.pairs) != 1 || reclaimer.pairs[0] != (store.GroupingPair{Project: "platform", Environment: "production"}) {
		t.Fatalf("reclaim pairs = %+v", reclaimer.pairs)
	}
	events := sink.groupingEvents()
	if len(events) != 2 || events[0].Verb != "apps.BlueprintGrouping.environment_reclaimed" || events[1].Verb != "apps.BlueprintGrouping.project_reclaimed" {
		t.Fatalf("reclaim audit events = %+v", events)
	}
	// w8/m37 t001: the disconnected row reads as absent on ordinary lookups,
	// while the stored row itself stays disconnected (durable, not deleted).
	if _, err := fs.GetBlueprint(context.Background(), "blp-1", "tea-a"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("disconnected blueprint read = %v, want not-found absence", err)
	}
	if b := fs.blueprints["blp-1"]; b.Status != "disconnected" || b.AutoSync {
		t.Fatalf("stored blueprint must stay disconnected, got %+v", b)
	}
}
