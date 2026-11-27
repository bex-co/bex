import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type AnyRoute,
} from "@tanstack/react-router";
import { Route as WebAlias } from "../web.$";
import { Route as WorkerAlias } from "../worker.$";
import { Route as PservAlias } from "../pserv.$";
import { Route as CronAlias } from "../cron.$";
import { Route as ServiceSubpath } from "../services.$serviceId.$";
import { Route as StaticSubpath } from "../static.$serviceId.$";
import { Route as LogsRoute } from "../services.$serviceId.logs";
import { Route as MetricsRoute } from "../services.$serviceId.metrics";
import { Route as ShellRoute } from "../services.$serviceId.shell";
import { Route as EventsRoute } from "../services.$serviceId.events";
import { Route as StaticMetricsRoute } from "../static.$serviceId.metrics";

// w3/m707: a link Render's dashboard builds into a service's sub-page opens
// the same bex page in the same state. The real alias routes, sub-path
// landings and search validators run; each page is a stub naming itself, and
// the router client answers the service's type.

const SERVICE_PAGES = ["disk", "logs", "metrics", "shell", "events", "deploys"];
const VALIDATORS: Record<string, AnyRoute> = {
  logs: LogsRoute as unknown as AnyRoute,
  metrics: MetricsRoute as unknown as AnyRoute,
  shell: ShellRoute as unknown as AnyRoute,
  events: EventsRoute as unknown as AnyRoute,
};

async function land(href: string, type = "web_service") {
  const root = createRootRoute({ component: Outlet });
  const route = (path: string, from: AnyRoute, parent: AnyRoute = root) =>
    createRoute({
      getParentRoute: () => parent,
      path,
      validateSearch: from.options.validateSearch as never,
      beforeLoad: from.options.beforeLoad as never,
    }) as unknown as AnyRoute;
  const stub = (parent: AnyRoute, path: string, from?: AnyRoute) =>
    createRoute({
      getParentRoute: () => parent,
      path,
      validateSearch: from?.options.validateSearch as never,
      component: () => <p>page:{path}</p>,
    }) as unknown as AnyRoute;
  const services = createRoute({
    getParentRoute: () => root,
    path: "/services/$serviceId",
    component: Outlet,
  }) as unknown as AnyRoute;
  const statics = createRoute({
    getParentRoute: () => root,
    path: "/static/$serviceId",
    component: Outlet,
  }) as unknown as AnyRoute;
  const router = createRouter({
    routeTree: root.addChildren([
      route("/web/$", WebAlias as unknown as AnyRoute),
      route("/worker/$", WorkerAlias as unknown as AnyRoute),
      route("/pserv/$", PservAlias as unknown as AnyRoute),
      route("/cron/$", CronAlias as unknown as AnyRoute),
      services.addChildren([
        ...SERVICE_PAGES.map((page) => stub(services, page, VALIDATORS[page])),
        stub(services, "deploys/$deployId"),
        route("$", ServiceSubpath as unknown as AnyRoute, services),
      ]),
      statics.addChildren([
        stub(statics, "events"),
        stub(statics, "metrics", StaticMetricsRoute as unknown as AnyRoute),
        route("$", StaticSubpath as unknown as AnyRoute, statics),
      ]),
    ]),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: {
      client: {
        query: async () => ({ data: { server: { type } } }),
      } as never,
      session: null,
    },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router.state.location;
}

describe("a service's Render sub-page links (w3/m707)", () => {
  it.each(["web", "worker", "pserv", "cron"])(
    "opens /%s/<id>/disks on the Disk page, query and hash kept",
    async (segment) => {
      const location = await land(`/${segment}/srv-a/disks?from=1#size`);
      expect(location.pathname).toBe("/services/srv-a/disk");
      expect(location.search).toEqual({ from: 1 });
      expect(location.hash).toBe("size");
      expect(await screen.findByText("page:disk")).toBeVisible();
    },
  );

  it("opens a legacy Metrics period on its window", async () => {
    const location = await land("/worker/srv-a/metrics/1w");
    expect(location.pathname).toBe("/services/srv-a/metrics");
    expect(location.search).toEqual({ range: "7d" });
  });

  it("opens Shell on the instance the link names", async () => {
    const location = await land("/pserv/srv-a/shell/lqtnn");
    expect(location.pathname).toBe("/services/srv-a/shell");
    expect(location.search).toEqual({ instance: "srv-a-lqtnn" });
    expect(await screen.findByText("page:shell")).toBeVisible();
  });

  it("opens a deploy's page for its sub-path", async () => {
    const location = await land("/web/srv-a/deploys/dep-b/logs");
    expect(location.pathname).toBe("/services/srv-a/deploys/dep-b");
  });

  it.each(["jobs", "previews", "pulls", "sharing", "no-such-page"])(
    "sends /web/<id>/%s to the service's Deploys, as Render's * route sends it to the root",
    async (page) => {
      const location = await land(`/web/srv-a/${page}`);
      expect(location.pathname).toBe("/services/srv-a/deploys");
    },
  );

  it("opens Render's Instance failed Logs link filtered to the instance and window", async () => {
    const location = await land(
      `/web/srv-a/logs?l=${encodeURIComponent('[["INSTANCE","srv-a-lqtnn"]]')}&r=2026-11-27%4009%3A00%3A00~2026-11-27%4010%3A01%3A00`,
    );
    expect(location.pathname).toBe("/services/srv-a/logs");
    expect(location.search).toMatchObject({
      instance: "srv-a-lqtnn",
      range: "custom",
      rangeStart: "2026-11-27T09:00:00.000Z",
      rangeEnd: "2026-11-27T10:01:00.000Z",
    });
  });

  it("opens Render's Metrics t and Events et", async () => {
    expect((await land("/web/srv-a/metrics?t=1h")).search).toMatchObject({
      range: "1h",
    });
    expect(
      (await land("/web/srv-a/events?et=DeployEnded")).search,
    ).toMatchObject({ et: "DeployEnded" });
  });

  it("opens a static site's legacy Metrics period, and its Events for a page it has not", async () => {
    const metrics = await land("/static/srv-s/metrics/24h", "static_site");
    expect(metrics.pathname).toBe("/static/srv-s/metrics");
    expect(metrics.search).toEqual({ range: "24h" });
    expect(
      (await land("/static/srv-s/shell/lqtnn", "static_site")).pathname,
    ).toBe("/static/srv-s/events");
  });
});
