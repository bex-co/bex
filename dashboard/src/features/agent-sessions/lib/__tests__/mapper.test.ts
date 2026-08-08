import { describe, expect, it } from "vitest";
import {
  agentSessionDurationMs,
  agentSessionStatusPhraseKey,
  isSteerablePhase,
  isTerminalPhase,
  toAgentSessionView,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionFieldsFragment } from "@/graphql/definitions";
import type { AgentSessionPhase } from "@/features/agent-sessions/types";

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
    evidence: null,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
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
    const view = toAgentSessionView(wire({ turns: null, evidence: null }));
    expect(view.turns).toBe(0);
    expect(view.evidence).toBeNull();
    expect(view.deliveryMode).toBeNull();
    expect(view.isTerminal).toBe(false); // phase "running"
    expect(view.isSteerable).toBe(false);
  });

  it("maps the evidence sub-object, defaulting missing lists/flags", () => {
    const view = toAgentSessionView(
      wire({
        evidence: {
          __typename: "AgentSessionEvidence",
          commandLog: ["go build ./..."],
          testOutput: null,
          outputTail: "ok",
          changedFiles: null,
          commits: 2,
          truncated: true,
        },
      }),
    );
    expect(view.evidence).toEqual({
      commandLog: ["go build ./..."],
      testOutput: [],
      outputTail: "ok",
      changedFiles: [],
      commits: 2,
      truncated: true,
    });
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
    expect(agentSessionStatusPhraseKey({ phase: "failed", prNumber: null })).toBe(
      "agentSessions.phase.failed",
    );
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
