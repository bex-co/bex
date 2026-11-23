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

// conformance_resources_test.go — composed-server Render response-conformance
// coverage for resource operation families (w5/m581 t002): build sources,
// disks & disk snapshots, blueprints, webhooks, registry credentials,
// environment groups, projects & environments, and core service / deploy /
// custom-domain / Postgres / Key Value writes and reads. Every request runs
// through buildHandler's rendertest.Wrap so both status codes (including
// 204-no-body) and response bodies are validated against the pinned OpenAPI
// contract without growing rendertest.Allowlist.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func conformDo(t *testing.T, h http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != wantStatus {
		t.Fatalf("%s %s = %d (want %d): %s", method, path, w.Code, wantStatus, w.Body.String())
	}
	return w
}

func fakeConformKratos(t *testing.T, subject, email string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sess-1","active":true,"identity":{"id":"` + subject + `","traits":{"email":"` + email + `","name":"Alice"}}}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func conformDoSession(t *testing.T, h http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: "ory_kratos_session", Value: "conform-sess"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != wantStatus {
		t.Fatalf("%s %s = %d (want %d): %s", method, path, w.Code, wantStatus, w.Body.String())
	}
	return w
}

func proPlan() core.WorkspacePlanFunc {
	return core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return store.PlanPro, nil })
}

// --- in-memory conformance stores -------------------------------------------

type conformDiskStore struct {
	apps.IntentStore
	mu    sync.Mutex
	disks map[string]store.Disk
}

func newConformDiskStore() *conformDiskStore {
	return &conformDiskStore{disks: map[string]store.Disk{}}
}

