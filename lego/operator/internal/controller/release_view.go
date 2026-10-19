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
	"encoding/json"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// releaseRecordAdoptedSpecKey holds, in a release record, the JSON-encoded
// AppSpec the release was adopted with (w8/m121). Operator-internal: bex-api
// reads only the pod template and ReleaseRecordSpec keys.
const releaseRecordAdoptedSpecKey = "adoptedSpec"

// releaseView is saved with every release and artifact input taken from
// adopted: what a release held through its rollout builds and runs once the
// saved spec has moved on to a newer release. Operational fields (replicas,
// suspend, autoscaling, domains, the clone Secret) keep following the saved
// spec. So does Disk, whose lifecycle (attach, detach, PVC cleanup) must never
// follow an older release; a remount since adoption fails heldReleaseView's
// identity check instead.
func releaseView(saved, adopted appv1alpha1.AppSpec) appv1alpha1.AppSpec {
	view := saved
	to, from := reflect.ValueOf(&view).Elem(), reflect.ValueOf(adopted)
	for i := range to.NumField() {
		if appSpecIdentityClasses[to.Type().Field(i).Name] != appv1alpha1.IdentityOperational {
			to.Field(i).Set(from.Field(i))
		}
	}
	view.Disk = saved.Disk
	return view
}

// heldReleaseView is the spec the release in flight was adopted with, viewed
// through the saved spec's operational fields, when the saved spec has since
// moved on to another release. It is nil when no release is in flight, when
// the saved spec is still the release's own, and for a release adopted before
// its spec was recorded, which keeps reading the saved spec (w8/124, m120). It
// is also nil when the saved spec selects a release (Rollback, Restart): that
// overrides the release in flight rather than waiting behind it, as before.
func (r *AppReconciler) heldReleaseView(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.AppSpec, error) {
	if app.Status.Phase != appv1alpha1.PhaseBuilding && app.Status.Phase != appv1alpha1.PhaseDeploying {
		return nil, nil
	}
	if app.ActiveReleaseConfig() != nil {
		return nil, nil
	}
	gen := app.Status.ReleaseGeneration
	if gen <= 0 || app.Status.ReleaseFingerprint == "" || desiredAppReleaseIdentity(app.Spec).release == app.Status.ReleaseFingerprint {
		return nil, nil
	}
	adopted, err := r.adoptedReleaseSpec(ctx, app, gen)
	if err != nil || adopted == nil {
		return nil, err
	}
	view := releaseView(app.Spec, *adopted)
	if desiredAppReleaseIdentity(view).release != app.Status.ReleaseFingerprint {
		return nil, nil
	}
	return &view, nil
}

// adoptedReleaseSpec reads the spec recordAdoptedRelease stored for
// generation, or nil when its record has none.
func (r *AppReconciler) adoptedReleaseSpec(ctx context.Context, app *appv1alpha1.App, generation int64) (*appv1alpha1.AppSpec, error) {
	rec, err := r.readReleaseRecordSecret(ctx, app, generation)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, ok := rec.Data[releaseRecordAdoptedSpecKey]
	if !ok {
		return nil, nil
	}
	spec := &appv1alpha1.AppSpec{}
	if err := json.Unmarshal(raw, spec); err != nil {
		// Unreadable, it gives no view: the release keeps the m120 fallback
		// rather than failing every pass, suspend and scale included.
		logf.FromContext(ctx).Error(err, "decode a release's adopted spec; running it without its view", "app", app.Name, "generation", generation)
		return nil, nil
	}
	return spec, nil
}

type viewedReleaseKey struct{}

// viewedReleaseMark is a pass running one App's held release on its view.
type viewedReleaseMark struct {
	app  client.ObjectKey
	spec appv1alpha1.AppSpec
}

// withViewedRelease runs the rest of the pass on app's held release view:
// app.Spec becomes the view, the release's own steps read its adoption
// snapshots (inFlightRelease), and keepReleaseView re-applies the view after a
// status write decodes the stored spec back into app.
func withViewedRelease(ctx context.Context, app *appv1alpha1.App, view appv1alpha1.AppSpec) context.Context {
	app.Spec = view
	return context.WithValue(ctx, viewedReleaseKey{}, &viewedReleaseMark{app: client.ObjectKeyFromObject(app), spec: *view.DeepCopy()})
}

func viewedReleaseOf(ctx context.Context, app *appv1alpha1.App) *viewedReleaseMark {
	mark, _ := ctx.Value(viewedReleaseKey{}).(*viewedReleaseMark)
	if mark == nil || mark.app != client.ObjectKeyFromObject(app) {
		return nil
	}
	return mark
}

func viewedRelease(ctx context.Context, app *appv1alpha1.App) bool {
	return viewedReleaseOf(ctx, app) != nil
}

// keepReleaseView re-applies a viewed pass's spec to obj after a status write
// or re-read replaced it with the stored, saved spec.
func keepReleaseView(ctx context.Context, obj client.Object) {
	if app, ok := obj.(*appv1alpha1.App); ok {
		if mark := viewedReleaseOf(ctx, app); mark != nil {
			app.Spec = *mark.spec.DeepCopy()
		}
	}
}
