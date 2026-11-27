import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  RouterProvider,
  createRootRoute,
  createRoute,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { Route as OrganizationAliasRoute } from "../organization.$";
import type { WorkspaceView } from "@/features/workspaces/types";

// Render's `/organization/<id>/billing` (w3/m705) is the org's primary
// workspace's Billing in bex: the alias reads that workspace from the
// switcher's list and hands it to `/w/<tea-id>/billing`, which checks the
// caller is in it. An org none of the caller's workspaces belongs to goes to
// the org's page, which refuses it.
const workspaceState: { workspaces: WorkspaceView[]; loading: boolean } = {
  workspaces: [],
  loading: false,
};
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => workspaceState,
}));

function ws(id: string, orgId?: string, primaryId?: string): WorkspaceView {
  return {
    id,
    name: id,
    plan: "scale",
    createdAt: null,
    organization: orgId
      ? { id: orgId, name: orgId, primaryWorkspaceId: primaryId ?? id }
      : null,
  } as WorkspaceView;
}

async function renderAt(initialPath: string) {
  const rootRoute = createRootRoute();
  const alias = createRoute({
    getParentRoute: () => rootRoute,
    path: "/organization/$",
    beforeLoad: OrganizationAliasRoute.options.beforeLoad as never,
    component: OrganizationAliasRoute.options.component!,
  });
  const landings = ["/w/$", "/org/$orgId/workspaces"].map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => <div>landed:{path}</div>,
    }),
  );
  const router = createRouter({
    // The file Route's options are typed for the generated tree, not this one.
    routeTree: rootRoute.addChildren([alias as never, ...landings]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { client: {} as never, session: {} as never },
  });
  await router.load();
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  workspaceState.loading = false;
  workspaceState.workspaces = [
    ws("tea-primary", "org-acme"),
    ws("tea-ml", "org-acme", "tea-primary"),
    ws("tea-solo"),
  ];
});

describe("/organization/{org-id}/billing (w3/m705)", () => {
  it("opens the org's primary workspace's Billing through the workspace alias", async () => {
    const router = await renderAt("/organization/org-acme/billing");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/w/tea-primary/billing"),
    );
    expect(router.history.length).toBe(1);
  });

  it("finds the primary from any of the org's workspaces the caller is in", async () => {
    workspaceState.workspaces = [ws("tea-ml", "org-acme", "tea-primary")];
    const router = await renderAt("/organization/org-acme/billing");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/w/tea-primary/billing"),
    );
  });

  it("sends an org none of the caller's workspaces is in to the org's page and its refusal", async () => {
    const router = await renderAt("/organization/org-other/billing");
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/org/org-other/workspaces"),
    );
    expect(
      screen.getByText("landed:/org/$orgId/workspaces"),
    ).toBeInTheDocument();
  });

  it("waits for the switcher's list before judging the org", async () => {
    workspaceState.loading = true;
    workspaceState.workspaces = [];
    const router = await renderAt("/organization/org-acme/billing");
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(router.state.location.pathname).toBe(
      "/organization/org-acme/billing",
    );
  });

  it("redirects every other sub-path before the component renders", async () => {
    const router = await renderAt("/organization/org-acme/members");
    expect(router.state.location.pathname).toBe("/org/org-acme/members");
  });
});
