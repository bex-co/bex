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

package audit

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// organizations.go is the org audit log (w7/m464, docs/ADR115-organizations.md
// D7): Render's GET /v1/organizations/{orgId}/audit-logs. Org events are rows
// whose resource is the org object, so they are keyed by the org id and the
// org log and the workspace logs never mix. Render: "Only org members with
// the Org Owner role can export these audit logs", and "Retention does not
// begin until you upgrade" (render.com/docs/audit-logs).

// organizationAuditStore is the org slice the log reads; *store.PGStore
// satisfies it.
type organizationAuditStore interface {
	OrganizationRole(ctx context.Context, orgID, subject string) (store.Organization, string, error)
}

// ListOrganization returns an org's audit trail, newest first unless
// filter.Direction is forward. Only its Owners read it; an org that does not
// exist, and one the caller is not in, answer the same 403
// ORGANIZATION_NOT_FOUND, so an org id reveals nothing. History starts when
// the org was created.
func (s *Service) ListOrganization(ctx context.Context, orgID string, filter Filter) ([]Event, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return nil, err
	}
	orgs, ok := s.Store.(organizationAuditStore)
	if !ok {
		return nil, core.ErrAuditUnavailable
	}
	id, _ := core.IdentityFrom(ctx)
	o, role, err := orgs.OrganizationRole(ctx, orgID, id.Subject)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, core.NewOrganizationNotFoundError(orgID)
	case err != nil:
		return nil, err
	case role != core.OrgRoleOwner:
		return nil, core.ErrForbidden
	}
	return s.list(ctx, orgID, o.AuditVisibleFrom, filter)
}
