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

// Package secrets is the tenant env-vars feature: a service's environment
// variables, stored in OpenBao (docs/secrets.md) and materialized into a per-app
// Kubernetes Secret the App consumes via envFrom. The Service gates + guards; the
// OpenBao KV v2 store is the injected seam. Reading and writing secret values are
// the most sensitive verbs on the API, gated through the same Checker as
// everything else. One implementation, three surfaces (REST/GraphQL/MCP).
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
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// EnvVarView is the Render-shaped env-var wire object ({key, value}, Render's
// serviceEnvVar) the REST adapter uses. The GraphQL surface renders the neutral
// core.EnvVar ({id,key,value}) nested under a Service instead — REST tracks
// Render's public API, GraphQL the dashboard.
type EnvVarView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SecretStore is the Service's seam to the versioned secret backend — OpenBao KV
// v2 in production (NewOpenBaoStore), a fake in tests. It reads/writes the whole
// env map for one service (a path convention inside the store); the domain layer
// depends on this narrow interface so it stays HTTP-free and trivial to fake. nil
// => the env-vars verbs report core.ErrSecretsUnavailable.
type SecretStore interface {
	// GetEnv returns a service's env map (empty, never nil-error, when none set).
	GetEnv(ctx context.Context, service string) (map[string]string, error)
	// PutEnv replaces a service's whole env map (Render's replace-set PUT).
	PutEnv(ctx context.Context, service string, env map[string]string) error
	// DeleteEnv removes a service's env path entirely.
	DeleteEnv(ctx context.Context, service string) error
}

// Service reads/writes tenant env vars over the injected SecretStore and
// materializes them into the App's runtime. Embeds *core.Base for the client,
// clock, and authorization gate.
type Service struct {
	*core.Base
	Store SecretStore
}

// ListEnvVars returns a service's environment variables, sorted by key for a
// stable response (Render's GET /v1/services/{id}/env-vars). Reading secret
// values is sensitive, gated like connection strings (RelCanViewSensitive).
func (s *Service) ListEnvVars(ctx context.Context, service string) ([]EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	if _, err := s.GetApp(ctx, service); err != nil {
		return nil, err // ErrNotFound for unknown services, exactly like Get
	}
	env, err := s.Store.GetEnv(ctx, service)
	if err != nil {
		return nil, err
	}
	return envVarViews(env), nil
}

// GetEnvVar returns a single variable (Render's GET .../env-vars/{key}), the bare
// {key,value}. Unknown service or key => core.ErrNotFound. Sensitive read.
func (s *Service) GetEnvVar(ctx context.Context, service, key string) (EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return EnvVarView{}, err
	}
	if s.Store == nil {
		return EnvVarView{}, core.ErrSecretsUnavailable
	}
	if _, err := s.GetApp(ctx, service); err != nil {
		return EnvVarView{}, err
	}
	env, err := s.Store.GetEnv(ctx, service)
	if err != nil {
		return EnvVarView{}, err
	}
	v, ok := env[key]
	if !ok {
		return EnvVarView{}, core.ErrNotFound
	}
	return EnvVarView{Key: key, Value: v}, nil
}

// SetEnvVars replaces a service's whole env set (Render's PUT semantics) and
// returns the new set. Writing secrets is a manage-scope verb (RelCanCreate). The
// values land in OpenBao (source of truth), are projected into the app's Secret,
// and the pods roll so the new values take effect.
func (s *Service) SetEnvVars(ctx context.Context, service string, vars []EnvVarView) ([]EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	a, err := s.GetApp(ctx, service) // one App read: existence check + patch base
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if !validEnvKey(key) {
			// Names only in the error — never the value (docs/secrets.md, t005).
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		env[key] = v.Value
	}
	if err := s.storeEnv(ctx, service, env); err != nil {
		return nil, err
	}
	if err := s.materializeEnv(ctx, a, env); err != nil {
		return nil, err
	}
	return envVarViews(env), nil
}

