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

// Package envgroups is the environment-groups feature (Render's /v1/env-groups):
// a named, reusable set of env vars + secret files that can be linked to many
// services at once. It reuses the same OpenBao-backed core.SecretKV store as the
// per-service secrets feature (no new store) — a group's contents live at
// "env-groups/<id>/{meta,env,files}" — and materializes each group into two
// Kubernetes Secrets ("<id>-env", "<id>-files"). Linking a group to a service
// appends those Secret names to the service's App spec (spec.envFromSecrets /
// spec.filesFromSecrets); the operator wires them into the container's envFrom and
// the shared /etc/secrets volume. One implementation, three surfaces + dashboard.
package envgroups

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service manages environment groups over the shared core.SecretKV store and
// projects them into linked services. Embeds *core.Base for the client, clock, and
// authorization gate.
type Service struct {
	*core.Base
	Store core.SecretKV
}

// EnvVarView is a group env var ({key, value}); value is empty in list/get
// responses (fetched per key), present only on the sensitive single-var read.
type EnvVarView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SecretFileView is a group secret file ({name, content}); content follows the
// same names-first discipline as EnvVarView.
type SecretFileView struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
}

// EnvGroupView is the Render-shaped env-group object. EnvVars/SecretFiles carry
// keys/names only (no secret material) — a list/get never leaks values; the
// per-var / per-file reveal verbs return them under the sensitive scope.
type EnvGroupView struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	ServiceLinks []string         `json:"serviceLinks"`
	EnvVars      []EnvVarView     `json:"envVars"`
	SecretFiles  []SecretFileView `json:"secretFiles"`
}

// --- store paths + materialized Secret names ----------------------------------

func metaPath(gid string) string  { return "env-groups/" + gid + "/meta" }
func envPath(gid string) string   { return "env-groups/" + gid + "/env" }
func filesPath(gid string) string { return "env-groups/" + gid + "/files" }

// envSecretName / filesSecretName are the per-group projection Secrets linked
// services consume (envFrom + /etc/secrets projected volume).
func envSecretName(gid string) string   { return gid + "-env" }
func filesSecretName(gid string) string { return gid + "-files" }

// meta is a group's non-secret metadata, stored as a string map in the KV store.
type meta struct {
	name  string
	links []string
}

// --- group lifecycle ----------------------------------------------------------

// ListEnvGroups returns every environment group (no secret material — keys/names
// and links only). View scope.
func (s *Service) ListEnvGroups(ctx context.Context) ([]EnvGroupView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	ids, err := s.Store.List(ctx, "env-groups")
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	out := make([]EnvGroupView, 0, len(ids))
	for _, gid := range ids {
		v, err := s.view(ctx, gid)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// GetEnvGroup returns one group (keys/names + links, no values). View scope.
func (s *Service) GetEnvGroup(ctx context.Context, gid string) (EnvGroupView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return EnvGroupView{}, err
	}
	if s.Store == nil {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	if _, err := s.requireGroup(ctx, gid); err != nil {
		return EnvGroupView{}, err
	}
	return s.view(ctx, gid)
}

// CreateEnvGroup mints a group with a name and materializes its (empty) projection
// Secrets so a link can reference them immediately. Manage scope.
func (s *Service) CreateEnvGroup(ctx context.Context, name string) (EnvGroupView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvGroupView{}, err
	}
	if s.Store == nil {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return EnvGroupView{}, fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	gid := id.New(id.EnvGroup)
	if err := s.writeMeta(ctx, gid, meta{name: name}); err != nil {
		return EnvGroupView{}, err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), nil); err != nil {
		return EnvGroupView{}, err
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), nil); err != nil {
		return EnvGroupView{}, err
	}
	return EnvGroupView{ID: gid, Name: name, ServiceLinks: []string{}, EnvVars: []EnvVarView{}, SecretFiles: []SecretFileView{}}, nil
}

