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

package sandbox

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
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestValidateCredentialRules(t *testing.T) {
	// Credential IDs.
	for _, good := range []string{"a", "my-inference-key", "Api_Key.v1~2", "UPPER.lower"} {
		if err := ValidateCredentialID(good); err != nil {
			t.Errorf("ValidateCredentialID(%q) = %v, want nil", good, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "rnd-123", "rnd-", "a/b", "a b", "a:b", "a@b", "a?b", "a#b"} {
		if err := ValidateCredentialID(bad); err == nil {
			t.Errorf("ValidateCredentialID(%q) = nil, want 400", bad)
		}
	}

	// Domains.
	if got, err := NormalizeCredentialDomain("API.Example.COM"); err != nil || got != "api.example.com" {
		t.Fatalf("NormalizeCredentialDomain uppercase = (%q, %v), want api.example.com", got, err)
	}
	for _, bad := range []string{
		"", "localhost", "intranet", "127.0.0.1", "8.8.8.8", "::1", "[::1]",
		"api.example.com:443", "https://api.example.com", "api.example.com/v1",
		"*.example.com", "a.*.example.com", "api.local", "db.internal",
		"api.bex-system.svc.cluster.local", "foo.svc", "my.localhost", "printer.home.arpa",
		"1.2.3.4", "a.1234",
	} {
		if got, err := NormalizeCredentialDomain(bad); err == nil {
			t.Errorf("NormalizeCredentialDomain(%q) = %q, want 400", bad, got)
		}
	}

	// Secret value bounds (never echoed in errors).
	if err := ValidateCredentialValue(strings.Repeat("x", MaxSandboxCredentialValueBytes)); err != nil {
		t.Errorf("5120-byte value rejected: %v", err)
	}
	secretMarker := "TOP_SECRET_VALUE_999"
	over := secretMarker + strings.Repeat("x", MaxSandboxCredentialValueBytes)
	if err := ValidateCredentialValue(over); err == nil || strings.Contains(err.Error(), secretMarker) {
		t.Errorf("over-5KiB value err = %v, want 400 without echoing secret", err)
	}
	if err := ValidateCredentialValue(""); err == nil {
		t.Error("empty value accepted, want 400")
	}
	if err := ValidateCredentialValue("bad\x00nul"); err == nil {
		t.Error("NUL byte accepted, want 400")
	}
	if err := ValidateCredentialValue(string([]byte{0xff, 0xfe})); err == nil {
		t.Error("invalid UTF-8 accepted, want 400")
	}

	// Methods.
	gotM, err := NormalizeCredentialMethods([]string{"post", "GET", "Post"})
	if err != nil || !slices.Equal(gotM, []string{"GET", "POST"}) {
		t.Fatalf("NormalizeCredentialMethods = (%v, %v), want [GET POST]", gotM, err)
	}
	for _, bad := range []string{"", "TRACE", "CONNECT", "CUSTOM"} {
		if _, err := NormalizeCredentialMethods([]string{bad}); err == nil {
			t.Errorf("NormalizeCredentialMethods(%q) = nil, want 400", bad)
		}
	}

	// Paths.
	gotP, err := NormalizeCredentialPaths([]string{"/v1/**", "/v1/*", "/", "/a/b/c"})
	if err != nil || len(gotP) != 4 {
		t.Fatalf("NormalizeCredentialPaths valid = (%v, %v)", gotP, err)
	}
	for _, bad := range []string{
		"", "v1/no-slash", "/a?query=1", "/a#frag", `/a\*`, "/a b", "/a\tb",
		"/a/foo*", "/a/*bar", "/a/***", "/a/*x*",
	} {
		if _, err := NormalizeCredentialPaths([]string{bad}); err == nil {
			t.Errorf("NormalizeCredentialPaths(%q) = nil, want 400", bad)
		}
	}

	// Injection specs.
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "Authorization", Template: "Bearer ${SECRET}"}, "tok"); err != nil {
		t.Errorf("valid header injection failed: %v", err)
	}
	for _, forbidden := range []string{
		"Host", "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"TE", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length",
		"X-Forwarded-For", "x-forwarded-host", "Forwarded", "Via",
	} {
		if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: forbidden}, "tok"); err == nil {
			t.Errorf("forbidden header %q accepted, want 400", forbidden)
		}
	}
	// Template 1-5 occurrences.
	five := strings.Repeat("${SECRET}-", 5)
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "X-Key", Template: five}, "tok"); err != nil {
		t.Errorf("5 placeholders rejected: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "X-Key", Template: "no-placeholder"}, "tok"); err == nil {
		t.Error("0 placeholders accepted, want 400")
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "X-Key", Template: strings.Repeat("${SECRET}", 6)}, "tok"); err == nil {
		t.Error("6 placeholders accepted, want 400")
	}
	// Basic auth rules.
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "basic"}, "alice:secret"); err != nil {
		t.Errorf("valid basic failed: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "basic", Template: "alice:${SECRET}"}, "secret"); err != nil {
		t.Errorf("valid basic template failed: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "basic", Name: "Authorization"}, "alice:secret"); err == nil {
		t.Error("basic with name accepted, want 400")
	}
	for _, badBasic := range []string{"no-colon", ":onlypass", "onlyuser:", "user:bad\r\nline"} {
		err := func() error {
			_, e := ValidateCredentialInjection(&GroupCredentialInjection{Type: "basic"}, badBasic)
			return e
		}()
		if err == nil || strings.Contains(err.Error(), badBasic) {
			t.Errorf("basic(%q) = %v, want 400 without echoing secret", badBasic, err)
		}
	}
	// Cookie & query rules.
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "cookie", Name: "session"}, "abc=123"); err != nil {
		t.Errorf("valid cookie failed: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "cookie", Name: "session"}, "abc;evil=1"); err == nil {
		t.Error("cookie with semicolon accepted, want 400")
	}
	// ADR123 F5: reject non-CRLF ASCII control characters in header/cookie values.
	for _, ctl := range []string{"val\x0bue", "val\x0cue", "val\x1fue", "val\x7fue"} {
		if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "X-Key"}, ctl); err == nil {
			t.Errorf("header value with control char %q accepted, want 400", ctl)
		}
		if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "cookie", Name: "session"}, ctl); err == nil {
			t.Errorf("cookie value with control char %q accepted, want 400", ctl)
		}
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "header", Name: "X-Key"}, "tok\tpart"); err != nil {
		t.Errorf("header with HTAB rejected: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "cookie", Name: "session"}, "tok\tpart"); err == nil {
		t.Error("cookie with HTAB accepted, want 400")
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "cookie", Name: ""}, "abc"); err == nil {
		t.Error("cookie without name accepted, want 400")
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "query", Name: "api_key"}, "abc"); err != nil {
		t.Errorf("valid query failed: %v", err)
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "query", Name: ""}, "abc"); err == nil {
		t.Error("query without name accepted, want 400")
	}
	if _, err := ValidateCredentialInjection(&GroupCredentialInjection{Type: "unknown"}, "abc"); err == nil {
		t.Error("unknown injection type accepted, want 400")
	}
}

