import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useKeyValue } from "@/features/keyvalue/hooks/use-key-value";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

/** A settled store: the baseline poll cadence applies until refresh is asked. */
function settledKeyValue() {
  return {
    keyValue: {
      __typename: "KeyValue",
      id: "kv",
      name: "kv",
      plan: "free",
      version: "8",
      status: "available",
      suspended: "not_suspended",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: null,
      externalHost: null,
      public: false,
      region: null,
    },
  };
}

describe("useKeyValue", () => {
  it("starts polling immediately after a lifecycle mutation refetch, before the refetch resolves (w1/m76)", () => {
    // A refetch that never resolves within the test: polling must not wait on it.
    const refetch = vi.fn(() => new Promise(() => {}));
    const startPolling = vi.fn();
    const stopPolling = vi.fn();
    mockUseQuery.mockReturnValue({
      data: settledKeyValue(),
      loading: false,
      error: undefined,
      refetch,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useKeyValue("kv"));
    startPolling.mockClear();

    act(() => result.current.refetch());

    expect(startPolling).toHaveBeenCalledWith(3000);
    expect(refetch).toHaveBeenCalledOnce();
    // Eager ordering: the 3s poll begins BEFORE the refetch is even issued, so
    // a stale "available" response cannot mask the upcoming transition.
    expect(startPolling.mock.invocationCallOrder[0]).toBeLessThan(
      refetch.mock.invocationCallOrder[0],
    );
  });
});
