// Session lineage helpers (ADR102): a session spawned by an agent carries its
// parent's id, and `agentSessionTree` returns the whole tree flat. These pure
// functions turn that flat list into what the three surfaces render: the
// parent's child cards, the Agents panel's status groups, and the sidebar's
// nested rows.

import type { AgentSessionView } from "@/features/agent-sessions/types";

/** The Agents panel's status buckets, in display order. */
export const AGENT_STATUS_GROUPS = [
  "working",
  "done",
  "failed",
  "canceled",
] as const;

export type AgentStatusGroup = (typeof AGENT_STATUS_GROUPS)[number];

/**
 * Buckets a session by the server-computed `state` (working/succeeded/failed/
 * canceled). A partial cache entry without `state` falls back to the phase,
 * and an unknown value reads as working — keep waiting rather than call it done.
 */
export function agentStatusGroup(
  session: Pick<AgentSessionView, "state" | "phase">,
): AgentStatusGroup {
  switch (session.state ?? session.phase) {
    case "succeeded":
    case "completed":
      return "done";
    case "failed":
      return "failed";
    case "canceled":
      return "canceled";
    default:
      return "working";
  }
}

/** Sessions in creation order (oldest first), without mutating the input. */
function byCreation(sessions: readonly AgentSessionView[]): AgentSessionView[] {
  return [...sessions].sort((a, b) => a.createdAt.localeCompare(b.createdAt));
}

/** The direct children of `parentId`, in creation order. */
export function directChildren(
  sessions: readonly AgentSessionView[],
  parentId: string,
): AgentSessionView[] {
  return byCreation(sessions.filter((s) => s.parentSessionId === parentId));
}

/** Every session below `rootId` in the flat tree, parents before children. */
export function descendantsOf(
  sessions: readonly AgentSessionView[],
  rootId: string,
): AgentSessionView[] {
  const out: AgentSessionView[] = [];
  const seen = new Set<string>([rootId]);
  let frontier = [rootId];
  while (frontier.length > 0) {
    const next: string[] = [];
    for (const id of frontier) {
      for (const child of directChildren(sessions, id)) {
        if (seen.has(child.id)) continue;
        seen.add(child.id);
        out.push(child);
        next.push(child.id);
      }
    }
    frontier = next;
  }
  return out;
}

export interface AgentStatusBucket {
  group: AgentStatusGroup;
  sessions: AgentSessionView[];
}

/** Non-empty status buckets in display order, each in input order. */
export function groupByAgentStatus(
  sessions: readonly AgentSessionView[],
): AgentStatusBucket[] {
  return AGENT_STATUS_GROUPS.flatMap((group) => {
    const members = sessions.filter((s) => agentStatusGroup(s) === group);
    return members.length > 0 ? [{ group, sessions: members }] : [];
  });
}

/**
 * The Agents panel shows only inside a lineage: when the session has children
 * or is itself a child. A top-level session with no children has no panel.
 */
export function showAgentsPanel(
  session: Pick<AgentSessionView, "parentSessionId">,
  descendants: readonly unknown[],
): boolean {
  return session.parentSessionId != null || descendants.length > 0;
}

export interface SessionNode {
  session: AgentSessionView;
  children: SessionNode[];
}

/**
 * Nests a flat list under parents. The input order is kept for top-level rows;
 * children sort oldest first (the order they were spawned). A child whose
 * parent is not in the list falls back to a top-level row.
 */
export function nestSessions(
  sessions: readonly AgentSessionView[],
): SessionNode[] {
  const nodes = new Map<string, SessionNode>(
    sessions.map((s) => [s.id, { session: s, children: [] }]),
  );
  const roots: SessionNode[] = [];
  for (const s of sessions) {
    const node = nodes.get(s.id);
    if (!node) continue;
    const parent = s.parentSessionId ? nodes.get(s.parentSessionId) : undefined;
    if (parent && parent !== node && !isAncestor(nodes, node, parent)) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }
  for (const node of nodes.values()) {
    node.children.sort((a, b) =>
      a.session.createdAt.localeCompare(b.session.createdAt),
    );
  }
  return roots;
}

/** True when `candidate` already sits above `node` (guards a cyclic payload). */
function isAncestor(
  nodes: ReadonlyMap<string, SessionNode>,
  node: SessionNode,
  candidate: SessionNode,
): boolean {
  let cursor: SessionNode | undefined = candidate;
  for (let hops = 0; cursor && hops <= nodes.size; hops++) {
    if (cursor === node) return true;
    cursor = cursor.session.parentSessionId
      ? nodes.get(cursor.session.parentSessionId)
      : undefined;
  }
  return false;
}
