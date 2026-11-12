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

// ServesEdgeCache is the one predicate the operator starts the edge-cache
// sidecar on and bex-api records edge_cache_enabled / _disabled by (w9/m345).
func TestServesEdgeCache(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec AppSpec
		want bool
	}{
		{"a paid web service with a caching profile", AppSpec{Tier: "standard", CacheProfile: CacheProfileStatic}, true},
		{"the all profile", AppSpec{Tier: "standard", Type: TypeWebService, CacheProfile: CacheProfileAll}, true},
		{"no caching profile", AppSpec{Tier: "standard", CacheProfile: CacheProfileNone}, false},
		{"a free web service", AppSpec{Tier: "free", CacheProfile: CacheProfileStatic}, false},
		{"an empty tier is free", AppSpec{CacheProfile: CacheProfileStatic}, false},
		{"not a web service", AppSpec{Tier: "standard", Type: TypePrivateService, CacheProfile: CacheProfileStatic}, false},
		{"a web service on its image's port", AppSpec{Tier: "standard", Type: TypeWebService, PortMode: PortModeWebImageV1, CacheProfile: CacheProfileStatic}, false},
	} {
		if got := tc.spec.ServesEdgeCache(); got != tc.want {
			t.Errorf("%s: ServesEdgeCache() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
