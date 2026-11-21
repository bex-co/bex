import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import {
  act,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { useState } from "react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import userEvent from "@testing-library/user-event";
import { LogViewer } from "../log-viewer";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import { RANGE_PRESETS, type RangeBound } from "@/features/metrics/lib/range";
import { EMPTY_LOG_FILTERS, type LogLine } from "../../types";
import {
  DEFAULT_LOG_RANGE,
  LIVE_TAIL,
  type LogRange,
} from "../../lib/log-range";
import { setupVirtualGeometry } from "@/test/virtual-geometry";
import {
  explorerThemeClasses,
  LOG_THEME_CASES,
  LogThemeContext,
} from "@/test/log-theme";

// Drive the viewer's states by stubbing its data layer, mirroring the
// services.$serviceId routing test's approach.
const historyState: UseLogHistoryResult = {
  lines: [],
  loading: false,
  error: undefined,
  storeUnavailable: false,
  timedOut: false,
  hasMore: false,
  loadingOlder: false,
  loadOlder: () => undefined,
  resources: undefined,
  retry: () => undefined,
};
const useHistorySpy = vi.fn();
vi.mock("../../hooks/use-log-history", () => ({
  useLogHistory: (...args: unknown[]) => {
    useHistorySpy(...args);
    return historyState;
  },
}));
const useLiveLogsSpy = vi.fn();
const liveState: { lines: LogLine[] } = { lines: [] };
// The paused view's new-line watch (w3/m528): what it reports, and what it
// was asked to watch.
const watchState = { available: false, dismiss: vi.fn() };
const useNewLogsWatchSpy = vi.fn();
vi.mock("../../hooks/use-live-logs", () => ({
  useLiveLogs: (...args: unknown[]) => {
    useLiveLogsSpy(...args);
    return { lines: liveState.lines, status: "idle" };
  },
  useNewLogsWatch: (...args: unknown[]) => {
    useNewLogsWatchSpy(...args);
    return watchState;
  },
}));
// The plan notice links to Billing, and Options' Theme Settings to account
// Settings (w3/m528); render the router Link as its anchor.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));
// The plan's log window (w9/m389) is its own query; the reader tests keep the
// picker unbounded, and the bound test sets one.
const retention: { bound: RangeBound | undefined } = { bound: undefined };
vi.mock("../../hooks/use-log-retention", () => ({
  useLogRetentionBound: () => retention.bound,
}));
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelDiscovery: () => ({ values: [], resolved: false }),
  useLogLabelValues: () => [],
}));

// The viewer embeds the virtualized LogLineList (w9/m83): give jsdom the layout
// geometry the virtualizer needs, or the rendered rows never mount.
setupVirtualGeometry();

// The route owns the range (the URL's `range`, w3/m537); this holds it the
// same way, so the picker, New logs in Live Tail and the empty state can
// change it.
function Viewer({
  initial = DEFAULT_LOG_RANGE,
  ...props
}: Omit<Parameters<typeof LogViewer>[0], "range" | "onRangeChange"> & {
  initial?: LogRange;
}) {
  const [range, setRange] = useState(initial);
  return <LogViewer {...props} range={range} onRangeChange={setRange} />;
}

const rangePicker = () =>
  screen.getByRole("combobox", { name: "Log history range" });

beforeEach(() => {
  historyState.lines = [];
  historyState.loading = false;
  historyState.error = undefined;
  historyState.storeUnavailable = false;
  historyState.timedOut = false;
  historyState.hasMore = false;
  historyState.loadingOlder = false;
  historyState.loadOlder = () => undefined;
  historyState.retry = () => undefined;
  liveState.lines = [];
  useHistorySpy.mockReset();
  useLiveLogsSpy.mockReset();
});

