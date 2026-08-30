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

// Package core is the leaf kernel every bex-api feature package imports: the
// shared Base (apiserver-thin client + namespace + clock + authorization gate),
// the caller Identity, the cross-feature error sentinels, and the HTTP/cache
// primitives the Ory-bound clients share. It imports the CRD types but no other
// bex package, and never the composition root — features depend on core, the
// root depends on features + core, so there is no import cycle.
package core

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

const CodeAccountDeletionPending = "ACCOUNT_DELETION_PENDING"

// The domain error sentinels every feature returns and the REST adapter maps to
// status codes (see WriteErr). Shared here so one WriteErr can map them all and
// the surfaces stay authorization/error identical.
var (
	// ErrNotFound is returned when a resource (App/Database) does not exist.
	ErrNotFound = errors.New("app not found")
	// ErrUnavailable is the class every "…Unavailable" sentinel below belongs to:
	// the verb exists but its backing dependency isn't wired. WriteErr maps the
	// class once (503) instead of naming each sentinel, so a feature declaring a
	// new one through Unavailable is mapped without editing this leaf package —
	// which is what let internal/projects and internal/environments delete their
	// package-local 503 shims. The members/jobs/blueprint sentinels are opted in
	// too: they answered 500 while api/server.go and jobs' own RegisterREST doc
	// promised 503, so the status change fixes a documented contract rather than
	// breaking one (see the REST tests in those packages).
	ErrUnavailable = errors.New("service unavailable")
	// ErrLogsUnavailable is returned by the logs verbs when no pod-log source is
	// wired (adapters surface it as 503, not 404 — the App exists, the source
	// doesn't).
	ErrLogsUnavailable = Unavailable("logs source not configured")
	// ErrLogStoreUnavailable is returned by the logs verbs when a caller asks for
	// something only the durable log store can answer — request logs, or a
	// structured filter (level/statusCode/method/path/host) — while bex-api runs
	// in pod-log fallback mode (BEX_LOKI_URL unset). Adapters surface it as 503:
	// refusing beats silently ignoring the filter and returning unfiltered lines
	// (docs/ADR010-observability.md § Log filters).
	ErrLogStoreUnavailable = Unavailable("request logs and structured log filters require the durable log store")
	// ErrMetricsUnavailable is returned by the metrics verbs when the backend a
	// metric needs isn't wired (adapters surface it as 503).
	ErrMetricsUnavailable = Unavailable("metrics source not configured")
	// ErrAPIKeysUnavailable is returned by the api-key verbs when no store is wired.
	ErrAPIKeysUnavailable = Unavailable("api-key store not configured")
	// ErrSSHKeysUnavailable is returned by the SSH-key verbs when the control-plane
	// store is not wired. SSH authentication cannot safely degrade to an in-memory
	// key registry because a restart would revoke access unpredictably.
	ErrSSHKeysUnavailable = Unavailable("ssh-key store not configured")
	// ErrSecretsUnavailable is returned by the env-vars verbs when no secret store
	// is wired (BEX_OPENBAO_URL unset); adapters surface it as 503.
	ErrSecretsUnavailable = Unavailable("secret store not configured")
	// ErrWorkspacesUnavailable is returned by the workspace verbs when the
	// control-plane store isn't wired (bex-api running without BEX_CP_DB_URI);
	// adapters surface it as 503 (the owners read API exists, the backing store
	// doesn't).
	ErrWorkspacesUnavailable = Unavailable("workspaces store not configured")
	// ErrDeploysUnavailable is returned by the deploy-history verbs when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503 — deploy history has no CR-only equivalent to fall back to.
	ErrDeploysUnavailable = Unavailable("deploy history store not configured")
	// ErrSandboxesUnavailable is returned by the sandbox verbs when the
	// OpenSandbox lifecycle client isn't wired (BEX_OPENSANDBOX_URL unset);
	// adapters surface it as 503 (pillar 5, ADR042/w3/m32).
	ErrSandboxesUnavailable = Unavailable("sandbox runtime not configured")
	// ErrAgentSessionsUnavailable is the stable, coded refusal when the agent-
	// session feature itself is not wired. Keep this sentinel as the legacy
	// errors.Is target, but do not use it for a configured dependency outage or
	// snapshot-store fault: callers need to tell operator action from retryable
	// failure without matching human copy (w4/m89).
	ErrAgentSessionsUnavailable = NewUnavailableError(
		AgentSessionNotConfiguredCode,
		"agent sessions are not configured",
		nil,
	)
	// ErrBadRequest is returned for invalid caller input (adapters map it to 400).
	ErrBadRequest = errors.New("bad request")
	// ErrForbidden is returned when the caller lacks the permission a verb requires
	// (adapters map it to 403; distinct from the auth gate's 401).
	ErrForbidden = errors.New("forbidden")
	// ErrConflict is returned when a verb refuses because of the resource's
	// current state (e.g. triggering a deploy on a suspended service); adapters
	// map it to 409.
	ErrConflict = errors.New("conflict")
	// ErrAuthzUnavailable is returned when a wired authorization checker cannot be
	// consulted — requests fail closed (503), never pass through.
	ErrAuthzUnavailable = Unavailable("authorization service unavailable")
	// ErrUsageUnavailable is returned by the usage verb when the store isn't
	// wired (BEX_CP_DB_URI unset); adapters surface it as 503.
	ErrUsageUnavailable = Unavailable("usage unavailable")
	// ErrBillingUnavailable is returned by the customer-billing onboarding
	// verbs when Stripe is disabled or cannot be reached. It is deliberately
	// distinct from ErrUsageUnavailable: advisory usage remains available while
	// hosted Checkout, Portal, and payment readiness fail closed.
	ErrBillingUnavailable = Unavailable("billing integration unavailable")
	// ErrBillingEnforced blocks new billable work and tenant-driven resumes
	// while the durable dunning lifecycle owns reversible suspension.
	ErrBillingEnforced = errors.New("workspace billing enforcement is active")
	// ErrPaymentRequired blocks a paid-tier create or plan change until the
	// workspace has completed hosted payment-method setup. It is distinct from
	// dunning enforcement: free-tier and non-plan mutations remain available.
	ErrPaymentRequired = errors.New("payment method required")
	// ErrAuditUnavailable is returned by the audit-log read verb when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503 — omitted, not faked (the deploy-history/env-vars precedent).
	ErrAuditUnavailable = Unavailable("audit log store not configured")
	// ErrGitHubUnavailable is returned by the git-connect verbs when the GitHub
	// App is not configured (BEX_GITHUB_APP_* unset) or the control-plane store
	// isn't wired (BEX_CP_DB_URI unset) — adapters surface it as 503
	// (docs/ADR026-github-integration.md).
	ErrGitHubUnavailable = Unavailable("github integration not configured")
	// ErrEventsUnavailable is returned by the service-events feed when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503. BOTH of the feed's sources (deploys, audit_events) are control-plane
	// tables, so there is no CR-only feed to degrade to — omitted, not faked
	// (w3/m7, the deploy-history precedent).
	ErrEventsUnavailable = Unavailable("events store not configured")
	// ErrNotificationsUnavailable is returned by the notification-settings verbs
	// when the control-plane store isn't wired (BEX_CP_DB_URI unset); adapters
	// surface it as 503 (w3/m9, the deploy-history precedent).
	ErrNotificationsUnavailable = Unavailable("notification settings store not configured")
	// ErrPushUnavailable is returned when a caller attempts to register a push
	// destination while the server transport is deliberately disabled. Reading
	// notification preferences and supervision remains available.
	ErrPushUnavailable = Unavailable("mobile push transport not configured")
	// ErrWebPushUnavailable is returned when a caller attempts to register a
	// browser push subscription while VAPID is unset. Native push and email
	// remain independently available.
	ErrWebPushUnavailable = Unavailable("browser web push transport not configured")
	// ErrRegistryCredentialsUnavailable is returned by the registry-credentials
	// verbs when the control-plane store (BEX_CP_DB_URI) or the secret store
	// (BEX_OPENBAO_URL) isn't wired — either is required, since a credential's
	// metadata lives in one and its secret in the other; adapters surface it
	// as 503.
	ErrRegistryCredentialsUnavailable = Unavailable("registry credential store not configured")
	// ErrWebhooksUnavailable is returned by the outbound-webhook verbs when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503 (w3/m11, the deploy-history precedent) — both the endpoint registry
	// and the delivery queue are control-plane tables, so there is nothing to
	// degrade to.
	ErrWebhooksUnavailable = Unavailable("webhook store not configured")
	// ErrLogoutUnavailable is returned by the CLI-logout revoke verb
	// (POST /v1/oauth/revoke) when the Hydra admin endpoint that clears a human's
	// consent chain is unwired (BEX_HYDRA_ADMIN_URL unset) or unreachable;
	// adapters surface it as 503. It exists so /v1/oauth/revoke — a Render-shaped
	// REST endpoint — speaks the one Render error dialect on every branch (w9/m38,
	// w9/008), unlike the RFC 8628 device endpoints whose OAuth-shaped bodies the
	// CLI parses as token responses.
	ErrLogoutUnavailable = Unavailable("logout revocation service unavailable")
	// ErrShellUnavailable is returned by the Web Shell ticket verb when the
	// browser-terminal transport is not configured (BEX_SHELL_TICKET_SECRET or
	// BEX_SHELL_WS_URL unset); adapters surface it as 503. Native `ssh` is
	// unaffected — the copy-ready command still works (docs/ADR035-ssh.md
	// § Browser Web Shell).
	ErrShellUnavailable = Unavailable("web shell transport not configured")
)

