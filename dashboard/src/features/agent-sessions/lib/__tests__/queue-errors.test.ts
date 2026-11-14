import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  AgentSessionError,
  agentSessionErrorMessage,
  toAgentSessionError,
} from "@/features/agent-sessions/lib/errors";
import { translator } from "@/test/translator";

// Every coded refusal of the message queue and Stop turn (ADR047 D12) reaches
// the person as a sentence of its own, in both languages, decided by the code.

function gqlError(code: string, params: Record<string, unknown> = {}) {
  return new CombinedGraphQLErrors({
    data: null,
    errors: [
      {
        message: "server wording is not part of the contract",
        extensions: { code, ...params },
      },
    ],
  } as never);
}

const CODES: Array<{
  code: string;
  params?: Record<string, unknown>;
  en: RegExp;
}> = [
  {
    code: "AGENT_SESSION_QUEUE_FULL",
    params: { id: "ags-1", max: 10 },
    en: /already has 10 messages queued/,
  },
  {
    code: "AGENT_SESSION_QUEUE_INLINE_SECRETS",
    en: /inline secrets can't wait in the queue/,
  },
  {
    code: "AGENT_QUEUED_MESSAGE_DISPATCHED",
    en: /already started running/,
  },
  { code: "AGENT_QUEUED_MESSAGE_NOT_FOUND", en: /no longer in the queue/ },
  {
    code: "AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED",
    en: /only the sender can edit it/,
  },
  { code: "AGENT_SESSION_NO_TURN_RUNNING", en: /No turn is running/ },
  {
    code: "AGENT_SESSION_TURN_STARTING",
    en: /still starting, so there is nothing to stop yet/,
  },
];

describe("queue and Stop turn error codes", () => {
  it.each(CODES)("types $code, whatever its prefix", ({ code, params }) => {
    const out = toAgentSessionError(gqlError(code, params));
    expect(out).toBeInstanceOf(AgentSessionError);
    expect((out as AgentSessionError).code).toBe(code);
    expect((out as AgentSessionError).messageKey).toBe(
      `agentSessions.errors.${code}`,
    );
  });

  it.each(CODES)(
    "says $code in English and in Chinese",
    ({ code, params, en }) => {
      const err = toAgentSessionError(gqlError(code, params));
      const english = agentSessionErrorMessage(err, translator("en"));
      const chinese = agentSessionErrorMessage(err, translator("zh"));
      expect(english).toMatch(en);
      expect(english).not.toContain("server wording");
      expect(english).not.toContain("agentSessions.errors");
      expect(chinese).not.toContain("server wording");
      expect(chinese).not.toContain("agentSessions.errors");
      expect(chinese).not.toBe(english);
      expect(chinese).toMatch(/[一-鿿]/);
    },
  );

  it("names the queue's limit in both languages", () => {
    const err = toAgentSessionError(
      gqlError("AGENT_SESSION_QUEUE_FULL", { max: 10 }),
    );
    expect(agentSessionErrorMessage(err, translator("zh"))).toContain("10");
  });

  it("leaves a code of another family alone", () => {
    const err = gqlError("QUEUED_ELSEWHERE");
    expect(toAgentSessionError(err)).toBe(err);
  });

  it("falls back to the server's sentence for a queue code it has no copy for", () => {
    const err = toAgentSessionError(gqlError("AGENT_QUEUED_MESSAGE_FUTURE"));
    expect(err).toBeInstanceOf(AgentSessionError);
    expect(agentSessionErrorMessage(err, translator("en"))).toBe(
      "server wording is not part of the contract",
    );
  });
});