describe("LogViewer store-unavailable state (w5/008)", () => {
  it("shows the explanatory 'needs the log store' state on a 503", () => {
    historyState.storeUnavailable = true;
    render(<LogViewer resource="web" />);
    expect(
      screen.getByText("Request logs need the log store"),
    ).toBeInTheDocument();
    // Not the generic error title.
    expect(screen.queryByText("Couldn't load logs")).not.toBeInTheDocument();
  });

  it("shows the generic error state for a non-store error, with a retry", async () => {
    const retry = vi.fn();
    historyState.error = new Error("upstream exploded");
    historyState.retry = retry;
    render(<LogViewer resource="web" />);
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load logs");
    expect(
      screen.getByText(
        "The request for this range failed. Try again, or pick a shorter range.",
      ),
    ).toBeInTheDocument();
    // A read that never got an answer shows no raw transport text (w5/167).
    expect(screen.queryByText("upstream exploded")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("shows the Pro-plan notice, not a load failure, when request logs are plan-refused (w8/m138)", () => {
    historyState.error = new CombinedGraphQLErrors({
      errors: [
        {
          message:
            "bad request: type: HTTP request logs require the Pro plan or higher; this workspace is on Hobby",
          extensions: {
            code: "PLAN_FEATURE_REQUIRED",
            plan: "hobby",
            feature: "requestLogs",
            requiredPlan: "pro",
            field: "type",
          },
        },
      ],
    } as never);
    render(
      <LogViewer
        resource="web"
        range={LIVE_TAIL}
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "request" }}
      />,
    );
    expect(
      screen.getByText("HTTP request logs require the Pro plan"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(screen.queryByText("Couldn't load logs")).not.toBeInTheDocument();
    // No live tail retrying against the refusal: a Live tail link reads
    // Last hour there (w3/m537).
    expect(useLiveLogsSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ enabled: false }),
    );
    expect(rangePicker()).toHaveTextContent("Last hour");
  });

  it("keeps the generic error for another feature's plan refusal", () => {
    historyState.error = new CombinedGraphQLErrors({
      errors: [
        {
          message: "nope",
          extensions: { code: "PLAN_FEATURE_REQUIRED", feature: "webhooks" },
        },
      ],
    } as never);
    render(<LogViewer resource="web" />);
    expect(screen.getByText("Couldn't load logs")).toBeInTheDocument();
    expect(
      screen.queryByText("HTTP request logs require the Pro plan"),
    ).toBeNull();
  });

  it("renders log lines when history has content", () => {
    historyState.lines = [
      {
        key: "k1",
        timestamp: "t",
        instance: "bv612",
        message: "hello from the app",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        host: "",
        path: "",
        spans: null,
        resource: "",
      },
    ];
    render(<LogViewer resource="web" />);
    expect(screen.getByText("hello from the app")).toBeInTheDocument();
  });

  it("shows filtered empty-state copy when a filter yields zero results", () => {
    render(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, text: "zzz_no_such_term" }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(
      screen.getByText("No logs match these filters."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("No logs in this time range"),
    ).not.toBeInTheDocument();
  });

  it("shows filtered empty-state copy when an instance filter yields zero results", () => {
    render(
      <LogViewer
        resource="web"
        urlFilters={{
          ...EMPTY_LOG_FILTERS,
          instance: "zzz-no-such-pod",
        }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(
      screen.queryByText("No logs in this time range"),
    ).not.toBeInTheDocument();
  });

  it("shows range-relative empty-state copy when there are no logs and no filter", () => {
    render(<LogViewer resource="web" />);
    expect(screen.getByText("No logs in this time range")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Try a longer time range, or pick Live tail to watch for new lines.",
      ),
    ).toBeInTheDocument();
    // Never the unassertable "this service has never logged" claim — every range
    // is bounded, so the page structurally can't know that (w6/m111).
    expect(
      screen.queryByText("This service hasn't produced any logs yet."),
    ).not.toBeInTheDocument();
  });

  it("shows a truncation notice only when hasMore is true (w4/m107)", () => {
    historyState.lines = [
      {
        key: "k1",
        timestamp: "t",
        instance: "bv612",
        message: "hello from the app",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        host: "",
        path: "",
        spans: null,
        resource: "",
      },
    ];
    historyState.hasMore = false;
    const { rerender } = render(<LogViewer resource="web" />);
    expect(
      screen.queryByText(/Only part of this time range is loaded/),
    ).not.toBeInTheDocument();

    // A full page with more behind it is a capped view (w4/m107)...
    historyState.lines = Array.from({ length: 100 }, (_, i) => ({
      ...historyState.lines[0],
      key: `k${i}`,
    }));
    historyState.hasMore = true;
    rerender(<LogViewer resource="web" />);
    expect(
      screen.getByText(/Only part of this time range is loaded/),
    ).toBeInTheDocument();

    // Older pages accumulate under the same caption: it names no count, so
    // 200 loaded rows are never described as the newest 100 (w4/280).
    historyState.lines = Array.from({ length: 200 }, (_, i) => ({
      ...historyState.lines[0],
      key: `k${i}`,
    }));
    rerender(<LogViewer resource="web" />);
    expect(
      screen.getByText(
        "Only part of this time range is loaded. Load older to continue.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/100/)).not.toBeInTheDocument();

    // ...while a short page with more behind it is a search the server
    // time-boxed: the rest of the range is unsearched, not capped (w4/m140).
    historyState.lines = historyState.lines.slice(0, 3);
    rerender(<LogViewer resource="web" />);
    expect(
      screen.getByText(/Only the newest part of this range has been searched/),
    ).toBeInTheDocument();
  });

  it("says a time-boxed search found nothing yet and offers to keep searching (w4/m140)", () => {
    historyState.hasMore = true;
    historyState.loadOlder = vi.fn();
    render(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, text: "zzqqxx-no-such-token" }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(
      screen.getByText(/Only the newest part of this range has been searched/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Load older" }));
    expect(historyState.loadOlder).toHaveBeenCalled();
  });

  it("shows a search timeout as advice to narrow it, not a transport error (w4/m140)", () => {
    historyState.timedOut = true;
    historyState.error = new Error("query timed out");
    render(<LogViewer resource="web" />);
    expect(screen.getByText("This search took too long")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Try a shorter time range or a more specific search term.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load logs")).not.toBeInTheDocument();
  });

  it("keeps the truncation notice while live tail remains available (w4/m107)", () => {
    historyState.lines = [
      {
        key: "k1",
        timestamp: "t",
        instance: "bv612",
        message: "historical",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        host: "",
        path: "",
        spans: null,
        resource: "",
      },
    ];
    historyState.hasMore = true;
    historyState.loadOlder = vi.fn();
    render(<LogViewer resource="web" range={LIVE_TAIL} />);
    expect(screen.getByText("historical")).toBeInTheDocument();
    expect(screen.getByRole("status")).toBeInTheDocument();
    // Live tail still tails alongside page-back.
    expect(useLiveLogsSpy).toHaveBeenCalled();
    const liveOpts = useLiveLogsSpy.mock.calls.at(-1)?.[0] as {
      enabled: boolean;
    };
    expect(liveOpts.enabled).toBe(true);
  });

  it("follows the live tail from the selected window's start, not offset 0 (w6/m111)", () => {
    render(<LogViewer resource="web" range={LIVE_TAIL} />);

    const opts = useLiveLogsSpy.mock.calls.at(-1)?.[0] as { startTime: string };
    const window = useHistorySpy.mock.calls.at(-1)?.[2] as {
      startTime: string;
      endTime: string;
    };
    // The tail's lower bound is exactly the history window's start, so the first
    // SSE connect can't surface lines older than the range. Live tail reads the
    // last 4 hours before new lines arrive, as Render's does (w3/m537).
    expect(opts.startTime).toBe(window.startTime);
    expect(opts).toMatchObject({ enabled: true });
    expect(Date.parse(window.endTime) - Date.parse(window.startTime)).toBe(
      4 * 60 * 60 * 1000,
    );
    expect(Date.parse(opts.startTime)).toBeLessThanOrEqual(Date.now());
  });

  it("passes the selected relative range as concrete history-query bounds", () => {
    const range = RANGE_PRESETS.find((preset) => preset.id === "4h")!;
    render(<LogViewer resource="web" range={range} />);

    const window = useHistorySpy.mock.calls.at(-1)?.[2] as {
      startTime: string;
      endTime: string;
    };
    expect(Date.parse(window.endTime) - Date.parse(window.startTime)).toBe(
      4 * 60 * 60 * 1000,
    );
    // A preset is a snapshot: no tail, as Render's relative range (w3/m537).
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: false,
    });
  });
});

describe("LogViewer display-text search (w4/m189)", () => {
  it("opts both history and the live tail into display matching", () => {
    render(
      <LogViewer
        resource="web"
        range={LIVE_TAIL}
        urlFilters={{ ...EMPTY_LOG_FILTERS, text: "QA_A31_ANSI_red_end" }}
      />,
    );
    expect(useHistorySpy.mock.calls.at(-1)?.[1]).toMatchObject({
      text: "QA_A31_ANSI_red_end",
    });
    expect(useHistorySpy.mock.calls.at(-1)?.[4]).toBe("display");
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      text: "QA_A31_ANSI_red_end",
      textMode: "display",
    });
  });
});

describe("LogViewer URL-backed filter state (w7/m42)", () => {
  it("derives the initial filter state from the URL and queries with it", () => {
    const onFiltersChange = vi.fn();
    render(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, level: "error" }}
        onFiltersChange={onFiltersChange}
      />,
    );

    // The history query receives the URL-derived filters…
    expect(useHistorySpy.mock.calls.at(-1)?.[1]).toMatchObject({
      level: "error",
    });
    // …and the mount does NOT sync upward — that state was just derived from
    // the URL, so there is nothing to write (and a mount-time navigate loops
    // under hydration).
    expect(onFiltersChange).not.toHaveBeenCalled();
  });

  it("reports a debounced filter change upward for the URL write", async () => {
    const onFiltersChange = vi.fn();
    render(<LogViewer resource="web" onFiltersChange={onFiltersChange} />);
    onFiltersChange.mockClear();

    fireEvent.change(screen.getByLabelText("Search logs"), {
      target: { value: "boom" },
    });

    // The URL write rides the same 300ms debounce as the query refetch.
    expect(onFiltersChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ text: "boom" }),
    );
    await waitFor(() =>
      expect(onFiltersChange).toHaveBeenCalledWith({
        ...EMPTY_LOG_FILTERS,
        text: "boom",
      }),
    );
  });

  it("turns a clicked short instance slug into the exact instance filter", async () => {
    const instance = "srv-c185th5c2rvvnhbfiltg-ohoplujmv928mnmaj7j7";
    historyState.lines = [
      {
        key: "k-instance",
        timestamp: "t",
        instance,
        message: "hello from one replica",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        host: "",
        path: "",
        spans: null,
        resource: "",
      },
    ];
    const onFiltersChange = vi.fn();
    render(<LogViewer resource="web" onFiltersChange={onFiltersChange} />);

    fireEvent.click(
      screen.getByRole("button", {
        name: `Filter logs by instance ${instance}`,
      }),
    );

    await waitFor(() =>
      expect(useHistorySpy.mock.calls.at(-1)?.[1]).toEqual({
        ...EMPTY_LOG_FILTERS,
        instance,
      }),
    );
    expect(onFiltersChange).toHaveBeenCalledWith({
      ...EMPTY_LOG_FILTERS,
      instance,
    });
  });
});

