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

package rollout

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// SelectServedImage makes the configuration-only release a's locked patch
// opens rerun the image a prebuilt-image service is serving (w4/m418), as
// Restart does: Render's "Save and deploy" "redeploys your service's existing
// build". A per-deploy imageUrl / imgURL override leaves spec.image alone
// (w4/414), so without a selection the release would roll the service back to
// the saved tag.
//
// It selects nothing when the service serves its saved image, so the saved
// tag keeps resolving as it always has and no saved-tag service starts
// reading image_override; nor for a service with no live release, unknown
// deploy history, a suspended service (its change is held, not stamped), or
// anything that is not a prebuilt image.
func (t *Tracker) SelectServedImage(ctx context.Context, a *appv1alpha1.App) error {
	if !runsPrebuiltImage(a) || a.Spec.Suspended {
		return nil
	}
	live, err := t.ServedRelease(ctx, a)
	if errors.Is(err, ErrNoDeployHistory) {
		return nil
	}
	if err != nil {
		return err
	}
	selectLiveImage(a, live)
	return nil
}

// keepSelectedImage makes the release a settings write opens on a
// prebuilt-image service (a plan change, a Settings save that deploys, a
// container-policy migration) keep the image the release in force runs when
// that release runs a selected image rather than the saved one: an imageUrl /
// imgURL override, a Restart or rollback, or a configuration-only release that
// kept one (w6/159). Those leave spec.image alone (w4/414), so without a
// selection a plan change after an override rolled the service back to the
// tag it was created with. A settings change redeploys what is live, as
// Restart and an environment "Save and deploy" (w4/m418) do.
//
// The selection is the live deploy's resolved image when the selected release
// serves, and the selected image itself while that release is still rolling,
// so a plan change behind an override in flight runs after it on the same
// image. A selected release that ended without serving leaves the live
// deploy's image, as SelectServedImage would.
//
// A release in force that runs the saved image selects nothing, so a saved
// change held for the next deploy (w8/m135), a new image included, ships with
// the settings release as before. So do a write that changes the image itself,
// a save-only or suspended write (nothing rolls), and unknown history.
func (t *Tracker) keepSelectedImage(ctx context.Context, trigger string, before Snapshot, a *appv1alpha1.App) error {
	if trigger != store.TriggerSettingsChange || before.saveOnly || !runsPrebuiltImage(a) || a.Spec.Suspended ||
		a.Spec.Image != before.spec.Image {
		return nil
	}
	ref := a.ActiveReleaseConfig()
	if ref == nil || ref.Image == "" || !appv1alpha1.SpecRollsRelease(before.spec, a.Spec) {
		return nil
	}
	lister, appID := t.history(a)
	if lister == nil {
		return nil
	}
	live, err := t.ServedRelease(ctx, a)
	if err != nil {
		return err
	}
	if live == nil || live.Generation != ref.Generation {
		open, err := lister.ListDeploys(ctx, appID, store.DeployFilter{Statuses: store.OpenDeployStatuses()})
		if err != nil {
			return err
		}
		for _, d := range open {
			if d.Generation == ref.Generation {
				if !savedImageReference(a, ref.Image) {
					SelectReleaseImage(a, ref.Image)
				}
				return nil
			}
		}
	}
	selectLiveImage(a, live)
	return nil
}

// runsPrebuiltImage reports whether a runs a prebuilt image: the services a
// served-image selection applies to.
func runsPrebuiltImage(a *appv1alpha1.App) bool {
	return a != nil && a.Spec.Image != "" && a.Spec.Repo == "" && a.Spec.Type != appv1alpha1.TypeStaticSite
}

// selectLiveImage selects the live deploy's resolved image for a's release
// unless that deploy serves the saved image or there is none.
func selectLiveImage(a *appv1alpha1.App, live *store.Deploy) {
	if live == nil || servesSavedImage(a, *live) {
		return
	}
	SelectReleaseImage(a, live.ResolvedImage)
}

// ErrNoLiveBuild reports a source-built service with no live release: there is
// no build for a configuration-only release to rerun.
var ErrNoLiveBuild = errors.New("rollout: no live build to rerun")

// ServedArtifact is the image a configuration-only "deploy" reruns: the current
// build with the new values, never a source build (w4/m180, ADR013). Empty
// means nothing serving to rerun: a static site (its environment is build-time
// only, so its deploy rebuilds), unknown history, or a prebuilt image with no
// live release, which deploys its saved image. A source-built service with no
// live release answers ErrNoLiveBuild.
func (t *Tracker) ServedArtifact(ctx context.Context, a *appv1alpha1.App) (string, error) {
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		return "", nil
	}
	live, err := t.ServedRelease(ctx, a)
	if errors.Is(err, ErrNoDeployHistory) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if live == nil && a.Spec.Image == "" {
		return "", ErrNoLiveBuild
	}
	if live == nil {
		return "", nil
	}
	return live.ResolvedImage, nil
}

// SelectServedBuild makes the configuration-only release a's locked patch
// opens rerun the build a serves now (ServedArtifact) with the new
// configuration instead of building: an environment "Save and deploy"
// (w4/m180) and every environment-group write that deploys (w4/708). A
// prebuilt image reruns the image it serves only when that is not its saved
// one (SelectServedImage). A suspended service selects nothing: its change is
// held for the next deploy, not stamped.
func (t *Tracker) SelectServedBuild(ctx context.Context, a *appv1alpha1.App) error {
	if a.Spec.Suspended {
		return nil
	}
	if a.Spec.Image != "" {
		return t.SelectServedImage(ctx, a)
	}
	image, err := t.ServedArtifact(ctx, a)
	if err != nil || image == "" {
		return err
	}
	SelectReleaseImage(a, image)
	return nil
}

// DeployInFlight reports whether a deploy of a is executing or queued. A
// release opened behind it runs after it, on whatever that deploy leaves
// serving, so selecting the build serving now would roll the older build back
// over it (w4/667). Unknown history reports none.
func (t *Tracker) DeployInFlight(ctx context.Context, a *appv1alpha1.App) (bool, error) {
	lister, appID := t.history(a)
	if lister == nil {
		return false, nil
	}
	rows, err := lister.ListDeploys(ctx, appID, store.DeployFilter{Statuses: store.OpenDeployStatuses(), Limit: 1})
	return len(rows) > 0, err
}

// SelectReleaseImage selects image as the runtime image of the release a's
// patch opens: bound to metadata.generation+1, the generation Stamp records,
// so a later deploy expires it.
func SelectReleaseImage(a *appv1alpha1.App, image string) {
	a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: a.Generation + 1, Image: image}
}

// servesSavedImage reports whether the live deploy runs a's saved image. The
// row was created from it (a standard deploy records spec.image), or the
// resolved reference is it as written, or — for an image-port service, which
// runs the digest its tag resolved to (w4/m243) — a recorded network resolved
// that digest from exactly spec.image (the operator's servesSavedImage reads
// the networks the same way).
func servesSavedImage(a *appv1alpha1.App, live store.Deploy) bool {
	return live.Image == a.Spec.Image || savedImageReference(a, live.ResolvedImage)
}

// savedImageReference reports whether image is a's saved image: as written,
// or the digest a recorded network resolved from exactly spec.image.
func savedImageReference(a *appv1alpha1.App, image string) bool {
	if image == a.Spec.Image {
		return true
	}
	for _, network := range []*appv1alpha1.ImageNetworkStatus{a.Status.ServingNetwork, a.Status.ImageNetwork} {
		if network != nil && network.Image == image && network.Source == a.Spec.Image {
			return true
		}
	}
	return false
}
