import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { DatabasesPage } from "../databases.index";
import type {
  DatabaseView,
  DatabaseInstanceTypeView,
} from "@/features/databases/types";

// The route is a pure client of the databases hooks; mock them so the test
// drives the list/loading/error/empty states directly (mirrors the services
// HomePage test).
const databasesState: {
  databases: DatabaseView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<DatabaseView[]>;
  startPolling: (ms: number) => void;
  stopPolling: () => void;
} = {
  databases: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
  startPolling: vi.fn(),
  stopPolling: vi.fn(),
};
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => databasesState,
}));

// The create dialog + row actions mount inside the page; stub their hooks.
const instanceTypes: DatabaseInstanceTypeView[] = [
  { id: "free", name: "Free", cpu: "100m", memory: "256Mi", storageGB: 1 },
];
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes,
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/databases/hooks/use-create-database", () => ({
  useCreateDatabase: () => ({ create: vi.fn(), busy: false }),
}));
vi.mock("@/features/databases/hooks/use-delete-database", () => ({
  useDeleteDatabase: () => ({ remove: vi.fn(), deleting: null }),
}));

function db(overrides: Partial<DatabaseView> = {}): DatabaseView {
  return {
    id: "shop-db",
    name: "shop-db",
    status: "available",
    plan: "free",
    version: "18",
    diskSizeGB: 1,
    createdAt: null,
    public: false,
    ...overrides,
  };
}

function renderPage() {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: DatabasesPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  databasesState.databases = [];
  databasesState.loading = false;
  databasesState.error = undefined;
});

describe("DatabasesPage", () => {
  it("renders live databases with status badges, detail links, and row actions", async () => {
    databasesState.databases = [
      db({ id: "shop-db", name: "shop-db", status: "available" }),
      db({ id: "prov-db", name: "prov-db", status: "creating", plan: "basic-1gb" }),
    ];
    renderPage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("shop-db")).toBeInTheDocument();
    expect(within(table).getByText("prov-db")).toBeInTheDocument();
    expect(within(table).getByText("Available")).toBeInTheDocument();
    expect(within(table).getByText("Creating")).toBeInTheDocument();

    // name links to the detail page
    expect(within(table).getByText("shop-db").closest("a")).toHaveAttribute(
      "href",
      "/databases/shop-db",
    );

    // one actions trigger per row + a create button in the header
    expect(
      within(table).getAllByRole("button", { name: "Open actions menu" }),
    ).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "New Database" }),
    ).toBeInTheDocument();
  });

  it("computes the stat tiles from the live list", async () => {
    databasesState.databases = [
      db({ id: "a", status: "available" }),
      db({ id: "b", status: "available" }),
      db({ id: "c", status: "creating" }),
    ];
    renderPage();
    const totalLabel = await screen.findByText("Total databases");
    expect(totalLabel.parentElement).toHaveTextContent("3");
  });

  it("shows an error card when the query fails with no data", async () => {
    databasesState.error = new Error("network down");
    renderPage();
    expect(
      await screen.findByText("Couldn't load databases"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an empty state when there are no databases", async () => {
    renderPage();
    expect(await screen.findByText("No databases yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
