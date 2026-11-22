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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeBrokerStore struct {
	mu      sync.Mutex
	records map[string]store.SandboxRecord
	members map[string]store.SandboxGroupMember
	groups  map[string]store.SandboxGroup
	creds   map[string][]store.SandboxCredential
	err     error
}

func newFakeBrokerStore() *fakeBrokerStore {
	return &fakeBrokerStore{
		records: map[string]store.SandboxRecord{},
		members: map[string]store.SandboxGroupMember{},
		groups:  map[string]store.SandboxGroup{},
		creds:   map[string][]store.SandboxCredential{},
	}
}

func (f *fakeBrokerStore) GetSandboxRecord(_ context.Context, ws, id string) (store.SandboxRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return store.SandboxRecord{}, f.err
	}
	r, ok := f.records[ws+"|"+id]
	if !ok {
		return store.SandboxRecord{}, store.ErrNotFound
	}
	return r, nil
}

func (f *fakeBrokerStore) GetSandboxGroupMember(_ context.Context, ws, id string) (store.SandboxGroupMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return store.SandboxGroupMember{}, f.err
	}
	m, ok := f.members[ws+"|"+id]
	if !ok {
		return store.SandboxGroupMember{}, store.ErrNotFound
	}
	return m, nil
}

func (f *fakeBrokerStore) SandboxGroup(_ context.Context, ws, id string) (store.SandboxGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return store.SandboxGroup{}, f.err
	}
	g, ok := f.groups[ws+"|"+id]
	if !ok {
		return store.SandboxGroup{}, store.ErrNotFound
	}
	return g, nil
}

func (f *fakeBrokerStore) ListSandboxGroupCredentials(_ context.Context, ws, gid string) ([]store.SandboxCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]store.SandboxCredential(nil), f.creds[ws+"|"+gid]...), nil
}

type fakeBrokerNonce struct {
	mu   sync.Mutex
	seen map[string]struct{}
	err  error
}

func (n *fakeBrokerNonce) ClaimShellNonce(_ context.Context, nonce string, _ time.Time) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil {
		return false, n.err
	}
	if n.seen == nil {
		n.seen = map[string]struct{}{}
	}
	if _, ok := n.seen[nonce]; ok {
		return false, nil
	}
	n.seen[nonce] = struct{}{}
	return true, nil
}

type fakeBrokerKV struct {
	mu   sync.Mutex
	data map[string]map[string]string
	err  error
}

func (k *fakeBrokerKV) Put(_ context.Context, path string, values map[string]string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.err != nil {
		return k.err
	}
	if k.data == nil {
		k.data = map[string]map[string]string{}
	}
	cp := make(map[string]string, len(values))
	for kk, vv := range values {
		cp[kk] = vv
	}
	k.data[path] = cp
	return nil
}

func (k *fakeBrokerKV) Get(_ context.Context, path string) (map[string]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.err != nil {
		return nil, k.err
	}
	return k.data[path], nil
}

func (k *fakeBrokerKV) Delete(_ context.Context, path string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.err != nil {
		return k.err
	}
	delete(k.data, path)
	return nil
}

func (k *fakeBrokerKV) List(_ context.Context, _ string) ([]string, error) { return nil, nil }

