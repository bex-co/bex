import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { PlanCard } from "@/features/usage/components/plan-card";
import type { WorkspaceView } from "@/features/workspaces/types";

const workspaceState = vi.hoisted(() => ({
  currentWorkspace: null as WorkspaceView | null,
  workspaces: [] as WorkspaceView[],
  loading: false,
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => workspaceState,
}));

const PRIMARY: WorkspaceView = {
  id: "tea-primary",
  name: "acme",
  slug: "acme",
  plan: "scale",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: {
    id: "org-1",
    name: "Acme",
    primaryWorkspaceId: "tea-primary",
    requireTwoFactor: false,
    requireGoogleLogin: false,
  },
  primary: true,
  privacy: "invite_only",
};
const SECONDARY: WorkspaceView = {
  ...PRIMARY,
  id: "tea-secondary",
  name: "staging",
  primary: false,
  privacy: "public",
};

async function renderCard() {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/billing",
    component: PlanCard,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/billing"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  workspaceState.currentWorkspace = PRIMARY;
  workspaceState.workspaces = [PRIMARY, SECONDARY];
  workspaceState.loading = false;
});

describe("PlanCard", () => {
  // w3/m708: Render's plan page, which comes back to Billing.
  it("links a workspace that holds its own plan to Render's plan page", async () => {
    await renderCard();

    expect(screen.getByRole("link", { name: "Change plan" })).toHaveAttribute(
      "href",
      "/billing/update-plan",
    );
    expect(
      screen.queryByTestId("organization-plan-note"),
    ).not.toBeInTheDocument();
  });

  // w7/m464: "Your primary workspace manages the billing for the
  // organization" (render.com/docs/organizations).
  it("shows a secondary's plan as its organization's, with no change control", async () => {
    workspaceState.currentWorkspace = SECONDARY;
    await renderCard();

    expect(screen.getByText("Scale")).toBeInTheDocument();
    expect(screen.getByTestId("organization-plan-note")).toHaveTextContent(
      "Included in your organization's plan. The organization's primary workspace manages its plan and billing.",
    );
    expect(
      screen.queryByRole("link", { name: "Change plan" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Go to the primary workspace's billing",
      }),
    ).toHaveAttribute("href", "/w/tea-primary/billing");
  });

  it("offers no link to a primary the caller is not in", async () => {
    workspaceState.currentWorkspace = SECONDARY;
    workspaceState.workspaces = [SECONDARY];
    await renderCard();

    expect(screen.getByTestId("organization-plan-note")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
