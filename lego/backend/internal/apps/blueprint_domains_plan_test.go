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

package apps

import (
	"slices"
	"testing"
	"time"
)

// TestBlueprintPlanNamesDomainChange pins that a sync changing a service's
// domains names the domains field, and only a field that changed: each
// field's probe starts from the same served-domain union the whole-spec probe
// does, never from the spec the whole-spec probe already rewrote.
func TestBlueprintPlanNamesDomainChange(t *testing.T) {
	const web = "services:\n  - name: web\n    type: web\n    runtime: image\n"
	for _, tc := range []struct {
		name, applied, planned string
		want                   []string
	}{
		{"domain added", web + "    image:\n      url: docker.io/library/nginx:1\n",
			web + "    image:\n      url: docker.io/library/nginx:1\n    domains: [app.example.com]\n", []string{"domains"}},
		{"image changed beside an unchanged domain", web + "    image:\n      url: docker.io/library/nginx:1\n    domains: [app.example.com]\n",
			web + "    image:\n      url: docker.io/library/nginx:2\n    domains: [app.example.com]\n", []string{"image"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := connectionService(t)
			svc.Store = &recordingStore{}
			svc.DomainTraffic = allowDomainTraffic{}
			now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			svc.Clock = func() time.Time { return now }
			ctx := ownershipCtx()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.applied}); err != nil {
				t.Fatalf("apply: %v", err)
			}
			actions := planActionsForTest(ctx, t, svc, tc.planned)
			if len(actions) != 1 || actions[0].Operation != BlueprintPlanUpdate {
				t.Fatalf("plan = %+v, want one update", actions)
			}
			var got []string
			for _, field := range actions[0].ChangedFields {
				got = append(got, field.Path)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("changed fields = %v, want %v", got, tc.want)
			}
		})
	}
}
