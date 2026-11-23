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

package keyvalue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// adminOnlyRefusal reports err as the coded refusal a protected environment
// gives a non-Admin (w4/m594): 403, PROTECTED_ENVIRONMENT_ADMIN_REQUIRED, and
// the environment's public id in params.
func adminOnlyRefusal(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == core.ProtectedPlacementCode &&
		errors.Is(err, core.ErrForbidden) && coded.Params["environmentId"] == "env-production"
}

// protectedReadyKeyValue is seedKeyValue's Ready public store, a member of
// env-production (protected unless protected is false), with saved rules.
func protectedReadyKeyValue(t *testing.T, protected bool) (*Service, client.Client, *fakeProtectionStore) {
	t.Helper()
	svc, cl, store := protectedKeyValueService()
	seedKeyValue(t, cl, "red-cache")
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "red-cache"}, &kv); err != nil {
		t.Fatal(err)
	}
	kv.Labels = map[string]string{core.LabelEnvironment: "env-production"}
	kv.Spec.Name = "cache"
	kv.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "office"}}
	if err := cl.Update(context.Background(), &kv); err != nil {
		t.Fatal(err)
	}
	if !protected {
		store.statuses["env-production"] = core.ProtectedStatusUnprotected
	}
	return svc, cl, store
}

func storedAllowList(t *testing.T, cl client.Client) []appv1alpha1.IPAllowEntry {
	t.Helper()
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "red-cache"}, &kv); err != nil {
		t.Fatal(err)
	}
	return kv.Spec.IPAllowList
}

var newRules = []core.IPAllowListEntry{{CIDRBlock: "198.51.100.0/24", Description: "vpn"}}

// Render: in a protected environment only Admins "view passwords or
// connection URLs for any … Key Value instance" and "modify access control
// IPs" for it (render.com/docs/projects; w4/m594). Before, a Developer read
// both and changed the rules, as can_view_sensitive and can_operate let them
// anywhere.
func TestProtectedKeyValueCredentialsAndIPRulesNeedAnAdmin(t *testing.T) {
	verbs := []struct {
		name string
		run  func(*Service, context.Context) error
	}{
		{"connection info", func(s *Service, ctx context.Context) error {
			_, err := s.KeyValueConnectionInfo(ctx, "red-cache")
			return err
		}},
		{"patch ipAllowList", func(s *Service, ctx context.Context) error {
			rules := slices.Clone(newRules)
			_, err := s.UpdateKeyValue(ctx, "red-cache", KeyValuePatch{IPAllowList: &rules})
			return err
		}},
		{"set ip allow list", func(s *Service, ctx context.Context) error {
			_, err := s.SetIPAllowList(ctx, "red-cache", slices.Clone(newRules))
			return err
		}},
		{"patch public off", func(s *Service, ctx context.Context) error {
			off := false
			_, err := s.UpdateKeyValue(ctx, "red-cache", KeyValuePatch{Public: &off})
			return err
		}},
		{"dry-run ipAllowList", func(s *Service, ctx context.Context) error {
			rules := slices.Clone(newRules)
			_, err := s.UpdateKeyValueDryRun(ctx, "red-cache", KeyValuePatch{IPAllowList: &rules})
			return err
		}},
		{"mixed patch: rename + ipAllowList", func(s *Service, ctx context.Context) error {
			rules, name := slices.Clone(newRules), "cache-2"
			_, err := s.UpdateKeyValue(ctx, "red-cache", KeyValuePatch{Name: &name, IPAllowList: &rules})
			return err
		}},
	}
	roles := []struct {
		name  string
		authz core.Checker
		admin bool
	}{{"admin", allowAllRelations{}, true}, {"developer", denyWorkspaceManage{}, false}}
	for _, verb := range verbs {
		for _, role := range roles {
			for _, protected := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", verb.name, role.name, protected), func(t *testing.T) {
					svc, cl, _ := protectedReadyKeyValue(t, protected)
					svc.Authz = role.authz
					sink := &webhookAuditSink{}
					svc.Audit = sink

					err := verb.run(svc, ctxAsCaller())
					if role.admin || !protected {
						if err != nil {
							t.Fatalf("%s by an allowed caller: %v", verb.name, err)
						}
						return
					}
					if !adminOnlyRefusal(err) {
						t.Fatalf("%s by a Developer on a protected store = %v, want the coded admin-only refusal", verb.name, err)
					}
					if strings.Contains(err.Error(), "s3cret") {
						t.Errorf("the refusal leaked the password: %v", err)
					}
					if got := storedAllowList(t, cl); len(got) != 1 || got[0].CIDR != "203.0.113.0/24" {
						t.Errorf("a refused write changed the rules: %+v", got)
					}
					for _, ev := range sink.events {
						if ev.Verb == core.AuditVerbViewConnectionInfo {
							t.Errorf("a refused read recorded a credential view: %+v", ev)
						}
					}
				})
			}
		}
	}
}

