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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CloneTokenSource resolves a private-repo clone token for a deploy
// (github.Service satisfies it). nil on the apps Service => GitHub integration is
// off and every deploy uses today's public-clone path, unchanged.
type CloneTokenSource interface {
	// CloneToken returns a fresh installation token to clone repoURL if it
	// belongs to workspaceID's GitHub connection. ok=false (nil err) => not a
	// connected repo (leave cloneSecret alone). A non-nil err is a GitHub failure
	// the caller must not swallow into a public-clone of a private repo.
	CloneToken(ctx context.Context, workspaceID, repoURL string) (token string, ok bool, err error)
}

// deployWorkspace resolves the workspace whose GitHub connection owns this App's
// clone token. The App's own tenant label (stamped by create / the control-plane
// projector) takes precedence — so a no-identity trigger (the push webhook) still
// resolves the right connection instead of falling back to the default workspace
// — then the caller's tenant, then the single-workspace default.
func (s *Service) deployWorkspace(ctx context.Context, a *appv1alpha1.App) string {
	if t := a.Labels[core.LabelTenant]; t != "" {
		return t
	}
	if t, ok := s.Tenant(ctx); ok && t != "" {
		return t
	}
	return core.DefaultTenant
}

// cloneSecretLabel marks the Secrets this feature writes, so the delete cascade
// (and an operator audit) can find them.
const cloneSecretManagedBy = "bex-api"

// cloneSecretName is the deterministic name of an App's clone-credential Secret.
func cloneSecretName(app string) string { return app + "-clone" }

// ensureCloneSecret, for an App about to be (re)deployed, mints a fresh
// installation token when the App's repo belongs to the workspace's GitHub
// connection, writes/refreshes the <app>-clone Secret in the App's namespace,
// and returns that Secret's name to set as spec.cloneSecret. It does NOT mutate
// the App (the caller sets spec.cloneSecret inside its patch mutator, so the
// change is part of the diff).
//
// Returns "" (no error) for public/unconnected repos or when GitHub is off — the
// public-clone path, unchanged. A GitHub failure returns an error: a private
// repo must fail loudly, never fall back to an unauthenticated clone.
func (s *Service) ensureCloneSecret(ctx context.Context, a *appv1alpha1.App) (string, error) {
	if s.GitHub == nil || a.Spec.Repo == "" {
		return "", nil
	}
	token, ok, err := s.GitHub.CloneToken(ctx, s.deployWorkspace(ctx, a), a.Spec.Repo)
	if err != nil {
		return "", fmt.Errorf("minting github clone token for %s: %w", a.Spec.Repo, err)
	}
	if !ok {
		return "", nil
	}
	name := cloneSecretName(a.Name)
	if err := s.writeCloneSecret(ctx, a.Namespace, name, a.Name, token); err != nil {
		return "", fmt.Errorf("writing clone secret %s/%s: %w", a.Namespace, name, err)
	}
	return name, nil
}

// writeCloneSecret upserts the Opaque <app>-clone Secret holding the git token
// under key "token". Labeled managed-by bex-api + the owning app for cleanup.
func (s *Service) writeCloneSecret(ctx context.Context, namespace, name, app, token string) error {
	// CreateOrUpdate is the module's Secret-upsert idiom (see secrets/envgroups):
	// one idempotent round-trip that both creates the Secret and refreshes the
	// token on every deploy trigger. No ownerRef — the clone Secret is deleted
	// explicitly by deleteCloneSecret (a service outlives any single deploy).
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		if sec.Labels == nil {
			sec.Labels = map[string]string{}
		}
		sec.Labels["app.kubernetes.io/managed-by"] = cloneSecretManagedBy
		sec.Labels["app.bex.co/app"] = app
		sec.Type = corev1.SecretTypeOpaque
		sec.StringData = map[string]string{"token": token}
		return nil
	})
	return err
}

// deleteCloneSecret removes an App's clone Secret on service delete. Best-effort:
// an absent Secret (public app, or already gone) is not an error.
func (s *Service) deleteCloneSecret(ctx context.Context, namespace, app string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cloneSecretName(app), Namespace: namespace}}
	return client.IgnoreNotFound(s.Client.Delete(ctx, sec))
}
