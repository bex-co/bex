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

package notifications

import "github.com/bex-co/bex/lego/backend/internal/core"

// destination_policy.go is the ONE destination-eligibility policy (ADR087,
// w6/m137): a notification preference expresses interest, not permission, so
// an item must not be inaccessible in its destination surface yet visible as
// a title, badge count, or inbox row. Everything derives from one table —
// destinationRequiredRelation — applied at push projection (fanOutPush),
// immediately before delivery (sendDelivery, via the claim query's own
// membership join), and at inbox reads (inboxExcludedEventTypes). Recipients
// are re-read from CURRENT membership at each stage, never copied from the
// event, and an authorization outage defers or suppresses — it never widens.
//
// The delivery pipeline maps the required relation onto membership ROLES
// (role→relation grants are exactly model.fga's, pinned executable by
// api/roleladder_test.go) because the fan-out is an offline batch with no
// request identity; it only ever reaches tenant members, so org-admin
// inheritance cannot widen it. The inbox read side checks the caller's REAL
// relation (core.Base.Can — fail-closed on outage) instead.

// resourceKindAgentSession is the stored ResourceKind the agent-session
// projection stamps (push_worker.go) and deep links route by.
const resourceKindAgentSession = "agentSession"

// destinationRequiredRelation maps an event family to the relation its
// destination read/decision requires: "" for the all-roles families
// (service/deploy/cron supervision rides can_view, which every role holds —
// so does managed-datastore supervision, whose destination read is the
// datastore's OWN can_view, `can_view from workspace` on type postgres in
// model.fga, not the App's), can_operate for agent session reads, can_create
// for a decision request —
// approving/steering asks for work only create-holders may dispatch (the
// event constant exists even though no producer ships yet).
//
// A NEW event must be classified: TestEveryDeliveryEventHasDestinationPolicy
// ranges the closed vocabulary (orderedDeliveryEvents) so an unclassified
// addition turns CI red instead of silently fanning out to every role.
func destinationRequiredRelation(event DeliveryEvent) string {
	switch event {
	case DeliveryEventAgentPRReady, DeliveryEventAgentFailed:
		return core.RelCanOperate
	case DeliveryEventAgentNeedsDecision:
		return core.RelCanCreate
	default:
		return ""
	}
}

// destinationEligibleRoles is destinationRequiredRelation in role form, for
// the delivery pipeline's role-based evaluation. nil means every member role.
func destinationEligibleRoles(event DeliveryEvent) []DeliveryWorkspaceRole {
	switch destinationRequiredRelation(event) {
	case core.RelCanOperate:
		return []DeliveryWorkspaceRole{DeliveryRoleContributor, DeliveryRoleDeveloper, DeliveryRoleAdmin}
	case core.RelCanCreate:
		return []DeliveryWorkspaceRole{DeliveryRoleDeveloper, DeliveryRoleAdmin}
	default:
		return nil
	}
}

// inboxExcludedEventTypes derives the caller's inbox exclusion list from the
// SAME relation table: every event whose required relation the caller does
// not currently hold is filtered — in SQL, so list pages and unread counts
// agree by construction and a downgrade removes historic items from the
// visible projection without touching the durable rows. can is probed per
// distinct relation (at most two) and must fail closed.
func inboxExcludedEventTypes(can func(relation string) bool) []string {
	var out []string
	held := map[string]bool{}
	for _, event := range orderedDeliveryEvents {
		relation := destinationRequiredRelation(event)
		if relation == "" {
			continue
		}
		if _, probed := held[relation]; !probed {
			held[relation] = can(relation)
		}
		if !held[relation] {
			out = append(out, string(event))
		}
	}
	return out
}
