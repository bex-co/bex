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
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// PullSecretSource resolves a workspace's stored external-registry credential
// into a materialized docker-config Secret for an App's runtime image or
// Dockerfile build (registrycreds.Service satisfies it — w2/m14, w6/m34). nil
// on the apps Service => registry credentials are off, unchanged.
type PullSecretSource interface {
	// MaterializePullSecret upserts (and returns the name of) a dockerconfigjson
	// Secret in a's namespace. Image-backed services resolve the registry host
	// from image; Dockerfile builds pass an empty image and require an explicit
	// credentialID whose stored host becomes the config target. ok=false (nil err) means
	// no credential matched — a public image, or one from a registry the
	// workspace hasn't stored credentials for — leave alone. A non-nil err
	// means a credential DID match but materializing it failed, which the
	// caller must surface.
	// credentialID carries Render's explicit binding. nil preserves the legacy
	// host-match behavior; pointer-to-empty explicitly disables credentials.
	MaterializePullSecret(ctx context.Context, workspaceID string, a *appv1alpha1.App, image string, credentialID *string) (secretName string, ok bool, err error)
	// ResolveCredentialNames batch-resolves names for a slice of credential ids
	// (one query for the page). Unknown ids are silently omitted.
	ResolveCredentialNames(ctx context.Context, ids []string) map[string]string
}

// ensureExternalRegistryPullSecret resolves and materializes the docker-config
// Secret for an App's runtime image or Dockerfile build, returning the name to
// set as spec.externalRegistryPullSecret.
// Mirrors ensureCloneSecret's shape: the caller sets the spec field itself
// (as part of its own patch/create diff), this only materializes the Secret
// and resolves the name.
//
// Image-backed Apps retain the legacy host-match behavior. A repo-backed
// Dockerfile build has no image reference from which bex can infer the private
// FROM registry, so it only materializes an explicitly bound credential; the
// operator merges that config into BuildKit's daemon-only Docker config. Native
// and buildpack runtimes reject an explicit binding by name rather than
// accepting authentication they cannot use.
func (s *Service) ensureExternalRegistryPullSecret(ctx context.Context, a *appv1alpha1.App) (string, error) {
	credentialSet := a.Spec.RegistryCredentialID != nil && strings.TrimSpace(*a.Spec.RegistryCredentialID) != ""
	if a.Spec.Image == "" && !credentialSet {
		return "", nil
	}
	if a.Spec.Image == "" && !isDockerfileBuild(a.Spec) {
		return "", fmt.Errorf("%w: registryCredentialId only applies to an image-backed or Dockerfile-built service", core.ErrBadRequest)
	}
	if s.RegistryCreds == nil {
		if credentialSet {
			return "", core.ErrRegistryCredentialsUnavailable
		}
		return "", nil
	}
	name, ok, err := s.RegistryCreds.MaterializePullSecret(ctx, s.deployWorkspace(ctx, a), a, a.Spec.Image, a.Spec.RegistryCredentialID)
	if err != nil {
		return "", fmt.Errorf("materializing registry pull secret for %s: %w", a.Spec.Image, err)
	}
	if !ok {
		return "", nil
	}
	return name, nil
}

func isDockerfileBuild(spec appv1alpha1.AppSpec) bool {
	if spec.Repo == "" || spec.Type == appv1alpha1.TypeStaticSite {
		return false
	}
	runtime := strings.ToLower(strings.TrimSpace(spec.Runtime))
	if runtime != "" {
		return runtime == "docker"
	}
	builder := strings.ToLower(strings.TrimSpace(spec.Builder))
	return builder == "" || builder == "auto" || builder == "dockerfile"
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