// w6/m47 t003 split the title so a filtered miss stops asserting "this service
// has never logged". w6/m111 finished the job: since every range is bounded, the
// unfiltered miss can't assert that either — it now reports the empty range.
describe("LogViewer zero-result empty state (w6/m47, w6/m111)", () => {
  it("titles a filtered zero-result view as a filter miss, not an empty range", () => {
    render(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, text: "zzz_no_such_term" }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(
      screen.getByText("No logs match these filters."),
    ).toBeInTheDocument();
    // The contradictory pairing is the bug: these two must never co-render.
    expect(
      screen.queryByText("No logs in this time range"),
    ).not.toBeInTheDocument();
  });

  it("reports the unfiltered zero-result view as an empty range, never a never-logged service", () => {
    render(<LogViewer resource="web" />);
    expect(screen.getByText("No logs in this time range")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Try a longer time range, or pick Live tail to watch for new lines.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("No matching logs")).not.toBeInTheDocument();
    // The claim this page can never truthfully make (w6/m111).
    expect(
      screen.queryByText("This service hasn't produced any logs yet."),
    ).not.toBeInTheDocument();
  });
});

describe("LogViewer populated background failures", () => {
  it.each(["transport", "timeout", "no-store"])(
    "keeps the reader mounted with an honest %s alert",
    (failure) => {
      historyState.lines = [
        {
          key: "reader-row",
          timestamp: new Date(Date.now() - 60_000).toISOString(),
          instance: "pod",
          message: "retained investigation row",
          type: "app",
          level: "",
          method: "",
          statusCode: "",
          host: "",
          path: "",
          spans: null,
          resource: "",
        },
      ];
      const { container, rerender } = render(<LogViewer resource="web" />);
      const viewport = container.querySelector("[data-log-viewport]");
      historyState.error = new Error("background request failed");
      historyState.timedOut = failure === "timeout";
      historyState.storeUnavailable = failure === "no-store";
      rerender(<LogViewer resource="web" />);
      expect(container.querySelector("[data-log-viewport]")).toBe(viewport);
      expect(
        screen.getByText("retained investigation row"),
      ).toBeInTheDocument();
      expect(screen.getByRole("alert")).toBeInTheDocument();
    },
  );
});

