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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// server_failed_reason_test.go is w8/m158 t003. A running service's process
// that exited or was killed for memory reads, on REST and MCP, as Render's
// "Server Failed" reason does: nonZeroExit, oomKilled.memoryLimit or
// earlyExit, with the instance that failed.
func TestServerFailedReasonNamesTheCause(t *testing.T) {
	exit := int32(3)
	for _, tc := range []struct {
		name   string
		row    store.ServiceEventRow
		tier   string
		want   string // a fragment of details.reason
		refuse string // must not appear
	}{
		{"a crash", store.ServiceEventRow{ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &exit}, "starter", `"nonZeroExit":3`, "unhealthy"},
		{"a memory kill on Free", store.ServiceEventRow{ReasonCode: store.EventReasonOOMKilled}, "free", `"oomKilled":{"memoryLimit":"512Mi"}`, "unhealthy"},
		{"a memory kill, plan unset", store.ServiceEventRow{ReasonCode: store.EventReasonOOMKilled}, "", `"oomKilled":{"memoryLimit":"512Mi"}`, "unhealthy"},
		{"an exit with status 0", store.ServiceEventRow{ReasonCode: store.EventReasonEarlyExit}, "starter", `"earlyExit":true`, "nonZeroExit"},
		{"a failed health check", store.ServiceEventRow{ReasonCode: store.EventReasonReadinessFailed}, "starter", `"unhealthy":"The instance failed its health check"`, "earlyExit"},
	} {
		tc.row.Source, tc.row.FactType, tc.row.Key = store.EventSourceFact, TypeServerFailed, "k:"+tc.name
		tc.row.InstanceID = "web-7d9f5c-abcde"
		ev := view(tc.row, "srv-1")
		withMemoryLimit(&ev, tc.tier)
		body, err := json.Marshal(toRenderEvent(ev))
		if err != nil {
			t.Fatal(err)
		}
		got := string(body)
		if !strings.Contains(got, tc.want) || strings.Contains(got, tc.refuse) {
			t.Errorf("%s: body %s, want %s and no %q", tc.name, got, tc.want, tc.refuse)
		}
		if !strings.Contains(got, `"instanceID":"web-7d9f5c-abcde"`) {
			t.Errorf("%s: body %s carries no instanceID", tc.name, got)
		}
	}
	// The limit belongs to a memory kill of a service: no other event gets it.
	other := view(store.ServiceEventRow{Source: store.EventSourceFact, FactType: TypeServerFailed, ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &exit, Key: "k"}, "srv-1")
	withMemoryLimit(&other, "free")
	if other.Details.MemoryLimit != "" {
		t.Errorf("a crash carries a memory limit %q", other.Details.MemoryLimit)
	}
}
