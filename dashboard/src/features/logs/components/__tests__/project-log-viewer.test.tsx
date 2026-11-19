import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
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
};
const useHistorySpy = vi.fn();
vi.mock("../../hooks/use-log-history", () => ({
  useLogHistory: (...args: unknown[]) => {
    useHistorySpy(...args);
    return historyState;
  },
}));
const useLiveLogsSpy = vi.fn();
vi.mock("../../hooks/use-live-logs", () => ({
  useLiveLogs: (...args: unknown[]) => {
    useLiveLogsSpy(...args);
    return { lines: [], status: "open" };
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
});
