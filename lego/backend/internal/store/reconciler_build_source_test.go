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

package store

import "testing"

// w2/m254 (ADR097): a linked service's row projects spec.buildSource — its
// carrier's CR, named by the carrier's bds- id — and a newly selected build
// reaches an existing CR through applyOwnedSpec, as any owned field does.
func TestProjectorCarriesTheBuildSourceLink(t *testing.T) {
	d := DesiredApp{App: App{TenantID: "tea-a", Name: "web", Type: "web_service", BuildSourceID: "bds-1", BuildSourceImage: "zot/tea-a/src@sha256:aaa"}}
	spec := projectSpec(d)
	if spec.BuildSource == nil || spec.BuildSource.Name != CRName("tea-a", "bds-1") || spec.BuildSource.Image != "zot/tea-a/src@sha256:aaa" {
		t.Fatalf("spec.buildSource = %+v", spec.BuildSource)
	}
	if plain := projectSpec(DesiredApp{App: App{TenantID: "tea-a", Name: "api", Image: "nginx"}}); plain.BuildSource != nil {
		t.Fatalf("an unlinked service projected %+v", plain.BuildSource)
	}

	live := spec
	d.BuildSourceImage = "zot/tea-a/src@sha256:bbb"
	if !applyOwnedSpec(&live, projectSpec(d)) || live.BuildSource.Image != "zot/tea-a/src@sha256:bbb" {
		t.Fatalf("selecting the next build did not reach the CR: %+v", live.BuildSource)
	}
	if applyOwnedSpec(&live, projectSpec(d)) {
		t.Error("an unchanged link reported a change")
	}
}
