import { describe, expect, it } from "vitest";
import {
  agentSessionDurationMs,
  agentSessionStatusPhraseKey,
  isSandboxCapacityFailure,
  SANDBOX_CAPACITY_LIMIT,
  isSteerablePhase,
  isTerminalPhase,
  toAgentSessionTicket,
  toAgentSessionView,
} from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionFieldsFragment,
  AgentSessionMintFieldsFragment,
} from "@/graphql/definitions";
import type { AgentSessionPhase } from "@/features/agent-sessions/types";
import { goStringConstant } from "@/test/go-source";
import { agentSessionView } from "@/test/mocks/agent-session";

const ALL_PHASES: AgentSessionPhase[] = [
  "creating",
  "running",
  "resuming",
  "redispatching",
  "completed",
  "failed",
  "canceling",
  "canceled",
];

/** A minimal, valid wire fragment; each field is overridable per assertion. */
function wire(
  over: Partial<AgentSessionFieldsFragment> = {},
): AgentSessionFieldsFragment {
  return {
    __typename: "AgentSession",
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      __typename: "AgentSessionConfig",
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "do the thing",
      template: null,
    },
    sandboxId: null,
    phase: "running",
    status: "working",
    headSha: null,
    prUrl: null,
    prNumber: null,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    failureReasonCode: null,
    createdAt: "2026-08-02T00:00:00.000Z",
    updatedAt: "2026-08-02T00:00:00.000Z",
    canceledAt: null,
    ...over,
  } as AgentSessionFieldsFragment;
}

describe("phase predicates", () => {
  it("isTerminalPhase is true only for completed/failed/canceled", () => {
    const terminal = new Set(["completed", "failed", "canceled"]);
    for (const phase of ALL_PHASES) {
      expect(isTerminalPhase(phase)).toBe(terminal.has(phase));
    }
  });

  it("isSteerablePhase is true only for the idle completed/failed phases", () => {
    // canceled is terminal but NOT steerable — the redispatch path rejects it.
    const steerable = new Set(["completed", "failed"]);
    for (const phase of ALL_PHASES) {
      expect(isSteerablePhase(phase)).toBe(steerable.has(phase));
    }
    expect(isSteerablePhase("canceled")).toBe(false);
  });

  it("treats an unknown phase string as neither terminal nor steerable", () => {
    expect(isTerminalPhase("bogus")).toBe(false);
    expect(isSteerablePhase("bogus")).toBe(false);
  });
});

