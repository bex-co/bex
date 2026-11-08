import { describe, it, expect } from "vitest";
import {
  agentStatusGroup,
  descendantsOf,
  directChildren,
  groupByAgentStatus,
  nestSessions,
  showAgentsPanel,
} from "@/features/agent-sessions/lib/lineage";
import { toAgentSessionView } from "@/features/agent-sessions/lib/mapper";
import { agentSessionView } from "@/test/mocks/agent-session";
import type { AgentSessionFieldsFragment } from "@/graphql/definitions";

/** Creation time follows the trailing digit of the id: c1 before c2. */
function s(
  id: string,
  parent: string | null,
  over: Parameters<typeof agentSessionView>[0] = {},
) {
  const rank = Number(id.slice(-1));
  return agentSessionView({
    id,
    parentSessionId: parent,
    createdAt: `2026-11-06T00:00:0${Number.isNaN(rank) ? 0 : rank}Z`,
    ...over,
  });
}

describe("agentStatusGroup", () => {
  it("maps the server state, falling back to the phase", () => {
    expect(agentStatusGroup({ state: "working", phase: "running" })).toBe(
      "working",
    );
    expect(agentStatusGroup({ state: "succeeded", phase: "completed" })).toBe(
      "done",
    );
    expect(agentStatusGroup({ state: "failed", phase: "failed" })).toBe(
      "failed",
    );
    expect(agentStatusGroup({ state: "canceled", phase: "canceled" })).toBe(
      "canceled",
    );
    expect(agentStatusGroup({ state: null, phase: "completed" })).toBe("done");
    expect(agentStatusGroup({ state: null, phase: "canceling" })).toBe(
      "working",
    );
    expect(agentStatusGroup({ state: "mystery", phase: "running" })).toBe(
      "working",
    );
  });
});

describe("groupByAgentStatus", () => {
  it("orders Working, Done, Failed, Canceled and drops empty groups", () => {
    const list = [
      s("c1", "p", { state: "failed" }),
      s("c2", "p", { state: "working" }),
      s("c3", "p", { state: "succeeded" }),
      s("c4", "p", { state: "succeeded" }),
    ];
    const grouped = groupByAgentStatus(list);
    expect(grouped.map((b) => [b.group, b.sessions.length])).toEqual([
      ["working", 1],
      ["done", 2],
      ["failed", 1],
    ]);
  });
});

describe("tree selectors", () => {
  const tree = [
    s("p", null),
    s("c2", "p"),
    s("c1", "p"),
    s("g1", "c1"),
    s("other", null),
  ];

  it("lists direct children in creation order", () => {
    expect(directChildren(tree, "p").map((x) => x.id)).toEqual(["c1", "c2"]);
  });

  it("collects every descendant, not the session itself or other trees", () => {
    expect(descendantsOf(tree, "p").map((x) => x.id)).toEqual([
      "c1",
      "c2",
      "g1",
    ]);
    expect(descendantsOf(tree, "g1")).toEqual([]);
  });

  it("shows the panel only for a parent or a child", () => {
    const [root, , , grandchild] = tree;
    expect(showAgentsPanel(root!, descendantsOf(tree, "p"))).toBe(true);
    expect(showAgentsPanel(grandchild!, [])).toBe(true);
    expect(showAgentsPanel(tree[4]!, [])).toBe(false);
  });
});

describe("nestSessions", () => {
  it("nests two children under their parent in creation order", () => {
    const nodes = nestSessions([s("c2", "p"), s("p", null), s("c1", "p")]);
    expect(nodes.map((n) => n.session.id)).toEqual(["p"]);
    expect(nodes[0]!.children.map((n) => n.session.id)).toEqual(["c1", "c2"]);
  });

  it("falls back to a top-level row when the parent is not loaded", () => {
    const nodes = nestSessions([s("c1", "missing"), s("a", null)]);
    expect(nodes.map((n) => n.session.id)).toEqual(["c1", "a"]);
  });

  it("does not loop on a cyclic payload", () => {
    const nodes = nestSessions([s("a", "b"), s("b", "a")]);
    expect(nodes.length).toBeGreaterThan(0);
  });
});

describe("mapper lineage fields", () => {
  const wire = (over: Partial<AgentSessionFieldsFragment>) =>
    ({
      id: "ags-1",
      ownerId: "tea-1",
      repo: "",
      branch: "",
      phase: "running",
      status: "running",
      state: "working",
      steerable: false,
      agentConfig: null,
      createdAt: "2026-11-06T00:00:00Z",
      updatedAt: "2026-11-06T00:00:00Z",
      ...over,
    }) as unknown as AgentSessionFieldsFragment;

  it("maps parent, root and depth", () => {
    const v = toAgentSessionView(
      wire({ parentSessionId: "ags-0", rootSessionId: "ags-r", depth: 2 }),
    );
    expect(v).toMatchObject({
      parentSessionId: "ags-0",
      rootSessionId: "ags-r",
      depth: 2,
    });
  });

  it("normalizes a top-level session", () => {
    const v = toAgentSessionView(
      wire({ parentSessionId: null, rootSessionId: null, depth: 0 }),
    );
    expect(v).toMatchObject({
      parentSessionId: null,
      rootSessionId: null,
      depth: 0,
    });
  });
});