// --- In-memory credential store for service/RBAC/REST unit tests ---

type memCredRecord struct {
	cred        store.SandboxCredential
	nextVersion int64
	versions    []store.SandboxCredentialVersion
}

type memCredStore struct {
	mu           sync.Mutex
	groups       map[string]store.SandboxGroup // groupID -> group
	salts        map[string][]byte
	creds        map[string]map[string]*memCredRecord // groupID -> credID -> rec
	failOnCommit error
	busyErr      bool
}

func newMemCredStore(ws, groupID string) *memCredStore {
	g := store.SandboxGroup{
		ID:               groupID,
		WorkspaceID:      ws,
		Name:             "Default",
		Region:           "oregon",
		ConcurrencyLimit: 100,
		IsDefault:        true,
	}
	return &memCredStore{
		groups: map[string]store.SandboxGroup{groupID: g},
		salts:  map[string][]byte{groupID: []byte("0123456789abcdef0123456789abcdef")},
		creds:  map[string]map[string]*memCredRecord{groupID: {}},
	}
}

// Implement minimal GroupStore methods needed by Service credential verbs.
func (m *memCredStore) CreateSandboxGroup(context.Context, string, string, string, string, int, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *memCredStore) EnsureDefaultSandboxGroup(context.Context, string, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *memCredStore) SandboxGroups(context.Context, string) ([]store.SandboxGroup, error) {
	return nil, errors.New("unused")
}
func (m *memCredStore) SandboxGroup(_ context.Context, workspaceID, id string) (store.SandboxGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[id]
	if !ok || g.WorkspaceID != workspaceID {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}
func (m *memCredStore) SandboxGroupWorkspace(_ context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return g.WorkspaceID, nil
}
func (m *memCredStore) SetSandboxGroupLimit(context.Context, string, string, int) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *memCredStore) SetDefaultSandboxGroup(context.Context, string, string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, errors.New("unused")
}
func (m *memCredStore) MarkSandboxGroupDeleting(_ context.Context, workspaceID, id string) (store.SandboxGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[id]
	if !ok || g.WorkspaceID != workspaceID {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}
func (m *memCredStore) ClaimStaleSandboxGroupDeletes(context.Context, time.Duration) ([]store.SandboxGroup, error) {
	return nil, nil
}
func (m *memCredStore) DeleteSandboxGroup(_ context.Context, workspaceID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[id]; ok && g.WorkspaceID == workspaceID {
		delete(m.groups, id)
		delete(m.creds, id)
	}
	return nil
}
func (m *memCredStore) SandboxGroupMembers(context.Context, string) (map[string]store.SandboxGroupMember, error) {
	return map[string]store.SandboxGroupMember{}, nil
}
func (m *memCredStore) WithSandboxAdmission(context.Context, string, func(store.SandboxAdmission) error) error {
	return nil
}
func (m *memCredStore) SetSandboxSlotLease(context.Context, string, string, *time.Time) error {
	return nil
}
func (m *memCredStore) ReleaseSandboxSlot(context.Context, string, string) error         { return nil }
func (m *memCredStore) PruneSandboxGroupMembers(context.Context, string, []string) error { return nil }

func (m *memCredStore) ListSandboxGroupCredentials(_ context.Context, workspaceID, groupID string) ([]store.SandboxCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.SandboxCredential, 0)
	for _, rec := range m.creds[groupID] {
		if rec.cred.WorkspaceID == workspaceID {
			out = append(out, rec.cred)
		}
	}
	slices.SortFunc(out, func(a, b store.SandboxCredential) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (m *memCredStore) GetSandboxGroupCredential(_ context.Context, workspaceID, groupID, credID string) (store.SandboxCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.creds[groupID][credID]
	if !ok || rec.cred.WorkspaceID != workspaceID {
		return store.SandboxCredential{}, store.ErrNotFound
	}
	return rec.cred, nil
}

func (m *memCredStore) ListSandboxGroupCredentialVersions(_ context.Context, workspaceID, groupID, credID string) ([]store.SandboxCredentialVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.creds[groupID][credID]
	if !ok || rec.cred.WorkspaceID != workspaceID {
		return nil, store.ErrNotFound
	}
	out := slices.Clone(rec.versions)
	slices.SortFunc(out, func(a, b store.SandboxCredentialVersion) int { return int(b.ID - a.ID) })
	return out, nil
}

func (m *memCredStore) CreateSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, prepare store.SandboxCredentialPrepareCreateFn) (store.SandboxCredential, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busyErr {
		return store.SandboxCredential{}, "", store.ErrSandboxCredentialBusy
	}
	if _, exists := m.creds[groupID][credID]; exists {
		return store.SandboxCredential{}, "", store.ErrSandboxCredentialExists
	}
	prepared, err := prepare(ctx, m.salts[groupID])
	if err != nil {
		return store.SandboxCredential{}, "", err
	}
	wroteRef := ""
	if prepared.OnBeforeCommit != nil {
		if err := prepared.OnBeforeCommit(ctx, prepared.SecretRef, false); err != nil {
			return store.SandboxCredential{}, "", err
		}
		wroteRef = prepared.SecretRef
	}
	if m.failOnCommit != nil {
		return store.SandboxCredential{}, wroteRef, m.failOnCommit
	}
	now := time.Now().UTC()
	cfg := store.CanonicalSandboxCredentialConfig(prepared.Config)
	v := store.SandboxCredentialVersion{
		ID: 1, UpdatedAt: now, UpdatedBy: prepared.UpdatedBy,
		Fingerprint: prepared.Fingerprint, SecretRef: prepared.SecretRef,
		Config: cfg, SecretBytes: prepared.SecretBytes,
	}
	c := store.SandboxCredential{
		WorkspaceID: workspaceID, SandboxGroupID: groupID, ID: credID,
		Config: cfg, CurrentVersion: v, CurrentSecretRef: prepared.SecretRef,
		CreatedAt: now, UpdatedAt: now,
	}
	if m.creds[groupID] == nil {
		m.creds[groupID] = map[string]*memCredRecord{}
	}
	m.creds[groupID][credID] = &memCredRecord{cred: c, nextVersion: 2, versions: []store.SandboxCredentialVersion{v}}
	return c, wroteRef, nil
}

func (m *memCredStore) UpdateSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, prepare store.SandboxCredentialPrepareUpdateFn) (store.SandboxCredential, []string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busyErr {
		return store.SandboxCredential{}, nil, "", store.ErrSandboxCredentialBusy
	}
	rec, ok := m.creds[groupID][credID]
	if !ok || rec.cred.WorkspaceID != workspaceID {
		return store.SandboxCredential{}, nil, "", store.ErrNotFound
	}
	prepared, err := prepare(ctx, m.salts[groupID], rec.cred)
	if err != nil {
		return store.SandboxCredential{}, nil, "", err
	}
	alreadyRetained := false
	for _, v := range rec.versions {
		if v.SecretRef == prepared.SecretRef {
			alreadyRetained = true
			break
		}
	}
	wroteNewRef := ""
	if prepared.OnBeforeCommit != nil {
		if err := prepared.OnBeforeCommit(ctx, prepared.SecretRef, alreadyRetained); err != nil {
			return store.SandboxCredential{}, nil, "", err
		}
		if !alreadyRetained {
			wroteNewRef = prepared.SecretRef
		}
	}
	if m.failOnCommit != nil {
		return store.SandboxCredential{}, nil, wroteNewRef, m.failOnCommit
	}
	now := time.Now().UTC()
	cfg := store.CanonicalSandboxCredentialConfig(prepared.Config)
	v := store.SandboxCredentialVersion{
		ID: rec.nextVersion, UpdatedAt: now, UpdatedBy: prepared.UpdatedBy,
		Fingerprint: prepared.Fingerprint, SecretRef: prepared.SecretRef,
		Config: cfg, SecretBytes: prepared.SecretBytes,
	}
	rec.nextVersion++
	rec.versions = append(rec.versions, v)
	evicted := m.pruneLocked(rec, v.Fingerprint)
	rec.cred.Config = cfg
	rec.cred.CurrentVersion = v
	rec.cred.CurrentSecretRef = v.SecretRef
	rec.cred.UpdatedAt = now
	return rec.cred, evicted, wroteNewRef, nil
}

