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

package apps

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// PullSecretSource resolves a workspace's stored external-registry credential
// into a materialized pull Secret for an App's image (registrycreds.Service
// satisfies it — w2/m14). nil on the apps Service => registry credentials are
// off, unchanged (no external pull secret is ever set).
type PullSecretSource interface {
	// MaterializePullSecret upserts (and returns the name of) a
	// dockerconfigjson Secret in a's namespace for image's registry host, if
	// workspaceID has a matching stored credential. ok=false (nil err) means
	// no credential matched — a public image, or one from a registry the
	// workspace hasn't stored credentials for — leave alone. A non-nil err
	// means a credential DID match but materializing it failed, which the
	// caller must surface.
	MaterializePullSecret(ctx context.Context, workspaceID string, a *appv1alpha1.App, image string) (secretName string, ok bool, err error)
}

// ensureExternalRegistryPullSecret resolves and materializes (when the
// workspace has a matching stored credential) the pull Secret for an App's
// image, returning the name to set as spec.externalRegistryPullSecret.
// Mirrors ensureCloneSecret's shape: the caller sets the spec field itself
// (as part of its own patch/create diff), this only materializes the Secret
// and resolves the name.
//
// Only image-backed Apps have an external registry host to match — a
// build-from-git App's image always lands in the internal Zot registry,
// which the operator's OWN pull-secret path already covers (w7/m8). Returns
// "" (no error) when RegistryCreds is off, the App has no Image, or no
// credential matches — the common case, left untouched.
func (s *Service) ensureExternalRegistryPullSecret(ctx context.Context, a *appv1alpha1.App) (string, error) {
	if s.RegistryCreds == nil || a.Spec.Image == "" {
		return "", nil
	}
	name, ok, err := s.RegistryCreds.MaterializePullSecret(ctx, s.deployWorkspace(ctx, a), a, a.Spec.Image)
	if err != nil {
		return "", fmt.Errorf("materializing registry pull secret for %s: %w", a.Spec.Image, err)
	}
	if !ok {
		return "", nil
	}
	return name, nil
}

// externalRegistryPullSecretName is the deterministic name of an App's
// materialized registry-pull Secret — MUST match registrycreds' own
// pullSecretName (unexported to that package; apps can't import it, so this
// mirrors the exact same "<app>-registry-pull" convention).
func externalRegistryPullSecretName(app string) string { return app + "-registry-pull" }

// deleteExternalRegistryPullSecret removes an App's registry-pull Secret on
// service delete. Best-effort: an absent Secret (no credential ever matched
// this App's image, or registry credentials are off) is not an error. Needed
// because the Secret carries no ownerRef (see registrycreds.materializePullSecret's
// doc comment) — the App CR's delete cascade wouldn't reach it otherwise.
func (s *Service) deleteExternalRegistryPullSecret(ctx context.Context, namespace, app string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: externalRegistryPullSecretName(app), Namespace: namespace}}
	return client.IgnoreNotFound(s.Client.Delete(ctx, sec))
}
