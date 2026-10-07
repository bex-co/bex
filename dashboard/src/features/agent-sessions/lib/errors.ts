// Typed error mapping for agent-session operations. bex-api reports coded
// availability, validation, and conflict errors (backend/internal/agentsessions,
// internal/sessionegress) carrying machine-readable params. Mapping keys off
// the stable `extensions.code`, never a message substring, so backend copy
// changes have no effect on which UI branch fires.

import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";

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

/**
 * Resolves a typed agent-session error into a display string via `t()`: the
 * availability copy for a configuration/dependency/storage refusal, the coded
 * `agentSessions.errors.*` message (with params + server fallback) for any other
 * coded error, else the raw error text. Callers own only the sink (toast vs
 * inline state); this keeps the error→copy mapping in one place.
 * Note: a caller that needs to branch on the *kind* (e.g. the create composer
 * anchoring a coded error to a form field) still inspects the typed error itself.
 */
export function agentSessionErrorMessage(
  err: unknown,
  t: (key: string, params?: Record<string, string | number>) => string,
): string {
  const availabilityCopy = agentSessionAvailabilityCopy(err);
  if (availabilityCopy) return t(availabilityCopy.bodyKey);
  if (err instanceof AgentSessionError) {
    return t(err.messageKey, { ...err.params, defaultValue: err.message });
  }
  return err instanceof Error ? err.message : String(err);
}

export interface AgentSessionAvailabilityCopy {
  titleKey: string;
  bodyKey: string;
  destructive: boolean;
}

const AVAILABILITY_COPY: Record<string, AgentSessionAvailabilityCopy> = {
  AGENT_SESSION_NOT_CONFIGURED: {
    titleKey: "agentSessions.unavailableTitle",
    bodyKey: "agentSessions.unavailableBody",
    destructive: false,
  },
  AGENT_SESSION_DEPENDENCY_UNAVAILABLE: {
    titleKey: "agentSessions.dependencyUnavailableTitle",
    bodyKey: "agentSessions.dependencyUnavailableBody",
    destructive: true,
  },
  AGENT_SESSION_SNAPSHOT_STORE_UNAVAILABLE: {
    titleKey: "agentSessions.snapshotUnavailableTitle",
    bodyKey: "agentSessions.snapshotUnavailableBody",
    destructive: true,
  },
};

/** The single code→title/body mapping used by create and generic error sinks. */
export function agentSessionAvailabilityCopy(
  err: unknown,
): AgentSessionAvailabilityCopy | null {
  return err instanceof AgentSessionError
    ? (AVAILABILITY_COPY[err.code] ?? null)
    : null;
}

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
 * Normalizes an Apollo operation rejection into a typed agent-session error:
 * coded GraphQL failures preserve their `AGENT_SESSION_*` code (an
 * unconfigured platform is AGENT_SESSION_NOT_CONFIGURED), and a cause-less
 * transport 503 becomes dependency-unavailable. Other failures, wording that
 * merely says "not configured" included, stay unchanged (w5/m130).
 */
export function toAgentSessionError(err: unknown): unknown {
  // A raw non-GraphQL 503 carries no cause code. Treat it as retryable rather
  // than claiming the platform was never configured — only the explicit code
  // is allowed to produce operator-directed copy.
  if (ServerError.is(err) && err.statusCode === 503) {
    return new AgentSessionError(
      "AGENT_SESSION_DEPENDENCY_UNAVAILABLE",
      "agent session dependencies are temporarily unavailable",
      {},
    );
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
  }
  return err;
}
