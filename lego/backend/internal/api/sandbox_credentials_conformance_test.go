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
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type conformSandboxCredRec struct {
	cred     store.SandboxCredential
	nextVer  int64
	versions []store.SandboxCredentialVersion
}

type conformSandboxCredStore struct {
	mu     sync.Mutex
	group  store.SandboxGroup
	salt   []byte
	byCred map[string]*conformSandboxCredRec
}

func newConformSandboxCredStore(ws, groupID string) *conformSandboxCredStore {
	return &conformSandboxCredStore{
		group: store.SandboxGroup{
			ID: groupID, WorkspaceID: ws, Name: "Default", Region: "oregon",
			ConcurrencyLimit: 100, IsDefault: true, CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
		},
		salt:   []byte("0123456789abcdef0123456789abcdef"),
		byCred: map[string]*conformSandboxCredRec{},
	}
}

func (m *conformSandboxCredStore) CreateSandboxGroup(context.Context, string, string, string, string, int, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *conformSandboxCredStore) EnsureDefaultSandboxGroup(context.Context, string, int) (store.SandboxGroup, error) {
	return m.group, nil
}
func (m *conformSandboxCredStore) SandboxGroups(context.Context, string) ([]store.SandboxGroup, error) {
	return []store.SandboxGroup{m.group}, nil
}
func (m *conformSandboxCredStore) SandboxGroup(_ context.Context, ws, id string) (store.SandboxGroup, error) {
	if id == m.group.ID && ws == m.group.WorkspaceID {
		return m.group, nil
	}
	return store.SandboxGroup{}, store.ErrNotFound
}
func (m *conformSandboxCredStore) SandboxGroupWorkspace(_ context.Context, id string) (string, error) {
	if id == m.group.ID {
		return m.group.WorkspaceID, nil
	}
	return "", store.ErrNotFound
}
func (m *conformSandboxCredStore) SetSandboxGroupLimit(context.Context, string, string, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *conformSandboxCredStore) SetDefaultSandboxGroup(context.Context, string, string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *conformSandboxCredStore) MarkSandboxGroupDeleting(context.Context, string, string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *conformSandboxCredStore) ClaimStaleSandboxGroupDeletes(context.Context, time.Duration) ([]store.SandboxGroup, error) {
	return nil, nil
}
func (m *conformSandboxCredStore) DeleteSandboxGroup(context.Context, string, string) error {
	return nil
}
func (m *conformSandboxCredStore) SandboxGroupMembers(context.Context, string) (map[string]store.SandboxGroupMember, error) {
	return map[string]store.SandboxGroupMember{}, nil
}
func (m *conformSandboxCredStore) WithSandboxAdmission(context.Context, string, func(store.SandboxAdmission) error) error {
	return nil
}
func (m *conformSandboxCredStore) SetSandboxSlotLease(context.Context, string, string, *time.Time) error {
	return nil
}
func (m *conformSandboxCredStore) ReleaseSandboxSlot(context.Context, string, string) error {
	return nil
}
func (m *conformSandboxCredStore) PruneSandboxGroupMembers(context.Context, string, []string) error {
	return nil
}

func (m *conformSandboxCredStore) ListSandboxGroupCredentialSecretRefs(context.Context, string, string) ([]store.SandboxCredentialSecretRefEntry, error) {
	return nil, nil
}

