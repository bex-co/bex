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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestBlueprintViewNeverAppliedReadsCreated (w4/368): a Blueprint whose
// history is empty must not claim in_sync. Every legitimate in_sync path
// stamps last_sync_at, so the stamp is the projection key; rows with one
// keep their lifecycle status untouched.
func TestBlueprintViewNeverAppliedReadsCreated(t *testing.T) {
	stamped := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		b    store.Blueprint
		want string
	}{
		"auto-registered before recording, no stamp": {store.Blueprint{Status: store.BlueprintStatusInSync}, store.BlueprintStatusCreated},
		"legacy active, no stamp":                    {store.Blueprint{Status: "active"}, store.BlueprintStatusCreated},
		"in_sync with a stamp":                       {store.Blueprint{Status: store.BlueprintStatusInSync, LastSyncAt: &stamped}, store.BlueprintStatusInSync},
		"legacy active with a stamp":                 {store.Blueprint{Status: "active", LastSyncAt: &stamped}, store.BlueprintStatusInSync},
		"paused without a stamp":                     {store.Blueprint{Status: store.BlueprintStatusPaused}, store.BlueprintStatusPaused},
		"error with a stamp":                         {store.Blueprint{Status: store.BlueprintStatusError, LastSyncAt: &stamped}, store.BlueprintStatusError},
		"syncing without a stamp":                    {store.Blueprint{Status: store.BlueprintStatusSyncing}, store.BlueprintStatusSyncing},
	} {
		if got := toBlueprintView(tc.b).Status; got != tc.want {
			t.Errorf("%s: status = %q, want %q", name, got, tc.want)
		}
	}
}
