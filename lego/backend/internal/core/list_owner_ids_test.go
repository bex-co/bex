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

package core_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// w8/m149: an unfiltered Render list spans every workspace the caller belongs
// to; a named ownerId narrows it; a caller with no membership of its own keeps
// the single-workspace default.
func TestListOwnerIDsSpansTheCallersWorkspaces(t *testing.T) {
	human := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	members := func(_ context.Context, identity core.Identity) ([]string, error) {
		if identity.Subject == "dana" {
			return []string{"tea-a", "tea-b"}, nil
		}
		return nil, nil
	}
	base := &core.Base{Workspace: coretest.Workspaces{"tea-a", "tea-b"}, MemberWorkspaceIDs: members}

	for _, tc := range []struct {
		name   string
		base   *core.Base
		ctx    context.Context
		owners []string
		want   []string
	}{
		{"omitted spans the caller's workspaces", base, human, nil, []string{"tea-a", "tea-b"}},
		{"named owners narrow", base, human, []string{"tea-b"}, []string{"tea-b"}},
		{"a machine identity keeps the default", base, core.WithIdentity(context.Background(), core.Identity{Subject: "cli-key"}), nil, nil},
		{"an unidentified caller keeps the default", base, context.Background(), nil, nil},
		{"store-off keeps the default", &core.Base{MemberWorkspaceIDs: members}, human, nil, nil},
	} {
		got, err := tc.base.ListOwnerIDs(tc.ctx, tc.owners)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: ListOwnerIDs = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}

	down := &core.Base{Workspace: coretest.Workspaces{"tea-a"}, MemberWorkspaceIDs: func(context.Context, core.Identity) ([]string, error) {
		return nil, errors.New("store down")
	}}
	if _, err := down.ListOwnerIDs(human, nil); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Errorf("membership lookup failure = %v, want ErrAuthzUnavailable (never a silent single-workspace answer)", err)
	}
}

// An unfiltered list spans the caller's memberships and skips one it cannot
// read; a named owner the caller cannot read still refuses the list.
func TestListForCallerSkipsUnreadableMemberWorkspaces(t *testing.T) {
	human := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	base := &core.Base{Workspace: coretest.Workspaces{"tea-a", "tea-b"},
		MemberWorkspaceIDs: func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }}
	list := func(_ context.Context, owner string) ([]string, error) {
		if owner == "tea-b" {
			return nil, core.ErrForbidden
		}
		return []string{owner + "/item"}, nil
	}
	if got, err := core.ListForCaller(human, base, nil, list); err != nil || !slices.Equal(got, []string{"tea-a/item"}) {
		t.Errorf("unfiltered = %v, %v; want tea-a's items with tea-b skipped", got, err)
	}
	if _, err := core.ListForCaller(human, base, []string{"tea-b"}, list); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("named unreadable owner = %v, want ErrForbidden", err)
	}
	if got, err := core.ListForCaller(context.Background(), base, nil, list); err != nil || !slices.Equal(got, []string{"/item"}) {
		t.Errorf("no identity = %v, %v; want the single default list", got, err)
	}
}
