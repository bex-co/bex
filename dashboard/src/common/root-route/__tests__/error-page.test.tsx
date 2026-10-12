import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

// w4/269: after a deploy, an open tab's lazy route chunk URL answers with HTML.
// Try again re-ran the route inside the same document, re-importing the same
// dead URL forever; only a fresh document at the same href recovers.
describe("route error retry after a bundle update", () => {
  const href = "/project/prj-1?env=evm-1&q=api#environments";

  function renderFailing(error: Error, asRootError = false) {
    const beforeLoad = vi.fn().mockRejectedValue(error);
    const root = createRootRoute({
      component: Outlet,
      ...(asRootError ? { errorComponent: ErrorPage } : {}),
    });
    const destination = createRoute({
      getParentRoute: () => root,
      path: "/project/$projectId",
      beforeLoad,
      component: () => <div>Project</div>,
    });
    const router = createRouter({
      routeTree: root.addChildren([destination]),
      history: createMemoryHistory({ initialEntries: [href] }),
      ...(asRootError ? {} : { defaultErrorComponent: ErrorPage }),
      defaultPendingMinMs: 0,
    });
    render(<RouterProvider router={router} />);
    return { beforeLoad, router };
  }

  const reload = vi.fn();
  beforeEach(() => {
    // React and the router report the intentional boundary failure.
    vi.spyOn(console, "error").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal("location", { ...window.location, reload });
  });
  afterEach(() => {
    reload.mockReset();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it.each([
    "Failed to fetch dynamically imported module: https://dashboard.bex.co/assets/project-sidebar-D2ai0shz.js",
    "error loading dynamically imported module: https://dashboard.bex.co/assets/project-sidebar-D2ai0shz.js",
    "Importing a module script failed.",
  ])("reloads the document once for %s", async (message) => {
    const { beforeLoad, router } = renderFailing(new TypeError(message));
    expect(await screen.findByText(message)).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(reload).toHaveBeenCalledOnce();
    expect(beforeLoad).toHaveBeenCalledOnce();
    expect(router.state.location.href).toBe(href);
  });

  it("does the same from the root route's error registration", async () => {
    const message = "Failed to fetch dynamically imported module: /assets/x.js";
    renderFailing(new TypeError(message), true);
    expect(await screen.findByText(message)).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(reload).toHaveBeenCalledOnce();
  });

  it.each([
    new TypeError("Failed to fetch"),
    new TypeError("Cannot read properties of undefined"),
    new Error("Invalid search params"),
  ])("keeps the in-place retry for $message", async (error) => {
    const { beforeLoad, router } = renderFailing(error);
    expect(await screen.findByText(error.message)).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() => expect(beforeLoad).toHaveBeenCalledTimes(2));
    expect(reload).not.toHaveBeenCalled();
    expect(router.state.location.href).toBe(href);
  });
});
