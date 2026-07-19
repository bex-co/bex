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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestGenerationOrDeletionPredicate(t *testing.T) {
	p := generationOrDeletionPredicate{}
	old := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

	t.Run("admits spec generation changes", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Generation = 2
		if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
			t.Fatal("generation change was filtered")
		}
	})

	t.Run("filters status-only updates", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Status.Phase = appv1alpha1.PhaseRunning
		if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
			t.Fatal("status-only update was admitted")
		}
	})

	t.Run("admits deletion timestamp without generation change", func(t *testing.T) {
		updated := old.DeepCopy()
		now := metav1.Now()
		updated.DeletionTimestamp = &now
		if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
			t.Fatal("deletion update was filtered")
		}
	})

	t.Run("admits registry credential rotation annotation edge", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Annotations = map[string]string{annotRotateRegistryCreds: "true"}
		if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
			t.Fatal("registry credential rotation request was filtered")
		}
	})

	t.Run("filters unrelated annotation updates", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Annotations = map[string]string{"example.com/noise": "changed"}
		if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
			t.Fatal("unrelated annotation update was admitted")
		}
	})
}

func TestGenerationDeletionOrFinalizerPredicate(t *testing.T) {
	p := generationDeletionOrFinalizerPredicate{}
	old := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Generation: 4, Finalizers: []string{"existing"}}}

	updated := old.DeepCopy()
	updated.Finalizers = append(updated.Finalizers, dbFinalizer)
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}) {
		t.Fatal("finalizer-only update was filtered")
	}
	if slices.Equal(old.Finalizers, updated.Finalizers) {
		t.Fatal("test did not change finalizers")
	}

	statusOnly := old.DeepCopy()
	statusOnly.Status.Phase = appv1alpha1.DBPhaseReady
	if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: statusOnly}) {
		t.Fatal("status-only update was admitted")
	}
}
