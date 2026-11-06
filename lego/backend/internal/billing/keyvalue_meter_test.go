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

package billing

import (
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w5/m266 D5: every paid Key Value tier exports on the one cost-denominated
// meter, in micro-USD at its tier's rate; free stays local; other kinds keep
// their per-tier meters.
func TestKeyValueHoursExportAsCost(t *testing.T) {
	row := func(rk, tier string) store.HourlyRow {
		return store.HourlyRow{WorkspaceID: "tea-a", ServiceID: "red-x", Kind: store.UsageKindInstanceSeconds,
			Tier: tier, ResourceKind: rk, WindowStart: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), Quantity: 3600}
	}
	for tier, want := range map[string]string{"standard": "19178.082192", "pro_plus": "191780.821918"} {
		name, value, skip := stripeMeterEvent(toEvent(row(store.ResourceKindKeyValue, tier)))
		if skip || name != "key_value_cost_microusd" || value != want {
			t.Errorf("%s hour = %q %q skip=%v, want key_value_cost_microusd %s", tier, name, value, skip, want)
		}
	}
	if _, _, skip := stripeMeterEvent(toEvent(row(store.ResourceKindKeyValue, "free"))); !skip {
		t.Error("a free Key Value hour must stay local")
	}
	if name, value, _ := stripeMeterEvent(toEvent(row(store.ResourceKindPostgres, "basic-1gb"))); name != "instance_seconds.postgres.basic-1gb" || value != "3600" {
		t.Errorf("postgres hour = %q %q, want its per-tier meter in seconds", name, value)
	}
}
