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
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// update_relation_test.go — a Postgres instance's settings are a Developer's
// to change: UpdatePostgres, SetPlan and SetVersion gate on can_create, so a
// Contributor, who Render's role table gives no "Modify service configuration"
// (render.com/docs/team-members, read 2026-11-23), is refused them before
// anything is written (w9/m619). Restart, suspend, resume and failover are
// runtime actions and stay on can_operate.

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

// developerGrants adds can_create and can_view_sensitive.
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

// TestAContributorChangesNoPostgresSetting proves each setting verb refuses a
// Contributor with ErrForbidden and leaves the instance as it was, read back
// from the store.
func TestAContributorChangesNoPostgresSetting(t *testing.T) {
	name, plan, version := "renamed-db", "basic_256mb", "18"
	for verb, call := range map[string]func(*Service, context.Context) error{
		"UpdatePostgres": func(s *Service, ctx context.Context) error {
			_, err := s.UpdatePostgres(ctx, "settings-db", PostgresPatch{Name: &name})
			return err
		},
		"UpdatePostgres(plan)": func(s *Service, ctx context.Context) error {
			_, err := s.UpdatePostgres(ctx, "settings-db", PostgresPatch{Plan: &plan})
			return err
		},
		"SetPlan": func(s *Service, ctx context.Context) error {
			_, err := s.SetPlan(ctx, "settings-db", plan)
			return err
		},
		"SetVersion": func(s *Service, ctx context.Context) error {
			_, err := s.SetVersion(ctx, "settings-db", version)
			return err
		},
	} {
		t.Run(verb, func(t *testing.T) {
			svc, chk, ctx := serviceAs(t, contributorGrants)
			seedDatabase(t, svc.Base.Client, "settings-db")
			before := getDatabase(t, svc.Base.Client, "settings-db").Spec

			if err := call(svc, ctx); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("contributor %s => %v, want ErrForbidden", verb, err)
			}
			if !slices.Contains(chk.relations, core.RelCanCreate) {
				t.Errorf("contributor %s asked %v, want a can_create check", verb, chk.relations)
			}
			after := getDatabase(t, svc.Base.Client, "settings-db").Spec
			if after.Name != before.Name || after.Plan != before.Plan || after.Version != before.Version {
				t.Errorf("a refused %s wrote the spec: %+v -> %+v", verb, before, after)
			}
		})
	}
}

// TestADeveloperChangesPostgresSettings is the counterweight: the same rename
// lands for a Developer.
func TestADeveloperChangesPostgresSettings(t *testing.T) {
	svc, _, ctx := serviceAs(t, developerGrants)
	seedDatabase(t, svc.Base.Client, "settings-db")

	name := "renamed-db"
	if _, err := svc.UpdatePostgres(ctx, "settings-db", PostgresPatch{Name: &name}); err != nil {
		t.Fatalf("developer rename => %v, want success", err)
	}
	if got := getDatabase(t, svc.Base.Client, "settings-db").Spec.Name; got != name {
		t.Errorf("developer rename stored %q, want %q", got, name)
	}
}

// TestAContributorKeepsPostgresRuntimeActions pins the other side of the line:
// restart, suspend, resume and failover are not settings, so a Contributor is
// never refused them for their role (a verb may still refuse on state).
func TestAContributorKeepsPostgresRuntimeActions(t *testing.T) {
	for verb, call := range map[string]func(*Service, context.Context) error{
		"Restart": func(s *Service, ctx context.Context) error {
			_, err := s.Restart(ctx, "runtime-db")
			return err
		},
		"Suspend": func(s *Service, ctx context.Context) error {
			_, err := s.Suspend(ctx, "runtime-db")
			return err
		},
		"Resume": func(s *Service, ctx context.Context) error {
			_, err := s.Resume(ctx, "runtime-db")
			return err
		},
		"Failover": func(s *Service, ctx context.Context) error {
			return s.Failover(ctx, "runtime-db")
		},
	} {
		t.Run(verb, func(t *testing.T) {
			svc, chk, ctx := serviceAs(t, contributorGrants)
			seedDatabase(t, svc.Base.Client, "runtime-db")
			if err := call(svc, ctx); errors.Is(err, core.ErrForbidden) {
				t.Fatalf("contributor %s => %v, want no role refusal", verb, err)
			}
			if slices.Contains(chk.relations, core.RelCanCreate) {
				t.Errorf("contributor %s asked can_create (%v); a runtime action must stay can_operate", verb, chk.relations)
			}
		})
	}
}
