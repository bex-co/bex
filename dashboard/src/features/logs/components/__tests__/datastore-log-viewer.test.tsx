import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import { DatastoreLogViewer } from "../datastore-log-viewer";
import { EMPTY_LOG_FILTERS, type LogLine } from "../../types";
import {
  scrollViewport,
  setupVirtualGeometry,
  VIRTUAL_VIEWPORT_HEIGHT,
} from "@/test/virtual-geometry";

const state: UseLogHistoryResult = {
  lines: [],
  loading: false,
  error: undefined,
  storeUnavailable: false,
  timedOut: false,
  hasMore: false,
  loadingOlder: false,
  loadOlder: () => undefined,
};
const useHistorySpy = vi.fn();

vi.mock("../../hooks/use-log-history", () => ({
  useLogHistory: (...args: unknown[]) => {
    useHistorySpy(...args);
    return state;
  },
}));
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: () => ["dpg-example-1"],
}));

// The viewer embeds the virtualized LogLineList (w9/m83): give jsdom the layout
// geometry the virtualizer needs, or the rendered rows never mount.
setupVirtualGeometry();

function line(i: number): LogLine {
  return {
    key: `line-${i}`,
    timestamp: `2026-07-15T12:00:${String(i % 60).padStart(2, "0")}Z`,
    time: "12:00:00",
    instance: "dpg-example-1",
    message: i === 0 ? "checkpoint complete" : `line ${i}`,
    spans: null,
    type: "postgres",
    level: "",
    method: "",
    statusCode: "",
  };
}

beforeEach(() => {
  state.lines = [];
  state.loading = false;
  state.error = undefined;
  state.hasMore = false;
  state.timedOut = false;
  state.loadingOlder = false;
  state.loadOlder = () => undefined;
  useHistorySpy.mockReset();
});

describe("DatastoreLogViewer", () => {
  it("renders timestamped, instance-attributed database lines", () => {
    state.lines = [line(0)];
    render(<DatastoreLogViewer kind="databases" resource="dpg-example" />);

    expect(screen.getByText("checkpoint complete")).toBeInTheDocument();
    expect(screen.getByText("[dpg-example-1]")).toBeInTheDocument();
    expect(screen.getByText("12:00:00")).toBeInTheDocument();
  });

  it("distinguishes empty, unavailable, unauthorized, and generic errors", () => {
    const { rerender } = render(
      <DatastoreLogViewer kind="databases" resource="dpg-example" />,
    );
    expect(screen.getByText("No database logs yet")).toBeInTheDocument();

    state.error = new Error("logs source not configured");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(
      screen.getByText("Database logs aren't configured"),
    ).toBeInTheDocument();

    state.error = new Error("forbidden: no access");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("You can't view these logs")).toBeInTheDocument();

    state.error = new Error("Loki is unavailable");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
    expect(screen.getByText("Loki is unavailable")).toBeInTheDocument();
  });

  it("uses Key Value copy for a red- resource", () => {
    const { rerender } = render(
      <DatastoreLogViewer kind="keyvalue" resource="red-example" />,
    );
    expect(screen.getByText("No log lines")).toBeInTheDocument();

    state.error = new Error("forbidden");
    rerender(<DatastoreLogViewer kind="keyvalue" resource="red-example" />);
    expect(screen.getByText("Access denied")).toBeInTheDocument();
  });

  it.each([
    ["databases", "dpg-example", "No database logs yet"],
    ["keyvalue", "red-example", "No log lines"],
  ] as const)(
    "titles a filtered-empty %s search as no match, not no logs (w4/195)",
    (kind, resource, unfilteredTitle) => {
      render(<DatastoreLogViewer kind={kind} resource={resource} />);
      expect(screen.getByText(unfilteredTitle)).toBeInTheDocument();

      fireEvent.change(screen.getByRole("textbox"), {
        target: { value: "PASSWORD" },
      });
      expect(screen.getByText("No matching logs")).toBeInTheDocument();
      // The service viewer's copy, shared since w5/m125.
      expect(
        screen.getByText("No logs match these filters."),
      ).toBeInTheDocument();
      expect(screen.queryByText(unfilteredTitle)).toBeNull();

      // The partial-search branch wraps the same empty state (w4/m140).
      state.hasMore = true;
      fireEvent.change(screen.getByRole("textbox"), {
        target: { value: "PASSWORDS" },
      });
      expect(screen.getByText("No matching logs")).toBeInTheDocument();
    },
  );

  it("sends only the datastore filters — no service-only filter (w4/m136)", () => {
    render(<DatastoreLogViewer kind="keyvalue" resource="red-example" />);
    const [resource, filters, window] = useHistorySpy.mock.calls.at(-1) as [
      string,
      typeof EMPTY_LOG_FILTERS,
      { startTime: string; endTime: string },
    ];
    expect(resource).toBe("red-example");
    // type "all" and every empty structured filter are sent as absent args.
    expect(filters).toEqual({ ...EMPTY_LOG_FILTERS, text: "", instance: "" });
    expect(Date.parse(window.endTime) - Date.parse(window.startTime)).toBe(
      60 * 60 * 1000,
    );
  });

  it("states truncation only when the server has more (w4/m136)", () => {
    state.lines = Array.from({ length: 100 }, (_, i) => line(i));
    const { rerender } = render(
      <DatastoreLogViewer kind="databases" resource="dpg-example" />,
    );
    expect(screen.queryByText(/newest 100 matching lines/)).toBeNull();

    state.hasMore = true;
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByRole("status")).toHaveTextContent(
      /newest 100 matching lines/,
    );
  });

  it("loads older history when the pane reaches its top (w4/m136)", async () => {
    state.lines = Array.from({ length: 100 }, (_, i) => line(i));
    state.hasMore = true;
    state.loadOlder = vi.fn();
    const { container } = render(
      <DatastoreLogViewer kind="keyvalue" resource="red-example" />,
    );
    const viewport = container.querySelector<HTMLElement>(
      "[data-log-viewport]",
    );
    if (!viewport) throw new Error("log viewport not found");

    scrollViewport(viewport, {
      scrollTop: 0,
      scrollHeight: 2400,
      clientHeight: VIRTUAL_VIEWPORT_HEIGHT,
    });
    expect(state.loadOlder).toHaveBeenCalled();

    // TanStack Virtual's debounced scroll-end callback must run before teardown.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 175));
    });
  });
});
