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

package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type protectionStub struct {
	protected bool
	err       error
}

func (p protectionStub) AppProtected(context.Context, *appv1alpha1.App) (bool, error) {
	return p.protected, p.err
}

// denyWorkspaceManage allows every relation except the admin can_manage on a
// workspace — a Developer in the role ladder.
type denyWorkspaceManage struct{}

func (denyWorkspaceManage) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == core.RelCanManage && strings.HasPrefix(object, "workspace:")), nil
}

type allowAllRelations struct{}

func (allowAllRelations) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// w8/193: Render restricts viewing and modifying a protected environment's
// environment variables and secret files to Admins.
func TestProtectedEnvironmentRestrictsVariablesAndFilesToAdmins(t *testing.T) {
	newStore := func() *fakeSecretStore {
		store := newFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"KEY": "v"}
		store.m[filesPath("web")] = map[string]string{"f": "x"}
		return store
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})

	calls := map[string]func(svc *Service) error{
		"list env vars": func(svc *Service) error { _, err := svc.ListEnvVars(ctx, "web"); return err },
		"get env var":   func(svc *Service) error { _, err := svc.GetEnvVar(ctx, "web", "KEY"); return err },
		"set env var": func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "web", "K2", EnvVarWrite{Value: "v"})
			return err
		},
		"delete env var":     func(svc *Service) error { return svc.DeleteEnvVar(ctx, "web", "KEY") },
		"list secret files":  func(svc *Service) error { _, err := svc.ListSecretFiles(ctx, "web"); return err },
		"get secret file":    func(svc *Service) error { _, err := svc.GetSecretFile(ctx, "web", "f"); return err },
		"set secret file":    func(svc *Service) error { _, err := svc.SetSecretFile(ctx, "web", "g", "y"); return err },
		"delete secret file": func(svc *Service) error { return svc.DeleteSecretFile(ctx, "web", "f") },
		"patch environment": func(svc *Service) error {
			_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeOnly, EnvVars: []EnvVarPatch{{Key: "K3", Value: "v"}}})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			svc := newService(newStore(), sampleApp("web"))
			svc.Protection = protectionStub{protected: true}
			svc.Authz = denyWorkspaceManage{}
			if err := call(svc); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("by a non-admin in a protected environment = %v, want ErrForbidden", err)
			}
			svc.Authz = allowAllRelations{}
			if err := call(svc); err != nil {
				t.Fatalf("by an admin in a protected environment: %v", err)
			}
		})
	}

	t.Run("an unprotected environment is open to a non-admin", func(t *testing.T) {
		svc := newService(newStore(), sampleApp("web"))
		svc.Protection = protectionStub{protected: false}
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.ListEnvVars(ctx, "web"); err != nil {
			t.Fatalf("unprotected list: %v", err)
		}
	})

	t.Run("a protection lookup that fails refuses", func(t *testing.T) {
		svc := newService(newStore(), sampleApp("web"))
		svc.Protection = protectionStub{err: errors.New("store down")}
		svc.Authz = allowAllRelations{}
		if _, err := svc.ListEnvVars(ctx, "web"); err == nil {
			t.Fatal("list with an unreadable protection status succeeded")
		}
	})
}
