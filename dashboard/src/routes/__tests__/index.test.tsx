import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { HomePage } from "../index";
import type { ServiceView } from "@/features/services/types";

// The route is a pure client of the services hooks; mock them so the test
// drives the list/loading/error/empty states directly.
const servicesState: {
  services: ServiceView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<ServiceView[]>;
} = {
  services: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => servicesState,
}));

const run = vi.fn();
vi.mock("@/features/services/hooks/use-service-lifecycle", () => ({
  useServiceLifecycle: () => ({ pending: null, run }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: null,
    replicas: 1,
    revision: "r1",
    ...overrides,
  };
}

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
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  servicesState.services = [];
  servicesState.loading = false;
  servicesState.error = undefined;
  run.mockReset();
});

describe("HomePage", () => {
  it("renders live services with status badges, overview links, and row actions", async () => {
    servicesState.services = [
      svc({ id: "beancount-cms", name: "beancount-cms", phase: "Running" }),
      svc({ id: "hello-go", name: "hello-go", phase: "Running" }),
      svc({
        id: "worker-queue",
        name: "worker-queue",
        suspended: true,
        phase: "Hibernated",
        url: null,
      }),
    ];

    renderHomePage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("beancount-cms")).toBeInTheDocument();
    expect(within(table).getByText("hello-go")).toBeInTheDocument();
    expect(within(table).getByText("worker-queue")).toBeInTheDocument();

    // capitalized status badges, suspension winning over phase
    expect(within(table).getAllByText("Running")).toHaveLength(2);
    expect(within(table).getByText("Suspended")).toBeInTheDocument();

    // name links to the service overview page
    expect(
      within(table).getByText("beancount-cms").closest("a"),
    ).toHaveAttribute("href", "/services/beancount-cms");

    // bex-native superset columns render from the live fields
    expect(within(table).getByText("Instances")).toBeInTheDocument();
    expect(within(table).getByText("Revision")).toBeInTheDocument();
    expect(within(table).getByText("Created")).toBeInTheDocument();
    expect(within(table).getAllByText("r1")).toHaveLength(3); // revision per row
    expect(within(table).getAllByText("1")).toHaveLength(3); // replicas per row

    // em-dash fallback for missing values (worker-queue URL + null createdAt)
    expect(within(table).getAllByText("—").length).toBeGreaterThanOrEqual(1);

    // one actions trigger per row
    expect(
      within(table).getAllByRole("button", { name: "Open actions menu" }),
    ).toHaveLength(3);
  });

  it("computes the stat tiles from the live list", async () => {
    servicesState.services = [
      svc({ id: "a", phase: "Running" }),
      svc({ id: "b", phase: "Running" }),
      svc({ id: "c", suspended: true, phase: "Hibernated" }),
    ];
    renderHomePage();

    // Total services = 3, Running = 2, Suspended = 1 (tiles render the value)
    const totalLabel = await screen.findByText("Total services");
    expect(totalLabel.parentElement).toHaveTextContent("3");
    // "Running" appears as a tile label and badge(s); assert the counts exist.
    expect(screen.getAllByText("2").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("1").length).toBeGreaterThanOrEqual(1);
  });

  it("shows skeleton placeholders while loading with no data", async () => {
    servicesState.loading = true;
    renderHomePage();

    // the shell renders, but no real rows yet, and no error/empty copy
    await screen.findByText("Total services");
    expect(
      screen.queryByRole("button", { name: "Open actions menu" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("No services yet")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Couldn't load services"),
    ).not.toBeInTheDocument();
  });

  it("shows an error card when the query fails with no data", async () => {
    servicesState.error = new Error("network down");
    renderHomePage();

    expect(
      await screen.findByText("Couldn't load services"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an empty state when there are no services", async () => {
    renderHomePage();

    expect(await screen.findByText("No services yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