func (m *conformSandboxCredStore) ListSandboxGroupCredentials(_ context.Context, ws, gid string) ([]store.SandboxCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.SandboxCredential, 0, len(m.byCred))
	if ws != m.group.WorkspaceID || gid != m.group.ID {
		return out, nil
	}
	for _, r := range m.byCred {
		out = append(out, r.cred)
	}
	slices.SortFunc(out, func(a, b store.SandboxCredential) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (m *conformSandboxCredStore) GetSandboxGroupCredential(_ context.Context, ws, gid, cid string) (store.SandboxCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ws != m.group.WorkspaceID || gid != m.group.ID {
		return store.SandboxCredential{}, store.ErrNotFound
	}
	r, ok := m.byCred[cid]
	if !ok {
		return store.SandboxCredential{}, store.ErrNotFound
	}
	return r.cred, nil
}

func (m *conformSandboxCredStore) ListSandboxGroupCredentialVersions(_ context.Context, ws, gid, cid string) ([]store.SandboxCredentialVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ws != m.group.WorkspaceID || gid != m.group.ID {
		return nil, store.ErrNotFound
	}
	r, ok := m.byCred[cid]
	if !ok {
		return nil, store.ErrNotFound
	}
	out := slices.Clone(r.versions)
	slices.SortFunc(out, func(a, b store.SandboxCredentialVersion) int { return int(b.ID - a.ID) })
	return out, nil
}

func (m *conformSandboxCredStore) CreateSandboxGroupCredential(ctx context.Context, ws, gid, cid string, prepare store.SandboxCredentialPrepareCreateFn) (store.SandboxCredential, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byCred[cid]; exists {
		return store.SandboxCredential{}, "", store.ErrSandboxCredentialExists
	}
	p, err := prepare(ctx, m.salt)
	if err != nil {
		return store.SandboxCredential{}, "", err
	}
	if p.OnBeforeCommit != nil {
		if err := p.OnBeforeCommit(ctx, p.SecretRef, false); err != nil {
			return store.SandboxCredential{}, "", err
		}
	}
	cfg := store.CanonicalSandboxCredentialConfig(p.Config)
	v := store.SandboxCredentialVersion{
		ID: 1, UpdatedAt: conformEpoch, UpdatedBy: p.UpdatedBy,
		Fingerprint: p.Fingerprint, SecretRef: p.SecretRef, Config: cfg, SecretBytes: p.SecretBytes,
	}
	c := store.SandboxCredential{
		WorkspaceID: ws, SandboxGroupID: gid, ID: cid, Config: cfg,
		CurrentVersion: v, CurrentSecretRef: p.SecretRef, CreatedAt: conformEpoch, UpdatedAt: conformEpoch,
	}
	m.byCred[cid] = &conformSandboxCredRec{cred: c, nextVer: 2, versions: []store.SandboxCredentialVersion{v}}
	return c, p.SecretRef, nil
}

func (m *conformSandboxCredStore) UpdateSandboxGroupCredential(ctx context.Context, ws, gid, cid string, prepare store.SandboxCredentialPrepareUpdateFn) (store.SandboxCredential, []string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.byCred[cid]
	if !ok || ws != m.group.WorkspaceID || gid != m.group.ID {
		return store.SandboxCredential{}, nil, "", store.ErrNotFound
	}
	p, err := prepare(ctx, m.salt, r.cred)
	if err != nil {
		return store.SandboxCredential{}, nil, "", err
	}
	if p.OnBeforeCommit != nil {
		if err := p.OnBeforeCommit(ctx, p.SecretRef, false); err != nil {
			return store.SandboxCredential{}, nil, "", err
		}
	}
	cfg := store.CanonicalSandboxCredentialConfig(p.Config)
	v := store.SandboxCredentialVersion{
		ID: r.nextVer, UpdatedAt: conformEpoch, UpdatedBy: p.UpdatedBy,
		Fingerprint: p.Fingerprint, SecretRef: p.SecretRef, Config: cfg, SecretBytes: p.SecretBytes,
	}
	r.nextVer++
	r.versions = append(r.versions, v)
	r.cred.Config = cfg
	r.cred.CurrentVersion = v
	r.cred.CurrentSecretRef = p.SecretRef
	r.cred.UpdatedAt = conformEpoch
	return r.cred, nil, p.SecretRef, nil
}

func (m *conformSandboxCredStore) RestoreSandboxGroupCredential(_ context.Context, ws, gid, cid string, targetVersionID int64, updatedBy string) (store.SandboxCredential, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.byCred[cid]
	if !ok || ws != m.group.WorkspaceID || gid != m.group.ID {
		return store.SandboxCredential{}, nil, store.ErrNotFound
	}
	var target *store.SandboxCredentialVersion
	for i := range r.versions {
		if r.versions[i].ID == targetVersionID {
			target = &r.versions[i]
			break
		}
	}
	if target == nil {
		return store.SandboxCredential{}, nil, store.ErrSandboxCredentialVersionNotFound
	}
	from := targetVersionID
	v := store.SandboxCredentialVersion{
		ID: r.nextVer, UpdatedAt: conformEpoch, UpdatedBy: updatedBy,
		Fingerprint: target.Fingerprint, RestoredFromVersionID: &from,
		SecretRef: target.SecretRef, Config: target.Config, SecretBytes: target.SecretBytes,
	}
	r.nextVer++
	r.versions = append(r.versions, v)
	r.cred.Config = target.Config
	r.cred.CurrentVersion = v
	r.cred.CurrentSecretRef = target.SecretRef
	r.cred.UpdatedAt = conformEpoch
	return r.cred, nil, nil
}

func (m *conformSandboxCredStore) DeleteSandboxGroupCredential(ctx context.Context, ws, gid, cid string, onBeforeCommit func(context.Context, []string) error) ([]string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ws != m.group.WorkspaceID || gid != m.group.ID {
		return nil, false, store.ErrNotFound
	}
	_, existed := m.byCred[cid]
	if onBeforeCommit != nil {
		if err := onBeforeCommit(ctx, nil); err != nil {
			return nil, false, err
		}
	}
	delete(m.byCred, cid)
	return nil, existed, nil
}

// checkSandboxGroupCredentialsConformance exercises all 7 pinned Render
// sandbox-group-credential operations against Render's OpenAPI response schemas
// with zero allowlist entries.
func checkSandboxGroupCredentialsConformance(t *testing.T, spec *renderSpec) {
	t.Helper()
	groupID := ids.New(ids.SandboxGroup)
	st := newConformSandboxCredStore(core.DefaultTenant, groupID)
	upstream := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(upstream.Close)

	h, srv := serverWith(t,
		&core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{
			Secrets:            newMemSecretStore(),
			SandboxClient:      sandbox.NewClient(upstream.URL),
			SandboxGroups:      st,
			SandboxCredentials: st,
		},
	)
	validate := func(opID string, w *httptest.ResponseRecorder, wantStatus int) {
		t.Helper()
		if w.Code != wantStatus {
			t.Fatalf("%s = %d (want %d): %s", opID, w.Code, wantStatus, w.Body.String())
		}
		if errs := rendertest.Unallowed(opID, spec.validateStatus(opID, w.Code, w.Body.Bytes()), unconfiguredSources(srv)...); len(errs) > 0 {
			t.Errorf("%s (%d) diverges from Render's schema:\n  %s\nbody: %s", opID, w.Code, strings.Join(errs, "\n  "), w.Body.String())
		}
	}

	basePath := "/v1/sandbox-groups/" + groupID + "/credentials"

	// 1. Empty list (200 []).
	w := do(t, h, http.MethodGet, basePath, testToken, "")
	validate("list-sandbox-group-credentials", w, http.StatusOK)

	// 2. Create (201).
	w = do(t, h, http.MethodPost, basePath, testToken,
		`{"id":"my-key","domain":"api.render.com","value":"s3cr3t","methods":["GET","POST"],"paths":["/v1/*"],"injection":{"type":"header","name":"Authorization","template":"Bearer ${SECRET}"}}`)
	validate("create-sandbox-group-credential", w, http.StatusCreated)
	var created sandbox.GroupCredential
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Version.ID != 1 {
		t.Fatalf("create body = %s, err=%v", w.Body.String(), err)
	}

	// Duplicate create error envelope (409).
	w = do(t, h, http.MethodPost, basePath, testToken,
		`{"id":"my-key","domain":"api.render.com","value":"s3cr3t","injection":{"type":"header","name":"Authorization"}}`)
	validate("create-sandbox-group-credential", w, http.StatusConflict)

	// 3. Populated list (200).
	w = do(t, h, http.MethodGet, basePath, testToken, "")
	validate("list-sandbox-group-credentials", w, http.StatusOK)

	// 4. Retrieve (200) & missing (404).
	item := basePath + "/my-key"
	w = do(t, h, http.MethodGet, item, testToken, "")
	validate("retrieve-sandbox-group-credential", w, http.StatusOK)
	w = do(t, h, http.MethodGet, basePath+"/missing", testToken, "")
	validate("retrieve-sandbox-group-credential", w, http.StatusNotFound)

	// 5. Update (200) & invalid input (400).
	w = do(t, h, http.MethodPatch, item, testToken, `{"enabled":false,"methods":[]}`)
	validate("update-sandbox-group-credential", w, http.StatusOK)
	w = do(t, h, http.MethodPatch, item, testToken, `{"domain":"127.0.0.1"}`)
	validate("update-sandbox-group-credential", w, http.StatusBadRequest)

	// 6. Restore v1 (200 with restoredFromVersionId).
	w = do(t, h, http.MethodPost, item+"/restore", testToken, `{"versionId":1}`)
	validate("restore-sandbox-group-credential", w, http.StatusOK)
	var restored sandbox.GroupCredential
	if err := json.Unmarshal(w.Body.Bytes(), &restored); err != nil || restored.Version.RestoredFromVersionID == nil || *restored.Version.RestoredFromVersionID != 1 {
		t.Fatalf("restore body = %s, err=%v", w.Body.String(), err)
	}

	// 7. List versions (200, includes both create/update without restoredFromVersionId and restore with restoredFromVersionId).
	w = do(t, h, http.MethodGet, item+"/versions", testToken, "")
	validate("list-sandbox-group-credential-versions", w, http.StatusOK)

	// 8. Delete (202 Accepted, empty body).
	w = do(t, h, http.MethodDelete, item, testToken, "")
	validate("delete-sandbox-group-credential", w, http.StatusAccepted)
	if w.Body.Len() != 0 {
		t.Errorf("delete body = %q, want empty", w.Body.String())
	}
}
