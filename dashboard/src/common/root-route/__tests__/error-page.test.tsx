import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";
import ErrorPage from "../error-page";

describe("route error retry", () => {
  it("retries a failed beforeLoad and preserves the cold destination", async () => {
    const beforeLoad = vi
      .fn()
      .mockRejectedValueOnce(new Error("Memberships unavailable"))
      .mockResolvedValue({});
    const root = createRootRoute({ component: Outlet });
    const destination = createRoute({
      getParentRoute: () => root,
      path: "/agents",
      beforeLoad,
      component: () => <div>Recovered destination</div>,
    });
    const href = "/agents?archived=all#history";
    const router = createRouter({
      routeTree: root.addChildren([destination]),
      history: createMemoryHistory({ initialEntries: [href] }),
      defaultErrorComponent: ErrorPage,
      defaultPendingMinMs: 0,
    });
    // React and the router report the intentional boundary failure.
    const report = vi.spyOn(console, "error").mockImplementation(() => {});
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      render(<RouterProvider router={router} />);
      expect(await screen.findByText("Memberships unavailable")).toBeVisible();
      expect(beforeLoad).toHaveBeenCalledOnce();

      fireEvent.click(screen.getByRole("button", { name: "Try again" }));

      await waitFor(() => expect(beforeLoad).toHaveBeenCalledTimes(2));
      expect(await screen.findByText("Recovered destination")).toBeVisible();
      expect(router.state.location.href).toBe(href);
    } finally {
      report.mockRestore();
      warn.mockRestore();
    }
  });
});
