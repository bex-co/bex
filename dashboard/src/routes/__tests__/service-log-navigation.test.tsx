import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import {
  Link,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Route as LogsRoute } from "../services.$serviceId.logs";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// w4/m204: the mounted service Logs viewer kept its filters and paused state
// after a link cleared the URL, so the page and its copied URL named different
// readers. The real route component and validator are driven through a real
// router Link shaped like the service nav's (a target with no search).

const reads = vi.hoisted(
  () => [] as { type: string; text: string; level: string }[],
);
const tails = vi.hoisted(() => [] as { enabled: boolean; text: string }[]);

vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: (
    _resource: string,
    filters: { type: string; text: string; level: string },
  ) => {
    reads.push({
      type: filters.type,
      text: filters.text,
      level: filters.level,
    });
    return {
      lines: [],
      loading: false,
      error: undefined,
      storeUnavailable: false,
      timedOut: false,
      hasMore: false,
      loadingOlder: false,
      loadOlder: () => undefined,
    };
  },
}));
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: (options: { enabled: boolean; text: string }) => {
    tails.push({ enabled: options.enabled, text: options.text });
    return { lines: [], status: "idle" };
  },
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelDiscovery: () => ({ values: [], resolved: false }),
  useLogLabelValues: () => [],
}));
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { id: "srv-one", type: "web_service" },
    loading: false,
  }),
}));

setupVirtualGeometry();

function renderAt(href: string) {
  const root = createRootRoute({
    component: () => (
      <>
        <Link to="/services/$serviceId/logs" params={{ serviceId: "srv-one" }}>
          Logs nav
        </Link>
        <Outlet />
      </>
    ),
  });
  const page = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId/logs",
    validateSearch: LogsRoute.options.validateSearch,
    component: LogsRoute.options.component,
  });
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const searchBox = () => screen.findByLabelText("Search logs");
const FILTERED = "/services/srv-one/logs?type=app&live=0&text=qa-alpha";

beforeEach(() => {
  reads.length = 0;
  tails.length = 0;
});

describe("service Logs follows navigation it did not write (w4/m204)", () => {
  it("shows the clean destination after its nav link clears a filtered URL", async () => {
    const router = renderAt(FILTERED);
    expect(await searchBox()).toHaveValue("qa-alpha");
    expect(reads.at(-1)).toMatchObject({ type: "app", text: "qa-alpha" });
    expect(tails.at(-1)).toMatchObject({ enabled: false });

    fireEvent.click(screen.getByText("Logs nav"));

    await waitFor(() => expect(router.state.location.search).toEqual({}));
    await waitFor(async () => expect(await searchBox()).toHaveValue(""));
    expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "" });
    expect(tails.at(-1)).toEqual({ enabled: true, text: "" });
    // Nothing writes the source's filters back once a debounce has passed.
    await act(() => new Promise((resolve) => setTimeout(resolve, 400)));
    expect(router.state.location.search).toEqual({});
    expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "" });
  });

  it("restores each history entry's filters and paused state on Back and Forward", async () => {
    const router = renderAt(FILTERED);
    expect(await searchBox()).toHaveValue("qa-alpha");
    await act(() =>
      router.navigate({
        to: "/services/$serviceId/logs",
        params: { serviceId: "srv-one" },
        search: { level: "error" },
      }),
    );
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "error" }),
    );
    expect(tails.at(-1)).toMatchObject({ enabled: false });

    act(() => router.history.back());
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({
        type: "app",
        text: "qa-alpha",
        level: "",
      }),
    );
    expect(await searchBox()).toHaveValue("qa-alpha");
    expect(tails.at(-1)).toMatchObject({ enabled: false });

    act(() => router.history.forward());
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "error" }),
    );
    expect(await searchBox()).toHaveValue("");
  });

  it("keeps its own debounced write: typing lands in the URL and stays in the box", async () => {
    const router = renderAt("/services/srv-one/logs?range=12h");
    fireEvent.change(await searchBox(), { target: { value: "boom" } });
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        range: "12h",
        text: "boom",
      }),
    );
    expect(await searchBox()).toHaveValue("boom");
    expect(reads.at(-1)).toMatchObject({ text: "boom" });
    expect(tails.at(-1)).toMatchObject({ enabled: true, text: "boom" });
  });
});
