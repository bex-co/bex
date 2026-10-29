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

// Package v1alpha1 contains API Schema definitions for the app v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=app.bex.co
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	// This name is used by applyconfiguration generators (e.g. controller-gen).
	SchemeGroupVersion = schema.GroupVersion{Group: "app.bex.co", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	// apimachinery's builder, as controller-runtime's scheme.Builder advises:
	// an API package keeps its dependencies to apimachinery.
	SchemeBuilder = &schemeBuilder{}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// schemeBuilder keeps the kubebuilder Register(objects...) shape on top of
// apimachinery's runtime.SchemeBuilder.
type schemeBuilder struct{ runtime.SchemeBuilder }

// Register adds objects to this group-version when AddToScheme runs.
func (b *schemeBuilder) Register(objects ...runtime.Object) {
	b.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, objects...)
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	})
}

// AddToScheme adds every registered object to s.
func (b *schemeBuilder) AddToScheme(s *runtime.Scheme) error { return b.SchemeBuilder.AddToScheme(s) }
