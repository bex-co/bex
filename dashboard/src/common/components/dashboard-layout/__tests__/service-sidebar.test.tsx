import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { ServiceSidebar } from "../service-sidebar";
import type { ServiceView } from "@/features/services/types";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  contributorCapabilities,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// The sidebar reads the same single-service hook as the detail shell, so the
// two agree on whether the service exists (w1/m45).
const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

// What the cache already holds — the workspace's service list after a click
// from it — which the header's loading frame reads too (w3/119).
const cached: { name: string | null; type: string | null } = {
  name: null,
  type: null,
};
vi.mock("@/features/services/hooks/use-cached-service-type", () => ({
  useCachedServiceName: () => cached.name,
  useCachedServiceType: () => cached.type,
}));

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const serviceRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId/$",
    component: () => (
      <SidebarProvider>
        <ServiceSidebar serviceId="srv-1" />
      </SidebarProvider>
    ),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([serviceRoute]),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = {
    id: "srv-1",
    name: "storefront-api",
    type: "web_service",
  } as ServiceView;
  serverState.loading = false;
  cached.name = null;
  cached.type = null;
});

// w9/m621, w9/145: Render's role table gives the Billing role neither "View
// service events" nor "View service metrics", and neither a Viewer nor a
// Billing member "View service logs"; a Contributor has all three.
const linkNames = () =>
  screen
    .getAllByRole("link")
    .map((link) => link.textContent)
    .filter(Boolean);

describe("ServiceSidebar per role (w9/m621, w9/145)", () => {
  it("drops Deploys, Events, Logs and Metrics for a Billing member once the role is confirmed", async () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    renderAt("/services/srv-1/settings");

    await screen.findByRole("link", { name: "Settings" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Settings",
      "Environment",
      "Disk",
      "Shell",
      "Compute",
    ]);
  });

  it("drops Logs alone for a Viewer", async () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    renderAt("/services/srv-1/settings");

    await screen.findByRole("link", { name: "Settings" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Deploys",
      "Settings",
      "Events",
      "Metrics",
      "Environment",
      "Disk",
      "Shell",
      "Compute",
    ]);
  });

  it("keeps every entry for a Contributor", async () => {
    vi.mocked(useCapabilities).mockReturnValue(contributorCapabilities());
    renderAt("/services/srv-1/settings");

    await screen.findByRole("link", { name: "Settings" });
    expect(linkNames()).toEqual([
      "Dashboard",
      "Deploys",
      "Settings",
      "Events",
      "Logs",
      "Metrics",
      "Environment",
      "Disk",
      "Shell",
      "Compute",
    ]);
  });

  it("keeps every entry while capabilities are still checking", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        loading: true,
        loaded: false,
        canViewActivity: false,
        canViewLogs: false,
      }),
    );
    renderAt("/services/srv-1/settings");

    expect(await screen.findByRole("link", { name: "Deploys" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Metrics" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Logs" })).toBeVisible();
  });
});