// SetEnvVar adds or updates one variable (Render's PUT .../env-vars/{key}, body
// {value}), merging it into the existing set rather than replacing it. Returns
// the bare {key,value}. Manage-scope verb.
func (s *Service) SetEnvVar(ctx context.Context, service, key, value string) (EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvVarView{}, err
	}
	if s.Store == nil {
		return EnvVarView{}, core.ErrSecretsUnavailable
	}
	key = strings.TrimSpace(key)
	if !validEnvKey(key) {
		return EnvVarView{}, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return EnvVarView{}, err
	}
	env, err := s.Store.GetEnv(ctx, service)
	if err != nil {
		return EnvVarView{}, err
	}
	env[key] = value
	if err := s.storeEnv(ctx, service, env); err != nil {
		return EnvVarView{}, err
	}
	if err := s.materializeEnv(ctx, a, env); err != nil {
		return EnvVarView{}, err
	}
	return EnvVarView{Key: key, Value: value}, nil
}

// DeleteEnvVar removes one variable (Render's DELETE .../env-vars/{key}),
// re-projecting the reduced set. Unknown key => core.ErrNotFound.
func (s *Service) DeleteEnvVar(ctx context.Context, service, key string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return err
	}
	env, err := s.Store.GetEnv(ctx, service)
	if err != nil {
		return err
	}
	if _, ok := env[key]; !ok {
		return core.ErrNotFound
	}
	delete(env, key)
	if err := s.storeEnv(ctx, service, env); err != nil {
		return err
	}
	return s.materializeEnv(ctx, a, env)
}

// --- core.EnvVarReader: the seam apps' GraphQL uses to nest env vars ------------

// EnvVarKeys lists a service's env-var keys only (value empty), the Render
// dashboard shape (`service{ envVarKeys{ id key } }`). id == key.
func (s *Service) EnvVarKeys(ctx context.Context, service string) ([]core.EnvVar, error) {
	vars, err := s.ListEnvVars(ctx, service)
	if err != nil {
		return nil, err
	}
	out := make([]core.EnvVar, 0, len(vars))
	for _, v := range vars {
		out = append(out, core.EnvVar{ID: v.Key, Key: v.Key}) // keys only; value fetched on demand
	}
	return out, nil
}

// EnvVarValue reads one variable's value (the dashboard's "Show secret").
func (s *Service) EnvVarValue(ctx context.Context, service, key string) (core.EnvVar, error) {
	v, err := s.GetEnvVar(ctx, service, key)
	if err != nil {
		return core.EnvVar{}, err
	}
	return core.EnvVar{ID: v.Key, Key: v.Key, Value: v.Value}, nil
}

// storeEnv writes the whole env map to the source of truth, deleting the service
// path outright once the set is empty rather than leaving an empty version behind.
func (s *Service) storeEnv(ctx context.Context, service string, env map[string]string) error {
	if len(env) == 0 {
		return s.Store.DeleteEnv(ctx, service)
	}
	return s.Store.PutEnv(ctx, service, env)
}

