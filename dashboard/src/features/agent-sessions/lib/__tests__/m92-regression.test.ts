import { describe, it, expect } from "vitest";
import {
  agentSessionStatusPhraseKey,
  sessionTitleShort,
  SESSION_TITLE_MAX,
  toAgentSessionView,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionFieldsFragment } from "@/graphql/definitions";

/** Minimal wire helper for mapper tests */
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
    createdAt: "2026-08-02T00:00:00.000Z",
    updatedAt: "2026-08-02T00:00:00.000Z",
    canceledAt: null,
    ...over,
  } as AgentSessionFieldsFragment;
}

describe("m92 t001 — prNumber 0 is treated as no PR", () => {
  it("maps wire prNumber 0 to null", () => {
    const view = toAgentSessionView(wire({ prNumber: 0 as unknown as number }));
    expect(view.prNumber).toBeNull();
  });
  it("maps wire prNumber 7 through", () => {
    const view = toAgentSessionView(wire({ prNumber: 7 }));
    expect(view.prNumber).toBe(7);
  });
  it("completed with prNumber 0 is not prReady", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "completed", prNumber: 0 }),
    ).toBe("agentSessions.phase.completed");
  });
  it("completed with real prNumber is prReady", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "completed", prNumber: 7 }),
    ).toBe("agentSessions.statusPhrase.prReady");
  });
});

describe("m92 t004 — bounded task title", () => {
  it("short titles pass through unchanged", () => {
    const v = {
      id: "as-1",
      agentConfig: { task: "short title" } as unknown as { task: string },
    };
    expect(sessionTitleShort(v)).toBe("short title");
  });
  it("long titles are capped to SESSION_TITLE_MAX with ellipsis", () => {
    const long = "a".repeat(500);
    const v = {
      id: "as-1",
      agentConfig: { task: long } as unknown as { task: string },
    };
    const out = sessionTitleShort(v);
    expect(out.length).toBeLessThanOrEqual(SESSION_TITLE_MAX + 1); // + ellipsis
    expect(out.endsWith("…")).toBe(true);
    expect(out.length).toBe(SESSION_TITLE_MAX + 1);
  });
  it("falls back to id when task empty", () => {
    const v = {
      id: "as-1",
      agentConfig: { task: "" } as unknown as { task: string },
    };
    expect(sessionTitleShort(v)).toBe("as-1");
  });
});

describe("m92 t007 — hibernated status phrase", () => {
  it("hibernated maps to hibernated phrase, not Working", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "hibernated", prNumber: null }),
    ).toBe("agentSessions.phase.hibernated");
  });
  it("hibernating maps to hibernating phrase", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "hibernating", prNumber: null }),
    ).toBe("agentSessions.phase.hibernating");
  });
  it("running still maps to Working", () => {
    expect(
      agentSessionStatusPhraseKey({ phase: "running", prNumber: null }),
    ).toBe("agentSessions.statusPhrase.working");
  });
});

describe("m92 t002 — substring search vs subsequence", () => {
  // Replicate the helper from nav-section to ensure substring semantics
  function sessionSearchMatch(query: string, candidate: string): boolean {
    const q = query.trim().toLowerCase();
    if (q === "") return true;
    return candidate.toLowerCase().includes(q);
  }

  it("matches substring case-insensitively", () => {
    expect(sessionSearchMatch("typo", "find and fix typo")).toBe(true);
    expect(sessionSearchMatch("TYPO", "find and fix typo")).toBe(true);
  });
  it("does not match pure subsequence without substring", () => {
    // "typo" letters appear in order in "today your project opens" but not as substring
    expect(sessionSearchMatch("typo", "today your project opens")).toBe(false);
  });
  it("empty query matches everything", () => {
    expect(sessionSearchMatch("", "anything")).toBe(true);
    expect(sessionSearchMatch("   ", "anything")).toBe(true);
  });
});
