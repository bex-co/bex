// Typed error mapping for the agent-session mutations. bex-api reports two
// distinct, actionable failure shapes the UI must render differently from a
// generic Apollo error: a 503 when the feature is unconfigured, and coded
// `AGENT_SESSION_*` validation/conflict errors (backend/internal/agentsessions,
// internal/sessionegress) carrying machine-readable params. Mapping keys off the
// stable `extensions.code`, never a message substring, so backend copy changes
// have no effect on which UI branch fires.

import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";

/**
 * Thrown when bex-api reports the agent-session surface is unconfigured
 * (`core.ErrAgentSessionsUnavailable` → 503 REST / "agent sessions not
 * configured" GraphQL error, when `BEX_AGENT_SESSION_GATEWAY_URL`/ticket secret
 * are unset). A distinct degraded state — the page says the feature is off
 * rather than surfacing a transient failure.
 */
export class AgentSessionsUnavailableError extends Error {
  constructor(message = "agent sessions not configured") {
    super(message);
    this.name = "AgentSessionsUnavailableError";
  }
}

/** Structured params carried on a coded agent-session error's extensions. */
export interface AgentSessionErrorParams {
  /** Egress-allowlist rejection reason (AGENT_SESSION_EGRESS_ALLOWLIST_INVALID). */
  reason?: string;
  /** The offending allowlist entry, when the reason names one. */
  entry?: string;
  /** The offending input field, when the backend named one. */
  field?: string;
  /** The session's current phase, on a conflict (NOT_STEERABLE/TURN_IN_FLIGHT). */
  phase?: string;
  [key: string]: unknown;
}

/**
 * A coded `AGENT_SESSION_*` failure. Carries the stable `code`, an i18n-ready
 * `messageKey` (`agentSessions.errors.<CODE>`) the caller resolves through
 * `t()`, the server's human message as the fallback, and the structured
 * `params` (egress errors carry `{reason, entry}`).
 */
export class AgentSessionError extends Error {
  readonly code: string;
  readonly messageKey: string;
  readonly params: AgentSessionErrorParams;

  constructor(code: string, message: string, params: AgentSessionErrorParams) {
    super(message);
    this.name = "AgentSessionError";
    this.code = code;
    this.messageKey = `agentSessions.errors.${code}`;
    this.params = params;
  }
}

const UNAVAILABLE = /not configured/i;

/** Pulls the flattened extension params (minus `code`) off one GraphQL error. */
function extensionParams(
  extensions: Record<string, unknown> | undefined,
): AgentSessionErrorParams {
  const out: AgentSessionErrorParams = {};
  if (!extensions) return out;
  for (const [key, value] of Object.entries(extensions)) {
    if (key === "code") continue;
    out[key] = value;
  }
  return out;
}

/**
 * Normalizes an Apollo mutation rejection into a typed agent-session error:
 * `AgentSessionsUnavailableError` for the 503/"not configured" state, an
 * `AgentSessionError` for any `AGENT_SESSION_*` coded GraphQL error, otherwise
 * the original error unchanged (transient/transport failures stay raw so the
 * caller can retry-toast them).
 */
export function toAgentSessionError(err: unknown): unknown {
  // Raw 503 (e.g. a non-GraphQL transport response) → unavailable.
  if (ServerError.is(err) && err.statusCode === 503) {
    return new AgentSessionsUnavailableError();
  }
  if (CombinedGraphQLErrors.is(err)) {
    for (const item of err.errors) {
      const code = item.extensions?.["code"];
      if (typeof code === "string" && code.startsWith("AGENT_SESSION_")) {
        return new AgentSessionError(
          code,
          item.message,
          extensionParams(item.extensions as Record<string, unknown>),
        );
      }
    }
    // No agent-session code but the feature is off → "not configured" message.
    if (err.errors.some((item) => UNAVAILABLE.test(item.message))) {
      return new AgentSessionsUnavailableError(err.errors[0]?.message);
    }
  }
  return err;
}
