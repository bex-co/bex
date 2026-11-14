import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentSessionInterruptedTurns } from "@/features/agent-sessions/hooks/use-agent-session-interrupted-turns";

const refetch = vi.fn(async () => undefined);
const queryOptions = vi.fn();
let data: unknown;

vi.mock("@apollo/client/react", () => ({
  useQuery: (_doc: unknown, options: unknown) => {
    queryOptions(options);
    return { data, refetch };
  },
}));

beforeEach(() => {
  refetch.mockClear();
  queryOptions.mockClear();
  data = undefined;
});

const turns = (...t: Array<{ turn: number; interrupted: boolean }>) => ({
  agentSessionTranscript: { turns: t },
});

describe("useAgentSessionInterruptedTurns", () => {
  it("is the set of turns flagged interrupted", () => {
    data = turns(
      { turn: 1, interrupted: false },
      { turn: 2, interrupted: true },
      { turn: 3, interrupted: false },
      { turn: 4, interrupted: true },
    );
    const { result } = renderHook(() =>
      useAgentSessionInterruptedTurns("ags-1", "k"),
    );
    expect([...result.current]).toEqual([2, 4]);
  });

  it("is empty before the read lands and when nothing was interrupted", () => {
    const { result, rerender } = renderHook(() =>
      useAgentSessionInterruptedTurns("ags-1", "k"),
    );
    expect(result.current.size).toBe(0);
    data = turns({ turn: 1, interrupted: false });
    rerender();
    expect(result.current.size).toBe(0);
  });

  it("reads once on mount, from the cache then the network, without polling", () => {
    renderHook(() => useAgentSessionInterruptedTurns("ags-1", "k"));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1" },
        fetchPolicy: "cache-and-network",
      }),
    );
    expect(queryOptions.mock.lastCall?.[0]).not.toHaveProperty("pollInterval");
    expect(refetch).not.toHaveBeenCalled();
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
});
