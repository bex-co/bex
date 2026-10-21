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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A manifest whose per-service create checks fail answers 400 before
// admission, leaving no auto-syncing Blueprint row behind (w8/166).
func TestBlueprintCreateRefusalLeavesNoRow(t *testing.T) {
	fs := newFakeBlueprintStore()
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Blueprints: fs,
		GitFetcher: multiPathFetcher{byPath: map[string]string{
			"render.yaml": "services:\n  - type: web\n    runtime: static\n    name: s\n    repo: https://github.com/acme/mono\n    staticPublishPath: public\n    rootDir: ../escape\n",
		}},
	}
	_, err := svc.CreateBlueprint(ownershipCtx(), connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: "render.yaml", Name: "bad"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("create = %v, want a bad request", err)
	}
	if len(fs.blueprints) != 0 {
		t.Fatalf("a refused create left %d blueprint row(s): %+v", len(fs.blueprints), fs.blueprints)
	}
}
