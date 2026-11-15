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
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// StampReleaseGeneration pins an App's open deploy to the release generation
// whose work it represents, so the operator adopts that identity instead of
// inferring one from whatever metadata generation it observes first (see
// AnnotationReleaseGeneration's own contract). It is the one writer for this
// annotation on the open path: the apps and deploys services share it so the
// stamp cannot drift between the two verbs that open a release.
func StampReleaseGeneration(a *appv1alpha1.App, generation int64) {
	metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration,
		strconv.FormatInt(generation, 10))
}