func (m *memCredStore) RestoreSandboxGroupCredential(_ context.Context, workspaceID, groupID, credID string, targetVersionID int64, updatedBy string) (store.SandboxCredential, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.creds[groupID][credID]
	if !ok || rec.cred.WorkspaceID != workspaceID {
		return store.SandboxCredential{}, nil, store.ErrNotFound
	}
	var target *store.SandboxCredentialVersion
	for i := range rec.versions {
		if rec.versions[i].ID == targetVersionID {
			target = &rec.versions[i]
			break
		}
	}
	if target == nil {
		return store.SandboxCredential{}, nil, store.ErrSandboxCredentialVersionNotFound
	}
	now := time.Now().UTC()
	from := targetVersionID
	v := store.SandboxCredentialVersion{
		ID: rec.nextVersion, UpdatedAt: now, UpdatedBy: updatedBy,
		Fingerprint: target.Fingerprint, RestoredFromVersionID: &from,
		SecretRef: target.SecretRef, Config: target.Config, SecretBytes: target.SecretBytes,
	}
	rec.nextVersion++
	rec.versions = append(rec.versions, v)
	evicted := m.pruneLocked(rec, v.Fingerprint)
	rec.cred.Config = target.Config
	rec.cred.CurrentVersion = v
	rec.cred.CurrentSecretRef = v.SecretRef
	rec.cred.UpdatedAt = now
	return rec.cred, evicted, nil
}

