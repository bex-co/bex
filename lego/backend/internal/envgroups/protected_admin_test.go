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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

// prodEnv is a protected environment's storage id, the spelling a group's row
// and a service's label carry; every view answers with its public evm- alias.
var prodEnv = id.EnvironmentStorageID(id.New(id.Environment))

// protectionStatuses is a core.EnvironmentProtectionStore over a fixed map.
type protectionStatuses map[string]string

func (p protectionStatuses) GetEnvironmentProtectedStatus(_ context.Context, environmentID string) (string, error) {
	if status := p[environmentID]; status != "" {
		return status, nil
	}
	return core.ProtectedStatusUnprotected, nil
}

type failingProtection struct{}

func (failingProtection) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	return "", errors.New("store down")
}

// developer allows every relation but the workspace Admin's can_manage.
var developer = refuseRelation{relation: core.RelCanManage}

// viewer may view and nothing more.
type viewer struct{}

func (viewer) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation == core.RelCanView, nil
}

func adminOnlyRefusal(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == core.ProtectedPlacementCode &&
		errors.Is(err, core.ErrForbidden) && coded.Params["environmentId"] == id.EnvironmentPublicID(prodEnv)
}

// protectedGroup is a group scoped to env-production holding one variable and
// one secret file, linked to web (a service there), with api a second service
// there it is not linked to. Authz and protection are set after the setup.
func protectedGroup(t *testing.T, protected bool, authz core.Checker) (*Service, EnvGroupView) {
	t.Helper()
	web, api := sampleApp("web"), sampleApp("api")
	web.Labels[core.LabelEnvironment] = prodEnv
	api.Labels[core.LabelEnvironment] = prodEnv
	svc := newService(newFakeStore(), web, api)
	svc.EnvironmentWorkspace = environmentResolver(prodEnv, "env-staging")
	group, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{
		Name: "prod-secrets", EnvironmentID: prodEnv,
		EnvVars:     []CreateEnvVarInput{{Key: "API_KEY", Value: "s3cret-value", ValueSet: true}},
		SecretFiles: []SecretFileView{{Name: "cert.pem", Content: "PEM-BODY"}},
		ServiceIDs:  []string{"web"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	status := core.ProtectedStatusUnprotected
	if protected {
		status = core.ProtectedStatusProtected
	}
	svc.Protection = protectionStatuses{prodEnv: status}
	svc.Authz = authz
	return svc, group
}

// Render: "Deleting a group and viewing or changing its variables or secret
// files are admin-only actions in a protected environment"
// (render.com/docs/projects; w4/m594). Before, a Developer did all of it, as
// can_view_sensitive and can_create let them anywhere.
func TestProtectedEnvGroupVerbsNeedAnAdmin(t *testing.T) {
	verbs := []struct {
		name string
		run  func(s *Service, ctx context.Context, gid string) error
	}{
		{"reveal a variable", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.GetEnvGroupVar(ctx, gid, "API_KEY")
			return err
		}},
		{"reveal a file", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.GetEnvGroupFile(ctx, gid, "cert.pem")
			return err
		}},
		{"replace the variables", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.SetEnvGroupVars(ctx, gid, []EnvVarView{{Key: "API_KEY", Value: "changed", ValueSet: true}})
			return err
		}},
		{"set a variable", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.SetEnvGroupVarInput(ctx, gid, EnvVarView{Key: "API_KEY", Value: "changed", ValueSet: true})
			return err
		}},
		{"delete a variable", func(s *Service, ctx context.Context, gid string) error {
			return s.DeleteEnvGroupVar(ctx, gid, "API_KEY")
		}},
		{"set a file", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.SetEnvGroupFile(ctx, gid, "cert.pem", "CHANGED")
			return err
		}},
		{"delete a file", func(s *Service, ctx context.Context, gid string) error {
			return s.DeleteEnvGroupFile(ctx, gid, "cert.pem")
		}},
		{"save the editor's draft", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.PatchEnvironment(ctx, gid, EnvironmentPatch{
				EnvVars: []EnvVarPatch{{Key: "API_KEY", Value: "changed", ValueSet: true}}, SaveMode: SaveModeOnly,
			})
			return err
		}},
		{"clone", func(s *Service, ctx context.Context, gid string) error {
			_, err := s.CloneEnvGroup(ctx, gid, CloneEnvGroupRequest{Name: "copy", EnvironmentID: "env-staging"})
			return err
		}},
		{"link a service", func(s *Service, ctx context.Context, gid string) error {
			return s.LinkService(ctx, gid, "api")
		}},
		{"unlink a service", func(s *Service, ctx context.Context, gid string) error {
			return s.UnlinkService(ctx, gid, "web")
		}},
		{"delete the group", func(s *Service, ctx context.Context, gid string) error {
			return s.DeleteEnvGroup(ctx, gid)
		}},
	}
	for _, verb := range verbs {
		for _, admin := range []bool{true, false} {
			for _, protected := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/admin=%t/protected=%t", verb.name, admin, protected), func(t *testing.T) {
					var authz core.Checker = developer
					if admin {
						authz = nil // every relation, can_manage included
					}
					svc, group := protectedGroup(t, protected, authz)
					err := verb.run(svc, testCtx(), group.ID)
					if admin || !protected {
						if err != nil {
							t.Fatalf("%s by an allowed caller: %v", verb.name, err)
						}
						return
					}
					if !adminOnlyRefusal(err) {
						t.Fatalf("%s by a Developer on a protected group = %v, want the coded admin-only refusal", verb.name, err)
					}
					for _, secret := range []string{"s3cret-value", "PEM-BODY"} {
						if strings.Contains(err.Error(), secret) {
							t.Errorf("the refusal leaked %q: %v", secret, err)
						}
					}
					// Nothing moved: the Admin still reads what was there.
					svc.Authz = nil
					if v, err := svc.GetEnvGroupVar(testCtx(), group.ID, "API_KEY"); err != nil || v.Value != "s3cret-value" {
						t.Errorf("after the refusal the variable reads %+v (%v), want unchanged", v, err)
					}
					if f, err := svc.GetEnvGroupFile(testCtx(), group.ID, "cert.pem"); err != nil || f.Content != "PEM-BODY" {
						t.Errorf("after the refusal the file reads %+v (%v), want unchanged", f, err)
					}
					after, err := svc.GetEnvGroup(testCtx(), group.ID)
					if err != nil || !slices.Equal(after.ServiceLinks, []string{"web"}) {
						t.Errorf("after the refusal the links read %v (%v), want [web]", after.ServiceLinks, err)
					}
				})
			}
		}
	}
}

