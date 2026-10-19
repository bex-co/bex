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
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestRebuildIsAConfigChangeAndTriggerStaysManual is w4/347: an environment
// save's rebuild was opened through Trigger and read as a Manual Deploy
// (api, envUpdated:false). Rebuild records the configuration change that
// asked for it; a Manual Deploy keeps its api cause. Both build the latest
// commit.
func TestRebuildIsAConfigChangeAndTriggerStaysManual(t *testing.T) {
	ds := newFakeStore()
	resolver := &fakeResolver{commit: store.CommitInfo{Hash: "abc1234def5678"}, ok: true}
	svc, _ := newService(ds, repoApp("web", "srv-1", "main"))
	svc.Commits = resolver

	rebuilt, err := svc.Rebuild(context.Background(), "web")
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if rebuilt.Trigger != store.TriggerConfigChange || rebuilt.CommitID != "abc1234def5678" || resolver.gotRef != "main" {
		t.Errorf("rebuild = trigger %q commit %q (ref %q), want config_change building main's head", rebuilt.Trigger, rebuilt.CommitID, resolver.gotRef)
	}

	manual, err := svc.Trigger(context.Background(), "web", TriggerParams{})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if manual.Trigger != store.TriggerAPI {
		t.Errorf("manual deploy trigger = %q, want %q", manual.Trigger, store.TriggerAPI)
	}
}