// unavailableErr carries its own message while reporting membership in the
// ErrUnavailable class. It is a pointer type so two sentinels that happen to
// share a message stay distinct under errors.Is, exactly like errors.New.
type unavailableErr struct{ msg string }

func (e *unavailableErr) Error() string { return e.msg }
func (e *unavailableErr) Unwrap() error { return ErrUnavailable }

// Unavailable declares a "dependency not wired" sentinel that WriteErr answers
// with 503. A feature package uses it for its own sentinel instead of a local
// response shim, since core (a leaf) cannot import the feature to name it.
func Unavailable(msg string) error { return &unavailableErr{msg: msg} }

// constErr is a comparable string error for fixed messages (config refusals,
// upstream "not found" summaries) — like the standard library's errors.New but
// usable as a package-level constant.
type constErr string

func (e constErr) Error() string { return string(e) }

// Err returns a comparable constant error carrying msg.
func Err(msg string) error { return constErr(msg) }

// CodedError is a domain error carrying a machine-readable Code and Params
// alongside its human-readable message. REST callers receive code+params in the
// JSON body alongside the error string; GraphQL callers receive them in the
// error's extensions field (graphql-go's gqlerrors.ExtendedError interface
// picks up Extensions() automatically). The wrapped sentinel keeps errors.Is
// checks working — e.g. errors.Is(err, ErrBadRequest) is true for a
// *CodedError that wraps ErrBadRequest.
type CodedError struct {
	Code     string
	Params   map[string]any
	sentinel error
	msg      string
}

