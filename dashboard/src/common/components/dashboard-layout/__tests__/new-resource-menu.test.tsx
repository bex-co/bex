import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it } from "vitest";
import { Route as IndexRoute } from "@/routes/index";
import { NewResourceMenu } from "../new-resource-menu";

// The topbar's "+ New" menu is Render's persistent create menu. Its entries
// either open a create page or land on the overview with the create dialog the
// URL owns (`/?new=…`).
function renderMenu() {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routes = [
    createRoute({
      getParentRoute: () => root,
      path: "/",
      component: NewResourceMenu,
      // The overview's own validator, so the link serializes as the app's does.
      validateSearch: IndexRoute.options.validateSearch,
    }),
    ...[
      "/static/new",
      "/web/new",
      "/pserv/new",
      "/worker/new",
      "/cron/new",
      "/new/database",
      "/new/redis",
      "/blueprint/new",
      "/webhooks/new",
    ].map((path) =>
      createRoute({
        getParentRoute: () => root,
        path,
        component: () => null,
      }),
    ),
  ];
  const router = createRouter({
    routeTree: root.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: {} as never,
  });
  return { ...render(<RouterProvider router={router} />), router };
}

describe("NewResourceMenu", () => {
  // w4/545: the trigger and the menu it labels are both named "New", never
  // "New New" from a duplicate hidden copy of the label.
  it("names the trigger and its menu New, once", async () => {
    const user = userEvent.setup();
    renderMenu();

    const trigger = await screen.findByRole("button", { name: "New" });
    await user.click(trigger);
    expect(screen.getByRole("menu", { name: "New" })).toBeInTheDocument();
  });

  // Render's New menu is flat (live capture 2026-11-21): each service type,
  // then the datastores, with no "New Service" submenu between.
  it("lists every create entry flat, in Render's bare names, Sandbox Group included", async () => {
    const user = userEvent.setup();
    renderMenu();

    await user.click(await screen.findByRole("button", { name: /^New/ }));

    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual([
      "Static Site",
      "Web Service",
      "Private Service",
      "Background Worker",
      "Cron Job",
      "Postgres",
      "Key Value",
      "Project",
      "Blueprint",
      "Sandbox Group",
      "Webhook",
    ]);
    expect(screen.queryByRole("menuitem", { name: "New Service" })).toBeNull();
  });

  it("deep-links each service type and the datastores to their create pages", async () => {
    const user = userEvent.setup();
    const { router } = renderMenu();

    await user.click(await screen.findByRole("button", { name: /^New/ }));
    expect(screen.getByRole("menuitem", { name: "Cron Job" })).toHaveAttribute(
      "href",
      "/cron/new",
    );
    expect(screen.getByRole("menuitem", { name: "Postgres" })).toHaveAttribute(
      "href",
      "/new/database",
    );
    expect(screen.getByRole("menuitem", { name: "Key Value" })).toHaveAttribute(
      "href",
      "/new/redis",
    );
    expect(screen.getByRole("menuitem", { name: "Blueprint" })).toHaveAttribute(
      "href",
      "/blueprint/new",
    );

    await user.click(screen.getByRole("menuitem", { name: "Static Site" }));
    expect(router.state.location.pathname).toBe("/static/new");
  });

  it("opens the overview's sandbox group dialog through the URL", async () => {
    const user = userEvent.setup();
    const { router } = renderMenu();

    await user.click(await screen.findByRole("button", { name: /^New/ }));
    const item = screen.getByRole("menuitem", { name: "Sandbox Group" });
    expect(item).toHaveAttribute("href", "/?new=sandbox-group");

    await user.click(item);
    expect(router.state.location.pathname).toBe("/");
    expect(router.state.location.search).toEqual({ new: "sandbox-group" });
  });
});
