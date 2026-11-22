import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { RouterContext } from "@/common/types/router-context";
import { redirectRenderAlias, splatParts } from "@/common/lib/render-alias";
import { redirectToComputePage } from "@/features/services/lib/compute-page";
import {
  redirectBuildsByType,
  redirectCronCompute,
  redirectRunsByType,
  redirectServiceRootByType,
} from "@/features/services/lib/service-root-redirect";
import { Route as BuildsRoute } from "../services.$serviceId.builds.$";
import { Route as ComputeRoute } from "../services.$serviceId.compute";
import { Route as DeploysRoute } from "../services.$serviceId.deploys.index";
import { Route as IndexRoute } from "../services.$serviceId.index";
import { Route as RunsRoute } from "../services.$serviceId.runs";

/**
 * w3/m554: a cron job's URLs land where Render's do. Render shows Runs at
 * `/cron/<id>`, replaces `/cron/<id>/builds` with `…/deploys`, keeps a build's
 * detail at `…/builds/<id>`, and has no Compute page for a cron job (its plan is
 * `settings#compute`); docs/render-artifacts/cron-dashboard.md. Every other
 * type keeps its pages.
 *
 * A file route's beforeLoad is typed against the registered root and cannot
 * join a synthetic tree, so the synthetic routes call the production helpers
 * the files call (the compute-redirects test's pattern), and the files are
 * checked to call them.
 */
function clientFor(type: string): RouterContext["client"] {
  return {
    query: async () => ({ data: { server: { type } } }),
  } as unknown as RouterContext["client"];
}

function page(text: string) {
  return () => <div>{text}</div>;
}

async function landingFor(type: string, initialPath: string) {
  const rootRoute = createRootRouteWithContext<RouterContext>()();
  const cronAlias = createRoute({
    getParentRoute: () => rootRoute,
    path: "/cron/$",
    beforeLoad: ({ params, location }) =>
      redirectRenderAlias("cron", params._splat, location),
  });
  const service = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: Outlet,
  });
  const index = createRoute({
    getParentRoute: () => service,
    path: "/",
    beforeLoad: ({ context, params }) =>
      redirectServiceRootByType(context.client, params.serviceId),
  });
  const runs = createRoute({
    getParentRoute: () => service,
    path: "runs",
    beforeLoad: ({ context, params }) =>
      redirectRunsByType(context.client, params.serviceId),
    component: page("runs page"),
  });
  const builds = createRoute({
    getParentRoute: () => service,
    path: "builds/$",
    beforeLoad: ({ context, params }) =>
      redirectBuildsByType(
        context.client,
        params.serviceId,
        splatParts(params._splat)[0],
      ),
  });
  const deploys = createRoute({
    getParentRoute: () => service,
    path: "deploys",
    component: Outlet,
  });
  const deployList = createRoute({
    getParentRoute: () => deploys,
    path: "/",
    component: page("deploys page"),
  });
  const deploy = createRoute({
    getParentRoute: () => deploys,
    path: "$deployId",
    component: () => <div>deploy {deploy.useParams().deployId}</div>,
  });
  const compute = createRoute({
    getParentRoute: () => service,
    path: "compute",
    beforeLoad: ({ context, params }) =>
      redirectCronCompute(context.client, params.serviceId),
    component: page("compute page"),
  });
  const plan = createRoute({
    getParentRoute: () => service,
    path: "plan",
    beforeLoad: ({ params, location }) =>
      redirectToComputePage(params.serviceId, location),
  });
  const settings = createRoute({
    getParentRoute: () => service,
    path: "settings",
    component: page("settings page"),
  });
  const staticEvents = createRoute({
    getParentRoute: () => rootRoute,
    path: "/static/$serviceId/events",
    component: page("static events page"),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      cronAlias,
      service.addChildren([
        index,
        runs,
        builds,
        deploys.addChildren([deployList, deploy]),
        compute,
        plan,
        settings,
      ]),
      staticEvents,
    ]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: clientFor(type), session: null },
  });
  render(<RouterProvider router={router} />);
  const text = await screen.findByText(/ page$|^deploy /);
  return { href: router.state.location.href, page: text.textContent };
}

describe("a cron job's URLs (w3/m554)", () => {
  it.each([
    // Render shows Runs at /cron/<id>; bex lands the bare URL there.
    ["/services/srv-1", "/services/srv-1/runs", "runs page"],
    ["/cron/srv-1", "/services/srv-1/runs", "runs page"],
    ["/cron/srv-1/runs", "/services/srv-1/runs", "runs page"],
    // Builds became Deploys, with the history's filters kept.
    ["/services/srv-1/builds", "/services/srv-1/deploys", "deploys page"],
    [
      "/services/srv-1/builds?status=failed",
      "/services/srv-1/deploys?status=failed",
      "deploys page",
    ],
    ["/cron/srv-1/builds", "/services/srv-1/deploys", "deploys page"],
    ["/services/srv-1/deploys", "/services/srv-1/deploys", "deploys page"],
    // A build's detail is its deploy's.
    [
      "/cron/srv-1/builds/dep-2?r=1h",
      "/services/srv-1/deploys/dep-2?r=1h",
      "deploy dep-2",
    ],
    [
      "/cron/srv-1/deploys/dep-2",
      "/services/srv-1/deploys/dep-2",
      "deploy dep-2",
    ],
    // No Compute page: the plan is Settings' Compute section.
    [
      "/services/srv-1/compute?lang=de",
      "/services/srv-1/settings?lang=de#compute",
      "settings page",
    ],
    [
      "/services/srv-1/plan",
      "/services/srv-1/settings#compute",
      "settings page",
    ],
  ])("%s lands on %s", async (from, href, shown) => {
    expect(await landingFor("cron_job", from)).toEqual({ href, page: shown });
  });
});

describe("every other type keeps its pages (w3/m554)", () => {
  it.each([
    [
      "web_service",
      "/services/srv-1",
      "/services/srv-1/deploys",
      "deploys page",
    ],
    [
      "web_service",
      "/services/srv-1/runs",
      "/services/srv-1/deploys",
      "deploys page",
    ],
    [
      "private_service",
      "/services/srv-1/builds",
      "/services/srv-1/deploys",
      "deploys page",
    ],
    [
      "background_worker",
      "/services/srv-1/compute",
      "/services/srv-1/compute",
      "compute page",
    ],
    [
      "web_service",
      "/services/srv-1/plan",
      "/services/srv-1/compute",
      "compute page",
    ],
    [
      "static_site",
      "/services/srv-1/runs",
      "/static/srv-1/events",
      "static events page",
    ],
  ])("a %s's %s lands on %s", async (type, from, href, shown) => {
    expect(await landingFor(type, from)).toEqual({ href, page: shown });
  });
});

describe("the route files call those helpers", () => {
  it.each([
    [IndexRoute, "redirectServiceRootByType"],
    [RunsRoute, "redirectRunsByType"],
    [BuildsRoute, "redirectBuildsByType"],
    [ComputeRoute, "redirectCronCompute"],
  ] as const)("%#", (route, helper) => {
    expect(String(route.options.beforeLoad)).toContain(helper);
  });

  it("keeps Builds redirect-only, and Deploys free of a cron redirect", () => {
    expect(BuildsRoute.options.component).toBeUndefined();
    expect(BuildsRoute.options.pendingComponent).toBeUndefined();
    expect(String(BuildsRoute.options.beforeLoad)).toContain("_splat");
    expect(DeploysRoute.options.beforeLoad).toBeUndefined();
  });
});
