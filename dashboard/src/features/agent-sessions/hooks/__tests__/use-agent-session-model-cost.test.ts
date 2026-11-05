import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentSessionModelCost } from "@/features/agent-sessions/hooks/use-agent-session-model-cost";

const refetch = vi.fn(async () => undefined);
const queryOptions = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useQuery: (_doc: unknown, options: unknown) => {
    queryOptions(options);
    return { data: undefined, loading: false, refetch };
  },
}));

// ADR091 D6: the session cost changes only with turns, so the read is not
// polled; it re-reads when a turn starts and once when the session finishes.
describe("useAgentSessionModelCost", () => {
  beforeEach(() => {
    refetch.mockClear();
    queryOptions.mockClear();
  });

  const mount = (turns: number, live: boolean) =>
    renderHook(
      (props: { turns: number; live: boolean }) =>
        useAgentSessionModelCost("ags-1", props.turns, props.live),
      { initialProps: { turns, live } },
    );

  it("never polls and does not refetch on mount", () => {
    mount(1, true);
    expect(queryOptions).toHaveBeenCalledWith(
      expect.not.objectContaining({ pollInterval: expect.anything() }),
    );
    expect(refetch).not.toHaveBeenCalled();
  });

  it("refetches when a turn starts and when the session finishes", () => {
    const { rerender } = mount(1, true);
    rerender({ turns: 1, live: true });
    expect(refetch).not.toHaveBeenCalled();
    rerender({ turns: 2, live: true });
    expect(refetch).toHaveBeenCalledTimes(1);
    rerender({ turns: 2, live: false });
    expect(refetch).toHaveBeenCalledTimes(2);
    // Finished stays finished: no further reads.
    rerender({ turns: 2, live: false });
    expect(refetch).toHaveBeenCalledTimes(2);
  });

  it("does not refetch when a finished session resumes", () => {
    const { rerender } = mount(2, false);
    rerender({ turns: 2, live: true });
    expect(refetch).not.toHaveBeenCalled();
  });
});