func (s *conformDiskStore) CreateDisk(_ context.Context, tenantID, appID, name, mountPath string, sizeGB int32) (store.Disk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := store.Disk{
		ID: ids.New(ids.Disk), TenantID: tenantID, AppID: appID,
		Name: name, MountPath: mountPath, SizeGB: sizeGB,
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	s.disks[d.ID] = d
	return d, nil
}

func (s *conformDiskStore) GetDisk(_ context.Context, id string) (store.Disk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.disks[id]
	if !ok {
		return store.Disk{}, store.ErrNotFound
	}
	return d, nil
}

func (s *conformDiskStore) ListDisks(_ context.Context, tenantID, appID string) ([]store.Disk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Disk
	for _, d := range s.disks {
		if (tenantID == "" || d.TenantID == tenantID) && (appID == "" || d.AppID == appID) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *conformDiskStore) UpdateDisk(_ context.Context, id string, name, mountPath *string, sizeGB *int32) (store.Disk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.disks[id]
	if !ok {
		return store.Disk{}, store.ErrNotFound
	}
	if name != nil {
		d.Name = *name
	}
	if mountPath != nil {
		d.MountPath = *mountPath
	}
	if sizeGB != nil {
		d.SizeGB = *sizeGB
	}
	d.UpdatedAt = conformEpoch.Add(time.Minute)
	s.disks[id] = d
	return d, nil
}

func (s *conformDiskStore) DeleteDisk(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.disks[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.disks, id)
	return nil
}

func (s *conformDiskStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, _ string, _ ...bool) (store.Deploy, error) {
	return store.Deploy{
		ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger, Image: image,
		Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message,
		Status: store.DeployCreated, CreatedAt: conformEpoch,
	}, nil
}
func (s *conformDiskStore) LatestDeployCommit(context.Context, string) (store.CommitInfo, error) {
	return store.CommitInfo{}, nil
}
func (s *conformDiskStore) SetAppSuspended(context.Context, string, bool) error { return nil }
func (s *conformDiskStore) SetAppReplicas(context.Context, string, int32) (bool, error) {
	return true, nil
}
func (s *conformDiskStore) AddDomain(context.Context, string, string, string) error { return nil }
func (s *conformDiskStore) RemoveDomain(context.Context, string, string) error      { return nil }
func (s *conformDiskStore) DomainHostsClaimed(context.Context, []string) ([]string, error) {
	return nil, nil
}

type conformSnapshotLister struct {
	objs []apps.DiskSnapshotObject
}

func (l conformSnapshotLister) ListSnapshots(context.Context, string) ([]apps.DiskSnapshotObject, error) {
	return l.objs, nil
}

type conformBlueprintStore struct {
	apps.BlueprintStore
	mu    sync.Mutex
	bps   map[string]store.Blueprint
	syncs map[string][]store.BlueprintSync
}

func newConformBlueprintStore(ws string) (*conformBlueprintStore, string) {
	bpID := ids.New(ids.Blueprint)
	syncID := ids.New(ids.BlueprintSync)
	done := conformEpoch.Add(time.Minute)
	bp := store.Blueprint{
		ID: bpID, TenantID: ws, Name: "stack",
		Repo: "https://github.com/bex-co/hello-go", Branch: "main", Path: "render.yaml",
		AutoSync: true, Status: "in_sync", LastSyncAt: &done,
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	run := store.BlueprintSync{
		ID: syncID, BlueprintID: bpID, CommitID: "0123456789abcdef0123456789abcdef01234567",
		State: "success", StartedAt: conformEpoch, CompletedAt: &done, CreatedAt: conformEpoch,
	}
	return &conformBlueprintStore{
		bps:   map[string]store.Blueprint{bpID: bp},
		syncs: map[string][]store.BlueprintSync{bpID: {run}},
	}, bpID
}

func (s *conformBlueprintStore) ListBlueprints(_ context.Context, tenantID string) ([]store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Blueprint
	for _, b := range s.bps {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *conformBlueprintStore) GetBlueprint(_ context.Context, id, tenantID string) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.ErrNotFound
	}
	return b, nil
}

func (s *conformBlueprintStore) BlueprintWorkspace(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return b.TenantID, nil
}

func (s *conformBlueprintStore) BlueprintNameTaken(_ context.Context, tenantID, name, exceptID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.bps {
		if b.TenantID == tenantID && b.ID != exceptID && strings.EqualFold(b.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (s *conformBlueprintStore) UpdateBlueprint(_ context.Context, id, tenantID string, name *string, autoSync *bool, path, branch *string, status *string, lastSyncAt *time.Time) (store.Blueprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, store.ErrNotFound
	}
	if name != nil {
		b.Name = *name
	}
	if autoSync != nil {
		b.AutoSync = *autoSync
	}
	if path != nil {
		b.Path = *path
	}
	if branch != nil {
		b.Branch = *branch
	}
	if status != nil {
		b.Status = *status
	}
	if lastSyncAt != nil {
		b.LastSyncAt = lastSyncAt
	}
	b.UpdatedAt = conformEpoch.Add(time.Minute)
	s.bps[id] = b
	return b, nil
}

func (s *conformBlueprintStore) DisconnectBlueprint(_ context.Context, id, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bps[id]
	if !ok || b.TenantID != tenantID {
		return store.ErrNotFound
	}
	delete(s.bps, id)
	return nil
}

func (s *conformBlueprintStore) ReleaseBlueprintResourceClaims(context.Context, string, string) error {
	return nil
}

func (s *conformBlueprintStore) ListBlueprintSyncs(_ context.Context, blueprintID, _ string, _ int) ([]store.BlueprintSync, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.syncs[blueprintID]), nil
}

type conformWebhookStore struct {
	webhooks.EndpointStore
	mu        sync.Mutex
	endpoints map[string]store.WebhookEndpoint
	attempts  map[string][]store.WebhookAttempt
}

func newConformWebhookStore() *conformWebhookStore {
	return &conformWebhookStore{
		endpoints: map[string]store.WebhookEndpoint{},
		attempts:  map[string][]store.WebhookAttempt{},
	}
}

func (s *conformWebhookStore) CreateWebhookEndpoint(_ context.Context, tenantID, name, url, secret string, eventTypes []string, enabled bool, createdBy string) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep := store.WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url,
		Secret: secret, EventTypes: slices.Clone(eventTypes), Enabled: enabled,
		CreatedBy: createdBy, CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	s.endpoints[ep.ID] = ep
	sentAt := conformEpoch.Add(time.Second)
	s.attempts[ep.ID] = []store.WebhookAttempt{{
		ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID,
		EventID: ids.New(ids.Event), EventType: webhooks.TypeDeployEnded,
		ServiceID: ids.New(ids.Service), Status: store.WebhookAttemptDelivered,
		AttemptNumber: 1, StatusCode: 200, SentAt: &sentAt, CreatedAt: conformEpoch,
	}}
	return ep, nil
}

func (s *conformWebhookStore) ListWebhookEndpoints(_ context.Context, tenantIDs []string, _ time.Time, _ string, _ int) ([]store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.WebhookEndpoint
	for _, ep := range s.endpoints {
		if len(tenantIDs) == 0 || slices.Contains(tenantIDs, ep.TenantID) {
			out = append(out, ep)
		}
	}
	return out, nil
}

func (s *conformWebhookStore) GetWebhookEndpoint(_ context.Context, tenantID, id string) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok || (tenantID != "" && ep.TenantID != tenantID) {
		return store.WebhookEndpoint{}, store.ErrNotFound
	}
	return ep, nil
}

func (s *conformWebhookStore) WebhookEndpointWorkspace(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return ep.TenantID, nil
}

func (s *conformWebhookStore) UpdateWebhookEndpoint(_ context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok || ep.TenantID != tenantID {
		return store.WebhookEndpoint{}, store.ErrNotFound
	}
	ep.Name, ep.URL, ep.EventTypes, ep.Enabled = name, url, slices.Clone(eventTypes), enabled
	ep.UpdatedAt = conformEpoch.Add(time.Minute)
	s.endpoints[id] = ep
	return ep, nil
}

func (s *conformWebhookStore) DeleteWebhookEndpoint(_ context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.endpoints[id]
	if !ok || ep.TenantID != tenantID {
		return store.ErrNotFound
	}
	delete(s.endpoints, id)
	return nil
}

func (s *conformWebhookStore) ListWebhookAttempts(_ context.Context, filter store.WebhookAttemptFilter) ([]store.WebhookAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.attempts[filter.EndpointID]), nil
}

