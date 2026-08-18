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

package members

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestViewerCapabilitiesProjection is the w9/m84 gate: the projection reports
// exactly the relations the caller holds (each a Can-probe), and refuses a
// caller who cannot even view the workspace — the same 403 every other
// workspace-scoped verb returns. The dashboard reads this to disable controls,
// so a wrong boolean would either block an admin or leave a contributor a
// control that 403s on save.
func TestViewerCapabilitiesProjection(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("admin-1", "admin")
	st.seedMember("viewer-1", "viewer")

	t.Run("admin holds every capability", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "admin"})
		caps, err := s.Capabilities(ctxWith("admin-1"), "tea-1")
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		for name, got := range map[string]bool{
			"canView":          caps.CanView,
			"canViewLogs":      caps.CanViewLogs,
			"canOperate":       caps.CanOperate,
			"canCreate":        caps.CanCreate,
			"canViewSensitive": caps.CanViewSensitive,
			"canManageKeys":    caps.CanManageKeys,
			"canManage":        caps.CanManage,
			"canManageBilling": caps.CanManageBilling,
		} {
			if !got {
				t.Errorf("admin %s = false, want true", name)
			}
		}
	})

	t.Run("viewer holds only can_view", func(t *testing.T) {
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
		caps, err := s.Capabilities(ctxWith("viewer-1"), "tea-1")
		if err != nil {
			t.Fatalf("Capabilities: %v", err)
		}
		if !caps.CanView {
			t.Errorf("viewer canView = false, want true")
		}
		// The write/reveal relations a contributor+ role would hold must all read
		// false — this is exactly what disables the create/reveal/manage controls.
		for name, got := range map[string]bool{
			"canOperate":       caps.CanOperate,
			"canCreate":        caps.CanCreate,
			"canViewSensitive": caps.CanViewSensitive,
			"canManageKeys":    caps.CanManageKeys,
			"canManage":        caps.CanManage,
			"canManageBilling": caps.CanManageBilling,
		} {
			if got {
				t.Errorf("viewer %s = true, want false", name)
			}
		}
	})

	t.Run("a non-member is refused, never served a projection", func(t *testing.T) {
		// A checker that grants nothing (not even can_view) models a caller with no
		// membership of the workspace — the cross-workspace case.
		s := svc(st, newFakeGranter(), nil, roleChecker{relation: "none"})
		_, err := s.Capabilities(ctxWith("stranger-1"), "tea-1")
		if !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Capabilities for a non-member = %v, want ErrForbidden", err)
		}
	})
}
