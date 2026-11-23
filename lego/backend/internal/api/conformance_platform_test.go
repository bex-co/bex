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

// conformance_platform_test.go — composed-server Render response-conformance
// coverage for platform operation families (w5/m581 t003): users/owners/members,
// organization audit logs, service notification overrides, one-off jobs, static
// routes and headers, metrics, and sandboxes (groups, builds, snapshots,
// sandboxes, executions, and connect tokens). Every request runs through
// buildHandler's rendertest.Wrap so status codes and response schemas are
// validated against the pinned OpenAPI contract without growing Allowlist.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- Members + Org Audit fakes ----------------------------------------------

type conformMembersStore struct {
	members.MembersStore
	mu      sync.Mutex
	ws      store.Tenant
	members map[string]store.TenantMember
	ownerID map[string]string
}

func (s *conformMembersStore) GetTenant(_ context.Context, id string) (store.Tenant, error) {
	if id == s.ws.ID {
		return s.ws, nil
	}
	return store.Tenant{}, store.ErrNotFound
}

func (s *conformMembersStore) ListTenantMembers(_ context.Context, tenantID string) ([]store.TenantMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenantID != s.ws.ID {
		return nil, nil
	}
	var out []store.TenantMember
	for _, m := range s.members {
		out = append(out, m)
	}
	return out, nil
}

func (s *conformMembersStore) GetTenantMember(_ context.Context, tenantID, subject string) (store.TenantMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenantID != s.ws.ID {
		return store.TenantMember{}, store.ErrNotFound
	}
	m, ok := s.members[subject]
	if !ok {
		return store.TenantMember{}, store.ErrNotFound
	}
	return m, nil
}

func (s *conformMembersStore) CountTenantMembers(_ context.Context, tenantID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenantID != s.ws.ID {
		return 0, nil
	}
	return len(s.members), nil
}

func (s *conformMembersStore) CountTenantAdmins(_ context.Context, tenantID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenantID != s.ws.ID {
		return 0, nil
	}
	n := 0
	for _, m := range s.members {
		if m.Role == "admin" {
			n++
		}
	}
	return n, nil
}

func (s *conformMembersStore) TenantOwnerSubject(context.Context, string) (string, error) {
	return "", nil
}

func (s *conformMembersStore) UpdateMemberRole(_ context.Context, tenantID, subject, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[subject]
	if !ok || tenantID != s.ws.ID {
		return store.ErrNotFound
	}
	m.Role = role
	s.members[subject] = m
	return nil
}

func (s *conformMembersStore) RemoveMember(_ context.Context, tenantID, subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[subject]; !ok || tenantID != s.ws.ID {
		return store.ErrNotFound
	}
	delete(s.members, subject)
	return nil
}

func (s *conformMembersStore) OwnerIDsForSubjects(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, sub := range subjects {
		if id, ok := s.ownerID[sub]; ok {
			out[sub] = id
		}
	}
	return out, nil
}

type noopMemberRoleSync struct{}

func (noopMemberRoleSync) GrantWorkspaceRole(context.Context, string, string, string) error {
	return nil
}
func (noopMemberRoleSync) RevokeWorkspaceMember(context.Context, string, string, string) error {
	return nil
}

type conformOrgAuditStore struct {
	*fakeAuditStore
	org store.Organization
}

func (s *conformOrgAuditStore) OrganizationRole(_ context.Context, orgID, _ string) (store.Organization, string, error) {
	if orgID != s.org.ID {
		return store.Organization{}, "", store.ErrNotFound
	}
	return s.org, core.OrgRoleOwner, nil
}

// --- Jobs fake --------------------------------------------------------------

type conformJobStore struct {
	mu   sync.Mutex
	jobs map[string]store.Job
}

func (s *conformJobStore) ListJobs(_ context.Context, serviceName, tenantID string, _ store.JobListFilter) ([]store.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.Job
	for _, j := range s.jobs {
		if j.ServiceName == serviceName && (tenantID == "" || j.TenantID == tenantID) {
			out = append(out, j)
		}
	}
	return out, nil
}

