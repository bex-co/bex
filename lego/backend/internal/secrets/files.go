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
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	if _, err := s.GetApp(ctx, core.RelCanViewSensitive, service); err != nil {
		return nil, err
	}
	files, err := s.Store.Get(ctx, filesPath(service))
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

// GetSecretFile returns one file's name + content (Render's GET
// .../secret-files/{name}). Unknown service or file => core.ErrNotFound. Sensitive.
func (s *Service) GetSecretFile(ctx context.Context, service, name string) (SecretFileView, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return SecretFileView{}, err
	}
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
	}
	if _, err := s.GetApp(ctx, core.RelCanViewSensitive, service); err != nil {
		return SecretFileView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(service))
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
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return SecretFileView{}, err
	}
	if s.Store == nil {
		return SecretFileView{}, core.ErrSecretsUnavailable
	}
	name = strings.TrimSpace(name)
	if !core.ValidSecretFileName(name) {
		// Name only in the error — never the content.
		return SecretFileView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
	}
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return SecretFileView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(service))
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

// DeleteSecretFile removes one file (Render's DELETE .../secret-files/{name}),
// re-projecting the reduced set. Unknown file => core.ErrNotFound.
func (s *Service) DeleteSecretFile(ctx context.Context, service, name string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	files, err := s.Store.Get(ctx, filesPath(service))
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
	name := filesSecretName(a.Name)
	base := client.MergeFrom(a.DeepCopy())
	if len(files) == 0 {
		if err := s.deleteSecret(ctx, name); err != nil {
			return err
		}
		a.Spec.FilesFromSecrets = removeString(a.Spec.FilesFromSecrets, name)
	} else {
		if err := s.upsertSecret(ctx, a, name, files); err != nil {
			return err
		}
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, name)
	}
	s.bumpRestart(a)
	return s.Client.Patch(ctx, a, base)
}

// deleteSecret removes a projection Secret by name (idempotent — absence is fine).
func (s *Service) deleteSecret(ctx context.Context, name string) error {
	sec := &corev1.Secret{}
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, sec); err != nil {
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
