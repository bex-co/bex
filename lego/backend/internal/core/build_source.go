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

package core

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Render Build Source is a build_source App CR (w2/m254, ADR097): built by
// the operator, never run, and never a service. Every App enumeration that
// means "the services" filters carriers out with ServiceApps; the guard in
// build_source_guard_test.go makes each enumeration declare which it means.

// IsBuildSource reports whether app is a build source carrier.
func IsBuildSource(app *appv1alpha1.App) bool {
	return app.Spec.Type == appv1alpha1.TypeBuildSource
}

// ServiceApps drops build source carriers from items, in place.
func ServiceApps(items []appv1alpha1.App) []appv1alpha1.App {
	out := items[:0]
	for i := range items {
		if !IsBuildSource(&items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

// notServiceApp turns a successful read of a build source carrier into the
// not-found a service lookup answers, so a service verb never acts on one.
func notServiceApp(err error, app *appv1alpha1.App) error {
	if err == nil && IsBuildSource(app) {
		return apierrors.NewNotFound(schema.GroupResource{Group: appv1alpha1.SchemeGroupVersion.Group, Resource: "apps"}, app.Name)
	}
	return err
}
