import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import {
  DeployHistorySkeleton,
  DeployHistoryView,
} from "../deploy-history-view";
import { autoDeployOff } from "../../lib/auto-deploy-off";

// w3/064: Render's Deploys page "banner flagging when auto-deploys are
// disabled" (changelog 2026-09-03).
const serverState: {
  repo: string | null;
  autoDeploy: boolean | null;
  branch: string | null;
  pushDeliveryMethod?: string | null;
} = { repo: null, autoDeploy: null, branch: null };
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({
    service: { ...serverState },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("../deploys-list-page", () => ({
  DeploysListPage: () => <div data-testid="deploy-list" />,
}));

function renderAt(path: string, pending = false) {
  const root = createRootRoute();
  const page = (base: "/services" | "/static") =>
    pending ? (
      <DeployHistorySkeleton />
    ) : (
      <DeployHistoryView
        serviceId="web"
        search={{}}
        replaceSearch={() => {}}
        base={base}
      />
    );
  const routes = [
    createRoute({
      getParentRoute: () => root,
      path: "/services/$serviceId/deploys",
      component: () => page("/services"),
    }),
    createRoute({
      getParentRoute: () => root,
      path: "/static/$serviceId/deploys",
      component: () => page("/static"),
    }),
    createRoute({
      getParentRoute: () => root,
      path: "/services/$serviceId/settings",
      component: () => null,
    }),
    createRoute({
      getParentRoute: () => root,
      path: "/static/$serviceId/settings",
      component: () => null,
    }),
  ];
  const router = createRouter({
    routeTree: root.addChildren(routes),
    history: createMemoryHistory({ initialEntries: [path] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

describe("auto-deploy-off banner", () => {
  beforeEach(() => {
    Object.assign(serverState, {
      repo: null,
      autoDeploy: null,
      branch: null,
      pushDeliveryMethod: null,
    });
  });

  it("flags only a repo-backed service whose auto-deploy is off", () => {
    expect(
      autoDeployOff({ repo: "https://github.com/a/b", autoDeploy: false }),
    ).toBe(true);
    expect(
      autoDeployOff({ repo: "https://github.com/a/b", autoDeploy: true }),
    ).toBe(false);
    expect(
      autoDeployOff({ repo: "https://github.com/a/b", autoDeploy: null }),
    ).toBe(false);
    expect(autoDeployOff({ repo: null, autoDeploy: false })).toBe(false);
    expect(autoDeployOff(null)).toBe(false);
  });

  it("shows above the list, names the branch and links to the service's Settings", async () => {
    Object.assign(serverState, {
      repo: "https://github.com/acme/web",
      autoDeploy: false,
      branch: "main",
    });
    renderAt("/services/web/deploys");
    const banner = await screen.findByTestId("auto-deploy-off-banner");
    expect(banner).toHaveTextContent("Auto-deploy is off");
    expect(banner).toHaveTextContent(
      "Pushes to main don't deploy this service",
    );
    expect(
      screen.getByRole("link", { name: "Auto-Deploy settings" }),
    ).toHaveAttribute("href", "/services/web/settings");
    expect(screen.getByTestId("deploy-list")).toBeInTheDocument();
    expect(
      banner.compareDocumentPosition(screen.getByTestId("deploy-list")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps a static site's link in its own tree", async () => {
    Object.assign(serverState, {
      repo: "https://github.com/acme/site",
      autoDeploy: false,
      branch: "main",
    });
    renderAt("/static/web/deploys");
    expect(
      await screen.findByRole("link", { name: "Auto-Deploy settings" }),
    ).toHaveAttribute("href", "/static/web/settings");
  });

  it("is absent when auto-deploy is on, and for an image-backed service", async () => {
    Object.assign(serverState, {
      repo: "https://github.com/acme/web",
      autoDeploy: true,
      branch: "main",
    });
    renderAt("/services/web/deploys");
    await screen.findByTestId("deploy-list");
    expect(screen.queryByTestId("auto-deploy-off-banner")).toBeNull();
  });

  it("the pending state shows the same banner, so the list does not shift", async () => {
    Object.assign(serverState, {
      repo: "https://github.com/acme/web",
      autoDeploy: false,
      branch: "main",
    });
    renderAt("/services/web/deploys", true);
    const banner = await screen.findByTestId("auto-deploy-off-banner");
    expect(banner).toHaveTextContent(
      "Pushes to main don't deploy this service",
    );
  });

  it("the pending state has no banner when the ready page has none", async () => {
    Object.assign(serverState, { repo: null, autoDeploy: null, branch: null });
    const { container } = renderAt("/services/web/deploys", true);
    await waitFor(() =>
      expect(
        container.querySelector('[data-route-skeleton="deploys-list"]'),
      ).not.toBeNull(),
    );
    expect(
      container.querySelector('[data-testid="auto-deploy-off-banner"]'),
    ).toBeNull();
  });

  // w10/m375: a repo whose pushes can't reach bex deploys manually, as
  // Render's public Git URLs do, so the banner says so and links to no setting.
  it.each([false, true])(
    "for a repo whose pushes can't reach bex, says deploys are manual with no link (pending: %s)",
    async (pending) => {
      Object.assign(serverState, {
        repo: "https://github.com/someone-else/side-project",
        autoDeploy: false,
        branch: "main",
        pushDeliveryMethod: "unavailable",
      });
      renderAt("/services/web/deploys", pending);
      const banner = await screen.findByTestId("auto-deploy-off-banner");
      expect(banner).toHaveTextContent("Deploys are manual");
      expect(banner).toHaveTextContent(
        "bex doesn't receive this repo's pushes",
      );
      expect(screen.queryByRole("link")).toBeNull();
    },
  );

  it("keeps the Settings link for a covered repo", async () => {
    Object.assign(serverState, {
      repo: "https://github.com/acme/web",
      autoDeploy: false,
      branch: "main",
      pushDeliveryMethod: "github_app",
    });
    renderAt("/services/web/deploys");
    expect(
      await screen.findByRole("link", { name: "Auto-Deploy settings" }),
    ).toBeInTheDocument();
  });
});
