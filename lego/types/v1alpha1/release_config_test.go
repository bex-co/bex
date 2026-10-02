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
package v1alpha1

import "testing"

func TestReleaseConfigExpiresOnlyWhenAnotherReleaseIsRequested(t *testing.T) {
	app := &App{}
	app.Generation = 8
	app.Status.ReleaseGeneration = 7
	app.Annotations = map[string]string{AnnotationReleaseGeneration: "7"}
	app.Spec.ReleaseConfig = &ReleaseConfigReference{Generation: 7, SourceGeneration: 3, Image: "image:A"}
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("operational mutation expired the running selection")
	}
	app.Annotations[AnnotationReleaseGeneration] = "9"
	if app.ActiveReleaseConfig() != nil {
		t.Fatal("normal deploy retained historical selection")
	}
	app.Spec.ReleaseConfig.Generation = 9
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("fresh restart selection was ignored before operator adoption")
	}
	app.Status.ReleaseGeneration = 10
	if app.ActiveReleaseConfig() != nil {
		t.Fatal("operator-adopted direct CR release retained stale selection")
	}
}
