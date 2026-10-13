import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Link,
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Sidebar, SidebarProvider } from "@/common/components/ui/sidebar";
import { DashboardHeader } from "../dashboard-header";

// w4/283: the shell (one SidebarProvider + header) persists across routes, so
// a link inside the mobile drawer navigated while the drawer stayed open over
// the destination. The real provider, Sheet, header and router Links are used;
// only the header's unrelated children are stubbed.

const viewport = vi.hoisted(() => ({ mobile: true }));
vi.mock("@/common/hooks/use-mobile", () => ({
  useIsMobile: () => viewport.mobile,
}));
vi.mock("../dashboard-breadcrumbs", () => ({
  DashboardBreadcrumbs: () => null,
}));
vi.mock("../global-search", () => ({ GlobalSearch: () => null }));
vi.mock("../new-resource-menu", () => ({ NewResourceMenu: () => null }));
vi.mock("@/common/components/authenticated", () => ({
  Authenticated: () => null,
}));
vi.mock("@/common/components/user-nav.tsx", () => ({ UserNav: () => null }));

function renderShell(initial = "/billing") {
  const root = createRootRoute({
    component: () => (
      <SidebarProvider>
        <Sidebar>
          <Link to="/billing">Billing</Link>
          <Link to="/settings">Settings</Link>
          <Link to="/settings" search={{ flow: "members" }}>
            Members
          </Link>
          <Link to="/settings" hash="danger">
            Danger zone
          </Link>
        </Sidebar>
        <main>
          <DashboardHeader />
          <Outlet />
        </main>
      </SidebarProvider>
    ),
  });
  const page = (path: string, title: string) =>
    createRoute({
      getParentRoute: () => root,
      path,
      validateSearch: (search: Record<string, unknown>) =>
        typeof search.flow === "string" ? { flow: search.flow } : {},
      component: () => <h1>{title}</h1>,
    });
  const router = createRouter({
    routeTree: root.addChildren([
      page("/billing", "Billing page"),
      page("/settings", "Settings page"),
    ]),
    history: createMemoryHistory({ initialEntries: [initial] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

const drawer = () => screen.queryByRole("dialog");
async function openDrawer(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    await screen.findByRole("button", { name: /toggle sidebar/i }),
  );
  expect(drawer()).toBeInTheDocument();
}
async function drawerGone() {
  await waitFor(() => expect(drawer()).not.toBeInTheDocument());
  // With the drawer gone the header offers its trigger again.
  expect(
    screen.getByRole("button", { name: /toggle sidebar/i }),
  ).toBeInTheDocument();
}

beforeEach(() => {
  viewport.mobile = true;
});

describe("mobile drawer after navigation (w4/283)", () => {
  it("closes once a drawer link reaches another page", async () => {
    const user = userEvent.setup();
    renderShell();
    await openDrawer(user);
    await user.click(screen.getByRole("link", { name: "Settings" }));
    expect(
      await screen.findByRole("heading", { name: "Settings page" }),
    ).toBeInTheDocument();
    await drawerGone();
  });

  it("closes when the link is followed from the keyboard", async () => {
    const user = userEvent.setup();
    renderShell("/settings");
    await openDrawer(user);
    screen.getByRole("link", { name: "Billing" }).focus();
    await user.keyboard("{Enter}");
    expect(
      await screen.findByRole("heading", { name: "Billing page" }),
    ).toBeInTheDocument();
    await drawerGone();
  });

  it.each(["Members", "Danger zone"])(
    "closes when only the query or hash changes (%s)",
    async (name) => {
      const user = userEvent.setup();
      const router = renderShell("/settings");
      await openDrawer(user);
      const before = router.state.location.href;
      await user.click(screen.getByRole("link", { name }));
      await waitFor(() => expect(router.state.location.href).not.toBe(before));
      await drawerGone();
    },
  );

  it("closes on Back to the previous entry", async () => {
    const user = userEvent.setup();
    const router = renderShell();
    await openDrawer(user);
    await user.click(screen.getByRole("link", { name: "Settings" }));
    await drawerGone();
    await openDrawer(user);
    act(() => router.history.back());
    expect(
      await screen.findByRole("heading", { name: "Billing page" }),
    ).toBeInTheDocument();
    await drawerGone();
  });

  it("stays open when nothing navigates: opening, a link to the current page, a modified click", async () => {
    const user = userEvent.setup();
    const router = renderShell();
    await openDrawer(user);
    await user.click(screen.getByRole("link", { name: "Billing" }));
    expect(drawer()).toBeInTheDocument();
    // A modified click belongs to the browser (new tab): this page stays put.
    await user.keyboard("{Meta>}");
    await user.click(screen.getByRole("link", { name: "Settings" }));
    await user.keyboard("{/Meta}");
    expect(router.state.location.pathname).toBe("/billing");
    expect(drawer()).toBeInTheDocument();
  });

  it("still closes by Escape, and reopens", async () => {
    const user = userEvent.setup();
    renderShell();
    await openDrawer(user);
    await user.keyboard("{Escape}");
    await drawerGone();
    await openDrawer(user);
  });

  it("leaves the desktop rail's expanded state alone when navigating", async () => {
    viewport.mobile = false;
    const user = userEvent.setup();
    renderShell();
    const rail = () =>
      document.querySelector('[data-slot="sidebar"][data-state]');
    await screen.findByRole("heading", { name: "Billing page" });
    const before = rail()?.getAttribute("data-state");
    expect(before).toBeTruthy();
    await user.click(screen.getByRole("link", { name: "Settings" }));
    await screen.findByRole("heading", { name: "Settings page" });
    expect(drawer()).not.toBeInTheDocument();
    expect(rail()?.getAttribute("data-state")).toBe(before);
  });
});