describe("LogViewer combined live reader (w4/m192)", () => {
  const row = (message: string, msAgo: number): LogLine => {
    const timestamp = new Date(Date.now() - msAgo).toISOString();
    return {
      key: `${timestamp}|pod|${message}`,
      timestamp,
      instance: "pod",
      message,
      type: "app",
      level: "",
      method: "",
      statusCode: "",
      host: "",
      path: "",
      spans: null,
      resource: "",
    };
  };

  it("reads older live replay above newer history, newest at the bottom", () => {
    // One object for the overlap: two row() calls can straddle a millisecond
    // and mint different keys, which defeats the dedupe under CI load.
    const overlap = row("QA n=369", 60_000);
    historyState.lines = [overlap];
    liveState.lines = [
      row("QA START", 600_000),
      overlap,
      row("QA n=370", 30_000),
    ];
    render(<LogViewer resource="web" range={LIVE_TAIL} />);
    const shown = screen.getAllByText(/^QA /).map((node) => node.textContent);
    expect(shown).toEqual(["QA START", "QA n=369", "QA n=370"]);
  });

  it("keys the tail's reader identity on line filters, not on the range", () => {
    const { rerender } = render(<LogViewer resource="web" range={LIVE_TAIL} />);
    const readerKey = () =>
      (useLiveLogsSpy.mock.calls.at(-1)?.[0] as { readerKey: string })
        .readerKey;
    const unfiltered = readerKey();
    rerender(<LogViewer resource="web" range={DEFAULT_LOG_RANGE} />);
    expect(readerKey()).toBe(unfiltered);
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: false,
    });

    rerender(
      <LogViewer
        key="filtered"
        resource="web"
        range={LIVE_TAIL}
        urlFilters={{ ...EMPTY_LOG_FILTERS, level: "error" }}
      />,
    );
    expect(readerKey()).not.toBe(unfiltered);
  });
});

describe("LogViewer adopts navigation it did not write (w4/m204)", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  const SOURCE = {
    ...EMPTY_LOG_FILTERS,
    type: "app" as const,
    text: "qa-alpha",
  };
  const search = () => screen.getByLabelText<HTMLInputElement>("Search logs");
  const historyFilters = () => useHistorySpy.mock.calls.at(-1)?.[1];
  const tail = () => useLiveLogsSpy.mock.calls.at(-1)?.[0];
  const type = (value: string) =>
    fireEvent.change(search(), { target: { value } });
  const settle = (ms = 300) => act(() => vi.advanceTimersByTime(ms));

  it("means all logs and no search once a clean Live tail link is followed", () => {
    const onFiltersChange = vi.fn();
    const { rerender } = render(
      <LogViewer
        resource="web"
        urlFilters={SOURCE}
        onFiltersChange={onFiltersChange}
      />,
    );
    expect(search().value).toBe("qa-alpha");
    expect(tail()).toMatchObject({ enabled: false, type: "app" });
    useHistorySpy.mockClear();
    useLiveLogsSpy.mockClear();

    rerender(
      <LogViewer
        resource="web"
        range={LIVE_TAIL}
        onFiltersChange={onFiltersChange}
      />,
    );

    // No pass after the navigation queries or tails with the source's filters.
    for (const [, filters] of useHistorySpy.mock.calls) {
      expect(filters).toEqual(EMPTY_LOG_FILTERS);
    }
    for (const [options] of useLiveLogsSpy.mock.calls) {
      expect(options).toMatchObject({ enabled: true, type: "all", text: "" });
    }
    expect(useHistorySpy).toHaveBeenCalled();
    expect(search().value).toBe("");
    expect(
      screen.getByRole("combobox", { name: "Log history range" }),
    ).toHaveTextContent("Live tail");
    // Nothing written back but the destination itself, now or after a debounce.
    settle(1000);
    for (const call of onFiltersChange.mock.calls) {
      expect(call).toEqual([EMPTY_LOG_FILTERS]);
    }
    expect(historyFilters()).toEqual(EMPTY_LOG_FILTERS);
  });

  it("adopts every filter field of a filtered destination", () => {
    const destination = {
      type: "request" as const,
      level: "error",
      method: "POST",
      statusCode: "503",
      instance: "pod-2",
      text: "needle",
      path: "/api",
      host: "",
    };
    const { rerender } = render(<LogViewer resource="web" range={LIVE_TAIL} />);
    rerender(<LogViewer resource="web" urlFilters={destination} />);
    expect(historyFilters()).toEqual(destination);
    expect(tail()).toMatchObject({
      enabled: false,
      type: "request",
      text: "needle",
      instance: "pod-2",
    });
    expect(search().value).toBe("needle");
  });

  it("drops a pending draft when an external navigation supersedes it", () => {
    const onFiltersChange = vi.fn();
    const { rerender } = render(
      <LogViewer
        resource="web"
        urlFilters={SOURCE}
        onFiltersChange={onFiltersChange}
      />,
    );
    type("qa-alpha-and-more");
    settle(200);
    rerender(<LogViewer resource="web" onFiltersChange={onFiltersChange} />);
    settle(1000);
    expect(search().value).toBe("");
    expect(historyFilters()).toEqual(EMPTY_LOG_FILTERS);
    expect(onFiltersChange).not.toHaveBeenCalledWith(
      expect.objectContaining({ text: "qa-alpha-and-more" }),
    );
  });

  it("keeps text typed since its own write when that write echoes back", () => {
    // A route-like host: every reported state comes back as the next URL.
    function Host() {
      const [filters, setFilters] = useState(EMPTY_LOG_FILTERS);
      return (
        <LogViewer
          resource="web"
          urlFilters={filters}
          onFiltersChange={setFilters}
        />
      );
    }
    render(<Host />);
    type("a");
    settle(299);
    expect(historyFilters()).toMatchObject({ text: "" });
    settle(1);
    // "a" is committed and written; its echo arrives while "ab" is a draft.
    expect(historyFilters()).toMatchObject({ text: "a" });
    type("ab");
    settle(100);
    expect(search().value).toBe("ab");
    settle(200);
    expect(historyFilters()).toMatchObject({ text: "ab" });
    expect(search().value).toBe("ab");
  });

  // w4/538: Render's documented field operators typed into the search box
  // become the structured filters (chips and query params); only the
  // remaining words search text.
  it("turns Render's search operators into structured filters", () => {
    render(<LogViewer resource="web" range={LIVE_TAIL} />);
    type("status_code:/2../ method:/(GET|POST)/ host:web.onbex.co qa-alpha");
    settle();
    expect(historyFilters()).toMatchObject({
      statusCode: "/2../",
      method: "/(GET|POST)/",
      host: "web.onbex.co",
      text: "qa-alpha",
    });
    expect(search().value).toBe("qa-alpha");
    expect(tail()).toMatchObject({
      text: "qa-alpha",
      requestFilters: {
        statusCode: "/2../",
        method: "/(GET|POST)/",
        host: "web.onbex.co",
      },
    });
    // An unknown name stays text.
    type("color:red");
    settle();
    expect(historyFilters()).toMatchObject({
      text: "color:red",
      statusCode: "/2../",
    });
  });

  it("cancels a draft timer when the destination changes another field", () => {
    const { rerender } = render(<LogViewer resource="web" />);
    type("boom");
    settle(200);
    rerender(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "request" }}
      />,
    );
    // The destination replaced the draft, so nothing commits "boom" later.
    settle(300);
    expect(historyFilters()).toEqual({ ...EMPTY_LOG_FILTERS, type: "request" });
  });

  it("does not reopen the reader on rerenders that navigate nowhere", () => {
    const { rerender } = render(
      <LogViewer resource="web" urlFilters={{ ...SOURCE }} />,
    );
    const first = historyFilters();
    type("typed");
    rerender(<LogViewer resource="web" urlFilters={{ ...SOURCE }} />);
    // Same URL, fresh object: the draft stays and the query identity holds.
    expect(search().value).toBe("typed");
    expect(historyFilters()).toBe(first);
  });

  it("keeps a change made before an earlier write echoes, and re-reports it", () => {
    const onFiltersChange = vi.fn();
    const view = (text: string) => (
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, text }}
        onFiltersChange={onFiltersChange}
      />
    );
    const { rerender } = render(view(""));
    type("a");
    settle(); // reported, not yet echoed
    type("");
    settle(); // cleared before the echo
    expect(onFiltersChange.mock.calls).toEqual([
      [{ ...EMPTY_LOG_FILTERS, text: "a" }],
      [EMPTY_LOG_FILTERS],
    ]);
    onFiltersChange.mockClear();

    rerender(view("a")); // the first write echoes back late

    expect(search().value).toBe("");
    expect(historyFilters()).toMatchObject({ text: "" });
    // The URL is told the current state again, so it converges on the view.
    expect(onFiltersChange).toHaveBeenLastCalledWith(EMPTY_LOG_FILTERS);
    rerender(view(""));
    expect(search().value).toBe("");
  });

  it("still adopts a state it once showed after that state has been acknowledged", () => {
    const view = (text: string) => (
      <LogViewer resource="web" urlFilters={{ ...EMPTY_LOG_FILTERS, text }} />
    );
    const { rerender } = render(view(""));
    type("a");
    settle();
    rerender(view("a")); // own write acknowledged
    rerender(view("")); // Back to the earlier entry: not an echo
    expect(search().value).toBe("");
    expect(historyFilters()).toMatchObject({ text: "" });
  });
});

