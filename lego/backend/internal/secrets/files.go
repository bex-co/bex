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
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return nil, err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
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
// names, the exact env-vars pattern (ListEnvVarsPage) applied to the sibling
// route: limit == 0 with no cursor preserves the original unbounded-list
// behavior; cursor with no limit uses Render's default; a positive limit is
// clamped to Render's public API maximum. after is the prior page's item cursor
// (the file name), stable across interleaved writes because ListSecretFiles
// returns a name-sorted slice.
func (s *Service) ListSecretFilesPage(ctx context.Context, service, after string, limit int) ([]SecretFileView, error) {
	files, err := s.ListSecretFiles(ctx, service)
	if err != nil {
		return nil, err
	}
	if limit == 0 && after == "" {
		return files, nil
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
	return core.Page(files, after, limit, func(f SecretFileView) string { return f.Name }), nil
}

// GetSecretFile returns one file's name + content (Render's GET
// .../secret-files/{name}). Unknown service or file => core.ErrNotFound. Sensitive.
func (s *Service) GetSecretFile(ctx context.Context, service, name string) (SecretFileView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, service)
	if err != nil {
		return SecretFileView{}, err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
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
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return SecretFileView{}, err
	}
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
	}
	name = strings.TrimSpace(name)
	if !core.ValidSecretFileName(name) {
		// Name only in the error — never the content.
		return SecretFileView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
	}
	files, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return SecretFileView{}, err
	}
	files[name] = content
	if err := s.storeMap(ctx, filesPath(service), files); err != nil {
		return SecretFileView{}, err
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
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return core.ErrSecretsUnavailable
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
	files, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return err
	}
	for _, f := range initial {
		files[f.Name] = f.Content
	}
	if err := s.storeMap(ctx, filesPath(service), files); err != nil {
		return err
	}
	return s.materializeFiles(ctx, a, files)
}

// PrepareSecretFiles persists and materializes create-time files before the App
// CR exists, then points the App spec at that projection. That ordering is what
// guarantees the operator's first Deployment already mounts /etc/secrets; a
// post-create patch could lose the race with the first reconciliation.
//
// The Secret is deliberately ownerless for this short prepare window because
// Kubernetes has not assigned the App UID yet. CommitSecretFiles adopts it as
// soon as the App create succeeds; AbortSecretFiles removes both writes on any
// failure.
func (s *Service) prepareSecretFiles(ctx context.Context, service string, a *appv1alpha1.App, initial []core.SecretFile) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	files := make(map[string]string, len(initial))
	for _, f := range initial {
		name := strings.TrimSpace(f.Name)
		if !core.ValidSecretFileName(name) {
			return fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
		}
		files[name] = f.Content
	}
	if len(files) == 0 {
		return nil
	}
	if err := s.storeMap(ctx, filesPath(service), files); err != nil {
		return err
	}
	data := make(map[string][]byte, len(files))
	for name, content := range files {
		data[name] = []byte(content)
	}
	name := filesSecretName(a.Name)
	sec := &corev1.Secret{
		// The pod projects this Secret as files (spec.filesFromSecrets) and later
		// owns it (commitSecretFiles' controller ref), so it MUST share the App's
		// namespace — the per-tenant `<ws>` namespace under ADR043. A cross-namespace
		// owner ref would also be garbage-collected.
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: a.Namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
	if err := s.Client.Create(ctx, sec); err != nil {
		if deleteErr := s.Store.Delete(ctx, filesPath(service)); deleteErr != nil {
			return errors.Join(err, fmt.Errorf("roll back secret-file store: %w", deleteErr))
		}
		return err
	}
	a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, name)
	return nil
}

// CommitSecretFiles adopts the prepared Secret after Kubernetes has assigned
// the App UID, restoring normal owner-reference garbage collection.
func (s *Service) commitSecretFiles(ctx context.Context, _ string, a *appv1alpha1.App) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	name := filesSecretName(a.Name)
	sec := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, sec); err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(a, sec, s.Client.Scheme()); err != nil {
		return err
	}
	return s.Client.Update(ctx, sec)
}

// AbortSecretFiles removes all state written by PrepareSecretFiles. Both
// operations are idempotent so callers can use it after any failed phase.
func (s *Service) abortSecretFiles(ctx context.Context, service string, a *appv1alpha1.App) error {
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	return errors.Join(
		s.deleteSecret(ctx, a.Namespace, filesSecretName(a.Name)),
		s.Store.Delete(ctx, filesPath(service)),
		s.Store.Delete(withTenant(ctx, baoTenant), filesPath(service)),
	)
}

// CreateSecretFileSeeder is the apps feature's create-time transaction seam.
// The implementation is intentionally a small adapter rather than exported
// methods on Service: these phases run only after apps.Create has performed the
// resource authorization and are not independent API verbs.
type CreateSecretFileSeeder interface {
	PrepareSecretFiles(context.Context, string, *appv1alpha1.App, []core.SecretFile) error
	CommitSecretFiles(context.Context, string, *appv1alpha1.App) error
	AbortSecretFiles(context.Context, string, *appv1alpha1.App) error
}

type createSecretFileSeeder struct{ service *Service }

func (s createSecretFileSeeder) PrepareSecretFiles(ctx context.Context, service string, a *appv1alpha1.App, files []core.SecretFile) error {
	return s.service.prepareSecretFiles(ctx, service, a, files)
}

func (s createSecretFileSeeder) CommitSecretFiles(ctx context.Context, service string, a *appv1alpha1.App) error {
	return s.service.commitSecretFiles(ctx, service, a)
}

func (s createSecretFileSeeder) AbortSecretFiles(ctx context.Context, service string, a *appv1alpha1.App) error {
	return s.service.abortSecretFiles(ctx, service, a)
}

// NewCreateSecretFileSeeder wraps Service for apps.Service wiring.
func NewCreateSecretFileSeeder(service *Service) CreateSecretFileSeeder {
	return createSecretFileSeeder{service: service}
}

// DeleteSecretFile removes one file (Render's DELETE .../secret-files/{name}),
// re-projecting the reduced set. Unknown file => core.ErrNotFound.
func (s *Service) DeleteSecretFile(ctx context.Context, service, name string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	service = storeServiceName(a, service)
	ctx = withTenant(ctx, storeTenant(a))
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	files, err := s.readMap(ctx, filesPath(service))
	if err != nil {
		return err
	}
	if _, ok := files[name]; !ok {
		return core.ErrNotFound
	}
	delete(files, name)
	if err := s.storeMap(ctx, filesPath(service), files); err != nil {
		return err
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
	for _, v := range list {
		if v == s {
			return list
		}
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
