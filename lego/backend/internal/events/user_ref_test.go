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
package events

import (
	"encoding/json"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/391: Render's suspender pair carries actor as the kind plus the human
// {id, email}; a human-triggered deploy carries trigger.user.
func TestEventUserObjects(t *testing.T) {
	ev := view(store.ServiceEventRow{Key: "aud-1:", Source: store.EventSourceAudit, Verb: "apps.Suspend", Caller: "user-x"}, "web")
	if ev.Details.Actor != "user" || ev.Details.SuspendedByUser == nil || ev.Details.SuspendedByUser.ID != "user-x" {
		t.Fatalf("suspender = %+v", ev.Details)
	}
	body, _ := json.Marshal(toRenderEvent(ev))
	var decoded struct {
		Details struct {
			Actor           string `json:"actor"`
			SuspendedByUser struct {
				ID string `json:"id"`
			} `json:"suspendedByUser"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Details.Actor != "user" || decoded.Details.SuspendedByUser.ID != "user-x" {
		t.Fatalf("rendered = %s", body)
	}

	deploy := view(store.ServiceEventRow{Key: "dep-1:", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, Caller: "user-y", Trigger: store.TriggerAPI}, "web")
	if deploy.Details.Trigger == nil || deploy.Details.Trigger.User == nil || deploy.Details.Trigger.User.ID != "user-y" {
		t.Fatalf("deploy trigger = %+v", deploy.Details.Trigger)
	}
	hook := view(store.ServiceEventRow{Key: "dep-2:", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, Trigger: store.TriggerDeployHook}, "web")
	if hook.Details.Trigger != nil && hook.Details.Trigger.User != nil {
		t.Fatalf("hook trigger carries a user: %+v", hook.Details.Trigger)
	}
}
