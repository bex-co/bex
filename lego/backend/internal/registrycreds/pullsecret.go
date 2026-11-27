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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pullsecret.go materializes a workspace's stored registry credential into a
// per-App, per-credential `kubernetes.io/dockerconfigjson` Secret (w2/m14/t002,
// w8/m529) — the same "bex-api creates the opaque Secret, the operator only
// references it" pattern the git clone-secret path already established (apps'
// copyCloneSecret). The operator stays registry-unaware: it never resolves
// which credential (if any) matches an image, it only wires the release's
// pull Secret (App.ReleasePullSecret) into imagePullSecrets when set.

// registryHost resolves the registry hostname a plain image reference pulls
// against, following Docker's own reference-parsing rule: the first "/"-
// delimited segment is a registry host only if it contains a "." or ":" or is
// exactly "localhost" — any other shape ("nginx:latest", "acme/app:1.0") is a
// Docker Hub repository name with no explicit host, so it resolves against
// Docker Hub's canonical host.
func registryHost(image string) string {
	first, _, found := strings.Cut(image, "/")
	if found && (first == "localhost" || strings.ContainsAny(first, ".:")) {
		return foldRegistryHost(first)
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
func (s *Service) resolvePullCredential(ctx context.Context, workspaceID, image string, credentialID *string) (host string, cred store.RegistryCredential, password string, ok bool, err error) {
	// No credential selected: the image is read anonymously, as on Render,
	// where a credential applies only to the services that choose it. A
	// workspace credential for the same host is never picked up by host
	// (w4/m301): it used to capture every public image from that host, so one
	// bad token broke unrelated deploys and could not be deleted. A blank id is
	// an explicit clear, which reads the same.
	if credentialID == nil || strings.TrimSpace(*credentialID) == "" {
		return "", store.RegistryCredential{}, "", false, nil
	}
	if !s.configured() {
		return "", store.RegistryCredential{}, "", false, core.ErrRegistryCredentialsUnavailable
	}
	if strings.TrimSpace(image) != "" {
		host = registryHost(image)
	}
	id := strings.TrimSpace(*credentialID)
	cred, err = s.Store.GetRegistryCredentialByID(ctx, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		// Names the id the caller sent, which may be a credential's name or a
		// stale id: the image is not what is missing (w8/354).
		return "", store.RegistryCredential{}, "", false, core.NotFound(fmt.Sprintf("registry credential %q", id))
	case err != nil:
		return "", store.RegistryCredential{}, "", false, err
	case cred.WorkspaceID != workspaceID:
		return "", store.RegistryCredential{}, "", false, fmt.Errorf("%w: registry credential %q does not belong to the target workspace", core.ErrForbidden, id)
	case host != "" && !sameRegistryHost(cred.Host, host):
		return "", store.RegistryCredential{}, "", false, fmt.Errorf("%w: registry credential %q is for %s, not %s", core.ErrBadRequest, id, cred.Host, host)
	case host == "":
		host = cred.Host // explicit Docker-build binding: FROM host is not known at create time
	}
	if password, err = s.storedSecret(ctx, workspaceID, cred.ID); err != nil {
		return "", store.RegistryCredential{}, "", false, err
	}
	return host, cred, password, true, nil
}

// storedSecret is a credential's password from OpenBao. A metadata row whose
// secret is gone (deleted out-of-band, or a prior create's secret write rolled
// back after this read raced it) is refused rather than used with an empty
// password that would just fail every pull.
func (s *Service) storedSecret(ctx context.Context, workspaceID, id string) (string, error) {
	secretMap, err := s.Secret.Get(ctx, secretPath(workspaceID, id))
	if err != nil {
		return "", err
	}
	password, ok := secretMap["password"]
	if !ok {
		return "", core.Err("registry credential " + id + " has no stored secret")
	}
	return password, nil
}

func (s *Service) materializePullSecret(ctx context.Context, workspaceID string, app *appv1alpha1.App, image string, credentialID *string) (secretName string, ok bool, err error) {
	host, cred, password, ok, err := s.resolvePullCredential(ctx, workspaceID, image, credentialID)
	if err != nil || !ok {
		return "", ok, err
	}
	// One Secret per credential: materializing B never rewrites the A that a
	// serving release, its Restart or a rollback to it pulls with (w8/m529).
	name := appv1alpha1.ExternalRegistryPullSecretName(app.Name, cred.ID)
	auths := dockerConfigJSON(host, cred.Username, password)
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	// No owner reference, deliberately: this can run as part of a brand-new
	// App's own Create call, before the App has a UID an owner reference could
	// point at (an owner reference with an empty UID is rejected by the API
	// server). Mirrors apps' clone-secret Secret, which has the identical
	// constraint and the identical fix: apps.Service.Delete deletes an App's
	// pull Secrets by LabelRegistryPullApp, and this package's Delete reaps a
	// credential's by LabelRegistryPullCredential.
	if _, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		// Round-11 #1: the workspace's external-registry credential is
		// operational — an App naming this Secret in envFrom would dump the
		// dockerconfig into its environment. The operator's App reconcile
		// refuses references to Secrets carrying this label; imagePullSecret
		// resolution ignores labels.
		if sec.Labels == nil {
			sec.Labels = map[string]string{}
		}
		sec.Labels[appv1alpha1.LabelProtectedFromTenantMount] = appv1alpha1.ProtectedFromTenantMount
		sec.Labels[appv1alpha1.LabelRegistryPullApp] = app.Name
		sec.Labels[appv1alpha1.LabelRegistryPullCredential] = cred.ID
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

// ValidatePullSecret satisfies apps.PullSecretSource without writing any pull
// Secret.
func (p pullSecretSource) ValidatePullSecret(ctx context.Context, workspaceID, image string, credentialID *string) error {
	_, _, _, _, err := p.s.resolvePullCredential(ctx, workspaceID, image, credentialID)
	return err
}

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
// ids owned by workspaceID (one query for the whole page). The workspace filter
// keeps enrichment within the caller's tenant — a foreign id resolves to
// nothing, not even its name. Unknown/foreign ids are silently omitted. Callers
// have already authorized each App that owns these ids.
func (p pullSecretSource) ResolveCredentialNames(ctx context.Context, workspaceID string, ids []string) map[string]string {
	if p.s.Store == nil || workspaceID == "" || len(ids) == 0 {
		return nil
	}
	creds, err := p.s.Store.GetRegistryCredentialsByIDs(ctx, workspaceID, ids)
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(creds))
	for _, c := range creds {
		out[c.ID] = c.Name
	}
	return out
}

// FindCredentialIDByName resolves a workspace registry credential by display
// name — the reference form render.yaml's fromRegistryCreds uses (w8/m19).
// found=false with nil err means no credential carries the name; two sharing
// it is an error (the reference would be ambiguous). Deploy-adapter seam like
// the rest: the blueprint caller has already authorized the workspace verb.
func (p pullSecretSource) FindCredentialIDByName(ctx context.Context, workspaceID, name string) (string, bool, error) {
	if p.s.Store == nil {
		return "", false, core.ErrRegistryCredentialsUnavailable
	}
	rows, err := p.s.Store.ListRegistryCredentials(ctx, workspaceID)
	if err != nil {
		return "", false, err
	}
	var id string
	for _, c := range rows {
		if c.Name != name {
			continue
		}
		if id != "" {
			return "", false, fmt.Errorf("%w: registry credential name %q is ambiguous (multiple credentials share it); bind by id through the service API instead", core.ErrBadRequest, name)
		}
		id = c.ID
	}
	return id, id != "", nil
}

// DeployPullSecretSource returns the deploy path's pull-secret seam (wired
// onto apps.Service in the composition root).
func (s *Service) DeployPullSecretSource() pullSecretSource { return pullSecretSource{s} }

// appBoundToCredential reports whether any App in workspaceID's namespace
// selects cred explicitly (spec.registryCredentialId), the only way an App
// uses a credential (w4/m301). A credential still bound is refused delete: the
// service would lose the credential its next deploy pulls with (w4/029.md
// #12). An App whose deletion has begun no longer counts: the service is gone
// on every read, and its Secrets are reaped with it (w4/599).
func (s *Service) appBoundToCredential(ctx context.Context, workspaceID string, cred store.RegistryCredential) (*appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	opts := []client.ListOption{client.InNamespace(s.AppNamespace(workspaceID))}
	// DefaultTenant is the legacy store-off/shared-namespace mode (mirrors
	// TenantNamespace's own special case): Apps there carry no LabelTenant, so
	// namespace scoping alone is correct and a label filter would find nothing.
	if workspaceID != "" && workspaceID != core.DefaultTenant {
		opts = append(opts, client.MatchingLabels{core.LabelTenant: workspaceID})
	}
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	for i := range list.Items {
		a := &list.Items[i]
		if a.DeletionTimestamp == nil && a.Spec.RegistryCredentialID != nil && strings.TrimSpace(*a.Spec.RegistryCredentialID) == cred.ID {
			return a, nil
		}
	}
	return nil, nil
}

// reapCredentialPullSecrets deletes every per-App copy of credential id in
// workspaceID's namespace once no App binds it (w8/m529): each still holds the
// password, and carries no owner reference to reap it. A release that went
// live with the credential pulls anonymously from then on, as a deleted
// credential leaves nothing to pull with.
func (s *Service) reapCredentialPullSecrets(ctx context.Context, workspaceID, id string) error {
	return s.DeleteListed(ctx, &corev1.SecretList{}, client.InNamespace(s.AppNamespace(workspaceID)),
		client.MatchingLabels{appv1alpha1.LabelRegistryPullCredential: id})
}