type conformEnvGroupStore struct {
	envgroups.GroupStore
	mu     sync.Mutex
	groups map[string]store.EnvGroup
}

func newConformEnvGroupStore() *conformEnvGroupStore {
	return &conformEnvGroupStore{groups: map[string]store.EnvGroup{}}
}

func (s *conformEnvGroupStore) WithEnvGroupContentLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (s *conformEnvGroupStore) InsertEnvGroup(_ context.Context, g store.EnvGroup) (store.EnvGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = conformEpoch
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = conformEpoch
	}
	if g.Content.State == "" {
		g.Content.State = store.EnvGroupIdle
	}
	s.groups[g.ID] = g
	return g, nil
}

func (s *conformEnvGroupStore) GetEnvGroup(_ context.Context, id string) (store.EnvGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return store.EnvGroup{}, store.ErrNotFound
	}
	return g, nil
}

func (s *conformEnvGroupStore) ListEnvGroups(_ context.Context, tenantID string) ([]store.EnvGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.EnvGroup
	for _, g := range s.groups {
		if g.TenantID == tenantID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *conformEnvGroupStore) CountEnvGroups(_ context.Context, tenantID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, g := range s.groups {
		if g.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (s *conformEnvGroupStore) EnvGroupNameTaken(_ context.Context, tenantID, name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.groups {
		if g.TenantID == tenantID && g.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (s *conformEnvGroupStore) UpdateEnvGroupMeta(_ context.Context, g store.EnvGroup) (store.EnvGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[g.ID]; !ok {
		return store.EnvGroup{}, store.ErrNotFound
	}
	g.MetaVersion++
	g.UpdatedAt = conformEpoch.Add(time.Minute)
	s.groups[g.ID] = g
	return g, nil
}

func (s *conformEnvGroupStore) DeleteEnvGroup(_ context.Context, id string) (store.EnvGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return store.EnvGroup{}, store.ErrNotFound
	}
	delete(s.groups, id)
	return g, nil
}

func (s *conformEnvGroupStore) ClaimEnvGroupContent(_ context.Context, id string, expectedVersion int64, _ store.EnvGroupOp) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return 0, store.ErrNotFound
	}
	g.Content.Version = expectedVersion + 1
	s.groups[id] = g
	return g.Content.Version, nil
}

func (s *conformEnvGroupStore) AdvanceEnvGroupOp(context.Context, string, string, string, time.Time) error {
	return nil
}

func (s *conformEnvGroupStore) ReleaseEnvGroupContent(_ context.Context, id string, expectedVersion int64, rel store.EnvGroupRelease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return store.ErrNotFound
	}
	g.Content.Version = expectedVersion
	if rel.Commit != nil {
		if rel.Commit.EnvKeys != nil {
			g.EnvKeys = slices.Clone(rel.Commit.EnvKeys)
		}
		if rel.Commit.FileNames != nil {
			g.FileNames = slices.Clone(rel.Commit.FileNames)
		}
	}
	g.UpdatedAt = conformEpoch.Add(time.Minute)
	s.groups[id] = g
	return nil
}

type conformProjectEnvStore struct {
	projects.ProjectStore
	environments.EnvironmentStore
	mu       sync.Mutex
	projects map[string]store.Project
	envs     map[string]store.Environment
	envSvcs  map[string][]string
}

func newConformProjectEnvStore() *conformProjectEnvStore {
	return &conformProjectEnvStore{
		projects: map[string]store.Project{},
		envs:     map[string]store.Environment{},
		envSvcs:  map[string][]string{},
	}
}

func (s *conformProjectEnvStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := store.Project{ID: ids.New(ids.Project), TenantID: tenantID, Name: name, CreatedAt: conformEpoch, UpdatedAt: conformEpoch}
	s.projects[p.ID] = p
	return p, nil
}

func (s *conformProjectEnvStore) GetProject(_ context.Context, id string) (store.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok {
		return store.Project{}, store.ErrNotFound
	}
	return p, nil
}

func (s *conformProjectEnvStore) ListProjects(_ context.Context, tenantID string) ([]store.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Project
	for _, p := range s.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *conformProjectEnvStore) RenameProject(_ context.Context, id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.projects[id]
	if !ok {
		return store.ErrNotFound
	}
	p.Name = name
	p.UpdatedAt = conformEpoch.Add(time.Minute)
	s.projects[id] = p
	return nil
}

func (s *conformProjectEnvStore) DeleteProject(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.projects, id)
	return nil
}

