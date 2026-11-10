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

package apps

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BuildMinutesWorker marks the Apps of a workspace that has used its plan's
// included pipeline minutes with no way to buy more (w6/m308, docs/ADR030
// §13); the operator then starts no build or pre-deploy run for them this
// month. A worker operation with no caller identity and no audit row, like
// BillingSuspensionWorker. The mark is an annotation only: it changes no spec, opens
// no deploy and survives the control-plane projector, which writes spec and
// labels.
type BuildMinutesWorker struct{ Svc *Service }

// Now is the service clock.
func (w BuildMinutesWorker) Now() time.Time { return w.Svc.Now() }

// List returns every App in the cluster, build sources included: a carrier
// builds too.
func (w BuildMinutesWorker) List(ctx context.Context) ([]appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := w.Svc.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// Mark sets a's mark to month ("2006-01"), or removes it when month is "".
func (w BuildMinutesWorker) Mark(ctx context.Context, a *appv1alpha1.App, month string) error {
	if a.Annotations[appv1alpha1.AnnotationBuildMinutesExhausted] == month {
		return nil
	}
	patch := client.MergeFrom(a.DeepCopy())
	if month == "" {
		delete(a.Annotations, appv1alpha1.AnnotationBuildMinutesExhausted)
	} else {
		metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.AnnotationBuildMinutesExhausted, month)
	}
	return client.IgnoreNotFound(w.Svc.Client.Patch(ctx, a, patch))
}
