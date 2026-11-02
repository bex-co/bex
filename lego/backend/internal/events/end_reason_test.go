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

// w8/m146: deploy_ended and build_ended carry Render's buildDeployEndReason,
// always present, filled only from structured facts.
func TestDeployAndBuildEndedCarryRendersReason(t *testing.T) {
	exit := int32(1)
	ended := func(status, cancel string, end store.DeployEnd) store.ServiceEventRow {
		return store.ServiceEventRow{Key: "dep-old:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded,
			DeployID: "dep-old", Status: status, CancelReason: cancel, End: end}
	}
	for _, tc := range []struct {
		name string
		row  store.ServiceEventRow
		want string
	}{
		{"startup crash", ended(store.DeployUpdateFailed, "", store.DeployEnd{ExitCode: &exit}),
			`{"failure":{"earlyExit":true,"evicted":false,"nonZeroExit":1}}`},
		{"deadline with a crash on record", ended(store.DeployUpdateFailed, "", store.DeployEnd{ExitCode: &exit, TimedOutSeconds: 900}),
			`{"failure":{"evicted":false,"nonZeroExit":1,"timedOutSeconds":900}}`},
		{"oom", ended(store.DeployUpdateFailed, "", store.DeployEnd{ExitCode: &[]int32{137}[0], OOMKilled: true}),
			`{"failure":{"earlyExit":true,"evicted":false,"nonZeroExit":137,"oomKilled":{"memoryLimit":""}}}`},
		{"failed with no structured cause", ended(store.DeployUpdateFailed, "", store.DeployEnd{}), `{}`},
		{"build failed", ended(store.DeployBuildFailed, "", store.DeployEnd{}), `{"buildFailed":{"id":"dep-old"}}`},
		{"superseded", ended(store.DeployCanceled, "Superseded by dep-new", store.DeployEnd{}), `{"newDeploy":{"id":"dep-new"}}`},
		{"restart", ended(store.DeployCanceled, "Canceled by restart dep-new, which kept the running release: what this deploy was applying was not deployed", store.DeployEnd{}),
			`{"newDeploy":{"id":"dep-new"}}`},
		{"user cancel", ended(store.DeployCanceled, "", store.DeployEnd{}), `{}`},
		{"live", ended(store.DeployLive, "", store.DeployEnd{}), `{}`},
		{"build superseded", store.ServiceEventRow{Key: "fact:x", Source: store.EventSourceFact, FactType: TypeBuildEnded,
			DeployID: "dep-old", FactStatus: store.EventStatusCanceled, CancelReason: "Superseded by dep-new"}, `{"newBuild":{"id":"dep-new"}}`},
		{"build succeeded", store.ServiceEventRow{Key: "fact:y", Source: store.EventSourceFact, FactType: TypeBuildEnded,
			DeployID: "dep-old", FactStatus: store.EventStatusSucceeded}, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(toRenderEvent(view(tc.row, "srv-1")).Details)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Reason json.RawMessage `json:"reason"`
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if string(got.Reason) != tc.want {
				t.Fatalf("reason = %s, want %s (details %s)", got.Reason, tc.want, body)
			}
		})
	}
}
