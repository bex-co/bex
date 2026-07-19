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
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// generationDeletionOrFinalizerPredicate filters status and unrelated metadata
// noise while admitting every primary-resource update that can change the
// controller's lifecycle: desired generation, deletion timestamp, or finalizers.
// Deletion and finalizer writes do not advance metadata.generation.
type generationDeletionOrFinalizerPredicate struct {
	predicate.GenerationChangedPredicate
}

func (p generationDeletionOrFinalizerPredicate) Update(e event.UpdateEvent) bool {
	if p.GenerationChangedPredicate.Update(e) {
		return true
	}
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !e.ObjectNew.GetDeletionTimestamp().IsZero() ||
		!slices.Equal(e.ObjectOld.GetFinalizers(), e.ObjectNew.GetFinalizers())
}
