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

package workspaces

import "strings"

// render.go maps OwnerView/MemberView onto Render's public-API "owner"/
// "teamMember" shapes, pinned verbatim from the OpenAPI spec in
// docs/render-artifacts/owners-api.md (w6/m2/t001). A client written for
// Render's /v1/owners sees the field names/envelopes it expects.

// renderOwnerType is the only owner type bex reports: every bex workspace has
// tenant_members (m1's create is the only writer, and w4/m12 grows real
// membership), the Render analogue of a "team" owner. bex has no equivalent of
// Render's personal (type=user) owner, so twoFactorAuthEnabled — documented as
// "only present if type is user" — is never emitted.
const renderOwnerType = "team"

// renderOwner mirrors components.schemas.owner (the fields bex has a real
// equivalent for). ipAllowList has no bex equivalent (omitted, not faked).
type renderOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Type  string `json:"type"`
}

// ownerWithCursor mirrors components.schemas.ownerWithCursor — the list-item
// envelope GET /v1/owners returns as a bare JSON array (cursor is a SIBLING of
// the owner object, not a wrapper member; docs/render-artifacts/owners-api.md
// verified this against Render's pagination.md).
type ownerWithCursor struct {
	Owner  renderOwner `json:"owner"`
	Cursor string      `json:"cursor"`
}

// renderUser mirrors components.schemas.user — GET /v1/users' response shape.
type renderUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func toRenderUser(u UserView) renderUser {
	return renderUser{Email: u.Email, Name: u.Name}
}

func toRenderOwner(o OwnerView) renderOwner {
	return renderOwner{ID: o.ID, Name: o.Name, Email: o.Email, Type: renderOwnerType}
}

func toOwnerList(os []OwnerView) []ownerWithCursor {
	out := make([]ownerWithCursor, 0, len(os))
	for _, o := range os {
		// cursor is opaque in Render; the workspace id is a stable, valid cursor
		// (same convention apps.toServiceList uses for services).
		out = append(out, ownerWithCursor{Owner: toRenderOwner(o), Cursor: o.ID})
	}
	return out
}

// renderTeamMemberRole is Render's uppercase role enum (teamMemberRole in the
// pinned spec); bex's lowercase OpenFGA relations (deploy/gitops/authz/model.fga)
// map onto it at this adapter boundary.
func renderTeamMemberRole(role string) string {
	switch role {
	case "contributor":
		return "WORKSPACE_CONTRIBUTOR"
	case "billing":
		return "WORKSPACE_BILLING"
	case "viewer":
		return "WORKSPACE_VIEWER"
	default:
		// "admin"/"developer" (whose Render spelling IS their uppercase form) and
		// any unknown role (a future model.fga addition bex hasn't mapped yet)
		// both pass through uppercased rather than dropping the member.
		return strings.ToUpper(role)
	}
}

// renderTeamMember mirrors components.schemas.teamMember. bex's Kratos schema
// defines only an email trait (no name), so Name is always "" — the key is
// still present (required in the pinned spec), an honest empty value rather
// than a faked one.
type renderTeamMember struct {
	UserID     string `json:"userId"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	Role       string `json:"role"`
	MFAEnabled bool   `json:"mfaEnabled"`
}

func toRenderTeamMember(m MemberView) renderTeamMember {
	return renderTeamMember{
		// Render's userId is an opaque own- id, not the raw Kratos subject (w6/m7).
		UserID: m.OwnerID,
		Email:  m.Email,
		// bex tracks no invited/deactivated member state yet — every row in
		// tenant_members is an active member.
		Status:     "active",
		Role:       renderTeamMemberRole(m.Role),
		MFAEnabled: m.MFAEnabled,
	}
}

func toRenderTeamMembers(ms []MemberView) []renderTeamMember {
	out := make([]renderTeamMember, 0, len(ms))
	for _, m := range ms {
		out = append(out, toRenderTeamMember(m))
	}
	return out
}