func (s *conformJobStore) GetJob(_ context.Context, _, tenantID, jobID string) (store.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[jobID]
	if !ok || (tenantID != "" && j.TenantID != tenantID) {
		return store.Job{}, store.ErrNotFound
	}
	return j, nil
}

func (s *conformJobStore) UpdateJobStatus(_ context.Context, jobID, status string) (store.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[jobID]
	if !ok {
		return store.Job{}, store.ErrNotFound
	}
	j.Status = status
	fin := conformEpoch.Add(time.Minute)
	j.FinishedAt = &fin
	s.jobs[jobID] = j
	return j, nil
}

// --- Sandbox fakes ----------------------------------------------------------

type conformSandboxStore struct {
	sandbox.GroupStore
	sandbox.ImageStore
	sandbox.SnapshotStore
	sandbox.ExecutionStore
	sandbox.RecordStore

	mu        sync.Mutex
	ws        string
	groups    map[string]store.SandboxGroup
	members   map[string]store.SandboxGroupMember
	builds    map[string]store.SandboxBuild
	snapshots map[string]store.SandboxSnapshot
	execs     map[string]store.SandboxExecution
	records   map[string]store.SandboxRecord
}

func newConformSandboxStore(ws string) *conformSandboxStore {
	defID := ids.New(ids.SandboxGroup)
	def := store.SandboxGroup{
		ID: defID, WorkspaceID: ws, Name: "Default", Region: "oregon",
		ConcurrencyLimit: 100, IsDefault: true, CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	return &conformSandboxStore{
		ws:        ws,
		groups:    map[string]store.SandboxGroup{defID: def},
		members:   map[string]store.SandboxGroupMember{},
		builds:    map[string]store.SandboxBuild{},
		snapshots: map[string]store.SandboxSnapshot{},
		execs:     map[string]store.SandboxExecution{},
		records:   map[string]store.SandboxRecord{},
	}
}

func (s *conformSandboxStore) CreateSandboxGroup(_ context.Context, ws, name, region, envID string, limit, _ int) (store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := store.SandboxGroup{
		ID: ids.New(ids.SandboxGroup), WorkspaceID: ws, Name: name, Region: region,
		EnvironmentID: envID, ConcurrencyLimit: limit,
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	s.groups[g.ID] = g
	return g, nil
}

func (s *conformSandboxStore) EnsureDefaultSandboxGroup(_ context.Context, ws string, _ int) (store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.groups {
		if g.WorkspaceID == ws && g.IsDefault {
			return g, nil
		}
	}
	return store.SandboxGroup{}, store.ErrNotFound
}

func (s *conformSandboxStore) SandboxGroups(_ context.Context, ws string) ([]store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.SandboxGroup
	for _, g := range s.groups {
		if g.WorkspaceID == ws {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *conformSandboxStore) SandboxGroup(_ context.Context, ws, id string) (store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok || g.WorkspaceID != ws {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}

func (s *conformSandboxStore) SandboxGroupWorkspace(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return g.WorkspaceID, nil
}

func (s *conformSandboxStore) SetSandboxGroupLimit(_ context.Context, ws, id string, limit int) (store.SandboxGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok || g.WorkspaceID != ws {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	g.ConcurrencyLimit = limit
	g.UpdatedAt = conformEpoch.Add(time.Minute)
	s.groups[id] = g
	return g, nil
}

func (s *conformSandboxStore) SandboxGroupMembers(context.Context, string) (map[string]store.SandboxGroupMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.members), nil
}

type conformSandboxAdmission struct {
	s *conformSandboxStore
}

func (a conformSandboxAdmission) Members(context.Context) (map[string]store.SandboxGroupMember, error) {
	return maps.Clone(a.s.members), nil
}

func (a conformSandboxAdmission) Bind(_ context.Context, sandboxID, groupID string, lease time.Time) error {
	l := lease
	a.s.members[sandboxID] = store.SandboxGroupMember{GroupID: groupID, LeaseUntil: &l}
	return nil
}

func (s *conformSandboxStore) WithSandboxAdmission(_ context.Context, _ string, fn func(store.SandboxAdmission) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(conformSandboxAdmission{s: s})
}
func (s *conformSandboxStore) SetSandboxSlotLease(context.Context, string, string, *time.Time) error {
	return nil
}
func (s *conformSandboxStore) ReleaseSandboxSlot(context.Context, string, string) error { return nil }
func (s *conformSandboxStore) PruneSandboxGroupMembers(context.Context, string, []string) error {
	return nil
}

// ImageStore methods.
func (s *conformSandboxStore) CreateSandboxBuild(_ context.Context, in store.NewSandboxBuild, _ int) (store.SandboxBuild, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sz := in.SizeBytes
	b := store.SandboxBuild{
		ID: ids.New(ids.SandboxBuild), WorkspaceID: in.WorkspaceID, SandboxGroupID: in.SandboxGroupID,
		Name: in.Name, SourceRepository: in.SourceRepository, SourceImage: in.SourceImage,
		SourceReference: in.SourceReference, SourceDigest: in.SourceDigest,
		Status: store.SandboxBuildPending, SizeBytes: &sz,
		CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	s.builds[b.ID] = b
	return b, true, nil
}

func (s *conformSandboxStore) SandboxBuild(_ context.Context, ws, gid, id string) (store.SandboxBuild, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.builds[id]
	if !ok || b.WorkspaceID != ws || b.SandboxGroupID != gid {
		return store.SandboxBuild{}, store.ErrNotFound
	}
	return b, nil
}

func (s *conformSandboxStore) SandboxBuilds(_ context.Context, ws, gid, _ string, _ []string) ([]store.SandboxBuild, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.SandboxBuild
	for _, b := range s.builds {
		if b.WorkspaceID == ws && b.SandboxGroupID == gid {
			out = append(out, b)
		}
	}
	return out, nil
}

func (s *conformSandboxStore) StartSandboxBuild(context.Context, string) (bool, error) {
	return false, nil
}

func (s *conformSandboxStore) DeleteSandboxBuild(_ context.Context, ws, gid, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.builds[id]
	if !ok || b.WorkspaceID != ws || b.SandboxGroupID != gid {
		return false, nil
	}
	delete(s.builds, id)
	return true, nil
}

// SnapshotStore methods.
func (s *conformSandboxStore) CreateSandboxSnapshot(_ context.Context, in store.NewSandboxSnapshot, _ store.SandboxSnapshotLimits, now time.Time) (store.SandboxSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exp *time.Time
	if in.TTLSeconds > 0 {
		t := now.Add(time.Duration(in.TTLSeconds) * time.Second)
		exp = &t
	}
	snap := store.SandboxSnapshot{
		ID: ids.New(ids.SandboxSnapshot), WorkspaceID: in.WorkspaceID, SandboxGroupID: in.SandboxGroupID,
		SourceSandboxID: in.SourceSandboxID, SourceRef: in.SourceRef, Name: in.Name,
		Kind: in.Kind, Status: store.SandboxSnapshotCreating, Plan: in.Plan,
		NetworkPolicy: in.NetworkPolicy, TimeoutSeconds: in.TimeoutSeconds,
		IdleTimeoutSeconds: in.IdleTimeoutSeconds, TTLSeconds: in.TTLSeconds,
		ExpiresAt: exp, RequestedBy: in.RequestedBy, RequestedAt: now,
	}
	s.snapshots[snap.ID] = snap
	return snap, nil
}

func (s *conformSandboxStore) SandboxSnapshot(_ context.Context, ws, gid, id string, _ time.Time) (store.SandboxSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snapshots[id]
	if !ok || snap.WorkspaceID != ws || snap.SandboxGroupID != gid {
		return store.SandboxSnapshot{}, store.ErrNotFound
	}
	return snap, nil
}

func (s *conformSandboxStore) SandboxSnapshots(_ context.Context, ws, gid, _ string, _ []string, _ time.Time) ([]store.SandboxSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.SandboxSnapshot
	for _, snap := range s.snapshots {
		if snap.WorkspaceID == ws && snap.SandboxGroupID == gid {
			out = append(out, snap)
		}
	}
	return out, nil
}

func (s *conformSandboxStore) DeleteSandboxSnapshot(_ context.Context, ws, gid, id string, _ time.Time) (store.SandboxSnapshot, store.SandboxSnapshotDeletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snapshots[id]
	if !ok || snap.WorkspaceID != ws || snap.SandboxGroupID != gid {
		return store.SandboxSnapshot{}, store.SandboxSnapshotGone, nil
	}
	delete(s.snapshots, id)
	return snap, store.SandboxSnapshotDeleted, nil
}

func (s *conformSandboxStore) SetSandboxSnapshotRuntimeID(context.Context, string, string) (bool, error) {
	return true, nil
}
func (s *conformSandboxStore) EndSandboxStorage(context.Context, string, string, time.Time) error {
	return nil
}

// ExecutionStore methods.
func (s *conformSandboxStore) RecordSandboxExecution(_ context.Context, e store.SandboxExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execs[e.ID] = e
	return nil
}

func (s *conformSandboxStore) SandboxExecutions(_ context.Context, ws, sandboxID, _ string, _ int) ([]store.SandboxExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.SandboxExecution
	for _, e := range s.execs {
		if e.WorkspaceID == ws && e.SandboxID == sandboxID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *conformSandboxStore) SandboxExecution(_ context.Context, ws, sandboxID, executionID string) (store.SandboxExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.execs[executionID]
	if !ok || e.WorkspaceID != ws || e.SandboxID != sandboxID {
		return store.SandboxExecution{}, store.ErrNotFound
	}
	return e, nil
}

func (s *conformSandboxStore) StopOpenSandboxExecutions(context.Context, string, string, time.Time) error {
	return nil
}
func (s *conformSandboxStore) StopStaleSandboxExecutions(context.Context, string, string, time.Time) error {
	return nil
}
func (s *conformSandboxStore) OpenSandboxExecutions(context.Context, string, string, time.Time, int) ([]store.OpenSandboxExecution, error) {
	return nil, nil
}
func (s *conformSandboxStore) PruneSandboxExecutions(context.Context, string, []string) error {
	return nil
}

// RecordStore methods.
func (s *conformSandboxStore) RecordSandbox(_ context.Context, r store.SandboxRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[r.SandboxID] = r
	return nil
}
func (s *conformSandboxStore) EndAbsentSandboxes(context.Context, string, []string, time.Time) error {
	return nil
}
func (s *conformSandboxStore) EndedSandbox(_ context.Context, _, _ string) (store.SandboxRecord, error) {
	return store.SandboxRecord{}, store.ErrNotFound
}
func (s *conformSandboxStore) EndedSandboxes(context.Context, string) ([]store.SandboxRecord, error) {
	return nil, nil
}
func (s *conformSandboxStore) EndedSandboxWorkspaces(context.Context, string) ([]string, error) {
	return nil, nil
}

// fakeOpenSandboxServer is a minimal stateful OpenSandbox HTTP server for
// composed-server conformance checks.
func fakeOpenSandboxServer(t *testing.T) *httptest.Server {
	t.Helper()
	type box struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
		Created  time.Time         `json:"createdAt"`
		Image    struct {
			URI string `json:"uri"`
		} `json:"image"`
		Status struct {
			State string `json:"state"`
		} `json:"status"`
	}
	var mu sync.Mutex
	boxes := map[string]*box{}
	seq := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			var body struct {
				Metadata map[string]string `json:"metadata"`
				Image    struct {
					URI string `json:"uri"`
				} `json:"image"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			seq++
			id := fmt.Sprintf("os-conform-%d", seq)
			b := &box{ID: id, Metadata: maps.Clone(body.Metadata), Created: conformEpoch}
			b.Image.URI = body.Image.URI
			b.Status.State = "Running"
			boxes[id] = b
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(b)
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			out := make([]*box, 0, len(boxes))
			for _, id := range slices.Sorted(maps.Keys(boxes)) {
				out = append(out, boxes[id])
			}
			_ = json.NewEncoder(w).Encode(out)
		case strings.HasPrefix(r.URL.Path, "/sandboxes/"):
			rest := strings.TrimPrefix(r.URL.Path, "/sandboxes/")
			parts := strings.Split(rest, "/")
			b, ok := boxes[parts[0]]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if len(parts) == 1 && r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(b)
				return
			}
			if len(parts) == 1 && r.Method == http.MethodDelete {
				delete(boxes, parts[0])
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if len(parts) == 2 && parts[1] == "snapshots" && r.Method == http.MethodPost {
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "cap-1", "status": map[string]any{"state": "Creating"}})
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- Conformance test suites ------------------------------------------------

func TestRenderConformanceUsersOwnersMembersAudit(t *testing.T) {
	const ws = "tea-conform"
	orgID := ids.New(ids.Organization)
	aliceUserID := ids.New(ids.Owner)
	bobUserID := ids.New(ids.Owner)

	wsStore := newFakeWSStore()
	wsStore.tenants = []store.Tenant{{ID: ws, Name: "Acme", Plan: store.PlanPro, CreatedAt: conformEpoch}}
	wsStore.members[ws] = []store.TenantMember{
		{TenantID: ws, Subject: "user-alice", Role: "admin"},
		{TenantID: ws, Subject: "user-bob", Role: "developer"},
	}
	wsStore.ownerIDs = map[string]string{"user-alice": aliceUserID, "user-bob": bobUserID}

	memStore := &conformMembersStore{
		ws: wsStore.tenants[0],
		members: map[string]store.TenantMember{
			"user-alice": {TenantID: ws, Subject: "user-alice", Role: "admin"},
			"user-bob":   {TenantID: ws, Subject: "user-bob", Role: "developer"},
		},
		ownerID: wsStore.ownerIDs,
	}

	orgAudit := &conformOrgAuditStore{
		fakeAuditStore: &fakeAuditStore{rows: []store.AuditRow{{
			ID: "aud-org-1", WorkspaceID: orgID, Caller: "user-alice", CallerMethod: "session",
			Verb: core.AuditVerbLogout, Resource: core.OrganizationObject(orgID),
			Outcome: string(core.AuditAllowed), At: conformEpoch,
		}}},
		org: store.Organization{ID: orgID, Name: "Acme Org", PrimaryWorkspaceID: ws, Plan: store.PlanEnterprise, CreatedAt: conformEpoch, AuditVisibleFrom: conformEpoch.Add(-time.Hour)},
	}

	base := &core.Base{
		Client:    fakeClient(),
		Namespace: "default",
		Workspace: fakeWorkspace{"user-alice": ws},
		Authz:     &fakeChecker{allow: true},
		Clock:     func() time.Time { return conformEpoch },
	}
	auditSvc := &audit.Service{
		Base:    base,
		Store:   orgAudit,
		UserIDs: auditUserIDs{"user-alice": aliceUserID},
		Emails:  auditEmails{"user-alice": "alice@example.com"},
	}

	srv := NewServer(base, Deps{
		WorkspaceStore: wsStore,
		MembersStore:   memStore,
		MembersGranter: noopMemberRoleSync{},
		MembersRevoker: noopMemberRoleSync{},
		Identities:     metadataIdentities{},
		Audit:          auditSvc,
		Region:         "fsn1",
		DashboardURL:   "https://dashboard.bex.co",
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.KratosURL = fakeConformKratos(t, "user-alice", "alice@example.com")
	h := buildHandler(t, srv)

	// Users, owners, members update & delete.
	conformDoSession(t, h, http.MethodGet, "/v1/users", "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/owners", "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/owners/"+ws, "", http.StatusOK)
	conformDoSession(t, h, http.MethodGet, "/v1/owners/"+ws+"/members", "", http.StatusOK)
	conformDoSession(t, h, http.MethodPatch, "/v1/owners/"+ws+"/members/"+bobUserID, `{"role":"ADMIN"}`, http.StatusOK)
	conformDoSession(t, h, http.MethodDelete, "/v1/owners/"+ws+"/members/"+bobUserID, "", http.StatusNoContent)

	// Organization audit logs.
	conformDoSession(t, h, http.MethodGet, "/v1/organizations/"+orgID+"/audit-logs", "", http.StatusOK)
}

func TestRenderConformanceNotificationsJobsRoutesHeaders(t *testing.T) {
	const ws = "tea-conform"
	webID := ids.New(ids.Service)
	web := conformApp("notif-web", webID)
	web.Spec.Type = appv1alpha1.TypeWebService
	web.Spec.Tier = "starter"
	web.Labels[core.LabelTenant] = ws

	siteID := ids.New(ids.Service)
	site := conformApp("static-site", siteID)
	site.Spec = appv1alpha1.AppSpec{
		Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/site",
		Branch: "main", BuildCommand: "npm run build", PublishPath: "dist", Replicas: 1,
	}
	site.Labels[core.LabelTenant] = ws

	jobID := ids.New(ids.Job)
	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobID, Namespace: ws},
		Status:     batchv1.JobStatus{Active: 1},
	}
	jobStore := &conformJobStore{jobs: map[string]store.Job{
		jobID: {
			ID: jobID, ServiceName: web.Name, TenantID: ws,
			StartCommand: "echo hi", PlanID: "starter", Status: store.JobRunning,
			CreatedAt: conformEpoch, StartedAt: &conformEpoch,
		},
	}}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(web, site, k8sJob),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{
			JobStore:     jobStore,
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)

	// Service notification overrides (GET + PATCH).
	conformDo(t, h, http.MethodGet, "/v1/notification-settings/overrides/services/"+webID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/notification-settings/overrides/services/"+webID,
		`{"notificationsToSend":"all","previewNotificationsEnabled":"false"}`, http.StatusOK)

	// Jobs: list (200), retrieve (200), cancel (200), and post-job (410 validated against error schema).
	conformDo(t, h, http.MethodGet, "/v1/services/"+webID+"/jobs", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/services/"+webID+"/jobs/"+jobID, "", http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/jobs/"+jobID+"/cancel", "", http.StatusOK)
	conformDo(t, h, http.MethodPost, "/v1/services/"+webID+"/jobs", `{"startCommand":"echo hi"}`, http.StatusGone)

	// Static site routes: add (201), patch (200), delete (204).
	w := conformDo(t, h, http.MethodPost, "/v1/services/"+siteID+"/routes",
		`{"type":"redirect","source":"/old","destination":"/new"}`, http.StatusCreated)
	var route struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &route); err != nil || route.ID == "" {
		t.Fatalf("add-route body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodPatch, "/v1/services/"+siteID+"/routes/"+route.ID,
		`{"priority":0}`, http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/services/"+siteID+"/routes/"+route.ID, "", http.StatusNoContent)

	// Static site headers: add (201), delete (204).
	w = conformDo(t, h, http.MethodPost, "/v1/services/"+siteID+"/headers",
		`{"path":"/*","name":"X-Frame-Options","value":"DENY"}`, http.StatusCreated)
	var hdr struct {
		Headers struct{ ID string } `json:"headers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hdr); err != nil || hdr.Headers.ID == "" {
		t.Fatalf("add-headers body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodDelete, "/v1/services/"+siteID+"/headers/"+hdr.Headers.ID, "", http.StatusNoContent)
}

func TestRenderConformanceMetricsRemaining(t *testing.T) {
	const ws = "tea-conform"
	webID := ids.New(ids.Service)
	web := conformApp("metrics-web", webID)
	web.Spec.Type = appv1alpha1.TypeWebService
	web.Spec.Tier = "starter"
	web.Labels[core.LabelTenant] = ws

	db := conformDatabase("metrics-pg")
	db.Spec.Plan = "basic-256mb"
	db.Spec.StorageGB = 10
	db.Labels = map[string]string{core.LabelTenant: ws}

	metricsNow := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	start := metricsNow.Add(-time.Hour).Format(time.RFC3339)
	end := metricsNow.Format(time.RFC3339)

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(web, db),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return metricsNow },
		},
		Deps{
			RequestMetrics: func(_ context.Context, req metrics.RequestMetricsRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{
					Unit:   "bytes",
					Labels: map[string]string{"resource": webID},
					Points: []metrics.MetricPoint{{Timestamp: metricsNow.Format(time.RFC3339), Value: 1024}},
				}}, nil
			},
			DiskUsage: func(_ context.Context, req metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) {
				return []metrics.MetricSeries{{
					Unit:   "bytes",
					Labels: map[string]string{"resource": db.Name},
					Points: []metrics.MetricPoint{{Timestamp: metricsNow.Format(time.RFC3339), Value: 10 * 1024 * 1024 * 1024}},
				}}, nil
			},
			MetricsFilterValues: func(_ context.Context, req metrics.MetricsFilterValuesRequest) ([]string, error) {
				switch req.Label {
				case "host":
					return []string{"metrics-web.onbex.co"}, nil
				case "statusCode", "code":
					return []string{"200"}, nil
				default:
					return []string{"pod-1"}, nil
				}
			},
			InstanceFilterValues: func(_ context.Context, _ metrics.InstanceFilterValuesRequest) ([]string, error) {
				return []string{"web-old-11111"}, nil
			},
			HTTPFilterValues: func(_ context.Context, _ metrics.HTTPFilterValuesRequest) (metrics.HTTPFilterValues, error) {
				return metrics.HTTPFilterValues{
					Hosts:       []string{"metrics-web.onbex.co"},
					StatusCodes: []string{"200"},
				}, nil
			},
			PathFilterSuggestions: func(_ context.Context, _ metrics.PathFilterSuggestionsRequest) ([]string, error) {
				return []string{"/api/v1/users", "/api/health"}, nil
			},
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)

	conformDo(t, h, http.MethodGet, "/v1/metrics/bandwidth-sources?resource="+webID+"&startTime="+start+"&endTime="+end, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/metrics/disk-capacity?resource="+db.Name+"&startTime="+start+"&endTime="+end, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/metrics/filters/application?resource="+webID+"&startTime="+start+"&endTime="+end, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/metrics/filters/http?resource="+webID+"&startTime="+start+"&endTime="+end+"&host=metrics-web.onbex.co&statusCode=200", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/metrics/filters/path?resource="+webID+"&startTime="+start+"&endTime="+end+"&host=metrics-web.onbex.co&statusCode=200&path=/api/*", "", http.StatusOK)
}

type conformNonceStore struct{}

func (conformNonceStore) ClaimShellNonce(context.Context, string, time.Time) (bool, error) {
	return true, nil
}

func TestRenderConformanceSandboxes(t *testing.T) {
	const ws = "tea-conform"
	st := newConformSandboxStore(ws)
	osSrv := fakeOpenSandboxServer(t)

	// In-process OCI registries so POST /v1/sandbox-groups/{id}/builds
	// inspects a valid linux/amd64 image manifest + config blob.
	quietReg := registry.Logger(log.New(io.Discard, "", 0))
	srcReg := httptest.NewServer(registry.New(quietReg))
	t.Cleanup(srcReg.Close)
	sinkReg := httptest.NewServer(registry.New(quietReg))
	t.Cleanup(sinkReg.Close)

	srcHost := strings.TrimPrefix(srcReg.URL, "http://")
	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = "linux", "amd64"
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(srcHost + "/library/python:3.12")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}

	imgSvc := &sandbox.Images{
		Store:           st,
		Registry:        sandbox.RegistryConfig{BaseURL: sinkReg.URL, Prefix: "snapshots"},
		SourceTransport: http.DefaultTransport,
	}
	snapSvc := &sandbox.Snapshots{
		Store: st,
	}

	h, _ := serverWith(t,
		&core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Workspace: fakeWorkspace{"client-1": ws},
			Clock:     func() time.Time { return conformEpoch },
			Plans:     proPlan(),
		},
		Deps{
			SandboxClient:          sandbox.NewClient(osSrv.URL),
			SandboxTemplates:       map[string]sandbox.Template{"node": {Image: "node:20", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}},
			SandboxDefaultTemplate: "node",
			SandboxGroups:          st,
			SandboxImages:          imgSvc,
			SandboxSnapshots:       snapSvc,
			SandboxExecutions:      st,
			SandboxRecords:         st,
			SandboxExec: &sandbox.ExecConfig{
				Secret:         []byte("0123456789abcdef0123456789abcdef"),
				GatewayURL:     "http://gateway.invalid:8081/sandbox-exec",
				FileGatewayURL: "http://gateway.invalid:8081/sandbox-files",
				PublicURL:      "https://api.bex.co",
				Nonces:         &sshgateway.NonceGuard{Store: conformNonceStore{}},
			},
			Region:       "fsn1",
			DashboardURL: "https://dashboard.bex.co",
		},
	)

	// Sandbox groups: create (201), list (200), retrieve (200), update (200).
	w := conformDo(t, h, http.MethodPost, "/v1/sandbox-groups",
		`{"name":"agents","ownerId":"`+ws+`","region":"oregon"}`,
		http.StatusCreated)
	var grp struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &grp); err != nil || grp.ID == "" {
		t.Fatalf("create-sandbox-group body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups?ownerId="+ws, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups/"+grp.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodPatch, "/v1/sandbox-groups/"+grp.ID, `{"concurrencyLimit":50}`, http.StatusOK)

	// Sandbox builds: create (201), list (200), retrieve (200), delete (204).
	w = conformDo(t, h, http.MethodPost, "/v1/sandbox-groups/"+grp.ID+"/builds",
		`{"name":"py312","sourceImage":"`+srcHost+`/library/python:3.12"}`,
		http.StatusCreated)
	var bld struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &bld); err != nil || bld.ID == "" {
		t.Fatalf("create-sandbox-build body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups/"+grp.ID+"/builds", "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups/"+grp.ID+"/builds/"+bld.ID, "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/sandbox-groups/"+grp.ID+"/builds/"+bld.ID, "", http.StatusNoContent)

	// Sandboxes: create (201), list (200), retrieve (200).
	w = conformDo(t, h, http.MethodPost, "/v1/sandboxes",
		`{"ownerId":"`+ws+`","sandboxGroupId":"`+grp.ID+`","plan":"starter"}`,
		http.StatusCreated)
	var sbx struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &sbx); err != nil || sbx.ID == "" {
		t.Fatalf("create-sandbox body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/sandboxes?ownerId="+ws, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/sandboxes/"+sbx.ID+"?ownerId="+ws, "", http.StatusOK)

	// Sandbox connect tokens & executions: connect-run (201), connect-files (201), list-execs (200), retrieve-exec (200).
	w = conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/runs/stream/token", `{"command":"echo ok"}`, http.StatusCreated)
	var runTok struct {
		ExecutionID string `json:"executionId"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &runTok)
	conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/files/upload/token?path=/workspace/a.txt", "", http.StatusCreated)
	w = conformDo(t, h, http.MethodGet, "/v1/sandboxes/"+sbx.ID+"/execs?ownerId="+ws, "", http.StatusOK)
	var execs []struct {
		Execution struct{ ID string } `json:"execution"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &execs); err != nil || len(execs) == 0 {
		t.Fatalf("list-sandbox-executions body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/sandboxes/"+sbx.ID+"/execs/"+execs[0].Execution.ID+"?ownerId="+ws, "", http.StatusOK)

	// Sandbox snapshots: create (202), list (200), retrieve (200), delete (204).
	w = conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/snapshots?ownerId="+ws,
		`{"name":"snap-1","kind":"filesystem"}`, http.StatusAccepted)
	var snap struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil || snap.ID == "" {
		t.Fatalf("create-sandbox-snapshot body = %s", w.Body.String())
	}
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups/"+grp.ID+"/snapshots?ownerId="+ws, "", http.StatusOK)
	conformDo(t, h, http.MethodGet, "/v1/sandbox-groups/"+grp.ID+"/snapshots/"+snap.ID+"?ownerId="+ws, "", http.StatusOK)
	conformDo(t, h, http.MethodDelete, "/v1/sandbox-groups/"+grp.ID+"/snapshots/"+snap.ID+"?ownerId="+ws, "", http.StatusNoContent)

	// Terminate sandbox (204).
	conformDo(t, h, http.MethodPost, "/v1/sandboxes/"+sbx.ID+"/terminate?ownerId="+ws, "", http.StatusNoContent)
}