func (e *CodedError) Error() string { return e.msg }
func (e *CodedError) Unwrap() error { return e.sentinel }

// Extensions satisfies gqlerrors.ExtendedError so the graphql-go formatter
// includes Code+Params in the GraphQL error's extensions field — the same
// mechanism the RATE_LIMITED envelope uses in ratelimit.go, now as a shared,
// reusable pattern rather than a one-off.
func (e *CodedError) Extensions() map[string]any {
	m := make(map[string]any, 1+len(e.Params))
	m["code"] = e.Code
	for k, v := range e.Params {
		m[k] = v
	}
	return m
}

// Compile-time assertion: *CodedError satisfies gqlerrors.ExtendedError.
var _ interface {
	error
	Extensions() map[string]any
} = (*CodedError)(nil)

// MCPError prefixes a *CodedError's stable Code onto the message an MCP tool
// returns. MCP has no structured error envelope like REST's JSON body or
// GraphQL's extensions, so the code has to travel in the text or an agent
// cannot tell a plan limit from a validation failure. A plain error passes
// through untouched.
//
// Applied once, by the shared tool seam (internal/mcputil.AddTool). It is
// idempotent so a handler that also wraps its own error cannot produce
// "CODE: CODE: msg".
func MCPError(err error) error {
	if err == nil {
		return nil
	}
	var coded *CodedError
	if errors.As(err, &coded) {
		if strings.HasPrefix(err.Error(), coded.Code+": ") {
			return err
		}
		return fmt.Errorf("%s: %w", coded.Code, err)
	}
	// Parity with WriteErr (REST) and the GraphQL sanitizer: an unclassified
	// internal failure must not spill its raw text (pgx/Kubernetes internals)
	// into the tool result. Log the real cause and return a generic error; every
	// sentinel-classified error keeps its message so the client still sees
	// "not found"/"forbidden"/etc.
	if !IsPublicError(err) {
		log.Printf("bex-api mcp: internal error: %v", err)
		return errors.New("internal error")
	}
	return err
}

