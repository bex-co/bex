import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { ProjectLogViewer } from "../project-log-viewer";
import type { UseLogHistoryResult } from "../../hooks/use-log-history";
import {
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
  type LogLine,
  type ProjectLogScope,
} from "../../types";
import { DEFAULT_LOG_RANGE } from "../../lib/log-search";
import { setupVirtualGeometry } from "@/test/virtual-geometry";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { ProjectLogViewPageSkeleton } from "@/common/components/route-skeletons";
import {
  explorerThemeClasses,
  LOG_THEME_CASES,
  LogThemeContext,
} from "@/test/log-theme";

// A project observability view's explorer (w3/m505): the history and the tail
// read the view's scope, each line names its resource, and every state reads
// differently — none says "no logs" for a read that failed.

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
// The paused view's new-line watch (w3/m528).
const watchState = { available: false, dismiss: vi.fn() };
const useNewLogsWatchSpy = vi.fn();
vi.mock("../../hooks/use-live-logs", () => ({
  useLiveLogs: (...args: unknown[]) => {
    useLiveLogsSpy(...args);
    return { lines: [], status: "open" };
  },
  useNewLogsWatch: (...args: unknown[]) => {
    useNewLogsWatchSpy(...args);
    return watchState;
  },
}));
const retentionSpy = vi.fn();
vi.mock("../../hooks/use-log-retention", () => ({
  useLogRetentionBound: (resource: string) => {
    retentionSpy(resource);
    return undefined;
  },
}));
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    params,
    children,
  }: {
    to: string;
    params?: Record<string, string>;
    children: ReactNode;
  }) => (
    <a href={to.replace(/\$(\w+)/g, (_, key: string) => params?.[key] ?? "")}>
      {children}
    </a>
  ),
}));

setupVirtualGeometry();

const SCOPE: ProjectLogScope = {
  project: "prj-observed",
  filterDocument: { filters: [], version: 1 },
};

function line(over: Partial<LogLine>): LogLine {
  return {
    key: over.key ?? "k",
    timestamp: new Date().toISOString(),
    instance: "",
    resource: "",
    type: "app",
    level: "",
    method: "",
    statusCode: "",
    host: "",
    path: "",
    message: "",
    spans: null,
    ...over,
  };
}

function viewer(logType: typeof LOG_TYPE_APP | typeof LOG_TYPE_REQUEST) {
  return render(
    <ProjectLogViewer
      scope={SCOPE}
      logType={logType}
      range={DEFAULT_LOG_RANGE}
      onRangeChange={() => undefined}
    />,
  );
}

const resources = [
  {
    id: "srv-web",
    name: "storefront",
    type: "web_service",
    environmentId: null,
  },
  { id: "dpg-db", name: "orders-db", type: "postgres", environmentId: null },
];