func (m *memCredStore) pruneLocked(rec *memCredRecord, currentFP string) []string {
	type fpLast struct {
		fp  string
		vid int64
	}
	priorMax := map[string]int64{}
	for _, v := range rec.versions {
		if v.Fingerprint == currentFP {
			continue
		}
		if v.ID > priorMax[v.Fingerprint] {
			priorMax[v.Fingerprint] = v.ID
		}
	}
	priors := make([]fpLast, 0, len(priorMax))
	for fp, vid := range priorMax {
		priors = append(priors, fpLast{fp, vid})
	}
	slices.SortFunc(priors, func(a, b fpLast) int { return int(b.vid - a.vid) })
	keptFP := map[string]bool{currentFP: true}
	for i := 0; i < len(priors) && i < store.SandboxCredentialMaxPriorConfigs; i++ {
		keptFP[priors[i].fp] = true
	}
	var kept []store.SandboxCredentialVersion
	keptRefs := map[string]bool{}
	deletedRefs := map[string]bool{}
	for _, v := range rec.versions {
		if keptFP[v.Fingerprint] {
			kept = append(kept, v)
			keptRefs[v.SecretRef] = true
		} else {
			deletedRefs[v.SecretRef] = true
		}
	}
	rec.versions = kept
	var evicted []string
	for ref := range deletedRefs {
		if !keptRefs[ref] {
			evicted = append(evicted, ref)
		}
	}
	slices.Sort(evicted)
	return evicted
}

