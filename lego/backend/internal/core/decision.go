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

package core

import (
	"context"
	"errors"
)

// The bounded tri-state outcome vocabulary for a capability probe (ADR087,
// w6/m136). A boolean Can collapses "the caller may not" and "the check could
// not run" into one false; the capability projection clients gate whole
// screens on needs the difference — a checker outage must read as a neutral
// "couldn't check", never as "your role forbids this". Both non-allowed
// outcomes gate identically (fail closed); only the explanation differs.
const (
	DecisionAllowed     = "allowed"
	DecisionDenied      = "denied"
	DecisionUnavailable = "unavailable"
)

// The bounded, non-sensitive reason codes a Decision may carry. Empty when
// allowed. Clients must treat an unknown reason exactly like an unknown
// outcome: not-allowed, generic copy.
const (
	ReasonMissingOAuthScope      = "missing_oauth_scope"     // the token's granted scopes lack the relation's capability
	ReasonInsufficientPermission = "insufficient_permission" // an affirmative refusal: membership/role/grant is insufficient
	ReasonAuthzUnavailable       = "authz_unavailable"       // the question could not be answered; not a permission verdict
)

// Decision is one capability probe's tri-state result.
type Decision struct {
	Outcome string
	Reason  string // empty when allowed
}

// Allowed reports the single affirmative outcome; every other outcome —
// denied, unavailable, or anything unrecognized — is not-allowed.
func (d Decision) Allowed() bool { return d.Outcome == DecisionAllowed }

// ClassifyDecision maps a shared-seam authorization error onto the bounded
// vocabulary. Precedence matters: the insufficient-scope CodedError WRAPS
// ErrForbidden (so transports stay on 403), which is why the scope test runs
// before the plain-forbidden one. An unrecognized error reads as unavailable —
// fail closed without inventing a permission verdict the checker never
// produced. The legacy boolean surfaces stay restrictive-but-reason-unknown;
// this is the seam that preserves the reason for callers that ask.
func ClassifyDecision(err error) Decision {
	var coded *CodedError
	switch {
	case err == nil:
		return Decision{Outcome: DecisionAllowed}
	case errors.As(err, &coded) && coded.Code == InsufficientScopeCode:
		return Decision{Outcome: DecisionDenied, Reason: ReasonMissingOAuthScope}
	case errors.Is(err, ErrAuthzUnavailable):
		return Decision{Outcome: DecisionUnavailable, Reason: ReasonAuthzUnavailable}
	case errors.Is(err, ErrForbidden):
		return Decision{Outcome: DecisionDenied, Reason: ReasonInsufficientPermission}
	default:
		return Decision{Outcome: DecisionUnavailable, Reason: ReasonAuthzUnavailable}
	}
}

// CanDecision is Can with the outcome preserved: the same non-auditing,
// response-shaping probe of relation on the caller's acting workspace, but
// distinguishing an affirmative refusal (denied + why) from an unanswerable
// check (unavailable). Never a verb gate — a verb still opens with
// Authorize/AuthorizeApp — and never permissive: every non-nil path is
// not-allowed, exactly like Can.
func (b *Base) CanDecision(ctx context.Context, relation string) Decision {
	object, err := b.callerWorkspace(ctx)
	if err != nil {
		return ClassifyDecision(err)
	}
	return ClassifyDecision(b.checkAuthz(ctx, relation, object))
}

// A fresh (cache-bypassing) probe is spelled
// ClassifyDecision(AuthorizeFreshOn(ctx, relation, object)) — the members
// Capabilities verb's recovery path — rather than a dedicated seam: the fresh
// answer is authoritative only for THIS replica; other replicas retain their
// positives until expiry, and membership→OpenFGA reconciliation adds its own
// boundary (see the Capabilities contract doc, ADR087).

// CanDecisionOn is CanDecision against an explicit OpenFGA object — the form a
// resource-scoped capability projection uses after it has fetched and bound
// the exact target, probing each action's relation against the RESOURCE'S own
// workspace rather than the caller's default (the w6/m17 lesson applied to
// probes). Same non-auditing, fail-closed contract as CanDecision.
func (b *Base) CanDecisionOn(ctx context.Context, relation, object string) Decision {
	return ClassifyDecision(b.checkAuthz(ctx, relation, object))
}

