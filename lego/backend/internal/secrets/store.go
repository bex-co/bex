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

package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// store.go is the OpenBao KV v2 implementation of core.SecretKV: a versioned,
// policy-scoped map store the env-vars/secret-files feature and the env-groups
// feature share. It authenticates as the bex-api ServiceAccount via the Kubernetes
// auth method (docs/ADR013-secrets.md §5) and speaks KV v2's split data/ + metadata/
// paths under the "tenants/" mount, tenant-prefixed per docs/ADR013-secrets.md §4.

const (
	baoRole      = "bex-api" // scripts/bao-k8s-auth.sh role bound to the bex-api SA
	baoMount     = "tenants" // KV v2 mount (docs/ADR013-secrets.md §4)
	baoTenant    = "default" // legacy single-tenant root; the ctx tenant (w7/m70) overrides it
	baoJWTPath   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	baoRenewSkew = 30 * time.Second // re-login this long before the lease expires

	// maxStoreResponseBytes bounds one OpenBao response body decoded into
	// memory (round-11 #6). Well above the tenant-map aggregate quotas, so it
	// only ever trips on a pathological or foreign payload.
	maxStoreResponseBytes = 8 << 20 // 8 MiB
)

// tenantCtxKey carries the workspace (tenant) id that scopes an OpenBao path. The
// secrets feature sets it per-request from the App's core.LabelTenant (w7/m70), so
// two same-named services in different workspaces resolve to disjoint KV paths
// (tenants/data/<tenant>/services/<name>/…). w2/m80 reuses the same seam (exported
// as WithTenant/TenantFromContext below) to move env-groups off the shared
// LegacyTenant root onto their own workspace-prefixed paths. A request that does
// not set it falls back to baoTenant, preserving the pre-w7/m70 single-tenant
// layout (byte-identical for env-groups' still-unmigrated legacy paths and for any
// caller that never sets the key).
type tenantCtxKey struct{}

// withTenant returns ctx annotated with the owning workspace so the store prefixes
// every KV path with it. tenant "" is normalized to baoTenant.
func withTenant(ctx context.Context, tenant string) context.Context {
	if tenant == "" {
		tenant = baoTenant
	}
	return context.WithValue(ctx, tenantCtxKey{}, tenant)
}

// tenantFromCtx returns the request's workspace id, defaulting to baoTenant when
// unset (env-groups' legacy paths, the lazy-migrator's legacy read, store-level
// tests).
func tenantFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(tenantCtxKey{}).(string); v != "" && ok {
		return v
	}
	return baoTenant
}

// LegacyTenant is the pre-w7/m70 shared single-tenant OpenBao root ("default").
// w2/m80 exports it so envgroups can address the same legacy root explicitly —
// the dual-read fallback and the one-time path migration's source — without
// duplicating the constant.
const LegacyTenant = baoTenant

// WithTenant is the exported form of withTenant: it scopes ctx to tenant's own
// OpenBao path prefix ("" normalizes to LegacyTenant). Exported for envgroups'
// w2/m80 workspace-prefixed env-group layout, which reuses this store's tenant
// seam exactly as the per-service secrets feature (w7/m70) does.
func WithTenant(ctx context.Context, tenant string) context.Context {
	return withTenant(ctx, tenant)
}

// TenantFromContext is the exported form of tenantFromCtx, letting envgroups
// (w2/m80) inspect which OpenBao tenant a context currently addresses.
func TenantFromContext(ctx context.Context) string {
	return tenantFromCtx(ctx)
}