func (m *memCredStore) DeleteSandboxGroupCredential(ctx context.Context, workspaceID, groupID, credID string, onBeforeCommit func(ctx context.Context, secretRefs []string) error) ([]string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok || g.WorkspaceID != workspaceID {
		return nil, false, store.ErrNotFound
	}
	var refs []string
	rec, exists := m.creds[groupID][credID]
	if exists {
		seen := map[string]bool{}
		for _, v := range rec.versions {
			if !seen[v.SecretRef] {
				seen[v.SecretRef] = true
				refs = append(refs, v.SecretRef)
			}
		}
	}
	if onBeforeCommit != nil {
		if err := onBeforeCommit(ctx, refs); err != nil {
			return nil, false, err
		}
	}
	delete(m.creds[groupID], credID)
	return refs, exists, nil
}

func (m *memCredStore) ListSandboxGroupCredentialSecretRefs(_ context.Context, workspaceID, groupID string) ([]store.SandboxCredentialSecretRefEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.SandboxCredentialSecretRefEntry
	for gid, byCred := range m.creds {
		if groupID != "" && gid != groupID {
			continue
		}
		for cid, rec := range byCred {
			if rec.cred.WorkspaceID != workspaceID {
				continue
			}
			for _, v := range rec.versions {
				out = append(out, store.SandboxCredentialSecretRefEntry{
					SandboxGroupID: gid, CredentialID: cid, SecretRef: v.SecretRef,
				})
			}
		}
	}
	return out, nil
}

type roleAuthzChecker struct {
	roles map[string]string // subject -> "viewer" | "developer" | "admin"
}

func (c roleAuthzChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	role := c.roles[strings.TrimPrefix(subject, "user:")]
	switch relation {
	case core.RelCanView:
		return role == "viewer" || role == "developer" || role == "admin", nil
	case core.RelCanCreate:
		return role == "developer" || role == "admin", nil
	case core.RelCanManage:
		return role == "admin", nil
	default:
		return false, nil
	}
}

type credWorkspaceResolver struct {
	members map[string][]string // subject -> workspaces
}

func (r credWorkspaceResolver) Tenant(_ context.Context, id core.Identity) (string, bool) {
	if ws := r.members[id.Subject]; len(ws) > 0 {
		return ws[0], true
	}
	return "", false
}

func (r credWorkspaceResolver) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return slices.Contains(r.members[id.Subject], tenantID), nil
}

type credAuditSpy struct {
	events []core.AuditEvent
}

