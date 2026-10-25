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
	"strconv"

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
		return
	}
	if _, own := r.ownImageReference(app, app.Status.Image); !own {
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
}