// openBaoStore implements core.SecretKV over OpenBao's KV v2 engine,
// authenticating as the bex-api ServiceAccount via the Kubernetes auth method
// (docs/ADR013-secrets.md §5): it logs in with its projected SA token, caches the
// returned client token until just before its lease expires, and re-authenticates
// on demand (including when a still-cached token is rejected). Scoped by policy to
// tenants/* only.
type openBaoStore struct {
	addr    string // BEX_OPENBAO_URL, e.g. http://openbao.secrets.svc:8200
	role    string
	mount   string
	jwtPath string
	client  *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// versionedStoreError keeps the implementation detail carried by OpenBao and
// net/http errors out of API-visible text. In particular, those errors normally
// include the complete request URL, which contains the tenant and logical secret
// path. Unwrap preserves errors.Is/errors.As behavior for context, transport, and
// HTTP-status handling without making that detail part of Error().
type versionedStoreError struct {
	action string
	cause  error
}

func (e *versionedStoreError) Error() string {
	return "openbao versioned secret " + e.action + " failed"
}
func (e *versionedStoreError) Unwrap() error { return e.cause }

func sanitizeVersionedStoreError(action string, err error) error {
	if err == nil {
		return nil
	}
	return &versionedStoreError{action: action, cause: err}
}

// NewOpenBaoStore returns the production core.SecretKV talking to the
// cluster-internal OpenBao at addr. The ServiceAccount token used to log in is
// the pod's projected token by default; a non-empty jwtPath (BEX_OPENBAO_JWT_PATH,
// read by cmd/api's config load — cmd/ is the only env reader) overrides that path
// so bex-api can run off-cluster (local dev, scripts/secrets-verify.sh) against a
// token minted with `kubectl create token bex-api`.
func NewOpenBaoStore(addr, jwtPath string) core.SecretKV {
	if jwtPath == "" {
		jwtPath = baoJWTPath
	}
	return &openBaoStore{
		addr:    strings.TrimSuffix(addr, "/"),
		role:    baoRole,
		mount:   baoMount,
		jwtPath: jwtPath,
		client:  &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport},
	}
}

// dataURL / metadataURL are KV v2's split paths: values live under data/, whole-
// path deletion + listing under metadata/. path is the caller's logical key (e.g.
// "services/web/env"); the mount + the request's workspace-tenant prefix
// (docs/ADR013-secrets.md §4, w7/m70) are added here.
func (s *openBaoStore) dataURL(ctx context.Context, path string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s/%s", s.addr, s.mount, tenantFromCtx(ctx), path)
}

func (s *openBaoStore) metadataURL(ctx context.Context, path string) string {
	return fmt.Sprintf("%s/v1/%s/metadata/%s/%s", s.addr, s.mount, tenantFromCtx(ctx), path)
}

// Get returns the map stored at path, or an empty map when it was never written or
// has been soft-deleted (a 404 is not an error — the caller treats "unset" as empty).
func (s *openBaoStore) Get(ctx context.Context, path string) (map[string]string, error) {
	var out struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	err := s.kv(ctx, http.MethodGet, s.dataURL(ctx, path), nil, &out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if out.Data.Data == nil {
		return map[string]string{}, nil
	}
	return out.Data.Data, nil
}

// GetVersioned atomically reads a KV-v2 value and its current version. An
// absent key has version zero, matching OpenBao's CAS=0 create-only contract.
// The version remains backend metadata; callers encode it before exposing it.
func (s *openBaoStore) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	var out struct {
		Data struct {
			Data     map[string]string `json:"data"`
			Metadata struct {
				Version uint64 `json:"version"`
			} `json:"metadata"`
		} `json:"data"`
	}
	err := s.kv(ctx, http.MethodGet, s.dataURL(ctx, path), nil, &out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return core.SecretKVSnapshot{Data: map[string]string{}}, nil
	}
	if err != nil {
		return core.SecretKVSnapshot{}, sanitizeVersionedStoreError("read", err)
	}
	if out.Data.Metadata.Version == 0 {
		return core.SecretKVSnapshot{}, core.Err("openbao versioned read returned no version")
	}
	if out.Data.Data == nil {
		out.Data.Data = map[string]string{}
	}
	return core.SecretKVSnapshot{Data: out.Data.Data, Version: out.Data.Metadata.Version}, nil
}

// Put replaces the whole map at path.
func (s *openBaoStore) Put(ctx context.Context, path string, data map[string]string) error {
	body, _ := json.Marshal(map[string]any{"data": data})
	return s.kv(ctx, http.MethodPost, s.dataURL(ctx, path), body, nil)
}

