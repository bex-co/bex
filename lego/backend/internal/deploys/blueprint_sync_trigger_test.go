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

package deploys

import (
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/301: a rollout a Blueprint sync opened read `other` on REST, though
// Render's enum has blueprint_sync for exactly that cause.
func TestABlueprintSyncsRolloutIsRendersBlueprintSync(t *testing.T) {
	if got := renderTrigger(store.TriggerBlueprintSync); got != "blueprint_sync" {
		t.Errorf("renderTrigger(%q) = %q, want blueprint_sync", store.TriggerBlueprintSync, got)
	}
}