beforeEach(() => {
  Object.assign(historyState, {
    lines: [],
    loading: false,
    error: undefined,
    storeUnavailable: false,
    timedOut: false,
    hasMore: false,
    resources: undefined,
  });
  useHistorySpy.mockReset();
  useLiveLogsSpy.mockReset();
  retentionSpy.mockReset();
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("ProjectLogViewer", () => {
  it("reads the view's scope with its log type, history and tail alike", () => {
    viewer(LOG_TYPE_REQUEST);
    const [resource, filters, , , , scope] = useHistorySpy.mock.calls.at(-1)!;
    expect(resource).toBe("");
    expect(filters.type).toBe("request");
    expect(scope).toBe(SCOPE);
    expect(useLiveLogsSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({
        project: SCOPE,
        type: "request",
        enabled: true,
      }),
    );
  });

  it("names each line's resource and bounds the range by the project's plan", () => {
    historyState.resources = resources;
    historyState.lines = [
      line({ key: "1", resource: "srv-web", message: "web line" }),
      line({ key: "2", resource: "dpg-db", message: "pg line" }),
    ];
    viewer(LOG_TYPE_APP);
    expect(screen.getByRole("link", { name: "[storefront]" })).toHaveAttribute(
      "href",
      "/services/srv-web",
    );
    expect(screen.getByRole("link", { name: "[orders-db]" })).toHaveAttribute(
      "href",
      "/databases/dpg-db",
    );
    expect(retentionSpy).toHaveBeenLastCalledWith("srv-web");
  });

  it("says a view with nothing in it is empty, apart from a quiet range", () => {
    historyState.resources = [];
    viewer(LOG_TYPE_REQUEST);
    expect(
      screen.getByText("Nothing to read in this view yet"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Request Logs read the project's web services, and there are none in this view yet.",
      ),
    ).toBeInTheDocument();
  });

  // w3/078: bex-api reads a caller without log access an empty scope, so a
  // Viewer's view must not claim the project has nothing in it.
  it("tells a Viewer their role reads no logs, not that the view is empty", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canViewLogs: false }),
    );
    historyState.resources = [];
    viewer(LOG_TYPE_APP);
    expect(screen.getByText("You can't view logs")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Logs in this workspace need the Contributor role or higher. Ask a workspace admin to change your role.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Nothing to read in this view yet"),
    ).not.toBeInTheDocument();
  });

  it.each([
    ["a Contributor", mockCapabilities({ role: "CONTRIBUTOR" })],
    [
      "a role not yet confirmed",
      mockCapabilities({ canViewLogs: false, loaded: false, loading: true }),
    ],
  ])("keeps the empty-view copy for %s", (_, capabilities) => {
    vi.mocked(useCapabilities).mockReturnValue(capabilities);
    historyState.resources = [];
    viewer(LOG_TYPE_APP);
    expect(
      screen.getByText("Nothing to read in this view yet"),
    ).toBeInTheDocument();
    expect(screen.queryByText("You can't view logs")).not.toBeInTheDocument();
  });

  it("shows the lines a page read, whatever the role answer says", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canViewLogs: false }),
    );
    historyState.resources = resources;
    historyState.lines = [
      line({ key: "1", resource: "srv-web", message: "web line" }),
    ];
    viewer(LOG_TYPE_APP);
    expect(screen.getByText("web line")).toBeInTheDocument();
    expect(screen.queryByText("You can't view logs")).not.toBeInTheDocument();
  });

  it("says a quiet range is quiet", () => {
    historyState.resources = resources;
    viewer(LOG_TYPE_APP);
    expect(screen.getByText("No logs in this range")).toBeInTheDocument();
    expect(
      screen.queryByText("Nothing to read in this view yet"),
    ).not.toBeInTheDocument();
  });

  it("shows the Pro-plan refusal for Request Logs on Hobby, with no tail", () => {
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
    viewer(LOG_TYPE_REQUEST);
    expect(
      screen.getByText("HTTP request logs require the Pro plan"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(screen.queryByText("No logs in this range")).not.toBeInTheDocument();
    expect(useLiveLogsSpy).toHaveBeenLastCalledWith(
      expect.objectContaining({ enabled: false }),
    );
  });

  it("never reads a failed or timed-out read as an empty one", () => {
    historyState.error = new Error("upstream exploded");
    historyState.resources = [];
    const { unmount } = viewer(LOG_TYPE_APP);
    expect(screen.getByText("Couldn't load logs")).toBeInTheDocument();
    expect(
      screen.queryByText("Nothing to read in this view yet"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("No logs in this range")).not.toBeInTheDocument();
    unmount();

    historyState.error = undefined;
    historyState.timedOut = true;
    viewer(LOG_TYPE_APP);
    expect(screen.queryByText("No logs in this range")).not.toBeInTheDocument();
  });

  it("says the log store is missing rather than empty", () => {
    historyState.storeUnavailable = true;
    viewer(LOG_TYPE_REQUEST);
    expect(
      screen.getByText("Request logs need the log store"),
    ).toBeInTheDocument();
  });

  it("turns a typed level: operator into a chip and the rest into text", async () => {
    vi.useFakeTimers();
    try {
      viewer(LOG_TYPE_APP);
      fireEvent.change(screen.getByRole("textbox", { name: "Search logs" }), {
        target: { value: "level:error timeout" },
      });
      await act(async () => vi.advanceTimersByTime(300));
      const filters = useHistorySpy.mock.calls.at(-1)![1];
      expect(filters).toMatchObject({ level: "error", text: "timeout" });
      expect(screen.getByText("Level: error")).toBeInTheDocument();
      // Application Logs carry no request line, so a request operator is
      // not sent.
      fireEvent.change(screen.getByRole("textbox", { name: "Search logs" }), {
        target: { value: "method:GET" },
      });
      await act(async () => vi.advanceTimersByTime(300));
      expect(useHistorySpy.mock.calls.at(-1)![1].method).toBe("");
    } finally {
      vi.useRealTimers();
    }
  });

  // The loading panel fills the screen, so a read that answers without lines
  // keeps that box instead of collapsing to a short card under it, as the
  // service Logs tab does (production's walk at 1440 and 390, w3/m505).
  const planRefusal = () =>
    new CombinedGraphQLErrors({
      errors: [
        {
          message: "HTTP request logs require the Pro plan or higher",
          extensions: { code: "PLAN_FEATURE_REQUIRED", feature: "requestLogs" },
        },
      ],
    } as never);
  it.each([
    [
      "the Pro-plan refusal",
      LOG_TYPE_REQUEST,
      "HTTP request logs require the Pro plan",
      () => {
        historyState.error = planRefusal();
      },
    ],
    [
      "a missing log store",
      LOG_TYPE_REQUEST,
      "Request logs need the log store",
      () => {
        historyState.storeUnavailable = true;
      },
    ],
    [
      "a timed-out read",
      LOG_TYPE_APP,
      "This search took too long",
      () => {
        historyState.timedOut = true;
      },
    ],
    [
      "a failed read",
      LOG_TYPE_APP,
      "Couldn't load logs",
      () => {
        historyState.error = new Error("upstream exploded");
      },
    ],
    [
      "a view with nothing in it",
      LOG_TYPE_APP,
      "Nothing to read in this view yet",
      () => {
        historyState.resources = [];
      },
    ],
    [
      "a quiet range",
      LOG_TYPE_APP,
      "No logs in this range",
      () => {
        historyState.resources = resources;
      },
    ],
  ] as const)(
    "draws %s in the loading panel's own box",
    (_, logType, title, arrange) => {
      historyState.loading = true;
      const loading = viewer(logType);
      const panelHeight = loading
        .getByRole("status", { name: "Loading logs…" })
        .querySelector<HTMLElement>("[style]")?.style.height;
      expect(panelHeight).toMatch(/^\d+px$/);
      loading.unmount();

      historyState.loading = false;
      arrange();
      viewer(logType);
      expect(
        screen.getByText(title).closest<HTMLElement>("[style]")?.style.height,
      ).toBe(panelHeight);
    },
  );

  // The Live control sets the toolbar's height: 32px on a desktop, a 44px
  // touch target below lg, where it shares a phone's second row with the
  // range. Production's walk at 390 (w3/m505) found the pending frame's Live
  // row 20px tall, so the ready view landed 12px lower.
  it("clears the live tail on Shift+L and maximizes from its toolbar (w3/m509)", () => {
    historyState.lines = [line({ key: "a", message: "from the web" })];
    historyState.resources = resources;
    const { container } = viewer(LOG_TYPE_APP);
    fireEvent.keyDown(window, { key: "L", shiftKey: true });
    expect(screen.getByText("Logs cleared")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Maximize" }));
    expect(container.firstElementChild?.className).toContain("fixed inset-0");
  });

  it("clears the live tail from Options ▸ Clear logs, and lists none once paused (w3/092)", async () => {
    const user = userEvent.setup();
    historyState.lines = [line({ key: "a", message: "from the web" })];
    historyState.resources = resources;
    viewer(LOG_TYPE_APP);
    const options = () =>
      user.click(screen.getByRole("button", { name: "Log options" }));
    await options();
    await user.click(screen.getByRole("menuitem", { name: /Clear logs/ }));
    expect(screen.getByText("Logs cleared")).toBeInTheDocument();

    await user.click(screen.getByRole("switch", { name: /live/i }));
    expect(screen.getByText("from the web")).toBeInTheDocument();
    await options();
    expect(screen.getByRole("menuitem", { name: /Copy logs/ })).toBeVisible();
    expect(screen.queryByRole("menuitem", { name: /Clear logs/ })).toBeNull();
  });

  it("reserves the ready Maximize and Options buttons in the pending frame (w3/m509)", () => {
    const sizes = (root: HTMLElement) =>
      [...root.querySelectorAll<HTMLElement>("button, [data-slot=skeleton]")]
        .filter((el) => el.className.includes("max-lg:size-11"))
        .map((el) =>
          el.className
            .split(/\s+/)
            .filter((name) => /^(\w+:)*size-/.test(name))
            .sort()
            .join(" "),
        );
    const ready = sizes(viewer(LOG_TYPE_APP).container);
    expect(ready).toHaveLength(2);
    expect(sizes(render(<ProjectLogViewPageSkeleton />).container)).toEqual(
      ready,
    );
  });

  it("reserves the ready Live control's height in the pending frame at every width", () => {
    const liveRowHeights = (root: HTMLElement) =>
      within(root)
        .getByRole("switch", { hidden: true })
        .closest("label")
        ?.className.split(/\s+/)
        .filter((name) => /(^|:)min-h-/.test(name))
        .sort();
    const ready = liveRowHeights(viewer(LOG_TYPE_APP).container);
    expect(ready).toEqual(
      expect.arrayContaining(["min-h-8", "max-lg:min-h-11"]),
    );
    expect(
      liveRowHeights(render(<ProjectLogViewPageSkeleton />).container),
    ).toEqual(ready);
  });
});

// Render's explorer pieces in a project view (w3/m528): a View in context link
// opens paused on its marked line, the link is offered only while a search
// narrows the view, and a project view offers no SSH (Render passes
// `servicePath: null`).
describe("ProjectLogViewer row menu and paused watch (w3/m528)", () => {
  beforeEach(() => {
    watchState.available = false;
    useNewLogsWatchSpy.mockReset();
  });
  const watchArgs = () =>
    useNewLogsWatchSpy.mock.calls.at(-1)?.[0] as {
      enabled: boolean;
      subscription: Record<string, unknown>;
    };

  it("opens a View in context link paused on its marked line, watching the view for a new one", () => {
    historyState.lines = [line({ key: "id:a", message: "first" })];
    const { container } = render(
      <ProjectLogViewer
        scope={SCOPE}
        logType={LOG_TYPE_APP}
        range={DEFAULT_LOG_RANGE}
        onRangeChange={() => undefined}
        contextLine="id:a"
      />,
    );
    expect(screen.getByRole("switch", { name: /live/i })).not.toBeChecked();
    expect(container.querySelector("[data-marked]")).toHaveTextContent("first");
    expect(watchArgs().enabled).toBe(true);
    expect(watchArgs().subscription).toMatchObject({
      project: SCOPE,
      type: LOG_TYPE_APP,
    });
  });

  it("offers View in context once a search narrows the view, and never SSH", async () => {
    const user = userEvent.setup();
    historyState.lines = [
      line({ key: "id:a", message: "first", instance: "srv-web-abcde" }),
    ];
    const contextHref = vi.fn(() => "/ctx");
    const { container } = render(
      <ProjectLogViewer
        scope={SCOPE}
        logType={LOG_TYPE_APP}
        range={DEFAULT_LOG_RANGE}
        onRangeChange={() => undefined}
        contextHref={contextHref}
      />,
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
      screen.queryByRole("menuitem", { name: "SSH into instance" }),
    ).toBeNull();
    await user.keyboard("{Escape}");
    await user.type(
      screen.getByRole("textbox", { name: "Search logs" }),
      "first",
    );
    await act(() => new Promise((resolve) => setTimeout(resolve, 350)));
    await openMenu();
    expect(
      screen.getByRole("menuitem", { name: "View in context" }),
    ).toHaveAttribute("href", "/ctx");
  });
});

// Log Explorer Theme (w3/m531): a project's Application Logs and Request
// Logs views keep their own theme, in its high-contrast twin under High
// Contrast Mode.
describe("ProjectLogViewer under Log Explorer Theme (w3/m531)", () => {
  it.each(
    LOG_THEME_CASES.flatMap(([logTheme, highContrast, classes]) =>
      ([LOG_TYPE_APP, LOG_TYPE_REQUEST] as const).map(
        (logType) => [logType, logTheme, highContrast, classes] as const,
      ),
    ),
  )(
    "the %s view under %s with High Contrast Mode %s puts %j on its root",
    (logType, logTheme, highContrast, classes) => {
      const { container } = render(
        <LogThemeContext logTheme={logTheme} highContrast={highContrast}>
          <ProjectLogViewer
            scope={SCOPE}
            logType={logType}
            range={DEFAULT_LOG_RANGE}
            onRangeChange={() => undefined}
          />
        </LogThemeContext>,
      );
      expect(explorerThemeClasses(container)).toEqual([...classes]);
    },
  );
});
