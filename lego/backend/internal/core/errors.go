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

import "errors"

// The domain error sentinels every feature returns and the REST adapter maps to
// status codes (see WriteErr). Shared here so one WriteErr can map them all and
// the surfaces stay authorization/error identical.
var (
	// ErrNotFound is returned when a resource (App/Database) does not exist.
	ErrNotFound = errors.New("app not found")
	// ErrLogsUnavailable is returned by the logs verbs when no pod-log source is
	// wired (adapters surface it as 503, not 404 — the App exists, the source
	// doesn't).
	ErrLogsUnavailable = errors.New("logs source not configured")
	// ErrLogStoreUnavailable is returned by the logs verbs when a caller asks for
	// something only the durable log store can answer — request logs, or a
	// structured filter (level/statusCode/method/path/host) — while bex-api runs
	// in pod-log fallback mode (BEX_LOKI_URL unset). Adapters surface it as 503:
	// refusing beats silently ignoring the filter and returning unfiltered lines
	// (docs/ADR010-observability.md § Log filters).
	ErrLogStoreUnavailable = errors.New("request logs and structured log filters require the durable log store")
	// ErrMetricsUnavailable is returned by the metrics verbs when the backend a
	// metric needs isn't wired (adapters surface it as 503).
	ErrMetricsUnavailable = errors.New("metrics source not configured")
	// ErrAPIKeysUnavailable is returned by the api-key verbs when no store is wired.
	ErrAPIKeysUnavailable = errors.New("api-key store not configured")
	// ErrSSHKeysUnavailable is returned by the SSH-key verbs when the control-plane
	// store is not wired. SSH authentication cannot safely degrade to an in-memory
	// key registry because a restart would revoke access unpredictably.
	ErrSSHKeysUnavailable = errors.New("ssh-key store not configured")
	// ErrSecretsUnavailable is returned by the env-vars verbs when no secret store
	// is wired (BEX_OPENBAO_URL unset); adapters surface it as 503.
	ErrSecretsUnavailable = errors.New("secret store not configured")
	// ErrWorkspacesUnavailable is returned by the workspace verbs when the
	// control-plane store isn't wired (bex-api running without BEX_CP_DB_URI);
	// adapters surface it as 503 (the owners read API exists, the backing store
	// doesn't).
	ErrWorkspacesUnavailable = errors.New("workspaces store not configured")
	// ErrDeploysUnavailable is returned by the deploy-history verbs when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503 — deploy history has no CR-only equivalent to fall back to.
	ErrDeploysUnavailable = errors.New("deploy history store not configured")
	// ErrSandboxesUnavailable is returned by the sandbox verbs when the
	// OpenSandbox lifecycle client isn't wired (BEX_OPENSANDBOX_URL unset);
	// adapters surface it as 503 (pillar 5, ADR042/w3/m32).
	ErrSandboxesUnavailable = errors.New("sandbox runtime not configured")
	// ErrAgentSessionsUnavailable is returned when any required session control
	// plane dependency is absent: Postgres, OpenSandbox, ticket signer, or the
	// browser-reachable gateway origin. A partially configured session cannot be
	// created safely, so the feature fails closed as one unit.
	ErrAgentSessionsUnavailable = errors.New("agent sessions not configured")
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
	ErrAuthzUnavailable = errors.New("authorization service unavailable")
	// ErrUsageUnavailable is returned by the usage verb when the store isn't
	// wired (BEX_CP_DB_URI unset); adapters surface it as 503.
	ErrUsageUnavailable = errors.New("usage unavailable")
	// ErrBillingUnavailable is returned by the customer-billing onboarding
	// verbs when Stripe is disabled or cannot be reached. It is deliberately
	// distinct from ErrUsageUnavailable: advisory usage remains available while
	// hosted Checkout, Portal, and payment readiness fail closed.
	ErrBillingUnavailable = errors.New("billing integration unavailable")
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
	ErrAuditUnavailable = errors.New("audit log store not configured")
	// ErrGitHubUnavailable is returned by the git-connect verbs when the GitHub
	// App is not configured (BEX_GITHUB_APP_* unset) or the control-plane store
	// isn't wired (BEX_CP_DB_URI unset) — adapters surface it as 503
	// (docs/ADR026-github-integration.md).
	ErrGitHubUnavailable = errors.New("github integration not configured")
	// ErrEventsUnavailable is returned by the service-events feed when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503. BOTH of the feed's sources (deploys, audit_events) are control-plane
	// tables, so there is no CR-only feed to degrade to — omitted, not faked
	// (w3/m7, the deploy-history precedent).
	ErrEventsUnavailable = errors.New("events store not configured")
	// ErrNotificationsUnavailable is returned by the notification-settings verbs
	// when the control-plane store isn't wired (BEX_CP_DB_URI unset); adapters
	// surface it as 503 (w3/m9, the deploy-history precedent).
	ErrNotificationsUnavailable = errors.New("notification settings store not configured")
	// ErrPushUnavailable is returned when a caller attempts to register a push
	// destination while the server transport is deliberately disabled. Reading
	// notification preferences and supervision remains available.
	ErrPushUnavailable = errors.New("mobile push transport not configured")
	// ErrRegistryCredentialsUnavailable is returned by the registry-credentials
	// verbs when the control-plane store (BEX_CP_DB_URI) or the secret store
	// (BEX_OPENBAO_URL) isn't wired — either is required, since a credential's
	// metadata lives in one and its secret in the other; adapters surface it
	// as 503.
	ErrRegistryCredentialsUnavailable = errors.New("registry credential store not configured")
	// ErrWebhooksUnavailable is returned by the outbound-webhook verbs when the
	// control-plane store isn't wired (BEX_CP_DB_URI unset); adapters surface it
	// as 503 (w3/m11, the deploy-history precedent) — both the endpoint registry
	// and the delivery queue are control-plane tables, so there is nothing to
	// degrade to.
	ErrWebhooksUnavailable = errors.New("webhook store not configured")
	// ErrLogoutUnavailable is returned by the CLI-logout revoke verb
	// (POST /v1/oauth/revoke) when the Hydra admin endpoint that clears a human's
	// consent chain is unwired (BEX_HYDRA_ADMIN_URL unset) or unreachable;
	// adapters surface it as 503. It exists so /v1/oauth/revoke — a Render-shaped
	// REST endpoint — speaks the one Render error dialect on every branch (w9/m38,
	// w9/008), unlike the RFC 8628 device endpoints whose OAuth-shaped bodies the
	// CLI parses as token responses.
	ErrLogoutUnavailable = errors.New("logout revocation service unavailable")
	// ErrShellUnavailable is returned by the Web Shell ticket verb when the
	// browser-terminal transport is not configured (BEX_SHELL_TICKET_SECRET or
	// BEX_SHELL_WS_URL unset); adapters surface it as 503. Native `ssh` is
	// unaffected — the copy-ready command still works (docs/ADR035-ssh.md
	// § Browser Web Shell).
	ErrShellUnavailable = errors.New("web shell transport not configured")
)

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

const PaymentRequiredMessage = "Payment information is required for paid plans. Call create_billing_checkout_session to add a payment method, then retry."

// NewPaymentRequiredError is the one cross-surface refusal for paid intent.
// graphql-go projects Code into extensions.code, REST maps the wrapped
// sentinel to 402, and MCP reports the actionable message unchanged.
func NewPaymentRequiredError() *CodedError {
	return &CodedError{
		Code:     "PAYMENT_REQUIRED",
		Params:   map[string]any{"checkoutTool": "create_billing_checkout_session"},
		sentinel: ErrPaymentRequired,
		msg:      PaymentRequiredMessage,
	}
}