// DeleteEnvGroup unlinks the group from every service, deletes its projection
// Secrets, and removes its store paths. Manage scope.
func (s *Service) DeleteEnvGroup(ctx context.Context, gid string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return err
	}
	// Detach from linked services first (drop the spec refs + roll) so no pod is
	// left referencing a Secret about to be deleted.
	for _, svc := range m.links {
		if err := s.detach(ctx, gid, svc); err != nil {
			return err
		}
	}
	if err := s.deleteSecret(ctx, envSecretName(gid)); err != nil {
		return err
	}
	if err := s.deleteSecret(ctx, filesSecretName(gid)); err != nil {
		return err
	}
	for _, p := range []string{envPath(gid), filesPath(gid), metaPath(gid)} {
		if err := s.Store.Delete(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// --- group contents (env vars) ------------------------------------------------

// SetEnvGroupVars replaces the group's whole env set, re-materializes the group's
// env Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupVars(ctx context.Context, gid string, vars []EnvVarView) ([]EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if !core.ValidEnvKey(key) {
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		env[key] = v.Value
	}
	if err := s.storeMap(ctx, envPath(gid), env); err != nil {
		return nil, err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), env); err != nil {
		return nil, err
	}
	if err := s.rollLinked(ctx, m.links); err != nil {
		return nil, err
	}
	return envViews(env), nil
}

// GetEnvGroupVar reveals one variable's value (sensitive read).
func (s *Service) GetEnvGroupVar(ctx context.Context, gid, key string) (EnvVarView, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return EnvVarView{}, err
	}
	if s.Store == nil {
		return EnvVarView{}, core.ErrSecretsUnavailable
	}
	if _, err := s.requireGroup(ctx, gid); err != nil {
		return EnvVarView{}, err
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return EnvVarView{}, err
	}
	v, ok := env[key]
	if !ok {
		return EnvVarView{}, core.ErrNotFound
	}
	return EnvVarView{Key: key, Value: v}, nil
}

// --- group contents (secret files) --------------------------------------------

// SetEnvGroupFile adds or updates one group secret file (merged), re-materializes
// the group's files Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupFile(ctx context.Context, gid, name, content string) (SecretFileView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return SecretFileView{}, err
	}
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return SecretFileView{}, err
	}
	name = strings.TrimSpace(name)
	if !core.ValidSecretFileName(name) {
		return SecretFileView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return SecretFileView{}, err
	}
	files[name] = content
	if err := s.storeMap(ctx, filesPath(gid), files); err != nil {
		return SecretFileView{}, err
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), files); err != nil {
		return SecretFileView{}, err
	}
	if err := s.rollLinked(ctx, m.links); err != nil {
		return SecretFileView{}, err
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// DeleteEnvGroupFile removes one group secret file (re-materializing the reduced
// set) and rolls linked services. Manage scope.
func (s *Service) DeleteEnvGroupFile(ctx context.Context, gid, name string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return err
	}
	if _, ok := files[name]; !ok {
		return core.ErrNotFound
	}
	delete(files, name)
	if err := s.storeMap(ctx, filesPath(gid), files); err != nil {
		return err
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), files); err != nil {
		return err
	}
	return s.rollLinked(ctx, m.links)
}

// GetEnvGroupFile reveals one file's content (sensitive read).
func (s *Service) GetEnvGroupFile(ctx context.Context, gid, name string) (SecretFileView, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return SecretFileView{}, err
	}
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
	}
	if _, err := s.requireGroup(ctx, gid); err != nil {
		return SecretFileView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return SecretFileView{}, err
	}
	content, ok := files[name]
	if !ok {
		return SecretFileView{}, core.ErrNotFound
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// --- linking ------------------------------------------------------------------

// LinkService links the group to a service: the service's App spec gains the
// group's env + files Secret refs, its pods roll, and the group's link set records
// the service. Idempotent. Manage scope.
func (s *Service) LinkService(ctx context.Context, gid, service string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(gid))
	a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	a.Spec.RestartedAt = s.now()
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return err
	}
	m.links = addString(m.links, service)
	return s.writeMeta(ctx, gid, m)
}

// UnlinkService reverses LinkService: drop the group's Secret refs from the
// service, roll it, and remove it from the group's link set. Idempotent.
func (s *Service) UnlinkService(ctx context.Context, gid, service string) error {
	// Authorize+fetch against the service's OWN workspace (w6/m17) — reused
	// below via detachFetched, so this is the only fetch of `service` UnlinkService
	// makes. detach (DeleteEnvGroup's bulk-unlink path over every linked service,
	// which authorizes once for the GROUP, not per service) still does its own
	// bare GetApp: it must not fan out into one audit event per linked service.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return err
	}
	if err := s.detachFetched(ctx, gid, a); err != nil {
		return err
	}
	m.links = removeString(m.links, service)
	return s.writeMeta(ctx, gid, m)
}

// detach removes the group's Secret refs from a service and rolls it, tolerating a
// service that no longer exists (a deleted service simply drops from the group).
func (s *Service) detach(ctx context.Context, gid, service string) error {
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if errors.Is(err, core.ErrNotFound) {
		return nil // a since-deleted service just drops from the group
	}
	if err != nil {
		return err
	}
	return s.detachFetched(ctx, gid, a)
}

