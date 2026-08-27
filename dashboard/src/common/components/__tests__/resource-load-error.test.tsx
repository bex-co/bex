import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ResourceLoadError } from "../resource-load-error";

function renderInRouter(node: React.ReactNode) {
  const rootRoute = createRootRoute();
  const detail = createRoute({
    getParentRoute: () => rootRoute,
    path: "/services/$serviceId",
    component: () => <>{node}</>,
  });
  const login = createRoute({
    getParentRoute: () => rootRoute,
    path: "/auth/login",
    validateSearch: (search: Record<string, unknown>) => ({
      next: typeof search.next === "string" ? search.next : undefined,
    }),
    component: () => <div>login page</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([detail, login]),
    history: createMemoryHistory({ initialEntries: ["/services/srv-1"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

describe("ResourceLoadError (w3/m80 t002)", () => {
  beforeEach(() => {
    // useSignInAgain reads window.location for the returnTo; anchor it to the
    // page the user is "on".
    window.history.replaceState(null, "", "/services/srv-1?tab=env");
  });

  it("shows the expired-session state and signs back in with returnTo", async () => {
    const router = renderInRouter(
      <ResourceLoadError variant="unauthenticated" />,
    );

    expect(
      await screen.findByText("Your session has expired"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/auth/login"),
    );
    expect(router.state.location.search).toEqual({
      next: "/services/srv-1?tab=env",
    });
  });

  it("shows the network error state with a working retry, not the auth state", async () => {
    const onRetry = vi.fn();
    renderInRouter(<ResourceLoadError onRetry={onRetry} />);

    expect(
      await screen.findByText("Something went wrong"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Your session has expired")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
