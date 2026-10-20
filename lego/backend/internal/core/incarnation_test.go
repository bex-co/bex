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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestDropOtherIncarnations(t *testing.T) {
	pod := func(name, id string) corev1.Pod {
		labels := map[string]string{PodLabelApp: "web"}
		if id != "" {
			labels[LabelAppID] = id
		}
		return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	got := DropOtherIncarnations([]corev1.Pod{pod("old", "srv-old"), pod("new", "srv-new"), pod("job", "")}, "srv-new", time.Time{})
	if len(got) != 2 || got[0].Name != "new" || got[1].Name != "job" {
		t.Fatalf("kept %v, want the new incarnation's pod and the unlabeled Job pod", got)
	}
	floor := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	stale := pod("old-job", "")
	stale.CreationTimestamp = metav1.NewTime(floor.Add(-time.Minute))
	fresh := pod("new-job", "")
	fresh.CreationTimestamp = metav1.NewTime(floor.Add(time.Minute))
	if got := DropOtherIncarnations([]corev1.Pod{stale, fresh}, "srv-new", floor); len(got) != 1 || got[0].Name != "new-job" {
		t.Fatalf("kept %v, want only the unlabeled pod created after the floor", got)
	}

	recreatedCR := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "web", Labels: map[string]string{LabelAppID: ids.New(ids.Service)},
		CreationTimestamp: metav1.NewTime(time.Now().Add(time.Hour)),
	}}
	if got := AppNotBefore(recreatedCR); !got.Before(recreatedCR.CreationTimestamp.Time) {
		t.Errorf("AppNotBefore = %s, want the id's earlier mint time, not a re-created CR's creation", got)
	}
	legacy := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	if id := AppIncarnationID(legacy); id != "web" {
		t.Errorf("legacy App id = %q, want its name (what the operator stamps)", id)
	}
}
