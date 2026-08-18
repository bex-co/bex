import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RESOURCE_POLL_INTERVAL_MS } from "@/common/lib/polling";

const mockUseQuery = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
}));

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useWebhookDeliveries } from "@/features/webhooks/hooks/use-webhook-deliveries";

function attempt(id: string, sentAt: string) {
  return {
    __typename: "WebhookDelivery" as const,
    id,
    eventId: `evt-${id}`,
    eventType: "deploy_started",
    serviceId: "srv-api",
    status: "failed",
    attemptNumber: 1,
    statusCode: 500,
    transportError: "",
    responseBody: "no",
    requestBody: "{}",
    sentAt,
    nextAttemptAt: null,
    parentStatus: "pending",
    cursor: `cursor-${id}`,
  };
}

function page(prefix: string, count: number, minuteOffset = 0) {
  return Array.from({ length: count }, (_, index) =>
    attempt(
      `${prefix}-${index}`,
      new Date(Date.UTC(2026, 7, 17, 12, minuteOffset - index)).toISOString(),
    ),
  );
}

let queryData: { webhookDeliveries: ReturnType<typeof page> };
const refetch = vi.fn();

beforeEach(() => {
  mockUseQuery.mockReset();
  mockClientQuery.mockReset();
  refetch.mockReset();
  refetch.mockResolvedValue({});
  currentWorkspaceId = "tea-1";
  queryData = { webhookDeliveries: page("first", 20) };
  mockUseQuery.mockImplementation(() => ({
    data: queryData,
    loading: false,
    error: undefined,
    refetch,
  }));
});

describe("useWebhookDeliveries", () => {
  it("polls only the cursor-less newest page and skips hidden-tab ticks", () => {
    renderHook(() => useWebhookDeliveries("whk-1"));

    const [, options] = mockUseQuery.mock.calls.at(-1) as [
      unknown,
      {
        variables: { cursor?: string };
        pollInterval: number;
        skipPollAttempt: () => boolean;
      },
    ];
    expect(options.variables.cursor).toBeUndefined();
    expect(options.pollInterval).toBe(RESOURCE_POLL_INTERVAL_MS);
    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    expect(options.skipPollAttempt()).toBe(true);
    hidden.mockRestore();
  });

  it("deduplicates polling while keeping the explicitly loaded window bounded", async () => {
    const secondPage = page("second", 20, -20);
    mockClientQuery.mockResolvedValue({
      data: { webhookDeliveries: secondPage },
    });
    const { result, rerender } = renderHook(() =>
      useWebhookDeliveries("whk-1"),
    );

    await act(async () => result.current.loadMore());
    await waitFor(() => expect(result.current.deliveries).toHaveLength(40));

    const unchanged = result.current.deliveries[0];
    const displaced = queryData.webhookDeliveries[19].id;
    queryData = {
      webhookDeliveries: [
        attempt("new-resend", "2026-08-17T12:01:00Z"),
        ...queryData.webhookDeliveries.slice(0, 19),
      ],
    };
    rerender();

    await waitFor(() => expect(result.current.deliveries).toHaveLength(40));
    const ids = result.current.deliveries.map((row) => row.id);
    expect(ids[0]).toBe("new-resend");
    expect(ids).toContain(displaced);
    expect(new Set(ids).size).toBe(ids.length);
    expect(
      result.current.deliveries.find((row) => row.id === unchanged.id),
    ).toBe(unchanged);

    await act(async () => result.current.refresh());
    expect(refetch).toHaveBeenCalledTimes(1);
    expect(result.current.deliveries).toHaveLength(40);
  });

  it("keeps polling bounded to the newest page until older pages are loaded", async () => {
    const { result, rerender } = renderHook(() =>
      useWebhookDeliveries("whk-1"),
    );

    const displaced = queryData.webhookDeliveries[19].id;
    queryData = {
      webhookDeliveries: [
        attempt("new-resend", "2026-08-17T12:01:00Z"),
        ...queryData.webhookDeliveries.slice(0, 19),
      ],
    };
    rerender();

    await waitFor(() => expect(result.current.deliveries).toHaveLength(20));
    expect(result.current.deliveries.map((row) => row.id)).not.toContain(
      displaced,
    );
  });

  it("drops retained pages when endpoint, owner, or filters change", async () => {
    mockClientQuery.mockResolvedValue({
      data: { webhookDeliveries: page("older", 20, -20) },
    });
    let endpointId = "whk-1";
    let filter = {};
    const { result, rerender } = renderHook(() =>
      useWebhookDeliveries(endpointId, filter),
    );
    await act(async () => result.current.loadMore());
    await waitFor(() => expect(result.current.deliveries).toHaveLength(40));

    endpointId = "whk-2";
    filter = {
      status: "failed" as const,
      sentAfter: "2026-08-01T00:00:00Z",
      sentBefore: "2026-08-18T00:00:00Z",
    };
    queryData = {
      webhookDeliveries: [attempt("new-endpoint", "2026-08-17T12:02:00Z")],
    };
    rerender();

    await waitFor(() =>
      expect(result.current.deliveries.map((row) => row.id)).toEqual([
        "new-endpoint",
      ]),
    );
    expect(mockUseQuery.mock.calls.at(-1)?.[1].variables).toMatchObject({
      endpointId: "whk-2",
      ownerId: "tea-1",
      ...filter,
    });

    currentWorkspaceId = "tea-2";
    queryData = {
      webhookDeliveries: [attempt("new-owner", "2026-08-17T12:03:00Z")],
    };
    rerender();

    await waitFor(() =>
      expect(result.current.deliveries.map((row) => row.id)).toEqual([
        "new-owner",
      ]),
    );
    expect(mockUseQuery.mock.calls.at(-1)?.[1].variables).toMatchObject({
      endpointId: "whk-2",
      ownerId: "tea-2",
      ...filter,
    });
  });

  it("keeps status and date bounds on both newest-page and load-more reads", async () => {
    queryData = { webhookDeliveries: page("failed", 20) };
    mockClientQuery.mockResolvedValue({ data: { webhookDeliveries: [] } });
    const filter = {
      status: "failed" as const,
      sentAfter: "2026-08-01T00:00:00Z",
      sentBefore: "2026-08-18T00:00:00Z",
    };
    const { result } = renderHook(() => useWebhookDeliveries("whk-1", filter));

    const [, newestOptions] = mockUseQuery.mock.calls.at(-1) as [
      unknown,
      { variables: Record<string, unknown> },
    ];
    expect(newestOptions.variables).toMatchObject(filter);

    await act(async () => result.current.loadMore());
    expect(mockClientQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining(filter),
      }),
    );
  });
});
