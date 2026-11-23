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
	"strings"
	"testing"
)

// TestClusterSpecLogDefaults: every tenant Postgres logs lock waits, the
// source of the Metrics page's Lock-Delayed Queries graph (w7/m430), and each
// statement that runs 2 s or longer with PostgreSQL's "duration: N ms
// statement: …" line, as Render's do (w4/721). A database whose legacy
// spec.parameters declares either setting, in any spelling, keeps its own
// value and gets no second, conflicting entry, on the upstream operand and on
// a bex operand alike.
func TestClusterSpecLogDefaults(t *testing.T) {
	defaults := []struct{ name, platform, declared string }{
		{"log_lock_waits", "on", "off"},
		{"log_min_duration_statement", "2000", "-1"},
	}
	for _, operand := range []string{"upstream", "bex"} {
		t.Run(operand, func(t *testing.T) {
			if operand == "bex" {
				withBexOperands(t, map[string]string{"16": "ghcr.io/bex-co/bex-operator:16.15-postgres-abc@sha256:0123"})
			}
			_, _, params := clusterPostgresql(t, "16", nil)
			for _, d := range defaults {
				if params[d.name] != d.platform {
					t.Errorf("%s = %v, want %s for every tenant database", d.name, params[d.name], d.platform)
				}
				for _, key := range []string{d.name, strings.ToUpper(d.name[:1]) + d.name[1:]} {
					_, _, params := clusterPostgresql(t, "16", map[string]string{key: d.declared, "work_mem": "8MB"})
					if params[key] != d.declared || params["work_mem"] != "8MB" {
						t.Errorf("declared %s=%s: parameters = %v, want the tenant's value and work_mem kept", key, d.declared, params)
					}
					if v, ok := params[d.name]; key != d.name && ok {
						t.Errorf("declared %s: platform default %v still projected beside it", key, v)
					}
				}
			}
		})
	}
}