// detachFetched is detach's second half, for a caller (UnlinkService) that
// already holds the App it authorized — reusing it rather than fetching (and
// authorizing, and auditing) a second time.
func (s *Service) detachFetched(ctx context.Context, gid string, a *appv1alpha1.App) error {
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecrets = removeString(a.Spec.EnvFromSecrets, envSecretName(gid))
	a.Spec.FilesFromSecrets = removeString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	a.Spec.RestartedAt = s.now()
	return s.Client.Patch(ctx, a, base)
}

// rollLinked bumps spec.restartedAt on every linked service so it picks up the
// group's changed Secret data (the Secret refs are already on the spec from the
// link). A since-deleted linked service is skipped.
func (s *Service) rollLinked(ctx context.Context, links []string) error {
	stamp := s.now()
	for _, svc := range links {
		a, err := s.GetApp(ctx, core.RelCanCreate, svc)
		if errors.Is(err, core.ErrNotFound) {
			continue // a since-deleted linked service is skipped
		}
		if err != nil {
			return err
		}
		base := client.MergeFrom(a.DeepCopy())
		a.Spec.RestartedAt = stamp
		if err := s.Client.Patch(ctx, a, base); err != nil {
			return err
		}
	}
	return nil
}

// --- store + secret helpers ---------------------------------------------------

// view builds the (secret-free) view of a group from its stored meta + contents.
func (s *Service) view(ctx context.Context, gid string) (EnvGroupView, error) {
	m, err := s.readMeta(ctx, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	links := m.links
	if links == nil {
		links = []string{}
	}
	return EnvGroupView{
		ID:           gid,
		Name:         m.name,
		ServiceLinks: links,
		EnvVars:      envKeyViews(env),
		SecretFiles:  fileNameViews(files),
	}, nil
}

// requireGroup returns a group's meta or core.ErrNotFound when it doesn't exist.
func (s *Service) requireGroup(ctx context.Context, gid string) (meta, error) {
	if !id.WellFormed(gid) {
		return meta{}, core.ErrNotFound
	}
	return s.readMeta(ctx, gid)
}

func (s *Service) readMeta(ctx context.Context, gid string) (meta, error) {
	raw, err := s.Store.Get(ctx, metaPath(gid))
	if err != nil {
		return meta{}, err
	}
	if len(raw) == 0 {
		return meta{}, core.ErrNotFound
	}
	m := meta{name: raw["name"]}
	if l := strings.TrimSpace(raw["links"]); l != "" {
		m.links = strings.Split(l, ",")
	}
	return m, nil
}

func (s *Service) writeMeta(ctx context.Context, gid string, m meta) error {
	return s.Store.Put(ctx, metaPath(gid), map[string]string{
		"name":  m.name,
		"links": strings.Join(m.links, ","),
	})
}

// storeMap writes a map to the source of truth, deleting the path when empty.
func (s *Service) storeMap(ctx context.Context, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.Store.Delete(ctx, path)
	}
	return s.Store.Put(ctx, path, data)
}

// upsertSecret creates or replaces a group projection Secret with exactly the
// given data. Group Secrets have no App owner (a group outlives any one service),
// so DeleteEnvGroup removes them explicitly.
func (s *Service) upsertSecret(ctx context.Context, name string, data map[string]string) error {
	bytesData := make(map[string][]byte, len(data))
	for k, v := range data {
		bytesData[k] = []byte(v)
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = bytesData
		return nil
	})
	return err
}

func (s *Service) deleteSecret(ctx context.Context, name string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace}}
	return client.IgnoreNotFound(s.Client.Delete(ctx, sec))
}

// now is the rolling-restart stamp (RFC3339Nano so back-to-back edits differ).
func (s *Service) now() string { return s.Now().UTC().Format(time.RFC3339Nano) }

// --- view + validation helpers ------------------------------------------------

func envViews(env map[string]string) []EnvVarView {
	out := make([]EnvVarView, 0, len(env))
	for k, v := range env {
		out = append(out, EnvVarView{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func envKeyViews(env map[string]string) []EnvVarView {
	out := envViews(env)
	for i := range out {
		out[i].Value = "" // keys only
	}
	return out
}

func fileNameViews(files map[string]string) []SecretFileView {
	out := make([]SecretFileView, 0, len(files))
	for name := range files {
		out = append(out, SecretFileView{Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func addString(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func removeString(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
