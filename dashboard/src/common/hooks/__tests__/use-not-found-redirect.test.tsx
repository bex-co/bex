import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { useNotFoundRedirect } from "../use-not-found-redirect";

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args) },
}));

function Probe({ notFound }: { notFound: boolean }) {
  useNotFoundRedirect(notFound);
  return <div>detail page</div>;
}

function renderAt(notFound: boolean) {
  const rootRoute = createRootRoute();
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>home page</div>,
  });
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/things/$thingId",
    component: () => <Probe notFound={notFound} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, detail]),
    history: createMemoryHistory({ initialEntries: ["/things/dead-id"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  toastError.mockReset();
});

describe("useNotFoundRedirect (w9/m55)", () => {
  it("replaces a dead resource URL with / and toasts why", async () => {
    const router = renderAt(true);

    expect(await screen.findByText("home page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    // replace, not push: the dead URL must not stack a history entry.
    expect(router.history.length).toBe(1);
    expect(toastError).toHaveBeenCalledWith(
      "That resource doesn't exist or was deleted.",
      { id: "resource-not-found" },
    );
  });

  it("stays put while the resource is not provably absent", async () => {
    const router = renderAt(false);

    expect(await screen.findByText("detail page")).toBeInTheDocument();
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/things/dead-id"),
    );
    expect(toastError).not.toHaveBeenCalled();
  });
});
