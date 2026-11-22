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
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// InternalBrokerLookupPath is mounted ONLY on bex-api's cluster-internal
	// :8091 listener (never on :8090). It serves the Pod-bound outbound HTTPS
	// credential broker in bex-ssh-gateway (w2/m534, ADR122 D6).
	InternalBrokerLookupPath = "/v1/sandbox-credential-broker/lookup"

	// BrokerLookupKind tags the JSON envelope and separates the HMAC domain
	// from every other internal gateway->bex-api verb.
	BrokerLookupKind = "sandbox-broker-lookup"

	// BrokerLookupMaxCacheTTL bounds in-memory credential caching in
	// bex-ssh-gateway so disable/delete/update/restore cuts off promptly.
	BrokerLookupMaxCacheTTL = 5 * time.Second

	brokerLookupClockSkew       = 30 * time.Second
	brokerLookupMaxRequestBytes = 16 << 10
	// Up to 25 credentials * 5 KiB secrets + settings (200 KiB Render cap),
	// with headroom for JSON escaping and networkPolicy rules.
	brokerLookupMaxResponseBytes = 512 << 10
	brokerLookupDefaultTimeout   = 5 * time.Second
	brokerLookupMaxCacheEntries  = 2048
	brokerNoncePrefix            = "sandbox-broker:"
)

var (
	// ErrBrokerInvalid indicates a malformed broker lookup request (HTTP 400).
	ErrBrokerInvalid = errors.New("invalid sandbox broker lookup request")
	// ErrBrokerForbidden indicates an unauthorized, agent-session, ended, or
	// mismatched sandbox lookup (HTTP 403).
	ErrBrokerForbidden = errors.New("sandbox broker lookup forbidden")
	// ErrBrokerUnavailable indicates store or OpenBao unavailability (HTTP 503).
	ErrBrokerUnavailable = core.Unavailable("sandbox broker lookup unavailable")
)

// BrokerLookupRequest identifies a verified /v1/sandboxes Pod resolved by
// bex-ssh-gateway from its direct source Pod IP.
type BrokerLookupRequest struct {
	Kind           string `json:"kind"`
	WorkspaceID    string `json:"workspaceId"`
	SandboxID      string `json:"sandboxId"`
	SandboxGroupID string `json:"sandboxGroupId"`
	PodName        string `json:"podName"`
	PodUID         string `json:"podUid"`
	AgentSession   bool   `json:"agentSession,omitempty"`
	Nonce          string `json:"nonce"`
}

// BrokerCredential carries one enabled sandbox group credential plus its
// OpenBao-resolved secret value for host-side injection inside bex-ssh-gateway.
// It is never returned on any public API and never written to logs or disk.
type BrokerCredential struct {
	ID        string                   `json:"id"`
	Domain    string                   `json:"domain"`
	Methods   []string                 `json:"methods"`
	Paths     []string                 `json:"paths"`
	Injection GroupCredentialInjection `json:"injection"`
	Value     string                   `json:"value"`
	VersionID int64                    `json:"versionId"`
}

// String redacts Value so fmt/log formatting can never leak secret material.
func (c BrokerCredential) String() string {
	return fmt.Sprintf("BrokerCredential{ID:%q Domain:%q VersionID:%d}", c.ID, c.Domain, c.VersionID)
}

// GoString redacts Value for %#v formatting.
func (c BrokerCredential) GoString() string { return c.String() }

// BrokerLookupResponse is the control-plane answer for one verified sandbox Pod:
// its authoritative networkPolicy and enabled group credentials (sorted by ID).
type BrokerLookupResponse struct {
	WorkspaceID    string             `json:"workspaceId"`
	SandboxID      string             `json:"sandboxId"`
	SandboxGroupID string             `json:"sandboxGroupId"`
	NetworkPolicy  NetworkPolicy      `json:"networkPolicy"`
	Credentials    []BrokerCredential `json:"credentials"`
	VersionStamp   string             `json:"versionStamp"`
}

// BrokerLookupStore is the control-plane read surface used by
// BrokerLookupResolver. *store.PGStore implements it.
type BrokerLookupStore interface {
	GetSandboxRecord(ctx context.Context, workspaceID, sandboxID string) (store.SandboxRecord, error)
	GetSandboxGroupMember(ctx context.Context, workspaceID, sandboxID string) (store.SandboxGroupMember, error)
	SandboxGroup(ctx context.Context, workspaceID, groupID string) (store.SandboxGroup, error)
	ListSandboxGroupCredentials(ctx context.Context, workspaceID, groupID string) ([]store.SandboxCredential, error)
}

