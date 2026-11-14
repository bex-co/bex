import { describe, expect, it, vi } from "vitest";
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

// w9/m388: setWorkspaceIPAllowList full-replaces the rules, so a shorter saved
// list replaces the cached one outright, without Apollo's data-loss warning
// (an array field with no merge function warns when it shrinks).
describe("Workspace.ipAllowList cache merge", () => {
  const workspacesQuery = gql`
    query Workspaces {
      workspaces {
        id
        ipAllowList {
          cidrBlock
          description
        }
      }
    }
  `;
  const rule = (cidrBlock: string) => ({
    __typename: "IPAllowListEntry",
    cidrBlock,
    description: null,
  });
  const writeRules = (cache: InMemoryCache, rules: string[]) =>
    cache.writeQuery({
      query: workspacesQuery,
      data: {
        workspaces: [
          {
            __typename: "Workspace",
            id: "tea-1",
            ipAllowList: rules.map(rule),
          },
        ],
      },
    });

  it("replaces a longer list with a shorter one, silently", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      const cache = new InMemoryCache(apolloCacheConfig);
      writeRules(cache, ["0.0.0.0/0", "::/0"]);
      writeRules(cache, ["203.0.113.0/24"]);
      expect(
        cache.readQuery<{
          workspaces: Array<{ ipAllowList: Array<{ cidrBlock: string }> }>;
        }>({ query: workspacesQuery })?.workspaces[0].ipAllowList,
      ).toEqual([rule("203.0.113.0/24")]);
      expect(warn).not.toHaveBeenCalled();
    } finally {
      warn.mockRestore();
    }
  });
});
