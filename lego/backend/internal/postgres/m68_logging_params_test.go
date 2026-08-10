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

package postgres

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w1/m68 F7 — statement logging is a developer capability, not a contributor one.
//
// The disclosure this closes: a contributor holds can_operate (so they could
// write any PostgreSQL parameter) AND can_view_logs (so they could read the
// database's logs). Setting log_statement=all therefore turned the database's
// own query text — passwords, tokens, personal data, whatever appears as a
// literal — into something they could read, while ExecuteQuery and the
// top-query views correctly reserve exactly that for developer-and-up. The
// privilege ladder inverted through a settings write.

// roleChecker allows only the relations a role holds.
type roleChecker struct {
	grants    map[string]bool
	relations []string
}

func (c *roleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.relations = append(c.relations, relation)
	return c.grants[relation], nil
}

// contributorGrants mirrors model.fga: contributor holds can_view, can_view_logs
// and can_operate, but neither can_create nor can_view_sensitive.
var contributorGrants = map[string]bool{
	core.RelCanView:     true,
	core.RelCanViewLogs: true,
	core.RelCanOperate:  true,
}

// developerGrants adds the create/sensitive rungs.
var developerGrants = map[string]bool{
	core.RelCanView:          true,
	core.RelCanViewLogs:      true,
	core.RelCanOperate:       true,
	core.RelCanCreate:        true,
	core.RelCanViewSensitive: true,
}

func serviceAs(t *testing.T, grants map[string]bool, objs ...client.Object) (*Service, *roleChecker, context.Context) {
	t.Helper()
	svc, _ := newService(objs...)
	chk := &roleChecker{grants: grants}
	svc.Base.Authz = chk
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	return svc, chk, ctx
}

// TestStatementLoggingParametersRequireDeveloper is the attack, then the
// legitimate use. It fails on the pre-m68 code, where every parameter write
// gated on can_operate.
func TestStatementLoggingParametersRequireDeveloper(t *testing.T) {
	// Every setting that puts statement text (or its bind parameters) in the log.
	sensitive := []map[string]string{
		{"log_statement": "all"},
		{"log_min_duration_statement": "0"},
		{"log_min_error_statement": "error"},
		{"log_parameter_max_length": "-1"},
		{"auto_explain.log_analyze": "on"},
		// Mixed with an innocuous key: the map is a full replacement, so the
		// presence of one sensitive key governs the whole write.
		{"work_mem": "8MB", "log_statement": "mod"},
	}

	for _, params := range sensitive {
		svc, _, ctx := serviceAs(t, contributorGrants)
		seedDatabase(t, svc.Base.Client, "param-db")
		if _, err := svc.SetParameterOverrides(ctx, "param-db", params); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("contributor setting %v => %v, want ErrForbidden — this puts SQL literals into "+
				"logs the same contributor can read", params, err)
		}
	}

	// A developer holds can_create, so the legitimate slow-query workflow is
	// unaffected — the point is to move the boundary, not to remove the setting.
	for _, params := range sensitive {
		svc, _, ctx := serviceAs(t, developerGrants)
		seedDatabase(t, svc.Base.Client, "param-db")
		if _, err := svc.SetParameterOverrides(ctx, "param-db", params); err != nil {
			t.Errorf("developer setting %v => %v, want success", params, err)
		}
	}
}

// TestNonLoggingParametersStayContributorWritable is the other half: the gate
// must not swallow ordinary performance tuning, which is genuinely operational
// work and the reason parameter overrides gate on can_operate at all.
func TestNonLoggingParametersStayContributorWritable(t *testing.T) {
	svc, chk, ctx := serviceAs(t, contributorGrants)
	seedDatabase(t, svc.Base.Client, "param-db")

	if _, err := svc.SetParameterOverrides(ctx, "param-db", map[string]string{
		"work_mem":                    "8MB",
		"max_parallel_workers":        "4",
		"effective_cache_size":        "2GB",
		"log_connections":             "on", // logs THAT a connection happened, not what it ran
		"log_min_messages":            "warning",
		"default_statistics_target":   "200",
		"random_page_cost":            "1.1",
		"idle_in_transaction_timeout": "60000",
	}); err != nil {
		t.Fatalf("contributor tuning non-logging parameters => %v, want success", err)
	}

	// And prove it took the lifecycle rung rather than passing for another reason.
	var sawOperate bool
	for _, rel := range chk.relations {
		if rel == core.RelCanOperate {
			sawOperate = true
		}
		if rel == core.RelCanCreate {
			t.Errorf("ordinary tuning gated on can_create — over-tightened")
		}
	}
	if !sawOperate {
		t.Error("no can_operate check observed; the fixture is not exercising the gate")
	}
}

// TestSensitiveLoggingParameterMatchIsRobust pins the matcher itself: PostgreSQL
// treats setting names case-insensitively, so a gate that only matched the exact
// lowercase spelling would be bypassed by "LOG_STATEMENT".
func TestSensitiveLoggingParameterMatchIsRobust(t *testing.T) {
	for _, name := range []string{"log_statement", "LOG_STATEMENT", "Log_Statement", "  log_statement  "} {
		if !setsSensitiveLoggingParameter(map[string]string{name: "all"}) {
			t.Errorf("%q not recognized as a statement-logging parameter", name)
		}
	}
	for _, name := range []string{"work_mem", "log_connections", "log_destination"} {
		if setsSensitiveLoggingParameter(map[string]string{name: "x"}) {
			t.Errorf("%q wrongly treated as statement-logging — this costs a contributor a legitimate knob", name)
		}
	}
}
