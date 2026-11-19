import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AGENT_SESSION_ACTIVE_POLL_MS } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { useAgentSessionInterruptedTurns } from "@/features/agent-sessions/hooks/use-agent-session-interrupted-turns";
import { RETAINED_UNTIL, wireTurn } from "@/test/mocks/agent-session";

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

beforeEach(() => {
  refetch.mockClear();
  startPolling.mockClear();
  stopPolling.mockClear();
  queryOptions.mockClear();
  data = undefined;
});

const turns = (...t: Array<Parameters<typeof wireTurn>[0]>) => ({
  agentSessionTranscript: { turns: t.map((turn) => wireTurn(turn)) },
});

describe("useAgentSessionInterruptedTurns", () => {
  it("is the turns flagged interrupted", () => {
    data = turns(
      { turn: 1, interrupted: false },
      { turn: 2 },
      { turn: 3, interrupted: false },
      { turn: 4 },
    );
    const { result } = renderHook(() =>
      useAgentSessionInterruptedTurns("ags-1", "k"),
    );
    expect([...result.current.keys()]).toEqual([2, 4]);
  });

  it("carries what became of each stopped turn's unpushed work", () => {
    data = turns(
      { turn: 1 },
      { turn: 2, retention: "kept" },
      { turn: 3, retention: "lost", retentionLostReason: "storage_full" },
    );
    const { result } = renderHook(() =>
      useAgentSessionInterruptedTurns("ags-1", "k"),
    );
    const stopped = (over: object) => ({
      interrupted: true,
      retention: {
        state: "none",
        until: null,
        lostReason: null,
        unpushedCommits: null,
        uncommittedChanges: null,
        ...over,
      },
    });
    expect(result.current.get(1)).toEqual(stopped({}));
    expect(result.current.get(2)).toEqual(
      stopped({ state: "kept", until: RETAINED_UNTIL }),
    );
    expect(result.current.get(3)).toEqual(
      stopped({ state: "lost", lostReason: "storage_full" }),
    );
  });

  it("is empty before the read lands and when nothing was interrupted", () => {
    const { result, rerender } = renderHook(() =>
      useAgentSessionInterruptedTurns("ags-1", "k"),
    );
    expect(result.current.size).toBe(0);
    data = turns({ interrupted: false });
    rerender();
    expect(result.current.size).toBe(0);
  });

  it("reads once on mount, from the cache then the network, without a standing poll", () => {
    renderHook(() => useAgentSessionInterruptedTurns("ags-1", "k"));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1" },
        fetchPolicy: "cache-and-network",
      }),
    );
    expect(queryOptions.mock.lastCall?.[0]).not.toHaveProperty("pollInterval");
    expect(refetch).not.toHaveBeenCalled();
    expect(startPolling).not.toHaveBeenCalled();
  });

  it("reads again each time the settle key changes, and only then", () => {
    const { rerender } = renderHook(
      (p: { key: string }) => useAgentSessionInterruptedTurns("ags-1", p.key),
      { initialProps: { key: "2:sbx-1:false" } },
    );
    rerender({ key: "2:sbx-1:false" });
    expect(refetch).not.toHaveBeenCalled();
    // The turn settled: the flag is written by now.
    rerender({ key: "2::true" });
    expect(refetch).toHaveBeenCalledTimes(1);
    // The next message started its turn.
    rerender({ key: "3:sbx-2:false" });
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it("does not query without a session", () => {
    renderHook(() => useAgentSessionInterruptedTurns("", "k"));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ skip: true }),
    );
  });

  // The disk is saved after the turn settles, for up to a few minutes, so the
  // first read after a Stop says "saving": the hook keeps reading until the
  // save has ended, and stops once it has.
  describe("while a stopped turn's disk is being saved", () => {
    it("polls at the active cadence until the save ends, then stops", () => {
      data = turns({ retention: "saving" });
      const { result, rerender } = renderHook(() =>
        useAgentSessionInterruptedTurns("ags-1", "k"),
      );
      expect(result.current.get(1)?.retention.state).toBe("saving");
      expect(startPolling).toHaveBeenLastCalledWith(
        AGENT_SESSION_ACTIVE_POLL_MS,
      );

      data = turns({ retention: "kept" });
      stopPolling.mockClear();
      rerender();
      expect(result.current.get(1)?.retention.state).toBe("kept");
      expect(stopPolling).toHaveBeenCalled();
      expect(startPolling).toHaveBeenCalledTimes(1);
    });

    it("does not poll for a kept, lost or continued disk", () => {
      data = turns(
        { turn: 1, interrupted: false, retention: "none" },
        { turn: 2, retention: "kept" },
        { turn: 3, retention: "lost" },
        { turn: 4, retention: "continued" },
      );
      renderHook(() => useAgentSessionInterruptedTurns("ags-1", "k"));
      expect(startPolling).not.toHaveBeenCalled();
    });

    // w10/m491: a turn that failed, idled out or was followed by a message
    // saves its disk too, and its answer is read the same way.
    it("polls for a turn nobody stopped whose disk is being saved", () => {
      data = turns({ turn: 1, interrupted: false, retention: "saving" });
      renderHook(() => useAgentSessionInterruptedTurns("ags-1", "k"));
      expect(startPolling).toHaveBeenCalledTimes(1);
    });

    it("lets the poll go when the page unmounts", () => {
      data = turns({ retention: "saving" });
      const { unmount } = renderHook(() =>
        useAgentSessionInterruptedTurns("ags-1", "k"),
      );
      stopPolling.mockClear();
      unmount();
      expect(stopPolling).toHaveBeenCalled();
    });
  });
});
