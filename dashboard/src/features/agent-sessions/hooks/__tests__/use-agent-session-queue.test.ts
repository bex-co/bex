import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  canHoldQueue,
  useAgentSessionQueue,
} from "@/features/agent-sessions/hooks/use-agent-session-queue";
import type { AgentSessionPhase } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

// The queue's own read (ADR047 D12): polled at the session's active cadence while
// a turn runs or a message waits, read again when the session settles.

const refetch = vi.fn(async () => undefined);
const startPolling = vi.fn();
const stopPolling = vi.fn();
const queryOptions = vi.fn();
let data: unknown;

vi.mock("@apollo/client/react", () => ({
  useQuery: (_doc: unknown, options: unknown) => {
    queryOptions(options);
    return { data, refetch, startPolling, stopPolling };
  },
}));

const queued = (id: string, position: number) => ({
  __typename: "AgentSessionQueuedMessage",
  id,
  sessionId: "as-1",
  position,
  prompt: `prompt ${id}`,
  author: "alice",
  attachmentIds: null,
  blockedReason: null,
  createdAt: null,
});

const queueData = (...ids: string[]) => ({
  agentSession: {
    id: "as-1",
    pendingMessages: ids.length,
    queuedMessages: ids.map((id, i) => queued(id, i + 1)),
  },
});

const session = (phase: AgentSessionPhase, over = {}) =>
  agentSessionView({ phase, ...over });

beforeEach(() => {
  refetch.mockClear();
  startPolling.mockClear();
  stopPolling.mockClear();
  queryOptions.mockClear();
  data = undefined;
});

describe("useAgentSessionQueue", () => {
  it("maps the cached queue, next first", () => {
    data = queueData("aqm-1", "aqm-2");
    const { result } = renderHook(() =>
      useAgentSessionQueue(session("running")),
    );
    expect(result.current.map((m) => [m.id, m.position])).toEqual([
      ["aqm-1", 1],
      ["aqm-2", 2],
    ]);
  });

  it("is empty before the read lands", () => {
    const { result } = renderHook(() =>
      useAgentSessionQueue(session("running")),
    );
    expect(result.current).toEqual([]);
  });

  it("reads the session's own queue, from the cache then the network", () => {
    renderHook(() => useAgentSessionQueue(session("running", { id: "as-9" })));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({
        variables: { id: "as-9" },
        fetchPolicy: "cache-and-network",
        skip: false,
      }),
    );
  });

  it("polls at the active cadence while a turn runs", () => {
    renderHook(() => useAgentSessionQueue(session("running")));
    expect(startPolling).toHaveBeenLastCalledWith(5000);
  });

  it("keeps polling an idle session while messages wait, then stops", () => {
    data = queueData("aqm-1");
    const { rerender } = renderHook(() =>
      useAgentSessionQueue(session("completed")),
    );
    expect(startPolling).toHaveBeenLastCalledWith(5000);

    data = queueData();
    rerender();
    expect(stopPolling).toHaveBeenCalled();
  });

  it("reads an idle session with nothing queued once, without polling", () => {
    renderHook(() => useAgentSessionQueue(session("completed")));
    expect(startPolling).not.toHaveBeenCalled();
    expect(stopPolling).toHaveBeenCalled();
  });

  it("reads again when the session's phase changes, and when its turn count does", () => {
    const { rerender } = renderHook(
      (p: { phase: AgentSessionPhase; turns: number }) =>
        useAgentSessionQueue(session(p.phase, { turns: p.turns })),
      { initialProps: { phase: "running" as AgentSessionPhase, turns: 1 } },
    );
    rerender({ phase: "running", turns: 1 });
    expect(refetch).not.toHaveBeenCalled();

    rerender({ phase: "completed", turns: 1 });
    expect(refetch).toHaveBeenCalledTimes(1);
    // The dispatcher started the next message: the turn count moves.
    rerender({ phase: "running", turns: 2 });
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it.each(["canceling", "canceled"] as const)(
    "does not read a %s session's queue: cancel clears it",
    (phase) => {
      data = queueData("aqm-1");
      const { result } = renderHook(() => useAgentSessionQueue(session(phase)));
      expect(queryOptions).toHaveBeenLastCalledWith(
        expect.objectContaining({ skip: true }),
      );
      expect(result.current).toEqual([]);
      expect(startPolling).not.toHaveBeenCalled();
    },
  );

  it("does not read an archived session's queue", () => {
    renderHook(() =>
      useAgentSessionQueue(
        session("completed", {
          archivedAt: "2026-11-14T00:00:00Z",
          isArchived: true,
        }),
      ),
    );
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ skip: true }),
    );
  });
});

describe("canHoldQueue", () => {
  it.each([
    ["creating", true],
    ["running", true],
    ["resuming", true],
    ["redispatching", true],
    ["completed", true],
    ["failed", true],
    ["canceling", false],
    ["canceled", false],
  ] as const)("%s -> %s", (phase, expected) => {
    expect(canHoldQueue({ phase, isArchived: false })).toBe(expected);
  });
});
