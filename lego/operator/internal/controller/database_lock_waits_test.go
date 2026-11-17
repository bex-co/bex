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

package controller

import "testing"

// TestClusterSpecLogsLockWaits: every tenant Postgres logs lock waits, the
// source of the Metrics page's Lock-Delayed Queries graph (w7/m430), and a
// database whose legacy spec.parameters declares the setting keeps its own
// value, on the upstream operand and on a bex operand alike.
func TestClusterSpecLogsLockWaits(t *testing.T) {
	for _, operand := range []string{"upstream", "bex"} {
		t.Run(operand, func(t *testing.T) {
			if operand == "bex" {
				withBexOperands(t, map[string]string{"16": "ghcr.io/bex-co/bex-operator:16.15-postgres-abc@sha256:0123"})
			}
			if _, _, params := clusterPostgresql(t, "16", nil); params["log_lock_waits"] != "on" {
				t.Errorf("log_lock_waits = %v, want on for every tenant database", params["log_lock_waits"])
			}
			_, _, params := clusterPostgresql(t, "16", map[string]string{"log_lock_waits": "off", "work_mem": "8MB"})
			if params["log_lock_waits"] != "off" || params["work_mem"] != "8MB" {
				t.Errorf("declared parameters = %v, want the tenant's log_lock_waits=off and work_mem kept", params)
			}
		})
	}
}