// NewPlanLimitError returns a *CodedError for plan capacity and role
// restrictions. plan is the workspace's current plan name; limit is the
// per-plan maximum (member count for a seat cap, 0 for a role-gate refusal).
// Wraps ErrBadRequest so errors.Is(err, ErrBadRequest) holds and REST/GraphQL
// adapters map it to 400.
func NewPlanLimitError(msg, plan string, limit int) *CodedError {
	return &CodedError{
		Code:     "PLAN_LIMIT",
		Params:   map[string]any{"plan": plan, "limit": limit},
		sentinel: ErrBadRequest,
		msg:      msg,
	}
}

// NewBadRequestError returns a machine-readable 400 error for feature-specific
// validation failures. Params are included in REST's `params` object and
// GraphQL's `extensions`, while ErrBadRequest keeps the shared transport
// mapping intact.
func NewBadRequestError(code, msg string, params map[string]any) *CodedError {
	return &CodedError{Code: code, Params: params, sentinel: ErrBadRequest, msg: msg}
}

// NewForbiddenError returns a machine-readable 403 for a caller class or
// permission that cannot perform a feature-specific operation.
func NewForbiddenError(code, msg string, params map[string]any) *CodedError {
	return &CodedError{Code: code, Params: params, sentinel: ErrForbidden, msg: msg}
}

// NewNotFoundError returns a machine-readable 404 for a feature-specific
// resource lookup. It deliberately wraps the shared ErrNotFound sentinel so all
// transports keep the same status mapping while avoiding the App-specific
// sentinel text in another feature's response.
func NewNotFoundError(code, msg string, params map[string]any) *CodedError {
	return &CodedError{Code: code, Params: params, sentinel: ErrNotFound, msg: msg}
}

// NewConflictError returns a machine-readable 409 error for a valid operation
// that the resource's current state makes unsafe.
func NewConflictError(code, msg string, params map[string]any) *CodedError {
	return &CodedError{Code: code, Params: params, sentinel: ErrConflict, msg: msg}
}

// NewAccountDeletionPendingError keeps onboarding and credential writers on
// the same durable-tombstone wire contract.
func NewAccountDeletionPendingError() *CodedError {
	return NewConflictError(CodeAccountDeletionPending, "account deletion is in progress", nil)
}

// NewUnavailableError returns a machine-readable 503 for a verb whose backing
// dependency is not wired — the deployment never configured it, as opposed to
// it having failed.
//
// The distinction matters to whoever is reading the screen. "Not configured" is
// an operator's job and nothing the tenant can act on; a genuine outage is
// neither. Without a code a client can only match on message text, and a
// sanitized message ("internal error") tells the tenant their service is broken
// when nothing is (w1/m87/t005 — this is exactly what the Disk tab's Snapshots
// card did on production, 2026-08-24).
func NewUnavailableError(code, msg string, params map[string]any) *CodedError {
	return &CodedError{Code: code, Params: params, sentinel: ErrUnavailable, msg: msg}
}

