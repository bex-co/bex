import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { Route as IndexRoute } from "../keyvalue.$keyValueId.index";
import { Route as LogsRoute } from "../keyvalue.$keyValueId.logs";
import { Route as MetricsRoute } from "../keyvalue.$keyValueId.metrics";
import { Route as SubpathRoute } from "../keyvalue.$keyValueId.$";
import { Route as AliasRoute } from "../r.$";

// w3/m649: a Key Value instance's single page became Render's pages, each at
// its own URL. The links the single page answered, and Render's own
// sub-paths, still open the page they mean. The real routes' search
// validation and beforeLoad redirects run; each page is a stub naming itself.

const PAGES = ["info", "logs", "metrics"] as const;

async function land(href: string) {
  const root = createRootRoute({ component: Outlet });
  const alias = createRoute({
    getParentRoute: () => root,
    path: "/r/$",
    beforeLoad: AliasRoute.options.beforeLoad as never,
  });
  const layout = createRoute({
    getParentRoute: () => root,
    path: "/keyvalue/$keyValueId",
    component: Outlet,
  });
  const stub = (name: string) => () => <p>page:{name}</p>;
  const children = [
    createRoute({
      getParentRoute: () => layout,
      path: "/",
      validateSearch: IndexRoute.options.validateSearch as never,
      beforeLoad: IndexRoute.options.beforeLoad as never,
      component: stub("bare"),
    }),
    ...PAGES.map((page) =>
      createRoute({
        getParentRoute: () => layout,
        path: `/${page}`,
        validateSearch: (page === "logs"
          ? LogsRoute.options.validateSearch
          : page === "metrics"
            ? MetricsRoute.options.validateSearch
            : undefined) as never,
        component: stub(page),
      }),
    ),
    createRoute({
      getParentRoute: () => layout,
      path: "/$",
      beforeLoad: SubpathRoute.options.beforeLoad as never,
    }),
  ];
  const router = createRouter({
    // The file Routes' options are typed for the generated tree, not this one.
    routeTree: root.addChildren([
      alias as never,
      layout.addChildren(children as never) as never,
    ]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router.state.location;
}

describe("a Key Value instance's old links and Render's sub-paths (w3/m649)", () => {
  it("opens Info at the bare URL", async () => {
    const location = await land("/keyvalue/red-main");
    expect(location.pathname).toBe("/keyvalue/red-main");
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it("sends the old ?tab=logs to the Logs page with its range and filters", async () => {
    const location = await land(
      "/keyvalue/red-main?tab=logs&range=custom&rangeStart=2026-07-01T00%3A00%3A00.000Z&rangeEnd=2026-07-01T06%3A00%3A00.000Z&text=boom&instance=red-main-0",
    );
    expect(location.pathname).toBe("/keyvalue/red-main/logs");
    expect(location.search).toEqual({
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
      text: "boom",
      instance: "red-main-0",
    });
    expect(await screen.findByText("page:logs")).toBeVisible();
  });

  it("sends the old ?tab=metrics to the Metrics page with its range", async () => {
    const location = await land("/keyvalue/red-main?tab=metrics&range=7d");
    expect(location.pathname).toBe("/keyvalue/red-main/metrics");
    expect(location.search).toEqual({ range: "7d" });
    expect(await screen.findByText("page:metrics")).toBeVisible();
  });

  it("keeps any other tab value on Info", async () => {
    const location = await land("/keyvalue/red-main?tab=overview");
    expect(location.pathname).toBe("/keyvalue/red-main");
  });

  it.each([
    "metadata",
    "connection",
    "networking",
    "plan",
    "maxmemory-policy",
    "persistence-mode",
    "danger-zone",
  ])(
    "keeps the old #%s anchor on Info, whose card it names",
    async (anchor) => {
      const location = await land(`/keyvalue/red-main#${anchor}`);
      expect(location.pathname).toBe("/keyvalue/red-main");
      expect(location.hash).toBe(anchor);
    },
  );

  it.each(PAGES)(
    "opens Render's /r/<id>/%s on its page, query kept",
    async (page) => {
      const location = await land(`/r/red-main/${page}?range=24h`);
      expect(location.pathname).toBe(`/keyvalue/red-main/${page}`);
      if (page === "logs" || page === "metrics") {
        expect(location.search).toEqual({ range: "24h" });
      }
      expect(await screen.findByText(`page:${page}`)).toBeVisible();
    },
  );

  it("opens Render's bare /r/<id> on Info", async () => {
    const location = await land("/r/red-main");
    expect(location.pathname).toBe("/keyvalue/red-main");
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it("keeps Render's Upgrade instance link /info#compute on Info", async () => {
    const location = await land("/r/red-main/info#compute");
    expect(location.pathname).toBe("/keyvalue/red-main/info");
    expect(location.hash).toBe("compute");
  });

  it.each([
    ["/plan", "", {}, "plan"],
    ["/plan?from=badge", "", { from: "badge" }, "plan"],
    // Render's period table has no 7d: the default window (w3/m707).
    ["/metrics/7d", "/metrics", {}, ""],
    // Its `1w` is its 7-day window, kept over the link's own query.
    ["/metrics/1w?range=1h", "/metrics", { range: "7d" }, ""],
    ["/metrics/5m", "/metrics", { range: "30m" }, ""],
    ["/no-such-page#connection", "", {}, "connection"],
  ])(
    "lands Render's sub-path %s where its control is",
    async (subpath, page, search, hash) => {
      const location = await land(`/r/red-main${subpath}`);
      expect(location.pathname).toBe(`/keyvalue/red-main${page}`);
      expect(location.search).toEqual(search);
      expect(location.hash).toBe(hash);
    },
  );
});
