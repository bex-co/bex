import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { Route as WAliasRoute } from "../w.$";
import type { WorkspaceView } from "@/features/workspaces/types";

// The alias selects the switcher's workspace from the URL — mock the context
// seam it drives.
const setCurrentWorkspaceId = vi.fn();
const workspaceState: {
  workspaces: WorkspaceView[];
  loading: boolean;
  currentWorkspaceId: string | null;
} = { workspaces: [], loading: false, currentWorkspaceId: null };
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ ...workspaceState, setCurrentWorkspaceId }),
}));

function ws(id: string): WorkspaceView {
  return { id, name: id, plan: "hobby", createdAt: null } as WorkspaceView;
}

/** Mounts the real /w/$ component (not the beforeLoad — the id-less forms
 *  redirect there; these tests cover the NAMED-id flow) plus stub landings. */
function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  const alias = createRoute({
    getParentRoute: () => rootRoute,
    path: "/w/$",
    component: WAliasRoute.options.component!,
  });
  const landings = ["/workspace/settings", "/usage"].map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <div>landed:{path}</div>,
    }),
  );
  const home = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <div>landed:/</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([alias, home, ...landings]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  workspaceState.workspaces = [ws("tea-mine"), ws("tea-other")];
  workspaceState.loading = false;
  workspaceState.currentWorkspaceId = "tea-mine";
  setCurrentWorkspaceId.mockReset();
});

describe("/w/{tea-id} alias (w1/m45)", () => {
  it("selects a member workspace from the URL and lands on settings", async () => {
    const router = renderAt("/w/tea-other/settings");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/workspace/settings"),
    );
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-other");
    // replace, not push: the alias must not stack a history entry.
    expect(router.history.length).toBe(1);
  });

  it("lands billing on Usage (bex's deliberate counterpart)", async () => {
    const router = renderAt("/w/tea-mine/billing");
    await waitFor(() => expect(router.state.location.pathname).toBe("/usage"));
  });

  it("lands the bare workspace root on the overview", async () => {
    const router = renderAt("/w/tea-other");
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(setCurrentWorkspaceId).toHaveBeenCalledWith("tea-other");
  });

  it("refuses a foreign workspace id — not-found, never the caller's own settings", async () => {
    const router = renderAt("/w/tea-foreign/settings");
    // The not-found page renders; no navigation, no selection change.
    expect(await screen.findByText(/not found/i)).toBeInTheDocument();
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
    expect(router.state.location.pathname).toBe("/w/tea-foreign/settings");
  });

  it("does not judge the id while the membership list is loading", async () => {
    workspaceState.loading = true;
    renderAt("/w/tea-foreign/settings");
    await waitFor(() =>
      expect(screen.queryByText(/not found/i)).not.toBeInTheDocument(),
    );
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });

  it("skips the redundant selection write when the URL names the current workspace", async () => {
    const router = renderAt("/w/tea-mine/settings");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/workspace/settings"),
    );
    expect(setCurrentWorkspaceId).not.toHaveBeenCalled();
  });
});
