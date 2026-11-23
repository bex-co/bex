import { describe, expect, it } from "vitest";
import {
  agentSessionDurationMs,
  agentSessionStatusPhraseKey,
  agentSessionWaitMs,
  formatDurationShort,
  isSandboxCapacityFailure,
  sandboxStartFailureCause,
  SANDBOX_CAPACITY_LIMIT,
  SANDBOX_START_FAILED,
  isSteerablePhase,
  isTerminalPhase,
  toAgentSessionTicket,
  toAgentSessionView,
  toInterruptedTurns,
  toTurnRetentionView,
} from "@/features/agent-sessions/lib/mapper";
import type {
  AgentSessionFieldsFragment,
  AgentSessionMintFieldsFragment,
} from "@/graphql/definitions";
import {
  AGENT_SESSION_PR_STATES,
  SANDBOX_START_FAILURE_CAUSES,
  TURN_RETENTION_LOST_REASONS,
  TURN_RETENTION_STATES,
  type AgentSessionPhase,
  type AgentSessionView,
} from "@/features/agent-sessions/types";
import {
  goStringConstant,
  goStringConstantsWithPrefix,
} from "@/test/go-source";
import {
  agentSessionView,
  RETAINED_UNTIL,
  wireTurn,
} from "@/test/mocks/agent-session";

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
    failureCause: null,
    waitingFor: null,
    waitingSince: null,
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
          githubCli: false,
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
      githubCli: false,
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
          summary: "Done, see the PR.",
          deliveryNote: null,
          failureReasonCode: null,
          result: '{"ok":true}',
          cost: {
            __typename: "AgentSessionCost",
            modelUSD: "0.001234",
            computeUSD: "0.000100",
            totalUSD: "0.001334",
          },
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
      summary: "Done, see the PR.",
      deliveryNote: null,
      failureReasonCode: null,
      result: '{"ok":true}',
      cost: {
        modelUSD: "0.001234",
        computeUSD: "0.000100",
        totalUSD: "0.001334",
      },
    });
    // Without the field, a completed session is steerable by phase.
    const partial = toAgentSessionView(wire({ phase: "completed" }));
    expect(partial.isSteerable).toBe(true);
    expect(partial.state).toBeNull();
    expect(partial.outcome).toBeNull();
  });

  // ADR111 D4: the build the latest turn's sandbox booted from.
  it("carries the environment build a turn booted from, else null", () => {
    expect(
      toAgentSessionView(wire({ environmentBuildId: "arb-1" }))
        .environmentBuildId,
    ).toBe("arb-1");
    expect(
      toAgentSessionView(wire({ environmentBuildId: null })).environmentBuildId,
    ).toBeNull();
    // A partial cache entry from before the field.
    expect(toAgentSessionView(wire()).environmentBuildId).toBeNull();
  });

  // w9/m386: the PR's state as bex last read it from GitHub.
  it.each(["open", "merged", "closed"])("carries PR state %s", (prState) => {
    expect(toAgentSessionView(wire({ prNumber: 7, prState })).prState).toBe(
      prState,
    );
  });

  // w10/047: the auto-merge armed on the PR and its merge queue.
  it("carries the PR's auto-merge and merge queue", () => {
    const armed = toAgentSessionView(
      wire({
        prNumber: 7,
        prAutoMerge: {
          __typename: "AgentSessionPRAutoMerge",
          method: "squash",
          enabledBy: "octocat",
          enabledAt: "2026-11-22T09:30:00Z",
        },
        prMergeQueued: true,
      }),
    );
    expect(armed.prAutoMerge).toEqual({
      method: "squash",
      enabledBy: "octocat",
      enabledAt: "2026-11-22T09:30:00Z",
    });
    expect(armed.prMergeQueued).toBe(true);
    // GitHub did not say who or when: null, never undefined.
    expect(
      toAgentSessionView(
        wire({
          prAutoMerge: {
            __typename: "AgentSessionPRAutoMerge",
            method: "merge",
            enabledBy: null,
            enabledAt: null,
          },
        }),
      ).prAutoMerge,
    ).toEqual({ method: "merge", enabledBy: null, enabledAt: null });
    // Nothing armed, or a cache entry from before the fields.
    const none = toAgentSessionView(wire());
    expect(none.prAutoMerge).toBeNull();
    expect(none.prMergeQueued).toBe(false);
  });

  it("maps an unread, absent or unknown PR state to null", () => {
    expect(toAgentSessionView(wire({ prState: null })).prState).toBeNull();
    expect(toAgentSessionView(wire()).prState).toBeNull();
    // A state a newer bex-api adds shows no badge rather than a raw key.
    expect(toAgentSessionView(wire({ prState: "draft" })).prState).toBeNull();
  });

  it("knows exactly bex-api's PR states", () => {
    const path = "lego/backend/internal/store/agentsession_pr_state.go";
    expect([...AGENT_SESSION_PR_STATES]).toEqual([
      goStringConstant(path, "PRStateOpen"),
      goStringConstant(path, "PRStateMerged"),
      goStringConstant(path, "PRStateClosed"),
    ]);
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

// w10/m497: a turn the sandbox pool has no room for waits for a machine; a
// start that still fails names its cause.
describe("waiting for a sandbox machine and a start failure's cause", () => {
  const since = "2026-11-18T12:00:00Z";

  it("projects the wait while the session can still start", () => {
    for (const phase of ["creating", "redispatching"] as const) {
      expect(
        toAgentSessionView(
          wire({ phase, waitingFor: "sandbox_capacity", waitingSince: since }),
        ),
      ).toMatchObject({ waitingFor: "sandbox_capacity", waitingSince: since });
    }
    expect(toAgentSessionView(wire())).toMatchObject({
      waitingFor: null,
      waitingSince: null,
      failureCause: null,
    });
  });

  it("reads a wait it does not know, or one on a settled session, as none", () => {
    expect(
      toAgentSessionView(
        wire({ phase: "creating", waitingFor: "gpu", waitingSince: since }),
      ),
    ).toMatchObject({ waitingFor: null, waitingSince: null });
    for (const phase of ["completed", "failed", "canceled"] as const) {
      expect(
        toAgentSessionView(
          wire({ phase, waitingFor: "sandbox_capacity", waitingSince: since }),
        ),
      ).toMatchObject({ waitingFor: null, waitingSince: null });
    }
    // A partial cache entry that predates the fields.
    expect(
      toAgentSessionView(
        wire({ waitingFor: undefined, waitingSince: undefined }),
      ),
    ).toMatchObject({ waitingFor: null, waitingSince: null });
  });

  it("keeps each cause bex-api names and drops a word it does not know", () => {
    for (const cause of SANDBOX_START_FAILURE_CAUSES) {
      expect(
        toAgentSessionView(
          wire({
            phase: "failed",
            failureReasonCode: SANDBOX_START_FAILED,
            failureCause: cause,
          }),
        ).failureCause,
      ).toBe(cause);
    }
    expect(
      toAgentSessionView(
        wire({
          phase: "failed",
          failureReasonCode: SANDBOX_START_FAILED,
          failureCause: "solar_flare",
        }),
      ).failureCause,
    ).toBeNull();
  });

  it("names a failed start's cause, a platform issue when none is known, and nothing for any other failure", () => {
    const failed = (over: Partial<AgentSessionView>) =>
      agentSessionView({ phase: "failed", ...over });
    expect(
      sandboxStartFailureCause(
        failed({
          failureReasonCode: SANDBOX_START_FAILED,
          failureCause: "quota",
        }),
      ),
    ).toBe("quota");
    expect(
      sandboxStartFailureCause(
        failed({ failureReasonCode: SANDBOX_START_FAILED }),
      ),
    ).toBe("unknown");
    expect(
      sandboxStartFailureCause(
        failed({
          failureReasonCode: SANDBOX_CAPACITY_LIMIT,
          failureCause: "pool_full",
        }),
      ),
    ).toBeNull();
    expect(sandboxStartFailureCause(failed({}))).toBeNull();
    expect(
      sandboxStartFailureCause(
        agentSessionView({
          phase: "running",
          failureReasonCode: SANDBOX_START_FAILED,
          failureCause: "pool_full",
        }),
      ),
    ).toBeNull();
  });

  it("is never a capacity failure, so it offers no upgrade", () => {
    expect(
      isSandboxCapacityFailure(
        agentSessionView({
          phase: "failed",
          failureReasonCode: SANDBOX_START_FAILED,
          failureCause: "pool_full",
        }),
      ),
    ).toBe(false);
  });

  it("uses bex-api's start-failure code", () => {
    expect(SANDBOX_START_FAILED).toBe(
      goStringConstant(
        "lego/backend/internal/sandbox/service.go",
        "CodeSandboxStartFailed",
      ),
    );
  });

  it("says Waiting for a sandbox machine instead of Working… while it waits", () => {
    expect(
      agentSessionStatusPhraseKey({
        phase: "creating",
        prNumber: null,
        waitingFor: "sandbox_capacity",
      }),
    ).toBe("agentSessions.statusPhrase.waitingForSandbox");
    expect(
      agentSessionStatusPhraseKey({
        phase: "creating",
        prNumber: null,
        waitingFor: null,
      }),
    ).toBe("agentSessions.statusPhrase.working");
    // A settled session's own phrase wins over a stale wait.
    expect(
      agentSessionStatusPhraseKey({
        phase: "failed",
        prNumber: null,
        waitingFor: "sandbox_capacity",
      }),
    ).toBe("agentSessions.phase.failed");
  });

  it("measures the time waited from waitingSince", () => {
    const at = Date.parse(since);
    expect(
      agentSessionWaitMs(
        { waitingFor: "sandbox_capacity", waitingSince: since },
        at + 192_000,
      ),
    ).toBe(192_000);
    // A clock a little behind the server's never reads negative.
    expect(
      agentSessionWaitMs(
        { waitingFor: "sandbox_capacity", waitingSince: since },
        at - 5_000,
      ),
    ).toBe(0);
    expect(
      agentSessionWaitMs({ waitingFor: null, waitingSince: since }, at),
    ).toBeNull();
    expect(
      agentSessionWaitMs(
        { waitingFor: "sandbox_capacity", waitingSince: "not a time" },
        at,
      ),
    ).toBeNull();
    expect(formatDurationShort(42_000)).toBe("42s");
    expect(formatDurationShort(192_000)).toBe("3m 12s");
    expect(formatDurationShort(3_725_000)).toBe("1h 2m");
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

// w22/m413 t004: what bex-api says became of a stopped turn's unpushed work.
describe("toTurnRetentionView", () => {
  const unknown = { unpushedCommits: null, uncommittedChanges: null };
  const none = { state: "none", until: null, lostReason: null, ...unknown };

  it("projects each state with only what it carries", () => {
    expect(toTurnRetentionView(wireTurn({ retention: "kept" }))).toEqual({
      state: "kept",
      until: RETAINED_UNTIL,
      lostReason: null,
      ...unknown,
    });
    expect(toTurnRetentionView(wireTurn({ retention: "saving" }))).toEqual({
      state: "saving",
      until: null,
      lostReason: null,
      ...unknown,
    });
    expect(toTurnRetentionView(wireTurn({ retention: "continued" }))).toEqual({
      state: "continued",
      until: null,
      lostReason: null,
      ...unknown,
    });
    expect(
      toTurnRetentionView(
        wireTurn({ retention: "lost", retentionLostReason: "storage_full" }),
      ),
    ).toEqual({
      state: "lost",
      until: null,
      lostReason: "storage_full",
      ...unknown,
    });
  });

  it("keeps a time only for a kept disk, and a reason only for a lost one", () => {
    expect(
      toTurnRetentionView(
        wireTurn({
          retention: "lost",
          retainedUntil: RETAINED_UNTIL,
          retentionLostReason: "expired",
        }),
      ),
    ).toMatchObject({ until: null, lostReason: "expired" });
    expect(
      toTurnRetentionView(
        wireTurn({
          retention: "continued",
          retainedUntil: RETAINED_UNTIL,
          retentionLostReason: "expired",
        }),
      ),
    ).toMatchObject({ until: null, lostReason: null });
  });

  it("reads a state, or a kept disk with no time, that it cannot say as none", () => {
    for (const wire of [
      wireTurn({ retention: "none" }),
      wireTurn({ retention: "a-newer-state" }),
      wireTurn({ retention: "" }),
      wireTurn({ retention: "kept", retainedUntil: null }),
      wireTurn({ retention: "kept", retainedUntil: "" }),
    ]) {
      expect(toTurnRetentionView(wire)).toEqual(none);
    }
  });

  it("leaves the cause of a loss unnamed when it is a newer one or absent", () => {
    expect(
      toTurnRetentionView(
        wireTurn({ retention: "lost", retentionLostReason: "something-new" }),
      ),
    ).toEqual({ state: "lost", until: null, lostReason: null, ...unknown });
    expect(toTurnRetentionView(wireTurn({ retention: "lost" }))).toMatchObject({
      lostReason: null,
      ...unknown,
    });
  });

  // w10/m491: what was at stake rides every state but none.
  it("carries what was at stake for a kept or lost disk, and nothing for none", () => {
    expect(
      toTurnRetentionView(
        wireTurn({
          retention: "lost",
          retentionLostReason: "sandbox_lost",
          unpushedCommits: 3,
          uncommittedChanges: false,
        }),
      ),
    ).toEqual({
      state: "lost",
      until: null,
      lostReason: "sandbox_lost",
      unpushedCommits: 3,
      uncommittedChanges: false,
    });
    expect(
      toTurnRetentionView(
        wireTurn({
          retention: "none",
          unpushedCommits: 3,
          uncommittedChanges: true,
        }),
      ),
    ).toEqual(none);
  });

  it("knows exactly bex-api's states and loss reasons", () => {
    const path = "lego/backend/internal/agentsessions/turn_retention.go";
    expect([...TURN_RETENTION_STATES]).toEqual([
      goStringConstant(path, "RetentionNone"),
      goStringConstant(path, "RetentionSaving"),
      goStringConstant(path, "RetentionKept"),
      goStringConstant(path, "RetentionContinued"),
      goStringConstant(path, "RetentionLost"),
    ]);
    expect([...TURN_RETENTION_LOST_REASONS].sort()).toEqual(
      goStringConstantsWithPrefix(path, "RetentionLostReason").sort(),
    );
  });
});

describe("toInterruptedTurns", () => {
  it("is the stopped turns and those that kept or lost work, by number, in turn order", () => {
    const turns = toInterruptedTurns([
      wireTurn({ turn: 1, interrupted: false }),
      wireTurn({ turn: 2, retention: "kept" }),
      wireTurn({
        turn: 3,
        interrupted: false,
        retention: "kept",
        unpushedCommits: 2,
        uncommittedChanges: true,
      }),
      wireTurn({ turn: 4, retention: "saving" }),
      wireTurn({ turn: 5, interrupted: false, retention: "none" }),
    ]);
    expect([...turns.keys()]).toEqual([2, 3, 4]);
    expect(turns.get(2)).toEqual({
      interrupted: true,
      retention: {
        state: "kept",
        until: RETAINED_UNTIL,
        lostReason: null,
        unpushedCommits: null,
        uncommittedChanges: null,
      },
    });
    // w10/m491: a turn nobody stopped is noted for the work it kept, with
    // what was at stake.
    expect(turns.get(3)).toEqual({
      interrupted: false,
      retention: {
        state: "kept",
        until: RETAINED_UNTIL,
        lostReason: null,
        unpushedCommits: 2,
        uncommittedChanges: true,
      },
    });
    expect(turns.get(4)?.retention.state).toBe("saving");
  });

  it("is empty with no read yet", () => {
    expect(toInterruptedTurns(undefined).size).toBe(0);
    expect(toInterruptedTurns(null).size).toBe(0);
    expect(toInterruptedTurns([]).size).toBe(0);
  });
});