// Render's envGroup omits envVars and secretFiles "if the caller doesn't have
// the permission to read the environment group's secret values": a Developer
// on a protected group reads its metadata, as a Viewer does anywhere, and an
// Admin reads its values.
func TestProtectedEnvGroupRESTReadOmitsTheValuesForANonAdmin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		authz  core.Checker
		values bool
	}{{"developer", developer, false}, {"admin", nil, true}} {
		t.Run(tc.name, func(t *testing.T) {
			svc, group := protectedGroup(t, true, tc.authz)
			rec := serveREST(svc, http.MethodGet, "/v1/env-groups/"+group.ID, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %s", rec.Code, rec.Body)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			_, hasVars := body["envVars"]
			_, hasFiles := body["secretFiles"]
			if hasVars != tc.values || hasFiles != tc.values || strings.Contains(rec.Body.String(), "s3cret-value") != tc.values {
				t.Fatalf("the %s's group = %s, want values=%t", tc.name, rec.Body, tc.values)
			}
			if body["id"] != group.ID || body["environmentId"] != id.EnvironmentPublicID(prodEnv) {
				t.Errorf("the group lacks its metadata: %s", rec.Body)
			}
		})
	}
}

// A Blueprint changes what its render.yaml manages whoever published it:
// "non-Admin workspace members can still modify those resources by publishing
// an update to the corresponding render.yaml file".
func TestProtectedEnvGroupBlueprintApplyStaysOpen(t *testing.T) {
	svc, group := protectedGroup(t, true, developer)
	if _, err := svc.ApplyEnvGroup(testCtx(), "prod-secrets", map[string]string{"API_KEY": "from-render-yaml"}, nil); err != nil {
		t.Fatalf("a Blueprint's apply to a protected group: %v", err)
	}
	if err := svc.LinkEnvGroup(testCtx(), "prod-secrets", "api"); err != nil {
		t.Fatalf("a Blueprint's link in a protected environment: %v", err)
	}
	svc.Authz = nil
	if v, err := svc.GetEnvGroupVar(testCtx(), group.ID, "API_KEY"); err != nil || v.Value != "from-render-yaml" {
		t.Fatalf("the applied variable reads %+v (%v)", v, err)
	}
}

