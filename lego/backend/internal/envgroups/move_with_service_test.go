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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/m211: the dashboard moves a service and its linked group in one batched
// request. The service move writes the store first and the App's environment
// label follows on the reconciler's next pass, so the group's scope guard must
// read the store, not the label, or the group move is refused.
func TestGroupMoveFollowsTheStoredServiceEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stored    string
		wantError bool
	}{
		{"service already moved in the store", "env-beta", false},
		{"service still in the old environment", "env-alpha", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			web := sampleApp("web")
			web.Labels[core.LabelAppID] = "srv-web"
			web.Labels[core.LabelEnvironment] = "env-alpha"
			svc := newService(newFakeStore(), web)
			svc.EnvironmentWorkspace = func(context.Context, string) (string, error) { return core.DefaultTenant, nil }
			ctx := testCtx()
			g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", OwnerID: core.DefaultTenant, EnvironmentID: "env-alpha"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			// Linked while the service was in env-alpha (label and store agree).
			if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
				t.Fatalf("link: %v", err)
			}
			// The label still says env-alpha; the store holds tc.stored.
			svc.ServiceEnvironment = func(_ context.Context, serviceID string) (string, error) {
				if serviceID != "srv-web" {
					return "", core.ErrNotFound
				}
				return tc.stored, nil
			}
			err = svc.SetEnvironmentID(ctx, g.ID, "env-beta")
			if tc.wantError {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES" {
					t.Fatalf("move = %v, want ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("move = %v, want success: the store has the service in env-beta", err)
			}
		})
	}
}
