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

package github

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func byIDSvc(st *fakeStore, workspaces ...string) *Service {
	return &Service{
		Base:   &core.Base{Namespace: "default", Workspace: coretest.Workspaces(workspaces)},
		GitHub: &fakeClient{login: "octo"},
		Store:  st,
	}
}

func seedSelection(st *fakeStore, id, workspaceID, subject string) {
	_ = st.CreateGitHubClaimSelection(context.Background(), store.GitHubClaimSelection{
		ID: id, WorkspaceID: workspaceID, Subject: subject,
		Candidates: []store.GitHubClaimCandidate{{InstallationID: 1, AccountLogin: "a"}, {InstallationID: 2, AccountLogin: "b"}},
		ExpiresAt:  time.Now().Add(time.Hour),
	})
}

func pending(st *fakeStore, id string) bool {
	_, err := st.GetGitHubClaimSelection(context.Background(), id)
	return err == nil
}

// w4/m172: a claim begun in a non-default workspace is read and completed by id
// alone; a refused call never spends the selection.
func TestClaimSelectionResolvesItsOwnWorkspace(t *testing.T) {
	t.Run("member reads and selects a tea-b selection without ownerId", func(t *testing.T) {
		st := newFakeStore()
		seedSelection(st, "sel-b", "tea-b", testCallerSubject)
		svc := byIDSvc(st, "tea-a", "tea-b")
		if sel, err := svc.GetClaimSelection(testCallerCtx(), "", "sel-b"); err != nil || len(sel.Candidates) != 2 {
			t.Fatalf("GetClaimSelection = %+v, %v", sel, err)
		}
		conn, err := svc.SelectClaim(testCallerCtx(), "", "sel-b", 2)
		if err != nil || conn.InstallationID != 2 {
			t.Fatalf("SelectClaim = %+v, %v", conn, err)
		}
		if got := st.workspacesFor(2); !slices.Equal(got, []string{"tea-b"}) {
			t.Fatalf("bound in %v, want [tea-b]", got)
		}
		if pending(st, "sel-b") {
			t.Fatal("a successful select must consume the selection")
		}
	})
	t.Run("mismatched ownerId refuses without consuming", func(t *testing.T) {
		st := newFakeStore()
		seedSelection(st, "sel-b", "tea-b", testCallerSubject)
		svc := byIDSvc(st, "tea-a", "tea-b")
		if _, err := svc.SelectClaim(testCallerCtx(), "tea-a", "sel-b", 2); !errors.Is(err, errClaimSelectionGone) {
			t.Fatalf("mismatched select = %v, want errClaimSelectionGone", err)
		}
		if !pending(st, "sel-b") {
			t.Fatal("a refused select consumed the selection")
		}
		if _, err := svc.SelectClaim(testCallerCtx(), "tea-b", "sel-b", 1); err != nil {
			t.Fatalf("select with the right ownerId afterwards: %v", err)
		}
	})
	t.Run("out-of-set choice refuses without consuming", func(t *testing.T) {
		st := newFakeStore()
		seedSelection(st, "sel-b", "tea-b", testCallerSubject)
		svc := byIDSvc(st, "tea-a", "tea-b")
		if _, err := svc.SelectClaim(testCallerCtx(), "", "sel-b", 999); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("out-of-set select = %v, want ErrBadRequest", err)
		}
		if !pending(st, "sel-b") {
			t.Fatal("an out-of-set select consumed the selection")
		}
	})
	t.Run("foreign subject refuses without consuming", func(t *testing.T) {
		st := newFakeStore()
		seedSelection(st, "sel-b", "tea-b", "someone-else")
		svc := byIDSvc(st, "tea-a", "tea-b")
		if _, err := svc.SelectClaim(testCallerCtx(), "", "sel-b", 2); !errors.Is(err, errClaimSelectionGone) {
			t.Fatalf("foreign-subject select = %v, want errClaimSelectionGone", err)
		}
		if !pending(st, "sel-b") {
			t.Fatal("a foreign-subject select consumed the selection")
		}
	})
	t.Run("non-member gets the unknown-id refusal and consumes nothing", func(t *testing.T) {
		st := newFakeStore()
		// The caller started it, then lost tea-b membership.
		seedSelection(st, "sel-b", "tea-b", testCallerSubject)
		svc := byIDSvc(st, "tea-a")
		_, missing := svc.GetClaimSelection(testCallerCtx(), "", "sel-missing")
		_, foreign := svc.GetClaimSelection(testCallerCtx(), "", "sel-b")
		if !errors.Is(missing, errClaimSelectionGone) || !errors.Is(foreign, errClaimSelectionGone) {
			t.Fatalf("non-member read %v vs missing %v, want identical gone", foreign, missing)
		}
		if _, err := svc.SelectClaim(testCallerCtx(), "", "sel-b", 2); !errors.Is(err, errClaimSelectionGone) {
			t.Fatalf("non-member select = %v, want errClaimSelectionGone", err)
		}
		if !pending(st, "sel-b") || len(st.conns) != 0 {
			t.Fatal("a non-member select consumed or bound something")
		}
	})
}

