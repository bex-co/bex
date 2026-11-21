import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import { DatastoreLogViewer } from "../datastore-log-viewer";
import type { RangeBound } from "@/features/metrics/lib/range";
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
import {
  explorerThemeClasses,
  LOG_THEME_CASES,
  LogThemeContext,
} from "@/test/log-theme";

// Options ends with Theme Settings, a router Link (w3/m528): render it as
// its anchor, with no router around the component.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: (await import("@/test/mocks/router-link")).PlainLink,
}));

const state: UseLogHistoryResult = {
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
    return state;
  },
}));
// The plan's log window (w9/m389) is its own query; the reader tests keep the
// picker unbounded, and the bound test sets one.
const retention: { bound: RangeBound | undefined } = { bound: undefined };
vi.mock("../../hooks/use-log-retention", () => ({
  useLogRetentionBound: () => retention.bound,
}));
vi.mock("../../hooks/use-log-label-values", () => ({
  useLogLabelValues: () => ["dpg-example-1"],
}));

// The viewer embeds the virtualized LogLineList (w9/m83): give jsdom the layout
// geometry the virtualizer needs, or the rendered rows never mount.
setupVirtualGeometry();

// Picks a range in the tab's picker, as a reader does (w3/m537).
async function pickRange(
  name: string,
  user: ReturnType<typeof userEvent.setup> = userEvent.setup(),
) {
  await user.click(screen.getByRole("combobox", { name: /range/i }));
  await user.click(screen.getByRole("option", { name }));
}

