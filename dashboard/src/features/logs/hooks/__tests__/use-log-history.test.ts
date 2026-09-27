import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useLogHistory } from "../use-log-history";
import { EMPTY_LOG_FILTERS, type LogFilters } from "../../types";

const mockUseQuery = vi.fn();
const mockClientQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
  useApolloClient: () => ({ query: mockClientQuery }),
}));

const entry = (timestamp: string, message: string) => ({
  __typename: "LogEntry",
  timestamp,
  message,
  type: "app",
  instance: "pod-1",
  level: null,
  method: null,
  statusCode: null,
});

function envelope(
  logs: ReturnType<typeof entry>[],
  hasMore: boolean,
  nextEndTime = "",
) {
  return {
    logs: {
      __typename: "LogList",
      hasMore,
      nextStartTime: "2026-09-26T00:00:00Z",
      nextEndTime,
      logs,
    },
  };
}

const WINDOW = {
  startTime: "2026-09-26T00:00:00Z",
  endTime: "2026-09-26T01:00:00Z",
};

// The first page, stable per test the way Apollo's cache hands it back.
let firstPage: ReturnType<typeof envelope> | undefined;

beforeEach(() => {
  mockUseQuery.mockReset();
  mockClientQuery.mockReset();
  firstPage = envelope(
    [entry("2026-09-26T00:50:00Z", "b"), entry("2026-09-26T00:51:00Z", "c")],
    true,
    "2026-09-26T00:50:00Z",
  );
  mockUseQuery.mockImplementation(() => ({
    data: firstPage,
    loading: false,
    error: undefined,
  }));
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe("useLogHistory paging (w4/m107, w4/m136)", () => {
  it("sends only window/text/instance for a datastore's filters", () => {
    const filters: LogFilters = {
      ...EMPTY_LOG_FILTERS,
      text: "restart",
      instance: "red-x-0",
    };
    renderHook(() => useLogHistory("red-x", filters, WINDOW));
    const variables = mockUseQuery.mock.calls.at(-1)?.[1].variables;
    // No service-only filter reaches a red-/dpg- resource: every one is absent.
    expect(
      Object.fromEntries(
        Object.entries(variables).filter(([, v]) => v !== undefined),
      ),
    ).toEqual({
      resource: "red-x",
      text: "restart",
      instance: ["red-x-0"],
      ...WINDOW,
      limit: 100,
    });
  });

  it("prepends older pages at the cursor without a seam duplicate, until hasMore is false", async () => {
    // The older page repeats the first page's oldest line at the seam.
    mockClientQuery.mockResolvedValueOnce({
      data: envelope(
        [
          entry("2026-09-26T00:10:00Z", "a"),
          entry("2026-09-26T00:50:00Z", "b"),
        ],
        false,
      ),
    });
    const { result } = renderHook(() =>
      useLogHistory("dpg-x", EMPTY_LOG_FILTERS, WINDOW),
    );
    expect(result.current.hasMore).toBe(true);

    await act(async () => result.current.loadOlder());

    expect(mockClientQuery.mock.calls[0][0].variables).toMatchObject({
      startTime: "2026-09-26T00:00:00Z",
      endTime: "2026-09-26T00:50:00Z",
    });
    expect(result.current.lines.map((l) => l.message)).toEqual(["a", "b", "c"]);
    expect(result.current.hasMore).toBe(false);

    // Exhausted: a further scroll-to-top issues nothing.
    act(() => result.current.loadOlder());
    expect(mockClientQuery).toHaveBeenCalledTimes(1);
  });

  it("drops older pages and discards an in-flight one when the filters change", async () => {
    const pending = deferred<unknown>();
    mockClientQuery.mockReturnValueOnce(pending.promise);
    const { result, rerender } = renderHook(
      ({ text }) =>
        useLogHistory("dpg-x", { ...EMPTY_LOG_FILTERS, text }, WINDOW),
      { initialProps: { text: "" } },
    );

    act(() => result.current.loadOlder());
    expect(result.current.loadingOlder).toBe(true);

    firstPage = envelope([entry("2026-09-26T00:55:00Z", "match")], false);
    rerender({ text: "match" });
    await act(async () => {
      pending.resolve({
        data: envelope([entry("2026-09-26T00:01:00Z", "stale")], true, "x"),
      });
    });

    expect(result.current.lines.map((l) => l.message)).toEqual(["match"]);
    expect(result.current.hasMore).toBe(false);
    expect(result.current.loadingOlder).toBe(false);
  });

  it("keeps its older cursor when the first page is re-fetched after paging back", async () => {
    mockClientQuery.mockResolvedValueOnce({
      data: envelope(
        [entry("2026-09-26T00:20:00Z", "a")],
        true,
        "2026-09-26T00:20:00Z",
      ),
    });
    const { result, rerender } = renderHook(() =>
      useLogHistory("dpg-x", EMPTY_LOG_FILTERS, WINDOW),
    );
    await act(async () => result.current.loadOlder());

    // A poll/cache refresh hands back a new first-page object.
    firstPage = envelope(
      [entry("2026-09-26T00:51:00Z", "c")],
      true,
      "2026-09-26T00:51:00Z",
    );
    rerender();

    mockClientQuery.mockResolvedValueOnce({ data: envelope([], false) });
    await act(async () => result.current.loadOlder());
    expect(mockClientQuery.mock.calls[1][0].variables.endTime).toBe(
      "2026-09-26T00:20:00Z",
    );
  });
});