// w4/m172: Disconnect by installation id acts where that connection lives.
func TestDisconnectResolvesTheConnectionsOwnWorkspace(t *testing.T) {
	bind := func(st *fakeStore, workspaceID string, installationID int64) {
		_, _ = st.BindGitConnection(context.Background(), store.GitConnection{WorkspaceID: workspaceID, InstallationID: installationID, AccountLogin: "a"}, 0, 0)
	}
	t.Run("member disconnects a tea-b connection without ownerId", func(t *testing.T) {
		st := newFakeStore()
		bind(st, "tea-b", 7)
		if err := byIDSvc(st, "tea-a", "tea-b").Disconnect(testCallerCtx(), "", 7); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
		if len(st.conns) != 0 {
			t.Fatalf("connection still stored: %+v", st.conns)
		}
	})
	t.Run("default-workspace binding wins when the installation is in both", func(t *testing.T) {
		st := newFakeStore()
		bind(st, "tea-b", 7)
		bind(st, "tea-a", 7)
		if err := byIDSvc(st, "tea-a", "tea-b").Disconnect(testCallerCtx(), "", 7); err != nil {
			t.Fatalf("Disconnect: %v", err)
		}
		if got := st.workspacesFor(7); !slices.Equal(got, []string{"tea-b"}) {
			t.Fatalf("remaining bindings %v, want [tea-b]", got)
		}
	})
	t.Run("several non-default bindings are ambiguous", func(t *testing.T) {
		st := newFakeStore()
		bind(st, "tea-b", 7)
		bind(st, "tea-c", 7)
		svc := byIDSvc(st, "tea-a", "tea-b", "tea-c")
		if err := svc.Disconnect(testCallerCtx(), "", 7); !errors.Is(err, core.ErrConflict) {
			t.Fatalf("ambiguous Disconnect = %v, want ErrConflict", err)
		}
		if err := svc.Disconnect(testCallerCtx(), "tea-c", 7); err != nil {
			t.Fatalf("Disconnect with ownerId: %v", err)
		}
		if got := st.workspacesFor(7); !slices.Equal(got, []string{"tea-b"}) {
			t.Fatalf("remaining bindings %v, want [tea-b]", got)
		}
	})
	t.Run("non-member gets the unknown-id answer and deletes nothing", func(t *testing.T) {
		st := newFakeStore()
		bind(st, "tea-b", 7)
		svc := byIDSvc(st, "tea-a")
		missing := svc.Disconnect(testCallerCtx(), "", 404)
		foreign := svc.Disconnect(testCallerCtx(), "", 7)
		if missing != nil || foreign != nil {
			t.Fatalf("non-member %v vs missing %v, want the same idempotent no-op", foreign, missing)
		}
		if got := st.workspacesFor(7); !slices.Equal(got, []string{"tea-b"}) {
			t.Fatalf("a non-member deleted a binding: %v", got)
		}
	})
}
