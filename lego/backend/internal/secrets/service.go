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

// Package secrets is the tenant env-vars + secret-files feature: a service's
// environment variables and mounted secret files, both stored in OpenBao
// (docs/ADR013-secrets.md) and materialized into per-app Kubernetes Secrets the App
// consumes — env vars via envFrom ("<name>-env"), files via a projected
// /etc/secrets volume ("<name>-files"). The Service gates + guards; the OpenBao KV
// v2 store is the injected core.SecretKV seam (shared with the env-groups
// feature). Reading and writing secret material are the most sensitive verbs on
// the API, gated through the same Checker as everything else. One implementation,
// three surfaces (REST/GraphQL/MCP).
package secrets

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
// Render's public API, GraphQL the dashboard. Revision is an additive bex
// extension: version-aware stores attach the same opaque whole-map token to
// every sensitive REST/MCP read so those adapters can use the same optional CAS
// patch as GraphQL. Legacy stores omit it.
//
// Generate is Render's generateValue (w8/m10): when true and Value is empty, the
// write verb mints a cryptographically random value server-side (core.GenerateValue,
// base64 256-bit) and stores it like any other value; the minted value comes back
// in the write response so the caller can read what was generated. Value + Generate
// together is rejected (Render leaves the combination undocumented — an explicit
// value and "please generate one" are contradictory intent).
type EnvVarView struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Generate bool   `json:"generateValue,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// EnvVarWrite is the mutually exclusive value-or-generate input accepted by a
// single-key env-var write on every adapter.
type EnvVarWrite struct {
	Value         string
	GenerateValue bool
}

// Service reads/writes tenant env vars + secret files over the injected
// core.SecretKV store and materializes them into the App's runtime. Embeds
// *core.Base for the client, clock, and authorization gate.
type Service struct {
	*core.Base
	Store core.SecretKV
}

// envPath is a service's env-map key in the store (docs/ADR013-secrets.md §4 layout,
// minus the mount/tenant prefix the store prepends).
func envPath(service string) string { return "services/" + service + "/env" }

// storeServiceName keeps OpenBao paths on the public service name even when a
// Render client addresses the resource by its stable srv- id. Store-managed
// Apps use a tenant-prefixed Kubernetes name, so neither the request token nor
// metadata.name is the correct key for data seeded during create.
func storeServiceName(a *appv1alpha1.App, fallback string) string {
	if name := a.Labels[core.LabelServiceName]; name != "" {
		return name
	}
	return fallback
}

// storeTenant returns the App's workspace (tenant) id for keying its OpenBao paths
// (w7/m70). Every CRD-projected and store-managed App carries core.LabelTenant; the
// baoTenant default only covers a degenerate labelless App.
func storeTenant(a *appv1alpha1.App) string {
	if t := a.Labels[core.LabelTenant]; t != "" {
		return t
	}
	return baoTenant
}

// readMap fetches exactly the map under the request's tenant. Legacy
// default-tenant data is deliberately not migrated on a tenant request: once
// same-named services can exist in multiple workspaces, a bare
// default/services/<name> path has no trustworthy owner. Operators must map and
// migrate those paths out of band using control-plane ownership evidence.
func (s *Service) readMap(ctx context.Context, path string) (map[string]string, error) {
	cur, err := s.Store.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return cur, nil
}

// ListEnvVars returns a service's environment variables, sorted by key for a
// stable response (Render's GET /v1/services/{id}/env-vars). Reading secret
// values is sensitive, gated like connection strings (RelCanViewSensitive).
func (s *Service) ListEnvVars(ctx context.Context, service string) ([]EnvVarView, error) {
	env, revision, err := s.readAuthorizedEnvMap(ctx, service)
	if err != nil {
		return nil, err
	}
	return envVarViews(env, revision), nil
}

// readAuthorizedEnvMap is the one sensitive-read boundary for the masked list,
// explicit single-key reveal, and the legacy REST list/get verbs. A versioned
// store returns one opaque whole-map revision alongside the snapshot; legacy
// SecretKV implementations keep returning an empty revision so old callers and
// test doubles remain compatible.
func (s *Service) readAuthorizedEnvMap(ctx context.Context, service string) (map[string]string, string, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return nil, "", err // ErrNotFound for unknown services, exactly like Get
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return nil, "", core.ErrSecretsUnavailable
	}
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		env, err := s.readMap(ctx, envPath(service))
		if err != nil {
			return nil, "", err
		}
		return env, "", nil
	}
	return s.readVersionedEnvMap(ctx, envPath(service), versioned)
}

// readVersionedEnvMap reads one atomic snapshot from the already tenant-scoped
// context. It never probes the ambiguous legacy default tenant.
func (s *Service) readVersionedEnvMap(ctx context.Context, path string, versioned core.VersionedSecretKV) (map[string]string, string, error) {
	snapshot, err := versioned.GetVersioned(ctx, path)
	if err != nil {
		return nil, "", envSourceUnavailable()
	}
	return snapshot.Data, encodeEnvRevision(snapshot.Version), nil
}

func envSourceUnavailable() error {
	return fmt.Errorf("%w: environment source is unavailable", core.ErrSecretsUnavailable)
}

// ListEnvVarsPage returns a stable keyset page of a service's environment
// variables. limit == 0 with no cursor preserves the original unbounded-list
// behavior; cursor with no limit uses Render's default. A positive limit is
// clamped to Render's public API maximum. after is the prior page's item cursor
// (the env-var key).
func (s *Service) ListEnvVarsPage(ctx context.Context, service, after string, limit int) ([]EnvVarView, error) {
	vars, err := s.ListEnvVars(ctx, service)
	if err != nil {
		return nil, err
	}
	if limit == 0 && after == "" {
		return vars, nil
	}
	if limit == 0 {
		limit = core.DefaultPageLimit
	}
	if limit < 0 {
		return nil, fmt.Errorf("%w: limit must be a positive integer", core.ErrBadRequest)
	}
	if limit > core.MaxPageLimit {
		limit = core.MaxPageLimit
	}
	return core.Page(vars, after, limit, func(v EnvVarView) string { return v.Key }), nil
}

// GetEnvVar returns a single variable (Render's GET .../env-vars/{key}), the bare
// {key,value}. Unknown service or key => core.ErrNotFound. Sensitive read.
func (s *Service) GetEnvVar(ctx context.Context, service, key string) (EnvVarView, error) {
	env, revision, err := s.readAuthorizedEnvMap(ctx, service)
	if err != nil {
		return EnvVarView{}, err
	}
	v, ok := env[key]
	if !ok {
		return EnvVarView{}, core.ErrNotFound
	}
	return EnvVarView{Key: key, Value: v, Revision: revision}, nil
}

// SetEnvVars replaces a service's whole env set (Render's PUT semantics) and
// returns the new set. Writing secrets is a manage-scope verb (RelCanCreate). The
// values land in OpenBao (source of truth), are projected into the app's Secret,
// and the pods roll so the new values take effect.
func (s *Service) SetEnvVars(ctx context.Context, service string, vars []EnvVarView) ([]EnvVarView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service) // one App read: existence check + patch base
	if err != nil {
		return nil, err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	env := make(map[string]string, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if !core.ValidEnvKey(key) {
			// Names only in the error — never the value (docs/ADR013-secrets.md, t005).
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		value, err := resolveValue(key, v.Value, v.Generate)
		if err != nil {
			return nil, err
		}
		env[key] = value
	}
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
		return nil, err
	}
	if err := s.materializeEnv(ctx, a, env); err != nil {
		return nil, err
	}
	return envVarViews(env, ""), nil
}

// resolveValue applies Render's generateValue semantics to one env-var write:
// generate=true with no explicit value mints a random secret (core.GenerateValue);
// generate=true WITH a value is a contradiction and rejected; otherwise the value
// passes through unchanged. Shared by every write path so the rule lives once.
func resolveValue(key, value string, generate bool) (string, error) {
	if !generate {
		return value, nil
	}
	if value != "" {
		return "", fmt.Errorf("%w: env var %q sets both value and generateValue — pick one", core.ErrBadRequest, key)
	}
	return core.GenerateValue()
}

// SetEnvVar adds or updates one variable (Render's PUT .../env-vars/{key}, body
// {value} or {generateValue:true}), merging it into the existing set rather than
// replacing it. Returns the bare {key,value}. Manage-scope verb.
func (s *Service) SetEnvVar(ctx context.Context, service, key string, write EnvVarWrite) (EnvVarView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return EnvVarView{}, err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return EnvVarView{}, core.ErrSecretsUnavailable
	}
	key = strings.TrimSpace(key)
	if !core.ValidEnvKey(key) {
		return EnvVarView{}, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
	}
	value, err := resolveValue(key, write.Value, write.GenerateValue)
	if err != nil {
		return EnvVarView{}, err
	}
	env, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return EnvVarView{}, err
	}
	env[key] = value
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
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
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	env, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if _, ok := env[key]; !ok {
		return core.ErrNotFound
	}
	delete(env, key)
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
		return err
	}
	return s.materializeEnv(ctx, a, env)
}

// SeedEnvVars is the blueprint apply path's seam (w1/m35): it seeds a service's
// sync:false and generateValue env vars into the OpenBao env store SEED-ONCE — a
// key already present keeps its live value, so a later blueprint sync never
// overwrites a dashboard edit (Render's "dashboard value wins" for sync:false, and
// generated values persist rather than re-minting). literals seed to their given
// value; generates mint a fresh core.GenerateValue when the key is unset. Because
// these land in the mutable env store (not the App's spec.Env), the env-vars API
// naturally overrides them later. A no-op when every key is already present: no
// Secret write, no pod roll — the stack re-apply idempotency contract. Manage scope.
func (s *Service) SeedEnvVars(ctx context.Context, service string, literals map[string]string, generates []string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	env, err := s.readMap(ctx, envPath(service))
	if err != nil {
		return err
	}
	changed := false
	seed := func(key, value string, generate bool) error {
		key = strings.TrimSpace(key)
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		if _, ok := env[key]; ok {
			return nil // seed-once: an already-set key keeps its live value
		}
		v, err := resolveValue(key, value, generate)
		if err != nil {
			return err
		}
		env[key] = v
		changed = true
		return nil
	}
	// Sort keys so a rand.Read failure (or a bad name) is deterministic across runs.
	for _, key := range core.SortedKeys(literals) {
		if err := seed(key, literals[key], false); err != nil {
			return err
		}
	}
	for _, key := range generates {
		if err := seed(key, "", true); err != nil {
			return err
		}
	}
	if !changed {
		return nil // every key already seeded — no Secret write, no roll
	}
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
		return err
	}
	return s.materializeEnv(ctx, a, env)
}

// --- core.EnvVarReader: the seam apps' GraphQL uses to nest env vars ------------

// EnvVarKeys lists a service's env-var keys only (value empty), the Render
// dashboard shape (`service{ envVarKeys{ id key } }`). id == key.
func (s *Service) EnvVarKeys(ctx context.Context, service string) ([]core.EnvVar, error) {
	env, revision, err := s.readAuthorizedEnvMap(ctx, service)
	if err != nil {
		return nil, err
	}
	keys := core.SortedKeys(env)
	out := make([]core.EnvVar, 0, len(keys))
	for _, key := range keys {
		out = append(out, core.EnvVar{ID: key, Key: key, Revision: revision}) // keys only; value fetched on demand
	}
	return out, nil
}

// EnvVarValue reads one variable's value (the dashboard's "Show secret").
func (s *Service) EnvVarValue(ctx context.Context, service, key string) (core.EnvVar, error) {
	env, revision, err := s.readAuthorizedEnvMap(ctx, service)
	if err != nil {
		return core.EnvVar{}, err
	}
	value, ok := env[key]
	if !ok {
		return core.EnvVar{}, core.ErrNotFound
	}
	return core.EnvVar{ID: key, Key: key, Value: value, Revision: revision}, nil
}

// storeMap writes the whole map to the source of truth at path, deleting the path
// outright once the set is empty rather than leaving an empty version behind.
func (s *Service) storeMap(ctx context.Context, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.Store.Delete(ctx, path)
	}
	return s.Store.Put(ctx, path, data)
}

// envVarViews renders an env map as a key-sorted slice.
func envVarViews(env map[string]string, revision string) []EnvVarView {
	out := make([]EnvVarView, 0, len(env))
	for k, v := range env {
		out = append(out, EnvVarView{Key: k, Value: v, Revision: revision})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// --- materialization: OpenBao values -> per-app k8s Secret -> rolling restart --

// envSecretName is the per-app projection Secret an App consumes via envFrom.
func envSecretName(service string) string { return service + "-env" }

// materializeEnv projects a service's env map into its <service>-env Secret and
// rolls the pods so the values take effect, given the App already fetched by the
// caller (so the write path reads the App once). OpenBao stays the source of
// truth; this Secret is a derived copy that lives in etcd (the accepted trade-off
// in docs/ADR013-secrets.md — OpenBao buys durability/versioning/audit/policy, not
// etcd-avoidance). Pointing spec.envFromSecret at the Secret wires envFrom into
// the Deployment; bumping spec.restartedAt rolls the pods, since envFrom is read
// only at pod creation — the same no-downtime mechanism as the restart verb.
func (s *Service) materializeEnv(ctx context.Context, a *appv1alpha1.App, env map[string]string) error {
	base := client.MergeFrom(a.DeepCopy())
	if err := s.projectEnv(ctx, a, env); err != nil {
		return err
	}
	s.bumpRestart(a)
	return s.Client.Patch(ctx, a, base)
}

// projectEnv updates the derived Kubernetes Secret and App reference without
// changing restartedAt or persisting the App. Batch environment writes call it
// together with projectFiles, then patch the App exactly once when a rollout was
// requested. The older single-item verbs keep calling materializeEnv and retain
// their immediate-roll behavior.
func (s *Service) projectEnv(ctx context.Context, a *appv1alpha1.App, env map[string]string) error {
	if err := s.upsertSecret(ctx, a, envSecretName(a.Name), env); err != nil {
		return err
	}
	a.Spec.EnvFromSecret = envSecretName(a.Name)
	return nil
}

// bumpRestart stamps spec.restartedAt so the pods roll on the next reconcile.
// RFC3339Nano (not the verb's RFC3339) so back-to-back writes still differ and
// roll — sub-second secret edits must not collapse to the same annotation.
func (s *Service) bumpRestart(a *appv1alpha1.App) {
	a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
}

// upsertSecret creates or replaces the named Secret with exactly the given data,
// owned by the App so deleting the App garbage-collects it. Data is rebuilt from
// the whole desired set on every write, so a removed key can't linger from a
// prior version.
func (s *Service) upsertSecret(ctx context.Context, a *appv1alpha1.App, name string, data map[string]string) error {
	bytesData := make(map[string][]byte, len(data))
	for k, v := range data {
		bytesData[k] = []byte(v)
	}
	// The pod injects this Secret via envFrom (spec.envFromSecret) and owns it
	// (controller ref below), so it MUST live in the App's namespace — the
	// per-tenant `<ws>` namespace under ADR043; a cross-namespace owner ref would
	// be garbage-collected and envFrom would resolve nothing.
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: a.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = bytesData
		return controllerutil.SetControllerReference(a, sec, s.Client.Scheme())
	})
	return err
}

// compile-time guards: the Service satisfies the kernel's reader seams apps'
// GraphQL nests env vars + secret files through.
var (
	_ core.EnvVarReader     = (*Service)(nil)
	_ core.SecretFileReader = (*Service)(nil)
)
