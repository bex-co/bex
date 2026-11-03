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

import (
	"context"
	"testing"
)

type carrierCountingStore struct {
	Store
	carriers int
}

func (c carrierCountingStore) CountBuildSourcesForTenant(context.Context, string) (int, error) {
	return c.carriers, nil
}

// w2/m254: a build source carrier is an App object but not a service, so the
// App count quota is widened by exactly the carriers that exist — services
// stay capped at the plan's ceiling by the API server.
func TestAppCountQuotaMakesRoomForBuildSources(t *testing.T) {
	for _, carriers := range []int{0, 3} {
		r := &NamespaceReconciler{Store: carrierCountingStore{carriers: carriers}}
		quotas := r.baseResourceQuotas("tea-hobby", Tenant{ID: "tea-hobby", Plan: PlanHobby})
		if err := r.roomForBuildSources(context.Background(), "tea-hobby", quotas); err != nil {
			t.Fatal(err)
		}
		want := QuotaCapsForPlan(PlanHobby).Services + int64(carriers)
		found := false
		for _, q := range quotas {
			if apps, ok := q.Spec.Hard[AppsQuotaCountKey]; ok {
				found = true
				if apps.Value() != want {
					t.Errorf("%d carriers: %s = %d, want %d", carriers, AppsQuotaCountKey, apps.Value(), want)
				}
			}
		}
		if !found {
			t.Fatalf("no quota carries %s", AppsQuotaCountKey)
		}
	}
}
