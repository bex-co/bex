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
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// staleAllowChecker models the codex round-8 #8 window: the cached path (Check)
// still answers a warm positive while the source of truth (CheckFresh) already
// says the membership is gone — a caller demoted or revoked on another replica
// inside PositiveTTL.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// codex round-8 #8: removing a member revokes their access irreversibly — a
// caller riding a stale can_manage positive must not drop one last member.
func TestRemoveFailsClosedOnFreshRevocation(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("bob", "developer")
	g := newFakeGranter()
	s := svc(st, g, nil, staleAllowChecker{})

	if err := s.Remove(ctxWith("admin-1"), "tea-1", "bob"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Remove on a stale positive: %v, want ErrForbidden", err)
	}
	if _, ok := st.members["bob"]; !ok {
		t.Error("denied Remove dropped the member row anyway")
	}
	if len(g.revoked) != 0 {
		t.Errorf("denied Remove revoked tuples anyway: %v", g.revoked)
	}
}
