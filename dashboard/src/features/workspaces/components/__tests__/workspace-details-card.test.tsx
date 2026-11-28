import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, it, expect, vi } from "vitest";
import { render as renderRaw, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterContextProvider,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { shape } from "@/test/card-shape";
import { WorkspaceDetailsCard } from "@/features/workspaces/components/workspace-details-card";
import { WorkspaceDetailsCardSkeleton } from "@/features/workspaces/components/workspace-details-card-skeleton";
import {
  organizationPlanOwner,
  type WorkspaceView,
} from "@/features/workspaces/types";

const rename = vi.fn();
vi.mock("@/features/workspaces/hooks/use-rename-workspace", () => ({
  useRenameWorkspace: () => ({ rename, busy: false, error: null }),
}));

// The caller's workspaces: a secondary links to its primary only when the
// caller is in it.
const workspaceList = vi.hoisted(() => ({ value: [] as { id: string }[] }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ refetch: vi.fn(), workspaces: workspaceList.value }),
}));

// The Avatar row (w1/m648): its upload and reset are the editor's own tests;
// here only who may use them. Admin unless a test says otherwise.
const avatarActions = vi.hoisted(() => ({ upload: vi.fn(), reset: vi.fn() }));
vi.mock("@/features/avatars/hooks/use-avatar-actions", async (original) => ({
  ...(await original<
    typeof import("@/features/avatars/hooks/use-avatar-actions")
  >()),
  useAvatarActions: () => avatarActions,
}));
const canManage = vi.hoisted(() => ({ value: true }));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManage: canManage.value }),
}));

const workspace: WorkspaceView = {
  id: "tea-1",
  name: "acme",
  slug: "acme",
  plan: "hobby",
  role: "admin",
  createdAt: null,
  requireTwoFactor: false,
  requireGoogleLogin: false,
  organization: null,
  primary: false,
  privacy: null,
};

/**
 * The card's links resolve through the router: a real one, with the card
 * rendered straight into its context so `rerender` re-renders the card.
 */
async function routedWrapper() {
  const rootRoute = createRootRoute();
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      createRoute({ getParentRoute: () => rootRoute, path: "/" }),
    ]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  await router.load();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <RouterContextProvider router={router}>{children}</RouterContextProvider>
  );
  return { router, wrapper };
}

let wrapper: Awaited<ReturnType<typeof routedWrapper>>["wrapper"];
let router: Awaited<ReturnType<typeof routedWrapper>>["router"];
beforeEach(async () => {
  ({ router, wrapper } = await routedWrapper());
});

/** Renders under the router (testing-library keeps the wrapper on rerender). */
function render(ui: ReactElement) {
  return renderRaw(ui, { wrapper });
}