func (s *conformProjectEnvStore) ListServicesForProjects(context.Context, []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (s *conformProjectEnvStore) CreateEnvironment(_ context.Context, projectID, tenantID, name string) (store.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := store.Environment{
		ID: ids.EnvironmentStorageID(ids.New(ids.Environment)), ProjectID: projectID, TenantID: tenantID,
		Name: name, ProtectedStatus: core.ProtectedStatusUnprotected,
		CreatedAt: conformEpoch,
	}
	s.envs[e.ID] = e
	return e, nil
}

func (s *conformProjectEnvStore) GetEnvironment(_ context.Context, id string) (store.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.envs[ids.EnvironmentStorageID(id)]
	if !ok {
		return store.Environment{}, store.ErrNotFound
	}
	return e, nil
}

func (s *conformProjectEnvStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Environment
	for _, e := range s.envs {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *conformProjectEnvStore) ListWorkspaceEnvironments(_ context.Context, tenantID string) ([]store.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Environment
	for _, e := range s.envs {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *conformProjectEnvStore) RenameEnvironment(_ context.Context, id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ids.EnvironmentStorageID(id)
	e, ok := s.envs[key]
	if !ok {
		return store.ErrNotFound
	}
	e.Name = name
	s.envs[key] = e
	return nil
}

func (s *conformProjectEnvStore) DeleteEnvironment(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ids.EnvironmentStorageID(id)
	if _, ok := s.envs[key]; !ok {
		return store.ErrNotFound
	}
	delete(s.envs, key)
	delete(s.envSvcs, key)
	return nil
}

func (s *conformProjectEnvStore) ListEnvironmentServices(_ context.Context, environmentID, _ string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.envSvcs[ids.EnvironmentStorageID(environmentID)]), nil
}

func (s *conformProjectEnvStore) ListWorkspaceEnvironmentServices(_ context.Context, _ string) (map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]string, len(s.envSvcs))
	for k, v := range s.envSvcs {
		out[k] = slices.Clone(v)
	}
	return out, nil
}

func (s *conformProjectEnvStore) ChangeEnvironmentServices(_ context.Context, environmentID, projectID, _ string, serviceIDs []string, add bool, _ core.EnvironmentExitGuard) ([]core.ServicePlacementChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ids.EnvironmentStorageID(environmentID)
	cur := slices.Clone(s.envSvcs[key])
	var changes []core.ServicePlacementChange
	if add {
		for _, sid := range serviceIDs {
			if !slices.Contains(cur, sid) {
				cur = append(cur, sid)
				toEnv, toPrj := environmentID, projectID
				changes = append(changes, core.ServicePlacementChange{ServiceID: sid, ServiceMove: core.ServiceMove{EnvironmentTo: &toEnv, ProjectTo: &toPrj}})
			}
		}
	} else {
		cur = slices.DeleteFunc(cur, func(sid string) bool { return slices.Contains(serviceIDs, sid) })
		for _, sid := range serviceIDs {
			fromEnv, fromPrj := environmentID, projectID
			changes = append(changes, core.ServicePlacementChange{ServiceID: sid, ServiceMove: core.ServiceMove{EnvironmentFrom: &fromEnv, ProjectFrom: &fromPrj}})
		}
	}
	s.envSvcs[key] = cur
	return changes, nil
}

type conformMutableDeployStore struct {
	deploys.DeployStore
	mu    sync.Mutex
	byApp map[string][]store.Deploy
}

func newConformMutableDeployStore() *conformMutableDeployStore {
	return &conformMutableDeployStore{byApp: map[string][]store.Deploy{}}
}

func (s *conformMutableDeployStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, _ string, _ ...bool) (store.Deploy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := store.Deploy{
		ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger, Image: image,
		Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message,
		Status: store.DeployBuildInProgress, CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	s.byApp[appID] = append([]store.Deploy{d}, s.byApp[appID]...)
	return d, nil
}

func (s *conformMutableDeployStore) CreateRollbackDeploy(_ context.Context, appID, _, image string, generation int64, commit store.CommitInfo, _ string) (store.Deploy, error) {
	return s.CreateDeploy(context.Background(), appID, store.TriggerRollback, image, generation, commit, "")
}

func (s *conformMutableDeployStore) LatestDeployCommit(context.Context, string) (store.CommitInfo, error) {
	return store.CommitInfo{Hash: "0123456789abcdef0123456789abcdef01234567", Message: "init"}, nil
}

func (s *conformMutableDeployStore) ListDeploys(_ context.Context, appID string, _ store.DeployFilter) ([]store.Deploy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.byApp[appID]), nil
}

func (s *conformMutableDeployStore) GetDeploy(_ context.Context, appID, deployID string) (store.Deploy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.byApp[appID] {
		if d.ID == deployID {
			return d, nil
		}
	}
	return store.Deploy{}, core.ErrNotFound
}

