import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import { DatastoreLogViewer } from "../datastore-log-viewer";
import userEvent from "@testing-library/user-event";
import { serverRendered } from "@/test/hydration";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import { EMPTY_LOG_FILTERS, type LogLine } from "../../types";
import { FakeEventSource } from "@/test/mocks/event-source";
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

    state.error = codedGraphQLError("LOGS_UNAVAILABLE");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(
      screen.getByText("Database logs aren't configured"),
    ).toBeInTheDocument();

    state.error = codedGraphQLError("FORBIDDEN");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("You can't view these logs")).toBeInTheDocument();

    state.error = new Error("Loki is unavailable");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
    expect(screen.getByText("Loki is unavailable")).toBeInTheDocument();

    // The server's wording without its code decides nothing (w5/m128).
    state.error = uncodedGraphQLError("forbidden: logs source not configured");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
  });

  it("uses Key Value copy for a red- resource", () => {
    const { rerender } = render(
      <DatastoreLogViewer kind="keyvalue" resource="red-example" />,
    );
    expect(screen.getByText("No log lines")).toBeInTheDocument();

    state.error = codedGraphQLError("FORBIDDEN");
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
    // Datastore search keeps the raw text contract (w4/m189 scoped the
    // display opt-in to the service viewer).
    expect(useHistorySpy.mock.calls.at(-1)?.[4]).toBeUndefined();
    expect(Date.parse(window.endTime) - Date.parse(window.startTime)).toBe(
      60 * 60 * 1000,
    );
  });

  it("states truncation only when the server has more (w4/m136)", () => {
    state.lines = Array.from({ length: 100 }, (_, i) => line(i));
    const { rerender } = render(
      <DatastoreLogViewer kind="databases" resource="dpg-example" />,
    );
    expect(
      screen.queryByText(/Only part of this time range is loaded/),
    ).toBeNull();

    state.hasMore = true;
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByRole("status")).toHaveTextContent(
      /Only part of this time range is loaded/,
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

// w4/m219 (a850): text typed into the server-rendered datastore log search
// before handlers attach is wiped by hydration — on both callers.
it.each([
  ["databases", "dpg-example", "Search database logs"],
  ["keyvalue", "red-example", "Search logs…"],
] as const)(
  "keeps the %s log search uneditable until hydration (w4/m219)",
  async (kind, resource, label) => {
    state.lines = [line(0)];
    const { hydrate, getByRole } = serverRendered(
      <DatastoreLogViewer kind={kind} resource={resource} />,
    );
    const search = () =>
      getByRole<HTMLInputElement>("textbox", { name: label });
    expect(search().disabled).toBe(true);
    await hydrate();
    expect(search().disabled).toBe(false);
    await userEvent.type(search(), "checkpoint");
    expect(search().value).toBe("checkpoint");
  },
);

// w5/m139: a datastore's public instance id is `<id>-<20-char opaque>`; shown
// whole it squeezed the message to nothing on a narrow screen. The row shows
// the short suffix and keeps the full id as its title.
it("shortens a long datastore instance id and keeps it whole as the title", () => {
  const instance = "red-dbbhh61jg4r11240u0l0-qblf27h3o38bsl8kh48l";
  state.lines = [{ ...line(0), instance }];
  render(<DatastoreLogViewer kind="keyvalue" resource="red-example" />);
  const label = screen.getByText("[qblf2]");
  expect(label).toHaveAttribute("title", instance);
});

// w5/m139: both datastores' lines carry a level, and Render's /logs/subscribe
// takes a postgres or redis resource, so the tab filters by level and tails live.
describe("DatastoreLogViewer level and live tail (w5/m139)", () => {
  let streams: FakeEventSource[] = [];
  const createEventSource = (url: string) => {
    const s = new FakeEventSource(url);
    streams.push(s);
    return s;
  };
  beforeEach(() => {
    streams = [];
  });

  it("filters the history by level and writes it to the URL", async () => {
    const onFiltersChange = vi.fn();
    render(
      <DatastoreLogViewer
        kind="keyvalue"
        resource="red-example"
        filters={{ text: "", instance: "", level: "warning" }}
        onFiltersChange={onFiltersChange}
      />,
    );
    expect(useHistorySpy).toHaveBeenLastCalledWith(
      "red-example",
      expect.objectContaining({ level: "warning" }),
      expect.anything(),
    );
    await userEvent.click(screen.getByRole("combobox", { name: "Level" }));
    // Valkey's markers name no error level, so Key Value does not offer one.
    expect(screen.queryByRole("option", { name: "error" })).toBeNull();
    await userEvent.click(screen.getByRole("option", { name: "debug" }));
    expect(onFiltersChange).toHaveBeenLastCalledWith({ level: "debug" });
  });

  it("opens no stream until Live is switched on, then tails with the same filters", async () => {
    // History inside the selected window, as the server returns it: the merge
    // drops lines stamped before the window's start.
    state.lines = [
      { ...line(0), timestamp: new Date(Date.now() - 60_000).toISOString() },
    ];
    render(
      <DatastoreLogViewer
        kind="keyvalue"
        resource="red-example"
        filters={{ text: "", instance: "", level: "warning" }}
        onFiltersChange={() => undefined}
        createEventSource={createEventSource}
      />,
    );
    expect(streams).toHaveLength(0);
    await userEvent.click(screen.getByRole("switch", { name: "Live" }));
    expect(streams).toHaveLength(1);
    const url = new URL(streams[0].url, "http://x");
    expect(url.pathname).toMatch(/\/v1\/logs\/subscribe$/);
    expect(url.searchParams.get("resource")).toBe("red-example");
    expect(url.searchParams.get("level")).toBe("warning");
    expect(url.searchParams.get("type")).toBeNull();
    expect(url.searchParams.get("startTime")).not.toBeNull();

    act(() => {
      streams[0].open();
      streams[0].frame({
        message: "1:M 15 Jul 2026 12:00:59.000 # WARNING low memory",
        timestamp: new Date().toISOString(),
        labels: [
          { name: "type", value: "keyvalue" },
          { name: "instance", value: "red-example-0" },
          { name: "level", value: "warning" },
        ],
      });
    });
    // Frames are flushed in batches.
    expect(
      await screen.findByText(
        "1:M 15 Jul 2026 12:00:59.000 # WARNING low memory",
      ),
    ).toBeInTheDocument();
    // History stays beside the tail.
    expect(screen.getByText("checkpoint complete")).toBeInTheDocument();
  });
});
