import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { BlueprintsPage } from "../blueprints";
import { BlueprintDetailPage } from "../blueprints.$blueprintId";
import type { BlueprintView } from "@/features/blueprints/types";

// --- list hook mock ---
const blueprintsState: {
  blueprints: BlueprintView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = { blueprints: [], loading: false, error: undefined, refetch: vi.fn(async () => undefined) };
vi.mock("@/features/blueprints/hooks/use-blueprints", () => ({
  useBlueprints: () => blueprintsState,
}));

// --- detail hook mock ---
const blueprintDetailState: {
  blueprint: BlueprintView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => void;
} = { blueprint: null, loading: false, error: undefined, refetch: vi.fn() };
vi.mock("@/features/blueprints/hooks/use-blueprint", () => ({
  useBlueprint: () => blueprintDetailState,
}));

// --- sync hook mock ---
const sync = vi.fn(async () => null);
vi.mock("@/features/blueprints/hooks/use-sync-blueprint", () => ({
  useSyncBlueprint: () => ({ sync, busy: false }),
}));

// --- validate hook stub (used by ValidatePanel inside detail page) ---
vi.mock("@/features/blueprints/hooks/use-validate-blueprint", () => ({
  useValidateBlueprint: () => ({
    validate: vi.fn(async () => null),
    result: null,
    loading: false,
  }),
}));

function bp(overrides: Partial<BlueprintView> = {}): BlueprintView {
  return {
    id: "blp-abc123",
    name: "hello-go",
    repo: "https://github.com/example/hello-go",
    branch: "main",
    manifest: "services:\n  - name: api\n    type: web_service",
    status: "active",
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-13T00:00:00Z",
    ...overrides,
  };
}

function renderBlueprintsPage() {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints",
    component: BlueprintsPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/blueprints"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

function renderDetailPage(blueprintId = "blp-abc123") {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints/$blueprintId",
    component: BlueprintDetailPage,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [`/blueprints/${blueprintId}`] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  blueprintsState.blueprints = [];
  blueprintsState.loading = false;
  blueprintsState.error = undefined;
  blueprintDetailState.blueprint = null;
  blueprintDetailState.loading = false;
  blueprintDetailState.error = undefined;
  sync.mockReset();
  sync.mockResolvedValue(null);
});

describe("BlueprintsPage (list)", () => {
  it("renders blueprint rows with name/repo/branch/status", async () => {
    blueprintsState.blueprints = [
      bp({ name: "hello-go", repo: "https://github.com/example/hello-go", branch: "main" }),
    ];

    renderBlueprintsPage();

    expect(await screen.findByText("hello-go")).toBeInTheDocument();
    expect(screen.getByText("https://github.com/example/hello-go")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows empty state with auto-registration copy when there are no blueprints", async () => {
    renderBlueprintsPage();

    expect(await screen.findByText("No blueprints yet")).toBeInTheDocument();
    expect(screen.getByText(/auto-register/i)).toBeInTheDocument();
  });

  it("shows an error state when the query fails", async () => {
    blueprintsState.error = new Error("network down");
    renderBlueprintsPage();

    expect(await screen.findByText("Couldn't load blueprints")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});

describe("BlueprintDetailPage", () => {
  it("renders the manifest and metadata for a found blueprint", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage();

    expect(await screen.findAllByText("hello-go")).toHaveLength(2);
    expect(screen.getByText(/services:.*name: api/s)).toBeInTheDocument();
    expect(screen.getByText("https://github.com/example/hello-go")).toBeInTheDocument();
  });

  it("shows not-found state when blueprint is null after loading", async () => {
    blueprintDetailState.blueprint = null;
    renderDetailPage("blp-missing");

    expect(await screen.findByText("Blueprint not found")).toBeInTheDocument();
  });

  it("calls sync and refetch when the confirm dialog is accepted", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage();

    await userEvent.click(await screen.findByRole("button", { name: /sync/i }));
    await userEvent.click(screen.getByRole("button", { name: /^sync$/i }));

    expect(sync).toHaveBeenCalledWith("blp-abc123");
    expect(blueprintDetailState.refetch).toHaveBeenCalled();
  });

  it("does not call sync when the confirm dialog is cancelled", async () => {
    blueprintDetailState.blueprint = bp();
    renderDetailPage();

    await userEvent.click(await screen.findByRole("button", { name: /sync/i }));
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));

    expect(sync).not.toHaveBeenCalled();
  });
});