describe("WorkspaceDetailsCard", () => {
  // Render's Settings links its Plan row to the plan page, which returns to
  // Billing (w3/m708); the dialog this card opened is gone.
  it("links Change plan to Render's plan page, returning to Billing", async () => {
    const user = userEvent.setup();
    render(<WorkspaceDetailsCard workspace={workspace} />);

    const link = screen.getByRole("link", { name: "Change plan" });
    expect(link).toHaveAttribute("href", "/billing/update-plan");
    await user.click(link);
    expect(router.state.location.pathname).toBe("/billing/update-plan");
    expect(router.state.location.state.afterPlanUpdate).toBe("/billing");
  });

  it("renders the created date long-form, or an em dash when unknown", () => {
    // Zone-less input parses as local time, so this holds in any runner TZ.
    const { rerender } = render(
      <WorkspaceDetailsCard
        workspace={{ ...workspace, createdAt: "2026-07-16T00:57:00" }}
      />,
    );
    expect(screen.getByText("July 16, 2026")).toBeInTheDocument();

    rerender(<WorkspaceDetailsCard workspace={workspace} />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});

// w5/059: the switcher swaps this card's workspace prop without remounting it,
// so a draft seeded once at mount kept the PREVIOUS workspace's name in the
// input while Plan/ID/Created updated around it. Save sits beside that input,
// so pressing it would have renamed the newly selected workspace to the old
// one's name.
describe("WorkspaceDetailsCard — switching workspaces (w5/059)", () => {
  const other: WorkspaceView = {
    id: "tea-2",
    name: "beta",
    slug: "beta",
    plan: "pro",
    role: "admin",
    createdAt: null,
    requireTwoFactor: false,
    requireGoogleLogin: false,
    organization: null,
    primary: false,
    privacy: null,
  };

  it("re-seeds the name input when pointed at a different workspace", () => {
    const { rerender } = render(<WorkspaceDetailsCard workspace={workspace} />);
    expect(screen.getByDisplayValue("acme")).toBeInTheDocument();

    rerender(<WorkspaceDetailsCard workspace={other} />);
    expect(screen.getByDisplayValue("beta")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("acme")).not.toBeInTheDocument();
  });

  it("does not clobber an in-progress edit of the same workspace", async () => {
    // The rename flow refetches on success, which re-renders this card with the
    // same workspace — a name-keyed reset would wipe whatever is being typed.
    const { rerender } = render(<WorkspaceDetailsCard workspace={workspace} />);
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "acme-renamed");

    rerender(<WorkspaceDetailsCard workspace={workspace} />);
    expect(screen.getByDisplayValue("acme-renamed")).toBeInTheDocument();
  });

  // w3/m246: the name is Render's freeform display name; the slug is shown
  // read-only next to the id.
  it("renames to a freeform name and shows the slug", async () => {
    rename.mockReset();
    rename.mockResolvedValue(true);
    render(
      <WorkspaceDetailsCard workspace={{ ...workspace, slug: "acme-key" }} />,
    );
    expect(screen.getByText("acme-key")).toBeInTheDocument();
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "  Tian Pan's Workspace ");
    expect(input).toHaveValue("  Tian Pan's Workspace ");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(rename).toHaveBeenCalledWith("tea-1", "Tian Pan's Workspace");
  });

  it("refuses a blank name", async () => {
    render(<WorkspaceDetailsCard workspace={workspace} />);
    const input = screen.getByDisplayValue("acme");
    await userEvent.clear(input);
    await userEvent.type(input, "   ");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });
});

// w7/m464 (ADR115 D2): a secondary workspace's plan is its organization's,
// managed on the primary, as on Render: no change control here, and a link to
// the primary's settings.
describe("WorkspaceDetailsCard — an organization's secondary (w7/m464)", () => {
  const secondary: WorkspaceView = {
    ...workspace,
    id: "tea-secondary",
    plan: "scale",
    organization: {
      id: "org-1",
      name: "Acme",
      primaryWorkspaceId: "tea-primary",
      requireTwoFactor: false,
      requireGoogleLogin: false,
    },
    primary: false,
    privacy: "public",
  };

  /** The note links through the router, so it renders under a real one. */
  async function renderUnderRouter(ui: ReactElement) {
    const rootRoute = createRootRoute();
    const route = createRoute({
      getParentRoute: () => rootRoute,
      path: "/",
      component: () => ui,
    });
    const router = createRouter({
      routeTree: rootRoute.addChildren([route]),
      history: createMemoryHistory({ initialEntries: ["/"] }),
      context: { client: {} as never, session: null },
    });
    await router.load();
    return renderRaw(<RouterProvider router={router} />);
  }

  it("shows the plan read-only, with a link to the primary's settings", async () => {
    workspaceList.value = [{ id: "tea-primary" }, { id: "tea-secondary" }];
    await renderUnderRouter(<WorkspaceDetailsCard workspace={secondary} />);

    expect(screen.getByText("Scale")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Change plan" }),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("organization-plan-note")).toHaveTextContent(
      "Included in your organization's plan. The organization's primary workspace manages its plan and billing.",
    );
    expect(
      screen.getByRole("link", { name: "Go to the primary workspace" }),
    ).toHaveAttribute("href", "/w/tea-primary/settings");
  });

  it("names the primary without a link when the caller is not in it", async () => {
    workspaceList.value = [{ id: "tea-secondary" }];
    await renderUnderRouter(<WorkspaceDetailsCard workspace={secondary} />);

    expect(screen.getByTestId("organization-plan-note")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Change plan" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the change-plan control on the organization's primary", () => {
    workspaceList.value = [{ id: "tea-primary" }];
    render(
      <WorkspaceDetailsCard
        workspace={{ ...secondary, id: "tea-primary", primary: true }}
      />,
    );

    expect(screen.getByRole("link", { name: "Change plan" })).toHaveAttribute(
      "href",
      "/billing/update-plan",
    );
    expect(
      screen.queryByTestId("organization-plan-note"),
    ).not.toBeInTheDocument();
  });

  // AGENTS.md "Shape parity is mandatory": the pending card reserves the
  // note's own paragraph, with the same copy, so the swap shifts nothing.
  it.each([
    ["in the primary", ["tea-primary", "tea-secondary"]],
    ["not in the primary", ["tea-secondary"]],
  ])("previews the note's paragraph for a caller %s", async (_, ids) => {
    workspaceList.value = ids.map((id) => ({ id }));
    const { container: pending } = render(
      <WorkspaceDetailsCardSkeleton
        planOwner={organizationPlanOwner(secondary, workspaceList.value)}
      />,
    );
    const { container: ready } = await renderUnderRouter(
      <WorkspaceDetailsCard workspace={secondary} />,
    );
    const pendingNote = pending.querySelector<HTMLElement>(
      '[data-skeleton-region="organization-plan-note"]',
    )!;
    const readyNote = ready.querySelector<HTMLElement>(
      '[data-testid="organization-plan-note"]',
    )!;
    expect(pendingNote.className).toBe(readyNote.className);
    // The same words, reserved invisibly: the bar wraps as the copy does.
    expect(pendingNote.textContent).toBe(readyNote.textContent);
    expect(pendingNote.querySelector("a")).toBeNull();
    // The note is the content's last child in both, after the same grid.
    const pendingContent = pendingNote.parentElement!;
    const readyContent = readyNote.parentElement!;
    expect(pendingContent.lastElementChild).toBe(pendingNote);
    expect(readyContent.lastElementChild).toBe(readyNote);
    expect(shape(pendingContent.querySelector("dl")!).split("[")[0]).toBe(
      shape(readyContent.querySelector("dl")!).split("[")[0],
    );
  });

  it("draws no note for a standalone workspace's pending card", () => {
    const { container } = render(
      <WorkspaceDetailsCardSkeleton
        planOwner={organizationPlanOwner(workspace, [])}
      />,
    );
    expect(
      container.querySelector(
        '[data-skeleton-region="organization-plan-note"]',
      ),
    ).toBeNull();
  });

  // Render's Avatar row, after the name (w1/m648): "Only workspace admins
  // can set the workspace image."
  describe("Avatar", () => {
    const card = (view: WorkspaceView = workspace) =>
      render(<WorkspaceDetailsCard workspace={view} />);

    it("shows an Admin the monogram of the name with Render's Edit menu", async () => {
      canManage.value = true;
      const user = userEvent.setup();
      const { container } = card();
      expect(screen.getByText("Avatar")).toBeVisible();
      expect(
        container.querySelector('[data-slot="monogram"]'),
      ).toHaveTextContent("A");
      await user.click(screen.getByRole("button", { name: "Edit" }));
      expect(
        screen.getByRole("menuitem", { name: "Upload image" }),
      ).toBeVisible();
      expect(
        screen.queryByRole("menuitem", { name: "Reset to default" }),
      ).toBeNull();
    });

    it("shows the upload, which an Admin can reset", async () => {
      canManage.value = true;
      const user = userEvent.setup();
      card({ ...workspace, avatarURL: "https://api.bex.test/v1/avatars/k" });
      expect(
        screen.getByRole("img", { name: "User-uploaded Avatar" }),
      ).toHaveAttribute("src", "https://api.bex.test/v1/avatars/k");
      await user.click(screen.getByRole("button", { name: "Edit" }));
      await user.click(
        screen.getByRole("menuitem", { name: "Reset to default" }),
      );
      expect(avatarActions.reset).toHaveBeenCalled();
    });

    it("shows every other member the avatar with Edit disabled and the reason", async () => {
      canManage.value = false;
      const user = userEvent.setup();
      card({ ...workspace, avatarURL: "https://api.bex.test/v1/avatars/k" });
      expect(
        screen.getByRole("img", { name: "User-uploaded Avatar" }),
      ).toBeVisible();
      const edit = screen.getByRole("button", { name: "Edit" });
      expect(edit).toBeDisabled();
      await user.hover(edit.parentElement!);
      expect(
        await screen.findAllByText(
          "Only workspace admins can set the workspace image.",
        ),
      ).not.toHaveLength(0);
      canManage.value = true;
    });

    it("reserves the row in the pending card: the 64 px circle over the Edit button", () => {
      const { container } = render(<WorkspaceDetailsCardSkeleton />);
      expect(screen.getByText("Avatar")).toBeInTheDocument();
      expect(container.querySelector(".size-16.rounded-full")).not.toBeNull();
    });
  });
});
