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

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/types/tiers"
)

// TestEveryPlansServerRunsItsConnectionLimit: w6/134. Each plan's PostgreSQL
// runs max_connections at the plan's Render figure (tiers maxConnections,
// render.com/docs/compute-plans), on the upstream operand and on a bex operand
// alike. No legacy declared value, in any spelling, reaches postgresql.conf
// beside it: PgBouncer's pool and the SNI proxies' route admission are sized
// from the same figure.
func TestEveryPlansServerRunsItsConnectionLimit(t *testing.T) {
	serverParameters := func(plan tiers.PostgresTier, declared map[string]string) map[string]any {
		spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: plan.StorageGB, version: "16", dbname: "app", owner: "app", parameters: declared})
		return spec["postgresql"].(map[string]any)["parameters"].(map[string]any)
	}
	declarations := []map[string]string{
		nil,
		{"max_connections": "500"},
		{" MAX_Connections ": "20", "work_mem": "8MB"},
	}
	// Render's 2c-8g (200, w6/m332): the figure is the plan's, not a constant.
	larger := tiers.Postgres.Resolve("basic-4gb")
	larger.ID, larger.MaxConnections = "2c-8g", 200
	ids := tiers.Postgres.IDs()
	plans := make([]tiers.PostgresTier, 0, len(ids)+1)
	plans = append(plans, larger)
	for _, id := range ids {
		plan, _ := tiers.Postgres.ByID(id)
		plans = append(plans, plan)
	}
	for _, operand := range []string{"upstream", "bex"} {
		t.Run(operand, func(t *testing.T) {
			if operand == "bex" {
				withBexOperands(t, map[string]string{"16": "ghcr.io/bex-co/bex-operator:16.15-postgres-abc@sha256:0123"})
			}
			for _, plan := range plans {
				want := strconv.Itoa(int(plan.MaxConnections))
				for _, declared := range declarations {
					params := serverParameters(plan, declared)
					if params["max_connections"] != want {
						t.Errorf("plan %q declaring %v: max_connections = %v, want the plan's %s", plan.ID, declared, params["max_connections"], want)
					}
					for name := range params {
						if name != "max_connections" && strings.EqualFold(strings.TrimSpace(name), "max_connections") {
							t.Errorf("plan %q: declared %q reached postgresql.conf beside the plan's limit", plan.ID, name)
						}
					}
					if declared["work_mem"] != "" && params["work_mem"] != declared["work_mem"] {
						t.Errorf("plan %q: declared work_mem dropped with max_connections: %v", plan.ID, params)
					}
				}
			}
		})
	}
}
