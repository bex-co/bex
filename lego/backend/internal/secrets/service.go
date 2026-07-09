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
// (docs/secrets.md) and materialized into per-app Kubernetes Secrets the App
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
// Render's public API, GraphQL the dashboard.
type EnvVarView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Service reads/writes tenant env vars + secret files over the injected
// core.SecretKV store and materializes them into the App's runtime. Embeds
// *core.Base for the client, clock, and authorization gate.
type Service struct {
	*core.Base
	Store core.SecretKV
}

// envPath is a service's env-map key in the store (docs/secrets.md §4 layout,
// minus the mount/tenant prefix the store prepends).
func envPath(service string) string { return "services/" + service + "/env" }

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
	env, err := s.Store.Get(ctx, envPath(service))
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
	env, err := s.Store.Get(ctx, envPath(service))
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
		if !core.ValidEnvKey(key) {
			// Names only in the error — never the value (docs/secrets.md, t005).
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		env[key] = v.Value
	}
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
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
	if !core.ValidEnvKey(key) {
		return EnvVarView{}, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return EnvVarView{}, err
	}
	env, err := s.Store.Get(ctx, envPath(service))
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
	env, err := s.Store.Get(ctx, envPath(service))
	if err != nil {
		return err
	}
	if _, ok := env[key]; !ok {
		return core.ErrNotFound
	}
	delete(env, key)
	if err := s.storeMap(ctx, envPath(service), env); err != nil {
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

// storeMap writes the whole map to the source of truth at path, deleting the path
// outright once the set is empty rather than leaving an empty version behind.
func (s *Service) storeMap(ctx context.Context, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.Store.Delete(ctx, path)
	}
	return s.Store.Put(ctx, path, data)
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
	if err := s.upsertSecret(ctx, a, envSecretName(a.Name), env); err != nil {
		return err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecret = envSecretName(a.Name)
	s.bumpRestart(a)
	return s.Client.Patch(ctx, a, base)
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
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace}}
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