// Re-sending the rules a read returned changes nothing, so it needs no Admin.
func TestProtectedKeyValueUnchangedRulesAreOpen(t *testing.T) {
	svc, cl, _ := protectedReadyKeyValue(t, true)
	svc.Authz = denyWorkspaceManage{}
	current, err := svc.GetIPAllowList(ctxAsCaller(), "red-cache")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetIPAllowList(ctxAsCaller(), "red-cache", current); err != nil {
		t.Fatalf("re-sending the stored rules: %v", err)
	}
	name := "cache-renamed"
	if _, err := svc.UpdateKeyValue(ctxAsCaller(), "red-cache", KeyValuePatch{Name: &name}); err != nil {
		t.Fatalf("a rename leaves the rules alone and stays open: %v", err)
	}
	if got := storedAllowList(t, cl); len(got) != 1 || got[0].Description != "office" {
		t.Fatalf("the rules moved: %+v", got)
	}
}

// A protection status that cannot be read refuses the credential read and the
// rule change rather than letting them through.
func TestProtectedKeyValueGatesFailClosed(t *testing.T) {
	svc, _, _ := protectedReadyKeyValue(t, true)
	svc.Authz = allowAllRelations{}
	svc.Protection = failingProtection{}
	if _, err := svc.KeyValueConnectionInfo(ctxAsCaller(), "red-cache"); err == nil {
		t.Fatal("connection info with an unreadable protection status succeeded")
	}
	if _, err := svc.SetIPAllowList(ctxAsCaller(), "red-cache", slices.Clone(newRules)); err == nil {
		t.Fatal("an IP rule change with an unreadable protection status succeeded")
	}
}

// The refusal reaches REST as a 403 with its code.
func TestProtectedKeyValueRefusalOverREST(t *testing.T) {
	svc, _, _ := protectedReadyKeyValue(t, true)
	svc.Authz = denyWorkspaceManage{}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/key-value/red-cache/connection-info", ""},
		{http.MethodPatch, "/v1/key-value/red-cache", `{"ipAllowList":[{"cidrBlock":"198.51.100.0/24","description":"vpn"}]}`},
	} {
		got := serveREST(svc, tc.method, tc.path, tc.body)
		if got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), core.ProtectedPlacementCode) ||
			strings.Contains(got.Body.String(), "s3cret") {
			t.Errorf("%s %s = %d %s, want 403 %s", tc.method, tc.path, got.Code, got.Body.String(), core.ProtectedPlacementCode)
		}
	}
}

// The projection says what the verbs do (w4/m594): for an Admin, a Developer
// and a Viewer, on a protected and an unprotected member, view_connection_info,
// update_ip_allow_list and delete (w4/724) read allowed exactly when the verb
// succeeds, a protected Developer's refusal is named, and a Viewer reads the
// role reason. Before w4/724 there was no delete row, and the dashboard gated
// Delete on can_create, which a Developer holds.
func TestActionCapabilities_CredentialsAndIPRulesAgreeWithTheVerbs(t *testing.T) {
	roles := []struct {
		name   string
		authz  core.Checker
		reason string // the projection's reason on a protected member, "" when allowed
	}{
		{"admin", allowAllRelations{}, ""},
		{"developer", denyWorkspaceManage{}, core.ReasonProtectedEnvironmentAdmin},
		{"viewer", viewerOnly{}, core.ReasonInsufficientPermission},
	}
	verbs := map[string]func(*Service, context.Context) error{
		core.ActionViewConnectionInfo: func(s *Service, ctx context.Context) error {
			_, err := s.KeyValueConnectionInfo(ctx, "red-cache")
			return err
		},
		core.ActionUpdateIPAllowList: func(s *Service, ctx context.Context) error {
			_, err := s.SetIPAllowList(ctx, "red-cache", slices.Clone(newRules))
			return err
		},
		core.ActionDelete: func(s *Service, ctx context.Context) error {
			return s.DeleteKeyValue(ctx, "red-cache")
		},
	}
	for _, role := range roles {
		for _, protected := range []bool{true, false} {
			for action, verb := range verbs {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", role.name, action, protected), func(t *testing.T) {
					svc, _, store := protectedReadyKeyValue(t, protected)
					svc.Authz = role.authz
					ctx := ctxAsCaller()
					acts, err := svc.ActionCapabilities(ctx, "red-cache")
					if err != nil {
						t.Fatal(err)
					}
					if role.name != "viewer" && store.calls != 1 {
						t.Errorf("projection read protection %d times, want once for every row", store.calls)
					}
					got := actionByID(t, acts, action)
					verbErr := verb(svc, ctx)
					if (got.Outcome == core.DecisionAllowed) != (verbErr == nil) {
						t.Fatalf("projection %+v and verb error %v disagree", got, verbErr)
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

// viewerOnly is a Viewer in the role ladder: it may view and nothing more.
type viewerOnly struct{}

func (viewerOnly) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation == core.RelCanView, nil
}
