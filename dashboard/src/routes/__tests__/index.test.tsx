import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { HomePage } from "../index";

function renderHomePage() {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: HomePage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as any, session: null },
  });
  return render(<RouterProvider router={router} />);
}

describe("HomePage", () => {
  it("renders the Services card with its sample rows and statuses", async () => {
    renderHomePage();

    const table = await screen.findByRole("table");

    // "Services" appears both as the sidebar nav link and the card title.
    expect(screen.getAllByText("Services").length).toBeGreaterThanOrEqual(2);
    expect(within(table).getByText("beancount-cms")).toBeInTheDocument();
    expect(within(table).getByText("eden-cms-v2")).toBeInTheDocument();
    expect(within(table).getByText("hello-go")).toBeInTheDocument();
    expect(within(table).getByText("worker-queue")).toBeInTheDocument();

    // status badges
    expect(within(table).getAllByText("running")).toHaveLength(3);
    expect(within(table).getByText("suspended")).toBeInTheDocument();

    // each service name links to its Metrics page (w3/m3)
    expect(
      within(table).getByText("beancount-cms").closest("a"),
    ).toHaveAttribute("href", "/services/beancount-cms/metrics");

    // URL column falls back to an em dash when a service has no URL
    expect(
      within(table).getByText("https://eden-cms-v2.onbex.co"),
    ).toBeInTheDocument();
    expect(within(table).getByText("—")).toBeInTheDocument();
  });
});
