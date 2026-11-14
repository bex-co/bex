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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A Blueprint on a repo whose pushes can't reach bex is reviewed and created
// without auto-sync, and turning auto-sync on is refused; a covered repo keeps
// it (w4/622 § Blueprints too). Only an install that rejects the shared manual
// webhook key has such repos.
func TestBlueprintOnAnUnreachableRepoIsCreatedWithoutAutoSync(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		granted, multitenant bool
		want                 bool
	}{
		{"uncovered", false, true, false},
		{"covered", true, true, true},
		{"uncovered, manual webhook accepted", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, fs := connectionService(t)
			svc.GitHub, svc.ManualWebhookDisabled = &fakeCloneTokens{token: "tok", ok: tc.granted}, tc.multitenant
			ctx := ownershipCtx()
			preview, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathCron, blueprintPreviewOptions{})
			if err != nil || preview.AutoSync == nil || *preview.AutoSync != tc.want {
				t.Fatalf("preview autoSync = %v (%v), want %v", preview.AutoSync, err, tc.want)
			}
			b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bp"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if stored := fs.blueprints[b.ID]; stored.AutoSync != tc.want || b.AutoSync != tc.want {
				t.Errorf("autoSync stored %v, read %v; want %v", stored.AutoSync, b.AutoSync, tc.want)
			}
			on := true
			_, err = svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{AutoSync: &on})
			if tc.want && err != nil {
				t.Errorf("autoSync on: %v", err)
			}
			if !tc.want && (!errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "autoSync needs a connected Git provider")) {
				t.Errorf("autoSync on for an unreachable repo = %v, want the refusal", err)
			}
		})
	}
}
