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

package projects

import (
	"testing"
	"time"
)

func TestToRenderProjectIncludesEnvironmentMembership(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	got := toRenderProject(ProjectView{ID: "prj-1", Name: "platform", OwnerID: "tea-1", CreatedAt: created}, []string{"env-1"})
	if got.ID != "prj-1" || got.Owner.ID != "tea-1" || got.Owner.Type != "team" || len(got.EnvironmentIDs) != 1 || got.EnvironmentIDs[0] != "env-1" {
		t.Fatalf("Render project = %+v", got)
	}
	if !got.UpdatedAt.Equal(created) {
		t.Fatalf("updatedAt = %v, want %v", got.UpdatedAt, created)
	}
}
