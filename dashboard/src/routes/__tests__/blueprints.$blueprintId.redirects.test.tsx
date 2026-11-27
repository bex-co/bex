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
import { Route as SubpathRoute } from "../blueprints.$blueprintId.$";
import { Route as AliasRoute } from "../blueprint.$";

// w3/m677: a Blueprint's single page became Render's pages, each at its own
// URL under /blueprints/<id>. Render's own /blueprint/<id>/… links, and its
// sub-paths bex has no page for, open the page they mean, with the query and
// hash kept; a sync has its own page (w3/m678). The real alias and sub-path
// redirects run; each page is a stub naming itself.

const PAGES = ["resources", "syncs", "settings"] as const;

async function land(href: string) {
  const root = createRootRoute({ component: Outlet });
  const stub = (name: string) => () => <p>page:{name}</p>;
  const home = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: stub("home"),
  });
  const create = createRoute({
    getParentRoute: () => root,
    path: "/blueprint/new",
    component: stub("create"),
  });
  const alias = createRoute({
    getParentRoute: () => root,
    path: "/blueprint/$",
    beforeLoad: AliasRoute.options.beforeLoad as never,
  });
  const layout = createRoute({
    getParentRoute: () => root,
    path: "/blueprints/$blueprintId",
    component: Outlet,
  });
  const children = [
    createRoute({
      getParentRoute: () => layout,
      path: "/",
      component: stub("bare"),
    }),
    ...PAGES.map((page) =>
      createRoute({
        getParentRoute: () => layout,
        path: `/${page}`,
        component: stub(page),
      }),
    ),
    createRoute({
      getParentRoute: () => layout,
      path: "/sync/$syncId",
      component: stub("sync"),
    }),
    createRoute({
      getParentRoute: () => layout,
      path: "/sync/",
      component: stub("handed-over sync"),
    }),
    createRoute({
      getParentRoute: () => layout,
      path: "/$",
      beforeLoad: SubpathRoute.options.beforeLoad as never,
    }),
  ];
  const router = createRouter({
    // The file Routes' options are typed for the generated tree, not this one.
    routeTree: root.addChildren([
      home,
      create,
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

const ID = "exs-d0k8jv3e4t2s73a1b2c0";

describe("Render's Blueprint URLs and a Blueprint's sub-paths (w3/m677)", () => {
  it("opens Resources at the bare URL", async () => {
    const location = await land(`/blueprints/${ID}`);
    expect(location.pathname).toBe(`/blueprints/${ID}`);
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it("opens Render's bare /blueprint/<id> on Resources, query and hash kept", async () => {
    const location = await land(`/blueprint/${ID}?from=cli#top`);
    expect(location.pathname).toBe(`/blueprints/${ID}`);
    expect(location.search).toEqual({ from: "cli" });
    expect(location.hash).toBe("top");
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it.each(PAGES)(
    "opens Render's /blueprint/<id>/%s on its page, query kept",
    async (page) => {
      const location = await land(`/blueprint/${ID}/${page}?tab=x`);
      expect(location.pathname).toBe(`/blueprints/${ID}/${page}`);
      expect(location.search).toEqual({ tab: "x" });
      expect(await screen.findByText(`page:${page}`)).toBeVisible();
    },
  );

  it("keeps a blp- bookmark's spelling through the alias", async () => {
    const location = await land("/blueprint/blp-d0k8jv3e4t2s73a1b2c0/syncs");
    expect(location.pathname).toBe(
      "/blueprints/blp-d0k8jv3e4t2s73a1b2c0/syncs",
    );
  });

  // Render's sync link, the dashboardUrl bex-api emits (w3/m678), opens the
  // sync's own page.
  it("opens Render's /blueprint/<id>/sync/<syncId> on the sync's page, query and hash kept", async () => {
    const location = await land(
      `/blueprint/${ID}/sync/exe-d0k8jv3e4t2s73a1b2d0?from=cli#actions`,
    );
    expect(location.pathname).toBe(
      `/blueprints/${ID}/sync/exe-d0k8jv3e4t2s73a1b2d0`,
    );
    expect(location.search).toEqual({ from: "cli" });
    expect(location.hash).toBe("actions");
    expect(await screen.findByText("page:sync")).toBeVisible();
  });

  it("opens Render's /blueprint/<id>/sync on the handed-over sync's page", async () => {
    const location = await land(`/blueprint/${ID}/sync`);
    expect(location.pathname).toBe(`/blueprints/${ID}/sync`);
    expect(await screen.findByText("page:handed-over sync")).toBeVisible();
  });

  it("lands a path past a sync's page on Syncs", async () => {
    const location = await land(
      `/blueprints/${ID}/sync/exe-d0k8jv3e4t2s73a1b2d0/more?from=email#latest`,
    );
    expect(location.pathname).toBe(`/blueprints/${ID}/syncs`);
    expect(location.search).toEqual({ from: "email" });
    expect(location.hash).toBe("latest");
    expect(await screen.findByText("page:syncs")).toBeVisible();
  });

  it("lands any other sub-path on Resources, as Render's * route does", async () => {
    const location = await land(`/blueprints/${ID}/no-such-page?from=docs`);
    expect(location.pathname).toBe(`/blueprints/${ID}`);
    expect(location.search).toEqual({ from: "docs" });
    expect(await screen.findByText("page:bare")).toBeVisible();
  });

  it("keeps /blueprint/new the create page", async () => {
    const location = await land("/blueprint/new");
    expect(location.pathname).toBe("/blueprint/new");
    expect(await screen.findByText("page:create")).toBeVisible();
  });

  // Render's Create Blueprint page with nothing selected goes home; bex's does
  // the same until w3/m681 builds the page.
  it("sends /blueprint/generate home", async () => {
    const location = await land("/blueprint/generate");
    expect(location.pathname).toBe("/");
    expect(await screen.findByText("page:home")).toBeVisible();
  });
});
