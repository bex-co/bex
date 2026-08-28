import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { InstanceTypeRow } from "@/features/services/components/instance-type-row";
import type { InstanceTypeView } from "@/features/services/hooks/use-instance-types";

const instanceTypesState: {
  byID: (id: string | null | undefined) => InstanceTypeView | undefined;
  loading: boolean;
} = {
  byID: () => undefined,
  loading: false,
};

vi.mock("@/features/services/hooks/use-instance-types", () => ({
  useInstanceTypes: () => instanceTypesState,
}));

const STANDARD: InstanceTypeView = {
  id: "standard",
  name: "Standard",
  cpu: "1",
  memory: "2Gi",
  monthlyUsd: "17.50",
};

function renderRow(plan: string | null) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <InstanceTypeRow serviceId="app" plan={plan} />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  instanceTypesState.byID = () => undefined;
  instanceTypesState.loading = false;
});

describe("InstanceTypeRow", () => {
  it("renders the plan's name, CPU, and memory when the catalog resolves it", async () => {
    instanceTypesState.byID = (id) =>
      id === "standard" ? STANDARD : undefined;
    renderRow("standard");

    expect(await screen.findByText("Standard")).toBeInTheDocument();
    expect(screen.getByText("1 CPU")).toBeInTheDocument();
    expect(screen.getByText("2 GB")).toBeInTheDocument();
  });

  it("falls back to the raw plan id when the catalog no longer recognizes it", async () => {
    instanceTypesState.byID = () => undefined;
    renderRow("legacy_tier");

    expect(await screen.findByText("legacy_tier")).toBeInTheDocument();
  });

  it("shows an honest no-plan state for an untiered App", async () => {
    renderRow(null);
    expect(await screen.findByText("No instance type set")).toBeInTheDocument();
  });

  it("links Update to the plan picker for this service", async () => {
    renderRow("standard");
    instanceTypesState.byID = (id) =>
      id === "standard" ? STANDARD : undefined;

    const link = await screen.findByRole("link", { name: "Update" });
    expect(link).toHaveAttribute("href", "/services/app/plan");
  });
});
