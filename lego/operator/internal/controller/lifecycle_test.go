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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestEffectiveReplicas(t *testing.T) {
	mk := func(replicas int32, suspended bool) *appv1alpha1.App {
		return &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Replicas: replicas, Suspended: suspended}}
	}
	cases := []struct {
		name string
		app  *appv1alpha1.App
		want int32
	}{
		{"default is 1", mk(0, false), 1},
		{"explicit count", mk(3, false), 3},
		{"suspended overrides default", mk(0, true), 0},
		// The ADR invariant: suspend derives 0 without rewriting spec.replicas.
		{"suspended overrides explicit count", mk(3, true), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveReplicas(tc.app); got != tc.want {
				t.Fatalf("effectiveReplicas = %d, want %d", got, tc.want)
			}
			// suspend must never mutate the stored count
			if tc.app.Spec.Suspended && tc.app.Spec.Replicas == 0 && tc.name == "suspended overrides explicit count" {
				t.Fatalf("spec.replicas was mutated")
			}
		})
	}
}
