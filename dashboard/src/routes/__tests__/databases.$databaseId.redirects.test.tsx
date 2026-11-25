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
import { Route as IndexRoute } from "../databases.$databaseId.index";
import { Route as LogsRoute } from "../databases.$databaseId.logs";
import { Route as MetricsRoute } from "../databases.$databaseId.metrics";
import { Route as SubpathRoute } from "../databases.$databaseId.$";
import { Route as AliasRoute } from "../d.$";

// w3/m641: a Postgres database's single page became Render's pages, each at
// its own URL. The links the single page answered, and Render's own
// sub-paths, still open the page they mean. The real routes' search
// validation and beforeLoad redirects run; each page is a stub naming itself.

const PAGES = ["info", "logs", "metrics", "plan", "apps", "recovery"] as const;

async function land(href: string) {
  const root = createRootRoute({ component: Outlet });
  const alias = createRoute({
    getParentRoute: () => root,
    path: "/d/$",
    beforeLoad: AliasRoute.options.beforeLoad as never,
  });
  const layout = createRoute({
    getParentRoute: () => root,
    path: "/databases/$databaseId",
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

describe("a Postgres database's old links and Render's sub-paths (w3/m641)", () => {
  it("opens Info at the bare URL", async () => {
    const location = await land("/databases/dpg-main");
    expect(location.pathname).toBe("/databases/dpg-main");
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it("sends the old ?tab=logs to the Logs page with its range and filters", async () => {
    const location = await land(
      "/databases/dpg-main?tab=logs&range=custom&rangeStart=2026-07-01T00%3A00%3A00.000Z&rangeEnd=2026-07-01T06%3A00%3A00.000Z&text=boom&level=error",
    );
    expect(location.pathname).toBe("/databases/dpg-main/logs");
    expect(location.search).toEqual({
      range: "custom",
      rangeStart: "2026-07-01T00:00:00.000Z",
      rangeEnd: "2026-07-01T06:00:00.000Z",
      text: "boom",
      level: "error",
    });
    expect(await screen.findByText("page:logs")).toBeVisible();
  });

  it("keeps any other tab value on Info", async () => {
    const location = await land("/databases/dpg-main?tab=metrics");
    expect(location.pathname).toBe("/databases/dpg-main");
  });

  it.each([
    ["metrics", "/metrics"],
    ["insights", "/metrics"],
    ["plan", "/plan"],
    ["high-availability", "/plan"],
    ["apps", "/apps"],
    ["recovery", "/recovery"],
  ])(
    "sends the old #%s anchor to the page its card moved to",
    async (anchor, page) => {
      const location = await land(`/databases/dpg-main#${anchor}`);
      expect(location.pathname).toBe(`/databases/dpg-main${page}`);
      expect(location.hash).toBe(anchor);
    },
  );

  it.each([
    "metadata",
    "connection",
    "sql-console",
    "access-control",
    "danger-zone",
  ])("keeps the #%s anchor on Info, whose card it names", async (anchor) => {
    const location = await land(`/databases/dpg-main#${anchor}`);
    expect(location.pathname).toBe("/databases/dpg-main");
    expect(location.hash).toBe(anchor);
  });

  it.each(PAGES)(
    "opens Render's /d/<id>/%s on its page, query kept",
    async (page) => {
      const location = await land(`/d/dpg-main/${page}?range=24h`);
      expect(location.pathname).toBe(`/databases/dpg-main/${page}`);
      if (page === "logs" || page === "metrics") {
        expect(location.search).toEqual({ range: "24h" });
      }
      expect(await screen.findByText(`page:${page}`)).toBeVisible();
    },
  );

  it.each([
    ["/upgrade?plan=pro#compute", "/plan", { plan: "pro" }, "compute"],
    ["/high-availability-upgrade", "/plan", {}, "high-availability"],
    ["/version-upgrade", "", {}, ""],
    ["/metrics/7d", "/metrics", {}, ""],
    ["/sharing", "", {}, ""],
    ["/no-such-page#connection", "", {}, "connection"],
  ])(
    "lands Render's sub-path %s where its control is",
    async (subpath, page, search, hash) => {
      const location = await land(`/d/dpg-main${subpath}`);
      expect(location.pathname).toBe(`/databases/dpg-main${page}`);
      expect(location.search).toEqual(search);
      expect(location.hash).toBe(hash);
    },
  );
});
