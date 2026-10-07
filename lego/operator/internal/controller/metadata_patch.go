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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// patchAppMeta patches the metadata change mutate makes to app, and keeps the
// pass's view of everything else. controller-runtime decodes the server's
// whole object into the one it patches, so patching app itself discarded the
// status the pass had set but not yet written (w5/096). The patch holds app's
// resourceVersion, so it lands only on the App the pass read: a conflict means
// the App changed since, and the pass retries from current state (w5/095).
// The resourceVersion that comes back then covers this change alone, and the
// pass's status write still guards everything else.
func (r *AppReconciler) patchAppMeta(ctx context.Context, app *appv1alpha1.App, mutate func(*metav1.ObjectMeta)) error {
	patched := app.DeepCopy()
	mutate(&patched.ObjectMeta)
	if err := r.Patch(ctx, patched, client.MergeFromWithOptions(app, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	app.ObjectMeta = patched.ObjectMeta
	return nil
}

// patchAppStatus is patchAppMeta for a status change: it patches the change
// mutate makes to a copy of app's status, under app's resourceVersion, then
// gives app the status it sent and the new resourceVersion. The server's status
// is never decoded into app, so status the pass has set but not yet written,
// such as its release decision, survives (w5/121).
func (r *AppReconciler) patchAppStatus(ctx context.Context, app *appv1alpha1.App, mutate func(*appv1alpha1.AppStatus)) error {
	patched := app.DeepCopy()
	mutate(&patched.Status)
	sent := patched.Status.DeepCopy()
	if err := r.Status().Patch(ctx, patched, client.MergeFromWithOptions(app, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	app.Status = *sent
	app.ResourceVersion = patched.ResourceVersion
	return nil
}