// envVarViews renders an env map as a key-sorted slice.
func envVarViews(env map[string]string) []EnvVarView {
	out := make([]EnvVarView, 0, len(env))
	for k, v := range env {
		out = append(out, EnvVarView{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// validEnvKey accepts a C-locale environment variable name ([A-Za-z_][A-Za-z0-9_]*):
// what a shell and Kubernetes' Secret-key validation both allow. Rejecting the
// rest keeps a bad name from later failing the Secret write with a cryptic error.
func validEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// --- materialization: OpenBao values -> per-app k8s Secret -> rolling restart --

// envSecretName is the per-app projection Secret an App consumes via envFrom.
func envSecretName(service string) string { return service + "-env" }

// materializeEnv projects a service's env map into its <service>-env Secret and
// rolls the pods so the values take effect, given the App already fetched by the
// caller (so the write path reads the App once). OpenBao stays the source of
// truth; this Secret is a derived copy that lives in etcd (the accepted trade-off
// in docs/secrets.md — OpenBao buys durability/versioning/audit/policy, not
// etcd-avoidance). Pointing spec.envFromSecret at the Secret wires envFrom into
// the Deployment; bumping spec.restartedAt rolls the pods, since envFrom is read
// only at pod creation — the same no-downtime mechanism as the restart verb.
func (s *Service) materializeEnv(ctx context.Context, a *appv1alpha1.App, env map[string]string) error {
	if err := s.upsertEnvSecret(ctx, a, env); err != nil {
		return err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecret = envSecretName(a.Name)
	// RFC3339Nano (not the verb's RFC3339) so back-to-back writes still differ and
	// roll — sub-second env edits must not collapse to the same annotation.
	a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
	return s.Client.Patch(ctx, a, base)
}

// upsertEnvSecret creates or replaces the <service>-env Secret with exactly the
// given data, owned by the App so deleting the App garbage-collects it. Data is
// rebuilt from the whole desired set on every write, so a removed key can't
// linger from a prior version.
func (s *Service) upsertEnvSecret(ctx context.Context, a *appv1alpha1.App, env map[string]string) error {
	data := make(map[string][]byte, len(env))
	for k, v := range env {
		data[k] = []byte(v)
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: envSecretName(a.Name), Namespace: s.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = data
		return controllerutil.SetControllerReference(a, sec, s.Client.Scheme())
	})
	return err
}

// --- OpenBao KV v2 implementation of SecretStore ------------------------------

const (
	baoRole      = "bex-api" // scripts/bao-k8s-auth.sh role bound to the bex-api SA
	baoMount     = "tenants" // KV v2 mount (docs/secrets.md §4)
	baoTenant    = "default" // single tenant until w1/m2 (mirrors authz DefaultWorkspace)
	baoJWTPath   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	baoRenewSkew = 30 * time.Second // re-login this long before the lease expires
)

// openBaoStore implements SecretStore over OpenBao's KV v2 engine, authenticating
// as the bex-api ServiceAccount via the Kubernetes auth method (docs/secrets.md
// §5): it logs in with its projected SA token, caches the returned client token
// until just before its lease expires, and re-authenticates on demand (including
// when a still-cached token is rejected). Scoped by policy to tenants/* only.
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

// NewOpenBaoStore returns the production SecretStore talking to the
// cluster-internal OpenBao at addr. The ServiceAccount token used to log in is
// the pod's projected token by default; BEX_OPENBAO_JWT_PATH overrides that path
// so bex-api can run off-cluster (local dev, scripts/secrets-verify.sh) against a
// token minted with `kubectl create token bex-api`.
func NewOpenBaoStore(addr string) SecretStore {
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

// dataURL / metadataURL are KV v2's split paths: data under data/, whole-path
// deletion under metadata/. tenants/<tenant>/services/<service>/env keeps the
// per-tenant convention from docs/secrets.md §4.
func (s *openBaoStore) dataURL(service string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s/services/%s/env", s.addr, s.mount, s.tenant, service)
}

func (s *openBaoStore) metadataURL(service string) string {
	return fmt.Sprintf("%s/v1/%s/metadata/%s/services/%s/env", s.addr, s.mount, s.tenant, service)
}

func (s *openBaoStore) GetEnv(ctx context.Context, service string) (map[string]string, error) {
	var out struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	err := s.kv(ctx, http.MethodGet, s.dataURL(service), nil, &out)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return map[string]string{}, nil // never written, or soft-deleted
	}
	if err != nil {
		return nil, err
	}
	if out.Data.Data == nil {
		return map[string]string{}, nil
	}
	return out.Data.Data, nil
}

func (s *openBaoStore) PutEnv(ctx context.Context, service string, env map[string]string) error {
	body, _ := json.Marshal(map[string]any{"data": env})
	return s.kv(ctx, http.MethodPost, s.dataURL(service), body, nil)
}

func (s *openBaoStore) DeleteEnv(ctx context.Context, service string) error {
	err := s.kv(ctx, http.MethodDelete, s.metadataURL(service), nil, nil)
	var se *core.HTTPStatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return nil // already gone
	}
	return err
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

// compile-time guards: the store satisfies SecretStore, the Service satisfies the
// kernel's EnvVarReader seam apps' GraphQL nests through.
var (
	_ SecretStore       = (*openBaoStore)(nil)
	_ core.EnvVarReader = (*Service)(nil)
)
