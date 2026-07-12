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
	baoTenant    = "default" // single tenant until w1/m2 (mirrors authz DefaultWorkspace)
	baoJWTPath   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	baoRenewSkew = 30 * time.Second // re-login this long before the lease expires
)

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
	tenant  string
	jwtPath string
	client  *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// NewOpenBaoStore returns the production core.SecretKV talking to the
// cluster-internal OpenBao at addr. The ServiceAccount token used to log in is
// the pod's projected token by default; BEX_OPENBAO_JWT_PATH overrides that path
// so bex-api can run off-cluster (local dev, scripts/secrets-verify.sh) against a
// token minted with `kubectl create token bex-api`.
func NewOpenBaoStore(addr string) core.SecretKV {
	jwtPath := baoJWTPath
	if p := os.Getenv("BEX_OPENBAO_JWT_PATH"); p != "" {
		jwtPath = p
	}
	return &openBaoStore{
		addr:    strings.TrimSuffix(addr, "/"),
		role:    baoRole,
		mount:   baoMount,
		tenant:  baoTenant,
		jwtPath: jwtPath,
		client:  &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport},
	}
}

// dataURL / metadataURL are KV v2's split paths: values live under data/, whole-
// path deletion + listing under metadata/. path is the caller's logical key (e.g.
// "services/web/env"); the mount + tenant prefix (docs/ADR013-secrets.md §4) are added here.
func (s *openBaoStore) dataURL(path string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s/%s", s.addr, s.mount, s.tenant, path)
}

func (s *openBaoStore) metadataURL(path string) string {
	return fmt.Sprintf("%s/v1/%s/metadata/%s/%s", s.addr, s.mount, s.tenant, path)
}

// Get returns the map stored at path, or an empty map when it was never written or
// has been soft-deleted (a 404 is not an error — the caller treats "unset" as empty).
func (s *openBaoStore) Get(ctx context.Context, path string) (map[string]string, error) {
	var out struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	err := s.kv(ctx, http.MethodGet, s.dataURL(path), nil, &out)
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

// Put replaces the whole map at path.
func (s *openBaoStore) Put(ctx context.Context, path string, data map[string]string) error {
	body, _ := json.Marshal(map[string]any{"data": data})
	return s.kv(ctx, http.MethodPost, s.dataURL(path), body, nil)
}

// Delete removes the path (and all its versions); an already-absent path is a no-op.
func (s *openBaoStore) Delete(ctx context.Context, path string) error {
	err := s.kv(ctx, http.MethodDelete, s.metadataURL(path), nil, nil)
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
	err := s.kv(ctx, http.MethodGet, s.metadataURL(path)+"?list=true", nil, &out)
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
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// compile-time guard: the store satisfies the shared core.SecretKV seam.
var _ core.SecretKV = (*openBaoStore)(nil)
