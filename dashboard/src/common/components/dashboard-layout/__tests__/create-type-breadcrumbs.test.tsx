import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeAll, describe, expect, it } from "vitest";
import {
  SERVICE_TYPE_CREATE_PATHS,
  parseNewServiceSearch,
} from "@/features/services/lib/create-context";
import { DashboardBreadcrumbs } from "../dashboard-breadcrumbs";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
});

// Render's create pages carry the type in the breadcrumb, "New Web Service ▾",
// whose menu ("Choose a service type") switches to another create type (live
// capture 2026-11-21); bex's create pages dropped their in-page type cards for
// it.
function renderAt(href: string) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routes = [
    ...Object.values(SERVICE_TYPE_CREATE_PATHS).map((path) =>
      createRoute({
        getParentRoute: () => root,
        path,
        component: DashboardBreadcrumbs,
        validateSearch: parseNewServiceSearch,
      }),
    ),
    ...["/new/database", "/new/redis", "/"].map((path) =>
      createRoute({
        getParentRoute: () => root,
        path,
        component: DashboardBreadcrumbs,
      }),
    ),
  ];
  const router = createRouter({
    routeTree: root.addChildren(routes),
    history: createMemoryHistory({ initialEntries: [href] }),
    context: {} as never,
  });
  render(<RouterProvider router={router} />);
  return router;
}

describe("create page breadcrumb type switcher", () => {
  it("names the page's type and lists every create type, the current one marked", async () => {
    const user = userEvent.setup();
    renderAt("/cron/new");

    const trigger = await screen.findByRole("button", {
      name: "New Cron Job: choose a service type",
    });
    expect(trigger).toHaveTextContent("New Cron Job");
    await user.click(trigger);

    const items = screen.getAllByRole("menuitem");
    expect(items.map((item) => item.textContent)).toEqual([
      "Static Site",
      "Web Service",
      "Private Service",
      "Background Worker",
      "Cron Job",
      "Postgres",
      "Key Value",
    ]);
    expect(screen.getByRole("menuitem", { name: "Cron Job" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("menuitem", { name: "Web Service" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("switches service types between Render's create paths, keeping the page's context", async () => {
    const user = userEvent.setup();
    const router = renderAt("/web/new?projectId=prj-a&environmentId=env-a");
    await user.click(
      await screen.findByRole("button", { name: /choose a service type/ }),
    );
    const menu = screen.getByRole("menu");
    const href = within(menu)
      .getByRole("menuitem", { name: "Background Worker" })
      .getAttribute("href")!;
    const url = new URL(href, "http://x");
    expect(url.pathname).toBe("/worker/new");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      projectId: "prj-a",
      environmentId: "env-a",
    });

    await user.click(
      within(menu).getByRole("menuitem", { name: "Background Worker" }),
    );
    expect(router.state.location.pathname).toBe("/worker/new");
    expect(router.state.location.search).toMatchObject({ projectId: "prj-a" });
    // replace: the type switch adds no history entry.
    expect(router.history.length).toBe(1);
  });

  it("links the datastores to their own create pages", async () => {
    const user = userEvent.setup();
    renderAt("/new/redis");
    await user.click(
      await screen.findByRole("button", {
        name: "New Key Value: choose a service type",
      }),
    );
    expect(screen.getByRole("menuitem", { name: "Postgres" })).toHaveAttribute(
      "href",
      "/new/database",
    );
    expect(
      screen.getByRole("menuitem", { name: "Web Service" }),
    ).toHaveAttribute("href", "/web/new");
    expect(screen.getByRole("menuitem", { name: "Key Value" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("leaves other pages' breadcrumbs alone", async () => {
    renderAt("/");
    expect(
      await screen.findByRole("navigation", { name: /breadcrumb/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /choose a service type/ }),
    ).toBeNull();
  });
});