// A protection status that cannot be read refuses rather than reveals.
func TestProtectedEnvGroupGatesFailClosed(t *testing.T) {
	svc, group := protectedGroup(t, true, nil)
	svc.Protection = failingProtection{}
	if _, err := svc.GetEnvGroupVar(testCtx(), group.ID, "API_KEY"); err == nil {
		t.Fatal("a reveal with an unreadable protection status succeeded")
	}
	if _, err := svc.SetEnvGroupVarInput(testCtx(), group.ID, EnvVarView{Key: "API_KEY", Value: "x", ValueSet: true}); err == nil {
		t.Fatal("a write with an unreadable protection status succeeded")
	}
}

// A group with no environment belongs to no protected environment.
func TestAWorkspaceGroupIsNotProtected(t *testing.T) {
	svc := newService(newFakeStore())
	group, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: "shared",
		EnvVars: []CreateEnvVarInput{{Key: "K", Value: "v", ValueSet: true}}})
	if err != nil {
		t.Fatal(err)
	}
	svc.Protection = protectionStatuses{prodEnv: core.ProtectedStatusProtected}
	svc.Authz = developer
	if _, err := svc.GetEnvGroupVar(testCtx(), group.ID, "K"); err != nil {
		t.Fatalf("a Developer's reveal on a workspace group: %v", err)
	}
}

// envGroupActions says what the verbs do (w4/m594): for an Admin, a Developer
// and a Viewer, on a protected and an unprotected group, each row reads
// allowed exactly when its verb succeeds, a protected Developer's refusal is
// named, and a Viewer reads the role reason.
func TestEnvGroupActionsAgreeWithTheVerbs(t *testing.T) {
	verbs := map[string]func(s *Service, ctx context.Context, gid string) error{
		core.ActionViewEnvVars: func(s *Service, ctx context.Context, gid string) error {
			_, err := s.GetEnvGroupVar(ctx, gid, "API_KEY")
			return err
		},
		core.ActionUpdateEnvVars: func(s *Service, ctx context.Context, gid string) error {
			_, err := s.SetEnvGroupVarInput(ctx, gid, EnvVarView{Key: "API_KEY", Value: "changed", ValueSet: true})
			return err
		},
		core.ActionDelete: func(s *Service, ctx context.Context, gid string) error {
			return s.DeleteEnvGroup(ctx, gid)
		},
	}
	roles := []struct {
		name   string
		authz  core.Checker
		reason string // on a protected group
	}{
		{"admin", nil, ""},
		{"developer", developer, core.ReasonProtectedEnvironmentAdmin},
		{"viewer", viewer{}, core.ReasonInsufficientPermission},
	}
	for _, role := range roles {
		for _, protected := range []bool{true, false} {
			for action, verb := range verbs {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", role.name, action, protected), func(t *testing.T) {
					svc, group := protectedGroup(t, protected, role.authz)
					acts, err := svc.ActionCapabilities(testCtx(), group.ID)
					if err != nil {
						t.Fatal(err)
					}
					var got *core.ActionDecision
					for i := range acts {
						if acts[i].Action == action {
							got = &acts[i]
						}
					}
					if got == nil {
						t.Fatalf("no %s row in %+v", action, acts)
					}
					verbErr := verb(svc, testCtx(), group.ID)
					if (got.Outcome == core.DecisionAllowed) != (verbErr == nil) {
						t.Fatalf("projection %+v and verb error %v disagree", *got, verbErr)
					}
					want := role.reason
					if !protected && role.name == "developer" {
						want = ""
					}
					if got.Reason != want {
						t.Errorf("%s reason = %q, want %q", action, got.Reason, want)
					}
				})
			}
		}
	}
}
