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
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// files.go is the per-service secret-files half of the feature (Render's
// /v1/services/{id}/secret-files): named files whose contents live in OpenBao and
// are materialized into a per-app "<name>-files" Secret the operator projects into
// a read-only /etc/secrets volume (one file per name). Same store, same
// materialize-then-roll mechanism as env vars — the file's mount path is
// /etc/secrets/<name>. Values (contents) are gated + leak-disciplined exactly like
// env-var values.

// SecretFileView is the Render-shaped secret-file wire object. content is omitted
// from list responses (names only) and present on a single-file GET, mirroring
// env-vars' keys-first "Show secret" discipline.
type SecretFileView struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
}

// filesPath is a service's secret-files map key in the store.
func filesPath(service string) string { return "services/" + service + "/files" }

// filesSecretName is the per-app projection Secret the operator mounts at /etc/secrets.
func filesSecretName(service string) string { return service + "-files" }

// ListSecretFiles returns a service's secret-file names, sorted (Render's GET
// .../secret-files). Names only — contents are fetched per file. Sensitive read.
func (s *Service) ListSecretFiles(ctx context.Context, service string) ([]SecretFileView, error) {
	_, ctx, service, err := s.scope(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return nil, err
	}
	files, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return nil, err
	}
	names := make([]SecretFileView, 0, len(files))
	for name := range files {
		names = append(names, SecretFileView{Name: name}) // names only
	}
	sort.Slice(names, func(i, j int) bool { return names[i].Name < names[j].Name })
	return names, nil
}

// ListSecretFilesPage returns a stable keyset page of a service's secret-file
// names, the env-vars pattern (ListEnvVarsPage) applied to the sibling route.
// after is the prior page's item cursor (the file name), stable across
// interleaved writes because ListSecretFiles returns a name-sorted slice.
// Paging policy: see applyPageLimits.
func (s *Service) ListSecretFilesPage(ctx context.Context, service, after string, limit int) ([]SecretFileView, error) {
	files, err := s.ListSecretFiles(ctx, service)
	if err != nil {
		return nil, err
	}
	return applyPageLimits(files, after, limit, func(f SecretFileView) string { return f.Name })
}