func (s *credAuditSpy) Record(_ context.Context, ev core.AuditEvent) error {
	s.events = append(s.events, ev)
	return nil
}

func TestGroupCredentialsSplitRBACAndLifecycle(t *testing.T) {
	ws := ids.New(ids.Workspace)
	otherWS := ids.New(ids.Workspace)
	groupID := ids.New(ids.SandboxGroup)
	foreignGroupID := ids.New(ids.SandboxGroup)

	st := newMemCredStore(ws, groupID)
	st.groups[foreignGroupID] = store.SandboxGroup{ID: foreignGroupID, WorkspaceID: otherWS, Name: "Other", Region: "oregon", ConcurrencyLimit: 10}
	kv := newMemCredentialKV()
	audit := &credAuditSpy{}
	base := &core.Base{
		Authz: roleAuthzChecker{roles: map[string]string{
			"viewer-1": "viewer",
			"dev-1":    "developer",
			"admin-1":  "admin",
		}},
		Workspace: credWorkspaceResolver{members: map[string][]string{
			"viewer-1": {ws},
			"dev-1":    {ws},
			"admin-1":  {ws},
		}},
		Audit: audit,
	}
	svc := &Service{
		Base:            base,
		Groups:          st,
		Credentials:     st,
		CredentialVault: NewCredentialVault(kv),
	}

	ctxAs := func(sub string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: sub, Method: core.MethodSession, Human: true})
	}
	adminCtx := ctxAs("admin-1")
	devCtx := ctxAs("dev-1")
	viewerCtx := ctxAs("viewer-1")

	secretVal := "sk-live-super-secret-42"
	createReq := CreateGroupCredentialRequest{
		ID:        "openai",
		Domain:    "API.OpenAI.com",
		Value:     secretVal,
		Methods:   []string{"post"},
		Paths:     []string{"/v1/**"},
		Injection: &GroupCredentialInjection{Type: "header", Name: "Authorization", Template: "Bearer ${SECRET}"},
	}

	// Viewer & Developer cannot create (403).
	if _, err := svc.CreateGroupCredential(viewerCtx, groupID, createReq); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("viewer create = %v, want ErrForbidden", err)
	}
	if _, err := svc.CreateGroupCredential(devCtx, groupID, createReq); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer create = %v, want ErrForbidden", err)
	}

	// Foreign group answers 404.
	if _, err := svc.CreateGroupCredential(adminCtx, foreignGroupID, createReq); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("foreign group create = %v, want ErrNotFound", err)
	}

	// Admin creates credential.
	created, err := svc.CreateGroupCredential(adminCtx, groupID, createReq)
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if created.Domain != "api.openai.com" || !created.Enabled || created.Version.ID != 1 || !slices.Equal(created.Methods, []string{"POST"}) {
		t.Fatalf("unexpected created cred: %+v", created)
	}
	// Secret must never appear in marshaled response or audit events.
	rawResp, _ := json.Marshal(created)
	if strings.Contains(string(rawResp), secretVal) {
		t.Fatalf("create response leaked secret: %s", rawResp)
	}

	// Viewer can read list, get, versions.
	list, err := svc.ListGroupCredentials(viewerCtx, groupID)
	if err != nil || len(list) != 1 || list[0].ID != "openai" {
		t.Fatalf("viewer list = (%+v, %v)", list, err)
	}
	if _, err := svc.GetGroupCredential(viewerCtx, groupID, "openai"); err != nil {
		t.Fatalf("viewer get: %v", err)
	}
	if vers, err := svc.ListGroupCredentialVersions(viewerCtx, groupID, "openai"); err != nil || len(vers) != 1 {
		t.Fatalf("viewer versions = (%+v, %v)", vers, err)
	}

	// Developer CAN patch injection/methods/paths/enabled.
	emptyMethods := []string{}
	disabled := false
	patched, err := svc.UpdateGroupCredential(devCtx, groupID, "openai", UpdateGroupCredentialRequest{
		Enabled:   &disabled,
		Methods:   &emptyMethods,
		Injection: &GroupCredentialInjection{Type: "header", Name: "X-API-Key"}, // template omitted -> defaults to ${SECRET}
	})
	if err != nil {
		t.Fatalf("developer patch non-admin fields: %v", err)
	}
	if patched.Version.ID != 2 || patched.Enabled || len(patched.Methods) != 0 || patched.Injection.Name != "X-API-Key" || patched.Injection.Template != "" {
		t.Fatalf("unexpected patched cred: %+v", patched)
	}

	// Developer trying to switch injection to basic when stored secret lacks username:password -> 400 without leaking secret.
	if _, err := svc.UpdateGroupCredential(devCtx, groupID, "openai", UpdateGroupCredentialRequest{
		Injection: &GroupCredentialInjection{Type: "basic"},
	}); !errors.Is(err, core.ErrBadRequest) || strings.Contains(err.Error(), secretVal) {
		t.Fatalf("developer basic patch on non-colon secret = %v, want 400 without secret leak", err)
	}

	// Developer CANNOT patch domain or value (403).
	newDomain := "api.anthropic.com"
	if _, err := svc.UpdateGroupCredential(devCtx, groupID, "openai", UpdateGroupCredentialRequest{Domain: &newDomain}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer patch domain = %v, want ErrForbidden", err)
	}
	newSecret := "sk-new-secret"
	if _, err := svc.UpdateGroupCredential(devCtx, groupID, "openai", UpdateGroupCredentialRequest{Value: &newSecret}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer patch value = %v, want ErrForbidden", err)
	}

	// Developer CANNOT delete or restore (403).
	if err := svc.DeleteGroupCredential(devCtx, groupID, "openai"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer delete = %v, want ErrForbidden", err)
	}
	if _, err := svc.RestoreGroupCredential(devCtx, groupID, "openai", 1); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer restore = %v, want ErrForbidden", err)
	}

	// Admin patching same secret value STILL mints a new version (Render rule).
	sameSecret := secretVal
	v3, err := svc.UpdateGroupCredential(adminCtx, groupID, "openai", UpdateGroupCredentialRequest{Value: &sameSecret})
	if err != nil || v3.Version.ID != 3 {
		t.Fatalf("admin patch same value = (%+v, %v), want v3", v3, err)
	}

	// No-op patch without value returns current without minting a new version.
	noop, err := svc.UpdateGroupCredential(devCtx, groupID, "openai", UpdateGroupCredentialRequest{})
	if err != nil || noop.Version.ID != 3 {
		t.Fatalf("noop patch = (%+v, %v), want v3 unchanged", noop, err)
	}

	// Admin restores v1 -> v4 with restoredFromVersionId=1 and matching fingerprint.
	restored, err := svc.RestoreGroupCredential(adminCtx, groupID, "openai", 1)
	if err != nil {
		t.Fatalf("admin restore: %v", err)
	}
	if restored.Version.ID != 4 || restored.Version.RestoredFromVersionID == nil || *restored.Version.RestoredFromVersionID != 1 {
		t.Fatalf("restored = %+v, want v4 restoredFrom=1", restored.Version)
	}
	if restored.Version.Fingerprint != created.Version.Fingerprint {
		t.Fatalf("restored fp %s != v1 fp %s", restored.Version.Fingerprint, created.Version.Fingerprint)
	}

	// Admin deletes credential -> OpenBao secret purged; repeat delete succeeds idempotently.
	if err := svc.DeleteGroupCredential(adminCtx, groupID, "openai"); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	if kv.count() != 0 {
		t.Fatalf("after delete, OpenBao still holds %d entries", kv.count())
	}
	if err := svc.DeleteGroupCredential(adminCtx, groupID, "openai"); err != nil {
		t.Fatalf("repeat delete: %v", err)
	}

	// Audit events recorded for write verbs and denials, and contain zero secret values.
	auditJSON, _ := json.Marshal(audit.events)
	if strings.Contains(string(auditJSON), secretVal) || strings.Contains(string(auditJSON), newSecret) {
		t.Fatalf("audit trail leaked secret material: %s", auditJSON)
	}
	var allowedVerbs []string
	for _, ev := range audit.events {
		if ev.Outcome == core.AuditAllowed {
			allowedVerbs = append(allowedVerbs, ev.Verb)
		}
	}
	for _, want := range []string{"sandbox.CreateGroupCredential", "sandbox.UpdateGroupCredential", "sandbox.RestoreGroupCredential", "sandbox.DeleteGroupCredential"} {
		if !slices.Contains(allowedVerbs, want) {
			t.Errorf("missing audit verb %q in %v", want, allowedVerbs)
		}
	}
}

