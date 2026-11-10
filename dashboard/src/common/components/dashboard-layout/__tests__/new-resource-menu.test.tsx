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
      "/services/new",
      "/keyvalue/new",
      "/blueprints/new",
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
  it("lists every create entry, Sandbox Group included, under one heading", async () => {
    const user = userEvent.setup();
    renderMenu();

    await user.click(await screen.findByRole("button", { name: /^New/ }));

    expect(screen.getByText("Create a resource")).toBeInTheDocument();
    expect(
      screen.getAllByRole("menuitem").map((item) => item.textContent),
    ).toEqual([
      "New Service",
      "New Postgres",
      "New Key Value",
      "New Blueprint Instance",
      "New Project",
      "New Sandbox Group",
      "New Webhook",
    ]);
  });

  it("opens the overview's sandbox group dialog through the URL", async () => {
    const user = userEvent.setup();
    const { router } = renderMenu();

    await user.click(await screen.findByRole("button", { name: /^New/ }));
    const item = screen.getByRole("menuitem", { name: "New Sandbox Group" });
    expect(item).toHaveAttribute("href", "/?new=sandbox-group");

    await user.click(item);
    expect(router.state.location.pathname).toBe("/");
    expect(router.state.location.search).toEqual({ new: "sandbox-group" });
  });
});