describe("toAgentSessionView", () => {
  it("projects every wire field and derives the phase helpers", () => {
    const view = toAgentSessionView(
      wire({
        phase: "completed",
        status: "done",
        headSha: "abcdef1234",
        prUrl: "https://github.com/acme/widgets/pull/7",
        prNumber: 7,
        turns: 3,
        deliveryMode: "redispatch",
        failureReason: "the workspace is at its sandbox limit",
        failureReasonCode: "SANDBOX_CAPACITY_LIMIT",
        agentConfig: {
          __typename: "AgentSessionConfig",
          agent: "gemini",
          model: "gemini-2.5",
          modelEndpoint: "https://api.example.com",
          task: "ship it",
          template: "tpl-1",
        },
      }),
    );

    expect(view).toMatchObject({
      id: "as-1",
      ownerId: "tea-1",
      repo: "acme/widgets",
      branch: "bex-agent/fix",
      phase: "completed",
      status: "done",
      headSha: "abcdef1234",
      prUrl: "https://github.com/acme/widgets/pull/7",
      prNumber: 7,
      turns: 3,
      deliveryMode: "redispatch",
      failureReason: "the workspace is at its sandbox limit",
      failureReasonCode: "SANDBOX_CAPACITY_LIMIT",
      isTerminal: true,
      isSteerable: true,
    });
    expect(view.agentConfig).toEqual({
      agent: "gemini",
      model: "gemini-2.5",
      modelEndpoint: "https://api.example.com",
      task: "ship it",
      template: "tpl-1",
    });
  });

  it("normalizes null/absent optionals to their defaults", () => {
    const view = toAgentSessionView(
      wire({ turns: null, failureReasonCode: undefined }),
    );
    expect(view.turns).toBe(0);
    expect(view.deliveryMode).toBeNull();
    expect(view.failureReasonCode).toBeNull();
    expect(view.isTerminal).toBe(false); // phase "running"
    expect(view.isSteerable).toBe(false);
  });

  // w21/001: the server computes steerability and the outcome; the phase table
  // is only the fallback for a partial cache entry.
  it("reads steerable, state and outcome from the server", () => {
    const view = toAgentSessionView(
      wire({
        phase: "completed",
        state: "needs_input",
        steerable: false, // the server's word wins over the phase table
        outcome: {
          __typename: "AgentSessionOutcome",
          state: "needs_input",
          steerable: false,
          turn: 2,
          waitingFor: "approval",
          summary: "Done, see the PR.",
          deliveryNote: null,
          failureReasonCode: null,
          result: '{"ok":true}',
          pendingDecision: {
            __typename: "AgentSessionDecision",
            id: "dcs-1",
            kind: "approval",
            title: "Run (force push): git push -f origin main",
            // Unknown fields and wrong types drop out of the parsed detail.
            detail:
              '{"toolKind":"execute","command":"git push -f origin main","rule":"force-push","paths":["a",1],"policyVersion":2}',
            state: "pending",
            createdAt: "2026-11-06T10:00:00Z",
            expiresAt: "2026-11-07T10:00:00Z",
          },
          cost: { __typename: "AgentSessionCost", modelUSD: "0.001234" },
          pullRequest: null,
        },
      }),
    );
    expect(view.isSteerable).toBe(false);
    expect(view.state).toBe("needs_input");
    expect(view.outcome).toEqual({
      state: "needs_input",
      steerable: false,
      turn: 2,
      waitingFor: "approval",
      pendingDecision: {
        id: "dcs-1",
        kind: "approval",
        title: "Run (force push): git push -f origin main",
        detail: {
          toolKind: "execute",
          command: "git push -f origin main",
          rule: "force-push",
        },
        state: "pending",
        createdAt: "2026-11-06T10:00:00Z",
        expiresAt: "2026-11-07T10:00:00Z",
      },
      summary: "Done, see the PR.",
      deliveryNote: null,
      failureReasonCode: null,
      result: '{"ok":true}',
      cost: { modelUSD: "0.001234" },
    });
    // Without the field, a completed session is steerable by phase.
    const partial = toAgentSessionView(wire({ phase: "completed" }));
    expect(partial.isSteerable).toBe(true);
    expect(partial.state).toBeNull();
    expect(partial.outcome).toBeNull();
  });
});

// w10/m9 t003 (w3/013): `url` is the phase-2 raw-ACP WebSocket gateway
// origin — never the phase-1 SSE stream endpoint a naive caller might build
// from it. `streamUrl` is the server-authoritative stream address; this pins
// the wire→domain mapping keeps the two distinct rather than conflating them.
describe("toAgentSessionTicket", () => {
  const mint = (
    over: Partial<AgentSessionMintFieldsFragment> = {},
  ): AgentSessionFieldsFragment & AgentSessionMintFieldsFragment => ({
    ...wire(),
    ticket: "ticket-1",
    url: "wss://ssh.bex.co/agent-sessions",
    streamUrl: "https://api.bex.co/v1/agent-sessions/as-1/stream",
    expiresAt: "2026-08-02T00:01:30.000Z",
    ...over,
  });

  it("carries ticket, url, streamUrl, and expiresAt through as distinct fields", () => {
    const result = toAgentSessionTicket(mint());
    expect(result.ticket).toBe("ticket-1");
    expect(result.url).toBe("wss://ssh.bex.co/agent-sessions");
    expect(result.streamUrl).toBe(
      "https://api.bex.co/v1/agent-sessions/as-1/stream",
    );
    expect(result.expiresAt).toBe("2026-08-02T00:01:30.000Z");
    expect(result.session.id).toBe("as-1");
  });

  it("normalizes an absent streamUrl to null (BEX_API_PUBLIC_URL unconfigured)", () => {
    const result = toAgentSessionTicket(mint({ streamUrl: null }));
    expect(result.streamUrl).toBeNull();
    // url is unaffected — the two fields degrade independently.
    expect(result.url).toBe("wss://ssh.bex.co/agent-sessions");
  });
});