describe("LogViewer search committed while paused (w4/306)", () => {
  const T0 = Date.parse("2026-10-15T00:59:00.000Z");
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(T0);
  });
  afterEach(() => vi.useRealTimers());

  const hour = RANGE_PRESETS.find((preset) => preset.id === "1h")!;
  type Window = { startTime: string; endTime: string };
  const queries = () =>
    useHistorySpy.mock.calls.map(([, filters, window]) => ({
      text: (filters as { text: string }).text,
      type: (filters as { type: string }).type,
      ...(window as Window),
    }));
  const search = (value: string) =>
    fireEvent.change(screen.getByLabelText("Search logs"), {
      target: { value },
    });
  const at = (ms: number) => new Date(T0 + ms).toISOString();

  it("queries a newly committed search up to its commit, before the next tick", () => {
    const { rerender } = render(<LogViewer resource="web" range={LIVE_TAIL} />);
    // A matching line arrives live at T0+10s, then Last hour is picked.
    act(() => vi.advanceTimersByTime(10_000));
    rerender(<LogViewer resource="web" range={hour} />);
    act(() => vi.advanceTimersByTime(4_700));
    useHistorySpy.mockClear();
    search("QA_A497_THIRD_LITERAL");
    act(() => vi.advanceTimersByTime(300)); // the commit debounce, T0+15s

    const searched = queries().filter(
      (q) => q.text === "QA_A497_THIRD_LITERAL",
    );
    expect(searched.length).toBeGreaterThan(0);
    // Not one query for the new search ends at the mount sample.
    for (const q of searched) {
      expect(q.endTime).toBe(at(15_000));
      expect(Date.parse(q.endTime) - Date.parse(q.startTime)).toBe(3600_000);
    }
    // On a preset the window stays frozen at the commit (w4/399): no
    // tick follows the new search, and no history query slides.
    const count = queries().length;
    act(() => vi.advanceTimersByTime(30_000));
    expect(queries().length).toBe(count);
    expect(queries().at(-1)?.endTime).toBe(at(15_000));

    // Clearing the search is a new predicate too, over the same span.
    act(() => vi.advanceTimersByTime(5_000));
    search("");
    act(() => vi.advanceTimersByTime(300));
    expect(queries().at(-1)).toMatchObject({ text: "", endTime: at(50_300) });
  });

  it("does not resample while typing or on a rerender that changes nothing", () => {
    const { rerender } = render(<LogViewer resource="web" range={hour} />);
    act(() => vi.advanceTimersByTime(10_000));
    search("QA");
    act(() => vi.advanceTimersByTime(100));
    search("QA_A");
    act(() => vi.advanceTimersByTime(100));
    // Drafts are not committed predicates: still the mount sample.
    expect(queries().at(-1)).toMatchObject({ text: "", endTime: at(0) });
    rerender(<LogViewer resource="web" range={hour} />);
    expect(queries().at(-1)?.endTime).toBe(at(0));
    const calls = useHistorySpy.mock.calls.length;
    act(() => vi.advanceTimersByTime(300));
    expect(queries().at(-1)).toMatchObject({
      text: "QA_A",
      endTime: at(10_500),
    });
    // Settling takes a bounded number of renders, not a reset loop.
    expect(useHistorySpy.mock.calls.length - calls).toBeLessThan(6);
  });

  it("samples again for a new resource or a structured filter", () => {
    const { rerender } = render(<LogViewer resource="web" range={hour} />);
    act(() => vi.advanceTimersByTime(7_000));
    rerender(<LogViewer resource="worker" range={hour} />);
    expect(queries().at(-1)?.endTime).toBe(at(7_000));
    act(() => vi.advanceTimersByTime(3_000));
    rerender(
      <LogViewer
        resource="worker"
        range={hour}
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "app" }}
      />,
    );
    expect(queries().at(-1)).toMatchObject({
      type: "app",
      endTime: at(10_000),
    });
  });

  // A custom range never tails, ended or not (w3/m537): Render's
  // `liveMode` is `timeRange === "live"`.
  it.each([
    ["has already ended", -7_200_000, -3_600_000],
    ["ends in the future", -3_600_000, 3_600_000],
  ])(
    "never opens the tail for a custom window that %s",
    (_, startMs, endMs) => {
      render(
        <LogViewer
          resource="web"
          range={{
            id: "custom",
            startTime: new Date(Date.now() + startMs).toISOString(),
            endTime: new Date(Date.now() + endMs).toISOString(),
            resolutionSeconds: 30,
          }}
        />,
      );
      expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
        enabled: false,
      });
      expect(screen.queryByRole("switch")).toBeNull();
    },
  );

  it("keeps a custom window fixed when the search changes", () => {
    const custom = {
      id: "custom" as const,
      startTime: "2026-10-14T00:00:00.000Z",
      endTime: "2026-10-14T01:00:00.000Z",
      resolutionSeconds: 30,
    };
    render(<LogViewer resource="web" range={custom} />);
    act(() => vi.advanceTimersByTime(60_000));
    search("QA");
    act(() => vi.advanceTimersByTime(300));
    expect(queries().at(-1)).toMatchObject({
      text: "QA",
      startTime: custom.startTime,
      endTime: custom.endTime,
    });
  });
});