function line(i: number): LogLine {
  return {
    key: `line-${i}`,
    timestamp: `2026-07-15T12:00:${String(i % 60).padStart(2, "0")}Z`,
    instance: "dpg-example-1",
    message: i === 0 ? "checkpoint complete" : `line ${i}`,
    spans: null,
    resource: "",
    type: "postgres",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
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
    expect(screen.getByText("12:00:00 PM")).toBeInTheDocument();
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

    // A read that never reached bex-api shows translated copy, not the
    // browser's transport text (w5/167).
    state.error = new TypeError("Failed to fetch");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Something went wrong while loading. Refresh the page to try again.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Failed to fetch")).toBeNull();

    // The server's wording without its code decides nothing (w5/m128), but
    // it is still the reason shown.
    state.error = uncodedGraphQLError("forbidden: logs source not configured");
    rerender(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    expect(screen.getByText("Couldn't load database logs")).toBeInTheDocument();
    expect(
      screen.getByText("Forbidden: logs source not configured"),
    ).toBeInTheDocument();
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

  // Live tail is a range (w3/m537): as on Render, a custom window never
  // tails, ended or not, and nothing watches it for a new line either.
  it.each([
    ["has already ended", -60, -59],
    ["ends in the future", -5, 60],
  ])(
    "never tails a custom range that %s, and still offers Live tail",
    async (_, startMinutes, endMinutes) => {
      render(
        <DatastoreLogViewer
          kind="keyvalue"
          resource="red-example"
          filters={{ text: "", instance: "", level: "" }}
          onFiltersChange={() => undefined}
          range={{
            id: "custom",
            startTime: new Date(
              Date.now() + startMinutes * 60_000,
            ).toISOString(),
            endTime: new Date(Date.now() + endMinutes * 60_000).toISOString(),
            resolutionSeconds: 60,
          }}
          onRangeChange={() => undefined}
          createEventSource={createEventSource}
        />,
      );
      expect(screen.queryByRole("switch")).toBeNull();
      expect(streams).toHaveLength(0);
      await userEvent.click(screen.getByRole("combobox", { name: /range/i }));
      expect(screen.getByRole("option", { name: "Live tail" })).toBeEnabled();
    },
  );

  it("opens no tail until Live tail is picked, then tails with the same filters", async () => {
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
    // On Last hour, the tab only watches for a new line (w3/m528): one stream
    // from the window's end, closed when Live tail opens the tail.
    expect(streams).toHaveLength(1);
    const watch = new URL(streams[0].url, "http://x");
    expect(watch.searchParams.get("level")).toBe("warning");
    expect(
      Date.parse(watch.searchParams.get("startTime") ?? ""),
    ).toBeGreaterThan(Date.now() - 60_000);
    await pickRange("Live tail");
    expect(streams).toHaveLength(2);
    expect(streams[0].closed).toBe(true);
    streams.shift();
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

// w9/m389: the viewer bounds its range picker by the workspace plan's log
// window the retention hook reads from bex-api.
describe("DatastoreLogViewer explorer controls (w3/m509)", () => {
  it("clears its lines on Shift+L only once Live tail is the range", async () => {
    const user = userEvent.setup();
    state.lines = [line(0)];
    render(
      <DatastoreLogViewer
        kind="databases"
        resource="dpg-example"
        createEventSource={(url) => new FakeEventSource(url)}
      />,
    );
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(screen.getByText("checkpoint complete")).toBeInTheDocument();

    await pickRange("Live tail", user);
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(screen.getByText("Logs cleared")).toBeInTheDocument();
    expect(screen.queryByText("checkpoint complete")).toBeNull();
  });

  it.each(["databases", "keyvalue"] as const)(
    "lists Options ▸ Clear logs in a %s tab only on Live tail, and clears as Shift+L does (w3/092)",
    async (kind) => {
      const user = userEvent.setup();
      state.lines = [line(0)];
      render(
        <DatastoreLogViewer
          kind={kind}
          resource="dpg-example"
          createEventSource={(url) => new FakeEventSource(url)}
        />,
      );
      const options = () =>
        user.click(screen.getByRole("button", { name: "Log options" }));
      await options();
      expect(screen.getByRole("menuitem", { name: /Copy logs/ })).toBeVisible();
      expect(screen.queryByRole("menuitem", { name: /Clear logs/ })).toBeNull();
      await user.keyboard("{Escape}");

      await pickRange("Live tail", user);
      await options();
      await user.click(screen.getByRole("menuitem", { name: /Clear logs/ }));
      expect(screen.getByText("Logs cleared")).toBeInTheDocument();
      expect(screen.queryByText("checkpoint complete")).toBeNull();
    },
  );

  it("maximizes on M and offers the Options menu", async () => {
    const user = userEvent.setup();
    state.lines = [line(0)];
    const { container } = render(
      <DatastoreLogViewer kind="keyvalue" resource="red-example" />,
    );
    await user.keyboard("m");
    expect(container.firstElementChild?.className).toContain("fixed inset-0");
    await user.click(screen.getByRole("button", { name: "Log options" }));
    expect(
      screen.getByRole("menuitem", { name: /Copy logs/ }),
    ).not.toHaveAttribute("aria-disabled");
  });
});

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
    render(<DatastoreLogViewer kind="databases" resource="dpg-example" />);
    await user.click(screen.getByRole("combobox", { name: /range/i }));
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

// Render's row menu in a datastore Logs tab (w3/m528): View in context while a
// search or pick narrows the tab, no SSH, and the link's marked line.
describe("DatastoreLogViewer row menu (w3/m528)", () => {
  const openMenu = async (
    user: ReturnType<typeof userEvent.setup>,
    container: HTMLElement,
  ) => {
    fireEvent.pointerEnter(container.querySelector('[data-index="0"]')!);
    await user.click(screen.getByRole("button", { name: "Log line options" }));
  };

  it("offers View in context on a narrowed tab, to the tab's own URL, and never SSH", async () => {
    const user = userEvent.setup();
    state.lines = [line(0)];
    const { container, rerender } = render(
      <DatastoreLogViewer kind="databases" resource="dpg-example" />,
    );
    await openMenu(user, container);
    expect(
      screen.getByRole("menuitem", { name: "Copy log line" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "View in context" }),
    ).toBeNull();
    expect(
      screen.queryByRole("menuitem", { name: "SSH into instance" }),
    ).toBeNull();
    await user.keyboard("{Escape}");

    rerender(
      <DatastoreLogViewer
        kind="databases"
        resource="dpg-example"
        filters={{ text: "", instance: "", level: "warning" }}
        onFiltersChange={() => undefined}
      />,
    );
    await openMenu(user, container);
    const href = screen
      .getByRole("menuitem", { name: "View in context" })
      .getAttribute("href")!;
    const url = new URL(href, "https://dashboard.bex.co");
    expect(url.pathname).toBe("/databases/dpg-example");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      tab: "logs",
      range: "custom",
      rangeStart: "2026-07-15T11:50:00.000Z",
      rangeEnd: "2026-07-15T12:10:00.000Z",
      contextLine: "line-0",
    });
  });

  it("marks the line a View in context link names", () => {
    state.lines = [line(0), line(1)];
    const { container } = render(
      <DatastoreLogViewer
        kind="keyvalue"
        resource="red-example"
        contextLine="line-1"
      />,
    );
    expect(container.querySelector("[data-marked]")).toHaveTextContent(
      "line 1",
    );
  });
});

// Log Explorer Theme (w3/m531): a Postgres or Key Value Logs tab keeps its
// own theme, in its high-contrast twin under High Contrast Mode.
// Live tail is a range (w3/m537).
describe("DatastoreLogViewer Live tail range (w3/m537)", () => {
  it("holds no Live switch, ready or pending, with the pending row's controls", async () => {
    const { DatastoreLogsSkeleton } =
      await import("@/common/components/route-skeletons");
    const ready = render(
      <DatastoreLogViewer kind="databases" resource="dpg-example" />,
    ).container;
    const pending = render(<DatastoreLogsSkeleton />).container;
    for (const root of [ready, pending]) {
      expect(root.querySelector('[role="switch"]')).toBeNull();
    }
    // Range, instance, level, search, then Maximize and Options.
    expect(
      pending.querySelector('[data-skeleton-region="log-filters"]')!.children,
    ).toHaveLength(5);
  });

  it("picks Live tail from New logs in Live Tail, which tails the tab", async () => {
    const streams: FakeEventSource[] = [];
    state.lines = [
      { ...line(0), timestamp: new Date(Date.now() - 60_000).toISOString() },
    ];
    render(
      <DatastoreLogViewer
        kind="databases"
        resource="dpg-example"
        createEventSource={(url) => {
          const s = new FakeEventSource(url);
          streams.push(s);
          return s;
        }}
      />,
    );
    // The watch on Last hour: one stream, whose first newer line brings the
    // button up.
    expect(streams).toHaveLength(1);
    act(() => {
      streams[0].open();
      streams[0].frame({
        message: "a newer line",
        timestamp: new Date().toISOString(),
        labels: [{ name: "type", value: "postgres" }],
      });
    });
    await userEvent.click(
      await screen.findByRole("button", { name: "New logs in Live Tail" }),
    );
    expect(screen.getByRole("combobox", { name: /range/i })).toHaveTextContent(
      "Live tail",
    );
    // The tail opens beside the closed watch.
    expect(streams).toHaveLength(2);
    expect(streams[0].closed).toBe(true);
  });
});

describe("DatastoreLogViewer under Log Explorer Theme (w3/m531)", () => {
  it.each(
    LOG_THEME_CASES.flatMap(([logTheme, highContrast, classes]) =>
      (["databases", "keyvalue"] as const).map(
        (kind) => [kind, logTheme, highContrast, classes] as const,
      ),
    ),
  )(
    "a %s log under %s with High Contrast Mode %s puts %j on its root",
    (kind, logTheme, highContrast, classes) => {
      state.lines = [line(0)];
      const { container } = render(
        <LogThemeContext logTheme={logTheme} highContrast={highContrast}>
          <DatastoreLogViewer
            kind={kind}
            resource={kind === "databases" ? "dpg-example" : "red-example"}
          />
        </LogThemeContext>,
      );
      expect(explorerThemeClasses(container)).toEqual([...classes]);
    },
  );
});