describe("ServiceSidebar (w1/m45 — Render's resource-scoped service nav)", () => {
  it("renders Render's grouping: top items, Monitor, Manage", async () => {
    renderAt("/services/srv-1/logs");

    // Back link + service name replace the global nav.
    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toHaveAttribute("href", "/");
    expect(screen.getByText("storefront-api")).toBeInTheDocument();

    // Navigation: top items {Deploys, Settings}, Monitor {Events, Logs,
    // Metrics}, Manage {Environment, Disk, Shell, Compute}.
    expect(screen.getByText("Monitor")).toBeInTheDocument();
    expect(screen.getByText("Manage")).toBeInTheDocument();
    for (const [name, href, iconClass] of [
      ["Deploys", "/services/srv-1/deploys", "lucide-rocket"],
      ["Settings", "/services/srv-1/settings", "lucide-settings"],
      ["Events", "/services/srv-1/events", "lucide-activity"],
      ["Logs", "/services/srv-1/logs", "lucide-scroll-text"],
      ["Metrics", "/services/srv-1/metrics", "lucide-chart-no-axes-combined"],
      ["Environment", "/services/srv-1/env", "lucide-braces"],
      ["Disk", "/services/srv-1/disk", "lucide-hard-drive"],
      ["Shell", "/services/srv-1/shell", "lucide-square-terminal"],
      ["Compute", "/services/srv-1/compute", "lucide-cpu"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link.querySelector("svg")).toHaveClass(iconClass);
    }

    expect(
      screen
        .getAllByRole("link")
        .map((link) => link.textContent)
        .filter(Boolean),
    ).toEqual([
      "Dashboard",
      "Deploys",
      "Settings",
      "Events",
      "Logs",
      "Metrics",
      "Environment",
      "Disk",
      "Shell",
      "Compute",
    ]);
  });

  it("hides the service nav (not the back link) for a service the caller can't see", async () => {
    serverState.service = null;
    renderAt("/services/srv-1/logs");

    expect(
      await screen.findByRole("link", { name: "Dashboard" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Events" }),
    ).not.toBeInTheDocument();
    // The header still names what was asked for (the id, no resolved name).
    expect(screen.getByText("srv-1")).toBeInTheDocument();
  });

  it("keeps the nav while the service is still loading (no empty flash)", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderAt("/services/srv-1/logs");

    expect(
      await screen.findByRole("link", { name: "Events" }),
    ).toBeInTheDocument();
    // Type-specific entries wait for the type: a static site must never flash
    // Shell/Compute (w5/m48/t001).
    expect(
      screen.queryByRole("link", { name: "Shell" }),
    ).not.toBeInTheDocument();
  });

  // Render's cron sidebar (w3/m554, docs/render-artifacts/cron-dashboard.md):
  // Runs, Deploys, Settings / Events, Logs, Metrics / Environment, Shell, with
  // Deploys highlighted on a deploy's detail as on the list.
  describe("a cron job", () => {
    beforeEach(() => {
      serverState.service = {
        id: "srv-1",
        name: "nightly-report",
        type: "cron_job",
      } as ServiceView;
    });

    const active = () =>
      screen
        .getAllByRole("link")
        .filter((link) => link.getAttribute("data-active") === "true")
        .map((link) => link.textContent);

    it("lists Render's entries in Render's order, with no Compute", async () => {
      renderAt("/services/srv-1/runs");

      expect(await screen.findByRole("link", { name: "Runs" })).toHaveAttribute(
        "href",
        "/services/srv-1/runs",
      );
      expect(screen.getByRole("link", { name: "Deploys" })).toHaveAttribute(
        "href",
        "/services/srv-1/deploys",
      );
      expect(
        screen
          .getAllByRole("link")
          .map((link) => link.textContent)
          .filter(Boolean),
      ).toEqual([
        "Dashboard",
        "Runs",
        "Deploys",
        "Settings",
        "Events",
        "Logs",
        "Metrics",
        "Environment",
        "Shell",
      ]);
      expect(active()).toEqual(["Runs"]);
    });

    it.each([
      ["/services/srv-1/deploys", "Deploys"],
      ["/services/srv-1/deploys/dep-1", "Deploys"],
      ["/services/srv-1/settings", "Settings"],
    ])("highlights %s under %s", async (path, entry) => {
      renderAt(path);

      await screen.findByRole("link", { name: "Runs" });
      expect(active()).toEqual([entry]);
    });
  });

  it("type-gates a static site: Events-first, no Deploys/Logs/Shell/Compute, adds Redirects + Headers (w5/m48, w5/m57)", async () => {
    serverState.service = {
      id: "srv-1",
      name: "docs-site",
      type: "static_site",
    } as ServiceView;
    renderAt("/services/srv-1/settings");

    expect(
      await screen.findByRole("link", { name: "Redirects/Rewrites" }),
    ).toHaveAttribute("href", "/services/srv-1/redirects");
    expect(screen.getByRole("link", { name: "Headers" })).toHaveAttribute(
      "href",
      "/services/srv-1/headers",
    );

    // Render's static sidebar leads with Events and has no Deploys tab (deploy
    // history/detail is reached from the Events feed) nor any runtime-instance
    // tab (w5/m57).
    expect(
      screen.queryByRole("link", { name: "Deploys" }),
    ).not.toBeInTheDocument();
    expect(
      screen
        .getAllByRole("link")
        .map((link) => link.textContent)
        .filter(Boolean),
    ).toEqual([
      "Dashboard",
      "Events",
      "Settings",
      "Metrics",
      "Environment",
      "Redirects/Rewrites",
      "Headers",
    ]);
  });
});

// w3/119: a click from the service list paints the header's frame from the
// list cache while the `Server` read loads; the sidebar beside it must already
// be the ready page's, not the id over a type-less nav.
describe("ServiceSidebar while the read loads (w3/119)", () => {
  // The title is the paragraph between the back link and the nav.
  const titleText = () =>
    screen
      .getByRole("link", { name: "Dashboard" })
      .parentElement?.querySelector("p")?.textContent;

  async function pendingThenReady(
    pathname: string,
    service: { name: string; type: string },
  ) {
    serverState.service = null;
    serverState.loading = true;
    cached.name = service.name;
    cached.type = service.type;
    const view = renderAt(pathname);
    await screen.findByRole("link", { name: "Settings" });
    const pending = { title: titleText(), links: linkNames() };

    view.unmount();
    serverState.service = { id: "srv-1", ...service } as ServiceView;
    serverState.loading = false;
    renderAt(pathname);
    await screen.findByRole("link", { name: "Settings" });
    return { pending, ready: { title: titleText(), links: linkNames() } };
  }

  it.each([
    [
      "a cron job",
      { name: "nightly-report", type: "cron_job" },
      "/services/srv-1/runs",
    ],
    [
      "a web service",
      { name: "storefront-api", type: "web_service" },
      "/services/srv-1/settings",
    ],
    [
      "a static site",
      { name: "docs-site", type: "static_site" },
      "/services/srv-1/settings",
    ],
  ])(
    "names %s and draws its nav from the cache, as the ready page does",
    async (_label, service, pathname) => {
      const { pending, ready } = await pendingThenReady(pathname, service);
      expect(pending.title).toBe(service.name);
      expect(pending).toEqual(ready);
    },
  );

  it("keeps the type-less nav and the id when nothing is cached", async () => {
    serverState.service = null;
    serverState.loading = true;
    renderAt("/services/srv-1/settings");

    await screen.findByRole("link", { name: "Settings" });
    expect(titleText()).toBe("srv-1");
    expect(linkNames()).toEqual([
      "Dashboard",
      "Settings",
      "Events",
      "Logs",
      "Metrics",
      "Environment",
    ]);
  });

  it("shows the id and no nav once the read settles on nothing, even with the service still in the list cache", async () => {
    serverState.service = null;
    serverState.loading = false;
    cached.name = "deleted-api";
    cached.type = "web_service";
    renderAt("/services/srv-1/settings");

    await screen.findByRole("link", { name: "Dashboard" });
    expect(titleText()).toBe("srv-1");
    expect(linkNames()).toEqual(["Dashboard"]);
  });
});
