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
	if a == nil || a.Spec.Image == "" || a.Spec.Repo != "" || a.Spec.Suspended || a.Spec.Type == appv1alpha1.TypeStaticSite {
		return nil
	}
	live, err := t.ServedRelease(ctx, a)
	if errors.Is(err, ErrNoDeployHistory) {
		return nil
	}
	if err != nil {
		return err
	}
	if live == nil || servesSavedImage(a, *live) {
		return nil
	}
	SelectReleaseImage(a, live.ResolvedImage)
	return nil
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
	if live.Image == a.Spec.Image || live.ResolvedImage == a.Spec.Image {
		return true
	}
	for _, network := range []*appv1alpha1.ImageNetworkStatus{a.Status.ServingNetwork, a.Status.ImageNetwork} {
		if network != nil && network.Image == live.ResolvedImage && network.Source == a.Spec.Image {
			return true
		}
	}
	return false
}