// BrokerLookupResolver verifies (workspaceID, sandboxID, sandboxGroupID)
// consistency against durable control-plane state and resolves active enabled
// credentials from OpenBao. Any OpenBao failure fails closed (503).
type BrokerLookupResolver struct {
	Store         BrokerLookupStore
	Vault         *CredentialVault
	SandboxEgress SandboxEgress
	Now           func() time.Time
}

func (r *BrokerLookupResolver) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// Lookup verifies a /v1/sandboxes Pod's ownership and live status and returns
// its networkPolicy plus enabled group credentials with resolved OpenBao secrets.
func (r *BrokerLookupResolver) Lookup(ctx context.Context, req BrokerLookupRequest) (BrokerLookupResponse, error) {
	if r == nil || r.Store == nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	if req.AgentSession {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	if req.WorkspaceID == "" || req.SandboxID == "" || req.SandboxGroupID == "" || req.PodName == "" || req.PodUID == "" {
		return BrokerLookupResponse{}, ErrBrokerInvalid
	}
	if !strings.HasPrefix(req.SandboxID, "sbx-") || !strings.HasPrefix(req.SandboxGroupID, "sbg-") {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}

	// 1. Verify (workspace, sandbox, group) binding in sandbox_group_members.
	member, err := r.Store.GetSandboxGroupMember(ctx, req.WorkspaceID, req.SandboxID)
	if errors.Is(err, store.ErrNotFound) {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	if member.GroupID != req.SandboxGroupID {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}

	// 2. Verify the sandbox group exists in the workspace and is not deleting
	// (SandboxGroup filters deleting_at IS NULL and returns ErrNotFound).
	group, err := r.Store.SandboxGroup(ctx, req.WorkspaceID, req.SandboxGroupID)
	if errors.Is(err, store.ErrNotFound) {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	if group.WorkspaceID != req.WorkspaceID {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}

	// 3. Verify the sandbox record is live (not terminated, not expired) and
	// read its authoritative networkPolicy.
	rec, err := r.Store.GetSandboxRecord(ctx, req.WorkspaceID, req.SandboxID)
	if errors.Is(err, store.ErrNotFound) {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	now := r.now()
	if rec.TerminatedAt != nil {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	if rec.ExpiresAt != nil && !now.Before(*rec.ExpiresAt) {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	var box Sandbox
	if err := json.Unmarshal(rec.View, &box); err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	if box.SandboxGroupID != "" && box.SandboxGroupID != req.SandboxGroupID {
		return BrokerLookupResponse{}, ErrBrokerForbidden
	}
	policy, err := r.resolvePolicy(ctx, req.WorkspaceID, req.SandboxID, box.NetworkPolicy)
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}

	// 4. Load enabled group credentials and resolve secrets from OpenBao.
	rows, err := r.Store.ListSandboxGroupCredentials(ctx, req.WorkspaceID, req.SandboxGroupID)
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	// Deterministic lexicographical ascending ID order (ADR122 D5).
	slices.SortFunc(rows, func(a, b store.SandboxCredential) int {
		return cmp.Compare(a.ID, b.ID)
	})

	enabledRows := make([]store.SandboxCredential, 0, len(rows))
	for _, row := range rows {
		if row.Config.Enabled {
			enabledRows = append(enabledRows, row)
		}
	}
	creds := make([]BrokerCredential, 0, len(enabledRows))
	if len(enabledRows) > 0 {
		if r.Vault == nil || !r.Vault.enabled() {
			return BrokerLookupResponse{}, ErrBrokerUnavailable
		}
		for _, row := range enabledRows {
			secretVal, found, getErr := r.Vault.Get(ctx, req.WorkspaceID, req.SandboxGroupID, row.ID, row.CurrentSecretRef)
			if getErr != nil || !found {
				// Fail closed on any OpenBao read error or missing secret entry.
				return BrokerLookupResponse{}, ErrBrokerUnavailable
			}
			methods := slices.Clone(row.Config.Methods)
			if methods == nil {
				methods = []string{}
			}
			paths := slices.Clone(row.Config.Paths)
			if paths == nil {
				paths = []string{}
			}
			creds = append(creds, BrokerCredential{
				ID:      row.ID,
				Domain:  strings.ToLower(row.Config.Domain),
				Methods: methods,
				Paths:   paths,
				Injection: GroupCredentialInjection{
					Type:     row.Config.Injection.Type,
					Name:     row.Config.Injection.Name,
					Template: row.Config.Injection.Template,
				},
				Value:     secretVal,
				VersionID: row.CurrentVersion.ID,
			})
		}
	}

	return BrokerLookupResponse{
		WorkspaceID:    req.WorkspaceID,
		SandboxID:      req.SandboxID,
		SandboxGroupID: req.SandboxGroupID,
		NetworkPolicy:  policy,
		Credentials:    creds,
		VersionStamp:   brokerVersionStamp(policy, enabledRows),
	}, nil
}

func (r *BrokerLookupResolver) resolvePolicy(ctx context.Context, workspaceID, sandboxID string, raw *NetworkPolicy) (NetworkPolicy, error) {
	if raw == nil || raw.Default == "" {
		// Fail closed if a record lacks a policy.
		return NetworkPolicy{Default: NetworkPolicyDenyAll}, nil
	}
	switch raw.Default {
	case NetworkPolicyAllowAll:
		return NetworkPolicy{Default: NetworkPolicyAllowAll}, nil
	case NetworkPolicyDenyAll:
		return NetworkPolicy{Default: NetworkPolicyDenyAll}, nil
	case NetworkPolicyAllowList:
		rules := slices.Clone(raw.Rules)
		if len(rules) == 0 && r.SandboxEgress != nil {
			domains, err := r.SandboxEgress.Rules(ctx, store.SandboxNamespace(workspaceID), sandboxID)
			if err != nil {
				return NetworkPolicy{}, err
			}
			rules = policyRules(domains)
		}
		slices.SortFunc(rules, func(a, b NetworkPolicyRule) int {
			return cmp.Compare(a.Domain, b.Domain)
		})
		return NetworkPolicy{Default: NetworkPolicyAllowList, Rules: rules}, nil
	default:
		return NetworkPolicy{Default: NetworkPolicyDenyAll}, nil
	}
}

func brokerVersionStamp(policy NetworkPolicy, enabled []store.SandboxCredential) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "policy:%s\n", policy.Default)
	for _, rule := range policy.Rules {
		_, _ = fmt.Fprintf(h, "rule:%s:%s\n", rule.Protocol, rule.Domain)
	}
	for _, c := range enabled {
		_, _ = fmt.Fprintf(h, "cred:%s:%d:%s\n", c.ID, c.CurrentVersion.ID, c.CurrentVersion.Fingerprint)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func brokerMACInput(envelope []byte) []byte {
	return append([]byte(BrokerLookupKind+"\n"), envelope...)
}

// BrokerLookupHandler serves InternalBrokerLookupPath on bex-api's :8091
// internal control-plane mux. It authenticates callers using the gateway shared
// HMAC secret with domain separation (BrokerLookupKind) and single-use nonces.
type BrokerLookupHandler struct {
	Secret   []byte
	Resolver *BrokerLookupResolver
	Nonce    agentsession.NonceClaimer
	Now      func() time.Time
}

func (h *BrokerLookupHandler) now() time.Time {
	if h != nil && h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *BrokerLookupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || len(h.Secret) == 0 || h.Resolver == nil {
		http.Error(w, "sandbox credential broker unavailable", http.StatusServiceUnavailable)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, brokerLookupMaxRequestBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > brokerLookupMaxRequestBytes {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	at := h.now()
	if err := agentsession.Verify(
		h.Secret,
		brokerMACInput(raw),
		r.Header.Get(agentsession.TimestampHeader),
		r.Header.Get(agentsession.SignatureHeader),
		at,
		brokerLookupClockSkew,
	); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var req BrokerLookupRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() || req.Kind != BrokerLookupKind {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if h.Nonce != nil {
		if req.Nonce == "" {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		claimCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		claimed, claimErr := h.Nonce.ClaimShellNonce(claimCtx, brokerNoncePrefix+req.Nonce, at.Add(2*brokerLookupClockSkew))
		cancel()
		if claimErr != nil || !claimed {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}
	resp, err := h.Resolver.Lookup(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrBrokerInvalid):
			http.Error(w, "invalid request", http.StatusBadRequest)
		case errors.Is(err, ErrBrokerForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.Error(w, "sandbox credential broker unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	agentsession.WriteNoStoreJSON(w, http.StatusOK, resp)
}

// BrokerLookupClient is the gateway-side HMAC client for
// InternalBrokerLookupPath with bounded timeouts and short-TTL (<= 5s)
// in-memory caching (zero disk persistence).
type BrokerLookupClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	// CacheTTL bounds in-memory cache lifetime; zero defaults to
	// BrokerLookupMaxCacheTTL (5s) and values above 5s are clamped to 5s.
	CacheTTL time.Duration
	// MaxCacheEntries bounds memory usage; zero defaults to 2048.
	MaxCacheEntries int
	Now             func() time.Time
	// Optional observability hooks (closed labels; never called with secrets).
	OnCacheEvent     func(hit bool)
	OnLookupDuration func(d time.Duration)

	mu    sync.Mutex
	cache map[string]brokerCacheEntry
}

type brokerCacheEntry struct {
	resp    BrokerLookupResponse
	expires time.Time
}

func (c *BrokerLookupClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *BrokerLookupClient) ttl() time.Duration {
	if c.CacheTTL <= 0 || c.CacheTTL > BrokerLookupMaxCacheTTL {
		return BrokerLookupMaxCacheTTL
	}
	return c.CacheTTL
}

func (c *BrokerLookupClient) maxEntries() int {
	if c.MaxCacheEntries > 0 {
		return c.MaxCacheEntries
	}
	return brokerLookupMaxCacheEntries
}

func (c *BrokerLookupClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: brokerLookupDefaultTimeout}
}

func brokerCacheKey(req BrokerLookupRequest) string {
	return req.WorkspaceID + "|" + req.SandboxID + "|" + req.SandboxGroupID + "|" + req.PodUID
}

func cloneBrokerLookupResponse(resp BrokerLookupResponse) BrokerLookupResponse {
	out := resp
	if resp.NetworkPolicy.Rules != nil {
		out.NetworkPolicy.Rules = slices.Clone(resp.NetworkPolicy.Rules)
	}
	if resp.Credentials != nil {
		out.Credentials = make([]BrokerCredential, len(resp.Credentials))
		for i, cred := range resp.Credentials {
			credCopy := cred
			credCopy.Methods = slices.Clone(cred.Methods)
			credCopy.Paths = slices.Clone(cred.Paths)
			out.Credentials[i] = credCopy
		}
	}
	return out
}

// Lookup fetches (or serves from the <= 5s in-memory cache) the verified
// sandbox's networkPolicy and active enabled credentials.
func (c *BrokerLookupClient) Lookup(ctx context.Context, req BrokerLookupRequest) (BrokerLookupResponse, error) {
	if c == nil || c.URL == "" || len(c.Secret) == 0 {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	key := brokerCacheKey(req)
	now := c.now()

	c.mu.Lock()
	if entry, ok := c.cache[key]; ok && now.Before(entry.expires) {
		cloned := cloneBrokerLookupResponse(entry.resp)
		c.mu.Unlock()
		if c.OnCacheEvent != nil {
			c.OnCacheEvent(true)
		}
		return cloned, nil
	}
	c.mu.Unlock()
	if c.OnCacheEvent != nil {
		c.OnCacheEvent(false)
	}

	req.Kind = BrokerLookupKind
	req.Nonce = newBrokerNonce()
	body, err := json.Marshal(req)
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerInvalid
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	ts, sig := agentsession.Sign(c.Secret, brokerMACInput(body), now)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(agentsession.TimestampHeader, ts)
	httpReq.Header.Set(agentsession.SignatureHeader, sig)

	start := time.Now()
	resp, err := c.httpClient().Do(httpReq)
	if c.OnLookupDuration != nil {
		c.OnLookupDuration(time.Since(start))
	}
	if err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		switch resp.StatusCode {
		case http.StatusBadRequest:
			return BrokerLookupResponse{}, ErrBrokerInvalid
		case http.StatusForbidden, http.StatusUnauthorized:
			return BrokerLookupResponse{}, ErrBrokerForbidden
		default:
			return BrokerLookupResponse{}, ErrBrokerUnavailable
		}
	}
	var out BrokerLookupResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, brokerLookupMaxResponseBytes)).Decode(&out); err != nil {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}
	if out.WorkspaceID != req.WorkspaceID || out.SandboxID != req.SandboxID || out.SandboxGroupID != req.SandboxGroupID {
		return BrokerLookupResponse{}, ErrBrokerUnavailable
	}

	expires := c.now().Add(c.ttl())
	c.mu.Lock()
	if c.cache == nil {
		c.cache = make(map[string]brokerCacheEntry)
	}
	// Sweep expired entries first; if still at capacity, evict the earliest expiring entry.
	for k, e := range c.cache {
		if !now.Before(e.expires) {
			delete(c.cache, k)
		}
	}
	if len(c.cache) >= c.maxEntries() {
		var oldestKey string
		var oldestExp time.Time
		for k, e := range c.cache {
			if oldestKey == "" || e.expires.Before(oldestExp) {
				oldestKey = k
				oldestExp = e.expires
			}
		}
		if oldestKey != "" {
			delete(c.cache, oldestKey)
		}
	}
	c.cache[key] = brokerCacheEntry{resp: cloneBrokerLookupResponse(out), expires: expires}
	c.mu.Unlock()
	return out, nil
}

func newBrokerNonce() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
