import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
// The service the Logs tab belongs to: a web service has request logs.
const served = vi.hoisted(() => ({ type: "web_service" }));

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
  useNewLogsWatch: () => ({ available: false, dismiss: () => undefined }),
}));
// The Logs tab reads the running instances for SSH into instance (w3/m528);
// this route test has no ApolloProvider.
vi.mock("@/features/services/hooks/use-service-instances", () => ({
  useServiceInstances: () => ({
    instances: [],
    loading: false,
    refetch: () => undefined,
  }),
}));
// The plan's log window (w9/m389) is its own query; this route test has no
// ApolloProvider, so the range picker stays unbounded.
vi.mock("@/features/logs/hooks/use-log-retention", () => ({
  useLogRetentionBound: () => undefined,
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelDiscovery: () => ({ values: [], resolved: false }),
  useLogLabelValues: () => [],
}));
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { id: "srv-one", type: served.type },
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

// The Logs page is heavy: on a loaded host its first render can take more
// than findBy's default second, which left an empty body and a failed lookup.
const searchBox = () =>
  screen.findByLabelText("Search logs", undefined, { timeout: 5000 });
// A link written before w3/m537, with the Live switch's `live=0`: it opens
// paused on the range it names, as every range but Live tail is now.
const FILTERED = "/services/srv-one/logs?type=request&live=0&text=qa-alpha";

beforeEach(() => {
  reads.length = 0;
  tails.length = 0;
  served.type = "web_service";
});

describe("service Logs follows navigation it did not write (w4/m204)", () => {
  it("shows the clean destination after its nav link clears a filtered URL", async () => {
    const router = renderAt(FILTERED);
    expect(await searchBox()).toHaveValue("qa-alpha");
    expect(reads.at(-1)).toMatchObject({ type: "request", text: "qa-alpha" });
    expect(tails.at(-1)).toMatchObject({ enabled: false });

    fireEvent.click(screen.getByText("Logs nav"));

    await waitFor(async () => expect(await searchBox()).toHaveValue(""));
    // The clean tab opens on Application logs (w3/101) and Last hour, as
    // Render's web service Logs does.
    expect(reads.at(-1)).toEqual({ type: "app", text: "", level: "" });
    expect(tails.at(-1)).toEqual({ enabled: false, text: "" });
    // Nothing writes the source's filters back once a debounce has passed;
    // the URL names the type it opened on, as Render's (`t=app`) does.
    await act(() => new Promise((resolve) => setTimeout(resolve, 400)));
    expect(router.state.location.search).toEqual({ type: "app" });
    expect(reads.at(-1)).toEqual({ type: "app", text: "", level: "" });
  });

  it("restores each history entry's filters and range on Back and Forward", async () => {
    const router = renderAt(FILTERED);
    expect(await searchBox()).toHaveValue("qa-alpha");
    await act(() =>
      router.navigate({
        to: "/services/$serviceId/logs",
        params: { serviceId: "srv-one" },
        search: { level: "error", range: "live" },
      }),
    );
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({ type: "app", text: "", level: "error" }),
    );
    // A level tails on Live tail on the Application logs view (w8/m147); the
    // paused entry below gets its own range back.
    expect(tails.at(-1)).toMatchObject({ enabled: true });

    act(() => router.history.back());
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({
        type: "request",
        text: "qa-alpha",
        level: "",
      }),
    );
    expect(await searchBox()).toHaveValue("qa-alpha");
    expect(tails.at(-1)).toMatchObject({ enabled: false });

    act(() => router.history.forward());
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({ type: "app", text: "", level: "error" }),
    );
    expect(await searchBox()).toHaveValue("");
  });

  it("keeps its own debounced write: typing lands in the URL and stays in the box", async () => {
    const router = renderAt("/services/srv-one/logs?range=live");
    fireEvent.change(await searchBox(), { target: { value: "boom" } });
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        range: "live",
        type: "app",
        text: "boom",
      }),
    );
    expect(await searchBox()).toHaveValue("boom");
    expect(reads.at(-1)).toMatchObject({ text: "boom" });
    expect(tails.at(-1)).toMatchObject({ enabled: true, text: "boom" });
  });

  // w3/m537: the Live switch's `live=0` is read as the paused range it names,
  // and leaves the URL on the first write, as the `t`/`r` aliases do.
  it("drops a live=0 link's live key on its first write, paused on its range", async () => {
    const router = renderAt("/services/srv-one/logs?range=4h&live=0");
    expect(await searchBox()).toHaveValue("");
    expect(tails.at(-1)).toMatchObject({ enabled: false });
    fireEvent.change(await searchBox(), { target: { value: "boom" } });
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        range: "4h",
        type: "app",
        text: "boom",
      }),
    );
    expect(tails.at(-1)).toMatchObject({ enabled: false, text: "boom" });
  });
});

// w3/101: Render's explorer opens a resource with both app and request logs
// on Application logs unless its URL names another type, and its URL writer
// never names All logs (`t` is deleted for it).
describe("a web service's Logs opens on Application logs (w3/101)", () => {
  const typePicker = () => screen.getByRole("combobox", { name: "Log type" });

  it.each([
    ["names no type", "/services/srv-one/logs", "app", "Application logs"],
    [
      "is Render's t=all",
      "/services/srv-one/logs?t=all",
      "app",
      "Application logs",
    ],
    [
      "names request",
      "/services/srv-one/logs?type=request",
      "request",
      "Request logs",
    ],
    [
      "is Render's t=request",
      "/services/srv-one/logs?t=request",
      "request",
      "Request logs",
    ],
    ["names app", "/services/srv-one/logs?type=app", "app", "Application logs"],
  ])("opens a link that %s on its type", async (_, href, type, label) => {
    renderAt(href);
    await searchBox();
    expect(reads.at(-1)).toEqual({ type, text: "", level: "" });
    expect(typePicker()).toHaveTextContent(label);
  });

  it("keeps All logs picked with no type in the URL, and opens Application logs again on reload", async () => {
    const router = renderAt("/services/srv-one/logs?type=request");
    await searchBox();
    await userEvent.click(typePicker());
    await userEvent.click(screen.getByRole("option", { name: "All logs" }));
    await waitFor(() =>
      expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "" }),
    );
    await act(() => new Promise((resolve) => setTimeout(resolve, 400)));
    expect(router.state.location.search).toEqual({});
    expect(typePicker()).toHaveTextContent("All logs");

    cleanup();
    reads.length = 0;
    renderAt(router.state.location.href);
    await searchBox();
    expect(reads.at(-1)).toEqual({ type: "app", text: "", level: "" });
  });

  it.each(["background_worker", "private_service", "cron_job"])(
    "keeps a %s, which has no request logs, on All logs",
    async (type) => {
      served.type = type;
      renderAt("/services/srv-one/logs");
      await searchBox();
      expect(reads.at(-1)).toEqual({ type: "all", text: "", level: "" });
      expect(typePicker()).toHaveTextContent("All logs");
    },
  );
});