func (s *conformMutableDeployStore) CloseDeploy(_ context.Context, deployID, status, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for appID := range s.byApp {
		for i := range s.byApp[appID] {
			if s.byApp[appID][i].ID == deployID {
				s.byApp[appID][i].Status = status
				fin := conformEpoch.Add(time.Minute)
				s.byApp[appID][i].FinishedAt = &fin
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *conformMutableDeployStore) SetAppImage(context.Context, string, string) error { return nil }

// --- Conformance test suites ------------------------------------------------

func TestRenderConformanceBuildSources(t *testing.T) {
	const ws = "tea-conform"
	depStore := newConformMutableDeployStore()
	evgStore := newConformEnvGroupStore()
	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{
			Secrets:        newMemSecretStore(),
			EnvGroupsStore: evgStore,
			DeployStore:    depStore,
			Region:         "fsn1",
			DashboardURL:   "https://dashboard.bex.co",
		},
	)

	// Create build source (201), list (200), get (200), update (200).
	w := conformDo(t, h, http.MethodPost, "/v1/build-sources",
		`{"name":"mono","ownerId":"`+ws+`","git":{"runtime":"node","repoUrl":"https://github.com/bex-co/mono","branch":"main","buildCommand":"npm run build","region":"frankfurt"}}`,
		http.StatusCreated)
	var bs struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &bs); err != nil || bs.ID == "" {
		t.Fatalf("create-build-source body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/build-sources", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/build-sources/"+bs.ID, `{"name":"mono-v2","git":{"buildCommand":"npm ci && npm run build"}}`, http.StatusOK)

	// Build source builds: trigger (201), list (200), retrieve (200).
	w = conformDo(t, h, http.MethodPost, "/v1/build-sources/"+bs.ID+"/builds", `{"clearCache":"do_not_clear"}`, http.StatusCreated)
	var build struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &build); err != nil || build.ID == "" {
		t.Fatalf("trigger-build-source-build body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/builds", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/builds/"+build.ID, "", http.StatusOK)

	// Build source env-vars: replace-all (200), list (200), upsert-one (200), get-one (200), delete-one (204).
	conformDo(t, h, http.MethodPut, "/v1/build-sources/"+bs.ID+"/env-vars", `[{"key":"NODE_ENV","value":"production"}]`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/env-vars", "", http.StatusOK)
	conformDo(t, h, http.MethodPut, "/v1/build-sources/"+bs.ID+"/env-vars/LOG_LEVEL", `{"value":"info"}`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/env-vars/LOG_LEVEL", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/build-sources/"+bs.ID+"/env-vars/LOG_LEVEL", "", http.StatusNoContent)

	// Build source secret-files: replace-all (200), list (200), upsert-one (201), get-one (200), delete-one (204).
	conformDo(t, h, http.MethodPut, "/v1/build-sources/"+bs.ID+"/secret-files", `[{"name":"ca.pem","content":"cert"}]`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/secret-files", "", http.StatusOK)
	conformDo(t, h, http.MethodPut, "/v1/build-sources/"+bs.ID+"/secret-files/token.txt", `{"content":"tok"}`, http.StatusCreated)
	conformDo(t, h, http.MethodGet, "/v1/build-sources/"+bs.ID+"/secret-files/token.txt", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/build-sources/"+bs.ID+"/secret-files/token.txt", "", http.StatusNoContent)

	// Build source env-group link (200) and unlink (204).
	w = conformDo(t, h, http.MethodPost, "/v1/env-groups",
		`{"name":"shared","ownerId":"`+ws+`","envVars":[{"key":"FOO","value":"bar"}]}`, http.StatusCreated)
	var evg struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &evg); err != nil || evg.ID == "" {
		t.Fatalf("create-env-group body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodPost, "/v1/build-sources/"+bs.ID+"/env-groups/"+evg.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/build-sources/"+bs.ID+"/env-groups/"+evg.ID, "", http.StatusNoContent)

	// Delete build source (204, empty body, no Content-Type — w5/m581 t001).
	del := conformDo(t, h, http.MethodDelete, "/v1/build-sources/"+bs.ID, "", http.StatusNoContent)
	if del.Body.Len() != 0 || del.Header().Get("Content-Type") != "" {
		t.Fatalf("DELETE build-source 204 body=%q ct=%q, want both empty", del.Body.String(), del.Header().Get("Content-Type"))
	}
}

func TestRenderConformanceDisksAndSnapshots(t *testing.T) {
	app := conformApp("disk-app", ids.New(ids.Service))
	app.Spec.Tier = "starter"
	app.Spec.Replicas = 1
	app.Labels[core.LabelTenant] = core.DefaultTenant
	app.Labels[store.LabelManagedBy] = store.ManagedByValue

	diskStore := newConformDiskStore()
	snapAt := conformEpoch.Add(-time.Hour)
	lister := conformSnapshotLister{objs: []apps.DiskSnapshotObject{{
		Key:       "default/disk-app/" + snapAt.Format(time.RFC3339) + ".tar.gz.age",
		CreatedAt: snapAt,
	}}}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(app),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			Store:              diskStore,
			DiskSnapshots:      lister,
			DiskSnapshotSecret: []byte("0123456789abcdef0123456789abcdef"),
			Region:             "fsn1",
			DashboardURL:       "https://dashboard.bex.co",
		},
	)

	w := conformDo(t, h, http.MethodPost, "/v1/disks",
		`{"serviceId":"`+app.Labels[store.LabelAppID]+`","name":"data","mountPath":"/var/data","sizeGB":10}`,
		http.StatusCreated)
	var disk struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &disk); err != nil || disk.ID == "" {
		t.Fatalf("add-disk body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/disks", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/disks/"+disk.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/disks/"+disk.ID, `{"name":"data-v2","sizeGB":20}`, http.StatusOK)

	// list-snapshots returns Render's documented 201 with signed snapshot keys.
	w = conformDo(t, h, http.MethodGet, "/v1/disks/"+disk.ID+"/snapshots", "", http.StatusCreated)
	var snaps []struct {
		SnapshotKey string `json:"snapshotKey"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &snaps); err != nil || len(snaps) != 1 || snaps[0].SnapshotKey == "" {
		t.Fatalf("list-snapshots body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodPost, "/v1/disks/"+disk.ID+"/snapshots/restore",
		`{"snapshotKey":"`+snaps[0].SnapshotKey+`"}`, http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/disks/"+disk.ID, "", http.StatusNoContent)
}

func TestRenderConformanceBlueprintsWebhooksRegistryCreds(t *testing.T) {
	const ws = "tea-conform"
	bpStore, bpID := newConformBlueprintStore(ws)
	whStore := newConformWebhookStore()
	rcStore := newFakeRCStore()

	srv := NewServer(
		&core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Workspace: fakeWorkspace{"user-1": ws},
			Authz:     &fakeChecker{allow: true},
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			Secrets:            newMemSecretStore(),
			BlueprintsStore:    bpStore,
			WebhookStore:       whStore,
			RegistryCredsStore: rcStore,
			Region:             "fsn1",
			DashboardURL:       "https://dashboard.bex.co",
		},
	)
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.KratosURL = fakeConformKratos(t, "user-1", "alice@example.com")
	h := buildHandler(t, srv)

	// Blueprints: list (200), retrieve (200), update (200), syncs (200), disconnect (204).
	pubBPID := ids.BlueprintPublicID(bpID)
	conformDoSession(t, h, http.MethodGet, "/v1/blueprints", "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/blueprints/"+pubBPID, "", http.StatusOK)
	conformDoSession(t, h, http.MethodPatch, "/v1/blueprints/"+pubBPID, `{"name":"stack-v2","autoSync":false}`, http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/blueprints/"+pubBPID+"/syncs", "", http.StatusOK)
	conformDoSession(t, h, http.MethodDelete, "/v1/blueprints/"+pubBPID, "", http.StatusNoContent)

	// Webhooks: create (201), list (200), retrieve (200), update (200), events (200), delete (204).
	w := conformDoSession(t, h, http.MethodPost, "/v1/webhooks",
		`{"name":"deploy-hook","url":"https://example.com/hook","ownerId":"`+ws+`","enabled":true,"eventFilter":["deploy_ended"]}`,
		http.StatusCreated)
	var wh struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &wh); err != nil || wh.ID == "" {
		t.Fatalf("create-webhook body = %s", w.Body.String())
	}
	conformDoSession(t, h, http.MethodGet, "/v1/webhooks", "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/webhooks/"+wh.ID, "", http.StatusOK)
	conformDoSession(t, h, http.MethodPatch, "/v1/webhooks/"+wh.ID, `{"name":"deploy-hook-v2","enabled":true,"eventFilter":["deploy_started"]}`, http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/webhooks/"+wh.ID+"/events", "", http.StatusOK)
	conformDoSession(t, h, http.MethodDelete, "/v1/webhooks/"+wh.ID, "", http.StatusNoContent)

	// Registry credentials: create (200), list (200), retrieve (200), update (200), delete (204).
	w = conformDoSession(t, h, http.MethodPost, "/v1/registrycredentials",
		`{"name":"ghcr","ownerId":"`+ws+`","registry":"GITHUB","username":"bot","authToken":"tok"}`,
		http.StatusOK)
	var rc struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &rc); err != nil || rc.ID == "" {
		t.Fatalf("create-registry-credential body = %s", w.Body.String())
	}
	conformDoSession(t, h, http.MethodGet, "/v1/registrycredentials", "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/registrycredentials/"+rc.ID, "", http.StatusOK)
	conformDoSession(t, h, http.MethodPatch, "/v1/registrycredentials/"+rc.ID, `{"name":"ghcr-v2","username":"bot2"}`, http.StatusOK)
	conformDoSession(t, h, http.MethodDelete, "/v1/registrycredentials/"+rc.ID, "", http.StatusNoContent)
}

func TestRenderConformanceEnvGroupsProjectsEnvironments(t *testing.T) {
	const ws = "tea-conform"
	svcID := ids.New(ids.Service)
	app := conformApp("grouped-web", svcID)
	app.Labels[core.LabelTenant] = ws

	evgStore := newConformEnvGroupStore()
	peStore := newConformProjectEnvStore()
	wsStore := newFakeWSStore()
	wsStore.tenants = []store.Tenant{{ID: ws, Name: "Acme", Plan: store.PlanPro, CreatedAt: conformEpoch}}
	wsStore.members[ws] = []store.TenantMember{{TenantID: ws, Subject: "client-1", Role: "admin"}}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(app),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			Secrets:           newMemSecretStore(),
			EnvGroupsStore:    evgStore,
			ProjectsStore:     peStore,
			EnvironmentsStore: peStore,
			WorkspaceStore:    wsStore,
			Identities:        metadataIdentities{},
			Region:            "fsn1",
			DashboardURL:      "https://dashboard.bex.co",
		},
	)

	// Environment groups: full CRUD + env-var + secret-file + service link/unlink.
	w := conformDo(t, h, http.MethodPost, "/v1/env-groups",
		`{"name":"app-cfg","ownerId":"`+ws+`","envVars":[{"key":"A","value":"1"}],"secretFiles":[{"name":"cert.pem","content":"pem"}]}`,
		http.StatusCreated)
	var evg struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &evg); err != nil || evg.ID == "" {
		t.Fatalf("create-env-group body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/env-groups/"+evg.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/env-groups/"+evg.ID, `{"name":"app-cfg-v2"}`, http.StatusOK)
	conformDo(t, h, http.MethodPut, "/v1/env-groups/"+evg.ID+"/env-vars/B", `{"value":"2"}`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/env-groups/"+evg.ID+"/env-vars/B", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/env-groups/"+evg.ID+"/env-vars/B", "", http.StatusNoContent)
	conformDo(t, h, http.MethodPut, "/v1/env-groups/"+evg.ID+"/secret-files/key.pem", `{"content":"k"}`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/env-groups/"+evg.ID+"/secret-files/key.pem", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/env-groups/"+evg.ID+"/secret-files/key.pem", "", http.StatusNoContent)
	conformDo(t, h, http.MethodPost, "/v1/env-groups/"+evg.ID+"/services/"+svcID, "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/env-groups/"+evg.ID+"/services/"+svcID, "", http.StatusNoContent)
	conformDo(t, h, http.MethodDelete, "/v1/env-groups/"+evg.ID, "", http.StatusNoContent)

	// Projects & Environments: full CRUD + add/remove resources.
	w = conformDo(t, h, http.MethodPost, "/v1/projects",
		`{"name":"core-proj","ownerId":"`+ws+`"}`,
		http.StatusCreated)
	var prj struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &prj); err != nil || prj.ID == "" {
		t.Fatalf("create-project body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/projects/"+prj.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/projects/"+prj.ID, `{"name":"core-proj-v2"}`, http.StatusOK)

	w = conformDo(t, h, http.MethodPost, "/v1/environments",
		`{"name":"prod","projectId":"`+prj.ID+`"}`, http.StatusCreated)
	var env struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.ID == "" {
		t.Fatalf("create-environment body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/environments?projectId="+prj.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/environments/"+env.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/environments/"+env.ID, `{"name":"production"}`, http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/environments/"+env.ID+"/resources", `{"resourceIds":["`+svcID+`"]}`, http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/environments/"+env.ID+"/resources?resourceIds="+svcID, "", http.StatusNoContent)
	conformDo(t, h, http.MethodDelete, "/v1/environments/"+env.ID, "", http.StatusNoContent)
	conformDo(t, h, http.MethodDelete, "/v1/projects/"+prj.ID, "", http.StatusNoContent)
}

type alwaysPointedDomainTraffic struct{}

func (alwaysPointedDomainTraffic) PointsAtService(context.Context, string, string) bool { return true }

func TestRenderConformanceCoreWritesAndDatastores(t *testing.T) {
	webID := ids.New(ids.Service)
	web := conformApp("core-web", webID)
	web.Spec.Type = appv1alpha1.TypeWebService
	web.Spec.Tier = "starter"
	web.Spec.Replicas = 1
	web.Spec.CacheProfile = appv1alpha1.CacheProfileAll
	web.Labels[core.LabelTenant] = core.DefaultTenant

	cronID := ids.New(ids.Service)
	cronApp := conformApp("core-cron", cronID)
	cronApp.Spec = appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeCronJob, Image: "busybox:1.36", Tier: "starter",
		Schedule: "*/5 * * * *", Command: "date", Replicas: 1,
	}
	cronApp.Labels[core.LabelTenant] = core.DefaultTenant

	db := conformDatabase("pg-ops")
	db.Spec.Plan = "basic-256mb"
	db.Spec.HighAvailability = true
	db.Spec.Version = "16"
	db.Status.BackupsEnabled = true
	db.Labels = map[string]string{core.LabelTenant: core.DefaultTenant}

	kv := conformKeyValue("kv-ops")
	kv.Spec.Plan = "starter"
	kv.Labels = map[string]string{core.LabelTenant: core.DefaultTenant}

	depStore := newConformMutableDeployStore()
	liveDep := conformDeploy(ids.New(ids.Deploy), webID)
	liveDep.Status = store.DeployDeactivated
	liveDep.Image = "core-web:v1"
	liveDep.ResolvedImage = "core-web:v1"
	liveDep.UpdatedAt = conformEpoch
	activeDep := conformDeploy(ids.New(ids.Deploy), webID)
	activeDep.Status = store.DeployBuildInProgress
	activeDep.UpdatedAt = conformEpoch
	activeDep.FinishedAt = nil
	depStore.byApp[webID] = []store.Deploy{activeDep, liveDep}

	h, srv := serverWith(t,
		&core.Base{
			Client: fakeClient(
				web, cronApp, db, kv,
				conformDatastoreSecret("pg-ops-app", map[string]string{
					"username": "u", "password": "p", "dbname": "d",
					"uri": "postgresql://u:p@pg-ops-rw.default:5432/d",
				}),
			),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			Secrets:      newMemSecretStore(),
			DeployStore:  depStore,
			Store:        newConformDiskStore(),
			BaseDomain:   "onbex.co",
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)
	srv.Apps.DomainTraffic = alwaysPointedDomainTraffic{}
	srv.Postgres.SetQueryExecutorForTest(func(_ context.Context, _, sql string, _ bool) (postgres.QueryResult, error) {
		switch {
		case strings.Contains(sql, "pg_stat_activity"):
			return postgres.QueryResult{Rows: [][]any{{
				int64(42), "app", "psql", "active", "SELECT 1", "", "", int64(2),
				"d", "10.0.0.1", conformEpoch.Format(time.RFC3339), conformEpoch.Format(time.RFC3339), true,
			}}}, nil
		case strings.Contains(sql, "pg_stat_statements"):
			return postgres.QueryResult{Rows: [][]any{{
				"SELECT 1", int64(10), 12.5, 1.25, int64(10), int64(5), int64(1), "12345",
				0.5, 2.0, 0.2, int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0), int64(0),
			}}}, nil
		case strings.Contains(sql, "pg_database_size"):
			return postgres.QueryResult{Rows: [][]any{{"d", int64(8192), "8192 bytes"}}}, nil
		case strings.Contains(sql, "pg_indexes"):
			return postgres.QueryResult{Rows: [][]any{{"public", "users", "users_pkey", int64(1024), "1024 bytes"}}}, nil
		case strings.Contains(sql, "pg_table_size"):
			return postgres.QueryResult{Rows: [][]any{{"public", "users", int64(4096), "4096 bytes"}}}, nil
		case strings.Contains(sql, "pg_stat_user_tables"):
			return postgres.QueryResult{Rows: [][]any{{"d", "public", "users", int64(1), int64(10), int64(5), int64(50), int64(100), int64(2)}}}, nil
		default:
			return postgres.QueryResult{}, nil
		}
	})

	// Services lifecycle, scaling, autoscaling, cache purge.
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/scale", `{"numInstances":2}`, http.StatusAccepted)
	conformDo(t, h, http.MethodPut, "/v1/services/"+webID+"/autoscaling",
		`{"enabled":true,"min":1,"max":3,"criteria":{"cpu":{"enabled":true,"percentage":75},"memory":{"enabled":false,"percentage":80}}}`,
		http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/services/"+webID+"/autoscaling", "", http.StatusNoContent)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/cache/purge", `{}`, http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/suspend", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/resume", "", http.StatusAccepted)

	// Service per-key env vars and secret files.
	conformDo(t, h, http.MethodPut, "/v1/services/"+webID+"/env-vars/FOO", `{"value":"bar"}`, http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/services/"+webID+"/env-vars/FOO", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/services/"+webID+"/env-vars/FOO", "", http.StatusNoContent)
	conformDo(t, h, http.MethodPut, "/v1/services/"+webID+"/secret-files/key.pem", `{"content":"pem"}`, http.StatusCreated)
	conformDo(t, h, http.MethodGet, "/v1/services/"+webID+"/secret-files/key.pem", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/services/"+webID+"/secret-files/key.pem", "", http.StatusNoContent)

	// Deploys cancel & rollback.
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/deploys/"+activeDep.ID+"/cancel", "", http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/rollback", `{"deployId":"`+liveDep.ID+`"}`, http.StatusCreated)

	// Custom domains CRUD + verify.
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/custom-domains", `{"name":"app.example.com"}`, http.StatusCreated)
	conformDo(t, h, http.MethodGet, "/v1/services/"+webID+"/custom-domains/app.example.com", "", http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/custom-domains/app.example.com/verify", "", http.StatusAccepted)
	conformDo(t, h, http.MethodDelete, "/v1/services/"+webID+"/custom-domains/app.example.com", "", http.StatusNoContent)

	// Cron job run & cancel.
	conformDo(t, h, http.MethodPost, "/v1/cron-jobs/"+cronID+"/runs", "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/cron-jobs/"+cronID+"/runs", "", http.StatusNoContent)

	// Key Value suspend, resume, delete.
	conformDo(t, h, http.MethodPost, "/v1/key-value/kv-ops/suspend", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/key-value/kv-ops/resume", "", http.StatusAccepted)
	conformDo(t, h, http.MethodDelete, "/v1/key-value/kv-ops", "", http.StatusNoContent)

	// Postgres insights, export, failover, restart, suspend, resume, delete.
	conformDo(t, h, http.MethodGet, "/v1/postgres/pg-ops/query/processes", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/postgres/pg-ops/query/sizes", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/postgres/pg-ops/query/table-scans", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/postgres/pg-ops/query/top-queries", "", http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/postgres/pg-ops/export", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/postgres/pg-ops/failover", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/postgres/pg-ops/restart", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/postgres/pg-ops/suspend", "", http.StatusAccepted)
	conformDo(t, h, http.MethodPost, "/v1/postgres/pg-ops/resume", "", http.StatusAccepted)
	conformDo(t, h, http.MethodDelete, "/v1/postgres/pg-ops", "", http.StatusNoContent)
}
