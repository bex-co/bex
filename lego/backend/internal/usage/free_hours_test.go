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

package usage

import (
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/m139: the summary's Free instance hours count only Free service
// instance-seconds, against 750 unless overridden.
func TestFreeInstanceHoursCountsFreeServiceSecondsOnly(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "srv-a", Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindService, Total: 7200},
		{ServiceID: "srv-b", Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindService, Total: 3600},
		{ServiceID: "srv-c", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 99999},
		{ServiceID: "dpg-d", Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindPostgres, Total: 99999},
		{ServiceID: "srv-a", Kind: store.UsageKindEgressBytes, Total: 99999},
	}
	got := (&Service{}).freeInstanceHours(rows)
	if got.Used != 3 || got.Included != 750 {
		t.Fatalf("free hours = %+v, want 3 of 750", got)
	}
	if got := (&Service{FreeInstanceHours: 0.05}).freeInstanceHours(rows); got.Included != 0.05 {
		t.Fatalf("override ignored: %+v", got)
	}
}
