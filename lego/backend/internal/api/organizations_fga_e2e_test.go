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

package api

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestOrganizationModelOnRealOpenFGA holds deploy/gitops/authz/model.json to
// Render's org roles (w7/m464, docs/ADR115-organizations.md D5) on a real
// OpenFGA: an Owner manages the org and may join any of its workspaces as
// admin, but is no workspace admin until they join ("Org Owners are not
// automatically added to any workspaces", render.com/docs/organizations); a
// Member may join a Public workspace and not an Invite-only one; a Guest has
// no org-wide relation beyond reading the org; and taking a workspace's
// public link away takes the join with it.
func TestOrganizationModelOnRealOpenFGA(t *testing.T) {
	fgaURL := os.Getenv("BEX_TEST_OPENFGA_URL")
	if fgaURL == "" {
		testenv.Skip(t, "BEX_TEST_OPENFGA_URL not set")
	}
	ctx := context.Background()
	checker := authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_TEST_OPENFGA_TOKEN"))
	writer, ok := checker.(core.OrganizationTupleWriter)
	if !ok {
		t.Fatal("the OpenFGA checker is no OrganizationTupleWriter")
	}
	fresh := checker.(core.FreshChecker)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	org := "org-fga" + stamp
	public, private := "tea-pub"+stamp, "tea-priv"+stamp
	owner, member, guest, outsider := "owner"+stamp, "member"+stamp, "guest"+stamp, "outsider"+stamp

	tuples := []core.Tuple{
		core.OrgRoleTuple(org, owner, "owner"),
		core.OrgRoleTuple(org, member, "member"),
		core.OrgRoleTuple(org, guest, "guest"),
		core.OrgWorkspaceTuple(org, public),
		core.OrgWorkspaceTuple(org, private),
		core.OrgPublicTuple(org, public),
	}
	if err := writer.WriteOrganizationTuples(ctx, tuples, nil); err != nil {
		t.Fatalf("write org tuples: %v", err)
	}
	// Idempotent: writing them again is no error (the startup reconcile does).
	if err := writer.WriteOrganizationTuples(ctx, tuples, nil); err != nil {
		t.Fatalf("rewrite org tuples: %v", err)
	}
	t.Cleanup(func() { _ = writer.WriteOrganizationTuples(context.Background(), nil, tuples) })

	check := func(subject, relation, object string, want bool) {
		t.Helper()
		got, err := fresh.CheckFresh(ctx, "user:"+subject, relation, object)
		if err != nil {
			t.Fatalf("check %s %s %s: %v", subject, relation, object, err)
		}
		if got != want {
			t.Errorf("%s %s %s = %v, want %v", subject, relation, object, got, want)
		}
	}
	orgObj := core.OrganizationObject(org)
	for _, c := range []struct {
		subject, relation, object string
		want                      bool
	}{
		// The org's own relations.
		{owner, core.RelCanManage, orgObj, true},
		{owner, core.RelCanCreate, orgObj, true},
		{owner, core.RelCanView, orgObj, true},
		{member, core.RelCanManage, orgObj, false},
		{member, core.RelCanCreate, orgObj, false},
		{member, core.RelCanView, orgObj, true},
		{guest, core.RelCanManage, orgObj, false},
		{guest, core.RelCanCreate, orgObj, false},
		{guest, core.RelCanView, orgObj, true},
		{outsider, core.RelCanView, orgObj, false},
		// An Owner is no workspace admin until they join.
		{owner, core.RelCanManage, core.WorkspaceObject(public), false},
		{owner, core.RelCanView, core.WorkspaceObject(private), false},
		{owner, core.RelCanJoinAsAdmin, core.WorkspaceObject(public), true},
		{owner, core.RelCanJoinAsAdmin, core.WorkspaceObject(private), true},
		// A Member joins a Public workspace, not an Invite-only one, and is
		// never offered admin.
		{member, core.RelCanJoin, core.WorkspaceObject(public), true},
		{member, core.RelCanJoin, core.WorkspaceObject(private), false},
		{member, core.RelCanJoinAsAdmin, core.WorkspaceObject(public), false},
		// A Guest joins nothing.
		{guest, core.RelCanJoin, core.WorkspaceObject(public), false},
		{guest, core.RelCanJoinAsAdmin, core.WorkspaceObject(public), false},
		{outsider, core.RelCanJoin, core.WorkspaceObject(public), false},
	} {
		check(c.subject, c.relation, c.object, c.want)
	}

	// Closing the workspace takes the Member's join away.
	if err := writer.WriteOrganizationTuples(ctx, nil, []core.Tuple{core.OrgPublicTuple(org, public)}); err != nil {
		t.Fatal(err)
	}
	check(member, core.RelCanJoin, core.WorkspaceObject(public), false)
	// Removing the Owner's role takes the org and the join-as-admin away.
	if err := writer.WriteOrganizationTuples(ctx, nil, []core.Tuple{core.OrgRoleTuple(org, owner, "owner")}); err != nil {
		t.Fatal(err)
	}
	check(owner, core.RelCanManage, orgObj, false)
	check(owner, core.RelCanJoinAsAdmin, core.WorkspaceObject(private), false)
}
