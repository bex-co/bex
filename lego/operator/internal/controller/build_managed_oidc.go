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

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Managed OIDC at build time (docs/ADR116-managed-oidc.md D12, w2/m480).
// render.com/docs/oidc: "OIDC is available at build time for services on any
// native runtime"; "Currently, OIDC auth is not available at build time for
// services that build from a Dockerfile." A native build decides its
// providers from the environment its build command sees, with the runtime's
// rules (decideManagedOIDC), and records the decision in
// status.buildManagedOIDC. bex-api mints those tokens into the App's
// <app>-oidc-build Secret, stamped with the build's generation; the operator,
// which never sees a key, copies them next to the build Job, whose preparer
// waits for them. The served release's <app>-oidc is never touched, so a
// build cannot change the files running instances read. When the build ends,
// or a cancel settles it (endCanceledBuild, w9/136), the decision and both
// Secrets go.

// buildManagedOIDCSecretName is the build-namespace copy of a native build's
// tokens, the only Secret the build Pod reads them from.
func buildManagedOIDCSecretName(app string) string { return build.JobName(app, "oidc") }

// buildManagedOIDC is the decision of the release app is building, taken
// once per release generation from the in-flight release's environment (the
// one the native build projects) and written to status before the build Job
// exists, so bex-api can mint while the Job is made. It is kept for the life
// of the build: a variable saved after dispatch reaches the next deploy.
func (r *AppReconciler) buildManagedOIDC(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.ManagedOIDCStatus, error) {
	gen := releaseGeneration(app)
	if st := app.Status.BuildManagedOIDC; st != nil && st.Generation == gen {
		return st, nil
	}
	decision, err := r.decideManagedOIDC(ctx, app, inFlightRelease(ctx, app), gen)
	if err != nil {
		return nil, fmt.Errorf("deciding the build's Managed OIDC tokens: %w", err)
	}
	app.Status.BuildManagedOIDC = decision
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		return nil, fmt.Errorf("recording the build's Managed OIDC decision: %w", err)
	}
	return decision, nil
}

// projectBuildManagedOIDC is the build-namespace Secret and the providers a
// native build reads tokens for, ("", nil) when its decision mounts none.
// The copy is written only from tokens bex-api minted for this build's
// generation, and rewritten on every pass while the build waits, so a queued
// build's Pod starts with a recent token; until then a copy left by an
// earlier build is removed and the preparer waits for the Secret.
func (r *AppReconciler) projectBuildManagedOIDC(ctx context.Context, app *appv1alpha1.App, buildNs string) (string, []appv1alpha1.ManagedOIDCProvider, error) {
	st, err := r.buildManagedOIDC(ctx, app)
	if err != nil {
		return "", nil, err
	}
	providers := appv1alpha1.ManagedOIDCProvidersNamed(st.Mounted())
	if len(providers) == 0 {
		return "", nil, nil
	}
	name := buildManagedOIDCSecretName(app.Name)
	src := &corev1.Secret{}
	err = r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ManagedOIDCBuildSecretName(app.Name)}, src)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", nil, fmt.Errorf("reading the build's Managed OIDC tokens: %w", err)
	}
	data, minted := buildTokensMinted(src, app, st.Generation, providers)
	if !minted {
		// bex-api has not minted this build's tokens yet: the preparer waits
		// for the copy, and one an earlier build left must not stand in.
		return name, providers, r.removeEarlierBuildTokens(ctx, app, buildNs, st.Generation)
	}
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: buildNs}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.buildPlaneClient(), dst, func() error {
		if err := checkOwnedArtifact(dst, app); err != nil {
			return err
		}
		dst.Type = corev1.SecretTypeOpaque
		dst.Data = data
		dst.Labels = artifactLabels(app, "managed-oidc-secret")
		carryProtectedMarker(dst.Labels, src.Labels)
		dst.Annotations = map[string]string{appv1alpha1.AnnotationManagedOIDCGeneration: strconv.FormatInt(st.Generation, 10)}
		return nil
	}); err != nil {
		return "", nil, fmt.Errorf("projecting the build's Managed OIDC tokens: %w", err)
	}
	return name, providers, nil
}

