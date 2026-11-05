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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// update_relation_test.go — UpdatePostgres is a lifecycle verb: it gates on
// can_operate, so a contributor can make an ordinary settings change. Before
// w2/066 a parameterOverrides write that turned on statement logging escalated
// to can_create (w1/m68 F7); with the field retired that escalation has nothing
// left to guard, and every update takes the plain lifecycle rung.

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

func serviceAs(t *testing.T, grants map[string]bool, objs ...client.Object) (*Service, *roleChecker, context.Context) {
	t.Helper()
	svc, _ := newService(objs...)
	chk := &roleChecker{grants: grants}
	svc.Base.Authz = chk
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	return svc, chk, ctx
}

// TestUpdatePostgresGatesOnOperate proves a contributor's rename lands on the
// can_operate rung and never asks for can_create.
func TestUpdatePostgresGatesOnOperate(t *testing.T) {
	svc, chk, ctx := serviceAs(t, contributorGrants)
	seedDatabase(t, svc.Base.Client, "param-db")

	name := "renamed-db"
	if _, err := svc.UpdatePostgres(ctx, "param-db", PostgresPatch{Name: &name}); err != nil {
		t.Fatalf("contributor rename => %v, want success", err)
	}

	var sawOperate bool
	for _, rel := range chk.relations {
		if rel == core.RelCanOperate {
			sawOperate = true
		}
		if rel == core.RelCanCreate {
			t.Errorf("an ordinary update gated on can_create — over-tightened")
		}
	}
	if !sawOperate {
		t.Error("no can_operate check observed; the fixture is not exercising the gate")
	}
}
