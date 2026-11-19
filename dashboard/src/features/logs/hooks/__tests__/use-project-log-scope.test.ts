import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useLogHistory } from "../use-log-history";
import { subscribeUrl, useLiveLogs } from "../use-live-logs";
import { EMPTY_LOG_FILTERS, type ProjectLogScope } from "../../types";
import { FakeEventSource } from "@/test/mocks/event-source";

// A project observability view's read (w3/m505): the history query and the
// tail name the project and the view's filter document instead of a resource,
// and the server's resource list and per-line resource reach the viewer.

const mockUseQuery = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
}));

const SCOPE: ProjectLogScope = {
  project: "prj-observed",
  filterDocument: {
    filters: [{ field: "ENVIRONMENT", operator: "IS", values: ["evm-prod"] }],
    version: 1,
  },
};
const WINDOW = {
  startTime: "2026-11-19T00:00:00Z",
  endTime: "2026-11-19T01:00:00Z",
};

const page = {
  logs: {
    __typename: "LogList",
    hasMore: true,
    nextStartTime: "2026-11-19T00:00:00Z",
    nextEndTime: "2026-11-19T00:30:00Z",
    logs: [
      {
        __typename: "LogEntry",
        id: "a",
        timestamp: "2026-11-19T00:40:00Z",
        message: "checkpoint complete",
        resource: "dpg-orders",
        type: "postgres",
        instance: "dpg-orders-1",
        level: null,
        method: null,
        statusCode: null,
        host: null,
        path: null,
      },
    ],
    resources: [
      {
        __typename: "LogResource",
        id: "srv-web",
        name: "storefront",
        type: "web_service",
        environmentId: "evm-prod",
      },
      {
        __typename: "LogResource",
        id: "dpg-orders",
        name: "orders-db",
        type: "postgres",
        environmentId: "evm-prod",
      },
    ],
  },
};

describe("useLogHistory with a project scope", () => {
  beforeEach(() => {
    mockUseQuery.mockReset();
    mockClientQuery.mockReset();
    mockUseQuery.mockImplementation(() => ({
      data: page,
      loading: false,
      error: undefined,
    }));
  });

  it("asks for the project and its filter document, never a resource", () => {
    const { result } = renderHook(() =>
      useLogHistory(
        "",
        { ...EMPTY_LOG_FILTERS, type: "app" },
        WINDOW,
        undefined,
        undefined,
        SCOPE,
      ),
    );
    const variables = mockUseQuery.mock.calls.at(-1)?.[1].variables;
    expect(variables).toMatchObject({
      project: "prj-observed",
      filterDocument: SCOPE.filterDocument,
      type: "app",
    });
    expect(variables).not.toHaveProperty("resource");
    expect(result.current.lines.map((l) => [l.resource, l.message])).toEqual([
      ["dpg-orders", "checkpoint complete"],
    ]);
    expect(result.current.resources).toMatchObject([
      {
        id: "srv-web",
        name: "storefront",
        type: "web_service",
        environmentId: "evm-prod",
      },
      {
        id: "dpg-orders",
        name: "orders-db",
        type: "postgres",
        environmentId: "evm-prod",
      },
    ]);
  });

  it("pages back over the same project scope", async () => {
    mockClientQuery.mockResolvedValueOnce({
      data: { logs: { ...page.logs, hasMore: false, logs: [] } },
    });
    const { result } = renderHook(() =>
      useLogHistory("", EMPTY_LOG_FILTERS, WINDOW, undefined, undefined, SCOPE),
    );
    await act(async () => result.current.loadOlder());
    expect(mockClientQuery.mock.calls[0][0].variables).toMatchObject({
      project: "prj-observed",
      filterDocument: SCOPE.filterDocument,
      endTime: "2026-11-19T00:30:00Z",
    });
  });

  it("keeps a resource read exactly as before: no project keys, no resource list", () => {
    const { result } = renderHook(() =>
      useLogHistory("srv-web", EMPTY_LOG_FILTERS, WINDOW),
    );
    const variables = mockUseQuery.mock.calls.at(-1)?.[1].variables;
    expect(variables.resource).toBe("srv-web");
    expect(variables).not.toHaveProperty("project");
    expect(variables).not.toHaveProperty("filterDocument");
    expect(result.current.resources).toBeUndefined();
  });
});

describe("useLiveLogs with a project scope", () => {
  let last: FakeEventSource | null = null;
  const factory = (url: string) => {
    last = new FakeEventSource(url);
    return last;
  };

  beforeEach(() => {
    last = null;
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("subscribes to the project view and labels each line with its resource", () => {
    const { result } = renderHook(() =>
      useLiveLogs({
        resource: "",
        project: SCOPE,
        enabled: true,
        type: "request",
        text: "",
        instance: "",
        createEventSource: factory,
      }),
    );
    const url = new URL(last!.url, "http://dashboard.test");
    expect(url.pathname).toMatch(/\/v1\/logs\/subscribe$/);
    expect(url.searchParams.get("project")).toBe("prj-observed");
    expect(JSON.parse(url.searchParams.get("filterDocument")!)).toEqual(
      SCOPE.filterDocument,
    );
    expect(url.searchParams.get("type")).toBe("request");
    expect(url.searchParams.has("resource")).toBe(false);

    act(() => {
      last!.frame({
        message: "GET /",
        timestamp: "2026-11-19T00:41:00Z",
        labels: [
          { name: "resource", value: "srv-web" },
          { name: "type", value: "request" },
        ],
      });
    });
    act(() => vi.advanceTimersByTime(100));
    expect(result.current.lines.map((l) => l.resource)).toEqual(["srv-web"]);
  });

  it("reopens when the view's filter document changes", () => {
    const { rerender } = renderHook(
      (scope: ProjectLogScope) =>
        useLiveLogs({
          resource: "",
          project: scope,
          enabled: true,
          type: "app",
          text: "",
          instance: "",
          createEventSource: factory,
        }),
      { initialProps: SCOPE },
    );
    const first = last;
    rerender({ ...SCOPE, filterDocument: { filters: [], version: 1 } });
    expect(last).not.toBe(first);
    expect(first?.closed).toBe(true);
  });

  it("keeps a resource tail's URL unchanged", () => {
    const url = subscribeUrl({
      resource: "srv-web",
      type: "all",
      text: "",
      textMode: undefined,
      instance: "",
      startTime: "",
      requestFilters: {
        level: "",
        method: "",
        statusCode: "",
        path: "",
        host: "",
      },
    });
    expect(url).toMatch(/\/v1\/logs\/subscribe\?resource=srv-web$/);
  });
});
