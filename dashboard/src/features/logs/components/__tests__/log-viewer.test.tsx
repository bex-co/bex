import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { LogViewer } from "../log-viewer";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import { RANGE_PRESETS } from "@/features/metrics/lib/range";
import { EMPTY_LOG_FILTERS } from "../../types";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// Drive the viewer's states by stubbing its data layer, mirroring the
// services.$serviceId routing test's approach.
const historyState: UseLogHistoryResult = {
  lines: [],
  loading: false,
  error: undefined,
  storeUnavailable: false,
};
const useHistorySpy = vi.fn();
vi.mock("../../hooks/use-log-history", () => ({
  useLogHistory: (...args: unknown[]) => {
    useHistorySpy(...args);
    return historyState;
  },
}));
vi.mock("../../hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
}));
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: () => [],
}));

// The viewer embeds the virtualized LogLineList (w9/m83): give jsdom the layout
// geometry the virtualizer needs, or the rendered rows never mount.
setupVirtualGeometry();

beforeEach(() => {
  historyState.lines = [];
  historyState.loading = false;
  historyState.error = undefined;
  historyState.storeUnavailable = false;
  useHistorySpy.mockReset();
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

  it("shows the generic error state for a non-store error", () => {
    historyState.error = new Error("upstream exploded");
    render(<LogViewer resource="web" />);
    expect(screen.getByText("Couldn't load logs")).toBeInTheDocument();
    expect(screen.getByText("upstream exploded")).toBeInTheDocument();
  });

  it("renders log lines when history has content", () => {
    historyState.lines = [
      {
        key: "k1",
        timestamp: "t",
        time: "10:36:01",
        instance: "bv612",
        message: "hello from the app",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        spans: null,
      },
    ];
    render(<LogViewer resource="web" />);
    expect(screen.getByText("hello from the app")).toBeInTheDocument();
  });

  it("shows filtered empty-state copy when a filter yields zero results", () => {
    render(
      <LogViewer
        resource="web"
        initialFilters={{ ...EMPTY_LOG_FILTERS, text: "zzz_no_such_term" }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(
      screen.getByText("No logs match these filters."),
    ).toBeInTheDocument();
    expect(screen.queryByText("No logs yet")).not.toBeInTheDocument();
  });

  it("shows unfiltered empty-state copy when there are no logs and no filter", () => {
    render(<LogViewer resource="web" />);
    expect(screen.getByText("No logs yet")).toBeInTheDocument();
    expect(
      screen.getByText("This service hasn't produced any logs yet."),
    ).toBeInTheDocument();
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
    expect(
      screen.getByText(
        "The range limits history. Live mode appends new lines as they arrive.",
      ),
    ).toBeInTheDocument();
  });
});

describe("LogViewer URL-backed filter state (w7/m42)", () => {
  it("derives the initial filter + live state from the URL and queries with it", () => {
    const onFiltersChange = vi.fn();
    render(
      <LogViewer
        resource="web"
        initialFilters={{ ...EMPTY_LOG_FILTERS, level: "error" }}
        initialLive={false}
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
      true,
    );
    await waitFor(() =>
      expect(onFiltersChange).toHaveBeenCalledWith(
        { ...EMPTY_LOG_FILTERS, text: "boom" },
        true,
      ),
    );
  });

  it("turns a clicked short instance slug into the exact instance filter", async () => {
    const instance = "web-6f7d8f9c4b-bv612";
    historyState.lines = [
      {
        key: "k-instance",
        timestamp: "t",
        time: "10:36:01",
        instance,
        message: "hello from one replica",
        type: "app",
        level: "",
        method: "",
        statusCode: "",
        spans: null,
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
    expect(onFiltersChange).toHaveBeenCalledWith(
      { ...EMPTY_LOG_FILTERS, instance },
      true,
    );
  });
});

// w6/m47 t003: the empty state used to hardcode "No logs yet" as its title
// while the body branched on the active filter, so a zero-result search on a
// service with real log history asserted both "this service has never logged"
// and "nothing matches your filter" at once.
describe("LogViewer zero-result empty state (w6/m47)", () => {
  it("titles a filtered zero-result view as a filter miss, not an empty service", () => {
    render(
      <LogViewer
        resource="web"
        initialFilters={{ ...EMPTY_LOG_FILTERS, text: "zzz_no_such_term" }}
      />,
    );
    expect(screen.getByText("No matching logs")).toBeInTheDocument();
    expect(screen.getByText("No logs match these filters.")).toBeInTheDocument();
    // The contradictory pairing is the bug: these two must never co-render.
    expect(screen.queryByText("No logs yet")).not.toBeInTheDocument();
  });

  it("keeps the unfiltered zero-result view reporting a genuinely empty service", () => {
    render(<LogViewer resource="web" />);
    expect(screen.getByText("No logs yet")).toBeInTheDocument();
    expect(
      screen.getByText("This service hasn't produced any logs yet."),
    ).toBeInTheDocument();
    expect(screen.queryByText("No matching logs")).not.toBeInTheDocument();
  });
});