describe("agentSessionDurationMs", () => {
  const start = "2026-08-02T00:00:00.000Z";
  const startMs = Date.parse(start);

  it("measures a running session up to the injected now", () => {
    const view = toAgentSessionView(
      wire({ phase: "running", createdAt: start }),
    );
    expect(agentSessionDurationMs(view, startMs + 90_000)).toBe(90_000);
  });

  it("pins a terminal session to its updatedAt, ignoring now", () => {
    const view = toAgentSessionView(
      wire({
        phase: "completed",
        createdAt: start,
        updatedAt: new Date(startMs + 60_000).toISOString(),
      }),
    );
    // now is way ahead, but the elapsed pins to updatedAt for a settled session.
    expect(agentSessionDurationMs(view, startMs + 10_000_000)).toBe(60_000);
  });

  it("prefers canceledAt over updatedAt for a canceled session", () => {
    const view = toAgentSessionView(
      wire({
        phase: "canceled",
        createdAt: start,
        canceledAt: new Date(startMs + 30_000).toISOString(),
        updatedAt: new Date(startMs + 999_000).toISOString(),
      }),
    );
    expect(agentSessionDurationMs(view, startMs + 10_000_000)).toBe(30_000);
  });

  it("never returns a negative elapsed", () => {
    const view = toAgentSessionView(
      wire({ phase: "running", createdAt: start }),
    );
    expect(agentSessionDurationMs(view, startMs - 5_000)).toBe(0);
  });

  it("returns 0 when createdAt is unparseable", () => {
    const view = toAgentSessionView(wire({ createdAt: "not-a-date" }));
    expect(agentSessionDurationMs(view, startMs)).toBe(0);
  });
});

// Moved here from the retired session-sidebar test when w5/m64 folded that
// rail into DashboardSidebar — this asserts the mapper, not the component.
describe("agentSessionStatusPhraseKey", () => {
  it("maps phase + PR presence onto the Devin-style phrase", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "completed", prNumber: 6 }),
    ).toBe("agentSessions.statusPhrase.prReady");
    // The settled phases reuse the phase chip's copy rather than restating it.
    expect(
      agentSessionStatusPhraseKey({ phase: "completed", prNumber: null }),
    ).toBe("agentSessions.phase.completed");
    expect(
      agentSessionStatusPhraseKey({ phase: "failed", prNumber: null }),
    ).toBe("agentSessions.phase.failed");
    expect(
      agentSessionStatusPhraseKey({ phase: "canceled", prNumber: null }),
    ).toBe("agentSessions.phase.canceled");
    expect(
      agentSessionStatusPhraseKey({ phase: "canceling", prNumber: null }),
    ).toBe("agentSessions.phase.canceled");
    for (const phase of [
      "running",
      "creating",
      "resuming",
      "redispatching",
    ] as const) {
      expect(agentSessionStatusPhraseKey({ phase, prNumber: null })).toBe(
        "agentSessions.statusPhrase.working",
      );
    }
  });
});

describe("isSandboxCapacityFailure", () => {
  it("decides by the code, whatever the sentence says", () => {
    expect(
      isSandboxCapacityFailure(
        agentSessionView({
          phase: "failed",
          failureReason: "the workspace is at its sandbox limit",
          failureReasonCode: SANDBOX_CAPACITY_LIMIT,
        }),
      ),
    ).toBe(true);
  });

  it("uses bex-api's capacity code", () => {
    expect(SANDBOX_CAPACITY_LIMIT).toBe(
      goStringConstant(
        "lego/backend/internal/sandbox/service.go",
        "CodeSandboxCapacityLimit",
      ),
    );
  });

  it("is false for a failure without the code, even one that mentions capacity", () => {
    expect(
      isSandboxCapacityFailure(
        agentSessionView({
          phase: "failed",
          failureReason: "sandbox capacity reached",
          status: "sandbox capacity reached",
        }),
      ),
    ).toBe(false);
  });

  it("is false for a non-failed session", () => {
    expect(
      isSandboxCapacityFailure({
        phase: "running",
        failureReasonCode: SANDBOX_CAPACITY_LIMIT,
      }),
    ).toBe(false);
  });
});
