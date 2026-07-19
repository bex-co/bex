import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

import { useServiceEvents } from "@/features/events/hooks/use-service-events";

function event(id: string, cursor = `cursor-${id}`) {
  return {
    __typename: "ServiceEvent" as const,
    id,
    type: "deploy_started",
    timestamp: "2026-07-18T00:00:00Z",
    cursor,
    details: null,
  };
}

const range = {
  startTime: "2026-06-18T00:00:00.000Z",
  endTime: "2026-07-18T00:00:00.000Z",
};

beforeEach(() => {
  mockUseQuery.mockReset();
});

describe("useServiceEvents", () => {
  it("sends the explicit selected range and cursor-accumulates with id deduplication", async () => {
    const first = [event("evt-1"), event("evt-2"), event("evt-3")];
    const second = [event("evt-3"), event("evt-4")];
    let data = { serviceEvents: first };
    const fetchMore = vi.fn(
      async ({
        updateQuery,
      }: {
        updateQuery: (
          previous: typeof data,
          input: { fetchMoreResult: typeof data },
        ) => typeof data;
      }) => {
        data = updateQuery(data, {
          fetchMoreResult: { serviceEvents: second },
        });
        return { data: { serviceEvents: second } };
      },
    );
    mockUseQuery.mockImplementation(() => ({
      data,
      loading: false,
      error: undefined,
      fetchMore,
      refetch: vi.fn(),
    }));

    const { result, rerender } = renderHook(() =>
      useServiceEvents("srv-web", { limit: 3, ...range }),
    );
    await waitFor(() => expect(result.current.hasMore).toBe(true));

    const [, queryOptions] = mockUseQuery.mock.calls[0] as [
      unknown,
      { variables: Record<string, unknown> },
    ];
    expect(queryOptions.variables).toEqual({
      serviceId: "srv-web",
      limit: 3,
      ...range,
    });

    await act(() => result.current.loadMore());
    rerender();

    expect(fetchMore).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: {
          serviceId: "srv-web",
          limit: 3,
          cursor: "cursor-evt-3",
          ...range,
        },
      }),
    );
    expect(result.current.events.map((item) => item.id)).toEqual([
      "evt-1",
      "evt-2",
      "evt-3",
      "evt-4",
    ]);
    expect(result.current.hasMore).toBe(false);
  });

  it("steps into the preceding bounded window instead of widening the request", async () => {
    const fetchMore = vi.fn(async () => ({
      data: { serviceEvents: [] },
    }));
    mockUseQuery.mockReturnValue({
      data: { serviceEvents: [] },
      loading: false,
      error: undefined,
      fetchMore,
      refetch: vi.fn(),
    });
    const historyStartTime = "2026-05-19T00:00:00.000Z";
    const { result } = renderHook(() =>
      useServiceEvents("srv-web", {
        limit: 20,
        ...range,
        historyStartTime,
        windowHours: 720,
      }),
    );
    await waitFor(() => expect(result.current.hasMore).toBe(true));

    await act(() => result.current.loadMore());

    expect(fetchMore).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({
          startTime: historyStartTime,
          endTime: range.startTime,
        }),
      }),
    );
  });
});