// GetSecretFile returns one file's name + content (Render's GET
// .../secret-files/{name}). Unknown service or file => core.ErrNotFound. Sensitive.
func (s *Service) GetSecretFile(ctx context.Context, service, name string) (SecretFileView, error) {
	a, ctx, service, err := s.scope(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return SecretFileView{}, err
	}
	// codex round-8 #8: secret-file content is a reveal — re-assert uncached so
	// a revocation inside PositiveTTL cannot surface one last file body.
	if err := s.AuthorizeAppFresh(ctx, core.RelCanViewSensitive, a); err != nil {
		return SecretFileView{}, err
	}
	files, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return SecretFileView{}, err
	}
	content, ok := files[name]
	if !ok {
		return SecretFileView{}, core.ErrNotFound
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// SetSecretFile adds or updates one secret file (Render's PUT
// .../secret-files/{name}, body {content}), merging into the existing set. Returns
// the file's name (content echoed). Manage-scope verb; the pods roll to pick up
// the change.
func (s *Service) SetSecretFile(ctx context.Context, service, name, content string) (SecretFileView, error) {
	a, ctx, service, err := s.scope(ctx, core.RelCanCreate, service)
	if err != nil {
		return SecretFileView{}, err
	}
	name = strings.TrimSpace(name)
	if !core.ValidSecretFileName(name) {
		// Name only in the error — never the content.
		return SecretFileView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
	}
	// codex-security round-19 #7: CAS through updateMapCAS (like SetEnvVar)
	// instead of a bare readMap+storeMap — a concurrent writer's whole-map
	// replacement between the read and this write could otherwise be silently
	// discarded (lost update).
	var quota error
	files, err := s.updateMapCAS(ctx, filesPath(service), func(current map[string]string) bool {
		if v, ok := current[name]; ok && v == content {
			return false // no change
		}
		current[name] = content
		// Round-11 #6: bound the aggregate map before the source-of-truth write so
		// a tenant cannot grow it without limit (and so a passing map can always
		// materialize under Kubernetes' 1 MiB Secret ceiling). Re-checked against
		// the fresh map on every CAS retry, matching SetEnvVar.
		if err := filesMapWithinQuota(current); err != nil {
			quota = err
			return false
		}
		return true
	})
	if err != nil {
		return SecretFileView{}, err
	}
	if quota != nil {
		return SecretFileView{}, quota
	}
	if err := s.materializeFiles(ctx, a, files); err != nil {
		return SecretFileView{}, err
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// SeedSecretFiles persists the official CLI's create-time secretFiles payload
// in one write and materializes the service's files Secret. All names are
// validated before any mutation, so one bad entry cannot partially seed the
// request. Existing values are merged for idempotent/retry-safe behavior.
func (s *Service) SeedSecretFiles(ctx context.Context, service string, initial []core.SecretFile) error {
	a, ctx, service, err := s.scope(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	if len(initial) == 0 {
		return nil
	}
	for i := range initial {
		initial[i].Name = strings.TrimSpace(initial[i].Name)
		if !core.ValidSecretFileName(initial[i].Name) {
			return fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, initial[i].Name)
		}
	}
	// codex-security round-19 #7: CAS through updateMapCAS instead of a bare
	// readMap+storeMap, so a concurrent Set/DeleteSecretFile between the read
	// and this write can't be clobbered.
	var quota error
	files, err := s.updateMapCAS(ctx, filesPath(service), func(current map[string]string) bool {
		for _, f := range initial {
			current[f.Name] = f.Content
		}
		if err := filesMapWithinQuota(current); err != nil {
			quota = err
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	if quota != nil {
		return quota
	}
	return s.materializeFiles(ctx, a, files)
}

// prepareSecretFiles persists and materializes create-time files before the App
// CR exists, then points the App spec at that projection. That ordering is what
// guarantees the operator's first Deployment already mounts /etc/secrets; a
// post-create patch could lose the race with the first reconciliation. The
// ownerless-then-adopt mechanics live in prepareProjection.
func (s *Service) prepareSecretFiles(ctx context.Context, service string, a *appv1alpha1.App, initial []core.SecretFile) error {
	if len(initial) == 0 {
		return nil
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	ctx, service = scopeApp(ctx, a, service)
	files := make(map[string]string, len(initial))
	for _, f := range initial {
		name := strings.TrimSpace(f.Name)
		if !core.ValidSecretFileName(name) {
			return fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
		}
		files[name] = f.Content
	}
	if err := filesMapWithinQuota(files); err != nil {
		return err
	}
	name := filesSecretName(a.Name)
	if err := s.prepareProjection(ctx, a, name, filesPath(service), files); err != nil {
		return err
	}
	a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, name)
	return nil
}

// prepareProjection is the write tail both create-time prepare phases share:
// the map into the store (source of truth), then its projection Secret — and
// the store path rolled back if that Secret cannot be written, so a failed
// prepare leaves nothing behind either side.
//
// The Secret is deliberately OWNERLESS for this prepare window because
// Kubernetes has not assigned the App UID yet; adoptPreparedSecret restores
// normal owner-reference garbage collection once the App create succeeds. The
// caller points the App spec at `name` afterward — that reference is what tells
// the commit phase which legs actually ran.
func (s *Service) prepareProjection(ctx context.Context, a *appv1alpha1.App, name, path string, values map[string]string) error {
	if err := s.storeMap(ctx, path, values); err != nil {
		return err
	}
	data := make(map[string][]byte, len(values))
	for key, value := range values {
		data[key] = []byte(value)
	}
	sec := &corev1.Secret{
		// The pod consumes this Secret (envFrom for env vars, a projected volume
		// for files) and later owns it, so it MUST share the App's namespace — the
		// per-tenant `<ws>` namespace under ADR043. A cross-namespace owner ref
		// would also be garbage-collected.
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: a.Namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
	if err := s.Client.Create(ctx, sec); err != nil {
		if deleteErr := s.Store.Delete(ctx, path); deleteErr != nil {
			return errors.Join(err, fmt.Errorf("roll back %s: %w", path, deleteErr))
		}
		return err
	}
	return nil
}

// adoptPreparedSecret restores owner-reference garbage collection on one
// prepared Secret once Kubernetes has assigned the App its UID.
func (s *Service) adoptPreparedSecret(ctx context.Context, a *appv1alpha1.App, name string) error {
	sec := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, sec); err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(a, sec, s.Client.Scheme()); err != nil {
		return err
	}
	return s.Client.Update(ctx, sec)
}

// abortProjection removes both halves of one prepared leg. Each operation is
// idempotent, so callers can use it after any failed phase.
func (s *Service) abortProjection(ctx context.Context, a *appv1alpha1.App, name, path string) error {
	return errors.Join(
		s.deleteSecret(ctx, a.Namespace, name),
		s.Store.Delete(ctx, path),
	)
}

func (s *Service) commitSecretFiles(ctx context.Context, _ string, a *appv1alpha1.App) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	return s.adoptPreparedSecret(ctx, a, filesSecretName(a.Name))
}

func (s *Service) abortSecretFiles(ctx context.Context, service string, a *appv1alpha1.App) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	ctx, service = scopeApp(ctx, a, service)
	return s.abortProjection(ctx, a, filesSecretName(a.Name), filesPath(service))
}

// prepareCreateEnvVars is the env-var twin of prepareSecretFiles (w6/m45): a
// create request's literal env vars land in the mutable env store, and their
// projection Secret is written and referenced from the spec, BEFORE the App CR
// exists — so the operator's first Deployment already carries them and the
// Environment tab / envVars API see them the moment create returns. Seeding
// them post-create instead would both race the first reconcile and roll the
// pods a second time.
func (s *Service) prepareCreateEnvVars(ctx context.Context, service string, a *appv1alpha1.App, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	ctx, service = scopeApp(ctx, a, service)
	for _, key := range core.SortedKeys(env) {
		if !core.ValidEnvKey(key) {
			// Names only in the error — never the value (docs/ADR013-secrets.md).
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
	}
	if err := envMapWithinQuota(env); err != nil {
		return err
	}
	name := envSecretName(a.Name)
	if err := s.prepareProjection(ctx, a, name, envPath(service), env); err != nil {
		return err
	}
	a.Spec.EnvFromSecret = name
	return nil
}

func (s *Service) abortCreateEnvVars(ctx context.Context, service string, a *appv1alpha1.App) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	ctx, service = scopeApp(ctx, a, service)
	return s.abortProjection(ctx, a, envSecretName(a.Name), envPath(service))
}

// CreateSecretsSeeder is the apps feature's create-time transaction seam: the
// secret files AND literal env vars a new service is born with, written before
// the App CR so its very first pod already carries them. The implementation is
// intentionally a small adapter rather than exported methods on Service: these
// phases run only after apps.Create has performed the resource authorization
// and are not independent API verbs.
type CreateSecretsSeeder interface {
	PrepareCreateSecrets(context.Context, string, *appv1alpha1.App, []core.SecretFile, map[string]string) error
	CommitCreateSecrets(context.Context, string, *appv1alpha1.App) error
	AbortCreateSecrets(context.Context, string, *appv1alpha1.App) error
}

type createSecretsSeeder struct{ service *Service }

// PrepareCreateSecrets runs both legs; each is a no-op for an empty input, so a
// create carrying only one of the two writes only that one.
func (s createSecretsSeeder) PrepareCreateSecrets(ctx context.Context, service string, a *appv1alpha1.App, files []core.SecretFile, env map[string]string) error {
	if err := s.service.prepareSecretFiles(ctx, service, a, files); err != nil {
		return err
	}
	return s.service.prepareCreateEnvVars(ctx, service, a, env)
}

// CommitCreateSecrets adopts exactly the projections prepare actually wrote —
// identified by the spec references it set, so a create with only one of the
// two legs never looks for a Secret that was never prepared.
func (s createSecretsSeeder) CommitCreateSecrets(ctx context.Context, service string, a *appv1alpha1.App) error {
	var errs []error
	if slices.Contains(a.Spec.FilesFromSecrets, filesSecretName(a.Name)) {
		errs = append(errs, s.service.commitSecretFiles(ctx, service, a))
	}
	if a.Spec.EnvFromSecret == envSecretName(a.Name) {
		errs = append(errs, s.service.adoptPreparedSecret(ctx, a, envSecretName(a.Name)))
	}
	return errors.Join(errs...)
}

// AbortCreateSecrets removes every write PrepareCreateSecrets could have made.
// Each operation is idempotent, so it is safe after any failed phase — and
// unconditional, because the App it rolls back is brand new and therefore owns
// nothing either path could be destroying.
func (s createSecretsSeeder) AbortCreateSecrets(ctx context.Context, service string, a *appv1alpha1.App) error {
	return errors.Join(
		s.service.abortSecretFiles(ctx, service, a),
		s.service.abortCreateEnvVars(ctx, service, a),
	)
}

// NewCreateSecretsSeeder wraps Service for apps.Service wiring.
func NewCreateSecretsSeeder(service *Service) CreateSecretsSeeder {
	return createSecretsSeeder{service: service}
}

// DeleteSecretFile removes one file (Render's DELETE .../secret-files/{name}),
// re-projecting the reduced set. Unknown file => core.ErrNotFound.
func (s *Service) DeleteSecretFile(ctx context.Context, service, name string) error {
	a, ctx, service, err := s.scope(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	// codex-security round-19 #7: CAS through updateMapCAS (like DeleteEnvVar)
	// instead of a bare readMap+storeMap, so a concurrent SetSecretFile/
	// DeleteSecretFile between the read and this write can't be clobbered.
	var nameFound bool
	files, err := s.updateMapCAS(ctx, filesPath(service), func(current map[string]string) bool {
		if _, ok := current[name]; !ok {
			return false
		}
		nameFound = true
		delete(current, name)
		return true
	})
	if err != nil {
		return err
	}
	if !nameFound {
		return core.ErrNotFound
	}
	return s.materializeFiles(ctx, a, files)
}

// --- core.SecretFileReader: the seam apps' GraphQL uses to nest secret files ----

// SecretFileNames lists a service's secret-file names only (content empty), the
// dashboard shape (`service{ secretFileNames{ id name } }`). id == name.
func (s *Service) SecretFileNames(ctx context.Context, service string) ([]core.SecretFile, error) {
	files, err := s.ListSecretFiles(ctx, service)
	if err != nil {
		return nil, err
	}
	out := make([]core.SecretFile, 0, len(files))
	for _, f := range files {
		out = append(out, core.SecretFile{ID: f.Name, Name: f.Name})
	}
	return out, nil
}

// SecretFileContent reads one file's content (the dashboard's "Show contents").
func (s *Service) SecretFileContent(ctx context.Context, service, name string) (core.SecretFile, error) {
	f, err := s.GetSecretFile(ctx, service, name)
	if err != nil {
		return core.SecretFile{}, err
	}
	return core.SecretFile{ID: f.Name, Name: f.Name, Content: f.Content}, nil
}

// materializeFiles projects a service's secret files into its <service>-files
// Secret and ensures the App mounts it, rolling the pods. When the set empties the
// Secret is deleted and the reference removed, so no empty /etc/secrets mount
// lingers. The operator merges this Secret with any linked env-group file Secrets
// into the single /etc/secrets projected volume (docs/ADR013-secrets.md).
func (s *Service) materializeFiles(ctx context.Context, a *appv1alpha1.App, files map[string]string) error {
	base := client.MergeFrom(a.DeepCopy())
	if err := s.projectFiles(ctx, a, files); err != nil {
		return err
	}
	s.bumpRestart(a)
	return s.Client.Patch(ctx, a, base)
}

// projectFiles updates the derived Kubernetes Secret and App reference without
// changing restartedAt or persisting the App. It is the no-roll primitive used
// by PatchEnvironment; materializeFiles layers the legacy immediate rollout on
// top for existing clients.
func (s *Service) projectFiles(ctx context.Context, a *appv1alpha1.App, files map[string]string) error {
	name := filesSecretName(a.Name)
	if len(files) == 0 {
		if err := s.deleteSecret(ctx, a.Namespace, name); err != nil {
			return err
		}
		a.Spec.FilesFromSecrets = removeString(a.Spec.FilesFromSecrets, name)
	} else {
		if err := s.upsertSecret(ctx, a, name, files); err != nil {
			return err
		}
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, name)
	}
	return nil
}

// deleteSecret removes a projection Secret by name in namespace (idempotent —
// absence is fine). namespace is the App's namespace (its pod mounted the
// Secret), the per-tenant `<ws>` namespace under ADR043; callers pass a.Namespace.
func (s *Service) deleteSecret(ctx context.Context, namespace, name string) error {
	sec := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, sec); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return err
	}
	if err := s.Client.Delete(ctx, sec); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// addString returns list with s present exactly once (order-stable, append if new).
func addString(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// removeString returns list without any occurrence of s.
func removeString(list []string, s string) []string {
	out := list[:0:0]
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