func seedBrokerFixture(t *testing.T, policy *NetworkPolicy) (*fakeBrokerStore, *fakeBrokerKV, *CredentialVault) {
	t.Helper()
	st := newFakeBrokerStore()
	kv := &fakeBrokerKV{}
	vault := NewCredentialVault(kv)
	view, err := json.Marshal(Sandbox{
		ID:             "sbx-12345",
		SandboxGroupID: "sbg-12345",
		NetworkPolicy:  policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	st.groups["tea-1|sbg-12345"] = store.SandboxGroup{ID: "sbg-12345", WorkspaceID: "tea-1"}
	st.members["tea-1|sbx-12345"] = store.SandboxGroupMember{GroupID: "sbg-12345"}
	st.records["tea-1|sbx-12345"] = store.SandboxRecord{
		WorkspaceID: "tea-1",
		SandboxID:   "sbx-12345",
		View:        view,
	}
	if err := vault.Put(context.Background(), "tea-1", "sbg-12345", "cred-b", "ref-b", "super-secret-b"); err != nil {
		t.Fatal(err)
	}
	if err := vault.Put(context.Background(), "tea-1", "sbg-12345", "cred-a", "ref-a", "super-secret-a"); err != nil {
		t.Fatal(err)
	}
	st.creds["tea-1|sbg-12345"] = []store.SandboxCredential{
		{
			ID:               "cred-b",
			CurrentSecretRef: "ref-b",
			CurrentVersion:   store.SandboxCredentialVersion{ID: 2, Fingerprint: "fp-b"},
			Config: store.SandboxCredentialConfig{
				Domain:    "api.example.com",
				Enabled:   true,
				Methods:   []string{"GET"},
				Paths:     []string{"/v1/**"},
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "Authorization", Template: "Bearer ${SECRET}"},
			},
		},
		{
			ID:               "cred-disabled",
			CurrentSecretRef: "ref-disabled",
			CurrentVersion:   store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-disabled"},
			Config: store.SandboxCredentialConfig{
				Domain:    "disabled.example.com",
				Enabled:   false,
				Injection: store.SandboxCredentialInjection{Type: "header", Name: "X-Api-Key"},
			},
		},
		{
			ID:               "cred-a",
			CurrentSecretRef: "ref-a",
			CurrentVersion:   store.SandboxCredentialVersion{ID: 1, Fingerprint: "fp-a"},
			Config: store.SandboxCredentialConfig{
				Domain:    "api.example.com",
				Enabled:   true,
				Injection: store.SandboxCredentialInjection{Type: "query", Name: "key"},
			},
		},
	}
	return st, kv, vault
}

func TestBrokerLookupHandlerAndClient(t *testing.T) {
	policy := &NetworkPolicy{
		Default: NetworkPolicyAllowList,
		Rules:   []NetworkPolicyRule{{Domain: "api.example.com", Protocol: NetworkPolicyProtocolHTTPS}},
	}
	st, kv, vault := seedBrokerFixture(t, policy)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	secret := []byte("0123456789abcdef0123456789abcdef")
	nonce := &fakeBrokerNonce{}

	handler := &BrokerLookupHandler{
		Secret:   secret,
		Resolver: &BrokerLookupResolver{Store: st, Vault: vault, Now: clock},
		Nonce:    nonce,
		Now:      clock,
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	var cacheHits, cacheMisses int
	client := &BrokerLookupClient{
		URL:      srv.URL,
		Secret:   secret,
		CacheTTL: 30 * time.Second, // must be clamped to <= 5s
		Now:      clock,
		OnCacheEvent: func(hit bool) {
			if hit {
				cacheHits++
			} else {
				cacheMisses++
			}
		},
	}
	req := BrokerLookupRequest{
		WorkspaceID:    "tea-1",
		SandboxID:      "sbx-12345",
		SandboxGroupID: "sbg-12345",
		PodName:        "sbx-12345-0",
		PodUID:         "uid-1",
	}

	resp, err := client.Lookup(context.Background(), req)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if resp.NetworkPolicy.Default != NetworkPolicyAllowList || len(resp.NetworkPolicy.Rules) != 1 || resp.NetworkPolicy.Rules[0].Domain != "api.example.com" {
		t.Fatalf("unexpected networkPolicy: %+v", resp.NetworkPolicy)
	}
	if len(resp.Credentials) != 2 {
		t.Fatalf("got %d creds, want 2 (disabled excluded)", len(resp.Credentials))
	}
	// Sorted lexicographically ascending by ID.
	if resp.Credentials[0].ID != "cred-a" || resp.Credentials[0].Value != "super-secret-a" {
		t.Fatalf("first cred = %+v, want cred-a", resp.Credentials[0])
	}
	if resp.Credentials[1].ID != "cred-b" || resp.Credentials[1].Value != "super-secret-b" {
		t.Fatalf("second cred = %+v, want cred-b", resp.Credentials[1])
	}
	if resp.VersionStamp == "" {
		t.Fatal("expected non-empty VersionStamp")
	}
	// Redaction check on String/GoString.
	if strings.Contains(fmt.Sprintf("%v %#v", resp.Credentials[0], resp.Credentials[0]), "super-secret-a") {
		t.Fatal("BrokerCredential formatting leaked secret value")
	}

	// Within 5s TTL: served from cache (no second HTTP call).
	now = now.Add(4 * time.Second)
	resp2, err := client.Lookup(context.Background(), req)
	if err != nil {
		t.Fatalf("cached Lookup: %v", err)
	}
	if hits.Load() != 1 || cacheHits != 1 || cacheMisses != 1 {
		t.Fatalf("expected cache hit: hits=%d cacheHits=%d cacheMisses=%d", hits.Load(), cacheHits, cacheMisses)
	}
	if resp2.VersionStamp != resp.VersionStamp {
		t.Fatalf("cached VersionStamp mismatch")
	}

	// Past 5s clamped TTL (even though 30s was requested): re-fetches live state.
	now = now.Add(2 * time.Second)
	st.mu.Lock()
	st.creds["tea-1|sbg-12345"][2].Config.Enabled = false // disable cred-a
	st.mu.Unlock()
	resp3, err := client.Lookup(context.Background(), req)
	if err != nil {
		t.Fatalf("refreshed Lookup: %v", err)
	}
	if hits.Load() != 2 || len(resp3.Credentials) != 1 || resp3.Credentials[0].ID != "cred-b" {
		t.Fatalf("expected cred-a disabled after TTL: hits=%d creds=%v", hits.Load(), resp3.Credentials)
	}
	if resp3.VersionStamp == resp.VersionStamp {
		t.Fatal("expected VersionStamp to change after disabling cred-a")
	}

	// OpenBao failure fails closed with 503 and is NOT cached.
	now = now.Add(6 * time.Second)
	kv.mu.Lock()
	kv.err = errors.New("vault sealed")
	kv.mu.Unlock()
	_, err = client.Lookup(context.Background(), req)
	if !errors.Is(err, ErrBrokerUnavailable) {
		t.Fatalf("vault failure err = %v, want ErrBrokerUnavailable", err)
	}
	// Recovering OpenBao succeeds immediately on next call (503 was not cached).
	kv.mu.Lock()
	kv.err = nil
	kv.mu.Unlock()
	if _, err := client.Lookup(context.Background(), req); err != nil {
		t.Fatalf("expected immediate recovery after vault unseals: %v", err)
	}
}

func TestBrokerLookupSecurityRejections(t *testing.T) {
	st, _, vault := seedBrokerFixture(t, &NetworkPolicy{Default: NetworkPolicyAllowAll})
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	secret := []byte("0123456789abcdef0123456789abcdef")
	nonce := &fakeBrokerNonce{}

	handler := &BrokerLookupHandler{
		Secret:   secret,
		Resolver: &BrokerLookupResolver{Store: st, Vault: vault, Now: clock},
		Nonce:    nonce,
		Now:      clock,
	}

	postSigned := func(t *testing.T, body []byte, macInput []byte, signSecret []byte, tsTime time.Time) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, InternalBrokerLookupPath, bytes.NewReader(body))
		if signSecret != nil {
			ts, sig := agentsession.Sign(signSecret, macInput, tsTime)
			req.Header.Set(agentsession.TimestampHeader, ts)
			req.Header.Set(agentsession.SignatureHeader, sig)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	validBody, _ := json.Marshal(BrokerLookupRequest{
		Kind:           BrokerLookupKind,
		WorkspaceID:    "tea-1",
		SandboxID:      "sbx-12345",
		SandboxGroupID: "sbg-12345",
		PodName:        "sbx-12345-0",
		PodUID:         "uid-1",
		Nonce:          "nonce-1",
	})

	// 1. GET rejected 405.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, InternalBrokerLookupPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}

	// 2. Unsigned / wrong secret / wrong MAC domain (bare body) / expired timestamp -> 401.
	if r := postSigned(t, validBody, brokerMACInput(validBody), nil, now); r.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status = %d, want 401", r.Code)
	}
	if r := postSigned(t, validBody, brokerMACInput(validBody), []byte("wrong-secret"), now); r.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret status = %d, want 401", r.Code)
	}
	if r := postSigned(t, validBody, validBody, secret, now); r.Code != http.StatusUnauthorized {
		t.Fatalf("bare-body MAC domain status = %d, want 401", r.Code)
	}
	if r := postSigned(t, validBody, brokerMACInput(validBody), secret, now.Add(-2*time.Minute)); r.Code != http.StatusUnauthorized {
		t.Fatalf("expired timestamp status = %d, want 401", r.Code)
	}

	// 3. Valid request succeeds once with Cache-Control: no-store, replay rejected 401.
	r1 := postSigned(t, validBody, brokerMACInput(validBody), secret, now)
	if r1.Code != http.StatusOK {
		t.Fatalf("valid status = %d, want 200", r1.Code)
	}
	if cc := r1.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	if r2 := postSigned(t, validBody, brokerMACInput(validBody), secret, now); r2.Code != http.StatusUnauthorized {
		t.Fatalf("replayed nonce status = %d, want 401", r2.Code)
	}

	// 4. Unknown JSON field rejected 400.
	unknownField := []byte(`{"kind":"sandbox-broker-lookup","workspaceId":"tea-1","sandboxId":"sbx-12345","sandboxGroupId":"sbg-12345","podName":"p","podUid":"u","nonce":"n2","extra":1}`)
	if r := postSigned(t, unknownField, brokerMACInput(unknownField), secret, now); r.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400", r.Code)
	}

	// 5. Rejections in Resolver: agent session, mismatched group, foreign workspace, ended sandbox, expired sandbox, deleting group.
	cases := []struct {
		name   string
		mutate func()
		req    BrokerLookupRequest
		want   error
	}{
		{
			name: "agent session rejected",
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u", AgentSession: true,
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "non-sbx id rejected",
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "ags-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "mismatched group rejected",
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-other",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "foreign workspace rejected",
			req: BrokerLookupRequest{
				WorkspaceID: "tea-other", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "ended sandbox rejected",
			mutate: func() {
				ended := now.Add(-time.Second)
				rec := st.records["tea-1|sbx-12345"]
				rec.TerminatedAt = &ended
				st.records["tea-1|sbx-12345"] = rec
			},
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "expired sandbox rejected",
			mutate: func() {
				exp := now.Add(-time.Second)
				rec := st.records["tea-1|sbx-12345"]
				rec.TerminatedAt = nil
				rec.ExpiresAt = &exp
				st.records["tea-1|sbx-12345"] = rec
			},
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
		{
			name: "deleting or absent group rejected",
			mutate: func() {
				rec := st.records["tea-1|sbx-12345"]
				rec.TerminatedAt = nil
				rec.ExpiresAt = nil
				st.records["tea-1|sbx-12345"] = rec
				delete(st.groups, "tea-1|sbg-12345")
			},
			req: BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: "u",
			},
			want: ErrBrokerForbidden,
		},
	}

	resolver := &BrokerLookupResolver{Store: st, Vault: vault, Now: clock}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mutate != nil {
				tc.mutate()
			}
			_, err := resolver.Lookup(context.Background(), tc.req)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

type fakeBrokerEgress struct {
	rules []string
	err   error
}

func (f fakeBrokerEgress) Apply(context.Context, string, string, []string) error { return nil }
func (f fakeBrokerEgress) Delete(context.Context, string, string) error          { return nil }
func (f fakeBrokerEgress) Rules(context.Context, string, string) ([]string, error) {
	return f.rules, f.err
}

func TestBrokerLookupPolicyFallbackAndClientCacheBounds(t *testing.T) {
	// Missing NetworkPolicy on record fails closed to deny-all.
	st, _, vault := seedBrokerFixture(t, nil)
	res := &BrokerLookupResolver{Store: st, Vault: vault}
	got, err := res.Lookup(context.Background(), BrokerLookupRequest{
		WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345", PodName: "p", PodUID: "u",
	})
	if err != nil {
		t.Fatalf("Lookup nil policy: %v", err)
	}
	if got.NetworkPolicy.Default != NetworkPolicyDenyAll {
		t.Fatalf("nil policy default = %q, want deny-all", got.NetworkPolicy.Default)
	}

	// allow-list record with empty rules falls back to SandboxEgress.Rules.
	st2, _, vault2 := seedBrokerFixture(t, &NetworkPolicy{Default: NetworkPolicyAllowList})
	res2 := &BrokerLookupResolver{
		Store:         st2,
		Vault:         vault2,
		SandboxEgress: fakeBrokerEgress{rules: []string{"b.example.com", "a.example.com"}},
	}
	got2, err := res2.Lookup(context.Background(), BrokerLookupRequest{
		WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345", PodName: "p", PodUID: "u",
	})
	if err != nil {
		t.Fatalf("Lookup allow-list fallback: %v", err)
	}
	if len(got2.NetworkPolicy.Rules) != 2 || got2.NetworkPolicy.Rules[0].Domain != "a.example.com" {
		t.Fatalf("fallback rules = %+v", got2.NetworkPolicy.Rules)
	}

	// Concurrent client lookups + bounded eviction at MaxCacheEntries=3.
	secret := []byte("0123456789abcdef0123456789abcdef")
	srv := httptest.NewServer(&BrokerLookupHandler{
		Secret:   secret,
		Resolver: res2,
		Nonce:    &fakeBrokerNonce{},
	})
	defer srv.Close()

	client := &BrokerLookupClient{
		URL:             srv.URL,
		Secret:          secret,
		CacheTTL:        5 * time.Second,
		MaxCacheEntries: 3,
	}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, callErr := client.Lookup(context.Background(), BrokerLookupRequest{
				WorkspaceID: "tea-1", SandboxID: "sbx-12345", SandboxGroupID: "sbg-12345",
				PodName: "p", PodUID: fmt.Sprintf("uid-%d", i%6),
			})
			if callErr != nil {
				t.Errorf("concurrent Lookup(%d): %v", i, callErr)
			}
		}(i)
	}
	wg.Wait()
	client.mu.Lock()
	cacheLen := len(client.cache)
	client.mu.Unlock()
	if cacheLen > 3 {
		t.Fatalf("client cache grew past MaxCacheEntries=3: got %d", cacheLen)
	}
}
