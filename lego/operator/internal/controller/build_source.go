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
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/registry"
)

// Render Build Sources (w2/m254, docs/ADR097-build-sources.md). A build_source
// App is a carrier: it builds through the unchanged pipeline and runs nothing.
// A service linked to one (spec.buildSource) builds nothing: it deploys a copy
// of the selected carrier build in its own repository.

// markBuilt settles a carrier whose build resolved: its image is recorded (and
// protected from retention like a serving image) and Ready=True closes the
// build, with no runtime object ever created.
func (r *AppReconciler) markBuilt(ctx context.Context, app *appv1alpha1.App, image string) error {
	app.Status.Image = image
	app.Status.URL = ""
	app.Status.ObservedGeneration = app.Generation
	_, err := r.settleReady(ctx, app, nil, "Built", "build ready; a build source runs no instances")
	return err
}

// linkedBuildImage resolves a linked service's image: the selected carrier
// build, copied into the service's own repository under the same digest, or —
// for an image-backed source — the carrier's image as is. halted reports a pass
// that must stop: no build selected yet, or a copy that failed and is retried;
// either way a release that already served keeps serving.
func (r *AppReconciler) linkedBuildImage(ctx context.Context, app *appv1alpha1.App) (string, ctrl.Result, bool, error) {
	link := app.Spec.BuildSource
	if link.Image == "" {
		if releaseHasServed(app) {
			_, res, err := r.holdPendingArtifact(ctx, app, ctrl.Result{})
			return "", res, true, err
		}
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "AwaitingBuildSource",
			Message:            fmt.Sprintf("waiting for build source %s to build", link.Name),
			ObservedGeneration: app.Generation,
		})
		app.Status.Phase = appv1alpha1.PhasePending
		return "", ctrl.Result{}, true, updateStatusIfChanged(ctx, r.Client, app)
	}
	image, err := r.verifiedLinkedImage(ctx, app)
	if err != nil {
		// A refused link stays refused until the spec changes, which triggers
		// the next pass: settle Failed without a retry.
		res, failErr := r.fail(ctx, app, appv1alpha1.ReasonBuildFailedUserError, fmt.Errorf("build source %s: %w", link.Name, err))
		if !apierrors.IsConflict(failErr) {
			failErr = nil
		}
		return "", res, true, failErr
	}
	if image.copyFrom != "" {
		if err := r.copyLinkedBuild(ctx, app, image); err != nil {
			res, failErr := r.fail(ctx, app, appv1alpha1.ReasonBuildFailedInfrastructure, fmt.Errorf("build source %s: %w", link.Name, err))
			return "", res, true, failErr
		}
	}
	return image.ref, ctrl.Result{}, false, nil
}

// linkedImage is what a linked service runs: ref, and for a platform build the
// carrier repository and digest it is copied from.
type linkedImage struct {
	ref              string
	copyFrom, digest string
}

// verifiedLinkedImage checks the selected image against the named carrier
// before anything is copied with the builder's credential: the carrier must be
// a build_source in the service's namespace, and the image either a digest in
// its own repository or, for an image-backed source, its image. A link can
// therefore never reach another repository.
func (r *AppReconciler) verifiedLinkedImage(ctx context.Context, app *appv1alpha1.App) (linkedImage, error) {
	link := app.Spec.BuildSource
	carrier, err := r.linkedCarrier(ctx, app)
	if err != nil {
		return linkedImage{}, err
	}
	repo, reference, platform := r.platformRepoReference(link.Image)
	switch {
	case !platform:
		if link.Image != carrier.Spec.Image {
			return linkedImage{}, fmt.Errorf("image %s is not the build source's image", link.Image)
		}
		return linkedImage{ref: link.Image}, nil
	case repo != appIdentity(carrier).Repo():
		return linkedImage{}, fmt.Errorf("image %s is not in the build source's repository", link.Image)
	case !registry.IsDigest(reference):
		return linkedImage{}, fmt.Errorf("image %s is not pinned to a digest", link.Image)
	}
	own := appIdentity(app).Repo()
	return linkedImage{ref: r.Registry + "/" + own + "@" + reference, copyFrom: repo, digest: reference}, nil
}

// copyLinkedBuild copies the carrier build into the service's repository. The
// copy is also tagged, so registry retention (which reaps untagged manifests)
// keeps it while the release has not served yet.
func (r *AppReconciler) copyLinkedBuild(ctx context.Context, app *appv1alpha1.App, image linkedImage) error {
	authHdr, ready, err := r.builderAuthHeader(ctx, app)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("the registry credential is not active yet")
	}
	tag := linkedBuildTagPrefix + strings.TrimPrefix(image.digest, "sha256:")[:12]
	return registry.CopyManifest(ctx, r.HTTPClient, r.Registry, image.copyFrom, appIdentity(app).Repo(), image.digest, tag, authHdr)
}

// linkedBuildTagPrefix names the tag a copied build carries in a linked
// service's repository.
const linkedBuildTagPrefix = "build-"

// linkedToNativeSource reports whether app deploys a native build of its
// build source: that image carries native-init and no CMD, so the service's
// start command runs as the entrypoint's arguments (launchNativeArgs). It
// holds only for the carrier's current build made from its current spec — an
// older digest, or one built before the source switched to native, may be a
// Docker image, and keeps the shell launch that runs on any image.
func (r *AppReconciler) linkedToNativeSource(ctx context.Context, app *appv1alpha1.App) bool {
	if app.Spec.BuildSource == nil {
		return false
	}
	carrier, err := r.linkedCarrier(ctx, app)
	return err == nil && effectiveBuilder(carrier.Spec) == build.BuilderNative &&
		app.Spec.BuildSource.Image != "" && app.Spec.BuildSource.Image == carrier.Status.ArtifactImage &&
		carrier.Status.ArtifactFingerprint == desiredAppReleaseIdentity(carrier.Spec).artifact
}

// linkedCarrier reads the build source carrier linked app deploys from,
// refusing anything that is not one.
func (r *AppReconciler) linkedCarrier(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.App, error) {
	name := app.Spec.BuildSource.Name
	var carrier appv1alpha1.App
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &carrier); err != nil {
		return nil, fmt.Errorf("read the build source: %w", err)
	}
	if carrier.Spec.Type != appv1alpha1.TypeBuildSource {
		return nil, fmt.Errorf("%s is not a build source", name)
	}
	return &carrier, nil
}
