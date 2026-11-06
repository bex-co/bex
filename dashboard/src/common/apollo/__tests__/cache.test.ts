import { describe, expect, it } from "vitest";
import { InMemoryCache, gql } from "@apollo/client";
import {
  apolloCacheConfig,
  mergeAgentSessionOutcome,
} from "@/common/apollo/cache";

const outcome = (over: Record<string, unknown> = {}) => ({
  __typename: "AgentSessionOutcome",
  state: "succeeded",
  turn: 1,
  steerable: true,
  summary: null,
  result: null,
  files: null,
  cost: null,
  ...over,
});

const query = gql`
  query Session($id: String!) {
    agentSession(id: $id) {
      id
      outcome {
        state
        turn
        steerable
        summary
        result
        files {
          id
        }
        cost {
          modelUSD
        }
      }
    }
  }
`;

function write(cache: InMemoryCache, out: Record<string, unknown>) {
  cache.writeQuery({
    query,
    variables: { id: "ags-1" },
    data: {
      agentSession: { __typename: "AgentSession", id: "ags-1", outcome: out },
    },
  });
}

function read(cache: InMemoryCache) {
  return cache.readQuery<{
    agentSession: { outcome: Record<string, unknown> };
  }>({ query, variables: { id: "ags-1" } })?.agentSession.outcome;
}

describe("AgentSession.outcome cache merge", () => {
  it("keeps a detail read's result when a list read of the same turn writes null", () => {
    const cache = new InMemoryCache(apolloCacheConfig);
    write(cache, outcome({ result: '{"ok":true}', summary: "done" }));
    write(cache, outcome());
    expect(read(cache)?.result).toBe('{"ok":true}');
    expect(read(cache)?.summary).toBe("done");
  });

  it("takes the incoming outcome whole when the turn or state changes", () => {
    const cache = new InMemoryCache(apolloCacheConfig);
    write(cache, outcome({ result: '{"ok":true}' }));
    write(cache, outcome({ turn: 2, state: "working" }));
    expect(read(cache)?.result).toBeNull();
    expect(
      mergeAgentSessionOutcome(
        outcome({ result: "1" }),
        outcome({ state: "failed" }),
      )?.result,
    ).toBeNull();
  });

  it("lets a newer non-null value replace the existing one", () => {
    expect(
      mergeAgentSessionOutcome(
        outcome({ result: "1" }),
        outcome({ result: "2" }),
      )?.result,
    ).toBe("2");
    expect(mergeAgentSessionOutcome(outcome(), null)).toBeNull();
  });
});

// ADR100: only the single-session read fills a paused turn's open decision.
describe("AgentSession.outcome.pendingDecision cache merge", () => {
  const decisionQuery = gql`
    query PausedSession($id: String!) {
      agentSession(id: $id) {
        id
        outcome {
          state
          turn
          pendingDecision {
            id
            state
          }
        }
      }
    }
  `;
  const decision = (state = "pending") => ({
    __typename: "AgentSessionDecision",
    id: "dcs-1",
    state,
  });
  const writePaused = (
    cache: InMemoryCache,
    state: string,
    pendingDecision: unknown,
  ) =>
    cache.writeQuery({
      query: decisionQuery,
      variables: { id: "ags-1" },
      data: {
        agentSession: {
          __typename: "AgentSession",
          id: "ags-1",
          outcome: {
            __typename: "AgentSessionOutcome",
            state,
            turn: 1,
            pendingDecision,
          },
        },
      },
    });
  const readDecision = (cache: InMemoryCache) =>
    cache.readQuery<{
      agentSession: {
        outcome: { pendingDecision: { id: string; state: string } | null };
      };
    }>({ query: decisionQuery, variables: { id: "ags-1" } })?.agentSession
      .outcome.pendingDecision;

  it("keeps the detail read's decision through a list poll of the same pause", () => {
    const cache = new InMemoryCache(apolloCacheConfig);
    writePaused(cache, "needs_input", decision());
    writePaused(cache, "needs_input", null);
    expect(readDecision(cache)).toEqual({
      __typename: "AgentSessionDecision",
      id: "dcs-1",
      state: "pending",
    });
  });

  it("reflects an answer written to the decision itself", () => {
    const cache = new InMemoryCache(apolloCacheConfig);
    writePaused(cache, "needs_input", decision());
    // The answer mutation's {id, state} lands on the normalized decision.
    cache.writeFragment({
      id: "AgentSessionDecision:dcs-1",
      fragment: gql`
        fragment Answered on AgentSessionDecision {
          state
        }
      `,
      data: { __typename: "AgentSessionDecision", state: "allowed" },
    });
    expect(readDecision(cache)?.state).toBe("allowed");
  });

  it("drops the decision once the turn is no longer paused", () => {
    const cache = new InMemoryCache(apolloCacheConfig);
    writePaused(cache, "needs_input", decision());
    writePaused(cache, "working", null);
    expect(readDecision(cache)).toBeNull();
  });
});
