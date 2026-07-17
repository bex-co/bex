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

package registrycreds

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pullsecret.go materializes a workspace's stored registry credential into a
// per-App `kubernetes.io/dockerconfigjson` Secret (w2/m14/t002) — the same
// "bex-api creates the opaque Secret, the operator only references it"
// pattern the git clone-secret path already established (apps'
// copyCloneSecret). The operator stays registry-unaware: it never resolves
// which credential (if any) matches an image, it only wires
// app.Spec.ExternalRegistryPullSecret into imagePullSecrets when set.

// pullSecretName is the per-app Secret name bex-api materializes and the
// operator references — deterministic so re-materializing on every
// create/update upserts the same object rather than accumulating stale ones.
func pullSecretName(appName string) string { return appName + "-registry-pull" }

// registryHost resolves the registry hostname a plain image reference pulls
// against, following Docker's own reference-parsing rule: the first "/"-
// delimited segment is a registry host only if it contains a "." or ":" or is
// exactly "localhost" — any other shape ("nginx:latest", "acme/app:1.0") is a
// Docker Hub repository name with no explicit host, so it resolves against
// Docker Hub's canonical host.
func registryHost(image string) string {
	first, _, found := strings.Cut(image, "/")
	if found && (first == "localhost" || strings.ContainsAny(first, ".:")) {
		return first
	}
	return "docker.io"
}

// materializePullSecret ensures app has an up-to-date dockerconfigjson Secret
// for image's registry host, if the workspace has a matching stored
// credential — the create/update-time hook apps.Service calls before writing
// the App CR (mirrors the clone-secret path). ok=false, nil error means the
// workspace has no credential for that host: a public image, or one from a
// registry the workspace hasn't stored credentials for — left untouched, not
// an error. A non-nil error means a credential DID match but materializing it
// failed, which the caller must surface rather than silently deploy without
// the pull secret it needs.
//
// Unexported and reached only through pullSecretSource (DeployPullSecretSource)
// on purpose: it authenticates via the deploy path's own authorization, not
// an Authorize call, so exposing it as a public Service verb would (rightly)
// trip TestAuthzGuardsEveryVerb — the same reason github.Service's cloneToken
// is unexported behind tokenSource.
func (s *Service) materializePullSecret(ctx context.Context, workspaceID string, app *appv1alpha1.App, image string, credentialID *string) (secretName string, ok bool, err error) {
	if credentialID != nil && strings.TrimSpace(*credentialID) == "" {
		return "", false, nil // explicit clear: do not fall back to host matching
	}
	if !s.configured() {
		if credentialID != nil {
			return "", false, core.ErrRegistryCredentialsUnavailable
		}
		return "", false, nil // legacy auto-match remains optional when unwired
	}
	host := ""
	if strings.TrimSpace(image) != "" {
		host = registryHost(image)
	}
	var cred store.RegistryCredential
	var lookupErr error
	if credentialID != nil {
		id := strings.TrimSpace(*credentialID)
		cred, lookupErr = s.Store.GetRegistryCredentialByID(ctx, id)
		if lookupErr == nil && cred.WorkspaceID != workspaceID {
			return "", false, fmt.Errorf("%w: registry credential %q does not belong to the target workspace", core.ErrForbidden, id)
		}
		if lookupErr == nil && host != "" && !strings.EqualFold(cred.Host, host) {
			return "", false, fmt.Errorf("%w: registry credential %q is for %s, not %s", core.ErrBadRequest, id, cred.Host, host)
		}
		if lookupErr == nil && host == "" {
			host = cred.Host // explicit Docker-build binding: FROM host is not known at create time
		}
	} else {
		if host == "" {
			return "", false, nil // repo builds never guess which private FROM registry to use
		}
		cred, lookupErr = s.Store.GetRegistryCredentialByHost(ctx, workspaceID, host)
	}
	if lookupErr != nil {
		if errors.Is(lookupErr, store.ErrNotFound) {
			if credentialID != nil {
				return "", false, core.ErrNotFound
			}
			return "", false, nil
		}
		return "", false, lookupErr
	}
	secretMap, err := s.Secret.Get(ctx, secretPath(workspaceID, cred.ID))
	if err != nil {
		return "", false, err
	}
	password, ok := secretMap["password"]
	if !ok {
		// The metadata row exists but its OpenBao secret is gone (deleted
		// out-of-band, or a prior create's secret write rolled back after this
		// read raced it) — refuse rather than materialize a Secret with an empty
		// password that would just fail every pull.
		return "", false, core.Err("registry credential " + cred.ID + " has no stored secret")
	}
	name := pullSecretName(app.Name)
	auths := dockerConfigJSON(host, cred.Username, password)
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	// No owner reference, deliberately: this can run as part of a brand-new
	// App's own Create call, before the App has a UID an owner reference could
	// point at (an owner reference with an empty UID is rejected by the API
	// server). Mirrors apps' clone-secret Secret, which has the identical
	// constraint and the identical fix — apps.Service.Delete explicitly deletes
	// this Secret by its deterministic name instead of relying on the App CR's
	// delete cascade to reach it.
	if _, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeDockerConfigJson
		sec.Data = map[string][]byte{corev1.DockerConfigJsonKey: auths}
		return nil
	}); err != nil {
		return "", false, err
	}
	return name, true, nil
}