// PutCAS replaces the whole KV-v2 value only when expectedVersion is current.
// OpenBao reports a failed check-and-set as 400 (some compatible frontends use
// 409); both become the shared conflict sentinel with a fixed, value-free
// message. The response body is never read on failure, preserving do's
// structural redaction guarantee.
func (s *openBaoStore) PutCAS(ctx context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	body, _ := json.Marshal(struct {
		Options struct {
			CAS uint64 `json:"cas"`
		} `json:"options"`
		Data map[string]string `json:"data"`
	}{
		Options: struct {
			CAS uint64 `json:"cas"`
		}{CAS: expectedVersion},
		Data: data,
	})
	var out struct {
		Data struct {
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	if err := s.kv(ctx, http.MethodPost, s.dataURL(ctx, path), body, &out); err != nil {
		var se *core.HTTPStatusError
		if errors.As(err, &se) && (se.Code == http.StatusBadRequest || se.Code == http.StatusConflict) {
			return 0, fmt.Errorf("%w: secret changed; refresh before saving", core.ErrConflict)
		}
		return 0, sanitizeVersionedStoreError("write", err)
	}
	if out.Data.Version == 0 {
		return 0, core.Err("openbao compare-and-set returned no version")
	}
	return out.Data.Version, nil
}

// Delete removes the path (and all its versions); an already-absent path is a no-op.
func (s *openBaoStore) Delete(ctx context.Context, path string) error {
	err := s.kv(ctx, http.MethodDelete, s.metadataURL(ctx, path), nil, nil)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return nil
	}
	return err
}

// List returns the immediate child keys under path (KV v2 metadata LIST). A child
// that is itself a subtree carries a trailing "/", which List strips so a caller
// gets bare ids. An absent path lists as empty, not an error.
func (s *openBaoStore) List(ctx context.Context, path string) ([]string, error) {
	var out struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	err := s.kv(ctx, http.MethodGet, s.metadataURL(ctx, path)+"?list=true", nil, &out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(out.Data.Keys))
	for _, k := range out.Data.Keys {
		keys = append(keys, strings.TrimSuffix(k, "/"))
	}
	return keys, nil
}

// kv runs one authenticated KV request, re-authenticating once if the cached
// token is rejected (403) — covers a lease revoked or expired ahead of our own
// tracking. The bex-api policy has no sys/* access, so a 403 here is a stale
// token, never a permission gap.
func (s *openBaoStore) kv(ctx context.Context, method, url string, body []byte, out any) error {
	token, err := s.authToken(ctx)
	if err != nil {
		return err
	}
	err = s.do(ctx, method, url, token, body, out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusForbidden {
		s.invalidateToken()
		if token, err = s.authToken(ctx); err != nil {
			return err
		}
		err = s.do(ctx, method, url, token, body, out)
	}
	return err
}

// authToken returns a valid OpenBao client token, logging in via the Kubernetes
// auth method when the cache is empty or near expiry.
func (s *openBaoStore) authToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}
	jwt, err := os.ReadFile(s.jwtPath)
	if err != nil {
		return "", fmt.Errorf("reading service account token: %w", err)
	}
	body, _ := json.Marshal(map[string]string{"role": s.role, "jwt": strings.TrimSpace(string(jwt))})
	var out struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := s.do(ctx, http.MethodPost, s.addr+"/v1/auth/kubernetes/login", "", body, &out); err != nil {
		return "", err
	}
	if out.Auth.ClientToken == "" {
		return "", core.Err("openbao login returned no client_token")
	}
	ttl := time.Duration(out.Auth.LeaseDuration) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	s.token = out.Auth.ClientToken
	s.tokenExp = time.Now().Add(ttl - baoRenewSkew)
	return s.token, nil
}

func (s *openBaoStore) invalidateToken() {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
}

// do runs one OpenBao HTTP call with the X-Vault-Token header (OpenBao's scheme,
// not RFC 6750 Bearer). Any non-2xx becomes a *core.HTTPStatusError; the error
// text carries the method + URL (a path, never a value) but never the response
// body, so secret material can't leak into an error string.
func (s *openBaoStore) do(ctx context.Context, method, url, token string, body []byte, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer core.DrainClose(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &core.HTTPStatusError{Code: resp.StatusCode, Summary: method + " " + url + " returned " + resp.Status}
	}
	if out != nil {
		// Round-11 #6: bound the decoded body so a runaway tenant map (or a
		// misbehaving store) cannot allocate unbounded memory. Legit payloads
		// are far under the bound (aggregate quotas cap tenant maps at
		// 512 KiB); a truncated value fails JSON parsing naturally.
		return json.NewDecoder(io.LimitReader(resp.Body, maxStoreResponseBytes)).Decode(out)
	}
	return nil
}

// Compile-time guards: the store preserves the original seam and additionally
// offers optimistic concurrency to callers that opt into it.
var (
	_ core.SecretKV          = (*openBaoStore)(nil)
	_ core.VersionedSecretKV = (*openBaoStore)(nil)
)