// removeEarlierBuildTokens deletes this App lifetime's build-namespace token
// copy when it was made for another generation's build.
func (r *AppReconciler) removeEarlierBuildTokens(ctx context.Context, app *appv1alpha1.App, buildNs string, gen int64) error {
	cl := r.buildPlaneClient()
	stale := &corev1.Secret{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: buildNs, Name: buildManagedOIDCSecretName(app.Name)}, stale); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading the build's Managed OIDC token copy: %w", err)
	}
	owned := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID)}
	if stale.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration] == strconv.FormatInt(gen, 10) || !owned.Owns(stale) {
		return nil
	}
	if err := cl.Delete(ctx, stale, client.Preconditions{UID: &stale.UID}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing an earlier build's Managed OIDC tokens: %w", err)
	}
	return nil
}

// buildTokensMinted is the files src holds for providers when bex-api
// minted them for app's build of generation gen: owned by this App
// lifetime, stamped with gen, and holding every file.
func buildTokensMinted(src *corev1.Secret, app *appv1alpha1.App, gen int64, providers []appv1alpha1.ManagedOIDCProvider) (map[string][]byte, bool) {
	if src.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration] != strconv.FormatInt(gen, 10) || !ownedByApp(src, app) {
		return nil, false
	}
	data := map[string][]byte{}
	for _, p := range providers {
		for _, f := range p.Files() {
			if len(src.Data[f]) == 0 {
				return nil, false
			}
			data[f] = src.Data[f]
		}
	}
	return data, true
}

// ownedByApp reports that bex-api wrote s for this App lifetime: the App
// owns it (not as controller, so its refreshes do not wake the App watch).
func ownedByApp(s *corev1.Secret, app *appv1alpha1.App) bool {
	return slices.ContainsFunc(s.OwnerReferences, func(o metav1.OwnerReference) bool { return o.UID == app.UID })
}

// endBuildManagedOIDC ends a native build's tokens once the observed build
// is over (succeeded or failed, a timeout included). Best-effort like the
// datastore reach: a leftover token expires within the hour, and the next
// build's end or the App's deletion removes it. A build that never reaches a
// terminal observation loses them where it ends instead: a settled cancel
// (endCanceledBuild) or the App's deletion.
func (r *AppReconciler) endBuildManagedOIDC(ctx context.Context, app *appv1alpha1.App, buildNs string, phase build.Phase) {
	if (phase != build.PhaseSucceeded && phase != build.PhaseFailed) || app.Status.BuildManagedOIDC == nil {
		return
	}
	if err := r.revokeBuildManagedOIDC(ctx, app, buildNs); err != nil {
		logf.FromContext(ctx).Error(err, "ending the build's Managed OIDC tokens", "app", app.Name)
	}
}

// revokeBuildManagedOIDC ends the App's build tokens: the decision goes from
// status first, so bex-api stops minting, then both Secrets are deleted. Each
// delete is ownership-gated: the build namespace is shared, and a Secret of
// another App lifetime stays.
func (r *AppReconciler) revokeBuildManagedOIDC(ctx context.Context, app *appv1alpha1.App, buildNs string) error {
	if app.Status.BuildManagedOIDC != nil {
		app.Status.BuildManagedOIDC = nil
		r.updateStatusRetrying(ctx, app, "buildManagedOIDC")
	}
	var errs []error
	owned := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID)}
	if _, err := deleteOwnedObject(ctx, r.buildPlaneClient(), buildNs, buildManagedOIDCSecretName(app.Name), &corev1.Secret{}, owned); err != nil {
		errs = append(errs, fmt.Errorf("removing the build's Managed OIDC token copy: %w", err))
	}
	src := &corev1.Secret{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ManagedOIDCBuildSecretName(app.Name)}
	switch err := r.uncachedSecretClient().Get(ctx, key, src); {
	case apierrors.IsNotFound(err):
	case err != nil:
		errs = append(errs, fmt.Errorf("reading the build's Managed OIDC tokens: %w", err))
	case ownedByApp(src, app):
		if err := r.Delete(ctx, src, client.Preconditions{UID: &src.UID}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("removing the build's Managed OIDC tokens: %w", err))
		}
	}
	return errors.Join(errs...)
}