// dockerConfigJSON builds the .dockerconfigjson payload kubelet/containerd
// read for registry auth: {"auths":{"<host>":{"username","password","auth"}}}
// — "auth" is the base64(username:password) form some pullers still expect
// alongside the plain fields.
func dockerConfigJSON(host, username, password string) []byte {
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	doc := map[string]any{
		"auths": map[string]any{
			host: map[string]string{
				"username": username,
				"password": password,
				"auth":     auth,
			},
		},
	}
	b, _ := json.Marshal(doc) // fixed shape, string values — cannot fail
	return b
}

// pullSecretSource adapts Service to apps.PullSecretSource (mirrors
// github.tokenSource) without exposing materializePullSecret as a public,
// authz-swept Service verb — the interface method itself must be exported for
// cross-package satisfaction, but the wrapper type is not a Service verb.
type pullSecretSource struct{ s *Service }

// MaterializePullSecret satisfies apps.PullSecretSource.
func (p pullSecretSource) MaterializePullSecret(ctx context.Context, workspaceID string, app *appv1alpha1.App, image string, credentialID *string) (string, bool, error) {
	return p.s.materializePullSecret(ctx, workspaceID, app, image, credentialID)
}

// RegistryCredentialName supplies the safe summary metadata Render includes
// on service reads. Keep this on the deploy adapter rather than exporting a
// second unguarded Service verb; the caller has already authorized the App.
func (p pullSecretSource) RegistryCredentialName(ctx context.Context, workspaceID, credentialID string) (string, bool) {
	if p.s.Store == nil {
		return "", false
	}
	credential, err := p.s.Store.GetRegistryCredential(ctx, workspaceID, credentialID)
	if err != nil {
		return "", false
	}
	return credential.Name, true
}

// ResolveCredentialNames batch-resolves display names for a slice of credential
// ids (one query for the whole page). Unknown ids are silently omitted. Callers
// have already authorized each App that owns these ids.
func (p pullSecretSource) ResolveCredentialNames(ctx context.Context, ids []string) map[string]string {
	if p.s.Store == nil || len(ids) == 0 {
		return nil
	}
	creds, err := p.s.Store.GetRegistryCredentialsByIDs(ctx, ids)
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(creds))
	for _, c := range creds {
		out[c.ID] = c.Name
	}
	return out
}

// DeployPullSecretSource returns the deploy path's pull-secret seam (wired
// onto apps.Service in the composition root).
func (s *Service) DeployPullSecretSource() pullSecretSource { return pullSecretSource{s} }