// w9/m389: the viewer bounds its range picker by the workspace plan's log
// window the retention hook reads from bex-api.
describe("the range picker follows the plan's log window", () => {
  beforeAll(() => {
    Element.prototype.hasPointerCapture ??= () => false;
    Element.prototype.releasePointerCapture ??= () => {};
    Element.prototype.scrollIntoView ??= () => {};
  });
  afterEach(() => {
    retention.bound = undefined;
  });

  it("disables the presets past the window and shows the plan's hint", async () => {
    retention.bound = {
      maxSpanSeconds: 7 * 24 * 60 * 60,
      hint: "Logs are kept for 7 days on the Hobby plan.",
    };
    const user = userEvent.setup();
    render(<LogViewer resource="web" />);
    await user.click(screen.getByLabelText("Log history range"));
    expect(
      screen.getByRole("option", { name: "Last 30 days" }),
    ).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("option", { name: "Last 7 days" }),
    ).not.toHaveAttribute("aria-disabled");
    expect(
      screen.getByText("Logs are kept for 7 days on the Hobby plan."),
    ).toBeInTheDocument();
  });
});

describe("LogViewer explorer controls (w3/m509)", () => {
  const appLine = (key: string, second: number): LogLine => ({
    key,
    timestamp: new Date(Date.now() - (60 - second) * 1000).toISOString(),
    instance: "",
    message: key,
    spans: null,
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
  });

  it("clears a live tail's lines on Shift+L and brings them back on request", async () => {
    historyState.lines = [appLine("older line", 1), appLine("newer line", 2)];
    render(<LogViewer resource="web" range={LIVE_TAIL} />);
    expect(screen.getByText("newer line")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(screen.getByText("Logs cleared")).toBeInTheDocument();
    expect(screen.queryByText("newer line")).toBeNull();

    await userEvent.click(
      screen.getByRole("button", { name: "Show earlier lines" }),
    );
    expect(screen.getByText("newer line")).toBeInTheDocument();
  });

  it("clears a live tail's lines from Options ▸ Clear logs, as Shift+L does (w3/092)", async () => {
    const user = userEvent.setup();
    historyState.lines = [appLine("older line", 1), appLine("newer line", 2)];
    render(<LogViewer resource="web" range={LIVE_TAIL} />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    await user.click(screen.getByRole("menuitem", { name: /Clear logs/ }));
    expect(screen.getByText("Logs cleared")).toBeInTheDocument();
    expect(screen.queryByText("newer line")).toBeNull();
  });

  it("does not clear a log on a preset range, and its Options list no Clear logs", async () => {
    const user = userEvent.setup();
    historyState.lines = [appLine("kept line", 1)];
    render(<LogViewer resource="web" />);
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(screen.getByText("kept line")).toBeInTheDocument();
    expect(screen.queryByText("Logs cleared")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(screen.getByRole("menuitem", { name: /Copy logs/ })).toBeVisible();
    expect(screen.queryByRole("menuitem", { name: /Clear logs/ })).toBeNull();
  });

  it("maximizes the explorer and focuses its search on /", async () => {
    const user = userEvent.setup();
    historyState.lines = [appLine("a line", 1)];
    const { container } = render(<LogViewer resource="web" />);
    const frame = container.querySelector<HTMLElement>("[data-log-frame]")!;
    await user.click(screen.getByRole("button", { name: "Maximize" }));
    expect(frame.className).toContain("fixed inset-0");
    await user.keyboard("{Escape}");
    expect(frame.className).not.toContain("fixed inset-0");
    await user.keyboard("/");
    expect(screen.getByRole("textbox", { name: "Search logs" })).toHaveFocus();
  });

  it("offers Copy logs only while lines are shown", async () => {
    const user = userEvent.setup();
    render(<LogViewer resource="web" />);
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(screen.getByRole("menuitem", { name: /Copy logs/ })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });
});

// Live tail is a range (w3/m537): what picks it, and where it cannot run.
describe("LogViewer Live tail range (w3/m537)", () => {
  it("picks Live tail from the empty state's Switch to Live Tail", () => {
    render(<Viewer resource="web" />);
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: false,
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Switch to Live Tail" }),
    );
    expect(rangePicker()).toHaveTextContent("Live tail");
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: true,
    });
    // On Live tail the empty state offers no switch to it.
    expect(
      screen.queryByRole("button", { name: "Switch to Live Tail" }),
    ).toBeNull();
  });

  it("reads Last hour, with Live tail disabled and its reason, where the tail cannot follow the filters", async () => {
    const user = userEvent.setup();
    render(
      <LogViewer
        resource="web"
        range={LIVE_TAIL}
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "app", method: "GET" }}
      />,
    );
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: false,
    });
    const window = useHistorySpy.mock.calls.at(-1)?.[2] as {
      startTime: string;
      endTime: string;
    };
    expect(Date.parse(window.endTime) - Date.parse(window.startTime)).toBe(
      60 * 60 * 1000,
    );
    expect(rangePicker()).toHaveTextContent("Last hour");
    const reason =
      "Live tail is unavailable — method, status, path and host filter request logs, so app logs can't be tailed with them.";
    expect(screen.getByText(reason)).toBeVisible();
    await user.click(rangePicker());
    const live = screen.getByRole("option", { name: "Live tail" });
    expect(live).toHaveAttribute("aria-disabled", "true");
    expect(live).toHaveAccessibleDescription(reason);
  });
});