func TestCreateCredentialRollbackCleansOpenBaoOnDBCommitFailure(t *testing.T) {
	ws := ids.New(ids.Workspace)
	groupID := ids.New(ids.SandboxGroup)
	st := newMemCredStore(ws, groupID)
	st.failOnCommit = errors.New("simulated commit abort")
	kv := newMemCredentialKV()
	svc := &Service{
		Base: &core.Base{
			Authz:     roleAuthzChecker{roles: map[string]string{"admin-1": "admin"}},
			Workspace: credWorkspaceResolver{members: map[string][]string{"admin-1": {ws}}},
		},
		Groups:          st,
		Credentials:     st,
		CredentialVault: NewCredentialVault(kv),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: core.MethodSession, Human: true})
	_, err := svc.CreateGroupCredential(ctx, groupID, CreateGroupCredentialRequest{
		ID:        "will-fail",
		Domain:    "api.example.com",
		Value:     "ephemeral-secret",
		Injection: &GroupCredentialInjection{Type: "header", Name: "X-Key"},
	})
	if err == nil {
		t.Fatal("expected commit failure, got nil")
	}
	if kv.count() != 0 {
		t.Fatalf("failed DB commit left %d orphaned OpenBao secret(s)", kv.count())
	}
}

func TestCredentialRESTStatusCodesAndContentionHeaders(t *testing.T) {
	ws := ids.New(ids.Workspace)
	groupID := ids.New(ids.SandboxGroup)
	st := newMemCredStore(ws, groupID)
	kv := newMemCredentialKV()
	svc := &Service{
		Base: &core.Base{
			Authz:     roleAuthzChecker{roles: map[string]string{"admin-1": "admin"}},
			Workspace: credWorkspaceResolver{members: map[string][]string{"admin-1": {ws}}},
		},
		Groups:          st,
		Credentials:     st,
		CredentialVault: NewCredentialVault(kv),
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(core.WithIdentity(r.Context(), core.Identity{Subject: "admin-1", Method: core.MethodSession, Human: true}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}

	// Empty list -> 200 [].
	w := req(http.MethodGet, "/v1/sandbox-groups/"+groupID+"/credentials", "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty list = %d %q, want 200 []", w.Code, w.Body.String())
	}

	// Create -> 201.
	w = req(http.MethodPost, "/v1/sandbox-groups/"+groupID+"/credentials",
		`{"id":"k1","domain":"api.example.com","value":"s1","injection":{"type":"header","name":"X-Key"}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s, want 201", w.Code, w.Body.String())
	}

	// Unknown JSON field rejected strictly (400).
	w = req(http.MethodPost, "/v1/sandbox-groups/"+groupID+"/credentials",
		`{"id":"k2","domain":"api.example.com","value":"s1","injection":{"type":"header","name":"X-Key"},"bogus":1}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", w.Code)
	}

	// Contention -> 429 with Retry-After.
	st.busyErr = true
	w = req(http.MethodPatch, "/v1/sandbox-groups/"+groupID+"/credentials/k1", `{"enabled":false}`)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("contention = %d Retry-After=%q body=%s, want 429 with Retry-After", w.Code, w.Header().Get("Retry-After"), w.Body.String())
	}
	st.busyErr = false

	// Delete -> 202 Accepted with empty body (and idempotent repeat -> 202).
	w = req(http.MethodDelete, "/v1/sandbox-groups/"+groupID+"/credentials/k1", "")
	if w.Code != http.StatusAccepted || w.Body.Len() != 0 {
		t.Fatalf("delete = %d %q, want 202 empty", w.Code, w.Body.String())
	}
	w = req(http.MethodDelete, "/v1/sandbox-groups/"+groupID+"/credentials/k1", "")
	if w.Code != http.StatusAccepted || w.Body.Len() != 0 {
		t.Fatalf("repeat delete = %d %q, want 202 empty", w.Code, w.Body.String())
	}
}
