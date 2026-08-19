import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NewWorkspacePage } from "../new.workspace";

const create = vi.fn();
vi.mock("@/features/workspaces/hooks/use-create-workspace", () => ({
  useCreateWorkspace: () => ({ create, busy: false, error: null }),
}));

const setCurrentWorkspaceId = vi.fn();
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ setCurrentWorkspaceId }),
}));

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/new/workspace",
    component: NewWorkspacePage,
  });
  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <p>Workspace home</p>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, homeRoute]),
    history: createMemoryHistory({ initialEntries: ["/new/workspace"] }),
    context: { client: {} as never, session: null },
  });
  return { ...render(<RouterProvider router={router} />), router };
}

beforeEach(() => {
  vi.clearAllMocks();
  create.mockResolvedValue({
    id: "tea-returned-id",
    name: "acme",
    plan: "hobby",
    role: "admin",
    createdAt: null,
  });
});

describe("NewWorkspacePage post-create landing", () => {
  it("selects the returned workspace before landing on its home context", async () => {
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Name"), "acme");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-returned-id");
    expect(router.history.location.pathname).toBe("/");
  });

  it("does not switch or navigate when create fails", async () => {
    create.mockResolvedValueOnce(null);
    const user = userEvent.setup();
    const { router } = renderPage();

    await user.type(await screen.findByLabelText("Name"), "acme");
    await user.click(screen.getByRole("button", { name: "Create Workspace" }));

    await waitFor(() => expect(create).toHaveBeenCalled());
    expect(router.state.location.pathname).toBe("/new/workspace");
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Name")).toHaveValue("acme");
  });
});
