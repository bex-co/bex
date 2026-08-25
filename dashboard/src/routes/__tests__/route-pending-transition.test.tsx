import { act, render, screen, waitFor } from "@testing-library/react";
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it } from "vitest";
import { BillingPageSkeleton } from "@/common/components/route-skeletons";

describe("deterministic route-pending transition seam (w5/m79)", () => {
  it("holds tailored geometry until a deferred loader reveals ready content", async () => {
    let release!: () => void;
    const blocked = new Promise<void>((resolve) => {
      release = resolve;
    });
    const root = createRootRoute({ component: Outlet });
    const slow = createRoute({
      getParentRoute: () => root,
      path: "/slow",
      loader: () => blocked,
      pendingComponent: BillingPageSkeleton,
      component: () => (
        <main>
          <h1>ready header</h1>
          <section>ready content</section>
        </main>
      ),
    });
    const router = createRouter({
      routeTree: root.addChildren([slow]),
      history: createMemoryHistory({ initialEntries: ["/slow"] }),
      context: {},
      defaultPendingMs: 0,
      defaultPendingMinMs: 0,
    });

    const { container } = render(<RouterProvider router={router} />);

    await waitFor(() =>
      expect(
        container.querySelector('[data-route-skeleton="billing"]'),
      ).not.toBeNull(),
    );
    expect(
      container.querySelectorAll(
        '[data-skeleton-region="page-header"], [data-skeleton-region="plan"], [data-skeleton-region="charges"], [data-skeleton-region="invoice-history"]',
      ),
    ).toHaveLength(4);
    expect(screen.queryByText("ready header")).not.toBeInTheDocument();

    await act(async () => {
      release();
      await blocked;
    });
    await waitFor(() => expect(screen.getByText("ready header")).toBeVisible());
    expect(
      container.querySelector('[data-route-skeleton="billing"]'),
    ).toBeNull();
  });
});
