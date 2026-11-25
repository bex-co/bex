import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import type { RouterContext } from "@/common/types/router-context";
import {
  MANUAL_SCALING_ANCHOR,
  redirectToComputePage,
} from "@/features/services/lib/compute-page";
import { redirectPlanByType } from "@/features/services/lib/service-root-redirect";
import { Route as PlanRoute } from "../services.$serviceId.plan";
import { Route as ScalingRoute } from "../services.$serviceId.scaling";

/**
 * w6/m483: the service's Plan and Scaling tabs became Render's Compute page.
 * Bookmarks and links minted before land there with their query: the Plan URL
 * on the page, the Scaling URL on its Manual Scaling section ("open your
 * service's Compute page and scroll down to the Manual Scaling section",
 * render.com/docs/scaling). A cron job's Plan URL is its own page instead
 * (w3/105, services.$serviceId.cron-routes.test.tsx).
 *
 * A file route's beforeLoad is typed against the registered root and cannot
 * join a synthetic tree, so the synthetic routes call the same production
 * helpers the two files do (the billing-redirects.test.tsx pattern), and the
 * files are checked to call them.
 */
const webService = {
  query: async () => ({ data: { server: { type: "web_service" } } }),
} as unknown as RouterContext["client"];

async function landingFor(initialPath: string): Promise<string> {
  const rootRoute = createRootRoute();
  const plan = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/plan",
    beforeLoad: ({ params, location }) =>
      redirectPlanByType(webService, params.serviceId, location),
  });
  const scaling = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/scaling",
    beforeLoad: ({ params, location }) =>
      redirectToComputePage(params.serviceId, location, MANUAL_SCALING_ANCHOR),
  });
  const compute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/compute",
    component: () => <div>landed on compute</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([plan, scaling, compute]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  await screen.findByText("landed on compute");
  return router.state.location.href;
}

describe("Plan and Scaling URLs open the Compute page (w6/m483)", () => {
  it.each([
    ["/services/srv-1/plan", "/services/srv-1/compute"],
    ["/services/srv-1/plan?lang=de", "/services/srv-1/compute?lang=de"],
    ["/services/srv-1/plan#top", "/services/srv-1/compute#top"],
  ])("Plan %s → %s", async (from, to) => {
    expect(await landingFor(from)).toBe(to);
  });

  it.each([
    ["/services/srv-1/scaling", "/services/srv-1/compute#manual-scaling"],
    [
      "/services/srv-1/scaling?lang=ja",
      "/services/srv-1/compute?lang=ja#manual-scaling",
    ],
    // A hash of the caller's own wins over the section anchor.
    ["/services/srv-1/scaling#top", "/services/srv-1/compute#top"],
  ])("Scaling %s → %s", async (from, to) => {
    expect(await landingFor(from)).toBe(to);
  });

  it("keeps Scaling redirect-only, and both files on their helpers", () => {
    expect(ScalingRoute.options.component).toBeUndefined();
    expect(ScalingRoute.options.pendingComponent).toBeUndefined();
    expect(String(ScalingRoute.options.beforeLoad)).toContain(
      "redirectToComputePage",
    );
    expect(String(ScalingRoute.options.beforeLoad)).toContain(
      "MANUAL_SCALING_ANCHOR",
    );
    expect(String(PlanRoute.options.beforeLoad)).toContain(
      "redirectPlanByType",
    );
  });
});
