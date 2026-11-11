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
	"slices"
	"strconv"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// protectServingImage pushes the serving release's own built image as
// live-<generation> so registry retention keeps it however many newer builds
// are pushed (w1/m183: a run of builds that never went live evicted
// beancount-cms-v2's image, and the next reschedule could not pull it). It
// runs on every Running reconcile but tags once per release; a failure is
// logged and retried next time, never failing the rollout it follows.
func (r *AppReconciler) protectServingImage(ctx context.Context, app *appv1alpha1.App) {
	d := app.Status.ImageDigest
	if d == nil || d.Digest == "" {
		return
	}
	tag := registry.LiveTagPrefix + strconv.FormatInt(d.Generation, 10)
	if app.Status.ProtectedImageTag == tag {
		r.refreshStoredImages(ctx, app, false)
		return
	}
	if _, own := r.ownImageReference(app, app.Status.Image); !own {
		// The service no longer runs a build of its own repository, so a
		// reading of that repository describes nothing it serves.
		app.Status.StoredImages = nil
		return
	}
	log := logf.FromContext(ctx)
	authHdr, ready, err := r.registryAuthHeader(ctx, app)
	if err != nil {
		log.Info("live image tag deferred", "error", err.Error())
		return
	}
	if !ready {
		log.V(1).Info("live image tag deferred: registry credential not active yet")
		return
	}
	if err := registry.TagManifest(ctx, r.HTTPClient, r.Registry, appIdentity(app).Repo(), d.Digest, tag, authHdr); err != nil {
		log.Info("live image tag failed; retrying next reconcile", "tag", tag, "error", err.Error())
		return
	}
	app.Status.ProtectedImageTag = tag
	r.refreshStoredImages(ctx, app, true)
}

// A settled App's repository is read again every storedImagesInterval:
// retention runs in the registry on its own clock after later pushes, so a
// reading ages. A reading that failed is tried again after storedImagesRetry,
// and one reading may take storedImagesTimeout of a reconcile at most.
const (
	storedImagesInterval = 10 * time.Minute
	storedImagesRetry    = 2 * time.Minute
	storedImagesTimeout  = 20 * time.Second
)

// refreshStoredImages publishes which manifests the registry still holds for
// the App's own repository (status.storedImages, w8/m156), so bex-api can
// refuse a rollback to a build that is gone instead of accepting it and
// failing at the pull. It reads when a release has just been tagged and when
// the previous reading is due.
//
// bex-api refuses on what is published, so a reading is only published when
// it is whole and plausible: the registry answered for every tag, and the
// build that is serving right now is among them. Anything else keeps the
// previous reading, which stays safe however old it gets, because it carries
// the generation it was read at and says nothing about later builds.
func (r *AppReconciler) refreshStoredImages(ctx context.Context, app *appv1alpha1.App, released bool) {
	serving := app.Status.ImageDigest
	if r.PerAppRegistry == nil || serving == nil {
		return
	}
	key := app.Namespace + "/" + app.Name
	if due, ok := r.storedImagesRead.Load(key); ok && !released && time.Now().Before(due.(time.Time)) {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, storedImagesTimeout)
	defer cancel()
	digests, err := r.PerAppRegistry.StoredDigestsFor(readCtx, appIdentity(app), app.Namespace)
	if err == nil && !slices.Contains(digests, serving.Digest) {
		err = fmt.Errorf("the listing does not hold the serving build %s", serving.Digest)
	}
	if err != nil {
		r.storedImagesRead.Store(key, time.Now().Add(storedImagesRetry))
		logf.FromContext(ctx).Info("stored image listing failed; keeping the previous one", "error", err.Error())
		return
	}
	r.storedImagesRead.Store(key, time.Now().Add(storedImagesInterval))
	if s := app.Status.StoredImages; s != nil && s.Generation == serving.Generation && slices.Equal(s.Digests, digests) {
		return
	}
	app.Status.StoredImages = &appv1alpha1.StoredImagesStatus{Generation: serving.Generation, Digests: digests}
}
