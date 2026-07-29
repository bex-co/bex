import { describe, expect, it } from "vitest";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { routeTree } from "@/routeTree.gen";
import { redirectRenderAlias } from "@/common/lib/render-alias";

interface RuntimeRoute {
  fullPath: string;
  children?: RuntimeRoute[] | Record<string, RuntimeRoute>;
}

// Render-shaped dashboard deep links (docs/render-artifacts/dashboard-routes.md)
// must each have a splat alias route so `/web/srv-x/deploys/dep-y`-style URLs
// resolve instead of falling through to the 404 catch-all.
describe("Render dashboard-route aliases", () => {
  // Building the router decorates every route with its resolved fullPath.
  createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  const root = routeTree as unknown as RuntimeRoute;
  const topLevel = Object.values(root.children ?? {});

  it.each([
    ["/web/$"],
    ["/worker/$"],
    ["/pserv/$"],
    // /static is NOT a splat alias — w5/m57 made static sites a canonical route
    // tree (/static/$serviceId); its mounting is asserted separately below.
    ["/cron/$"],
    ["/d/$"],
    ["/r/$"],
    // w1/m45: workspace/user-scoped + create-flow + auth shapes
    ["/w/$"],
    ["/u/$"],
    ["/billing/$"],
    ["/login"],
    ["/register"],
    ["/new/database"],
    ["/new/redis"],
    ["/new/project"],
  ])("mounts the %s alias", (fullPath) => {
    expect(topLevel.find((route) => route.fullPath === fullPath)).toBeDefined();
  });

  it("keeps the root 404 catch-all for unknown segments", () => {
    expect(topLevel.find((route) => route.fullPath === "/$")).toBeDefined();
  });

  // w5/m57: static sites are a canonical route tree under /static/$serviceId
  // (Render's /static/<id> scheme), not a splat alias. The layout, its tab
  // pages, the deploy detail, and the create URL must all mount.
  function collectPaths(node: RuntimeRoute, acc: string[] = []): string[] {
    if (node.fullPath) acc.push(node.fullPath);
    for (const child of Object.values(node.children ?? {})) {
      collectPaths(child as RuntimeRoute, acc);
    }
    return acc;
  }
  const allPaths = collectPaths(root);

  it.each([
    ["/static/$serviceId"],
    ["/static/$serviceId/events"],
    ["/static/$serviceId/settings"],
    ["/static/$serviceId/metrics"],
    ["/static/$serviceId/env"],
    ["/static/$serviceId/redirects"],
    ["/static/$serviceId/headers"],
    ["/static/$serviceId/deploys/$deployId"],
    ["/static/new"],
  ])("mounts the static-site canonical route %s", (fullPath) => {
    expect(allPaths).toContain(fullPath);
  });

  // w1/m45: Webhooks/Notifications became first-class pages (Render's
  // Integrations sidebar entries), not aliases — but their routes must mount.
  it.each([["/webhooks"], ["/notifications"]])(
    "mounts the %s page",
    (fullPath) => {
      expect(
        topLevel.find((route) => route.fullPath === fullPath),
      ).toBeDefined();
    },
  );

  // Prove the redirect actually navigates: a minimal router (no auth guards)
  // wired exactly like the alias route files, loaded on a Render-shaped deploy
  // deep link, must settle on the canonical route with the query preserved.
  it("navigates /web/{id}/deploys/{dep} to the canonical deploy route", async () => {
    const rootRoute = createRootRoute();
    const alias = createRoute({
      getParentRoute: () => rootRoute,
      path: "/web/$",
      beforeLoad: ({ params, location }) => {
        redirectRenderAlias("web", params._splat, location);
      },
    });
    const target = createRoute({
      getParentRoute: () => rootRoute,
      path: "/services/$serviceId/deploys/$deployId",
      component: () => null,
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([alias, target]),
      history: createMemoryHistory({
        initialEntries: ["/web/srv-1/deploys/dep-2?x=1"],
      }),
    });
    await router.load();
    expect(router.state.location.pathname).toBe(
      "/services/srv-1/deploys/dep-2",
    );
    expect(router.state.location.search).toEqual({ x: 1 });
  });
});
