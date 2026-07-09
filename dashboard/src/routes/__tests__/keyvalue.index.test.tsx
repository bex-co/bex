import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { KeyValuePage } from "../keyvalue.index";
import type { KeyValueView } from "@/features/keyvalue/types";

// The route is a pure client of the keyvalue hooks; mock them so the test
// drives the list/loading/error/empty states directly (mirrors the databases
// DatabasesPage test).
const keyValuesState: {
  keyValues: KeyValueView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<KeyValueView[]>;
  startPolling: (ms: number) => void;
  stopPolling: () => void;
} = {
  keyValues: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
  startPolling: vi.fn(),
  stopPolling: vi.fn(),
};
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => keyValuesState,
}));

// The row actions dropdown mounts inside the page; stub its hook.
vi.mock("@/features/keyvalue/hooks/use-delete-key-value", () => ({
  useDeleteKeyValue: () => ({ remove: vi.fn(), deleting: null }),
}));

function kv(overrides: Partial<KeyValueView> = {}): KeyValueView {
  return {
    id: "sessions-cache",
    name: "sessions-cache",
    status: "available",
    plan: "starter",
    version: "8",
    createdAt: null,
    externalHost: null,
    public: false,
    suspended: false,
    ...overrides,
  };
}

function renderPage() {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: KeyValuePage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  keyValuesState.keyValues = [];
  keyValuesState.loading = false;
  keyValuesState.error = undefined;
});

describe("KeyValuePage", () => {
  it("renders live Key Value stores with status chips, detail links, and row actions", async () => {
    keyValuesState.keyValues = [
      kv({ id: "sessions-cache", name: "sessions-cache", status: "available" }),
      kv({ id: "rate-limiter", name: "rate-limiter", status: "creating", plan: "free" }),
    ];
    renderPage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("sessions-cache")).toBeInTheDocument();
    expect(within(table).getByText("rate-limiter")).toBeInTheDocument();
    expect(within(table).getByText("Available")).toBeInTheDocument();
    expect(within(table).getByText("Creating")).toBeInTheDocument();

    // name links to the detail page
    expect(
      within(table).getByText("sessions-cache").closest("a"),
    ).toHaveAttribute("href", "/keyvalue/sessions-cache");

    // one actions trigger per row + a create link in the header
    expect(
      within(table).getAllByRole("button", { name: "Open actions menu" }),
    ).toHaveLength(2);
    expect(
      screen.getByRole("link", { name: /new key value/i }),
    ).toHaveAttribute("href", "/keyvalue/new");
  });

  it("computes the stat tiles from the live list", async () => {
    keyValuesState.keyValues = [
      kv({ id: "a", status: "available" }),
      kv({ id: "b", status: "available" }),
      kv({ id: "c", status: "creating" }),
    ];
    renderPage();
    const totalLabel = await screen.findByText("Total Key Value stores");
    expect(totalLabel.parentElement).toHaveTextContent("3");
  });

  it("shows an error card when the query fails with no data", async () => {
    keyValuesState.error = new Error("network down");
    renderPage();
    expect(
      await screen.findByText("Couldn't load Key Value stores"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an empty state when there are no Key Value stores", async () => {
    renderPage();
    expect(await screen.findByText("No Key Value stores yet")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
