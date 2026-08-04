import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors, ServerError } from "@apollo/client/errors";
import {
  AgentSessionError,
  AgentSessionsUnavailableError,
  toAgentSessionError,
} from "@/features/agent-sessions/lib/errors";

function gqlError(
  code: string | undefined,
  message = "copy is not part of the contract",
  extra: Record<string, unknown> = {},
) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [
      {
        message,
        extensions: { ...(code ? { code } : {}), ...extra },
      },
    ],
  } as never);
}

function serverError(statusCode: number) {
  return new ServerError(`status ${statusCode}`, {
    response: new Response(null, { status: statusCode }),
    bodyText: "",
  });
}

// Every coded error the backend can raise → its typed messageKey.
const CODES = [
  "AGENT_SESSION_INPUT_INVALID",
  "AGENT_SESSION_ID_INVALID",
  "AGENT_SESSION_NOT_FOUND",
  "AGENT_SESSION_CONFLICT",
  "AGENT_SESSION_NOT_STEERABLE",
  "AGENT_SESSION_NOT_RESUMABLE",
  "AGENT_SESSION_NOT_ATTACHABLE",
  "AGENT_SESSION_TURN_IN_FLIGHT",
  "AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  "AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  "AGENT_SESSION_EGRESS_PHASE_INVALID",
];

describe("toAgentSessionError", () => {
  it("maps a raw 503 ServerError to AgentSessionsUnavailableError", () => {
    const out = toAgentSessionError(serverError(503));
    expect(out).toBeInstanceOf(AgentSessionsUnavailableError);
  });

  it("leaves a non-503 ServerError raw", () => {
    const err = serverError(500);
    expect(toAgentSessionError(err)).toBe(err);
  });

  it("maps a 'not configured' GraphQL error to AgentSessionsUnavailableError, keeping the message", () => {
    const out = toAgentSessionError(
      gqlError(undefined, "agent sessions not configured"),
    );
    expect(out).toBeInstanceOf(AgentSessionsUnavailableError);
    expect((out as Error).message).toBe("agent sessions not configured");
  });

  it.each(CODES)(
    "maps %s to a typed AgentSessionError with the right key",
    (code) => {
      const out = toAgentSessionError(gqlError(code, "server said no"));
      expect(out).toBeInstanceOf(AgentSessionError);
      const e = out as AgentSessionError;
      expect(e.code).toBe(code);
      expect(e.messageKey).toBe(`agentSessions.errors.${code}`);
      // The server's own message is preserved as the i18n fallback.
      expect(e.message).toBe("server said no");
    },
  );

  it("carries the egress {reason, entry} params (minus code) on the typed error", () => {
    const out = toAgentSessionError(
      gqlError("AGENT_SESSION_EGRESS_ALLOWLIST_INVALID", "bad entry", {
        reason: "not a hostname",
        entry: "http://x",
      }),
    );
    const e = out as AgentSessionError;
    expect(e.params).toEqual({ reason: "not a hostname", entry: "http://x" });
    // `code` is stripped from params (it lives on `.code`).
    expect(e.params).not.toHaveProperty("code");
  });

  it("carries the phase param on a NOT_STEERABLE conflict", () => {
    const out = toAgentSessionError(
      gqlError("AGENT_SESSION_NOT_STEERABLE", "nope", { phase: "running" }),
    );
    expect((out as AgentSessionError).params.phase).toBe("running");
  });

  it("returns the original error unchanged for an unknown code (no AGENT_SESSION_ prefix)", () => {
    const err = gqlError("PLAN_LIMIT", "over the limit");
    // No agent-session code and not a 'not configured' message → passthrough.
    expect(toAgentSessionError(err)).toBe(err);
  });

  it("passes a plain non-Apollo error straight through", () => {
    const err = new Error("network down");
    expect(toAgentSessionError(err)).toBe(err);
  });

  it("prefers the coded error even when a later error mentions 'not configured'", () => {
    const combined = new CombinedGraphQLErrors({
      data: null,
      errors: [
        {
          message: "bad input",
          extensions: { code: "AGENT_SESSION_INPUT_INVALID" },
        },
        { message: "agent sessions not configured" },
      ],
    } as never);
    const out = toAgentSessionError(combined);
    expect(out).toBeInstanceOf(AgentSessionError);
    expect((out as AgentSessionError).code).toBe("AGENT_SESSION_INPUT_INVALID");
  });
});