describe("the Logs tab's pending frame holds its explorer buttons (w3/m509)", () => {
  it("reserves Maximize (hidden on a phone) and Options, at the ready buttons' sizes", async () => {
    const { ServiceLogsBodySkeleton } =
      await import("@/common/components/route-skeletons");
    const boxes = (root: HTMLElement) =>
      [...root.querySelectorAll<HTMLElement>("button, [data-slot=skeleton]")]
        .filter((el) => el.className.includes("max-lg:size-11"))
        .map((el) =>
          el.className
            .split(/\s+/)
            .filter((name) => /^(\w[\w-]*:)*(size-|hidden$)/.test(name))
            .sort()
            .join(" "),
        );
    historyState.lines = [];
    const ready = boxes(render(<LogViewer resource="web" />).container);
    expect(ready).toEqual([
      "max-lg:size-11 max-sm:hidden pointer-coarse:size-11 size-8",
      "max-lg:size-11 pointer-coarse:size-11 size-8",
    ]);
    expect(boxes(render(<ServiceLogsBodySkeleton />).container)).toEqual(ready);
  });

  // Live tail is a range (w3/m537): no Live switch in the bar or its pending
  // frame, and the Filters slot spans a phone's second row as the button does.
  it("holds no Live switch, and gives Filters the ready button's phone columns", async () => {
    const { ServiceLogsBodySkeleton } =
      await import("@/common/components/route-skeletons");
    const ready = render(<LogViewer resource="web" />).container;
    const pending = render(<ServiceLogsBodySkeleton />).container;
    for (const root of [ready, pending]) {
      expect(root.querySelector('[role="switch"]')).toBeNull();
    }
    const filters = ready.querySelector<HTMLElement>(
      '[data-log-dock] > button[aria-haspopup="dialog"]',
    )!;
    expect(filters).toHaveTextContent("Filters");
    expect(filters.className).toContain("max-sm:col-span-2");
    const slots = pending.querySelectorAll<HTMLElement>(
      "[data-log-dock] > [data-slot=skeleton]",
    );
    expect(slots[3].className).toContain("max-sm:col-span-2");
    // Its fixed desktop width gives way to the grid's columns on a phone.
    expect(slots[3].className).toContain("max-sm:w-auto");
  });
});