// Agent-session availability codes deliberately distinguish a platform that
// was never configured from a configured dependency that is temporarily down,
// and from the optional snapshot tier being unavailable. All remain 503: a
// retry cannot repair missing operator configuration, but none is caller input
// or a resource-state conflict.
const (
	AgentSessionNotConfiguredCode         = "AGENT_SESSION_NOT_CONFIGURED"
	AgentSessionDependencyUnavailableCode = "AGENT_SESSION_DEPENDENCY_UNAVAILABLE"
	AgentSessionSnapshotUnavailableCode   = "AGENT_SESSION_SNAPSHOT_STORE_UNAVAILABLE"
)

// NewAgentSessionDependencyUnavailableError returns the sanitized retryable
// refusal for a configured agent-session dependency. Callers must log the
// underlying cause before returning this error; wrapping it would expose the
// dependency detail through the public CodedError message.
func NewAgentSessionDependencyUnavailableError() *CodedError {
	return NewUnavailableError(
		AgentSessionDependencyUnavailableCode,
		"agent session dependencies are temporarily unavailable",
		nil,
	)
}

// NewAgentSessionSnapshotUnavailableError returns the snapshot-tier-specific
// retryable refusal. It is separate from whole-feature configuration because
// ordinary sessions can remain healthy while restore/delete storage is down.
func NewAgentSessionSnapshotUnavailableError() *CodedError {
	return NewUnavailableError(
		AgentSessionSnapshotUnavailableCode,
		"agent session snapshot storage is temporarily unavailable",
		nil,
	)
}

const PaymentRequiredMessage = "Payment information is required for paid plans. Call create_billing_checkout_session to add a payment method, then retry."

// PaymentRequiredCode is the machine-readable code the paid-intent gate returns
// on every surface.
const PaymentRequiredCode = "PAYMENT_REQUIRED"

const (
	BillingErrorEnforced   = "BILLING_ENFORCED"
	BillingEnforcedMessage = "workspace billing enforcement is active"
)

// NewBillingEnforcedError is the stable cross-surface refusal for mutations
// blocked by the durable billing-enforcement lifecycle. It is deliberately
// distinct from PAYMENT_REQUIRED: enforcement is a 409 state conflict, while
// paid-intent onboarding is a 402 missing-payment-method refusal.
func NewBillingEnforcedError() *CodedError {
	return &CodedError{
		Code:     BillingErrorEnforced,
		sentinel: ErrBillingEnforced,
		msg:      BillingEnforcedMessage,
	}
}

// NewPaymentRequiredError is the one cross-surface refusal for paid intent.
// graphql-go projects Code into extensions.code, REST maps the wrapped
// sentinel to 402, and MCP reports the actionable message unchanged.
func NewPaymentRequiredError() *CodedError {
	return &CodedError{
		Code:     PaymentRequiredCode,
		Params:   map[string]any{"checkoutTool": "create_billing_checkout_session"},
		sentinel: ErrPaymentRequired,
		msg:      PaymentRequiredMessage,
	}
}

// InsufficientScopeCode is the one machine-readable refusal when a third-party
// human OAuth token lacks the capability a relation requires. REST, GraphQL,
// and MCP all project it; the required capability is the only parameter so the
// response cannot reveal whether an inaccessible target exists.
const (
	InsufficientScopeCode    = "INSUFFICIENT_SCOPE"
	InsufficientScopeMessage = "insufficient OAuth scope"
)

// NewInsufficientScopeError is the shared insufficient-scope refusal. It wraps
// ErrForbidden so transports stay on 403 (RFC 6750's insufficient_scope
// status) rather than inventing a 401 that would look like an expired token.
// required is one of the closed capabilities, or empty when the relation
// itself is unknown (still fail closed, still the same code).
func NewInsufficientScopeError(required string) *CodedError {
	params := map[string]any{}
	if required != "" {
		params["required"] = required
	}
	return &CodedError{
		Code:     InsufficientScopeCode,
		Params:   params,
		sentinel: ErrForbidden,
		msg:      InsufficientScopeMessage,
	}
}
