import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { SidebarProvider } from "@/common/components/ui/sidebar.tsx";
import { DashboardSidebar } from "../dashboard-sidebar";
import type { UseBlueprintResult } from "@/features/blueprints/hooks/use-blueprint";
import type { BlueprintView } from "@/features/blueprints/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  billingCapabilities,
  mockCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// w3/m677: a Blueprint's pages swap the workspace nav for Render's Blueprint
// sidebar. DashboardSidebar picks it on the route's `blueprintId`; it reads
// the same Blueprint the layout reads, so the two agree on whether it exists.
const blueprintState: UseBlueprintResult = {
  blueprint: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(),
};
const blueprintReads = vi.hoisted(() => [] as { id: string; poll?: boolean }[]);
vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: (id: string, options?: { poll?: boolean }) => {
    blueprintReads.push({ id, poll: options?.poll });
    return blueprintState;
  },
}));
vi.mock(
  "@/common/components/dashboard-layout/use-router-nav-available",
  () => ({
    useRouterNavAvailable: () => false,
  }),
);

function renderAt(pathname: string) {
  const rootRoute = createRootRoute();
  const sidebar = () => (
    <SidebarProvider>
      <DashboardSidebar />
    </SidebarProvider>
  );
  const routes = [
    "/blueprints/$blueprintId/$",
    "/blueprints/$blueprintId",
    "/blueprints/new",
    "/blueprint/new",
    "/blueprint/generate",
  ].map((path) =>
    createRoute({ getParentRoute: () => rootRoute, path, component: sidebar }),
  );
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: [pathname] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

const linkNames = () =>
  screen
    .getAllByRole("link")
    .map((link) => link.textContent)
    .filter(Boolean);
const activeNames = () =>
  screen
    .getAllByRole("link")
    .filter((link) => link.getAttribute("data-active") === "true")
    .map((link) => link.textContent);

beforeEach(() => {
  blueprintState.blueprint = {
    id: "exs-1",
    name: "platform",
  } as BlueprintView;
  blueprintState.loading = false;
  blueprintReads.length = 0;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("BlueprintSidebar (w3/m677, Render's Blueprint sidebar)", () => {
  it("draws the Blueprints back link, the name, then Resources / Syncs / Settings", async () => {
    renderAt("/blueprints/exs-1/syncs");

    expect(
      await screen.findByRole("link", { name: "Blueprints" }),
    ).toHaveAttribute("href", "/blueprints");
    expect(screen.getByText("platform")).toBeInTheDocument();
    for (const [name, href, iconClass] of [
      ["Resources", "/blueprints/exs-1/resources", "lucide-boxes"],
      ["Syncs", "/blueprints/exs-1/syncs", "lucide-refresh-ccw"],
      ["Settings", "/blueprints/exs-1/settings", "lucide-settings"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link.querySelector("svg")).toHaveClass(iconClass);
    }
    expect(linkNames()).toEqual([
      "Blueprints",
      "Resources",
      "Syncs",
      "Settings",
    ]);
    // The workspace nav is gone.
    expect(screen.queryByRole("link", { name: "Projects" })).toBeNull();
    // The layout's own read, served from Apollo's cache with no second poll.
    expect(blueprintReads.length).toBeGreaterThan(0);
    expect(blueprintReads.every((read) => read.id === "exs-1")).toBe(true);
    expect(blueprintReads.every((read) => read.poll === false)).toBe(true);
  });

  it.each([
    ["/blueprints/exs-1", "Resources"],
    ["/blueprints/exs-1/resources", "Resources"],
    ["/blueprints/exs-1/syncs", "Syncs"],
    ["/blueprints/exs-1/sync/exe-9", "Syncs"],
    ["/blueprints/exs-1/settings", "Settings"],
  ])("highlights the open page on %s", async (path, open) => {
    renderAt(path);
    await screen.findByRole("link", { name: "Resources" });
    expect(activeNames()).toEqual([open]);
  });

  it("reads a blp- bookmark by its exs- id and links under the URL's spelling", async () => {
    renderAt("/blueprints/blp-db136288mmqc73d4hpug/settings");

    expect(await screen.findByRole("link", { name: "Syncs" })).toHaveAttribute(
      "href",
      "/blueprints/blp-db136288mmqc73d4hpug/syncs",
    );
    expect(activeNames()).toEqual(["Settings"]);
    expect(
      blueprintReads.every((read) => read.id === "exs-db136288mmqc73d4hpug"),
    ).toBe(true);
  });

  it("draws every entry while the Blueprint loads, so nothing shifts", async () => {
    blueprintState.blueprint = null;
    blueprintState.loading = true;
    renderAt("/blueprints/exs-1/settings");

    await screen.findByRole("link", { name: "Resources" });
    expect(linkNames()).toEqual([
      "Blueprints",
      "Resources",
      "Syncs",
      "Settings",
    ]);
    // The name waits for the read; the id stands in.
    expect(screen.getByText("exs-1")).toBeInTheDocument();
  });

  it("hides the Blueprint's nav (not the back link) for a Blueprint the caller can't see", async () => {
    blueprintState.blueprint = null;
    renderAt("/blueprints/exs-1/syncs");

    expect(
      await screen.findByRole("link", { name: "Blueprints" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Resources" })).toBeNull();
  });

  // As on Render, no entry depends on a permission.
  it.each([
    ["a Viewer", viewerCapabilities],
    ["a Billing member", billingCapabilities],
  ])("keeps every entry for %s", async (_, capabilities) => {
    vi.mocked(useCapabilities).mockReturnValue(capabilities());
    renderAt("/blueprints/exs-1");

    await screen.findByRole("link", { name: "Resources" });
    expect(linkNames()).toEqual([
      "Blueprints",
      "Resources",
      "Syncs",
      "Settings",
    ]);
  });

  it.each(["/blueprint/new", "/blueprints/new", "/blueprint/generate"])(
    "leaves the create page %s on the workspace sidebar",
    async (path) => {
      renderAt(path);

      expect(
        await screen.findByRole("link", { name: "Projects" }),
      ).toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "Resources" })).toBeNull();
      expect(blueprintReads).toEqual([]);
    },
  );
});