// Render's explorer pieces w3/m509 left out (w3/m528), as the service Logs tab
// wires them: held lines while tailing, the paused view's New logs in Live
// Tail, the row menu's View in context and SSH, and the marked line.
describe("LogViewer row menu and new-log buttons (w3/m528)", () => {
  // A minute apart, the newest a minute ago: inside the default hour.
  const lines = (from: number, to: number): LogLine[] =>
    Array.from({ length: to - from }, (_, i) => {
      const n = from + i;
      return {
        key: `id:${n}`,
        timestamp: new Date(Date.now() - (200 - n) * 1000).toISOString(),
        instance: "srv-web-0123456789abcdefghij",
        message: `m${n}`,
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        host: "",
        path: "",
        spans: null,
        resource: "",
      };
    });
  const watchArgs = () =>
    useNewLogsWatchSpy.mock.calls.at(-1)?.[0] as {
      enabled: boolean;
      since: string;
      subscription: Record<string, unknown>;
    };
  const viewport = (container: HTMLElement) =>
    container.querySelector<HTMLElement>("[data-log-viewport]")!;
  beforeEach(() => {
    watchState.available = false;
    watchState.dismiss = vi.fn();
    useNewLogsWatchSpy.mockReset();
  });

  it("watches for a new line only on a preset range, from the frozen window's end", () => {
    historyState.lines = lines(0, 3);
    const { rerender } = render(<LogViewer resource="web" />);
    expect(watchArgs().enabled).toBe(true);
    const history = useHistorySpy.mock.calls.at(-1)?.[2] as {
      endTime: string;
    };
    expect(watchArgs().since).toBe(history.endTime);
    expect(watchArgs().subscription).toMatchObject({
      resource: "web",
      type: "all",
      textMode: "display",
    });
    // Live tail: the tail itself streams.
    rerender(<LogViewer key="live" resource="web" range={LIVE_TAIL} />);
    expect(watchArgs().enabled).toBe(false);
    // A custom range, as View in context opens, is never invited back to Live.
    rerender(
      <LogViewer
        key="custom"
        resource="web"
        range={{
          id: "custom",
          startTime: new Date(Date.now() - 600_000).toISOString(),
          endTime: new Date(Date.now() + 600_000).toISOString(),
          resolutionSeconds: 60,
        }}
      />,
    );
    expect(watchArgs().enabled).toBe(false);
    // A filter the tail cannot follow.
    rerender(
      <LogViewer
        key="unsupported"
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "app", method: "GET" }}
      />,
    );
    expect(watchArgs().enabled).toBe(false);
  });

  it("watches nothing on a plan that refuses the request view", () => {
    historyState.error = new CombinedGraphQLErrors({
      errors: [
        {
          message: "HTTP request logs require the Pro plan or higher",
          extensions: { code: "PLAN_FEATURE_REQUIRED", feature: "requestLogs" },
        },
      ],
    });
    render(
      <LogViewer
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, type: "request" }}
      />,
    );
    expect(watchArgs().enabled).toBe(false);
  });

  it("picks Live tail from New logs in Live Tail, and leaves Dismiss to the watch", () => {
    historyState.lines = lines(0, 3);
    watchState.available = true;
    render(<Viewer resource="web" />);
    expect(rangePicker()).toHaveTextContent("Last hour");
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(watchState.dismiss).toHaveBeenCalledTimes(1);
    fireEvent.click(
      screen.getByRole("button", { name: "New logs in Live Tail" }),
    );
    expect(rangePicker()).toHaveTextContent("Live tail");
    expect(useLiveLogsSpy.mock.calls.at(-1)?.[0]).toMatchObject({
      enabled: true,
    });
    expect(watchArgs().enabled).toBe(false);
  });

  it("holds tailed lines while the reader is away, and never on a preset", () => {
    historyState.lines = lines(0, 60);
    const { container, rerender } = render(
      <LogViewer resource="web" range={LIVE_TAIL} />,
    );
    const scrollAway = () =>
      act(() => {
        const el = viewport(container);
        Object.defineProperty(el, "scrollHeight", {
          configurable: true,
          value: 5000,
        });
        Object.defineProperty(el, "clientHeight", {
          configurable: true,
          value: 520,
        });
        el.scrollTop = 0;
        el.dispatchEvent(new Event("scroll"));
      });
    scrollAway();
    liveState.lines = lines(60, 75);
    rerender(<LogViewer resource="web" range={LIVE_TAIL} />);
    expect(
      screen.getByRole("button", { name: "15+ new logs" }),
    ).toBeInTheDocument();

    // On a preset, nothing is held: a resumed reader's lines show at once.
    rerender(<LogViewer key="paused" resource="web" />);
    scrollAway();
    liveState.lines = lines(60, 80);
    rerender(<LogViewer key="paused" resource="web" />);
    expect(screen.queryByRole("button", { name: /new logs$/ })).toBeNull();
  });

  it("offers View in context only while a search or filter narrows the view, and SSH where the route gives it", async () => {
    const user = userEvent.setup();
    historyState.lines = lines(0, 3);
    const contextHref = vi.fn((line: LogLine) => `/ctx/${line.key}`);
    const shell = vi.fn(() => ({ href: "/services/web/shell?instance=x" }));
    const { container, rerender } = render(
      <LogViewer resource="web" contextHref={contextHref} shell={shell} />,
    );
    const openMenu = async () => {
      fireEvent.pointerEnter(container.querySelector('[data-index="0"]')!);
      await user.click(
        screen.getByRole("button", { name: "Log line options" }),
      );
    };
    await openMenu();
    expect(
      screen.queryByRole("menuitem", { name: "View in context" }),
    ).toBeNull();
    expect(
      screen.getByRole("menuitem", { name: "SSH into instance" }),
    ).toHaveAttribute("href", "/services/web/shell?instance=x");
    await user.keyboard("{Escape}");

    rerender(
      <LogViewer
        key="filtered"
        resource="web"
        urlFilters={{ ...EMPTY_LOG_FILTERS, text: "m" }}
        contextHref={contextHref}
        shell={shell}
      />,
    );
    await openMenu();
    const link = screen.getByRole("menuitem", { name: "View in context" });
    expect(link).toHaveAttribute("href", "/ctx/id:0");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("marks the line a View in context link names", () => {
    historyState.lines = lines(0, 3);
    const { container } = render(
      <LogViewer resource="web" contextLine="id:1" />,
    );
    expect(container.querySelector("[data-marked]")).toHaveTextContent("m1");
  });
});

// Log Explorer Theme (w3/m531): a service's Logs tab keeps its own theme,
// in its high-contrast twin under High Contrast Mode, or follows the page.
describe("LogViewer under Log Explorer Theme (w3/m531)", () => {
  it.each(LOG_THEME_CASES)(
    "%s with High Contrast Mode %s puts %j on its root",
    (logTheme, highContrast, classes) => {
      const { container } = render(
        <LogThemeContext logTheme={logTheme} highContrast={highContrast}>
          <LogViewer resource="web" />
        </LogThemeContext>,
      );
      expect(explorerThemeClasses(container)).toEqual(classes);
    },
  );
});