// The bounded action-id vocabulary for resource-scoped capability projections
// (ADR087, w6/m136/t004) — the verbs the mobile matrix gates. A projection
// omits an action that does not exist for the resource's type at all (a
// non-cron service has no cron_run_now row); clients treat an unknown action
// id as absent. These are projection ids, not new verbs: each maps onto an
// existing execution path whose own guards still run at dispatch.
const (
	ActionRestart       = "restart"
	ActionSuspend       = "suspend"
	ActionResume        = "resume"
	ActionDeploy        = "deploy" // bare redeploy — no executable selection
	ActionCancelDeploy  = "cancel_deploy"
	ActionRollback      = "rollback"
	ActionCronRunNow    = "cron_run_now"
	ActionCronCancelRun = "cron_cancel_run"
)

// The bounded precondition vocabulary: why a PERMITTED action cannot execute
// right now. Empty means ready. Preconditions explain readiness — they never
// manufacture authorization, and an unknown one reads as blocked on every
// client (fail closed).
const (
	// The target belongs to a protected Environment; the verb requires the
	// exact server-issued confirmation phrase through the safe-action flow.
	PrecondProtectedConfirmation = "protected_confirmation_required"
	// The resource's current state does not admit the verb (e.g. suspended).
	PrecondSuspended = "suspended"
	// Nothing for the verb to act on right now.
	PrecondNoActiveDeploy           = "no_active_deploy"
	PrecondNoActiveRun              = "no_active_run"
	PrecondNoEligibleRollbackTarget = "no_eligible_rollback_target"
	// Billing enforcement (dunning) currently refuses billable mutations.
	PrecondBillingBlocked = "billing_blocked"
	// The precondition itself could not be computed — blocked, not waived.
	PrecondUnavailable = "unavailable"
)

// ActionDecision is one resource verb's projected decision: the tri-state
// permission plus, when permitted, the bounded precondition that currently
// blocks execution. The execute path still runs its own guards — a projection
// snapshot is never a bearer grant.
type ActionDecision struct {
	Action       string `json:"action"`
	Outcome      string `json:"outcome"`
	Reason       string `json:"reason,omitempty"`
	Precondition string `json:"precondition,omitempty"`
}

// DecideAction composes a permission decision with its precondition. A
// not-allowed decision carries no precondition: readiness is meaningless (and
// mildly disclosive) for an action the caller may not perform.
func DecideAction(action string, d Decision, precondition string) ActionDecision {
	ad := ActionDecision{Action: action, Outcome: d.Outcome, Reason: d.Reason}
	if d.Allowed() {
		ad.Precondition = precondition
	}
	return ad
}

// BillingPrecondition classifies a RequireBillingMutation outcome for a
// capability projection: nil is ready, the dunning refusal is billing_blocked,
// and anything else is unavailable — blocked, never silently waived.
func BillingPrecondition(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrBillingEnforced):
		return PrecondBillingBlocked
	default:
		return PrecondUnavailable
	}
}

// BillingPreconditionFor is the composed form every projection uses: the
// billing gate for tenantID, classified as a precondition.
func (b *Base) BillingPreconditionFor(ctx context.Context, tenantID string) string {
	return BillingPrecondition(b.RequireBillingMutation(ctx, tenantID))
}

// RequireResourceInActingWorkspace binds a fetched resource to the workspace
// this request is ACTING in (ADR087 explicit context, w6/m136/t003): a
// capability decision is asked about "this resource in this workspace", so a
// target belonging to another workspace — even one the caller could reach
// there through the verbs' own cross-workspace fallback — reads as absent
// here, never as an answer about the other workspace. That stops a client
// from rendering workspace A's screen with workspace B's resource, and stops
// the response from distinguishing a foreign target from a nonexistent one.
// An unlabeled CR (hand-applied, store-off) belongs to the default workspace.
func (b *Base) RequireResourceInActingWorkspace(ctx context.Context, labels map[string]string) error {
	owner := labels[LabelTenant]
	if owner == "" {
		owner = DefaultTenant
	}
	if owner != b.WorkspaceOrDefault(ctx) {
		return ErrNotFound
	}
	return nil
}
