import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentSessionTurnResult } from "@/features/agent-sessions/hooks/use-agent-session-turn-result";

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

const turns = (...t: object[]) => ({
  agentSessionTranscript: { turns: t },
});

describe("useAgentSessionTurnResult", () => {
  it("polls only while the session is live", () => {
    renderHook(() => useAgentSessionTurnResult("ags-1", true));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ pollInterval: 5000 }),
    );
    renderHook(() => useAgentSessionTurnResult("ags-1", false));
    expect(queryOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ pollInterval: 0 }),
    );
  });

  it("reads the latest turn's result and finality", () => {
    data = turns(
      { turn: 1, result: '{"a":1}', resultFinal: true },
      { turn: 2, result: '{"a":2}', resultFinal: false },
    );
    const { result } = renderHook(() =>
      useAgentSessionTurnResult("ags-1", true),
    );
    expect(result.current).toEqual({
      turn: 2,
      result: '{"a":2}',
      final: false,
    });
  });

  it("is null when the latest turn saved nothing", () => {
    data = turns({ turn: 2, result: null, resultFinal: false });
    const { result } = renderHook(() =>
      useAgentSessionTurnResult("ags-1", true),
    );
    expect(result.current).toBeNull();
  });

  it("re-reads once when the session settles", () => {
    const { rerender } = renderHook(
      (p: { live: boolean }) => useAgentSessionTurnResult("ags-1", p.live),
      { initialProps: { live: true } },
    );
    expect(refetch).not.toHaveBeenCalled();
    rerender({ live: false });
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
